package board

import (
	"golab/bot"
)

type Position struct{ X, Y int }

type Wall struct{ Pos Position }
type Poison struct{ Pos Position }
type Food struct{ Pos Position }
type Board struct {
	grid map[Position]Occupant
}

func (b Board) SyncBots(bots map[Position]bot.Bot) {
	for r := range rows {
		for c := range cols {
			pos := NewPosition(r, c)
			if b.isBot(pos) {
				bots[pos] = b.At(pos).(bot.Bot)
			}
		}
	}
}

type Occupant interface{}

const (
	rows = 20
	cols = 40
)

func NewPosition(r, c int) Position {
	return Position{X: c, Y: r}
}

func NewBoard() *Board {
	return &Board{
		grid: make(map[Position]Occupant),
	}
}

func (b *Board) Set(pos Position, o Occupant) {
	if o == nil {
		delete(b.grid, pos)
		return
	}
	b.grid[pos] = o
}

func (b *Board) IsEmpty(pos Position) bool {
	_, ok := b.grid[pos]
	return !ok
}

func (b *Board) At(pos Position) Occupant {
	return b.grid[pos]
}

func (b *Board) IsWall(pos Position) bool {
	return pos.X == 0 || pos.Y == 0 || pos.X == cols-1 || pos.Y == rows-1
}

func (b *Board) isBot(pos Position) bool {
	if b.IsEmpty(pos) {
		return false
	}
	_, ok := b.At(pos).(bot.Bot)
	return ok
}
