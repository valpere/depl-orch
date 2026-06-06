package agent

import (
	"context"
	"errors"
	"fmt"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// errLoopExhausted means the model kept calling tools past the iteration budget
// without a clean stop. It is non-fatal on its own: the caller should still
// verify the result (the model may have applied a working fix but stayed chatty).
var errLoopExhausted = errors.New("tool loop exhausted")

// runToolLoop drives a bounded generate -> dispatch-tool-calls -> append loop. It
// binds the tools to the model, then repeatedly Generates: each assistant turn
// that requests tool calls is executed and the results fed back. It stops when
// the model returns no tool calls (done) or maxIter is reached (exhausted ->
// error). Control flow here is ordinary Go — the LLM only fills in tool calls.
func runToolLoop(ctx context.Context, m model.ToolCallingChatModel, tools []tool.BaseTool, msgs []*schema.Message, maxIter int) ([]*schema.Message, error) {
	infos := make([]*schema.ToolInfo, 0, len(tools))
	invokable := make(map[string]tool.InvokableTool, len(tools))
	for _, t := range tools {
		info, err := t.Info(ctx)
		if err != nil {
			return msgs, fmt.Errorf("tool info: %w", err)
		}
		infos = append(infos, info)
		if it, ok := t.(tool.InvokableTool); ok {
			invokable[info.Name] = it
		}
	}

	bound, err := m.WithTools(infos)
	if err != nil {
		return msgs, fmt.Errorf("bind tools: %w", err)
	}

	for range maxIter {
		resp, err := bound.Generate(ctx, msgs)
		if err != nil {
			return msgs, fmt.Errorf("generate: %w", err)
		}
		msgs = append(msgs, resp)
		if len(resp.ToolCalls) == 0 {
			return msgs, nil // model is done
		}
		for _, tc := range resp.ToolCalls {
			it, ok := invokable[tc.Function.Name]
			if !ok {
				msgs = append(msgs, schema.ToolMessage("error: unknown tool "+tc.Function.Name, tc.ID))
				continue
			}
			out, err := it.InvokableRun(ctx, tc.Function.Arguments)
			if err != nil {
				out = "error: " + err.Error()
			}
			msgs = append(msgs, schema.ToolMessage(out, tc.ID))
		}
	}
	return msgs, fmt.Errorf("%w after %d iterations", errLoopExhausted, maxIter)
}
