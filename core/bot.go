package core

import (
	"golab/util"
	"math/rand"
	"sync"
)

var BotPool = sync.Pool{
	New: func() any { return new(Bot) },
}

type Inventory struct {
	Amount int
}

type Bot struct {
	Dir                util.Direction
	Genome             Genome
	Inventory          Inventory
	Colony             *Colony
	ConnnectedToColony bool
	Parent             *Bot
	Offsprings         map[*Bot]struct{}
	Hp                 int
	Color              [3]float32
	PrevColor          [3]float32
	HasSpawner         bool
	Pos                util.Position
	CurrTask           *ColonyTask
	Path               []util.Position

	hashLo, hashHi uint64
	Unloading      bool
	Usp            [2]int // unloading starting pos
}

func (m *Bot) DisconnectFromColony() {
	m.ConnnectedToColony = false
	m.Colony = nil
	if m.CurrTask != nil {
		m.UnassignTask()
	}
}

func NewBot(pos util.Position) Bot {
	color := util.RandomColor()
	return Bot{
		Dir:                RandomDir(),
		Pos:                pos,
		Genome:             NewRandomGenome(),
		Inventory:          NewEmptyInventory(),
		Colony:             nil,
		ConnnectedToColony: false,
		Parent:             nil,
		Offsprings:         make(map[*Bot]struct{}),
		Hp:                 botHp,
		Color:              color,
		PrevColor:          color,
		HasSpawner:         false,
		Unloading:          false,
		Usp:                [2]int{0, 0},
	}
}

func (b *Bot) SetColor(color [3]float32, markDirty func(int)) {
	// b.PrevColor = b.Color
	b.Color = color
	markDirty(util.Idx(b.Pos))
}

func (b *Bot) AssignTask(task *ColonyTask) {
	assert(!b.HasTask(), "Bot already has a task.")
	assert(!task.HasOwner(), "Task already has an owner.")

	b.CurrTask = task
	b.CurrTask.Owner = b
}

func (b *Bot) UnassignTask() {
	assert(b.CurrTask != nil, "No task to unassign")

	b.CurrTask.Owner = nil
	b.CurrTask = nil
	b.Path = nil
}

func (parent *Bot) AddOffspring(offspring *Bot) {
	parent.Offsprings[offspring] = struct{}{}
}

func (parent *Bot) RemoveOffspring(offspring *Bot) {
	delete(parent.Offsprings, offspring)
}

func (b *Bot) PeekNextPos() util.Position {
	return b.Path[0]
}

func (b *Bot) HasTask() bool {
	return b.CurrTask != nil
}

func (b *Bot) HasDoneTask() bool {
	return b.CurrTask != nil && b.CurrTask.IsDone
}

func (b *Bot) HasUndoneTask() bool {
	return b.CurrTask != nil && !b.CurrTask.IsDone
}

func (b *Bot) PopNextPos() util.Position {
	path := b.Path

	assert(len(path) > 0, "Trying to pop from empty path")
	assert(path[len(path)-1] == b.CurrTask.Pos, "No target in path")

	pos := path[0]
	b.Path = path[1:]
	return pos
}

func assert(cond bool, msg string) {
	if !cond {
		panic(msg)
	}
}

func (b *Bot) AssignRandomColor() {
	b.Color = util.RandomColor()
}

func (parent *Bot) NewChild(pos util.Position, shouldMutateColor bool) *Bot {
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
	b.Pos = pos

	b.Parent = parent
	if b.Offsprings == nil {
		b.Offsprings = make(map[*Bot]struct{})
	}
	b.Hp = botHp

	if shouldMutateColor && doMutation {
		b.Color = mutatedColor(parent.Color, doMutation)
	} else {
		b.Color = parent.Color
	}
	b.PrevColor = b.Color

	b.ConnnectedToColony = false

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
	const mutationStrength = 0.05
	var newColor [3]float32
	for i := range 3 {
		delta := (rand.Float32()*2 - 1) * mutationStrength
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

func (b *Bot) MaintainingConn() bool {
	return b.CurrTask != nil && b.CurrTask.Type == MaintainConnectionTask && !b.CurrTask.IsDone
}

// direction logic, might go to util
type Direction = util.Direction

var (
	Up    = Direction{0, 1}
	Right = Direction{1, 0}
	Down  = Direction{0, -1}
	Left  = Direction{-1, 0}
)

var Opposite = map[Direction]Direction{
	Up:    Down,
	Down:  Up,
	Right: Left,
	Left:  Right,
}

var Dirs = []Direction{Up, Right, Down, Left}
var DirIdx = map[Direction]int{
	Up:    0,
	Right: 1,
	Down:  2,
	Left:  3,
}

func RandomDir() Direction {
	return Dirs[rand.Intn(4)]
}
