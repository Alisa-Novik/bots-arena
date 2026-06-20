package game

import (
	"golab/internal/core"
	"golab/internal/util"
	"sort"
	"time"
)

const (
	colonyTaskScanRadius       = 26
	colonyTaskMaxAttempts      = 180
	colonyTaskMinScoutDistance = 18
)

func (g *Game) processColonyRoleTasks(colony *core.Colony, center core.Position) {
	if g == nil || g.Board == nil || colony == nil {
		return
	}
	colony.Counter++
	now := time.Now()
	g.pruneColonyRoleTasks(colony, now)

	connected := g.liveConnectedColonyMembers(colony)
	if len(connected) == 0 {
		return
	}

	g.ensureColonyFoodTasks(colony, center, min(10, max(2, len(connected)/5)))
	g.ensureColonyFarmingTasks(colony, center, min(6, max(1, len(connected)/10)))
	g.ensureColonyBuildingTasks(colony, center, min(6, max(1, len(connected)/12)))
	g.ensureColonyScoutTasks(colony, center, min(8, max(2, len(connected)/8)))
	g.assignColonyRoleTasks(colony, center, now)
}

func (g *Game) liveConnectedColonyMembers(colony *core.Colony) []*core.Bot {
	members := g.liveColonyMembers(colony)
	out := members[:0]
	for _, bot := range members {
		if bot != nil && bot.ConnnectedToColony {
			out = append(out, bot)
		}
	}
	return out
}

func (g *Game) pruneColonyRoleTasks(colony *core.Colony, now time.Time) {
	tasks := colony.Tasks[:0]
	for _, task := range colony.Tasks {
		if task == nil {
			continue
		}
		if !isColonyRoleTask(task) {
			tasks = append(tasks, task)
			continue
		}
		if task.IsDone || task.IsExpired(now) || !g.colonyRoleTaskStillValid(task) {
			g.releaseColonyTask(colony, task, false)
			continue
		}
		tasks = append(tasks, task)
	}
	colony.Tasks = tasks
}

func (g *Game) ensureColonyFoodTasks(colony *core.Colony, center core.Position, desired int) {
	need := desired - g.colonyRoleTaskCount(colony, core.FoodGatheringTask)
	if need <= 0 {
		return
	}
	for _, pos := range g.resourceTaskCandidates(center, colonyTaskScanRadius) {
		if need <= 0 {
			return
		}
		if g.hasSimilarColonyRoleTask(colony, core.FoodGatheringTask, pos, 1, 0) {
			continue
		}
		colony.AddTask(colony.NewFoodGatheringTask(pos))
		need--
	}
}

func (g *Game) ensureColonyFarmingTasks(colony *core.Colony, center core.Position, desired int) {
	need := desired - g.colonyRoleTaskCount(colony, core.FarmingTask)
	if need <= 0 {
		return
	}
	for _, pos := range g.farmingTaskCandidates(colony, center, colonyTaskScanRadius) {
		if need <= 0 {
			return
		}
		if g.hasSimilarColonyRoleTask(colony, core.FarmingTask, pos, 2, core.BuildFarm) {
			continue
		}
		colony.AddTask(colony.NewFarmingTask(pos))
		need--
	}
}

func (g *Game) ensureColonyBuildingTasks(colony *core.Colony, center core.Position, desired int) {
	need := desired - g.colonyRoleTaskCount(colony, core.BuildingTask)
	if need <= 0 {
		return
	}
	buildTypes := []core.BuildType{
		core.BuildColonyFlag,
		core.BuildDepot,
		core.BuildSpawner,
		core.BuildFarm,
	}
	for i := 0; need > 0 && i < desired*4; i++ {
		buildType := buildTypes[(colony.Counter+i)%len(buildTypes)]
		pos, ok := g.frontierBuildTaskCandidate(colony, center, i)
		if !ok {
			return
		}
		if g.hasSimilarColonyRoleTask(colony, core.BuildingTask, pos, 3, buildType) {
			continue
		}
		colony.AddTask(colony.NewBuildingTask(pos, buildType))
		need--
	}
}

