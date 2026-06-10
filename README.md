# Chiplet-GPU Scheduling and Data Placement Artifact

This repository contains the simulator, planner, command templates, and collected results for a chiplet/MCM GPU scheduling and data placement project.

The artifact is organized so that a new user can download the code, build the analysis tools, generate plans for a benchmark, and run the simulator. The file `run_script.tcl` contains the original command blocks used in the experiments. It is a command reference, not a Tcl program that should be executed from top to bottom.

## 1. Directory layout

After unpacking the artifact, the top-level directory should look like this:

```text
submission_neurips/
  mgpusim-chiplet/     Modified MGPUSim-based chiplet/MCM GPU simulator.
  planner/             Step1/Step2 analyzers, Step3 planner, benchmark metadata, and plans.
  run_script.tcl       Command templates for reproducing the workflow.
  results_4chiplets/   Saved outputs for the 4-chiplet configuration.
  results_8chiplet/    Saved outputs for the 8-chiplet configuration.
```

The main code paths are:

```text
planner/step1/                         Kernel/IR analyzer.
planner/step2/                         Host launch and metadata analyzer.
planner/step3/                         Main planner: seed, static, and profiled modes.
planner/step3_clap/                    Standalone CLAP-style placement-only planner.
planner/benchmarks/<BENCH>/            Benchmark source and metadata for the main planner.
planner/benchmarks-clap/<BENCH>/       Benchmark metadata for the standalone CLAP baseline.
mgpusim-chiplet/mgpusim-4.1.5/amd/samples/<BENCH>/
                                       Simulator benchmark directory.
```

Available benchmarks include examples such as:

```text
bert_tiny, gpt_mini, lenet, mamba_tiny, performer,
resnet, switch_tiny, t5_small, tinyllama
```

## 2. One-time environment setup

Set paths for the artifact and ROCm/LLVM toolchain. Adjust the paths for your machine.

```bash
cd /path/to/submission_neurips

export ARTIFACT_ROOT=$PWD
export PLANNER=$ARTIFACT_ROOT/planner
export SIM=$ARTIFACT_ROOT/mgpusim-chiplet/mgpusim-4.1.5

export ROCM_PREFIX=/home/user/opt/llvm-rocm-3.8
export ROCM_PATH=$ROCM_PREFIX
export PATH="$ROCM_PREFIX/bin:$PATH"
export LD_LIBRARY_PATH="$ROCM_PREFIX/lib:${LD_LIBRARY_PATH:-}"
```

The commands below assume this directory structure. If you put the folders somewhere else, only the path variables need to change.

## 3. Build the planner tools

Build Step1, the kernel analyzer:

```bash
cd $PLANNER/step1
mkdir -p build
cd build
cmake .. -DLLVM_DIR="$($ROCM_PREFIX/bin/llvm-config --cmakedir)"
cmake --build . -j
```

Build Step2, the host analyzer:

```bash
cd $PLANNER/step2
go build -o mgvm-host-analyzer .
```

## 4. Build a simulator benchmark binary

Go to the simulator sample directory for the benchmark and build it if the executable is not already present.

```bash
BENCH=tinyllama
cd $SIM/amd/samples/$BENCH
go build -o $BENCH .
```

After this, the benchmark should be runnable as:

```bash
./$BENCH -help
```

## 5. Generate benchmark metadata

Run these commands inside the planner benchmark directory:

```bash
BENCH=tinyllama
cd $PLANNER/benchmarks/$BENCH
```

Compile the OpenCL kernels:

```bash
clang-ocl -mcpu=gfx803 -mno-code-object-v3 \
  --rocm-path="$ROCM_PREFIX" \
  -save-temps -v \
  kernels.cl -o kernels.hsaco 2>&1 | tee build.log
```

Run Step1:

```bash
../../step1/build/mgvm-kernel-analyzer \
  --input kernels.hsaco.orig.bc \
  --output kernels.json
```

Run Step2:

```bash
../../step2/mgvm-host-analyzer \
  --kernels-json kernels.json \
  --benchmark-go benchmark.go \
  --kernels-cl kernels.cl \
  --config kernels.config.json \
  --output global.${BENCH}.json
```

The important outputs are:

```text
kernels.json

global.<BENCH>.json
```

These files are already included for the provided benchmarks, but regenerate them if you modify `benchmark.go`, `kernels.cl`, `kernels.config.json`, launch shapes, or buffer sizes.

## 6. Main workflow: seed -> profile -> profiled plan -> final run

This is the main profile-guided planner workflow.

### 6.1 Generate the seed plan

Run inside `planner/benchmarks/<BENCH>/`:

