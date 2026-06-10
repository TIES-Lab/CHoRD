package runner

import (
	"fmt"
	"os"
	"sync"
)

// csvCollector collects metrics and dumps a MGVM-compatible CSV file.
//
// The MGVM artifact uses a simple CSV format with 4 columns:
//   index, where, what, value
//
// We keep the same format so existing analysis scripts can be reused.
type csvCollector struct {
	mu      sync.Mutex
	metrics []csvMetric
}

type csvMetric struct {
	where string
	what  string
	value float64
}

func newCSVCollector() *csvCollector {
	return &csvCollector{}
}

func (c *csvCollector) Collect(where, what string, value float64) {
	c.mu.Lock()
	c.metrics = append(c.metrics, csvMetric{where: where, what: what, value: value})
	c.mu.Unlock()
}

// Dump writes "<baseName>.csv".
func (c *csvCollector) Dump(baseName string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	f, err := os.Create(baseName + ".csv")
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := fmt.Fprintf(f, ", where, what, value\n"); err != nil {
		return err
	}

	for i, m := range c.metrics {
		// Keep the MGVM style (no special CSV escaping). Component names and metric
		// names in MGPUSim typically contain no commas.
		if _, err := fmt.Fprintf(f, "%d, %s, %s, %.12f\n", i, m.where, m.what, m.value); err != nil {
			return err
		}
	}

	return nil
}
