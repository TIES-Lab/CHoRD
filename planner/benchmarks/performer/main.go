package main

import (
	"flag"
	"github.com/sarchlab/mgpusim/v4/amd/benchmarks/llm/performer"
	"github.com/sarchlab/mgpusim/v4/amd/samples/runner"
)

var (
	seqFlag        = flag.Int("seq", 256, "Sequence length.")
	dModelFlag     = flag.Int("dmodel", 128, "Hidden size.")
	headsFlag      = flag.Int("heads", 8, "Attention heads.")
	dffFlag        = flag.Int("dff", 256, "FFN hidden size.")
	layersFlag     = flag.Int("layers", 2, "Number of layers.")
	batchTotalFlag = flag.Int("batch-total", 1, "Total inference batches.")
	batchStartFlag = flag.Int("batch-start", 0, "Batch start.")
	batchCountFlag = flag.Int("batch-count", 0, "Batch count.")
	batchSeedFlag  = flag.Int64("batch-seed", 1, "Batch seed.")
	batchUpdateIn  = flag.Bool("batch-update-input", false, "Update input per batch.")
)

func main() {
	flag.Parse()
	r := new(runner.Runner).Init()
	b := performer.NewBenchmark(r.Driver())
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
