package core

import (
	"golab/util"
	"slices"
)

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

func (t *ColonyTask) MarkDone() {
	t.IsDone = true
}

func (c *Colony) HasTaskOfType(taskType ColonyTaskType) bool {
	for _, task := range c.Tasks {
		if task.Type == taskType {
			return true
		}
	}
	return false
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
