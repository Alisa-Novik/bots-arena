package game

import (
	"golab/internal/core"
	"golab/internal/util"
)

type colonyMoveCandidate struct {
	pos   core.Position
	dir   core.Direction
	score int
	index int
}

func (g *Game) tryColonyCohesion(pos core.Position, bot *core.Bot) bool {
	if !g.colonyOrganismEnabled() || bot == nil || bot.Colony == nil || !bot.ConnnectedToColony {
		return false
	}
	if bot.MaintainingConn() || g.Board.IsFrozen(pos) || g.Board.GetBot(pos) != bot {
		return false
	}
	if bot.HasUndoneTask() && isColonyRoleTask(bot.CurrTask) {
		return false
	}

	inNest := g.isColonyNestCell(bot.Colony, pos)
	returning := g.shouldReturnToColonyNest(pos, bot)
	if !util.RollChance(clampChance(g.config.ColonyCohesionChance, 100)) {
		g.emitConnectedColonyHome(pos, bot)
		return false
	}

	if g.tryColonyOrganismDivision(pos, bot) {
		return true
	}

	if bot.Inventory.Total() == 0 {
		if g.tryColonyForage(pos, bot) {
			return true
		}
		if g.tryColonyFrontierExpansion(pos, bot) {
			return true
		}
	}

	if returning {
		candidate, ok := g.bestColonyHomeStep(pos, bot)
		g.emitConnectedColonyHome(pos, bot)
		if ok {
			return g.applyColonyMove(pos, candidate, bot)
		}
		return g.isOwnedColonyTissue(bot.Colony, pos)
	}

	g.emitConnectedColonyHome(pos, bot)
	if candidate, ok := g.bestAdjacentHomePheromoneStep(pos, bot); ok {
		return g.applyColonyMove(pos, candidate, bot)
	}
	if candidate, ok := g.bestColonyNestFillStep(pos, bot); ok {
		return g.applyColonyMove(pos, candidate, bot)
	}
	return inNest
}

func (g *Game) tryColonyForage(pos core.Position, bot *core.Bot) bool {
	if bot == nil || bot.Colony == nil || bot.Inventory.Total() > 0 {
		return false
	}
	if !util.RollChance(clampChance(g.config.ColonyForageChance, 100)) {
		return false
	}
	candidate, ok := g.bestColonyForageStep(pos, bot)
	if !ok {
		return false
	}
	return g.applyColonyDirectedStep(pos, candidate, bot)
}

func (g *Game) tryColonyFrontierExpansion(pos core.Position, bot *core.Bot) bool {
	if bot == nil || bot.Colony == nil || bot.Inventory.Total() > 0 {
		return false
	}
	if !util.RollChance(clampChance(g.config.ColonyFrontierChance, 100)) {
		return false
	}
	if g.colonyFrontierPressure(pos, bot) < 3 {
		return false
	}
	candidate, ok := g.bestColonyFrontierStep(pos, bot)
	if !ok {
		return false
	}
	return g.applyColonyMove(pos, candidate, bot)
}

func (g *Game) tryColonyOrganismDivision(parentPos core.Position, bot *core.Bot) bool {
	if bot == nil || bot.Colony == nil || !bot.ConnnectedToColony {
		return false
	}
	if bot.Hp < g.divisionThreshold(bot) || !g.canPayShared(bot, g.config.DivisionFoodCost, g.config.DivisionOreCost) {
		return false
	}
	if !g.isColonyNestCell(bot.Colony, parentPos) && !g.isOwnedColonyTissue(bot.Colony, parentPos) {
		return false
	}
	childPos, ok := g.findColonyDivisionChildPos(parentPos, bot)
	if !ok {
		return false
	}
	if !g.spendShared(bot, g.config.DivisionFoodCost, g.config.DivisionOreCost) {
		return false
	}

	bot.Divisions++
	bot.Evolution.SuccessfulDivisions++
	child := bot.NewChildWithMutationRate(childPos, g.config.ShouldMutateColor, g.baseMutationRate())
	g.inheritColonyConnection(bot, child)
	bot.Hp -= g.config.DivisionCost
	g.Board.AddBot(childPos, child)
	g.successfulDivisions++
	g.emitConnectedColonyHome(childPos, child)
	g.Board.MarkDirty(util.Idx(parentPos))
	return true
}