```bash
BENCH=tinyllama
cd $PLANNER/benchmarks/$BENCH

python3 ../../step3/ilp_planner_profiled_seed_freeze.py \
  --kernels kernels.json \
  --global global.${BENCH}.json \
  --hardware-config ../../step3/hardware.config.json \
  --profile-seed-plan \
  --seed-placement rr \
  --debug-out ${BENCH}.seed.debug.json \
  --out ${BENCH}.seed.plan.json
```

The seed plan freezes the initial kernel-to-chiplet mapping. It does not perform kernel overlap or final CLAP-style placement.

### 6.2 Run the seed/profile simulation

Copy the seed plan into the simulator benchmark directory, then run the simulator:

```bash
cp $PLANNER/benchmarks/$BENCH/${BENCH}.seed.plan.json \
   $SIM/amd/samples/$BENCH/

cd $SIM/amd/samples/$BENCH

./${BENCH} -timing \
  -profile-json ${BENCH}.profile.json \
  -profile-group-pages 1 \
  -report-all -verify -report-wg-mapping \
  -platform-type privatetlb \
  -plan-json ${BENCH}.seed.plan.json \
  -num-chiplets-per-gpu 8 \
  -num-sa-per-chiplet 4 \
  -num-banks-per-chiplet 4 \
  -scheduling round-robin \
  -ptw-walkers-per-chiplet 16 \
  -pwc-entries 32 \
  -pwc-latency 10 \
  -batch-total 2 \
  -batch-start 0 \
  -batch-count 1 \
  -batch-update-input \
  -batch-seed 1
```

This produces runtime truth for the profiled planner:

```text
<BENCH>.profile.json
metrics.csv
metrics_kernels.csv
metrics_wgmap.csv
```

Copy the profile and kernel metrics back to the planner benchmark directory:

```bash
cp ${BENCH}.profile.json metrics_kernels.csv metrics_wgmap.csv \
   $PLANNER/benchmarks/$BENCH/
```

### 6.3 Generate the profiled plan

Run inside `planner/benchmarks/<BENCH>/`:

```bash
cd $PLANNER/benchmarks/$BENCH

python3 ../../step3/ilp_planner_profiled_seed_freeze.py \
  --kernels kernels.json \
  --global global.${BENCH}.json \
  --hardware-config ../../step3/hardware.config.json \
  --profile-json ${BENCH}.profile.json \
  --freeze-kernel-scheduling-from ${BENCH}.seed.plan.json \
  --emit-launch-hints \
  --launch-metrics-csv metrics_kernels.csv \
  --clap-primary-placement \
  --debug-out ${BENCH}.profiled.debug.json \
  --out ${BENCH}.profiled.plan.json
```

The profiled plan keeps the seed mapping frozen, then adds profile-guided launch hints, chiplet-preserving split, and CLAP-style explicit page placement.

### 6.4 Run the final profiled simulation

Copy the profiled plan to the simulator benchmark directory and run:

```bash
cp $PLANNER/benchmarks/$BENCH/${BENCH}.profiled.plan.json \
   $SIM/amd/samples/$BENCH/

cd $SIM/amd/samples/$BENCH

./${BENCH} -timing \
  -report-all -verify -report-wg-mapping \
  -platform-type privatetlb \
  -plan-json ${BENCH}.profiled.plan.json \
  -num-chiplets-per-gpu 8 \
  -num-sa-per-chiplet 4 \
  -num-banks-per-chiplet 4 \
  -scheduling round-robin \
  -ptw-walkers-per-chiplet 16 \
  -pwc-entries 32 \
  -pwc-latency 10 \
  -batch-total 2 \
  -batch-start 1 \
  -batch-count 1 \
  -batch-update-input \
  -batch-seed 1
```

This is the final run for the proposed profile-guided planner.

## 7. Static planner path

The static path does not use the seed runtime profile. It generates a profile-free static plan, then runs it in the simulator.

Generate the static plan:

```bash
BENCH=bert_tiny
cd $PLANNER/benchmarks/$BENCH

python3 ../../step3/ilp_planner_profiled_seed_freeze.py \
  --kernels kernels.json \
  --global global.${BENCH}.json \
  --hardware-config ../../step3/hardware.config.json \
  --emit-launch-hints \
  --debug-out ${BENCH}.static.debug.json \
  --out ${BENCH}.static.plan.json
```

Run the static plan:

```bash
cp $PLANNER/benchmarks/$BENCH/${BENCH}.static.plan.json \
   $SIM/amd/samples/$BENCH/

cd $SIM/amd/samples/$BENCH

./${BENCH} -timing \
  -report-all -verify -report-wg-mapping \
  -platform-type privatetlb \
  -plan-json ${BENCH}.static.plan.json \
  -num-chiplets-per-gpu 8 \
  -num-sa-per-chiplet 4 \
  -num-banks-per-chiplet 4 \
  -scheduling round-robin \
  -ptw-walkers-per-chiplet 16 \
  -pwc-entries 32 \
  -pwc-latency 10 \
  -batch-total 2 \
  -batch-start 1 \
  -batch-count 1 \
  -batch-update-input \
  -batch-seed 1
```

