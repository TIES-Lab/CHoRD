package main

import (
	"flag"
	"github.com/sarchlab/mgpusim/v4/amd/benchmarks/llm/tinyllama"
	"github.com/sarchlab/mgpusim/v4/amd/samples/runner"
)

var (
	seqFlag        = flag.Int("seq", 256, "Sequence length.")
	dModelFlag     = flag.Int("dmodel", 128, "Hidden size.")
	headsFlag      = flag.Int("heads", 8, "Attention heads.")
	dffFlag        = flag.Int("dff", 352, "SwiGLU hidden size.")
	layersFlag     = flag.Int("layers", 4, "Number of decoder layers.")
	batchTotalFlag = flag.Int("batch-total", 1, "Total inference batches.")
	batchStartFlag = flag.Int("batch-start", 0, "Batch start index.")
	batchCountFlag = flag.Int("batch-count", 0, "Number of batches to run.")
	batchSeedFlag  = flag.Int64("batch-seed", 1, "Per-batch RNG seed.")
	batchUpdateIn  = flag.Bool("batch-update-input", false, "Update input tensor per batch.")
)

func main() {
	flag.Parse()
	r := new(runner.Runner).Init()
	b := tinyllama.NewBenchmark(r.Driver())
	b.SeqLen = *seqFlag
	b.DModel = *dModelFlag
	b.NumHeads = *headsFlag
	b.DFF = *dffFlag
	b.Layers = *layersFlag
	b.BatchTotal = *batchTotalFlag
	b.BatchStart = *batchStartFlag
	b.BatchCount = *batchCountFlag
	b.BatchSeed = *batchSeedFlag
	b.BatchUpdateInput = *batchUpdateIn
	r.AddBenchmark(b)
	r.Run()
}
