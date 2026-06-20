package core

import (
	"golab/internal/assert"
	"golab/internal/util"
	"math/rand"
	"sync"
	"time"
)

var BotPool = sync.Pool{
	New: func() any { return new(Bot) },
}

type Inventory struct {
	Food int
	Ore  int
}

func (i Inventory) Total() int {
	return i.Food + i.Ore
}

func (i Inventory) CanPay(food, ore int) bool {
	food = max(0, food)
	ore = max(0, ore)
	return i.Food >= food && i.Ore >= ore
}

func (i *Inventory) Spend(food, ore int) bool {
	food = max(0, food)
	ore = max(0, ore)
	if !i.CanPay(food, ore) {
		return false
	}
	i.Food -= food
	i.Ore -= ore
	return true
}

func (i *Inventory) AddFood(n int) {
	if n <= 0 {
		return
	}
	i.Food += n
}

func (i *Inventory) AddOre(n int) {
	if n <= 0 {
		return
	}
	i.Ore += n
}

func (i *Inventory) Clear() {
	i.Food = 0
	i.Ore = 0
}

type BotEvolutionStats struct {
	FoodGathered        int
	OreGathered         int
	StolenFood          int
	StolenOre           int
	CombatKills         int
	ControllerRaids     int
	DepotRaids          int
	SuccessfulDivisions int
	ControllerBuilds    int
	FarmBuilds          int
	MineBuilds          int
	DepotBuilds         int
	SpawnerBuilds       int
	SpawnerBirths       int
	DepotDepositedFood  int
	DepotDepositedOre   int
	TaskCompletions     int
}

type Bot struct {
	Dir                util.Direction
	Genome             Genome
	Inventory          Inventory
	Evolution          BotEvolutionStats
	Colony             *Colony
	ConnnectedToColony bool
	Parent             *Bot
	Offsprings         map[*Bot]struct{}
	OffspringCount     int
	Divisions          int
	LineageDepth       int
	Age                int
	Hp                 int
	Color              [3]float32
	PrevColor          [3]float32
	IsSelected         bool
	HasSpawner         bool
	Pos                util.Position
	CurrTask           *ColonyTask
	// Path               []util.Position
	CooldownUntil time.Time
}

func (m *Bot) HasCooldown(now time.Time) bool {
	return now.Before(m.CooldownUntil)
}

func (m *Bot) StartCooldown(now time.Time) {
	m.CooldownUntil = now.Add(5 * time.Second)
}

func (m *Bot) DisconnectFromColony() {
	m.ConnnectedToColony = false
	// m.Colony = nil
	// if m.CurrTask != nil {
	// 	m.UnassignTask()
	// }
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
		Hp:                 botHp,
		Color:              color,
		PrevColor:          color,
		IsSelected:         false,
		HasSpawner:         false,
	}
}

func (b *Bot) GatherFood(n int) {
	if b == nil || n <= 0 {
		return
	}
	b.Inventory.AddFood(n)
	b.Evolution.FoodGathered += n
}

func (b *Bot) GatherOre(n int) {
	if b == nil || n <= 0 {
		return
	}
	b.Inventory.AddOre(n)
	b.Evolution.OreGathered += n
}

func (b *Bot) StealFood(n int) {
	if b == nil || n <= 0 {
		return
	}
	b.Inventory.AddFood(n)
	b.Evolution.StolenFood += n
}

func (b *Bot) StealOre(n int) {
	if b == nil || n <= 0 {
		return
	}
	b.Inventory.AddOre(n)
	b.Evolution.StolenOre += n
}

func (b *Bot) RecordCombatKill() {
	if b == nil {
		return
	}
	b.Evolution.CombatKills++
}

func (b *Bot) RecordControllerRaid() {
	if b == nil {
		return
	}
	b.Evolution.ControllerRaids++
}

func (b *Bot) RecordDepotRaid() {
	if b == nil {
		return
	}
	b.Evolution.DepotRaids++
}

func (b *Bot) RecordDepotDeposit(food, ore int) {
	if b == nil {
		return
	}
	if food > 0 {
		b.Evolution.DepotDepositedFood += food
	}
	if ore > 0 {
		b.Evolution.DepotDepositedOre += ore
	}
}

func (b *Bot) SetColor(color [3]float32, markDirty func(int)) {
	// b.PrevColor = b.Color
	b.Color = color
	markDirty(util.Idx(b.Pos))
}

