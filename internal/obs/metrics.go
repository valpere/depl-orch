// Package obs provides Prometheus metrics for the depl-orch pipeline.
// It bridges the stdlib-only pipeline package with the Prometheus client via
// the pipeline.StageObserver interface.
package obs

import (
	dto "github.com/prometheus/client_model/go"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics holds all instrumentation for one pipeline run. Each instance uses
// its own registry so tests can create isolated instances without global state.
type Metrics struct {
	reg           *prometheus.Registry
	stageDuration *prometheus.HistogramVec
	stageAttempts *prometheus.CounterVec
	agentTokens   *prometheus.CounterVec
	agentCost     *prometheus.CounterVec
	runInfo       *prometheus.GaugeVec
}

// NewMetrics creates and registers all metrics on a fresh isolated registry.
func NewMetrics() *Metrics {
	reg := prometheus.NewRegistry()
	m := &Metrics{
		reg: reg,
		stageDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "depl_orch",
			Name:      "stage_duration_seconds",
			Help:      "Duration of each pipeline stage attempt in seconds.",
			// Custom buckets: pipeline stages range from sub-second (go build) to
			// many minutes (docker build, deploy). DefBuckets top out at 10s.
			Buckets: []float64{0.1, 0.5, 1, 5, 15, 30, 60, 120, 300, 600},
		}, []string{"stage", "status"}),
		stageAttempts: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "depl_orch",
			Name:      "stage_attempts_total",
			Help:      "Total pipeline stage attempts by stage and outcome.",
		}, []string{"stage", "status"}),
		agentTokens: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "depl_orch",
			Name:      "agent_tokens_total",
			Help:      "Total tokens consumed by agentic recovery steps.",
		}, []string{"step", "token_type"}),
		agentCost: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "depl_orch",
			Name:      "agent_cost_usd_total",
			Help:      "Estimated USD cost of agentic recovery steps.",
		}, []string{"step"}),
		runInfo: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "depl_orch",
			Name:      "run_info",
			Help:      "Metadata about the current pipeline run (always 1).",
		}, []string{"image", "target"}),
	}
	reg.MustRegister(
		m.stageDuration, m.stageAttempts,
		m.agentTokens, m.agentCost,
		m.runInfo,
	)
	return m
}

// SetRunInfo records a run_info gauge with value 1 labelled by image and deploy target.
func (m *Metrics) SetRunInfo(image, target string) {
	m.runInfo.WithLabelValues(image, target).Set(1)
}

// Gather collects all metrics from the registry. Used by tests and e2e
// verification to assert metrics were recorded.
func (m *Metrics) Gather() ([]*dto.MetricFamily, error) {
	return m.reg.Gather()
}
