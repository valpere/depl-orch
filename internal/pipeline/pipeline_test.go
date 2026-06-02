package pipeline

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// fakeCommander records every command and optionally fails when the joined
// "name args" contains failOn.
type fakeCommander struct {
	calls    []string
	failOn   string
	noOutput bool // return nil output (exercise the empty-output error path)
}

func (f *fakeCommander) Run(_ context.Context, _, name string, args ...string) ([]byte, error) {
	line := strings.TrimSpace(name + " " + strings.Join(args, " "))
	f.calls = append(f.calls, line)
	var out []byte
	if !f.noOutput {
		out = []byte("output of " + name)
	}
	if f.failOn != "" && strings.Contains(line, f.failOn) {
		return out, errors.New("boom")
	}
	return out, nil
}

func newState() *State {
	return NewState("/work", "Dockerfile", "docker.io/pereval/app:test", true)
}

func TestRunner_OrderAndSuccess(t *testing.T) {
	fc := &fakeCommander{}
	r := &Runner{Stages: DefaultStages(fc)}
	if err := r.Run(context.Background(), newState()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	want := []string{
		"go build ./...",
		"go test ./...",
		"docker build -f /work/Dockerfile -t docker.io/pereval/app:test /work",
		"docker push docker.io/pereval/app:test",
	}
	if len(fc.calls) != len(want) {
		t.Fatalf("got %d calls %v, want %d", len(fc.calls), fc.calls, len(want))
	}
	for i, w := range want {
		if fc.calls[i] != w {
			t.Errorf("call %d = %q, want %q", i, fc.calls[i], w)
		}
	}
}

func TestRunner_StopsOnFailure(t *testing.T) {
	fc := &fakeCommander{failOn: "go test"}
	r := &Runner{Stages: DefaultStages(fc)}
	err := r.Run(context.Background(), newState())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "test") {
		t.Errorf("error %q should name the failing stage", err)
	}
	// build + test ran; docker stages did not.
	if len(fc.calls) != 2 {
		t.Fatalf("expected 2 calls before stop, got %v", fc.calls)
	}
}

func TestDockerPush_SkippedWhenPushFalse(t *testing.T) {
	fc := &fakeCommander{}
	st := NewState("/work", "Dockerfile", "docker.io/pereval/app:test", false)
	r := &Runner{Stages: DefaultStages(fc)}
	if err := r.Run(context.Background(), st); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	for _, c := range fc.calls {
		if strings.HasPrefix(c, "docker push") {
			t.Errorf("docker push must be skipped when Push=false, calls: %v", fc.calls)
		}
	}
}

func TestRun_CapturesOutput(t *testing.T) {
	fc := &fakeCommander{}
	st := newState()
	r := &Runner{Stages: DefaultStages(fc)}
	if err := r.Run(context.Background(), st); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	for _, stage := range []string{"build", "test", "docker-build", "docker-push"} {
		if _, ok := st.Outputs[stage]; !ok {
			t.Errorf("State.Outputs missing captured output for stage %q", stage)
		}
	}
}

func TestRunCmd_ErrorWithEmptyOutput(t *testing.T) {
	fc := &fakeCommander{failOn: "go build", noOutput: true}
	r := &Runner{Stages: DefaultStages(fc)}
	err := r.Run(context.Background(), newState())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "build") {
		t.Errorf("error %q should name the failing stage", err)
	}
}

func TestDockerBuild_AbsoluteDockerfileNotJoined(t *testing.T) {
	fc := &fakeCommander{}
	st := NewState("/work", "/etc/Dockerfile.custom", "img:tag", false)
	r := &Runner{Stages: []Stage{dockerBuildStage{fc}}}
	if err := r.Run(context.Background(), st); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !strings.Contains(fc.calls[0], "-f /etc/Dockerfile.custom ") {
		t.Errorf("absolute Dockerfile should be used as-is, got %q", fc.calls[0])
	}
}
