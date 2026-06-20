package core

import (
	"golab/internal/util"
	"slices"
	"sort"
	"time"
)

type Controller struct {
	Pos         util.Position
	Colony      *Colony
	Owner       *Bot
	Amount      int
	WaterAmount int
}

type Colony struct {
	Center   util.Position
	Members  []*Bot
	HasWater bool
	FoodBank int
	OreBank  int
	Flags    []*ColonyFlag
	Markers  []*ColonyMarker
	Tasks    []*ColonyTask
	Color    [3]float32

	SpawnerGenome      Genome
	HasSpawnerGenome   bool
	SpawnerGenomeScore int

	WaterPathFlowField []int16
	PathToWater        []util.Position
	pathToWaterMask    []bool
	pathToWaterIndex   []int
	WaterPositions     []util.Position
	WaterGroupIds      []int

	AssignedTasksCount int
	Counter            int
}

func NewColony(pos util.Position) Colony {
	return Colony{
		Center:   pos,
		HasWater: false,
		Color:    colonyColor(pos),
		// Members: make(map[*Bot]struct{}),
		// WaterPositions: make([]util.Position, 10),
		// WaterGroupIds:  make([]int, 10),
	}
}

func colonyColor(pos util.Position) [3]float32 {
	hash := uint32(pos.R*73856093) ^ uint32(pos.C*19349663) ^ 0x9e3779b9
	hue := float32(hash%360) / 360
	return hsvColor(hue, 0.76, 0.96)
}

func hsvColor(h, s, val float32) [3]float32 {
	h = h - float32(int(h))
	if h < 0 {
		h += 1
	}
	sector := h * 6
	i := int(sector)
	f := sector - float32(i)
	p := val * (1 - s)
	q := val * (1 - s*f)
	t := val * (1 - s*(1-f))

	switch i % 6 {
	case 0:
		return [3]float32{val, t, p}
	case 1:
		return [3]float32{q, val, p}
	case 2:
		return [3]float32{p, val, t}
	case 3:
		return [3]float32{p, q, val}
	case 4:
		return [3]float32{t, p, val}
	default:
		return [3]float32{val, p, q}
	}
}

func (c *Colony) BankTotal() int {
	if c == nil {
		return 0
	}
	return c.FoodBank + c.OreBank
}

func (c *Colony) CanBankPay(food, ore int) bool {
	if c == nil {
		return false
	}
	food = max(0, food)
	ore = max(0, ore)
	return c.FoodBank >= food && c.OreBank >= ore
}

func (c *Colony) Deposit(food, ore int) {
	if c == nil {
		return
	}
	c.FoodBank += max(0, food)
	c.OreBank += max(0, ore)
}

func (c *Colony) SpendBank(food, ore int) bool {
	if !c.CanBankPay(food, ore) {
		return false
	}
	food = max(0, food)
	ore = max(0, ore)
	c.FoodBank -= food
	c.OreBank -= ore
	return true
}

func (c *Colony) CanPayWithBank(bot *Bot, food, ore int) bool {
	if bot == nil {
		return false
	}
	food = max(0, food)
	ore = max(0, ore)
	if !c.canUseBank(bot) {
		return bot.Inventory.CanPay(food, ore)
	}
	return bot.Inventory.Food+c.FoodBank >= food && bot.Inventory.Ore+c.OreBank >= ore
}

func (c *Colony) SpendWithBank(bot *Bot, food, ore int) bool {
	if bot == nil || !c.CanPayWithBank(bot, food, ore) {
		return false
	}
	food = max(0, food)
	ore = max(0, ore)
	if !c.canUseBank(bot) {
		return bot.Inventory.Spend(food, ore)
	}

	personalFood := min(bot.Inventory.Food, food)
	bot.Inventory.Food -= personalFood
	c.FoodBank -= food - personalFood

	personalOre := min(bot.Inventory.Ore, ore)
	bot.Inventory.Ore -= personalOre
	c.OreBank -= ore - personalOre
	return true
}

func (c *Colony) DepositMemberSurplus(bot *Bot, reserveFood, reserveOre int) {
	if !c.canUseBank(bot) {
		return
	}
	foodSurplus := max(0, bot.Inventory.Food-max(0, reserveFood))
	oreSurplus := max(0, bot.Inventory.Ore-max(0, reserveOre))
	if foodSurplus == 0 && oreSurplus == 0 {
		return
	}
	bot.Inventory.Food -= foodSurplus
	bot.Inventory.Ore -= oreSurplus
	c.Deposit(foodSurplus, oreSurplus)
}

func (c *Colony) canUseBank(bot *Bot) bool {
	return c != nil && bot != nil && bot.Colony == c && bot.ConnnectedToColony
}

