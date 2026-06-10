package main

import (
	"flag"
	"github.com/sarchlab/mgpusim/v4/amd/benchmarks/cnn/resnet"
	"github.com/sarchlab/mgpusim/v4/amd/samples/runner"
)

var (
	inHFlag      = flag.Int("h", 128, "Input height.")
	inWFlag      = flag.Int("w", 128, "Input width.")
	cinFlag      = flag.Int("cin", 3, "Input channels.")
	cFlag        = flag.Int("c", 16, "Residual stage channels.")
	blocksFlag   = flag.Int("blocks", 3, "Number of repeated residual basic blocks.")
	clsFlag      = flag.Int("classes", 10, "Number of output classes.")
	gapTilesFlag = flag.Int("gap-tiles", 64, "Parallel tiles for GAP reduction.")

	batchTotalFlag = flag.Int("batch-total", 1, "Total number of inference batches.")
	batchStartFlag = flag.Int("batch-start", 0, "Batch start index.")
	batchCountFlag = flag.Int("batch-count", 0, "Number of batches to run (0 means total-start).")
	batchSeedFlag  = flag.Int64("batch-seed", 1, "RNG seed for per-batch input generation.")
	batchUpdateIn  = flag.Bool("batch-update-input", false, "Update input tensor per batch.")
)

func main() {
	flag.Parse()
	r := new(runner.Runner).Init()
	b := resnet.NewBenchmark(r.Driver())
	b.InH = *inHFlag
	b.InW = *inWFlag
	b.Cin = *cinFlag
	b.C = *cFlag
	b.Blocks = *blocksFlag
	b.NumClass = *clsFlag
	b.GapTiles = *gapTilesFlag
	b.BatchTotal = *batchTotalFlag
	b.BatchStart = *batchStartFlag
	b.BatchCount = *batchCountFlag
	b.BatchSeed = *batchSeedFlag
	b.BatchUpdateInput = *batchUpdateIn
	r.AddBenchmark(b)
	r.Run()
}
