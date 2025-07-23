package tasking

import (
	"container/heap"
	"golab/internal/util"
)

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

func CalcPath(
	botPos util.Position,
	targetPos util.Position,
	filter func(util.Position) bool,
	isSurrounded func(util.Position) bool,
) []util.Position {
	if isSurrounded != nil && isSurrounded(botPos) {
		return nil
	}
	path := findPath(botPos, targetPos, filter)

	if len(path) != 0 {
		assert(path[0] != botPos, "Current pos in path")
		assert(path[len(path)-1] == targetPos, "No target in path")
	}

	return path
}

func findPath(start, end Position, passable func(Position) bool) []Position {
	if start == end {
		return nil
	}
	h := func(a, b Position) int { return util.Abs(a.R-b.R) + util.Abs(a.C-b.C) }

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
		for _, d := range util.PosCross {
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

type Position = util.Position
