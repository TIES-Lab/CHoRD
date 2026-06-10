// NOTE: This tool is intended to build against AMD ROCm 3.8's LLVM (11.x).
// In LLVM 11, `ScalarEvolution.h` forward-declares most concrete `SCEV*`
// subclasses; their definitions live in `ScalarEvolutionExpressions.h`.
// If we don't include that header, `dyn_cast<SCEVConstant>(...)` etc. will
// fail to compile with "incomplete type" errors.

#include "scev_linear.h"

#include "llvm/Analysis/ScalarEvolution.h"
#include "llvm/Analysis/ScalarEvolutionExpressions.h"
#include "llvm/IR/Constants.h"
#include "llvm/IR/Instructions.h"
#include "llvm/Support/raw_ostream.h"

#include <optional>
#include <algorithm>

namespace mgvm {
namespace {

static std::string ToString(const llvm::SCEV* s) {
  std::string out;
  llvm::raw_string_ostream os(out);
  os << *s;
  os.flush();
  return os.str();
}

// In many pipelines, thread/workgroup IDs are immediately cast/extended.
// The ScalarEvolution expression may reference the casted SSA value (e.g., zext/sext),
// so we strip trivial cast chains to recover the original defining value.
static const llvm::Value* StripTrivialCasts(const llvm::Value* v) {
  const llvm::Value* cur = v;
  while (true) {
    const auto* inst = llvm::dyn_cast<llvm::Instruction>(cur);
    if (!inst)
      break;

    // Covers zext/sext/trunc/bitcast/addrspacecast/ptrtoint/inttoptr.
    if (llvm::isa<llvm::CastInst>(inst)) {
      cur = inst->getOperand(0);
      continue;
    }

    break;
  }
  return cur;
}

static std::optional<int64_t> ToInt64(const llvm::SCEV* s) {
  if (auto* c = llvm::dyn_cast<llvm::SCEVConstant>(s)) {
    // best-effort: clamp to signed 64
    // LLVM 11: SCEVConstant stores a ConstantInt.
    llvm::APInt ap = c->getValue()->getValue();
    if (ap.getBitWidth() > 64) {
      // try to sign-extend / truncate; if information is lost, still continue.
      ap = ap.sextOrTrunc(64);
    }
    return static_cast<int64_t>(ap.getSExtValue());
  }
  return std::nullopt;
}

static void AddVarRef(LinearExpr& e, const std::string& name) {
  if (std::find(e.var_refs.begin(), e.var_refs.end(), name) == e.var_refs.end()) {
    e.var_refs.push_back(name);
  }
}

static void Merge(LinearExpr& dst, const LinearExpr& src, int64_t scale = 1) {
  dst.constant += src.constant * scale;
  for (auto& [k, v] : src.coeffs) {
    dst.coeffs[k] += v * scale;
  }
  for (auto& t : src.unknown_terms) {
    dst.unknown_terms.push_back(t);
  }
  for (auto& v : src.var_refs) {
    AddVarRef(dst, v);
  }
  dst.complete = dst.complete && src.complete;
}

static LinearExpr Parse(const llvm::SCEV* s, const VarMap& vars);

static LinearExpr ParseUnknown(const llvm::SCEVUnknown* u, const VarMap& vars) {
  LinearExpr e;
  const llvm::Value* v = u->getValue();
  auto it = vars.find(v);
  if (it != vars.end()) {
    e.coeffs[it->second] = 1;
    AddVarRef(e, it->second);
    return e;
  }

  // Try to recover the original thread/workgroup ID producer through trivial casts.
  const llvm::Value* base = StripTrivialCasts(v);
  if (base != v) {
    auto it2 = vars.find(base);
    if (it2 != vars.end()) {
      e.coeffs[it2->second] = 1;
      AddVarRef(e, it2->second);
      return e;
    }
  }

  // Not a known variable; keep the printed term.
  // `u` is-a `SCEV`; record the printed term so the planner can still
  // reason about it (even if it's non-linear or opaque).
  e.unknown_terms.push_back(ToString(u));
  e.complete = false;
  return e;
}

static LinearExpr Parse(const llvm::SCEV* s, const VarMap& vars) {
  LinearExpr e;

  if (auto c = ToInt64(s)) {
    e.constant = *c;
    return e;
  }

  if (auto* u = llvm::dyn_cast<llvm::SCEVUnknown>(s)) {
    return ParseUnknown(u, vars);
  }

  if (auto* add = llvm::dyn_cast<llvm::SCEVAddExpr>(s)) {
    // Use getOperand/getNumOperands for LLVM 11 compatibility.
    for (unsigned i = 0; i < add->getNumOperands(); ++i) {
      Merge(e, Parse(add->getOperand(i), vars), 1);
    }
    return e;
  }

  if (auto* mul = llvm::dyn_cast<llvm::SCEVMulExpr>(s)) {
    // Try constant * expr
    if (mul->getNumOperands() == 2) {
      auto* a = mul->getOperand(0);
      auto* b = mul->getOperand(1);
      if (auto ca = ToInt64(a)) {
        Merge(e, Parse(b, vars), *ca);
        return e;
      }
      if (auto cb = ToInt64(b)) {
        Merge(e, Parse(a, vars), *cb);
        return e;
      }
    }

    // Fallback: keep the opaque term, but still preserve any thread/workgroup
    // variables referenced by its operands so downstream passes can do coarse
    // kernel classification even when the coefficient is symbolic.
    for (unsigned i = 0; i < mul->getNumOperands(); ++i) {
      Merge(e, Parse(mul->getOperand(i), vars), 0);
    }
    e.unknown_terms.push_back(ToString(s));
    e.complete = false;
    return e;
  }

  // Strip cast-like wrappers
  if (auto* tr = llvm::dyn_cast<llvm::SCEVTruncateExpr>(s)) {
    return Parse(tr->getOperand(), vars);
  }
  if (auto* zext = llvm::dyn_cast<llvm::SCEVZeroExtendExpr>(s)) {
    return Parse(zext->getOperand(), vars);
  }
  if (auto* sext = llvm::dyn_cast<llvm::SCEVSignExtendExpr>(s)) {
    return Parse(sext->getOperand(), vars);
  }

  // Anything else: keep as unknown term
  e.unknown_terms.push_back(ToString(s));
  e.complete = false;
  return e;
}

} // namespace

LinearExpr ExtractLinearExpr(const llvm::SCEV* scev, const VarMap& vars) {
  return Parse(scev, vars);
}

} // namespace mgvm
