package util

import (
	"math/rand"
)

type Position struct{ R, C int }

func (p Position) AddPos(other Position) Position {
	return Position{C: p.C + other.C, R: p.R + other.R}
}

func (p Position) Add(r int, c int) Position {
	return Position{C: p.C + c, R: p.R + r}
}

func RollChance(percent int) bool {
	return rand.Intn(100) < percent
}

var PosClock = [8][2]int{
	// x, y clockwise
	{0, 1}, {1, 1}, {1, 0}, {1, -1},
	{0, -1}, {-1, -1}, {-1, 0}, {-1, 1},
}