func CanPayWithBank(bot *Bot, food, ore int) bool {
	if bot == nil {
		return false
	}
	if bot.Colony == nil {
		return bot.Inventory.CanPay(food, ore)
	}
	return bot.Colony.CanPayWithBank(bot, food, ore)
}

func SpendWithBank(bot *Bot, food, ore int) bool {
	if bot == nil {
		return false
	}
	if bot.Colony == nil {
		return bot.Inventory.Spend(food, ore)
	}
	return bot.Colony.SpendWithBank(bot, food, ore)
}

func AccessibleInventory(bot *Bot) Inventory {
	if bot == nil {
		return Inventory{}
	}
	out := bot.Inventory
	if bot.Colony != nil && bot.Colony.canUseBank(bot) {
		out.Food += bot.Colony.FoodBank
		out.Ore += bot.Colony.OreBank
	}
	return out
}

func (c *Colony) HealMember(m *Bot, ctrl *Controller) {
	if !m.ConnnectedToColony {
		return
	}
	if ctrl.Amount <= 0 {
		if !c.SpendWithBank(m, 1, 0) {
			return
		}
		ctrl.Amount++
	}
	if m.Inventory.Food > 0 {
		m.Hp += 15
	} else {
		m.Hp += 3
	}
	if ctrl.Amount > 0 {
		ctrl.Amount--
	}
}

func (c *Colony) HealBotsInFlagRadius(radius, hpChange int, ctrl *Controller) {
	for _, m := range c.Members {
		for _, f := range c.Flags {
			if ctrl.Amount <= 0 {
				return
			}
			if m.Pos.InRadius(f.Pos, radius) {
				m.Hp += hpChange
				ctrl.Amount--
			}
		}
	}
}

func (c *Colony) AssignedUndoneTasksCount() int {
	count := 0
	for _, t := range c.Tasks {
		if t.HasOwner() && !t.IsDone {
			count++
		}
	}
	return count
}

func (c *Colony) NewMaintainConnectionTask(pos Position, flowField *[]int16) *ColonyTask {
	return &ColonyTask{
		Type:      MaintainConnectionTask,
		Owner:     nil,
		ExpiresAt: CalcExpiresAt(),
		FlowField: flowField,
		Pos:       pos,
	}
}

func (c *Colony) NewConnectionTask(pos util.Position) *ColonyTask {
	return &ColonyTask{
		Pos:       pos,
		Type:      ConnectToPosTask,
		Owner:     nil,
		ExpiresAt: CalcExpiresAt(),
	}
}

func (c *Colony) NewBuildingTask(pos util.Position, buildType BuildType) *ColonyTask {
	return &ColonyTask{
		Pos:       pos,
		Type:      BuildingTask,
		BuildType: buildType,
		ExpiresAt: CalcRoleTaskExpiresAt(),
	}
}

func (c *Colony) NewFoodGatheringTask(pos util.Position) *ColonyTask {
	return &ColonyTask{
		Pos:       pos,
		Type:      FoodGatheringTask,
		ExpiresAt: CalcRoleTaskExpiresAt(),
	}
}

func (c *Colony) NewScoutTask(pos util.Position) *ColonyTask {
	return &ColonyTask{
		Pos:       pos,
		Type:      ScoutTask,
		ExpiresAt: CalcRoleTaskExpiresAt(),
	}
}

func (c *Colony) NewFarmingTask(pos util.Position) *ColonyTask {
	return &ColonyTask{
		Pos:       pos,
		Type:      FarmingTask,
		BuildType: BuildFarm,
		ExpiresAt: CalcRoleTaskExpiresAt(),
	}
}

func (c *Colony) SetPathToWater(path []util.Position) {
	c.PathToWater = path
	if len(path) == 0 {
		c.pathToWaterMask = nil
		c.pathToWaterIndex = nil
		return
	}
	if len(c.pathToWaterMask) != util.Cells {
		c.pathToWaterMask = make([]bool, util.Cells)
	} else {
		clear(c.pathToWaterMask)
	}
	if len(c.pathToWaterIndex) != util.Cells {
		c.pathToWaterIndex = make([]int, util.Cells)
	}
	for i := range c.pathToWaterIndex {
		c.pathToWaterIndex[i] = -1
	}
	for _, pos := range path {
		if util.OutOfBounds(pos) {
			continue
		}
		c.pathToWaterMask[util.Idx(pos)] = true
	}
	for pathIdx, pos := range path {
		if util.OutOfBounds(pos) {
			continue
		}
		c.pathToWaterIndex[util.Idx(pos)] = pathIdx
	}
}

func (c *Colony) IsPathToWater(pos util.Position) bool {
	if util.OutOfBounds(pos) || len(c.pathToWaterMask) == 0 {
		return false
	}
	return c.pathToWaterMask[util.Idx(pos)]
}

