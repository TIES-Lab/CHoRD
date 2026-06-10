# compile build step1
cd ~/mgvm_planner/step1
cd build
export ROCM_PREFIX=/home/user/opt/llvm-rocm-3.8
cmake .. -DLLVM_DIR="$($ROCM_PREFIX/bin/llvm-config --cmakedir)"
cmake --build . -j

# build step2
#compile the tool of step2
go build -o mgvm-host-analyzer .


BENCH="tinyllama"
export ROCM_PREFIX=/home/user/opt/llvm-rocm-3.8
export ROCM_PATH=$ROCM_PREFIX
export PATH="$ROCM_PREFIX/bin:$PATH"
export LD_LIBRARY_PATH="$ROCM_PREFIX/lib:${LD_LIBRARY_PATH:-}"

# build hsaco
clang-ocl -mcpu=gfx803 -mno-code-object-v3 \
  --rocm-path="$ROCM_PREFIX" \
  -save-temps -v \
  kernels.cl -o kernels.hsaco 2>&1 | tee build.log
  
# run step1
../../step1/build/mgvm-kernel-analyzer \
  --input kernels.hsaco.orig.bc \
  --output kernels.json
  
BENCH="resnet"
# run step2
../../step2/mgvm-host-analyzer \
  --kernels-json kernels.json \
  --benchmark-go benchmark.go \
  --kernels-cl kernels.cl \
  --config kernels.config.json \
  --output global.${BENCH}.json



BENCH=bert_tiny
python3 ../../step3/ilp_planner_profiled_seed_freeze.py \
  --kernels kernels.json \
  --global global.${BENCH}.json \
  --hardware-config ../../step3/hardware.config.4chiplet_8sa_4cu.json\
  --profile-seed-plan \
  --seed-placement rr \
  --debug-out ${BENCH}.seed.debug.json \
  --out ${BENCH}.seed.plan.json
# hardware.config.json
# hardware.config.4chiplet_8sa_4cu.json

# run phase1
BENCH="bert_tiny"
./${BENCH} -timing \
  -profile-json ${BENCH}.profile.json \
  -profile-group-pages 1 \
  -report-all -verify -report-wg-mapping \
  -platform-type privatetlb \
  -plan-json ${BENCH}.seed.plan.json \
  -num-chiplets-per-gpu 4 \
  -num-sa-per-chiplet 8 \
  -num-banks-per-chiplet 8 \
  -scheduling round-robin \
  -ptw-walkers-per-chiplet 16 \
  -pwc-entries 32 \
  -pwc-latency 10 \
  -batch-total 2 \
  -batch-start 0 \
  -batch-count 1 \
  -batch-update-input \
  -batch-seed 1

BENCH=tinyllama
python3 ../../step3/ilp_planner_profiled_seed_freeze.py \
  --kernels kernels.json \
  --global global.${BENCH}.json \
  --hardware-config ../../step3/hardware.config.4chiplet_8sa_4cu.json \
  --profile-json ${BENCH}.profile.json \
  --freeze-kernel-scheduling-from ${BENCH}.seed.plan.json \
  --emit-launch-hints \
  --launch-metrics-csv metrics_kernels.csv \
  --clap-primary-placement \
  --debug-out ${BENCH}.profiled.debug.json \
  --out ${BENCH}.profiled.plan.json
# hardware.config.json
# hardware.config.4chiplet_8sa_4cu.json

# rerun with lenet plan
BENCH="t5_small"
./${BENCH} -timing \
  -report-all -verify -report-wg-mapping \
  -platform-type privatetlb \
  -plan-json ${BENCH}.profiled.plan.json \
  -num-chiplets-per-gpu 4 \
  -num-sa-per-chiplet 8 \
  -num-banks-per-chiplet 8 \
  -scheduling round-robin \
  -ptw-walkers-per-chiplet 16 \
  -pwc-entries 32 \
  -pwc-latency 10 \
  -batch-total 2 \
  -batch-start 1 \
  -batch-count 1 \
  -batch-update-input \
  -batch-seed 1

################################ static ####################################
BENCH="t5_small"
python3 ../../step3/ilp_planner_profiled_seed_freeze.py \
  --kernels kernels.json \
  --global global.${BENCH}.json \
  --hardware-config ../../step3/hardware.config.4chiplet_8sa_4cu.json \
  --emit-launch-hints \
  --debug-out ${BENCH}.static.debug.json \
  --out ${BENCH}.static.plan.json
# hardware.config.4chiplet_8sa_4cu.json
  
BENCH="t5_small"
./${BENCH} -timing \
  -report-all -verify -report-wg-mapping \
  -platform-type privatetlb \
  -plan-json ${BENCH}.static.plan.json \
  -num-chiplets-per-gpu 4 \
  -num-sa-per-chiplet 8 \
  -num-banks-per-chiplet 8 \
  -scheduling round-robin \
  -ptw-walkers-per-chiplet 16 \
  -pwc-entries 32 \
  -pwc-latency 10 \
  -batch-total 2 \
  -batch-start 1 \
  -batch-count 1 \
  -batch-update-input \
  -batch-seed 1

################################ clap ####################################
BENCH="t5_small"
./${BENCH} -timing \
  -profile-json ${BENCH}.rr.profile.json \
  -profile-group-pages 1 \
  -report-all -verify -report-wg-mapping \
  -platform-type privatetlb \
  -num-chiplets-per-gpu 4 \
  -num-sa-per-chiplet 8 \
  -num-banks-per-chiplet 8 \
  -scheduling round-robin \
  -ptw-walkers-per-chiplet 16 \
  -pwc-entries 32 \
  -pwc-latency 10 \
  -batch-total 1 \
  -batch-update-input \
  -batch-seed 1
  
BENCH="resnet"  
python3 ../../step3_clap/clap_planner_current_sim_standalone.py \
  --profile-json ${BENCH}.rr.profile.json \
  --hardware-config ../../step3_clap/hardware.config.clap.current_sim.4chiplet_8sa_4cu.json \
  --global-json global.${BENCH}.json \
  --use-global-sizes \
  --out ${BENCH}.clap.plan.json \
  --debug-out ${BENCH}.clap.debug.json
# hardware.config.clap.current_sim.4chiplet_8sa_4cu.json  
# hardware.config.clap.current_sim.json

BENCH="resnet"  
./${BENCH} -timing \
  -report-all -verify -report-wg-mapping \
  -platform-type privatetlb \
  -plan-json ${BENCH}.clap.plan.json \
  -num-chiplets-per-gpu 4 \
  -num-sa-per-chiplet 8 \
  -num-banks-per-chiplet 8 \
  -scheduling round-robin \
  -ptw-walkers-per-chiplet 16 \
  -pwc-entries 32 \
  -pwc-latency 10 \
  -batch-total 1 \
  -batch-update-input \
  -batch-seed 1
  


