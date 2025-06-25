package board

import (
	"golab/bot"
	"golang.org/x/exp/rand"
)

type Position struct{ X, Y int }

type Wall struct{ Pos Position }
type Resource struct {
	Pos    Position
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

func (b Board) SyncBots(botsMap map[Position]bot.Bot) {
	for r := range Rows {
		for c := range Cols {
			pos := NewPosition(r, c)
			if b.isBot(pos) {
				botsMap[pos] = b.At(pos).(bot.Bot)
			}
		}
	}
}

type Occupant any

const scaleFactor = 1
const (
	Rows = 40 * scaleFactor
	Cols = 60 * scaleFactor
)

var PosClock = [8][2]int{
	// x, y clockwise
	{0, 1}, {1, 1}, {1, 0}, {1, -1},
	{0, -1}, {-1, -1}, {-1, 0}, {-1, 1},
}

func NewRandomPosition() Position {
	return Position{X: rand.Intn(Cols), Y: rand.Intn(Rows)}
}

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
	return pos.X == 0 || pos.Y == 0 || pos.X == Cols-1 || pos.Y == Rows-1
}

func (b *Board) IsResource(pos Position) bool {
	if b.IsEmpty(pos) {
		return false
	}
	_, ok := b.At(pos).(Resource)
	return ok
}

func (b *Board) IsBuilding(pos Position) bool {
	if b.IsEmpty(pos) {
		return false
	}
	_, ok := b.At(pos).(Building)
	return ok
}

func (b *Board) isBot(pos Position) bool {
	if b.IsEmpty(pos) {
		return false
	}
	_, ok := b.At(pos).(bot.Bot)
	return ok
}
