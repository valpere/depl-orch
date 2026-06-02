package pipeline

import (
	"context"
	"os/exec"
)

// Commander runs an external command in dir and returns its combined
// stdout+stderr. Stages depend on this interface (not os/exec directly) so unit
// tests can inject a fake and run without go or docker installed.
//
// Implementations MUST exec the binary directly with discrete args (as
// osCommander does) and MUST NOT route through a shell (`sh -c`, string
// concatenation). The agentic layer (M2+) will pass model-generated values
// through here; a shell wrapper would turn those into a command-injection vector.
type Commander interface {
	Run(ctx context.Context, dir, name string, args ...string) ([]byte, error)
}

// osCommander is the real implementation, shelling out via os/exec.
type osCommander struct{}

func (osCommander) Run(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	return cmd.CombinedOutput()
}

// DefaultCommander shells out to the real os/exec.
var DefaultCommander Commander = osCommander{}
