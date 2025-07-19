package bot

import (
	"golab/util"
	"math/rand"
	"slices"
	"sync"
)

type Direction = util.Direction

type Bot struct {
	Dir                Direction
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
	PathToTaskStart    []util.Position
	AttemptsToPassTask int

	hashLo, hashHi uint64
	Unloading      bool
	Usp            [2]int // unloading starting pos
}

func (b *Bot) SetColor(color [3]float32, markDirty func(int)) {
	// b.PrevColor = b.Color
	b.Color = color
	markDirty(util.Idx(b.Pos))
}

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

type Inventory struct {
	Amount int
}

func (parent *Bot) AddOffspring(offspring *Bot) {
	parent.Offsprings[offspring] = struct{}{}
}

func (parent *Bot) RemoveOffspring(offspring *Bot) {
	delete(parent.Offsprings, offspring)
}

type ColonyMarkerType int

const (
	ResourceMarker ColonyMarkerType = iota
	WaterMarker
)

type ColonyMarker struct {
	Colony *Colony
	Pos    util.Position
	Type   ColonyMarkerType
}

type ColonyTaskType int

func (t ColonyTaskType) String() string {
	switch t {
	case MaintainConnectionTask:
		return "MaintainConnectionTask"
	case ConnectToPosTask:
		return "ConnectToPosTask"
	case FindWaterTask:
		return "FindWaterTask"
	}
	return "UnknownTaskType"
}

const (
	FindWaterTask ColonyTaskType = iota
	ConnectToPosTask
	MaintainConnectionTask
)

type ColonyTask struct {
	Colony   *Colony
	Type     ColonyTaskType
	Attempts int
	Owner    *Bot
	IsDone   bool
	Pos      util.Position
}

type ColonyFlag struct {
	Colony *Colony
	Pos    util.Position
}

type Colony struct {
	Center         util.Position
	Members        map[*Bot]struct{}
	HasWater       bool
	Flags          []*ColonyFlag
	Markers        []*ColonyMarker
	Tasks          []*ColonyTask
	Color          [3]float32
	PathToWater    []util.Position
	WaterPositions []util.Position
	WaterGroupIds  []int
}

func NewColony(pos util.Position) Colony {
	return Colony{
		Center:   pos,
		HasWater: false,
		// Color:    util.RandomColor(),
		Color:   util.RedColor(),
		Members: make(map[*Bot]struct{}),

		// WaterPositions: make([]util.Position, 10),
		// WaterGroupIds:  make([]int, 10),
	}
}

func (b *Bot) PassTaskTo(other *Bot) {
}

func (c *Colony) HasTask(taskType ColonyTaskType) bool {
	for _, task := range c.Tasks {
		if task.Type == FindWaterTask {
			return true
		}
	}
	return false
}

func (b *Bot) ReassignTask(task *ColonyTask) {
	if task.Owner != nil {
		task.Owner.CurrTask = nil
	}
	task.Owner = b
	b.CurrTask = task
}

func (c *Colony) NewMaintainConnectionTask(pos util.Position) *ColonyTask {
	return c.NewTask(pos, MaintainConnectionTask)
}

func (c *Colony) NewConnectionTask(pos util.Position) *ColonyTask {
	return c.NewTask(pos, ConnectToPosTask)
}

func (c *Colony) NewTask(pos util.Position, taskType ColonyTaskType) *ColonyTask {
	return &ColonyTask{
		Colony: c,
		Pos:    pos,
		Type:   taskType,
		Owner:  nil,
	}
}

func (c *Colony) NewMarker(pos util.Position, markerType ColonyMarkerType) *ColonyMarker {
	return &ColonyMarker{
		Pos:    pos,
		Type:   markerType,
		Colony: c,
	}
}

func (b *Bot) MaintainingConn() bool {
	return b.CurrTask != nil && b.CurrTask.Type == MaintainConnectionTask && !b.CurrTask.IsDone
}

func (c *Colony) KnowsWaterGroupId(groupId int) bool {
	return slices.Contains(c.WaterGroupIds, groupId)
}

func (c *Colony) AddWaterPosition(pos util.Position, groupId int) {
	c.WaterPositions = append(c.WaterPositions, pos)
	c.WaterGroupIds = append(c.WaterGroupIds, groupId)
}

func (c *Colony) AddTask(task *ColonyTask) {
	c.Tasks = append(c.Tasks, task)
}

func (c *Colony) AddMarker(marker *ColonyMarker) {
	c.Markers = append(c.Markers, marker)
}

func (c *Colony) AddFlag(flag *ColonyFlag) {
	c.Flags = append(c.Flags, flag)
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

func (c *Colony) FlagsCount() int {
	return len(c.Flags)
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

func (b *Bot) AssignRandomColor() {
	b.Color = util.RandomColor()
}

var BotPool = sync.Pool{
	New: func() any { return new(Bot) },
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

func RandomDir() Direction {
	return Dirs[rand.Intn(4)]
}
