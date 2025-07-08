package util

import (
	"math/rand"
)

func RollChance(percent int) bool {
	return rand.Intn(100) < percent
}

var PosClock = [8][2]int{
	// x, y clockwise
	{0, 1}, {1, 1}, {1, 0}, {1, -1},
	{0, -1}, {-1, -1}, {-1, 0}, {-1, 1},
}
