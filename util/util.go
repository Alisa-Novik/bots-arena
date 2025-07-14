package util

import (
	"math/rand"
)

const (
	ScaleFactor = 5
	Rows        = 40 * ScaleFactor
	Cols        = 60 * ScaleFactor
	Cells       = Rows * Cols
)

type Direction [2]int

type Queue[T any] struct {
	data []T
}

func (q *Queue[T]) Enqueue(x T) {
	q.data = append(q.data, x)
}

func (q *Queue[T]) Dequeue() T {
	x := q.data[0]
	q.data = q.data[1:]
	return x
}

func (q *Queue[T]) Empty() bool {
	return len(q.data) == 0
}

type Position struct{ R, C int }

func NewPos(r, c int) Position {
	nc := (c + Cols) % Cols
	return Position{R: r, C: nc}
}

func OutOfBounds(p Position) bool {
	return p.R >= Rows-1 || p.R < 1
}

func (p Position) AddPos(other Position) Position {
	nc := (p.C + other.C + Cols) % Cols
	return Position{R: p.R + other.R, C: nc}
}

func (p Position) AddDir(dir Direction) Position {
	nr := p.R + dir[1]
	nc := (p.C + dir[0] + Cols) % Cols
	return Position{R: nr, C: nc}
}

func (p Position) Add(dr, dc int) Position {
	nr := p.R + dr
	nc := (p.C + dc + Cols) % Cols
	return Position{R: nr, C: nc}
}

func RollChanceOf(total, percent int) bool {
	return rand.Intn(total) < percent
}

func PosOf(idx int) Position {
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

func GreenColor() [3]float32 {
	return [3]float32{0, 1, 0}
}