func (g *Game) ensureColonyScoutTasks(colony *core.Colony, center core.Position, desired int) {
	need := desired - g.colonyRoleTaskCount(colony, core.ScoutTask)
	if need <= 0 {
		return
	}
	for i := 0; need > 0 && i < desired*6; i++ {
		pos, ok := g.scoutTaskCandidate(colony, center, i)
		if !ok {
			continue
		}
		if g.hasSimilarColonyRoleTask(colony, core.ScoutTask, pos, 6, 0) {
			continue
		}
		colony.AddTask(colony.NewScoutTask(pos))
		need--
	}
}

func (g *Game) assignColonyRoleTasks(colony *core.Colony, center core.Position, now time.Time) {
	freeBots := g.sortedFreeConnectedColonyBots(colony, center, now)
	used := make([]bool, len(freeBots))
	for _, task := range g.sortedOpenColonyRoleTasks(colony) {
		if task.HasOwner() {
			continue
		}
		bestIdx := -1
		bestDist := 0
		for i, bot := range freeBots {
			if used[i] || bot == nil || bot.HasTask() || bot.HasCooldown(now) {
				continue
			}
			dist := boardDistance(bot.Pos, task.Pos)
			if bestIdx < 0 || dist < bestDist {
				bestIdx = i
				bestDist = dist
			}
		}
		if bestIdx < 0 {
			return
		}
		freeBots[bestIdx].AssignTask(task)
		used[bestIdx] = true
	}
}

func (g *Game) sortedFreeConnectedColonyBots(colony *core.Colony, center core.Position, now time.Time) []*core.Bot {
	out := make([]*core.Bot, 0, len(colony.Members))
	for _, bot := range colony.Members {
		if bot == nil || bot.Colony != colony || !bot.ConnnectedToColony || bot.HasTask() || bot.HasCooldown(now) {
			continue
		}
		if g.Board.GetBot(bot.Pos) != bot {
			continue
		}
		out = append(out, bot)
	}
	sort.Slice(out, func(i, j int) bool {
		left := boardDistance(out[i].Pos, center)
		right := boardDistance(out[j].Pos, center)
		if left != right {
			return left > right
		}
		return util.Idx(out[i].Pos) < util.Idx(out[j].Pos)
	})
	return out
}

func (g *Game) sortedOpenColonyRoleTasks(colony *core.Colony) []*core.ColonyTask {
	out := make([]*core.ColonyTask, 0, len(colony.Tasks))
	for _, task := range colony.Tasks {
		if isColonyRoleTask(task) && !task.IsDone && !task.HasOwner() {
			out = append(out, task)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		left := colonyRolePriority(out[i].Type)
		right := colonyRolePriority(out[j].Type)
		if left != right {
			return left < right
		}
		return util.Idx(out[i].Pos) < util.Idx(out[j].Pos)
	})
	return out
}

func colonyRolePriority(taskType core.ColonyTaskType) int {
	switch taskType {
	case core.FoodGatheringTask:
		return 0
	case core.FarmingTask:
		return 1
	case core.BuildingTask:
		return 2
	case core.ScoutTask:
		return 3
	default:
		return 10
	}
}

