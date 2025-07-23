package util

import (
	"container/heap"
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

func (p Position) SortByDist(dir Direction) Position {
	nr := p.R + dir[1]
	nc := (p.C + dir[0] + Cols) % Cols
	return Position{R: nr, C: nc}
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

type node struct {
	p Position
	f int
	g int
	i int
}
type hp []node

func (h hp) Len() int           { return len(h) }
func (h hp) Less(i, j int) bool { return h[i].f < h[j].f }
func (h hp) Swap(i, j int)      { h[i], h[j] = h[j], h[i]; h[i].i, h[j].i = i, j }
func (h *hp) Push(x any)        { *h = append(*h, x.(node)) }
func (h *hp) Pop() any          { n := len(*h) - 1; x := (*h)[n]; *h = (*h)[:n]; return x }

var i = 0

func FindPath(start, end Position, passable func(Position) bool) []Position {
	// i++
	// fmt.Println(i)
	if start == end {
		return nil
	}
	h := func(a, b Position) int { return abs(a.R-b.R) + abs(a.C-b.C) }

	open := &hp{{p: start, g: 0, f: h(start, end)}}
	heap.Init(open)
	gScore := map[Position]int{start: 0}
	prev := make(map[Position]Position)
	closed := make(map[Position]struct{})

	for open.Len() > 0 {
		curr := heap.Pop(open).(node)
		if curr.p == end {
			var path []Position
			for p := end; p != start; p = prev[p] {
				path = append([]Position{p}, path...)
			}
			return path
		}
		closed[curr.p] = struct{}{}
		for _, d := range PosCross {
			next := curr.p.AddRowCol(d[0], d[1])
			if next != end && !passable(next) {
				continue
			}
			if _, seen := closed[next]; seen {
				continue
			}
			gNext := curr.g + 1
			if gOld, ok := gScore[next]; !ok || gNext < gOld {
				gScore[next] = gNext
				prev[next] = curr.p
				heap.Push(open, node{p: next, g: gNext, f: gNext + h(next, end)})
			}
		}
	}
	return nil
}
