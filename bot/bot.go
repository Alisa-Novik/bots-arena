package bot

import (
	"golab/util"
	"math/rand"
	"os"
	"strconv"
	"strings"
)

type Direction [2]int

const genomeLen = 128
const genomeMaxValue = 128
const mutationRate = 2
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

var dirs = []Direction{Up, Right, Down, Left}

type Inventory struct {
	Amount int
}

type Bot struct {
	Dir        Direction
	Genome     Genome
	Inventory  Inventory
	Hp         int
	Color      [3]float32
	HasSpawner bool
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
		Dir:        RandomDir(),
		Genome:     NewRandomGenome(),
		Inventory:  NewEmptyInventory(),
		Hp:         botHp,
		Color:      blueColor(),
		HasSpawner: false,
	}
}

func blueColor() [3]float32 {
	return [3]float32{0, 1, 0.8}
}

func (parent *Bot) NewChild() Bot {
	if rand.Intn(1000) < 2 {
		return NewBot()
	}
	doMutation := util.RollChance(2)
	return Bot{
		Dir:        RandomDir(),
		Genome:     NewMutatedGenome(parent.Genome, doMutation),
		Inventory:  NewEmptyInventory(),
		Hp:         botHp,
		Color:      mutatedColor(parent.Color, doMutation),
		HasSpawner: false,
	}
}

func mutatedColor(f [3]float32, doMutation bool) [3]float32 {
	if !doMutation {
		return f
	}
	const mutationStrength = 0.1
	var newColor [3]float32
	for i := range 3 {
		delta := (rand.Float32()*2 - 1) * mutationStrength // [-0.1, +0.1]
		v := f[i] + delta
		if v < 0 {
			v = 0
		} else if v > 1 {
			v = 1
		}
		newColor[i] = v
	}
	return newColor
}

func NewMutatedGenome(genome Genome, doMutation bool) Genome {
	if !doMutation {
		return genome
	}
	for _ = range mutationRate {
		mutationIdx := rand.Intn(genomeLen)
		for i := range genome.Matrix {
			if i == mutationIdx {
				genome.Matrix[i] = rand.Intn(genomeMaxValue)
			}
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

func ReadGenome() *Genome {
	data, _ := os.ReadFile("genome")
	parts := strings.Split(strings.TrimSuffix(string(data), ","), ",")
	var genome [128]int
	for i := range genome {
		genome[i], _ = strconv.Atoi(parts[i])
	}
	return &Genome{Matrix: genome}
}

func ExportGenome(b Bot) {
	var bld strings.Builder
	for _, v := range b.Genome.Matrix {
		bld.WriteString(strconv.Itoa(v))
		bld.WriteByte(',')
	}
	os.WriteFile("genome", []byte(bld.String()), 0644)
}
