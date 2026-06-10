package runner

// reportPTWSummary dumps global PTW/PWC counters into the MGVM-style metrics CSV.
func (r *reporter) reportPTWSummary() {
	if r.kernelStatsTracer == nil || r.kernelStatsTracer.ptwCounter == nil {
		return
	}

	s := r.kernelStatsTracer.ptwCounter.Snapshot()
	where := "PTW"

	// Counts
	r.dataRecorder.InsertData(tableName, metric{Location: where, What: "translation_reqs", Value: float64(s.translationReqCount), Unit: "count"})
	r.dataRecorder.InsertData(tableName, metric{Location: where, What: "walks_completed", Value: float64(s.walkCompletedCount), Unit: "count"})
	r.dataRecorder.InsertData(tableName, metric{Location: where, What: "walks_failed", Value: float64(s.walkFailedCount), Unit: "count"})

	// PWC
	r.dataRecorder.InsertData(tableName, metric{Location: where, What: "pwc_lookups", Value: float64(s.pwcLookupCount), Unit: "count"})
	r.dataRecorder.InsertData(tableName, metric{Location: where, What: "pwc_hits", Value: float64(s.pwcHitCount), Unit: "count"})
	r.dataRecorder.InsertData(tableName, metric{Location: where, What: "pwc_misses", Value: float64(s.pwcMissCount), Unit: "count"})
	r.dataRecorder.InsertData(tableName, metric{Location: where, What: "pwc_hit_level0", Value: float64(s.pwcHitLvl0), Unit: "count"})
	r.dataRecorder.InsertData(tableName, metric{Location: where, What: "pwc_hit_level1", Value: float64(s.pwcHitLvl1), Unit: "count"})
	r.dataRecorder.InsertData(tableName, metric{Location: where, What: "pwc_hit_level2", Value: float64(s.pwcHitLvl2), Unit: "count"})

	// Page-table reads
	r.dataRecorder.InsertData(tableName, metric{Location: where, What: "pte_reads", Value: float64(s.pteReadCount), Unit: "count"})
	r.dataRecorder.InsertData(tableName, metric{Location: where, What: "pte_reads_local", Value: float64(s.pteReadLocalCount), Unit: "count"})
	r.dataRecorder.InsertData(tableName, metric{Location: where, What: "pte_reads_remote", Value: float64(s.pteReadRemoteCount), Unit: "count"})

	// Latency (picoseconds)
	r.dataRecorder.InsertData(tableName, metric{Location: where, What: "walk_latency_sum_ps", Value: float64(s.walkLatencySumPS), Unit: "ps"})
	r.dataRecorder.InsertData(tableName, metric{Location: where, What: "walk_latency_count", Value: float64(s.walkLatencyCount), Unit: "count"})
	if s.walkLatencyCount > 0 {
		avgPS := float64(s.walkLatencySumPS) / float64(s.walkLatencyCount)
		r.dataRecorder.InsertData(tableName, metric{Location: where, What: "walk_latency_avg_ps", Value: avgPS, Unit: "ps"})
	}
}
