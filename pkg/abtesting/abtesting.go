package abtesting

import (
	"hash/fnv"
)

// Experiment — A/B-тест с вариантами и весами.
type Experiment struct {
	Name       string
	Variations []Variation
}

// Variation — один вариант эксперимента.
type Variation struct {
	Name   string
	Weight int
}

// Assign распределяет пользователя по варианту детерминированно: hash(userID + имя эксперимента) % 100.
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
