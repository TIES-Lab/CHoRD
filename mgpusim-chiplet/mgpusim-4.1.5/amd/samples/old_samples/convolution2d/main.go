package main

import (
	"flag"

	"github.com/sarchlab/mgpusim/v4/amd/benchmarks/polybench/convolution2d"
	"github.com/sarchlab/mgpusim/v4/amd/samples/runner"
)

var niFlag = flag.Int("ni", 1024, "The I dimension (rows).")
var njFlag = flag.Int("nj", 1024, "The J dimension (cols).")

func main() {
	flag.Parse()

	runner := new(runner.Runner).Init()

	benchmark := convolution2d.NewBenchmark(runner.Driver())
	benchmark.NI = *niFlag
	benchmark.NJ = *njFlag

	runner.AddBenchmark(benchmark)
	runner.Run()
}
