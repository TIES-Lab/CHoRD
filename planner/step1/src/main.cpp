#include "scev_linear.h"

#include "llvm/ADT/StringRef.h"
#include "llvm/Analysis/ScalarEvolution.h"
#include "llvm/Analysis/ScalarEvolutionExpressions.h"
#include "llvm/Analysis/AssumptionCache.h"
#include "llvm/Analysis/LoopInfo.h"
#include "llvm/Analysis/TargetLibraryInfo.h"
#include "llvm/Analysis/ValueTracking.h"
#include "llvm/IR/Dominators.h"
#include "llvm/IR/CallingConv.h"
#include "llvm/IR/DataLayout.h"
#include "llvm/IR/Function.h"
#include "llvm/IR/Instructions.h"
#include "llvm/IR/IntrinsicsAMDGPU.h"
#include "llvm/IR/LLVMContext.h"
#include "llvm/IR/Module.h"
#include "llvm/IRReader/IRReader.h"
#include "llvm/Support/CommandLine.h"
#include "llvm/Support/ErrorOr.h"
#include "llvm/Support/FileSystem.h"
#include "llvm/Support/JSON.h"
#include "llvm/Support/SourceMgr.h"
#include "llvm/Support/raw_ostream.h"
#include "llvm/ADT/Triple.h"

#include <memory>
#include <cctype>
#include <optional>
#include <string>
#include <unordered_map>
#include <vector>

using llvm::StringRef;

