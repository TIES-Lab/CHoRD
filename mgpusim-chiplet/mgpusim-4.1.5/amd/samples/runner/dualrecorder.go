package runner

import "github.com/sarchlab/akita/v4/datarecording"

// dualDataRecorder wraps a DataRecorder and mirrors metric insertions into a
// csvCollector, so that we can dump MGVM-compatible CSV output without touching
// every InsertData call site.
type dualDataRecorder struct {
	inner       datarecording.DataRecorder
	csv         *csvCollector
	metricTable string
}

func newDualDataRecorder(
	inner datarecording.DataRecorder,
	csv *csvCollector,
	metricTable string,
) datarecording.DataRecorder {
	if csv == nil {
		return inner
	}
	return &dualDataRecorder{
		inner:       inner,
		csv:         csv,
		metricTable: metricTable,
	}
}

func (d *dualDataRecorder) CreateTable(tableName string, sample interface{}) {
	d.inner.CreateTable(tableName, sample)
}

func (d *dualDataRecorder) InsertData(tableName string, entry interface{}) {
	d.inner.InsertData(tableName, entry)

	if d.csv == nil || tableName != d.metricTable {
		return
	}

	switch m := entry.(type) {
	case metric:
		d.csv.Collect(m.Location, m.What, m.Value)
	case *metric:
		d.csv.Collect(m.Location, m.What, m.Value)
	default:
		// Ignore non-metric tables.
	}
}

func (d *dualDataRecorder) ListTables() []string {
	return d.inner.ListTables()
}

func (d *dualDataRecorder) Flush() {
	d.inner.Flush()
}

func (d *dualDataRecorder) Close() error {
	return d.inner.Close()
}
