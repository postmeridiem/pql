package cli

import "testing"

func TestCompletion_AllShells(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		t.Run(shell, func(t *testing.T) {
			code := Run([]string{"completion", shell})
			if code != 0 {
				t.Fatalf("pql completion %s: exit %d, want 0", shell, code)
			}
		})
	}
}

func TestCompletion_InvalidShell(t *testing.T) {
	code := Run([]string{"completion", "nushell"})
	if code != 64 {
		t.Fatalf("expected EX_USAGE (64), got %d", code)
	}
}
