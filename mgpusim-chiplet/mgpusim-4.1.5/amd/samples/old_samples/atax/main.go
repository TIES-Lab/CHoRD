package main

import (
	"flag"

	"github.com/sarchlab/mgpusim/v4/amd/benchmarks/polybench/atax"
	"github.com/sarchlab/mgpusim/v4/amd/samples/runner"
)

var xFlag = flag.Int("x", 4096, "The width of the matrix.")
var yFlag = flag.Int("y", 4096, "The height of the matrix.")

// localSizeXFlag controls the work-group size used when launching the kernels.
// MGVM artifact uses 64 for ATAX; MGPUSim upstream often uses larger values (e.g., 256).
var localSizeXFlag = flag.Int("local-size", 32, "The local work-group size in x dimension.")

func main() {
	flag.Parse()

	runner := new(runner.Runner).Init()

	benchmark := atax.NewBenchmark(runner.Driver())
	benchmark.NX = *xFlag
	benchmark.NY = *yFlag
	benchmark.LocalSizeX = *localSizeXFlag

	runner.AddBenchmark(benchmark)

	runner.Run()
}
