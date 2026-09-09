package connect

import (
	"testing"

	"github.com/postmeridiem/pql/internal/connect/signal"
)

// mapSignal scores candidates from a fixed table — the simplest way to
// manufacture the tied scores T-127 is about.
type mapSignal struct {
	name   string
	scores map[string]float64
}

func (s mapSignal) Name() string { return s.name }
func (s mapSignal) Score(_ *signal.Context, path string) (float64, error) {
	return s.scores[path], nil
}

// T-127: tied scores used to come out in whatever arrangement sort.Slice
// left them in — a function of candidate order, which is itself not
// guaranteed. The path tie-break makes the order total, so the same
// candidate set must rank identically regardless of the order it arrives in.
func TestRankTiedScoresOrderByPath(t *testing.T) {
	sig := mapSignal{name: "textual", scores: map[string]float64{
		"b.md": 1.0,
		"d.md": 0.0, // three-way tie at zero, common in link-sparse vaults
		"a.md": 0.0,
		"c.md": 0.0,
	}}
	weights := WeightProfile{"textual": 1.0}

	permutations := [][]string{
		{"b.md", "d.md", "a.md", "c.md"},
		{"c.md", "a.md", "d.md", "b.md"},
		{"d.md", "c.md", "b.md", "a.md"},
	}
	want := []string{"b.md", "a.md", "c.md", "d.md"}

	for _, candidates := range permutations {
		enriched, err := Rank(&signal.Context{}, candidates, []signal.Signal{sig}, weights)
		if err != nil {
			t.Fatalf("Rank(%v): %v", candidates, err)
		}
		got := make([]string, len(enriched))
		for i, e := range enriched {
			got[i] = e.Path
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("Rank(%v) order = %v, want %v", candidates, got, want)
				break
			}
		}
	}
}
