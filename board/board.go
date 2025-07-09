package board

import (
	"golab/bot"
	"golab/util"

	"golang.org/x/exp/rand"
)

type Occupant any
type Position = util.Position
type Wall struct{ Pos Position }

type Resource struct {
	Pos    Position
	Amount int
}
type Food struct {
	Pos    Position
	Amount int
}
type Farm struct {
	Pos    Position
	Owner  *bot.Bot
	Amount int
}
type Spawner struct {
	Pos    Position
	Owner  *bot.Bot
	Amount int
}
type Controller struct {
	Pos    Position
	Owner  *bot.Bot
	Amount int
}
type Building struct {
	Pos   Position
	Owner *bot.Bot
	Hp    int
}
type Board struct {
	grid map[Position]Occupant
}

var PathToPt = make(map[[2]int][]Position)

func (b *Board) FindPath(start, end Position) []Position {
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
			if !b.IsEmpty(next) || visited[next] {
				continue
			}
			queue = append(queue, Node{Pos: next, Prev: &curr})
		}
	}
	return nil
}

func (b *Board) Clear(pos Position) {
	delete(b.grid, pos)
}

func inside(p Position) bool {
	return p.R >= 0 && p.C >= 0 && p.R < Rows && p.C < Cols
}

func (b *Board) FindEmptyPosAround(center Position) (Position, bool) {
	for _, d := range PosClock {
		pos := center.Add(d[0], d[1])
		if inside(pos) && b.IsEmpty(pos) {
			return pos, true
		}
	}
	return Position{}, false
}

func (b *Board) HasController() bool {
	for r := range Rows {
		for c := range Cols {
			pos := NewPosition(r, c)
			if b.IsController(pos) {
				return true
			}
		}
	}
	return false
}

func (b *Board) IsGrabable(pos Position) bool {
	o := b.At(pos)
	switch o.(type) {
	case Farm, Food:
		return true
	}

	return b.IsController(pos) || b.IsResource(pos) || b.IsBuilding(pos) || b.IsSpawner(pos)
}

const scaleFactor = 2
const (
	Rows = 40 * scaleFactor
	Cols = 60 * scaleFactor
)

var PosClock = util.PosClock

func NewRandomPosition() Position {
	return Position{C: rand.Intn(Cols), R: rand.Intn(Rows)}
}

func NewPosition(r, c int) Position {
	return Position{C: c, R: r}
}

func NewBoard() *Board {
	return &Board{
		grid: make(map[Position]Occupant),
	}
}

func (b *Board) Set(pos Position, o Occupant) {
	if !inside(pos) {
		return
	}
	if o == nil {
		delete(b.grid, pos)
		return
	}
	b.grid[pos] = o
}

func (b *Board) IsEmpty(pos Position) bool {
	if !inside(pos) {
		return false
	}
	_, ok := b.grid[pos]
	return !ok
}

func (b *Board) At(pos Position) Occupant {
	return b.grid[pos]
}

func (b *Board) IsWall(pos Position) bool {
	if !inside(pos) {
		return true
	}
	return pos.C == 0 || pos.R == 0 || pos.C == Cols-1 || pos.R == Rows-1
}

func (b *Board) IsResource(pos Position) bool {
	if b.IsEmpty(pos) {
		return false
	}
	_, ok := b.At(pos).(Resource)
	return ok
}

func (b *Board) IsController(pos Position) bool {
	if b.IsEmpty(pos) {
		return false
	}
	_, ok := b.At(pos).(Controller)
	return ok
}

func (b Board) IsSpawner(pos Position) bool {
	if b.IsEmpty(pos) {
		return false
	}
	_, ok := b.At(pos).(Spawner)
	return ok
}

func (b *Board) IsBuilding(pos Position) bool {
	if b.IsEmpty(pos) {
		return false
	}
	_, ok := b.At(pos).(Building)
	return ok
}
