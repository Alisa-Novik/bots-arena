package util

import (
	"math/rand"
)

func RollChance(percent int) bool {
	return rand.Intn(100) < percent
}
