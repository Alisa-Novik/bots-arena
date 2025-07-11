package util

import (
	"math/rand"
)

const (
	ScaleFactor = 5
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

func UnIdx(idx int) Position {
	return Position{R: idx / Cols, C: idx % Cols}
}

func Idx(p Position) int {
	return p.R*Cols + p.C
}

func RollChance(percent int) bool {
	return rand.Intn(100) < percent
}

var PosClock = [8][2]int{
	{0, 1}, {1, 1}, {1, 0}, {1, -1},
	{0, -1}, {-1, -1}, {-1, 0}, {-1, 1},
}

func FindPath(start, end Position, isEmpty func(Position) bool) []Position {
	if start == end {
		return nil
	}

	type Node struct {
		Pos  Position
		Prev *Node
	}

	visited := make(map[Position]bool)
	queue := []Node{{Pos: start}}

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		if curr.Pos == end {
			var path []Position
			for n := &curr; n != nil; n = n.Prev {
				path = append([]Position{n.Pos}, path...)
			}
			return path[1:]
		}

		if visited[curr.Pos] {
			continue
		}
		visited[curr.Pos] = true

		for _, dir := range PosClock {
			next := curr.Pos.Add(dir[0], dir[1])
			if !isEmpty(next) || visited[next] {
				continue
			}
			queue = append(queue, Node{Pos: next, Prev: &curr})
		}
	}
	return nil
}
