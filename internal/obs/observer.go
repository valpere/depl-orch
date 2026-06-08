package obs

import "time"

// PipelineObserver implements pipeline.StageObserver. Wire it into Runner.Observer
// to record per-stage duration and attempt counts.
type PipelineObserver struct {
	Metrics *Metrics
}

// ObserveStageAttempt records one stage attempt. status is "ok" on success,
// "failed" on error. Satisfies pipeline.StageObserver.
func (o *PipelineObserver) ObserveStageAttempt(stage string, elapsed time.Duration, err error) {
	status := "ok"
	if err != nil {
		status = "failed"
	}
	o.Metrics.stageDuration.WithLabelValues(stage, status).Observe(elapsed.Seconds())
	o.Metrics.stageAttempts.WithLabelValues(stage, status).Inc()
}
