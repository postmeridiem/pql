package cli

import "testing"

func TestRun_NoArgsReturnsUsage(t *testing.T) {
	got := Run(nil)
	if got != 64 {
		t.Fatalf("expected EX_USAGE (64), got %d", got)
	}
}

func TestRun_HelpReturnsOK(t *testing.T) {
	got := Run([]string{"--help"})
	if got != 0 {
		t.Fatalf("expected 0, got %d", got)
	}
}

func TestRun_VersionReturnsOK(t *testing.T) {
	got := Run([]string{"--version"})
	if got != 0 {
		t.Fatalf("expected 0, got %d", got)
	}
}

func TestRun_UnknownSubcommandReturnsUsage(t *testing.T) {
	got := Run([]string{"definitely-not-a-subcommand"})
	if got != 64 {
		t.Fatalf("expected EX_USAGE (64), got %d", got)
	}
}