func (g *Game) colonyOrganismEnabled() bool {
	return g != nil && g.config != nil && g.config.ColonyOrganismEnabled && g.Board != nil
}

func (g *Game) emitConnectedColonyHome(pos core.Position, bot *core.Bot) bool {
	if bot == nil || bot.Colony == nil || !bot.ConnnectedToColony {
		return false
	}
	return g.emitHomePheromone(pos, bot.Colony, max(1, g.config.ColonyMemberHomeDeposit))
}

func (g *Game) shouldReturnToColonyNest(pos core.Position, bot *core.Bot) bool {
	if bot == nil || bot.Colony == nil {
		return false
	}
	if bot.Inventory.Total() > 0 {
		return true
	}
	return !g.isColonyNestCell(bot.Colony, pos)
}

func (g *Game) bestColonyHomeStep(pos core.Position, bot *core.Bot) (colonyMoveCandidate, bool) {
	colony := bot.Colony
	currentAnchorDistance := g.colonyStaticAnchorDistance(colony, pos)
	best := colonyMoveCandidate{}
	found := false
	for _, dir := range core.PosClock {
		next := pos.AddDir(dir)
		if !g.canColonyCohesionMoveInto(next) {
			continue
		}
		nextAnchorDistance := g.colonyStaticAnchorDistance(colony, next)
		home := int(g.ownedHomePheromone(colony, next))
		if home <= 0 && nextAnchorDistance >= currentAnchorDistance && !g.isColonyNestCell(colony, next) {
			continue
		}
		candidate := colonyMoveCandidate{
			pos:   next,
			dir:   dir,
			score: g.colonyReturnScore(colony, next, bot),
			index: util.Idx(next),
		}
		if !found || colonyCandidateBefore(candidate, best) {
			best = candidate
			found = true
		}
	}
	if !found {
		return colonyMoveCandidate{}, false
	}
	return best, true
}

func (g *Game) bestAdjacentHomePheromoneStep(pos core.Position, bot *core.Bot) (colonyMoveCandidate, bool) {
	if bot == nil {
		return colonyMoveCandidate{}, false
	}
	threshold := g.colonyHomeFollowThreshold()
	if threshold <= 0 {
		return colonyMoveCandidate{}, false
	}
	found := false
	best := colonyMoveCandidate{}
	for _, dir := range core.PosClock {
		next := pos.AddDir(dir)
		if !g.canColonyCohesionMoveInto(next) {
			continue
		}
		home := int(g.ownedHomePheromone(bot.Colony, next))
		if home < threshold {
			continue
		}
		candidate := colonyMoveCandidate{
			pos:   next,
			dir:   dir,
			score: home,
			index: util.Idx(next),
		}
		if !found || colonyCandidateBefore(candidate, best) {
			best = candidate
			found = true
		}
	}
	return best, found
}

func (g *Game) bestColonyNestFillStep(pos core.Position, bot *core.Bot) (colonyMoveCandidate, bool) {
	colony := bot.Colony
	if !g.isColonyNestCell(colony, pos) {
		return colonyMoveCandidate{}, false
	}
	current := colonyMoveCandidate{
		pos:   pos,
		score: g.colonyNestFillScore(colony, pos, bot),
		index: util.Idx(pos),
	}
	best := current
	found := false
	for _, dir := range core.PosClock {
		next := pos.AddDir(dir)
		if !g.canColonyCohesionMoveInto(next) || !g.isColonyNestCell(colony, next) {
			continue
		}
		candidate := colonyMoveCandidate{
			pos:   next,
			dir:   dir,
			score: g.colonyNestFillScore(colony, next, bot),
			index: util.Idx(next),
		}
		if !found || colonyCandidateBefore(candidate, best) {
			best = candidate
			found = true
		}
	}
	if !found || !colonyCandidateBefore(best, current) {
		return colonyMoveCandidate{}, false
	}
	return best, true
}

