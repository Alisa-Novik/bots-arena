package board

import (
	"golab/bot"
	"golab/util"
	"unsafe"

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
type Poison struct {
	Pos Position
}
type Building struct {
	Pos   Position
	Owner *bot.Bot
	Hp    int
}
type Board struct {
	grid     []Occupant
	occupied []bool
	dirty    []bool
	patch    []int
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
		occupied: make([]bool, (Rows+1)*(Cols+1)),
		dirty:    make([]bool, Rows*Cols),
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
	if !inside(pos) {
		return
	}
	b.occupied[i] = true
	b.MarkDirty(i)
	b.grid[i] = o
}

func (b *Board) IsEmpty(pos Position) bool {
	if !(pos.R >= 0 && pos.R < Rows) {
		return false
	}
	return b.grid[idx(pos)] == nil
}

func (b *Board) At(pos Position) Occupant {
	if !(pos.R >= 0 && pos.R < Rows) {
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

func inside(p Position) bool {
	return p.R >= 0 && p.R < Rows
}

func (b *Board) firstEmptyAround(idx int) int {

	base := unsafe.Pointer(&neighbourIdx[idx][0])
	size := unsafe.Sizeof(neighbourIdx[0][0]) // 8 on 64-bit, 4 on 32-bit

	for i := range 8 {
		n := *(*int)(unsafe.Pointer(uintptr(base) + uintptr(i)*size))
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
	if !inside(pos) {
		return true
	}
	return pos.R == 0 || pos.R == Rows-1
}
