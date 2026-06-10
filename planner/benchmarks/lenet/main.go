package main

import (
	"flag"

	"github.com/sarchlab/mgpusim/v4/amd/benchmarks/cnn/lenet"
	"github.com/sarchlab/mgpusim/v4/amd/samples/runner"
)

var (
	inHFlag = flag.Int("h", 512, "Input height (H).")
	inWFlag = flag.Int("w", 512, "Input width (W).")
	cinFlag = flag.Int("cin", 1, "Input channels (Cin).")
	c1Flag  = flag.Int("c1", 4, "Conv1 output channels.")
	c2Flag  = flag.Int("c2", 8, "Conv2 output channels.")
	fc1Flag = flag.Int("fc1", 32, "FC1 output units.")
	clsFlag = flag.Int("classes", 10, "Number of output classes.")

	// Batch inference knobs (for offline two-stage: profile warmup -> replan -> run rest)
	batchTotalFlag = flag.Int("batch-total", 1, "Total number of inference batches (default 1).")
	batchStartFlag = flag.Int("batch-start", 0, "Batch start index (0-based).")
	batchCountFlag = flag.Int("batch-count", 0, "Number of batches to run (0 means batch-total-batch-start).")
	batchSeedFlag  = flag.Int64("batch-seed", 1, "RNG seed for per-batch input generation.")
	batchUpdateIn  = flag.Bool("batch-update-input", false, "Update input tensor per batch (deterministic).")
)

func main() {
	flag.Parse()

	r := new(runner.Runner).Init()

	b := lenet.NewBenchmark(r.Driver())
	b.InH = *inHFlag
	b.InW = *inWFlag
	b.Cin = *cinFlag
	b.Conv1Out = *c1Flag
	b.Conv2Out = *c2Flag
	b.FC1Out = *fc1Flag
	b.NumClass = *clsFlag

	// Batch options
	b.BatchTotal = *batchTotalFlag
	b.BatchStart = *batchStartFlag
	b.BatchCount = *batchCountFlag
	b.BatchSeed = *batchSeedFlag
	b.BatchUpdateInput = *batchUpdateIn

	r.AddBenchmark(b)
	r.Run()
}