func (g *Game) bestColonyForageStep(pos core.Position, bot *core.Bot) (colonyMoveCandidate, bool) {
	if bot == nil || bot.Colony == nil {
		return colonyMoveCandidate{}, false
	}
	threshold := max(4, g.config.PheromoneSenseThreshold/2)
	found := false
	best := colonyMoveCandidate{}
	for _, dir := range core.PosClock {
		next := pos.AddDir(dir)
		if !g.canColonyForageInto(next) {
			continue
		}
		food := int(g.sensePheromone(next, bot, core.PheromoneFood))
		ore := int(g.sensePheromone(next, bot, core.PheromoneOre))
		danger := int(g.sensePheromone(next, bot, core.PheromoneDanger))
		resource := max(food, ore)
		cellBonus := g.colonyForageCellBonus(next)
		if cellBonus == 0 && resource < threshold {
			continue
		}
		if danger > resource+cellBonus/200+32 {
			continue
		}
		adjacent := g.sameConnectedColonyNeighborCount(bot.Colony, next, bot)
		candidate := colonyMoveCandidate{
			pos:   next,
			dir:   dir,
			score: cellBonus + food*34 + ore*28 + adjacent*80 - danger*60 - g.colonyStaticAnchorDistance(bot.Colony, next)*3,
			index: util.Idx(next),
		}
		if !found || colonyCandidateBefore(candidate, best) {
			best = candidate
			found = true
		}
	}
	return best, found
}

func (g *Game) bestColonyFrontierStep(pos core.Position, bot *core.Bot) (colonyMoveCandidate, bool) {
	if bot == nil || bot.Colony == nil {
		return colonyMoveCandidate{}, false
	}
	colony := bot.Colony
	currentAnchorDistance := g.colonyStaticAnchorDistance(colony, pos)
	currentHome := int(g.ownedHomePheromone(colony, pos))
	frontierLimit := g.colonyNestRadius() + max(4, g.colonyNestRadius()/2)
	found := false
	best := colonyMoveCandidate{}
	for _, dir := range core.PosClock {
		next := pos.AddDir(dir)
		if !g.canColonyCohesionMoveInto(next) {
			continue
		}
		adjacent := g.sameConnectedColonyNeighborCount(colony, next, bot)
		if adjacent == 0 {
			continue
		}
		nextAnchorDistance := g.colonyStaticAnchorDistance(colony, next)
		home := int(g.ownedHomePheromone(colony, next))
		resource := int(g.sensePheromone(next, bot, core.PheromoneFood)) + int(g.sensePheromone(next, bot, core.PheromoneOre))
		danger := int(g.sensePheromone(next, bot, core.PheromoneDanger))
		if danger > 0 && resource == 0 {
			continue
		}
		if nextAnchorDistance > frontierLimit && resource < g.config.PheromoneSenseThreshold {
			continue
		}
		outward := nextAnchorDistance - currentAnchorDistance
		if outward < 0 && resource == 0 {
			continue
		}
		surface := currentHome - home
		candidate := colonyMoveCandidate{
			pos: next,
			dir: dir,
			score: outward*1100 +
				max(0, surface)*18 +
				adjacent*460 +
				resource*32 -
				danger*90 -
				max(0, nextAnchorDistance-frontierLimit)*1800,
			index: util.Idx(next),
		}
		if !found || colonyCandidateBefore(candidate, best) {
			best = candidate
			found = true
		}
	}
	return best, found
}

func (g *Game) canColonyForageInto(pos core.Position) bool {
	if !core.Inside(pos) || g.Board.IsFrozen(pos) || g.Board.GetBot(pos) != nil {
		return false
	}
	if g.Board.IsEmpty(pos) {
		return true
	}
	switch g.Board.At(pos).(type) {
	case core.Food, core.Resource:
		return true
	default:
		return false
	}
}

func (g *Game) colonyForageCellBonus(pos core.Position) int {
	switch v := g.Board.At(pos).(type) {
	case core.Food:
		return 5200 + max(1, v.Amount)*900
	case core.Resource:
		return 4700 + max(1, v.Amount)*120
	default:
		return 0
	}
}

