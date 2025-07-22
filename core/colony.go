package core

import (
	"golab/util"
	"slices"
	"time"
)

type Controller struct {
	Pos         util.Position
	Colony      *Colony
	Owner       *Bot
	Amount      int
	WaterAmount int
}

func (c *Colony) HealMember(m *Bot, ctrl *Controller) {
	if !m.ConnnectedToColony {
		return
	}
	if ctrl.Amount == 0 && m.Inventory.Amount > 0 {
		ctrl.Amount++
		m.Inventory.Amount--
	}
	if m.Inventory.Amount > 0 {
		m.Hp += 5
	} else {
		m.Hp += 3
	}
	if ctrl.Amount > 0 {
		ctrl.Amount--
	}
}

func (c *Colony) HealBotsInFlagRadius(radius, hpChange int) {
	for m := range c.Members {
		for _, f := range c.Flags {
			if m.Pos.InRadius(f.Pos, radius) {
				m.Hp += hpChange
			}
		}
	}
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

type ColonyMarkerType int

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

const (
	ResourceMarker ColonyMarkerType = iota
	WaterMarker
)

type ColonyMarker struct {
	Pos  util.Position
	Type ColonyMarkerType
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
	Type      ColonyTaskType
	Attempts  int
	Owner     *Bot
	IsDone    bool
	Pos       util.Position
	ExpiresAt time.Time
}

func (t *ColonyTask) IsExpired(now time.Time) bool {
	return t.ExpiresAt.Before(now)
}

func (c *ColonyTask) HasOwner() bool {
	return c.Owner != nil
}

func (t *ColonyTask) MarkDone() {
	t.IsDone = true
}

type ColonyFlag struct {
	Pos util.Position
}

func (c *Colony) NewMaintainConnectionTask(pos util.Position) *ColonyTask {
	return c.NewTask(pos, MaintainConnectionTask)
}

func (c *Colony) NewConnectionTask(pos util.Position) *ColonyTask {
	return c.NewTask(pos, ConnectToPosTask)
}

func (c *Colony) NewTask(pos util.Position, taskType ColonyTaskType) *ColonyTask {
	return &ColonyTask{
		Pos:       pos,
		Type:      taskType,
		Owner:     nil,
		ExpiresAt: CalcExpiresAt(),
	}
}

func CalcExpiresAt() time.Time {
	return time.Now().Add(3 * time.Second)
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

func (c *Colony) HasTaskOfType(taskType ColonyTaskType) bool {
	for _, task := range c.Tasks {
		if task.Type == taskType {
			return true
		}
	}
	return false
}
