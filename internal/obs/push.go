package obs

import "github.com/prometheus/client_golang/prometheus/push"

// Push ships metrics to a Prometheus Pushgateway. It is a no-op when
// gatewayURL is empty so the deployer works without a Pushgateway in local
// or CI environments that don't need metrics.
func Push(m *Metrics, gatewayURL, jobLabel string) error {
	if gatewayURL == "" {
		return nil
	}
	if jobLabel == "" {
		jobLabel = "depl-orch"
	}
	return push.New(gatewayURL, jobLabel).
		Gatherer(m.reg).
		Push()
}
