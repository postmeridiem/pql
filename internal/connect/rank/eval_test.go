//go:build eval

package rank

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/postmeridiem/pql/internal/config"
	"github.com/postmeridiem/pql/internal/index"
	intentctx "github.com/postmeridiem/pql/internal/intent/context"
	"github.com/postmeridiem/pql/internal/intent/related"
	"github.com/postmeridiem/pql/internal/intent/search"
	"github.com/postmeridiem/pql/internal/store"
)

type goldenCase struct {
	Query       string   `json:"query"`
	Intent      string   `json:"intent"`
	TargetPath  string   `json:"target_path"`
	ExpectedTopK []string `json:"expected_top_k"`
	K           int      `json:"k"`
	Notes       string   `json:"notes"`
}

func TestEval_Council(t *testing.T) {
	repoRoot := findRepoRoot(t)
	vaultPath := filepath.Join(repoRoot, "testdata", "council-snapshot")
	if _, err := os.Stat(vaultPath); err != nil {
		t.Skip("council-snapshot not available")
	}

	ctx := context.Background()
	cfg, err := config.Load(config.LoadOpts{VaultFlag: vaultPath})
	if err != nil {
		t.Fatalf("config: %v", err)
	}

	st, err := store.Open(ctx, cfg.DBPath)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	defer func() { _ = st.Close() }()

	if _, err := index.New(st, cfg).Run(ctx); err != nil {
		t.Fatalf("index: %v", err)
	}

	data, err := os.ReadFile("testdata/golden/council.json")
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	var cases []goldenCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatalf("parse golden: %v", err)
	}

	for _, tc := range cases {
		t.Run(tc.Query, func(t *testing.T) {
			ranked, err := runIntent(ctx, st, tc)
			if err != nil {
				t.Fatalf("run: %v", err)
			}

			paths := make([]string, len(ranked))
			for i, r := range ranked {
				paths[i] = r.Path
			}

			ndcg := NDCG(paths, tc.ExpectedTopK, tc.K)
			mrr := MRR(paths, tc.ExpectedTopK)
			pk := PrecisionAtK(paths, tc.ExpectedTopK, tc.K)

			t.Logf("NDCG@%d=%.3f  MRR=%.3f  P@%d=%.3f  (%s)",
				tc.K, ndcg, mrr, tc.K, pk, tc.Notes)

			if ndcg == 0 && len(tc.ExpectedTopK) > 0 {
				t.Errorf("NDCG@%d = 0 — no expected results in top-%d", tc.K, tc.K)
			}
		})
	}
}

func runIntent(ctx context.Context, st *store.Store, tc goldenCase) ([]struct{ Path string }, error) {
	type enriched struct {
		Path string
	}

	switch tc.Intent {
	case "related":
		results, err := related.Run(ctx, st.DB(), tc.TargetPath, tc.K)
		if err != nil {
			return nil, err
		}
		out := make([]struct{ Path string }, len(results))
		for i, r := range results {
			out[i].Path = r.Path
		}
		return out, nil
	case "search":
		q := tc.TargetPath
		if q == "" {
			parts := strings.SplitN(tc.Query, " ", 2)
			if len(parts) > 1 {
				q = parts[1]
			}
		}
		results, err := search.Run(ctx, st.DB(), q, tc.K)
		if err != nil {
			return nil, err
		}
		out := make([]struct{ Path string }, len(results))
		for i, r := range results {
			out[i].Path = r.Path
		}
		return out, nil
	case "context":
		results, err := intentctx.Run(ctx, st.DB(), tc.TargetPath, tc.K)
		if err != nil {
			return nil, err
		}
		out := make([]struct{ Path string }, len(results))
		for i, r := range results {
			out[i].Path = r.Path
		}
		return out, nil
	}
	return nil, nil
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Fatalf("find repo root: %v", err)
	}
	return strings.TrimSpace(string(out))
}