## 8. Standalone CLAP-style placement-only baseline

The standalone CLAP baseline keeps RR scheduling and only changes explicit page placement.

### 8.1 Run a plain RR profile

Run inside the simulator benchmark directory:

```bash
BENCH=switch_tiny
cd $SIM/amd/samples/$BENCH

./${BENCH} -timing \
  -profile-json ${BENCH}.rr.profile.json \
  -profile-group-pages 1 \
  -report-all -verify -report-wg-mapping \
  -platform-type privatetlb \
  -num-chiplets-per-gpu 8 \
  -num-sa-per-chiplet 4 \
  -num-banks-per-chiplet 4 \
  -scheduling round-robin \
  -ptw-walkers-per-chiplet 16 \
  -pwc-entries 32 \
  -pwc-latency 10 \
  -batch-total 1 \
  -batch-update-input \
  -batch-seed 1
```

Copy the RR profile to the CLAP benchmark directory:

```bash
cp ${BENCH}.rr.profile.json $PLANNER/benchmarks-clap/$BENCH/
```

### 8.2 Generate the CLAP placement plan

Run inside `planner/benchmarks-clap/<BENCH>/`:

```bash
BENCH=switch_tiny
cd $PLANNER/benchmarks-clap/$BENCH

python3 ../../step3_clap/clap_planner_current_sim_standalone.py \
  --profile-json ${BENCH}.rr.profile.json \
  --hardware-config ../../step3_clap/hardware.config.clap.current_sim.json \
  --global-json global.${BENCH}.json \
  --use-global-sizes \
  --out ${BENCH}.clap.plan.json \
  --debug-out ${BENCH}.clap.debug.json
```

### 8.3 Run the CLAP baseline

Copy the CLAP plan to the simulator benchmark directory and run:

```bash
cp $PLANNER/benchmarks-clap/$BENCH/${BENCH}.clap.plan.json \
   $SIM/amd/samples/$BENCH/

cd $SIM/amd/samples/$BENCH

./${BENCH} -timing \
  -report-all -verify -report-wg-mapping \
  -platform-type privatetlb \
  -plan-json ${BENCH}.clap.plan.json \
  -num-chiplets-per-gpu 8 \
  -num-sa-per-chiplet 4 \
  -num-banks-per-chiplet 4 \
  -scheduling round-robin \
  -ptw-walkers-per-chiplet 16 \
  -pwc-entries 32 \
  -pwc-latency 10 \
  -batch-total 1 \
  -batch-update-input \
  -batch-seed 1
```

## 9. Hardware configuration

The default command blocks use the 8-chiplet configuration:

```text
Planner config:  planner/step3/hardware.config.json
Simulator flags: -num-chiplets-per-gpu 8 -num-sa-per-chiplet 4 -num-banks-per-chiplet 4
```

For the 4-chiplet configuration, use the 4-chiplet hardware config files and update every simulator run consistently:

```text
Planner main config:  planner/step3/hardware.config.4chiplet_8sa_4cu.json
CLAP config:          planner/step3_clap/hardware.config.clap.current_sim.4chiplet_8sa_4cu.json
Simulator flags:      -num-chiplets-per-gpu 4 -num-sa-per-chiplet 8 -num-banks-per-chiplet 8
```

When changing hardware configuration, regenerate the seed plan, profile, profiled plan, static plan, RR profile, and CLAP plan. Do not mix 8-chiplet profiles/plans with 4-chiplet simulator runs.

## 10. Existing results

The saved output directories are provided for inspection:

```text
results_8chiplet/
results_4chiplets/
```

Typical files to inspect are:

```text
metrics.csv              Overall simulator metrics.
metrics_kernels.csv      Per-kernel latency, memory, translation, and launch metrics.
metrics_wgmap.csv        Workgroup-to-chiplet/CU mapping trace.
*.plan.json              Planner output.
*.debug.json             Planner debug information.
*.profile.json           Runtime page/chiplet profile.
```

## 11. Practical notes

Use `run_script.tcl` as a set of copyable command blocks. The `BENCH` variable in the file changes between examples, so set it explicitly before each block.

For a clean run of one benchmark, the safest order is:

```text
1. Build Step1 and Step2 tools.
2. Build the simulator benchmark binary.
3. Generate or confirm kernels.json and global.<BENCH>.json.
4. Generate seed plan.
5. Run seed/profile simulation.
6. Copy profile and metrics back to planner directory.
7. Generate profiled plan.
8. Run final profiled simulation.
9. Optionally run static and standalone CLAP baselines.
```

All simulator runs in this artifact include `-verify`. Each benchmark has a CPU reference check, so a successful run should not report verification errors.
