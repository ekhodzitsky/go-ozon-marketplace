package abtesting

import (
	"hash/fnv"
)

// Experiment represents an A/B test with weighted variations.
type Experiment struct {
	Name       string
	Variations []Variation
}

// Variation is a single variant in an experiment.
type Variation struct {
	Name   string
	Weight int
}

// Assign deterministically assigns a user to a variation based on hash(userID + experimentName) % 100.
func (e *Experiment) Assign(userID string) string {
	if len(e.Variations) == 0 {
		return ""
	}
	h := hashString(userID + e.Name)
	slot := int(h % 100)
	cumulative := 0
	for _, v := range e.Variations {
		cumulative += v.Weight
		if slot < cumulative {
			return v.Name
		}
	}
	return e.Variations[len(e.Variations)-1].Name
}

func hashString(s string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return h.Sum32()
}
