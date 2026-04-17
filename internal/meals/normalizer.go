package meals

import (
	"math"
	"strings"
)

// Normalizer maps raw ingredient strings to canonical dictionary entries.
type Normalizer struct {
	// dict maps lowercase canonical name → canonical name (original casing)
	dict map[string]string
}

// NewNormalizer creates a Normalizer from the given canonical dictionary.
func NewNormalizer(canonical []string) *Normalizer {
	dict := make(map[string]string, len(canonical))
	for _, c := range canonical {
		dict[strings.ToLower(c)] = c
	}
	return &Normalizer{dict: dict}
}

// Normalize maps raw ingredient strings to canonical entries.
// Exact (case-insensitive) match is tried first; then fuzzy match within
// Levenshtein distance 2. Unmatched ingredients are returned in flagged.
func (n *Normalizer) Normalize(raw []string) (canonical []string, flagged []string) {
	for _, r := range raw {
		key := strings.ToLower(strings.TrimSpace(r))

		// Exact match first.
		if c, ok := n.dict[key]; ok {
			canonical = append(canonical, c)
			continue
		}

		// Fuzzy match: pick canonical with minimum edit distance ≤ 2.
		best, bestDist := "", math.MaxInt
		for k, c := range n.dict {
			if d := levenshtein(key, k); d < bestDist {
				bestDist = d
				best = c
			}
		}
		if bestDist <= 2 {
			canonical = append(canonical, best)
		} else {
			flagged = append(flagged, r)
		}
	}
	return canonical, flagged
}

// levenshtein computes the edit distance between two strings.
func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	m, n := len(ra), len(rb)
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
		dp[i][0] = i
	}
	for j := range dp[0] {
		dp[0][j] = j
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if ra[i-1] == rb[j-1] {
				dp[i][j] = dp[i-1][j-1]
			} else {
				dp[i][j] = 1 + min(dp[i-1][j], dp[i][j-1], dp[i-1][j-1])
			}
		}
	}
	return dp[m][n]
}

