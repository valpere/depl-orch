package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// Classification is the triage outcome for a failed pipeline stage.
type Classification int

const (
	Trivial    Classification = iota // cheap fixer can handle it (1-2 line fix)
	Complex                          // needs a strong model
	NotFixable                       // skip recovery, fail fast
)

func (c Classification) String() string {
	switch c {
	case Trivial:
		return "trivial"
	case Complex:
		return "complex"
	case NotFixable:
		return "not-fixable"
	default:
		return "unknown"
	}
}

// Classifier makes a single cheap LLM call to triage a stage failure before
// committing to a full agentic recovery loop.
type Classifier struct {
	Model model.ToolCallingChatModel
	Log   *slog.Logger
}

const classifySystem = `You are a triage assistant for a CI pipeline.
A Go stage has failed. Classify the failure as exactly one of:
  trivial     — off-by-one, typo, missing nil check, simple type mismatch; 1-2 line fix
  complex     — logic error requiring system understanding; fixable but costly
  not-fixable — data-dependent, flaky, external service, product decision needed

Reply with ONLY this JSON object and nothing else:
{"classification":"trivial|complex|not-fixable","reason":"<one sentence>"}`

// Classify inspects stage and its failure output and returns a routing decision.
// On any LLM or parse error it defaults to Complex (fail-safe: don't skip recovery).
func (c *Classifier) Classify(ctx context.Context, stage, output string) (Classification, error) {
	log := c.Log
	if log == nil {
		log = slog.Default()
	}

	prompt := fmt.Sprintf("Stage: %s\n\nFailure output:\n%s", stage, tailString([]byte(output), 2000))
	msgs := []*schema.Message{
		schema.SystemMessage(classifySystem),
		schema.UserMessage(prompt),
	}

	resp, err := c.Model.Generate(ctx, msgs)
	if err != nil {
		log.Warn("classifier: generate failed, defaulting to complex", "err", err)
		return Complex, nil
	}

	raw := strings.TrimSpace(resp.Content)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var result struct {
		Classification string `json:"classification"`
		Reason         string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		log.Warn("classifier: unparseable response, defaulting to complex", "raw", raw)
		return Complex, nil
	}

	switch result.Classification {
	case "trivial":
		log.Info("classifier: trivial", "stage", stage, "reason", result.Reason)
		return Trivial, nil
	case "not-fixable":
		log.Info("classifier: not-fixable", "stage", stage, "reason", result.Reason)
		return NotFixable, nil
	case "complex":
		log.Info("classifier: complex", "stage", stage, "reason", result.Reason)
		return Complex, nil
	default:
		log.Warn("classifier: unknown value, defaulting to complex", "class", result.Classification)
		return Complex, nil
	}
}
