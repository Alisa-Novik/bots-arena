package bot

import (
	"golab/util"
	"math/rand"
)

type Direction [2]int

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
	Registers  [4]int

	Unloading bool
	Usp       [2]int // unloading starting pos
}

func NewBot() Bot {
	return Bot{
		Dir:        RandomDir(),
		Genome:     NewRandomGenome(),
		Inventory:  NewEmptyInventory(),
		Hp:         botHp,
		Color:      redColor(),
		HasSpawner: false,
		Unloading:  false,
		Usp:        [2]int{0, 0},
	}
}

func redColor() [3]float32 {
	return [3]float32{1, 0, 0}
}

func blueColor() [3]float32 {
	return [3]float32{0, 1, 0.8}
}

func (parent *Bot) NewChild() Bot {
	if rand.Intn(1000) < 5 {
		return NewBot()
	}
	doMutation := util.RollChance(25)
	return Bot{
		Dir:        RandomDir(),
		Genome:     NewMutatedGenome(parent.Genome, doMutation),
		Inventory:  NewEmptyInventory(),
		Hp:         botHp,
		Color:      mutatedColor(parent.Color, doMutation),
		HasSpawner: false,
		Unloading:  false,
		Usp:        [2]int{0, 0},
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

func NewEmptyInventory() Inventory {
	return Inventory{Amount: 0}
}

func RandomDir() Direction {
	return dirs[rand.Intn(4)]
}
