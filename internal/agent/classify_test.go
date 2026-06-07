package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// fakeErrorModel always returns an error from Generate — simulates network/timeout failure.
type fakeErrorModel struct{ err error }

func (m *fakeErrorModel) Generate(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	return nil, m.err
}
func (m *fakeErrorModel) Stream(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("stream not implemented in fake")
}
func (m *fakeErrorModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

func TestClassifier_Trivial(t *testing.T) {
	m := &fakeModel{responses: []*schema.Message{
		schema.AssistantMessage(`{"classification":"trivial","reason":"off-by-one"}`, nil),
	}}
	class, err := (&Classifier{Model: m}).Classify(context.Background(), "test", "FAIL: TestAdd")
	if err != nil {
		t.Fatal(err)
	}
	if class != Trivial {
		t.Errorf("got %v, want Trivial", class)
	}
}

func TestClassifier_Complex(t *testing.T) {
	m := &fakeModel{responses: []*schema.Message{
		schema.AssistantMessage(`{"classification":"complex","reason":"logic error"}`, nil),
	}}
	class, err := (&Classifier{Model: m}).Classify(context.Background(), "test", "FAIL: TestSort")
	if err != nil {
		t.Fatal(err)
	}
	if class != Complex {
		t.Errorf("got %v, want Complex", class)
	}
}

func TestClassifier_NotFixable(t *testing.T) {
	m := &fakeModel{responses: []*schema.Message{
		schema.AssistantMessage(`{"classification":"not-fixable","reason":"external service"}`, nil),
	}}
	class, err := (&Classifier{Model: m}).Classify(context.Background(), "test", "connection refused")
	if err != nil {
		t.Fatal(err)
	}
	if class != NotFixable {
		t.Errorf("got %v, want NotFixable", class)
	}
}

func TestClassifier_GarbageDefaultsToComplex(t *testing.T) {
	m := &fakeModel{responses: []*schema.Message{
		schema.AssistantMessage("this is not json", nil),
	}}
	class, err := (&Classifier{Model: m}).Classify(context.Background(), "test", "failure")
	if err != nil {
		t.Fatal(err)
	}
	if class != Complex {
		t.Errorf("garbage response should default to Complex, got %v", class)
	}
}

func TestClassifier_UnknownValueDefaultsToComplex(t *testing.T) {
	m := &fakeModel{responses: []*schema.Message{
		schema.AssistantMessage(`{"classification":"maybe","reason":"dunno"}`, nil),
	}}
	class, err := (&Classifier{Model: m}).Classify(context.Background(), "test", "failure")
	if err != nil {
		t.Fatal(err)
	}
	if class != Complex {
		t.Errorf("unknown class should default to Complex, got %v", class)
	}
}

func TestClassifier_GenerateError(t *testing.T) {
	m := &fakeErrorModel{err: errors.New("connection refused")}
	class, err := (&Classifier{Model: m}).Classify(context.Background(), "test", "failure")
	if err != nil {
		t.Fatal(err)
	}
	if class != Complex {
		t.Errorf("generate error should default to Complex, got %v", class)
	}
}

func TestClassifier_CodeFenceStripped(t *testing.T) {
	m := &fakeModel{responses: []*schema.Message{
		schema.AssistantMessage("```json\n{\"classification\":\"trivial\",\"reason\":\"typo\"}\n```", nil),
	}}
	class, _ := (&Classifier{Model: m}).Classify(context.Background(), "test", "FAIL")
	if class != Trivial {
		t.Errorf("code fence not stripped, got %v", class)
	}
}
