package rank

import (
	"math"
	"testing"
)

func TestNDCG_PerfectRanking(t *testing.T) {
	ideal := []string{"a", "b", "c"}
	got := NDCG(ideal, ideal, 3)
	if math.Abs(got-1.0) > 0.001 {
		t.Errorf("NDCG = %f, want 1.0", got)
	}
}

func TestNDCG_IrrelevantFirst(t *testing.T) {
	ranked := []string{"x", "y", "a"}
	ideal := []string{"a", "b"}
	got := NDCG(ranked, ideal, 3)
	if got >= 1.0 || got <= 0 {
		t.Errorf("NDCG = %f, want 0 < x < 1", got)
	}
}

func TestNDCG_NoRelevant(t *testing.T) {
	ranked := []string{"x", "y", "z"}
	ideal := []string{"a", "b"}
	got := NDCG(ranked, ideal, 3)
	if got != 0 {
		t.Errorf("NDCG = %f, want 0", got)
	}
}

func TestMRR_FirstIsRelevant(t *testing.T) {
	got := MRR([]string{"a", "b", "c"}, []string{"a"})
	if got != 1.0 {
		t.Errorf("MRR = %f, want 1.0", got)
	}
}

func TestMRR_ThirdIsRelevant(t *testing.T) {
	got := MRR([]string{"x", "y", "a"}, []string{"a"})
	want := 1.0 / 3.0
	if math.Abs(got-want) > 0.001 {
		t.Errorf("MRR = %f, want %f", got, want)
	}
}

func TestPrecisionAtK(t *testing.T) {
	ranked := []string{"a", "x", "b", "y", "c"}
	relevant := []string{"a", "b", "c"}
	got := PrecisionAtK(ranked, relevant, 5)
	want := 3.0 / 5.0
	if math.Abs(got-want) > 0.001 {
		t.Errorf("P@5 = %f, want %f", got, want)
	}
}
