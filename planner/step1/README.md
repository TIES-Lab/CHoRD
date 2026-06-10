# Checkpoint1 Step1: LLVM IR Kernel Analyzer (Static, SCEV-based)

This tool performs **static analysis** on GPU kernel LLVM IR/bitcode and exports a `kernels.json`
containing per-kernel/per-buffer memory-access features.

It is designed for your chiplet MGPUSim workflow:

- You (ROCm 3.8) compile OpenCL kernels to **LLVM bitcode/IR** (intermediate artifact).
- Run this analyzer to generate `kernels.json`.
- Later (Step2/Step3) a planner consumes `kernels.json + host manifest + hw config` and outputs `plan.json`.

## What it extracts (v1)
For each kernel function (AMDGPU/OpenCL kernel):

- Kernel name
- Kernel args (index/name/address-space/type)
- Global memory accesses:
  - load/store
  - which kernel arg buffer it belongs to
  - element bytes
  - a best-effort **SCEV** string for the address
  - a best-effort **linear coefficients** over (wg_x,wg_y,wg_z,lid_x,lid_y,lid_z) when recognizable
- Summary counters: #loads/#stores/#global_loads/#global_stores

> Note: This is a **feature extractor**. It does not try to fully reproduce LASP/LADM.
> The planner can decide how to use these raw features.

## Build (CMake + LLVM)
You need an LLVM install that provides `LLVMConfig.cmake` and libraries.

Typical ROCm machines already have LLVM.
If not, you can build against system LLVM (matching `opt/clang` you use to produce the IR).

```bash
mkdir -p build
cd build
cmake -DLLVM_DIR=/path/to/lib/cmake/llvm ..
make -j
```

Output binary:
- `build/mgvm-kernel-analyzer`

If `LLVM_DIR` is not set, CMake will try to find LLVM automatically.

## Usage

### 1) Compile OpenCL to LLVM bitcode (example)
The exact flags depend on your ROCm/OpenCL compilation pipeline.
A reasonable starting point (amdgcn target) is:

```bash
clang -x cl -cl-std=CL2.0 -O2 -emit-llvm -c \
  --target=amdgcn-amd-amdhsa -mcpu=gfx906 \
  -o kernels.bc kernels.cl
```

### 2) Analyze
```bash
./mgvm-kernel-analyzer --input kernels.bc --output kernels.json
```

### 3) Optional: canonicalize IR before analysis
ScalarEvolution works better on simplified IR.

```bash
opt -O2 -S kernels.bc -o kernels.opt.ll
./mgvm-kernel-analyzer --input kernels.opt.ll --output kernels.json
```

## Notes / limitations
- HSACO itself is an ELF code object; this tool analyzes LLVM IR/bitcode **before** final codegen.
- Coefficient extraction is best-effort. If SCEV becomes too complex (div/mod/non-affine),
  it will still dump the SCEV string but coefficients may be incomplete.

## Output schema (kernels.json) (v1)
Top-level:
- `tool`, `version`, `input`, `kernels` (array)

Each kernel:
- `name`
- `args`: array of `{index,name,is_pointer,addrspace,type,elem_bytes}`
- `mem_accesses`: array of access records
- `summary`

See `src/main.cpp` for exact fields.

