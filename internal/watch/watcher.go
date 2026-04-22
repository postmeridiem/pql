// Package watch implements a filesystem watcher that keeps the pql index
// hot by reindexing on file changes. See docs/watching.md for the spec.
package watch

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/postmeridiem/pql/internal/config"
	"github.com/postmeridiem/pql/internal/diag"
	"github.com/postmeridiem/pql/internal/index"
	"github.com/postmeridiem/pql/internal/store"
)

const debounceWindow = 250 * time.Millisecond

var builtinExcludes = []string{".git", config.VaultStateDir}

// Run starts the watcher loop. It blocks until ctx is cancelled or a
// fatal error occurs. The caller is responsible for signal handling and
// PID file management.
func Run(ctx context.Context, cfg *config.Config, scope string) error {
	st, err := store.Open(ctx, cfg.DBPath)
	if err != nil {
		return fmt.Errorf("watch: open store: %w", err)
	}
	defer func() { _ = st.Close() }()

	if _, err := index.New(st, cfg).Run(ctx); err != nil {
		return fmt.Errorf("watch: initial index: %w", err)
	}
	diag.Warn("watch", "initial index complete")

	w, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("watch: create watcher: %w", err)
	}
	defer func() { _ = w.Close() }()

	if err := addRecursive(w, scope); err != nil {
		return fmt.Errorf("watch: add watches: %w", err)
	}
	diag.Warn("watch", fmt.Sprintf("watching %s", scope))

	timer := time.NewTimer(0)
	if !timer.Stop() {
		<-timer.C
	}
	pending := false

	for {
		select {
		case <-ctx.Done():
			if pending {
				reindex(ctx, st, cfg)
			}
			return nil

		case event, ok := <-w.Events:
			if !ok {
				return nil
			}
			if shouldIgnore(event.Name, scope) {
				continue
			}
			if event.Has(fsnotify.Create) {
				addIfDir(w, event.Name)
			}
			if !pending {
				timer.Reset(debounceWindow)
				pending = true
			}

		case err, ok := <-w.Errors:
			if !ok {
				return nil
			}
			diag.Warn("watch.fsnotify", err.Error())

		case <-timer.C:
			pending = false
			reindex(ctx, st, cfg)
		}
	}
}

func reindex(ctx context.Context, st *store.Store, cfg *config.Config) {
	stats, err := index.New(st, cfg).Run(ctx)
	if err != nil {
		diag.Error("watch.reindex", err.Error(), "")
		return
	}
	diag.Warn("watch.reindex", fmt.Sprintf("walked %d, indexed %d, removed %d",
		stats.Walked, stats.Indexed, stats.Removed))
}

func addRecursive(w *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			base := filepath.Base(path)
			for _, excl := range builtinExcludes {
				if base == excl {
					return filepath.SkipDir
				}
			}
			return w.Add(path)
		}
		return nil
	})
}

func addIfDir(w *fsnotify.Watcher, path string) {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return
	}
	_ = addRecursive(w, path)
}

func shouldIgnore(path, scope string) bool {
	rel, err := filepath.Rel(scope, path)
	if err != nil {
		return true
	}
	parts := strings.Split(rel, string(filepath.Separator))
	for _, p := range parts {
		for _, excl := range builtinExcludes {
			if p == excl {
				return true
			}
		}
		if strings.HasSuffix(p, "-wal") || strings.HasSuffix(p, "-shm") {
			return true
		}
	}
	return false
}
