package util

import (
	"math/rand"
)

const (
	ScaleFactor = 2
	Rows        = 40 * ScaleFactor
	Cols        = 60 * ScaleFactor
)

type Position struct{ R, C int }

func (p Position) AddPos(other Position) Position {
	return Position{C: p.C + other.C, R: p.R + other.R}
}

func (p Position) Add(dr, dc int) Position {
	nr := p.R + dr
	nc := (p.C + dc + Cols) % Cols
	return Position{R: nr, C: nc}
}

func RollChanceOf(total, percent int) bool {
	return rand.Intn(total) < percent
}

func RollChance(percent int) bool {
	return rand.Intn(100) < percent
}

var PosClock = [8][2]int{
	// x, y clockwise
	{0, 1}, {1, 1}, {1, 0}, {1, -1},
	{0, -1}, {-1, -1}, {-1, 0}, {-1, 1},
}