func (g *Game) colonyFrontierPressure(pos core.Position, bot *core.Bot) int {
	if bot == nil || bot.Colony == nil {
		return 0
	}
	adjacent := g.sameConnectedColonyNeighborCount(bot.Colony, pos, bot)
	local := g.connectedColonyBotsNear(bot.Colony, pos, 2)
	return adjacent*2 + max(0, local-1)
}

func (g *Game) applyColonyDirectedStep(oldPos core.Position, candidate colonyMoveCandidate, bot *core.Bot) bool {
	if bot == nil {
		return false
	}
	oldBotPos := bot.Pos
	oldInventory := bot.Inventory
	bot.Dir = candidate.dir
	g.tryMove(oldPos, bot)
	if bot.Pos == oldBotPos && bot.Inventory == oldInventory {
		return false
	}
	g.emitConnectedColonyHome(bot.Pos, bot)
	return true
}

func (g *Game) findDivisionChildPosAround(center core.Position, bot *core.Bot) (core.Position, bool) {
	if g.colonyOrganismEnabled() && bot != nil && bot.Colony != nil && bot.ConnnectedToColony {
		if pos, ok := g.findColonyDivisionChildPos(center, bot); ok {
			return pos, true
		}
	}
	pos, ok := g.Board.FindEmptyPosAround(center)
	if ok && !g.Board.IsFrozen(pos) {
		return pos, true
	}
	return g.findEmptyUnfrozenPosAround(center)
}

func (g *Game) findColonyDivisionChildPos(center core.Position, bot *core.Bot) (core.Position, bool) {
	colony := bot.Colony
	found := false
	best := colonyMoveCandidate{}
	for _, dir := range core.PosClock {
		pos := center.AddDir(dir)
		if !g.canColonyCohesionMoveInto(pos) {
			continue
		}
		anchorProximity := g.colonyAnchorProximity(colony, pos)
		adjacent := g.sameConnectedColonyNeighborCount(colony, pos, nil)
		home := int(g.ownedHomePheromone(colony, pos))
		if anchorProximity == 0 && adjacent == 0 && home == 0 {
			continue
		}
		candidate := colonyMoveCandidate{
			pos:   pos,
			dir:   dir,
			score: adjacent*1600 + home*7 + anchorProximity*100 - boardDistance(pos, colony.Center),
			index: util.Idx(pos),
		}
		if !found || colonyCandidateBefore(candidate, best) {
			best = candidate
			found = true
		}
	}
	if !found {
		return core.Position{}, false
	}
	return best.pos, true
}

func colonyCandidateBefore(a, b colonyMoveCandidate) bool {
	if a.score != b.score {
		return a.score > b.score
	}
	return a.index < b.index
}

func (g *Game) applyColonyMove(oldPos core.Position, candidate colonyMoveCandidate, bot *core.Bot) bool {
	if bot == nil || !g.canColonyCohesionMoveInto(candidate.pos) {
		return false
	}
	bot.Dir = candidate.dir
	if !g.Board.MoveBot(oldPos, candidate.pos, bot) {
		return false
	}
	g.emitConnectedColonyHome(candidate.pos, bot)
	return true
}

func (g *Game) canColonyCohesionMoveInto(pos core.Position) bool {
	return core.Inside(pos) && !g.Board.IsFrozen(pos) && g.Board.IsEmpty(pos) && g.Board.GetBot(pos) == nil
}

func (g *Game) colonyReturnScore(colony *core.Colony, pos core.Position, moving *core.Bot) int {
	home := int(g.ownedHomePheromone(colony, pos))
	anchor := g.colonyAnchorProximity(colony, pos)
	adjacent := g.sameConnectedColonyNeighborCount(colony, pos, moving)
	anchorDistance := g.colonyStaticAnchorDistance(colony, pos)
	return home*30 + anchor*1500 + adjacent*400 - anchorDistance*2000
}

func (g *Game) colonyNestFillScore(colony *core.Colony, pos core.Position, moving *core.Bot) int {
	adjacent := g.sameConnectedColonyNeighborCount(colony, pos, moving)
	home := int(g.ownedHomePheromone(colony, pos))
	anchor := g.colonyAnchorProximity(colony, pos)
	centerDistance := boardDistance(pos, colony.Center)
	return adjacent*2000 + home/8 + anchor*120 - centerDistance
}