namespace {

llvm::cl::opt<std::string> InputPath(
    "input", llvm::cl::desc("Input LLVM IR/bitcode (.ll/.bc)"),
    llvm::cl::value_desc("filename"), llvm::cl::Required);

llvm::cl::opt<std::string> OutputPath(
    "output", llvm::cl::desc("Output JSON path (kernels.json)"),
    llvm::cl::value_desc("filename"), llvm::cl::Required);

llvm::cl::opt<unsigned> MaxAccessesPerKernel(
    "max-accesses-per-kernel",
    llvm::cl::desc("Max mem-access records per kernel (safety cap)"),
    llvm::cl::init(2000));

llvm::cl::opt<bool> IncludeNonGlobal(
    "include-non-global",
    llvm::cl::desc(
        "Record non-global memory accesses (addrspace!=1). Default: only global."),
    llvm::cl::init(false));

static std::string SCEVToString(const llvm::SCEV* s) {
  std::string out;
  llvm::raw_string_ostream os(out);
  os << *s;
  os.flush();
  return out;
}

static llvm::json::Object LinearExprToJson(const mgvm::LinearExpr& lin) {
  llvm::json::Object o;
  llvm::json::Object coeffObj;
  for (auto& [k, v] : lin.coeffs) {
    coeffObj[k] = static_cast<int64_t>(v);
  }
  o["coeffs"] = std::move(coeffObj);
  o["constant"] = lin.constant;
  o["complete_linear"] = lin.complete;
  llvm::json::Array unk;
  for (auto& t : lin.unknown_terms) {
    unk.push_back(t);
  }
  o["unknown_terms"] = std::move(unk);
  llvm::json::Array refs;
  for (auto& t : lin.var_refs) {
    refs.push_back(t);
  }
  o["var_refs"] = std::move(refs);
  return o;
}


static bool IsKernelFunction(const llvm::Function& F) {
  if (F.isDeclaration())
    return false;

  // AMDGPU kernel
  if (F.getCallingConv() == llvm::CallingConv::AMDGPU_KERNEL)
    return true;

  // SPIR/OpenCL kernel
  if (F.getCallingConv() == llvm::CallingConv::SPIR_KERNEL)
    return true;

  if (F.hasFnAttribute("amdgpu_kernel"))
    return true;

  return false;
}

static std::optional<std::string> IdentifyThreadVar(const llvm::CallBase& CB) {
  // getCalledFunction() returns null if the callee is bitcasted; be robust.
  const llvm::Value* called = CB.getCalledOperand();
  const llvm::Function* callee =
      llvm::dyn_cast<llvm::Function>(called->stripPointerCasts());
  if (!callee)
    return std::nullopt;

  StringRef name = callee->getName();

  // AMDGPU intrinsics (common lowering)
  if (name == "llvm.amdgcn.workitem.id.x")
    return "lid_x";
  if (name == "llvm.amdgcn.workitem.id.y")
    return "lid_y";
  if (name == "llvm.amdgcn.workitem.id.z")
    return "lid_z";
  if (name == "llvm.amdgcn.workgroup.id.x")
    return "wg_x";
  if (name == "llvm.amdgcn.workgroup.id.y")
    return "wg_y";
  if (name == "llvm.amdgcn.workgroup.id.z")
    return "wg_z";

  // Some SPIR/OpenCL pipelines use builtin-style names.
  // Add more patterns as you encounter them.
  if (name.contains("get_local_id")) {
    // Heuristic: get_local_id(dim)
    if (CB.arg_size() >= 1) {
      if (auto* c = llvm::dyn_cast<llvm::ConstantInt>(CB.getArgOperand(0))) {
        switch (c->getZExtValue()) {
          case 0: return "lid_x";
          case 1: return "lid_y";
          case 2: return "lid_z";
          default: break;
        }
      }
    }
  }

  if (name.contains("get_group_id")) {
    if (CB.arg_size() >= 1) {
      if (auto* c = llvm::dyn_cast<llvm::ConstantInt>(CB.getArgOperand(0))) {
        switch (c->getZExtValue()) {
          case 0: return "wg_x";
          case 1: return "wg_y";
          case 2: return "wg_z";
          default: break;
        }
      }
    }
  }


  if (name.contains("get_global_id")) {
    if (CB.arg_size() >= 1) {
      if (auto* c = llvm::dyn_cast<llvm::ConstantInt>(CB.getArgOperand(0))) {
        switch (c->getZExtValue()) {
          case 0: return "gid_x";
          case 1: return "gid_y";
          case 2: return "gid_z";
          default: break;
        }
      }
    }
  }

  return std::nullopt;
}

static std::unordered_map<unsigned, std::string>
ExtractKernelArgNamesFromOpenCLMetadata(const llvm::Module& M,
                                       const llvm::Function& F) {
  std::unordered_map<unsigned, std::string> names;

  const llvm::NamedMDNode* kernels = M.getNamedMetadata("opencl.kernels");
  if (!kernels)
    return names;

  for (unsigned i = 0; i < kernels->getNumOperands(); i++) {
    const llvm::MDNode* node = kernels->getOperand(i);
    if (node->getNumOperands() < 1)
      continue;

    const llvm::ValueAsMetadata* vmd =
        llvm::dyn_cast<llvm::ValueAsMetadata>(node->getOperand(0));
    if (!vmd)
      continue;

    const llvm::Value* v = vmd->getValue();
    const llvm::Function* kf = llvm::dyn_cast<llvm::Function>(v);
    if (!kf || kf != &F)
      continue;

    // This is the metadata node for this kernel.
    // Look for an operand that is a node like: {"kernel_arg_name", ...}
    for (unsigned j = 1; j < node->getNumOperands(); j++) {
      const llvm::MDNode* sub = llvm::dyn_cast<llvm::MDNode>(node->getOperand(j));
      if (!sub || sub->getNumOperands() < 1)
        continue;

      const llvm::MDString* key = llvm::dyn_cast<llvm::MDString>(sub->getOperand(0));
      if (!key)
        continue;

      if (key->getString() == "kernel_arg_name") {
        // operands 1..n are names
        for (unsigned a = 1; a < sub->getNumOperands(); a++) {
          if (const llvm::MDString* n = llvm::dyn_cast<llvm::MDString>(sub->getOperand(a))) {
            names[a - 1] = n->getString().str();
          }
        }
      }
    }
  }

  return names;
}

static std::string ArgFallbackName(const llvm::Argument& A, unsigned idx) {
  if (A.hasName())
    return A.getName().str();

  return "arg" + std::to_string(idx);
}


static std::string SanitizeVarName(std::string s) {
  for (char& c : s) {
    unsigned char uc = static_cast<unsigned char>(c);
    if (!(std::isalnum(uc) || c == '_'))
      c = '_';
  }
  return s;
}

} // namespace

