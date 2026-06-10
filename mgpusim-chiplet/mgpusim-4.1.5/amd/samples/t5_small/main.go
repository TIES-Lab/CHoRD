package main

import (
	"flag"
	"github.com/sarchlab/mgpusim/v4/amd/benchmarks/llm/t5_small"
	"github.com/sarchlab/mgpusim/v4/amd/samples/runner"
)

var (
	encSeqFlag     = flag.Int("enc-seq", 128, "Encoder sequence length.")
	decSeqFlag     = flag.Int("dec-seq", 32, "Decoder sequence length.")
	dModelFlag     = flag.Int("dmodel", 128, "Hidden size.")
	headsFlag      = flag.Int("heads", 8, "Attention heads.")
	dffFlag        = flag.Int("dff", 256, "FFN hidden size.")
	layersFlag     = flag.Int("layers", 2, "Number of encoder/decoder layers.")
	vocabFlag      = flag.Int("vocab", 64, "Output vocabulary size.")
	batchTotalFlag = flag.Int("batch-total", 1, "Total inference batches.")
	batchStartFlag = flag.Int("batch-start", 0, "Batch start.")
	batchCountFlag = flag.Int("batch-count", 0, "Batch count.")
	batchSeedFlag  = flag.Int64("batch-seed", 1, "Batch seed.")
	batchUpdateIn  = flag.Bool("batch-update-input", false, "Update inputs per batch.")
)

func main() {
	flag.Parse()
	r := new(runner.Runner).Init()
	b := t5_small.NewBenchmark(r.Driver())
	b.EncSeq = *encSeqFlag
	b.DecSeq = *decSeqFlag
	b.DModel = *dModelFlag
	b.NumHeads = *headsFlag
	b.DFF = *dffFlag
	b.Layers = *layersFlag
	b.Vocab = *vocabFlag
	b.BatchTotal = *batchTotalFlag
	b.BatchStart = *batchStartFlag
	b.BatchCount = *batchCountFlag
	b.BatchSeed = *batchSeedFlag
	b.BatchUpdateInput = *batchUpdateIn
	r.AddBenchmark(b)
	r.Run()
}
