package bot

import (
	"math/rand"
)

type Direction [2]int

const genomeLen = 128
const genomeMaxValue = 128

const botHp = 100

type Genome struct {
	Matrix  [genomeLen]int
	Pointer int
}

var (
	Up    = Direction{0, 1}
	Right = Direction{1, 0}
	Down  = Direction{0, -1}
	Left  = Direction{-1, 0}
)

var IntToDir = map[int]Direction{
	0: Up,
	1: Right,
	2: Down,
	3: Left,
}

var dirs = []Direction{Up, Right, Down, Left}

type Inventory struct {
	Amount int
}

type Bot struct {
	Dir       Direction
	Genome    Genome
	Inventory Inventory
	Hp        int
}

func (b *Bot) PointerJumpBy(toAdd int) {
	ptr := b.Genome.Pointer
	nextPtr := ptr + toAdd
	nextPtr %= genomeLen

	b.Genome.Pointer = nextPtr
}

func (b *Bot) PointerJump() {
	ptr := b.Genome.Pointer
	toAdd := b.Genome.Matrix[ptr]
	nextPtr := ptr + toAdd
	nextPtr %= genomeLen

	b.Genome.Pointer = nextPtr
}

func NewBot() Bot {
	return Bot{
		Dir:       RandomDir(),
		Genome:    NewRandomGenome(),
		Inventory: NewEmptyInventory(),
		Hp:        botHp,
	}
}

func (parent *Bot) NewChild() Bot {
	return Bot{
		Dir:       RandomDir(),
		Genome:    NewMutatedGenome(parent.Genome),
		Inventory: NewEmptyInventory(),
		Hp:        botHp,
	}
}

func NewMutatedGenome(genome Genome) Genome {
	mutationIdx := rand.Intn(genomeLen)
	for i := range genome.Matrix {
		if i == mutationIdx {
			genome.Matrix[i] = rand.Intn(genomeMaxValue)
		}
	}
	return genome
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
		g.Matrix[i] = rand.Intn(genomeMaxValue)
	}
	g.Pointer = rand.Intn(genomeLen)
	return g
}