func (g *Game) sameConnectedColonyNeighborCount(colony *core.Colony, pos core.Position, moving *core.Bot) int {
	if colony == nil {
		return 0
	}
	count := 0
	for _, dir := range core.PosClock {
		neighbor := g.Board.GetBot(pos.AddDir(dir))
		if neighbor == nil || neighbor == moving {
			continue
		}
		if neighbor.Colony == colony && neighbor.ConnnectedToColony {
			count++
		}
	}
	return count
}

func (g *Game) isOwnedColonyTissue(colony *core.Colony, pos core.Position) bool {
	return colony != nil && g.ownedHomePheromone(colony, pos) > 0
}

func (g *Game) ownedHomePheromone(colony *core.Colony, pos core.Position) uint8 {
	if !g.pheromonesEnabled() || colony == nil || g.Board.PheromoneHomeOwnerAt(pos) != colony {
		return 0
	}
	return g.Board.PheromoneAt(pos).Home
}

func (g *Game) isColonyNestCell(colony *core.Colony, pos core.Position) bool {
	if colony == nil {
		return false
	}
	return g.nearestColonyAnchorDistance(colony, pos, g.colonyNestRadius()) <= g.colonyNestRadius()
}

func (g *Game) colonyStaticAnchorDistance(colony *core.Colony, pos core.Position) int {
	if colony == nil {
		return core.Rows + core.Cols
	}
	best := boardDistance(pos, colony.Center)
	for _, flag := range colony.Flags {
		if flag == nil {
			continue
		}
		if dist := boardDistance(pos, flag.Pos); dist < best {
			best = dist
		}
	}
	return best
}

func (g *Game) colonyAnchorProximity(colony *core.Colony, pos core.Position) int {
	radius := g.colonyNestRadius()
	distance := g.nearestColonyAnchorDistance(colony, pos, radius)
	if distance > radius {
		return 0
	}
	return radius - distance + 1
}

func (g *Game) nearestColonyAnchorDistance(colony *core.Colony, pos core.Position, radius int) int {
	if colony == nil || radius < 0 {
		return radius + 1
	}
	best := boardDistance(pos, colony.Center)
	for _, flag := range colony.Flags {
		if flag == nil {
			continue
		}
		if dist := boardDistance(pos, flag.Pos); dist < best {
			best = dist
		}
	}
	if best > radius {
		return radius + 1
	}
	return best
}

func (g *Game) isColonyAnchorAt(colony *core.Colony, pos core.Position) bool {
	if colony == nil || g == nil || g.Board == nil {
		return false
	}
	if g.colonyHasFlagAt(colony, pos) {
		return true
	}
	switch cell := g.Board.At(pos).(type) {
	case core.Controller:
		return cell.Colony == colony
	case *core.Controller:
		return cell != nil && cell.Colony == colony
	case core.Depot:
		return cell.Colony == colony
	case *core.Depot:
		return cell != nil && cell.Colony == colony
	case core.Farm:
		return cell.Colony == colony || (cell.Owner != nil && cell.Owner.Colony == colony && g.farmOwnerAlive(cell.Owner))
	case core.Spawner:
		return cell.Owner != nil && cell.Owner.Colony == colony && g.spawnerOwnerAlive(cell.Owner)
	default:
		return false
	}
}

func (g *Game) colonyHasFlagAt(colony *core.Colony, pos core.Position) bool {
	if colony == nil {
		return false
	}
	for _, flag := range colony.Flags {
		if flag != nil && flag.Pos == pos {
			return true
		}
	}
	return false
}

func (g *Game) colonyNestRadius() int {
	if g == nil || g.config == nil || g.config.ColonyNestRadius <= 0 {
		return controllerRecruitRadius
	}
	return g.config.ColonyNestRadius
}

func (g *Game) colonyHomeFollowThreshold() int {
	if g == nil || g.config == nil || g.config.ColonyHomeFollowThreshold < 0 {
		return 0
	}
	return g.config.ColonyHomeFollowThreshold
}
