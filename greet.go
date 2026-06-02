// Package deplorch is the root package of the depl-orch deployment orchestrator.
package deplorch

import "fmt"

// Greet returns a friendly greeting for name. When name is empty it greets an
// anonymous "stranger" instead.
func Greet(name string) string {
	if name == "" {
		return "Hello, stranger!"
	}
	return fmt.Sprintf("Hello, %s!", name)
}