int main(int argc, char** argv) {
  llvm::cl::ParseCommandLineOptions(argc, argv, "MGVM kernel analyzer (SCEV)\n");

  llvm::LLVMContext ctx;
  llvm::SMDiagnostic err;

  std::unique_ptr<llvm::Module> mod = llvm::parseIRFile(InputPath, err, ctx);
  if (!mod) {
    llvm::errs() << "Failed to parse IR: " << InputPath << "\n";
    err.print(argv[0], llvm::errs());
    return 1;
  }

  const llvm::DataLayout& DL = mod->getDataLayout();

  llvm::json::Object top;
  top["tool"] = "mgvm-kernel-analyzer";
  top["version"] = 2;
  top["input"] = InputPath;

  llvm::json::Array kernelsArr;

  for (llvm::Function& F : *mod) {
    if (!IsKernelFunction(F))
      continue;

    // Build arg-index map and derive arg names (OpenCL metadata if available).
    std::unordered_map<const llvm::Argument*, unsigned> argIndex;
    auto mdArgNames = ExtractKernelArgNamesFromOpenCLMetadata(*mod, F);

    unsigned idx = 0;
    for (llvm::Argument& A : F.args()) {
      argIndex[&A] = idx++;
    }

    llvm::json::Object kobj;
    kobj["name"] = F.getName().str();

    llvm::json::Array argsArr;
    idx = 0;
    for (llvm::Argument& A : F.args()) {
      llvm::json::Object aobj;
      aobj["index"] = static_cast<int64_t>(idx);

      std::string aname = ArgFallbackName(A, idx);
      auto it = mdArgNames.find(idx);
      if (it != mdArgNames.end() && !it->second.empty())
        aname = it->second;

      aobj["name"] = aname;

      bool isPtr = A.getType()->isPointerTy();
      aobj["is_pointer"] = isPtr;

      int64_t addrSpace = -1;
      if (isPtr) {
        auto* PT = llvm::cast<llvm::PointerType>(A.getType());
        addrSpace = static_cast<int64_t>(PT->getAddressSpace());
      }
      aobj["addrspace"] = addrSpace;

      std::string typeStr;
      llvm::raw_string_ostream ts(typeStr);
      A.getType()->print(ts);
      ts.flush();
      aobj["type"] = typeStr;

      // elem_bytes is only meaningful for pointers; we leave -1 if unknown.
      aobj["elem_bytes"] = -1;

      argsArr.push_back(std::move(aobj));
      idx++;
    }
    kobj["args"] = std::move(argsArr);

    // Pre-scan: record SSA values representing thread/workgroup IDs.
    mgvm::VarMap vars;

    // Also expose scalar kernel arguments as symbolic parameters so SCEV linearization
    // can attach coefficients to them. We do NOT map pointer args (they are bases).
    idx = 0;
    for (llvm::Argument& A : F.args()) {
      bool isPtr = A.getType()->isPointerTy();

      std::string aname = ArgFallbackName(A, idx);
      auto it = mdArgNames.find(idx);
      if (it != mdArgNames.end() && !it->second.empty())
        aname = it->second;

      if (!isPtr) {
        vars[&A] = "param_" + SanitizeVarName(aname);
      }
      idx++;
    }

    for (llvm::BasicBlock& BB : F) {
      for (llvm::Instruction& I : BB) {
        if (auto* CB = llvm::dyn_cast<llvm::CallBase>(&I)) {
          auto v = IdentifyThreadVar(*CB);
          if (v) {
            vars[CB] = *v;
          }
        }
      }
    }

    // Prepare analysis objects for ScalarEvolution.
    llvm::DominatorTree DT(F);
    llvm::LoopInfo LI(DT);
    llvm::AssumptionCache AC(F);

    llvm::Triple triple(mod->getTargetTriple());
    llvm::TargetLibraryInfoImpl TLII(triple);
    llvm::TargetLibraryInfo TLI(TLII);

    llvm::ScalarEvolution SE(F, TLI, AC, DT, LI);

    // Walk instructions and collect mem accesses.
    llvm::json::Array accessArr;

    int64_t numLoads = 0;
    int64_t numStores = 0;
    int64_t numGlobalLoads = 0;
    int64_t numGlobalStores = 0;

    unsigned accessCount = 0;

    auto RecordAccess = [&](StringRef kind, llvm::Value* ptr, llvm::Type* elemTy,
                            llvm::Instruction& inst) -> bool {
      if (accessCount >= MaxAccessesPerKernel) {
        return false;
      }

      accessCount++;

      llvm::json::Object a;

      a["kind"] = kind.str();

      int64_t elemBytes = -1;
      if (elemTy) {
        elemBytes = static_cast<int64_t>(DL.getTypeStoreSize(elemTy));
      }
      a["elem_bytes"] = elemBytes;

      // Determine addrspace
      int64_t addrSpace = -1;
      if (ptr->getType()->isPointerTy()) {
        addrSpace = static_cast<int64_t>(
            llvm::cast<llvm::PointerType>(ptr->getType())->getAddressSpace());
      }
      a["addrspace"] = addrSpace;

      bool isGlobal = (addrSpace == 1); // AMDGPU global
      a["is_global"] = isGlobal;

      // Map to kernel argument if possible
      std::string bufName = "unknown";
      int64_t bufArgIndex = -1;

      llvm::Value* base = llvm::GetUnderlyingObject(ptr, DL);
      if (auto* A = llvm::dyn_cast<llvm::Argument>(base)) {
        auto it = argIndex.find(A);
        if (it != argIndex.end()) {
          bufArgIndex = static_cast<int64_t>(it->second);

          // Use metadata-derived arg name if available
          auto it2 = mdArgNames.find(it->second);
          if (it2 != mdArgNames.end() && !it2->second.empty()) {
            bufName = it2->second;
          } else if (A->hasName()) {
            bufName = A->getName().str();
          } else {
            bufName = "arg" + std::to_string(it->second);
          }
        }
      } else if (auto* GV = llvm::dyn_cast<llvm::GlobalVariable>(base)) {
        bufName = GV->getName().str();
      }

      a["buffer"] = bufName;
      a["buffer_arg_index"] = bufArgIndex;

      // SCEV for pointer. Keep the full pointer SCEV for debugging, but attempt
      // to linearize an offset relative to the underlying base object so the
      // exported coeffs/constant are buffer-relative rather than base-pointer-relative.
      const llvm::SCEV* scev = SE.getSCEV(ptr);
      a["addr_scev"] = SCEVToString(scev);

      const llvm::SCEV* analysisScev = scev;
      if (base && base != ptr) {
        const llvm::SCEV* baseScev = SE.getSCEV(base);
        if (baseScev && baseScev->getType() == scev->getType()) {
          const llvm::SCEV* off = SE.getMinusSCEV(scev, baseScev);
          analysisScev = off;
          a["addr_offset_scev"] = SCEVToString(off);
        }
      }

      // If this is an AddRec (loop recurrence), also emit structured fields
      const llvm::SCEVAddRecExpr* addrec = llvm::dyn_cast<llvm::SCEVAddRecExpr>(analysisScev);
      mgvm::LinearExpr lin;
      if (addrec) {
        llvm::json::Object arObj;
        const llvm::SCEV* start = addrec->getStart();
        const llvm::SCEV* step = addrec->getStepRecurrence(SE);
        arObj["start_scev"] = SCEVToString(start);
        arObj["step_scev"] = SCEVToString(step);
      
        const llvm::Loop* L = addrec->getLoop();
        if (L && L->getHeader()) {
          arObj["loop_header"] = L->getHeader()->getName().str();
          arObj["loop_depth"] = static_cast<int64_t>(L->getLoopDepth());
        }
        arObj["is_affine"] = addrec->isAffine();
      
        // Trip count information (may be unknown)
        if (L) {
          const llvm::SCEV* btc = SE.getBackedgeTakenCount(L);
          arObj["backedge_taken_count_scev"] = SCEVToString(btc);
      
          int64_t tripConst = -1;
          unsigned tc = SE.getSmallConstantTripCount(L);
          if (tc != 0) {
            tripConst = static_cast<int64_t>(tc);
          }
          arObj["trip_count_const"] = tripConst;
      
          if (!llvm::isa<llvm::SCEVCouldNotCompute>(btc)) {
            const llvm::SCEV* one = SE.getOne(btc->getType());
            const llvm::SCEV* trip = SE.getAddExpr(btc, one);
            arObj["trip_count_scev"] = SCEVToString(trip);
          }
        }
      
        // Linearize start/step separately (planner-friendly)
        mgvm::LinearExpr startLin = mgvm::ExtractLinearExpr(start, vars);
        mgvm::LinearExpr stepLin = mgvm::ExtractLinearExpr(step, vars);
        arObj["start_linear"] = LinearExprToJson(startLin);
        arObj["step_linear"] = LinearExprToJson(stepLin);
      
        a["addrec"] = std::move(arObj);
      
        // For the top-level (coeffs/constant/unknown_terms), use the base (start).
        lin = std::move(startLin);
        lin.complete = false; // recurrence makes the full expression non-linear over loop iters
      } else {
        // Best-effort linearization of buffer-relative SCEV when possible.
        lin = mgvm::ExtractLinearExpr(analysisScev, vars);
      }
      
      llvm::json::Object coeffObj;
      for (auto& [k, v] : lin.coeffs) {
        coeffObj[k] = static_cast<int64_t>(v);
      }
      a["coeffs"] = std::move(coeffObj);
      a["constant"] = lin.constant;
      a["complete_linear"] = lin.complete;

      llvm::json::Array unk;
      for (auto& t : lin.unknown_terms) {
        unk.push_back(t);
      }
      a["unknown_terms"] = std::move(unk);

      // Basic location info (for debugging)
      a["inst"] = inst.getOpcodeName();

      accessArr.push_back(std::move(a));

      return true;
    };

    int64_t recordedLoads = 0;
    int64_t recordedStores = 0;
    int64_t recordedGlobalLoads = 0;
    int64_t recordedGlobalStores = 0;

    for (llvm::BasicBlock& BB : F) {
      for (llvm::Instruction& I : BB) {
        if (auto* LI = llvm::dyn_cast<llvm::LoadInst>(&I)) {
          numLoads++;
          llvm::Value* ptr = LI->getPointerOperand();

          int64_t as = -1;
          if (ptr->getType()->isPointerTy()) {
            as = static_cast<int64_t>(
                llvm::cast<llvm::PointerType>(ptr->getType())->getAddressSpace());
          }
          if (as == 1)
            numGlobalLoads++;

          // By default, only record global (addrspace==1) accesses to keep
          // the output focused on data placement / remote access modeling.
          if (IncludeNonGlobal || as == 1) {
            if (RecordAccess("load", ptr, LI->getType(), I)) {
              recordedLoads++;
              if (as == 1)
                recordedGlobalLoads++;
            }
          }
        } else if (auto* SI = llvm::dyn_cast<llvm::StoreInst>(&I)) {
          numStores++;
          llvm::Value* ptr = SI->getPointerOperand();

          int64_t as = -1;
          if (ptr->getType()->isPointerTy()) {
            as = static_cast<int64_t>(
                llvm::cast<llvm::PointerType>(ptr->getType())->getAddressSpace());
          }
          if (as == 1)
            numGlobalStores++;

          if (IncludeNonGlobal || as == 1) {
            if (RecordAccess("store", ptr, SI->getValueOperand()->getType(), I)) {
              recordedStores++;
              if (as == 1)
                recordedGlobalStores++;
            }
          }
        }
      }
    }

    kobj["mem_accesses"] = std::move(accessArr);

    llvm::json::Object summary;
    summary["num_loads"] = numLoads;
    summary["num_stores"] = numStores;
    summary["num_global_loads"] = numGlobalLoads;
    summary["num_global_stores"] = numGlobalStores;
    summary["recorded_loads"] = recordedLoads;
    summary["recorded_stores"] = recordedStores;
    summary["recorded_global_loads"] = recordedGlobalLoads;
    summary["recorded_global_stores"] = recordedGlobalStores;
    summary["record_non_global"] = IncludeNonGlobal.getValue();
    summary["capped_access_records"] = static_cast<int64_t>(accessCount);
    summary["cap_max_access_records"] =
        static_cast<int64_t>(MaxAccessesPerKernel.getValue());

    kobj["summary"] = std::move(summary);

    kernelsArr.push_back(std::move(kobj));
  }

  top["kernels"] = std::move(kernelsArr);

  std::error_code ec;
  llvm::raw_fd_ostream os(OutputPath, ec, llvm::sys::fs::OF_Text);
  if (ec) {
    llvm::errs() << "Failed to open output: " << OutputPath << " error=" << ec.message()
                 << "\n";
    return 1;
  }

  os << llvm::formatv("{0:2}", llvm::json::Value(std::move(top))) << "\n";
  os.flush();

  return 0;
}
