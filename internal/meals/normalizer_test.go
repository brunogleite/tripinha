package meals_test

import (
	"testing"

	"github.com/brunogleite/tripinha/internal/meals"
)

// Cycle 14: fuzzy match within edit distance 2 → canonical
func TestNormalizer_FuzzyMatch(t *testing.T) {
	dict := []string{"Sugar", "Skimmed Milk", "Palm Oil"}
	n := meals.NewNormalizer(dict)

	tests := []struct {
		raw  string
		want string
	}{
		{"Suger", "Sugar"},         // 1 edit: substitution
		{"Skimed Milk", "Skimmed Milk"}, // 1 deletion: "Skimed" → "Skimmed"
		{"Plam Oil", "Palm Oil"},    // 1 transposition
	}
	for _, tt := range tests {
		canonical, flagged := n.Normalize([]string{tt.raw})
		if len(canonical) != 1 || canonical[0] != tt.want {
			t.Errorf("Normalize(%q): got canonical=%v flagged=%v, want canonical=[%q]", tt.raw, canonical, flagged, tt.want)
		}
	}
}

// Cycle 15: ingredient with edit distance > 2 → flagged, canonical untouched
func TestNormalizer_UnknownIngredient(t *testing.T) {
	dict := []string{"Sugar", "Palm Oil"}
	n := meals.NewNormalizer(dict)

	canonical, flagged := n.Normalize([]string{"Sugar", "xylitol"})

	if len(canonical) != 1 || canonical[0] != "Sugar" {
		t.Errorf("canonical: got %v, want [Sugar]", canonical)
	}
	if len(flagged) != 1 || flagged[0] != "xylitol" {
		t.Errorf("flagged: got %v, want [xylitol]", flagged)
	}
}

func TestNormalizer_ExactMatch(t *testing.T) {
	dict := []string{"Sugar", "Palm Oil", "Hazelnuts"}
	n := meals.NewNormalizer(dict)

	canonical, flagged := n.Normalize([]string{"Sugar", "palm oil", "HAZELNUTS"})

	wantCanonical := []string{"Sugar", "Palm Oil", "Hazelnuts"}
	if len(canonical) != len(wantCanonical) {
		t.Fatalf("canonical len: got %d, want %d; got %v", len(canonical), len(wantCanonical), canonical)
	}
	for i, w := range wantCanonical {
		if canonical[i] != w {
			t.Errorf("canonical[%d]: got %q, want %q", i, canonical[i], w)
		}
	}
	if len(flagged) != 0 {
		t.Errorf("flagged: got %v, want empty", flagged)
	}
}
