// Package rank provides ranking-quality metrics (NDCG@k, MRR, P@k)
// for evaluating the enrichment layer.
package rank

import "math"

// NDCG computes Normalized Discounted Cumulative Gain at k.
func NDCG(ranked, ideal []string, k int) float64 {
	dcg := dcgAt(ranked, ideal, k)
	idcg := dcgAt(ideal, ideal, k)
	if idcg == 0 {
		return 0
	}
	return dcg / idcg
}

func dcgAt(ranked, relevant []string, k int) float64 {
	relSet := make(map[string]bool, len(relevant))
	for _, r := range relevant {
		relSet[r] = true
	}
	var dcg float64
	for i := range k {
		if i >= len(ranked) {
			break
		}
		if relSet[ranked[i]] {
			dcg += 1.0 / math.Log2(float64(i+2))
		}
	}
	return dcg
}

// MRR computes Mean Reciprocal Rank — the reciprocal of the rank of
// the first relevant result.
func MRR(ranked, relevant []string) float64 {
	relSet := make(map[string]bool, len(relevant))
	for _, r := range relevant {
		relSet[r] = true
	}
	for i, path := range ranked {
		if relSet[path] {
			return 1.0 / float64(i+1)
		}
	}
	return 0
}

// PrecisionAtK computes the fraction of the top-k results that are relevant.
func PrecisionAtK(ranked, relevant []string, k int) float64 {
	relSet := make(map[string]bool, len(relevant))
	for _, r := range relevant {
		relSet[r] = true
	}
	hits := 0
	for i := range k {
		if i >= len(ranked) {
			break
		}
		if relSet[ranked[i]] {
			hits++
		}
	}
	if k == 0 {
		return 0
	}
	return float64(hits) / float64(k)
}
