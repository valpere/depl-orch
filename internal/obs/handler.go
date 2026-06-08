package obs

import (
	"context"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
)

// NewCallbackHandler returns an Eino callback handler that records token usage
// into m. inputCostPer1M and outputCostPer1M are USD per 1M tokens; pass 0 to
// disable cost tracking. Register the returned handler once via
// callbacks.AppendGlobalHandlers before running the pipeline.
func NewCallbackHandler(m *Metrics, inputCostPer1M, outputCostPer1M float64) callbacks.Handler {
	return callbacks.NewHandlerBuilder().
		OnEndFn(func(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
			co := model.ConvCallbackOutput(output)
			if co == nil || co.TokenUsage == nil {
				return ctx
			}
			step := info.Name
			if step == "" {
				step = "unknown"
			}
			prompt := float64(co.TokenUsage.PromptTokens)
			completion := float64(co.TokenUsage.CompletionTokens)
			m.agentTokens.WithLabelValues(step, "prompt").Add(prompt)
			m.agentTokens.WithLabelValues(step, "completion").Add(completion)
			if inputCostPer1M > 0 || outputCostPer1M > 0 {
				cost := prompt*inputCostPer1M/1e6 + completion*outputCostPer1M/1e6
				m.agentCost.WithLabelValues(step).Add(cost)
			}
			return ctx
		}).
		Build()
}
