package bot

import (
	"golab/util"
	"math/rand"
	"sync"
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

func (parent *Bot) AddOffspring(offspring *Bot) {
	parent.Offsprings[offspring] = struct{}{}
}

func (parent *Bot) RemoveOffspring(offspring *Bot) {
	delete(parent.Offsprings, offspring)
}

type Colony struct {
	Center  util.Position
	Members map[*Bot]struct{}
}

func (c *Colony) AddFamily(b *Bot) {
	b.Colony = c
	c.AddMember(b)
	for o := range b.Offsprings {
		o.Colony = c
		c.AddMember(o)
	}
}

func (c *Colony) RemoveMember(offspring *Bot) {
	delete(c.Members, offspring)
}

func (c *Colony) AddMember(offspring *Bot) {
	c.Members[offspring] = struct{}{}
}

type Bot struct {
	Dir        Direction
	Genome     Genome
	Inventory  Inventory
	Colony     *Colony
	Parent     *Bot
	Offsprings map[*Bot]struct{}
	Hp         int
	Color      [3]float32
	HasSpawner bool

	hashLo, hashHi uint64
	Unloading      bool
	Usp            [2]int // unloading starting pos
}

func twoBitHash(g int) uint64 {
	return uint64((g*0x9e3779b1)>>30) & 3
}

func NewColony(pos util.Position) Colony {
	return Colony{
		Center:  pos,
		Members: make(map[*Bot]struct{}),
	}
}

func NewBot() Bot {
	return Bot{
		Dir:        RandomDir(),
		Genome:     NewRandomGenome(),
		Inventory:  NewEmptyInventory(),
		Colony:     nil,
		Parent:     nil,
		Offsprings: make(map[*Bot]struct{}),
		Hp:         botHp,
		Color:      blueColor(),
		HasSpawner: false,
		Unloading:  false,
		Usp:        [2]int{0, 0},
	}
}

func randomColor() [3]float32 {
	return [3]float32{rand.Float32(), rand.Float32(), rand.Float32()}
}

func blueColor() [3]float32 {
	return [3]float32{0, 0, 1}
}

func redColor() [3]float32 {
	return [3]float32{1, 0, 0}
}

var BotPool = sync.Pool{
	New: func() any { return new(Bot) },
}

func (parent *Bot) NewChild() *Bot {
	if rand.Intn(1000) < 5 {
		return BotPool.Get().(*Bot)
	}

	doMutation := util.RollChance(25)

	b := BotPool.Get().(*Bot)
	*b = Bot{}
	b.Dir = RandomDir()
	b.Genome = NewMutatedGenome(parent.Genome, doMutation)
	b.Inventory = NewEmptyInventory()
	b.Colony = parent.Colony
	b.Parent = parent
	if b.Offsprings == nil {
		b.Offsprings = make(map[*Bot]struct{})
	}
	b.Hp = botHp
	// b.Color = mutatedColor(parent.Color, doMutation)
	b.Color = parent.Color

	parent.AddOffspring(b)
	if parent.Colony != nil {
		parent.Colony.AddMember(b)
	}
	return b
}

func (b *Bot) CountOffsprings() int {
	if b == nil {
		return 0
	}
	sum := 0
	for o := range b.Offsprings {
		sum += o.CountOffsprings()
	}
	return 1 + sum
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