func (b *Bot) AssignTask(task *ColonyTask) {
	assert.Assert(b.Colony != nil, "Bot doesn't have a colony.")
	assert.Assert(!b.HasTask(), "Bot already has a task.")
	assert.Assert(!task.HasOwner(), "Task already has an owner.")

	b.CurrTask = task
	b.CurrTask.Owner = b
	task.ExpiresAt = CalcTaskExpiresAt(task.Type)
	b.Colony.AssignedTasksCount++
}

func (b *Bot) UnassignTask(now time.Time) {
	assert.Assert(b.CurrTask != nil, "No task to unassign")

	b.CurrTask.ExpiresAt = CalcTaskExpiresAt(b.CurrTask.Type)

	b.CurrTask.Owner = nil
	b.CurrTask = nil
	// b.Path = nil
	b.Colony.AssignedTasksCount--
	b.StartCooldown(now)
}

func (parent *Bot) AddOffspring(offspring *Bot) {
	if parent == nil || offspring == nil {
		return
	}
	if parent.Offsprings == nil {
		parent.Offsprings = make(map[*Bot]struct{})
	}
	if _, ok := parent.Offsprings[offspring]; ok {
		return
	}
	parent.Offsprings[offspring] = struct{}{}
	parent.OffspringCount++
}

func (parent *Bot) RemoveOffspring(offspring *Bot) {
	if parent == nil || offspring == nil || parent.Offsprings == nil {
		return
	}
	if _, ok := parent.Offsprings[offspring]; ok && parent.OffspringCount > 0 {
		parent.OffspringCount--
	}
	delete(parent.Offsprings, offspring)
}

// func (b *Bot) PeekNextPos() util.Position {
// 	return b.Path[0]
// }

func (b *Bot) HasTask() bool {
	return b.CurrTask != nil
}

func (b *Bot) HasDoneTask() bool {
	return b.CurrTask != nil && b.CurrTask.IsDone
}

func (b *Bot) HasUndoneTask() bool {
	return b.CurrTask != nil && !b.CurrTask.IsDone
}

// func (b *Bot) PopNextPos() util.Position {
// 	path := b.Path
//
// 	assert.Assert(len(path) > 0, "Trying to pop from empty path")
// 	assert.Assert(path[len(path)-1] == b.CurrTask.Pos, "No target in path")
//
// 	pos := path[0]
// 	b.Path = path[1:]
// 	return pos
// }

func (b *Bot) AssignRandomColor() {
	b.Color = util.RandomColor()
}

func (parent *Bot) NewChild(pos util.Position, shouldMutateColor bool) *Bot {
	return parent.NewChildWithMutationRate(pos, shouldMutateColor, defaultGenomeMutationRate)
}

func (parent *Bot) NewChildWithMutationRate(pos util.Position, shouldMutateColor bool, mutationRate int) *Bot {
	// Keep the historical RNG stream stable while initializing fresh child bots.
	_ = rand.Intn(1000)
	doMutation := util.RollChance(25)

	b := &Bot{}
	b.Dir = RandomDir()
	if doMutation {
		b.Genome = NewMutatedGenomeWithRate(parent.Genome, mutationRate)
	} else {
		b.Genome = parent.Genome
	}
	b.Inventory = NewEmptyInventory()
	b.Colony = parent.Colony
	b.Pos = pos

	b.Parent = parent
	b.LineageDepth = parent.LineageDepth + 1
	b.Age = 0
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
	if len(b.Offsprings) == 0 {
		return 1 + b.OffspringCount
	}
	sum := 0
	for o := range b.Offsprings {
		sum += o.CountOffsprings()
	}
	return 1 + sum
}

func (b *Bot) ReproductionScore() int {
	if b == nil {
		return 0
	}
	return b.LineageDepth + b.Divisions
}

func (b *Bot) EvolutionScore() int {
	if b == nil {
		return 0
	}
	return b.Divisions*10000 +
		b.LineageDepth*3500 +
		b.Evolution.ControllerBuilds*1200 +
		b.Evolution.DepotBuilds*900 +
		b.Evolution.SpawnerBuilds*700 +
		b.Evolution.SpawnerBirths*1400 +
		b.Evolution.FarmBuilds*600 +
		b.Evolution.TaskCompletions*500 +
		min(b.Evolution.DepotDepositedFood*45+b.Evolution.DepotDepositedOre*30, 4500) +
		b.Evolution.FoodGathered*120 +
		b.Evolution.OreGathered*35 +
		min(b.Inventory.Food, b.Inventory.Ore)*160 +
		b.Inventory.Total()*20 +
		b.Age/4 +
		b.Hp
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
	return Inventory{}
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

var Dirs = []Direction{Up, Right, Down, Left}

func RandomDir() Direction {
	return Dirs[rand.Intn(4)]
}
