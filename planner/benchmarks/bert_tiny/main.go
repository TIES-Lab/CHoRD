package main

import (
	"flag"
	"github.com/sarchlab/mgpusim/v4/amd/benchmarks/llm/bert_tiny"
	"github.com/sarchlab/mgpusim/v4/amd/samples/runner"
)

var (
	seqFlag        = flag.Int("seq", 64, "Sequence length (tokens).")
	dModelFlag     = flag.Int("dmodel", 128, "Hidden size (d_model).")
	headsFlag      = flag.Int("heads", 8, "Number of attention heads.")
	dffFlag        = flag.Int("dff", 512, "FFN hidden size.")
	layersFlag     = flag.Int("layers", 2, "Number of encoder layers.")
	classesFlag    = flag.Int("classes", 2, "Number of output classes.")
	batchTotalFlag = flag.Int("batch-total", 1, "Total number of inference batches.")
	batchStartFlag = flag.Int("batch-start", 0, "Batch start index.")
	batchCountFlag = flag.Int("batch-count", 0, "Number of batches to run (0 means total-start).")
	batchSeedFlag  = flag.Int64("batch-seed", 1, "RNG seed for per-batch input generation.")
	batchUpdateIn  = flag.Bool("batch-update-input", false, "Update input tensors per batch.")
)

func main() {
	flag.Parse()
	r := new(runner.Runner).Init()
	b := bert_tiny.NewBenchmark(r.Driver())
	b.SeqLen = *seqFlag
	b.DModel = *dModelFlag
	b.NumHeads = *headsFlag
	b.DFF = *dffFlag
	b.Layers = *layersFlag
	b.NumClass = *classesFlag
	b.BatchTotal = *batchTotalFlag
	b.BatchStart = *batchStartFlag
	b.BatchCount = *batchCountFlag
	b.BatchSeed = *batchSeedFlag
	b.BatchUpdateInput = *batchUpdateIn
	r.AddBenchmark(b)
	r.Run()
}
