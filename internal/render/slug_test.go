package render

import (
	"encoding/json"
	"os"
	"testing"
)

type slugVector struct {
	History []string `json:"history"`
	Input   string   `json:"input"`
	Want    string   `json:"want"`
}

func TestSlugifyVectors(t *testing.T) {
	data, err := os.ReadFile("testdata/slug_vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var vectors []slugVector
	if err := json.Unmarshal(data, &vectors); err != nil {
		t.Fatal(err)
	}
	for _, v := range vectors {
		seen := map[string]int{}
		for _, h := range v.History {
			// Prime the counter with the historical slug exactly as produced.
			if _, ok := seen[h]; !ok {
				seen[h] = 1
			} else {
				seen[h]++
			}
		}
		got := Slugify(v.Input, seen)
		if got != v.Want {
			t.Errorf("Slugify(%q, history=%v) = %q, want %q", v.Input, v.History, got, v.Want)
		}
	}
}
