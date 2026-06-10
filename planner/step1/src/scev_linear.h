#pragma once

#include <cstdint>
#include <map>
#include <string>
#include <vector>

namespace llvm {
class SCEV;
class DataLayout;
class Value;
}

namespace mgvm {

struct LinearExpr {
  // coefficients in **bytes** (best-effort)
  std::map<std::string, int64_t> coeffs;
  int64_t constant = 0;

  // Any term we couldn't linearize (we keep them as strings for debugging).
  std::vector<std::string> unknown_terms;

  // If false, expression is partially parsed; coefficients/constant may be incomplete.
  bool complete = true;

  // Best-effort list of known thread / workgroup variables referenced anywhere in
  // the original SCEV, even when the expression is not fully linearizable.
  std::vector<std::string> var_refs;
};

// VarMap maps LLVM SSA Values (typically CallInsts for thread-id intrinsics)
// to human-friendly variable names (e.g., "lid_x", "wg_y").
using VarMap = std::map<const llvm::Value*, std::string>;

// Extract a best-effort linear expression: const + Σ coeff[var]*var + unknown_terms
LinearExpr ExtractLinearExpr(const llvm::SCEV* scev,
                             const VarMap& vars);

} // namespace mgvm
