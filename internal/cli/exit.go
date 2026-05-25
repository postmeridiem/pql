package cli

import "fmt"

// exitError carries a process exit code through cobra's error-returning
// path. Run() unwraps these via errors.As; cobra's own error printing is
// silenced (cmd.SilenceErrors), so RunE returning an exitError doesn't
// produce ceremony for expected non-zero exits like "zero matches".
//
// Subcommands construct one with the matching diag.* code. If msg is set,
// Run() emits it as a stderr JSON diagnostic before exiting.
type exitError struct {
	code int
	msg  string
	hint string
}

func (e *exitError) Error() string {
	if e.msg != "" {
		return e.msg
	}
	return fmt.Sprintf("pql: exit code %d", e.code)
}
