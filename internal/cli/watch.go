package cli

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/postmeridiem/pql/internal/cli/render"
	"github.com/postmeridiem/pql/internal/config"
	"github.com/postmeridiem/pql/internal/diag"
	"github.com/postmeridiem/pql/internal/watch"
)

func newWatchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Filesystem watcher for live index updates",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = cmd.Help()
			return &exitError{code: diag.Usage}
		},
	}
	cmd.AddCommand(newWatchStartCmd())
	cmd.AddCommand(newWatchStopCmd())
	cmd.AddCommand(newWatchStatusCmd())
	return cmd
}

func newWatchStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start [path]",
		Short: "Start watching the vault (foreground)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(loadOptsFromFlags(cmd))
			if err != nil {
				return &exitError{code: diag.NoInput, msg: err.Error()}
			}

			scope := cfg.Vault.Path
			if len(args) > 0 {
				scope, err = filepath.Abs(args[0])
				if err != nil {
					return &exitError{code: diag.NoInput, msg: err.Error()}
				}
			}

			if existing := watch.ReadPIDFile(cfg.Vault.Path); existing != nil {
				return &exitError{
					code: diag.Usage,
					msg: fmt.Sprintf("a watcher is already running on %s (pid %d)",
						existing.Scope, existing.PID),
					hint: "run 'pql watch stop' first",
				}
			}

			if err := watch.WritePIDFile(cfg.Vault.Path, scope); err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			defer watch.RemovePIDFile(cfg.Vault.Path)

			ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			if err := watch.Run(ctx, cfg, scope); err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			return nil
		},
	}
}

func newWatchStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the active watcher",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(loadOptsFromFlags(cmd))
			if err != nil {
				return &exitError{code: diag.NoInput, msg: err.Error()}
			}

			info := watch.ReadPIDFile(cfg.Vault.Path)
			if info == nil {
				return &exitError{code: diag.Usage, msg: "no watcher running for this vault"}
			}

			proc, err := os.FindProcess(info.PID)
			if err != nil {
				watch.RemovePIDFile(cfg.Vault.Path)
				return &exitError{code: diag.Usage, msg: "no watcher running for this vault"}
			}

			if err := proc.Signal(syscall.SIGTERM); err != nil {
				watch.RemovePIDFile(cfg.Vault.Path)
				return &exitError{code: diag.Software, msg: fmt.Sprintf("signal pid %d: %v", info.PID, err)}
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "stopped (was watching %s)\n", info.Scope)
			return nil
		},
	}
}

func newWatchStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Report watcher state for this vault",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(loadOptsFromFlags(cmd))
			if err != nil {
				return &exitError{code: diag.NoInput, msg: err.Error()}
			}

			info := watch.ReadPIDFile(cfg.Vault.Path)
			rOpts, err := renderOptsFromFlags(cmd)
			if err != nil {
				return &exitError{code: diag.Usage, msg: err.Error()}
			}
			rOpts.Out = cmd.OutOrStdout()

			type statusResult struct {
				Running    bool   `json:"running"`
				PID        int    `json:"pid,omitempty"`
				Scope      string `json:"scope,omitempty"`
				StartedAt  string `json:"started_at,omitempty"`
				PQLVersion string `json:"pql_version,omitempty"`
			}

			if info == nil {
				if _, err := render.One(&statusResult{Running: false}, rOpts); err != nil {
					return &exitError{code: diag.Software, msg: err.Error()}
				}
				return nil
			}

			if _, err := render.One(&statusResult{
				Running:    true,
				PID:        info.PID,
				Scope:      info.Scope,
				StartedAt:  info.StartedAt,
				PQLVersion: info.PQLVersion,
			}, rOpts); err != nil {
				return &exitError{code: diag.Software, msg: err.Error()}
			}
			return nil
		},
	}
}