func (g *Game) resourceTaskCandidates(center core.Position, radius int) []core.Position {
	out := make([]core.Position, 0, 16)
	for r := center.R - radius; r <= center.R+radius; r++ {
		if r < 1 || r >= core.Rows-1 {
			continue
		}
		for dc := -radius; dc <= radius; dc++ {
			pos := util.NewPos(r, center.C+dc)
			switch g.Board.At(pos).(type) {
			case core.Food, core.Resource:
				out = append(out, pos)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		left := boardDistance(center, out[i])
		right := boardDistance(center, out[j])
		if left != right {
			return left < right
		}
		return util.Idx(out[i]) < util.Idx(out[j])
	})
	return out
}

func (g *Game) farmingTaskCandidates(colony *core.Colony, center core.Position, radius int) []core.Position {
	out := make([]core.Position, 0, 16)
	for r := center.R - radius; r <= center.R+radius; r++ {
		if r < 1 || r >= core.Rows-1 {
			continue
		}
		for dc := -radius; dc <= radius; dc++ {
			pos := util.NewPos(r, center.C+dc)
			switch farm := g.Board.At(pos).(type) {
			case core.Farm:
				if farm.Colony == colony || (farm.Owner != nil && farm.Owner.Colony == colony) {
					out = append(out, pos)
				}
				continue
			}
			if g.Board.BiomeAt(pos) != core.BiomeFertile || !g.canPlaceFreeColonyInfrastructure(pos) {
				continue
			}
			if !g.hasColonyTissueOrMemberNear(colony, pos, 3) {
				continue
			}
			out = append(out, pos)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		leftFarm := g.Board.At(out[i]) != nil
		rightFarm := g.Board.At(out[j]) != nil
		if leftFarm != rightFarm {
			return leftFarm
		}
		left := boardDistance(center, out[i])
		right := boardDistance(center, out[j])
		if left != right {
			return left < right
		}
		return util.Idx(out[i]) < util.Idx(out[j])
	})
	return out
}

func (g *Game) frontierBuildTaskCandidate(colony *core.Colony, center core.Position, ordinal int) (core.Position, bool) {
	targetRadius := g.colonyNestRadius() + 4 + ordinal%10
	best := colonyMoveCandidate{}
	found := false
	for r := center.R - targetRadius; r <= center.R+targetRadius; r++ {
		if r < 1 || r >= core.Rows-1 {
			continue
		}
		for dc := -targetRadius; dc <= targetRadius; dc++ {
			if max(util.Abs(r-center.R), util.Abs(dc)) != targetRadius {
				continue
			}
			pos := util.NewPos(r, center.C+dc)
			if !g.canPlaceFreeColonyInfrastructure(pos) {
				continue
			}
			if !g.hasColonyTissueOrMemberNear(colony, pos, 4) {
				continue
			}
			home := int(g.ownedHomePheromone(colony, pos))
			adjacent := g.sameConnectedColonyNeighborCount(colony, pos, nil)
			candidate := colonyMoveCandidate{
				pos:   pos,
				score: home*8 + adjacent*1200 + g.colonyAnchorProximity(colony, pos)*500 - boardDistance(center, pos),
				index: util.Idx(pos),
			}
			if !found || colonyCandidateBefore(candidate, best) {
				best = candidate
				found = true
			}
		}
	}
	return best.pos, found
}

func (g *Game) scoutTaskCandidate(colony *core.Colony, center core.Position, ordinal int) (core.Position, bool) {
	dir := core.PosClock[(colony.Counter+ordinal)%len(core.PosClock)]
	radius := colonyTaskMinScoutDistance + (ordinal%5)*5
	base := center.AddRowCol(dir[1]*radius, dir[0]*radius)
	best := colonyMoveCandidate{}
	found := false
	for dr := -4; dr <= 4; dr++ {
		for dc := -4; dc <= 4; dc++ {
			pos := base.AddRowCol(dr, dc)
			if !core.Inside(pos) || g.Board.IsWall(pos) || g.Board.IsFrozen(pos) || g.Board.GetBot(pos) != nil {
				continue
			}
			if _, ok := g.Board.At(pos).(core.Poison); ok {
				continue
			}
			home := int(g.ownedHomePheromone(colony, pos))
			candidate := colonyMoveCandidate{
				pos:   pos,
				score: boardDistance(center, pos)*40 - home*5 - g.colonyStaticAnchorDistance(colony, pos),
				index: util.Idx(pos),
			}
			if !found || colonyCandidateBefore(candidate, best) {
				best = candidate
				found = true
			}
		}
	}
	return best.pos, found
}

func (g *Game) hasColonyTissueOrMemberNear(colony *core.Colony, center core.Position, radius int) bool {
	if colony == nil {
		return false
	}
	for r := center.R - radius; r <= center.R+radius; r++ {
		if r < 1 || r >= core.Rows-1 {
			continue
		}
		for dc := -radius; dc <= radius; dc++ {
			pos := util.NewPos(r, center.C+dc)
			if g.ownedHomePheromone(colony, pos) > 0 {
				return true
			}
			if bot := g.Board.GetBot(pos); bot != nil && bot.Colony == colony && bot.ConnnectedToColony {
				return true
			}
		}
	}
	return false
}

func (g *Game) hasSimilarColonyRoleTask(colony *core.Colony, taskType core.ColonyTaskType, pos core.Position, radius int, buildType core.BuildType) bool {
	for _, task := range colony.Tasks {
		if task == nil || task.IsDone || task.Type != taskType {
			continue
		}
		if taskType == core.BuildingTask && task.BuildType != buildType {
			continue
		}
		if boardDistance(task.Pos, pos) <= radius {
			return true
		}
	}
	return false
}

func (g *Game) colonyRoleTaskCount(colony *core.Colony, taskType core.ColonyTaskType) int {
	count := 0
	for _, task := range colony.Tasks {
		if task != nil && !task.IsDone && task.Type == taskType {
			count++
		}
	}
	return count
}

func (g *Game) colonyRoleTaskStillValid(task *core.ColonyTask) bool {
	if task.Attempts > colonyTaskMaxAttempts {
		return false
	}
	switch task.Type {
	case core.BuildingTask:
		return g.Board.IsEmpty(task.Pos)
	case core.FarmingTask:
		if _, ok := g.Board.At(task.Pos).(core.Farm); ok {
			return true
		}
		return g.Board.IsEmpty(task.Pos)
	case core.FoodGatheringTask:
		switch g.Board.At(task.Pos).(type) {
		case core.Food, core.Resource:
			return true
		default:
			return task.Attempts < colonyTaskMaxAttempts/2
		}
	case core.ScoutTask:
		return core.Inside(task.Pos) && !g.Board.IsFrozen(task.Pos)
	default:
		return true
	}
}

func isColonyRoleTask(task *core.ColonyTask) bool {
	if task == nil {
		return false
	}
	switch task.Type {
	case core.BuildingTask, core.FoodGatheringTask, core.ScoutTask, core.FarmingTask:
		return true
	default:
		return false
	}
}

func (g *Game) colonyTaskOpcode(pos core.Position, bot *core.Bot, op core.Opcode) (core.Opcode, bool) {
	if g == nil || g.Board == nil || bot == nil || !bot.HasUndoneTask() || !isColonyRoleTask(bot.CurrTask) {
		return op, false
	}
	task := bot.CurrTask
	if bot.Colony == nil || !bot.ConnnectedToColony {
		g.releaseColonyTask(bot.Colony, task, false)
		return op, false
	}
	if op == core.OpDivide {
		return op, false
	}

	switch task.Type {
	case core.BuildingTask:
		if g.roleTaskBuildSatisfied(task) {
			g.finishColonyTask(bot)
			return op, false
		}
		if roleTaskAdjacentToTarget(pos, task) {
			return core.OpBuild, true
		}
		return core.OpMove, true
	case core.FarmingTask:
		if farm, ok := g.Board.At(task.Pos).(core.Farm); ok {
			if g.farmFriendlyToBot(farm, bot) && roleTaskAdjacentToTarget(pos, task) {
				return core.OpGrab, true
			}
			if g.farmFriendlyToBot(farm, bot) {
				return core.OpMove, true
			}
			g.finishColonyTask(bot)
			return op, false
		}
		if roleTaskAdjacentToTarget(pos, task) && g.Board.IsEmpty(task.Pos) {
			return core.OpBuild, true
		}
		if !g.Board.IsEmpty(task.Pos) {
			g.finishColonyTask(bot)
			return op, false
		}
		return core.OpMove, true
	case core.FoodGatheringTask:
		if bot.Inventory.Total() > 0 {
			g.finishColonyTask(bot)
			return op, false
		}
		if _, ok := g.colonyTaskGrabTarget(pos, bot); ok {
			return core.OpGrab, true
		}
		return core.OpMove, true
	case core.ScoutTask:
		g.recordScoutSighting(pos, bot)
		if boardDistance(pos, task.Pos) <= 1 {
			g.finishColonyTask(bot)
			return op, false
		}
		return core.OpMove, true
	default:
		return op, false
	}
}

func (g *Game) nextColonyRoleTaskStep(oldPos core.Position, bot *core.Bot) (core.Position, bool) {
	if g == nil || g.Board == nil || bot == nil || !bot.HasUndoneTask() || !isColonyRoleTask(bot.CurrTask) {
		return core.Position{}, false
	}
	task := bot.CurrTask
	task.Attempts++
	if task.Attempts > colonyTaskMaxAttempts {
		g.releaseColonyTask(bot.Colony, task, false)
		return core.Position{}, false
	}

	if task.Type == core.ScoutTask {
		g.recordScoutSighting(oldPos, bot)
		if boardDistance(oldPos, task.Pos) <= 1 {
			g.finishColonyTask(bot)
			return core.Position{}, false
		}
	} else if roleTaskAdjacentToTarget(oldPos, task) {
		return core.Position{}, false
	}

	best := colonyMoveCandidate{}
	found := false
	currentDistance := boardDistance(oldPos, task.Pos)
	for _, dir := range core.PosClock {
		next := oldPos.AddDir(dir)
		if !g.canRoleTaskMoveInto(next, bot, task) {
			continue
		}
		distance := boardDistance(next, task.Pos)
		progress := currentDistance - distance
		home := int(g.ownedHomePheromone(bot.Colony, next))
		danger := int(g.sensePheromone(next, bot, core.PheromoneDanger))
		resource := int(g.sensePheromone(next, bot, core.PheromoneFood)) + int(g.sensePheromone(next, bot, core.PheromoneOre))
		score := progress*1200 - distance*100 - danger*45 + home*2
		switch task.Type {
		case core.FoodGatheringTask:
			score += resource * 8
		case core.ScoutTask:
			score -= home * 4
		default:
			score += g.sameConnectedColonyNeighborCount(bot.Colony, next, bot) * 35
		}
		candidate := colonyMoveCandidate{
			pos:   next,
			dir:   dir,
			score: score,
			index: util.Idx(next),
		}
		if !found || colonyCandidateBefore(candidate, best) {
			best = candidate
			found = true
		}
	}
	if !found {
		return core.Position{}, false
	}
	bot.Dir = best.dir
	return best.pos, true
}

func (g *Game) colonyTaskGrabTarget(pos core.Position, bot *core.Bot) (core.Position, bool) {
	if g == nil || g.Board == nil || bot == nil || !bot.HasUndoneTask() || !isColonyRoleTask(bot.CurrTask) {
		return core.Position{}, false
	}
	task := bot.CurrTask
	if roleTaskAdjacentToTarget(pos, task) && g.canGrabForColonyTask(task, bot, task.Pos) {
		return task.Pos, true
	}

	if task.Type != core.FoodGatheringTask {
		return core.Position{}, false
	}
	best := colonyMoveCandidate{}
	found := false
	for _, dir := range core.PosClock {
		next := pos.AddDir(dir)
		if !g.canGrabForColonyTask(task, bot, next) {
			continue
		}
		candidate := colonyMoveCandidate{
			pos:   next,
			dir:   dir,
			score: -boardDistance(next, task.Pos),
			index: util.Idx(next),
		}
		if !found || colonyCandidateBefore(candidate, best) {
			best = candidate
			found = true
		}
	}
	return best.pos, found
}

func (g *Game) colonyTaskBuildDirective(botPos core.Position, bot *core.Bot) (core.Position, core.BuildType, bool) {
	if g == nil || g.Board == nil || bot == nil || !bot.HasUndoneTask() || !isColonyRoleTask(bot.CurrTask) {
		return core.Position{}, 0, false
	}
	task := bot.CurrTask
	if !roleTaskAdjacentToTarget(botPos, task) || !g.Board.IsEmpty(task.Pos) || g.Board.IsFrozen(task.Pos) {
		return core.Position{}, 0, false
	}
	switch task.Type {
	case core.BuildingTask:
		return task.Pos, task.BuildType, true
	case core.FarmingTask:
		return task.Pos, core.BuildFarm, true
	default:
		return core.Position{}, 0, false
	}
}

func (g *Game) completeColonyTaskAfterPickup(bot *core.Bot, pickupPos core.Position) {
	if g == nil || bot == nil || !bot.HasUndoneTask() || !isColonyRoleTask(bot.CurrTask) {
		return
	}
	task := bot.CurrTask
	if task.Type == core.FoodGatheringTask && (bot.Inventory.Total() > 0 || pickupPos == task.Pos) {
		g.finishColonyTask(bot)
	}
}

func (g *Game) completeColonyTaskAfterGrab(bot *core.Bot) {
	if g == nil || bot == nil || !bot.HasUndoneTask() || !isColonyRoleTask(bot.CurrTask) {
		return
	}
	task := bot.CurrTask
	switch task.Type {
	case core.FoodGatheringTask:
		if bot.Inventory.Total() > 0 || !g.resourceTaskTargetPresent(task.Pos) {
			g.finishColonyTask(bot)
		}
	case core.FarmingTask:
		if _, ok := g.Board.At(task.Pos).(core.Farm); ok {
			g.finishColonyTask(bot)
		}
	}
}

func (g *Game) completeColonyTaskAfterMove(bot *core.Bot) {
	if g == nil || bot == nil || !bot.HasUndoneTask() || !isColonyRoleTask(bot.CurrTask) {
		return
	}
	if bot.CurrTask.Type == core.ScoutTask {
		g.recordScoutSighting(bot.Pos, bot)
		if boardDistance(bot.Pos, bot.CurrTask.Pos) <= 1 {
			g.finishColonyTask(bot)
		}
	}
}

func (g *Game) completeColonyTaskAfterBuild(bot *core.Bot) {
	if g == nil || bot == nil || !bot.HasUndoneTask() || !isColonyRoleTask(bot.CurrTask) {
		return
	}
	task := bot.CurrTask
	switch task.Type {
	case core.BuildingTask:
		if g.roleTaskBuildSatisfied(task) {
			g.finishColonyTask(bot)
		}
	case core.FarmingTask:
		if _, ok := g.Board.At(task.Pos).(core.Farm); ok {
			g.finishColonyTask(bot)
		}
	}
}

func (g *Game) finishColonyTask(bot *core.Bot) {
	if bot == nil || bot.CurrTask == nil {
		return
	}
	g.releaseColonyTask(bot.Colony, bot.CurrTask, true)
}

func (g *Game) releaseColonyTask(colony *core.Colony, task *core.ColonyTask, markDone bool) {
	if task == nil {
		return
	}
	if markDone {
		task.MarkDone()
	}
	owner := task.Owner
	if owner != nil && owner.CurrTask == task {
		owner.CurrTask = nil
		if colony == nil {
			colony = owner.Colony
		}
		if owner.Colony != nil && owner.Colony.AssignedTasksCount > 0 {
			owner.Colony.AssignedTasksCount--
		} else if colony != nil && colony.AssignedTasksCount > 0 {
			colony.AssignedTasksCount--
		}
		if !markDone {
			owner.StartCooldown(time.Now())
		}
		if g != nil && g.Board != nil {
			g.Board.MarkDirty(util.Idx(owner.Pos))
		}
	}
	task.Owner = nil
}

func (g *Game) canGrabForColonyTask(task *core.ColonyTask, bot *core.Bot, pos core.Position) bool {
	if g == nil || g.Board == nil || task == nil {
		return false
	}
	switch v := g.Board.At(pos).(type) {
	case core.Food, core.Resource:
		return task.Type == core.FoodGatheringTask
	case core.Farm:
		return task.Type == core.FarmingTask && g.farmFriendlyToBot(v, bot)
	default:
		return false
	}
}

func (g *Game) canRoleTaskMoveInto(pos core.Position, bot *core.Bot, task *core.ColonyTask) bool {
	if g == nil || g.Board == nil || bot == nil || task == nil {
		return false
	}
	if !core.Inside(pos) || g.Board.IsWall(pos) || g.Board.IsFrozen(pos) || g.Board.GetBot(pos) != nil {
		return false
	}
	if g.Board.IsEmpty(pos) {
		return true
	}
	switch g.Board.At(pos).(type) {
	case core.Food, core.Resource:
		return task.Type == core.FoodGatheringTask
	default:
		return false
	}
}

func (g *Game) roleTaskBuildSatisfied(task *core.ColonyTask) bool {
	if g == nil || g.Board == nil || task == nil {
		return false
	}
	switch task.Type {
	case core.BuildingTask:
		switch task.BuildType {
		case core.BuildFarm:
			_, ok := g.Board.At(task.Pos).(core.Farm)
			return ok
		case core.BuildSpawner:
			_, ok := g.Board.At(task.Pos).(core.Spawner)
			return ok
		case core.BuildColonyFlag:
			_, ok := g.Board.At(task.Pos).(core.ColonyFlag)
			return ok
		case core.BuildDepot:
			switch g.Board.At(task.Pos).(type) {
			case core.Depot, *core.Depot:
				return true
			default:
				return false
			}
		case core.BuildMine:
			_, ok := g.Board.At(task.Pos).(core.Mine)
			return ok
		default:
			_, ok := g.Board.At(task.Pos).(core.Building)
			return ok
		}
	case core.FarmingTask:
		_, ok := g.Board.At(task.Pos).(core.Farm)
		return ok
	default:
		return false
	}
}

func (g *Game) resourceTaskTargetPresent(pos core.Position) bool {
	if g == nil || g.Board == nil {
		return false
	}
	switch g.Board.At(pos).(type) {
	case core.Food, core.Resource:
		return true
	default:
		return false
	}
}

func roleTaskAdjacentToTarget(pos core.Position, task *core.ColonyTask) bool {
	return task != nil && boardDistance(pos, task.Pos) <= 1
}

func (g *Game) recordScoutSighting(pos core.Position, bot *core.Bot) {
	if g == nil || g.Board == nil || bot == nil || bot.Colony == nil {
		return
	}
	g.emitConnectedColonyHome(pos, bot)
	for dr := -3; dr <= 3; dr++ {
		for dc := -3; dc <= 3; dc++ {
			scanPos := pos.AddRowCol(dr, dc)
			if !core.Inside(scanPos) {
				continue
			}
			switch v := g.Board.At(scanPos).(type) {
			case core.Food:
				g.emitEventPheromone(scanPos, core.PheromoneFood)
			case core.Resource:
				g.emitEventPheromone(scanPos, core.PheromoneOre)
			case core.Poison:
				g.emitEventPheromone(scanPos, core.PheromoneDanger)
			case core.Water:
				if !bot.Colony.KnowsWaterGroupId(v.GroupId) {
					bot.Colony.AddWaterPosition(scanPos, v.GroupId)
				}
			}
		}
	}
}
