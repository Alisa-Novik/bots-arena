package core

import (
	"golab/internal/util"
	"golang.org/x/exp/rand"
)

type Occupant any
type Position = util.Position
type Wall struct{ Pos Position }

type Resource struct {
	Pos    Position
	Amount int
}
type Water struct {
	GroupId int
	Amount  int
}
type Organics struct {
	Pos    Position
	Amount int
}
type Food struct {
	Pos    Position
	Amount int
}
type Farm struct {
	Pos    Position
	Owner  *Bot
	Amount int
}
type Spawner struct {
	Pos    Position
	Owner  *Bot
	Amount int
}
type Mine struct {
	Pos    Position
	Owner  *Bot
	Amount int
}
type Poison struct {
	Pos Position
}
type Building struct {
	Pos   Position
	Owner *Bot
	Hp    int
}
type Board struct {
	TaskTargetsR   []util.Position
	PathsToRenderR []util.Position
	UnreachablesR  []util.Position

	grid     []Occupant
	occupied []bool
	dirty    []bool
	Bots     []*Bot
	// colonyCells []ColonyCell
	patch []int
}

const (
	Rows = util.Rows
	Cols = util.Cols
)

var PosClock = util.PosClock

var PathToPt = make(map[[2]int][]Position)

var neighbourIdx [Rows * Cols][8]int

func (b *Board) PullPatch() []int {
	p := b.patch
	b.patch = b.patch[:0]
	for _, i := range p {
		b.dirty[i] = false
	}
	return p
}

func (b *Board) MarkClean(i int) {
	b.dirty[i] = false
}

func (b *Board) MarkDirty(i int) {
	if !b.dirty[i] {
		b.dirty[i] = true
		b.patch = append(b.patch, i)
	}
}

func (b *Board) DirtyBitmap() []bool {
	return b.dirty
}

func initNeighbourTable() {
	for r := range Rows {
		for c := range Cols {
			idx := r*Cols + c
			for n, d := range PosClock {
				nr, nc := r+d[1], c+d[0]
				if nr < 0 || nr >= Rows || nc < 0 || nc >= Cols {
					neighbourIdx[idx][n] = -1
					continue
				}
				neighbourIdx[idx][n] = nr*Cols + ((nc + Cols) % Cols)
			}
		}
	}
}

func NewBoard() *Board {
	initNeighbourTable()
	return &Board{
		grid:     make([]Occupant, Rows*Cols),
		occupied: make([]bool, Rows*Cols),
		dirty:    make([]bool, Rows*Cols),
		Bots:     make([]*Bot, util.Cells),
	}
}

func NewRandomPosition() Position {
	return Position{C: rand.Intn(Cols), R: rand.Intn(Rows)}
}

func (b *Board) GetGrid() *[]Occupant {
	return &b.grid
}

func idx(p Position) int {
	return util.Idx(p)
}

func (b *Board) Clear(pos Position) {
	i := idx(pos)
	b.occupied[i] = false
	b.MarkDirty(i)
	b.grid[i] = nil
}

func (b *Board) Set(pos Position, o Occupant) {
	i := idx(pos)
	if !Inside(pos) {
		return
	}
	b.occupied[i] = true
	b.MarkDirty(i)
	b.grid[i] = o
}

func (b *Board) IsEmptyNoBot(pos Position) bool {
	if !(pos.R >= 0 && pos.R < Rows) {
		return false
	}

	return b.grid[idx(pos)] == nil
}

func (b *Board) IsEmpty(pos Position) bool {
	if !(pos.R >= 0 && pos.R < Rows) {
		return false
	}

	return b.grid[idx(pos)] == nil
}

func (b *Board) At(pos Position) Occupant {
	if pos.R < 0 || pos.R >= Rows {
		return nil
	}
	return b.grid[idx(pos)]
}

func (b *Board) IsPreserved(o Occupant) bool {
	switch o.(type) {
	case Controller, Farm, Food, Poison, Building, Water:
		return true
	default:
		return false
	}
}

func Inside(p Position) bool {
	return p.R >= 0 && p.R < Rows
}

func (b *Board) firstEmptyAround(idx int) int {
	start := rand.Intn(8)
	for i := range 8 {
		n := neighbourIdx[idx][(start+i)&7]
		if n >= 0 && !b.occupied[n] {
			return n
		}
	}
	return -1
}

func (b *Board) FindEmptyPosAround(p Position) (Position, bool) {
	n := b.firstEmptyAround(idx(p))
	if n < 0 {
		return Position{}, false
	}
	return Position{R: n / Cols, C: n % Cols}, true
}

func (b *Board) IsGrabable(pos Position) bool {
	switch b.At(pos).(type) {
	case Farm, Food, Poison, Controller, Resource, Building, Spawner:
		return true
	default:
		return false
	}
}

func (b *Board) IsWall(pos Position) bool {
	if !Inside(pos) {
		return true
	}
	return pos.R == 0 || pos.R == Rows-1
}

func (b *Board) GetBot(pos util.Position) *Bot {
	if !Inside(pos) {
		return nil
	}
	return b.Bots[idx(pos)]
}

func (b *Board) IsSurrounded(bp util.Position) bool {
	for _, dir := range Dirs {
		dPos := bp.AddDir(dir)
		if b.GetBot(dPos) == nil {
			return false
		}
	}
	return true
}

func (b *Board) IsEmptyOrBot(p util.Position) bool {
	return !util.OutOfBounds(p) && (b.IsEmpty(p) || b.GetBot(p) != nil)
}