func (c *Colony) NextPathStep(pos, target util.Position) (util.Position, bool) {
	currIdx, ok := c.PathToWaterIndex(pos)
	if !ok {
		return util.Position{}, false
	}
	targetIdx, ok := c.PathToWaterIndex(target)
	if !ok || currIdx == targetIdx {
		return util.Position{}, false
	}
	if currIdx < targetIdx {
		return c.PathToWater[currIdx+1], true
	}
	return c.PathToWater[currIdx-1], true
}

func (c *Colony) PathToWaterIndex(pos util.Position) (int, bool) {
	if util.OutOfBounds(pos) || len(c.pathToWaterIndex) == 0 {
		return 0, false
	}
	idx := c.pathToWaterIndex[util.Idx(pos)]
	return idx, idx >= 0
}

func CalcExpiresAt() time.Time {
	return time.Now().Add(10 * time.Second)
}

func CalcRoleTaskExpiresAt() time.Time {
	return time.Now().Add(45 * time.Second)
}

func CalcTaskExpiresAt(taskType ColonyTaskType) time.Time {
	switch taskType {
	case BuildingTask, FoodGatheringTask, ScoutTask, FarmingTask:
		return CalcRoleTaskExpiresAt()
	default:
		return CalcExpiresAt()
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
	c.addFamily(b, map[*Bot]struct{}{})
}

func (c *Colony) addFamily(b *Bot, visited map[*Bot]struct{}) {
	if c == nil || b == nil {
		return
	}
	if _, ok := visited[b]; ok {
		return
	}
	visited[b] = struct{}{}
	if b.Colony != nil && b.Colony != c {
		b.Colony.RemoveMember(b)
	}
	b.Colony = c
	c.AddMember(b)
	for _, o := range sortedOffsprings(b.Offsprings) {
		c.addFamily(o, visited)
	}
}

func sortedOffsprings(offsprings map[*Bot]struct{}) []*Bot {
	if len(offsprings) == 0 {
		return nil
	}
	out := make([]*Bot, 0, len(offsprings))
	for offspring := range offsprings {
		out = append(out, offspring)
	}
	sort.Slice(out, func(i, j int) bool {
		left := out[i]
		right := out[j]
		if left == nil || right == nil {
			return right != nil
		}
		leftIdx := util.Idx(left.Pos)
		rightIdx := util.Idx(right.Pos)
		if leftIdx != rightIdx {
			return leftIdx < rightIdx
		}
		if left.LineageDepth != right.LineageDepth {
			return left.LineageDepth < right.LineageDepth
		}
		return left.Age < right.Age
	})
	return out
}

func (c *Colony) HasNoFreeMembers() bool {
	count := 0
	for _, m := range c.Members {
		if m.HasTask() {
			count++
		}
	}
	return count == 0
}

func (c *Colony) RemoveMember(m *Bot) {
	for i, b := range c.Members {
		if b == m {
			c.Members[i] = c.Members[len(c.Members)-1]
			c.Members = c.Members[:len(c.Members)-1]
			return
		}
	}
}

func (c *Colony) AddMember(m *Bot) {
	if c == nil || m == nil {
		return
	}
	for _, existing := range c.Members {
		if existing == m {
			return
		}
	}
	c.Members = append(c.Members, m)
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

const (
	ResourceMarker ColonyMarkerType = iota
	WaterMarker
)

type ColonyMarker struct {
	Pos  util.Position
	Type ColonyMarkerType
}

type ColonyMarkerType int

type ColonyTaskType int

const (
	FindWaterTask ColonyTaskType = iota
	ConnectToPosTask
	MaintainConnectionTask
	BuildingTask
	FoodGatheringTask
	ScoutTask
	FarmingTask
)

func (t ColonyTaskType) String() string {
	switch t {
	case BuildingTask:
		return "BuildingTask"
	case MaintainConnectionTask:
		return "MaintainConnectionTask"
	case ConnectToPosTask:
		return "ConnectToPosTask"
	case FindWaterTask:
		return "FindWaterTask"
	case FoodGatheringTask:
		return "FoodGatheringTask"
	case ScoutTask:
		return "ScoutTask"
	case FarmingTask:
		return "FarmingTask"
	}
	return "UnknownTaskType"
}

type ColonyTask struct {
	Type      ColonyTaskType
	Attempts  int
	Owner     *Bot
	IsDone    bool
	Pos       util.Position
	BuildType BuildType
	ExpiresAt time.Time
	FlowField *[]int16
}

func (t *ColonyTask) IsExpired(now time.Time) bool {
	return t.ExpiresAt.Before(now)
}

func (c *ColonyTask) HasOwner() bool {
	return c.Owner != nil
}

func (t *ColonyTask) MarkDone() {
	if t.IsDone {
		return
	}
	t.IsDone = true
	if t.Owner != nil {
		t.Owner.Evolution.TaskCompletions++
	}
}

type ColonyFlag struct {
	Pos util.Position
}
