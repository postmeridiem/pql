package repo

import (
	"context"
	"path/filepath"
	"sync"
	"testing"

	"github.com/postmeridiem/pql/internal/planning"
)

// TestConcurrentCreate_NoBusy exercises the 1.10.2 concurrency fix against a
// real file-backed pql.db: many goroutines creating tickets at once must not
// trip SQLITE_BUSY (exit 69). With the single-connection cap the writes
// serialize on the busy_timeout-bearing connection; without it, writes on
// fresh pooled connections (busy_timeout=0) error under contention.
func TestConcurrentCreate_NoBusy(t *testing.T) {
	ctx := context.Background()
	d, err := planning.OpenPath(ctx, filepath.Join(t.TempDir(), "pql.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = d.Close() }()

	const n = 24
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := CreateTicket(ctx, d.SQL(), NewTicketOpts{Type: "task", Title: "concurrent"})
			errs <- e
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatalf("concurrent CreateTicket failed (db contention regressed): %v", e)
		}
	}
	tks, err := ListTickets(ctx, d.SQL(), TicketFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(tks) != n {
		t.Errorf("created %d tickets, want %d (lost writes under contention)", len(tks), n)
	}
}
