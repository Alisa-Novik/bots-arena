package tasking

import (
	"container/heap"
	"golab/internal/assert"
	"golab/internal/core"
	"golab/internal/util"
	"math"
	"slices"
)

type Position = util.Position

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
		assert.Assert(path[0] != botPos, "Current pos in path")
		assert.Assert(path[len(path)-1] == targetPos, "No target in path")
	}

	return path
}

func CalcFlowField(sources []util.Position, brd *core.Board) []int16 {
	dist := make([]int16, util.Cells)
	for i := range dist {
		dist[i] = math.MaxInt16
	}
	q := make([]int32, 0, util.Cells)
	for _, p := range sources {
		if util.OutOfBounds(p) {
			continue
		}
		sourceIdx := util.Idx(p)
		dist[sourceIdx] = 0
		q = append(q, int32(sourceIdx))
	}
	head := 0
	for head < len(q) {
		i := int(q[head])
		head++
		d := dist[i]
		for _, ni := range crossNeighborIndices(i) {
			if !brd.IsEmptyOrBotIdx(ni) {
				continue
			}
			if dist[ni] > d+1 {
				dist[ni] = d + 1
				q = append(q, int32(ni))
			}
		}
	}
	return dist
}

func crossNeighborIndices(i int) [4]int {
	c := i % util.Cols
	left := i - 1
	if c == 0 {
		left = i + util.Cols - 1
	}
	right := i + 1
	if c == util.Cols-1 {
		right = i - util.Cols + 1
	}
	return [4]int{
		i + util.Cols,
		right,
		i - util.Cols,
		left,
	}
}

func findPath(start, end Position, passable func(Position) bool) []Position {
	if start == end {
		return nil
	}
	h := func(a, b Position) int {
		dc := util.Abs(a.C - b.C)
		if wrapped := util.Cols - dc; wrapped < dc {
			dc = wrapped
		}
		return util.Abs(a.R-b.R) + dc
	}

	open := &hp{{p: start, g: 0, f: h(start, end)}}
	heap.Init(open)
	gScore := map[Position]int{start: 0}
	prev := make(map[Position]Position)
	closed := make(map[Position]struct{})

	for open.Len() > 0 {
		curr := heap.Pop(open).(node)
		if gScore[curr.p] != curr.g {
			continue
		}
		if curr.p == end {
			path := make([]Position, 0, curr.g)
			for p := end; p != start; p = prev[p] {
				path = append(path, p)
			}
			slices.Reverse(path)
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
