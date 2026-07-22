package chat

import (
	"sort"

	"archadilm/internal/domain/provider"
)

// rrfConstant is the standard RRF constant (k=60 from the original paper:
// "Reciprocal Rank Fusion outperforms Condorcet and individual Rank
// Learning Methods", Cormack et al. 2009).
const rrfConstant = 60

// reciprocalRankFusion merges multiple ranked result sets into a single
// list using the RRF formula:
//
//	RRF_score(d) = Σ 1 / (k + rank_i(d))
//
// where rank_i(d) is the 1-based rank of document d in result set i.
// Documents that appear in multiple result sets accumulate higher scores.
//
// Returns a single merged slice sorted by descending RRF score, capped
// at topK results.
func reciprocalRankFusion(resultSets [][]provider.VectorSearchResult, topK int) []provider.VectorSearchResult {
	scores := make(map[string]float64)

	for _, results := range resultSets {
		for rank, r := range results {
			// rank is 0-based from the slice index; RRF uses 1-based ranks
			scores[r.ChunkID] += 1.0 / float64(rrfConstant+rank+1)
		}
	}

	// Collect unique chunk IDs with their aggregated RRF scores
	type scored struct {
		chunkID string
		score   float64
	}
	items := make([]scored, 0, len(scores))
	for id, s := range scores {
		items = append(items, scored{chunkID: id, score: s})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].score > items[j].score
	})

	if len(items) > topK {
		items = items[:topK]
	}

	merged := make([]provider.VectorSearchResult, len(items))
	for i, item := range items {
		merged[i] = provider.VectorSearchResult{
			ChunkID: item.chunkID,
			Score:   item.score,
		}
	}
	return merged
}
