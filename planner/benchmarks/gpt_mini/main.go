package main

import (
	"flag"

	"github.com/sarchlab/mgpusim/v4/amd/benchmarks/llm/gpt_mini"
	"github.com/sarchlab/mgpusim/v4/amd/samples/runner"
)

var (
	seqFlag    = flag.Int("seq", 256, "Sequence length (tokens).")
	dModelFlag = flag.Int("dmodel", 128, "Hidden size (d_model). Must be divisible by -heads.")
	headsFlag  = flag.Int("heads", 4, "Number of attention heads.")
	dffFlag    = flag.Int("dff", 256, "FFN hidden size (d_ff).")
	layersFlag = flag.Int("layers", 2, "Number of decoder layers.")

	batchTotalFlag = flag.Int("batch-total", 1, "Total number of inference batches.")
	batchStartFlag = flag.Int("batch-start", 0, "Batch start index.")
	batchCountFlag = flag.Int("batch-count", 0, "Number of batches to run (0 means total-start).")
	batchSeedFlag  = flag.Int64("batch-seed", 1, "RNG seed for per-batch input generation.")
	batchUpdateIn  = flag.Bool("batch-update-input", false, "Update input tensor per batch.")
)

func main() {
	flag.Parse()
	r := new(runner.Runner).Init()
	b := gpt_mini.NewBenchmark(r.Driver())
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
