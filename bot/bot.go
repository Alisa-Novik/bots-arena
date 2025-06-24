package bot

import (
	"math/rand"
)

type Direction [2]int

const genomeMatrixCells = 64

type Genome struct {
	Matrix  [genomeMatrixCells]int
	Pointer int
}

var (
	Up    = Direction{0, 1}
	Right = Direction{1, 0}
	Down  = Direction{0, -1}
	Left  = Direction{-1, 0}
)

var dirs = []Direction{Up, Right, Down, Left}

type Inventory struct {
	Amount int
}

type Bot struct {
	Name      string
	Dir       Direction
	Genome    Genome
	Inventory Inventory
	Hp        int
}

func (b *Bot) PointerJump() {
	ptr := b.Genome.Pointer
	toAdd := b.Genome.Matrix[ptr]
	nextPtr := ptr + toAdd
	nextPtr %= genomeMatrixCells

	b.Genome.Pointer = nextPtr
}

func NewBot(name string) Bot {
	return Bot{
		Name:      name,
		Dir:       RandomDir(),
		Genome:    NewRandomGenome(),
		Inventory: NewEmptyInventory(),
		Hp:        50,
	}
}

func NewEmptyInventory() Inventory {
	return Inventory{Amount: 0}
}

func RandomDir() Direction {
	return dirs[rand.Intn(4)]
}

func NewRandomGenome() Genome {
	var g Genome
	for i := range g.Matrix {
		g.Matrix[i] = rand.Intn(64)
	}
	g.Pointer = rand.Intn(genomeMatrixCells)
	return g
}
