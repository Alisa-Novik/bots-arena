package util

import (
	"math"
	"math/rand"
)

const (
	ScaleFactor = 7
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

func (p Position) IsZero() bool {
	return p.R == 0 && p.C == 0
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func toroidalDelta(a, b, size int) int {
	d := abs(a - b)
	return min(d, size-d)
}

func NewPos(r, c int) Position {
	nc := (c + Cols) % Cols
	return Position{R: r, C: nc}
}

func CalcDistancePos(a, b Position) Position {
	return Position{R: b.R - a.R, C: b.C - a.C}
}

func CalcDistance(a, b Position) int {
	dr := float64(a.R - b.R)
	dc := float64(a.C - b.C)
	return int(math.Floor(math.Sqrt(dr*dr + dc*dc)))
}

func OutOfBounds(p Position) bool {
	return p.R >= Rows-1 || p.R < 1
}

func (p Position) AddPos(other Position) Position {
	nc := (p.C + other.C + Cols) % Cols
	return Position{R: p.R + other.R, C: nc}
}

func (p Position) InRadius(center Position, radius int) bool {
	dr := toroidalDelta(p.R, center.R, Rows)
	dc := toroidalDelta(p.C, center.C, Cols)
	return dr <= radius && dc <= radius
}

func (p Position) AddDir(dir Direction) Position {
	nr := p.R + dir[1]
	nc := (p.C + dir[0] + Cols) % Cols
	return Position{R: nr, C: nc}
}

func (p Position) AddRowCol(dr, dc int) Position {
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

var PosCross = [8][2]int{
	{0, 1}, {1, 0}, {0, -1}, {-1, 0},
}

var PosClock = [8][2]int{
	{0, 1}, {1, 1}, {1, 0}, {1, -1},
	{0, -1}, {-1, -1}, {-1, 0}, {-1, 1},
}

func FindPath(start, end Position, passable func(Position) bool) []Position {
	if start == end {
		return nil
	}

	prev := make(map[Position]Position)
	visited := make(map[Position]struct{})
	queue := []Position{start}
	visited[start] = struct{}{}

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		for _, d := range PosCross {
			next := curr.AddRowCol(d[0], d[1])
			if _, seen := visited[next]; seen || (!passable(next) && next != end) {
				continue
			}
			prev[next] = curr
			if next == end {
				var path []Position
				for p := end; p != start; p = prev[p] {
					path = append([]Position{p}, path...)
				}
				return path
			}
			visited[next] = struct{}{}
			queue = append(queue, next)
		}
	}
	return nil
}
