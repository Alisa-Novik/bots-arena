package game

import (
	"golab/internal/core"
	"golab/internal/util"
)

func sameConnectedColony(a, b *core.Bot) bool {
	return a != nil && b != nil &&
		a.Colony != nil &&
		a.Colony == b.Colony &&
		a.ConnnectedToColony &&
		b.ConnnectedToColony
}

func (g *Game) connectedColonyBotInOwnedNestOrTissue(pos core.Position, bot *core.Bot) bool {
	return bot != nil &&
		bot.Colony != nil &&
		bot.ConnnectedToColony &&
		(g.isColonyNestCell(bot.Colony, pos) || g.isOwnedColonyTissue(bot.Colony, pos))
}

func (g *Game) colonyFarmInitialAmount(builder *core.Bot) int {
	amount := g.config.FarmInitialAmount
	if builder != nil && builder.Colony != nil && builder.ConnnectedToColony {
		amount += max(0, g.config.ColonyFarmChargeBonus)
	}
	return amount
}

func (g *Game) colonyFarmChargeAmount(farm core.Farm, bot *core.Bot) int {
	amount := 1
	if bot != nil && bot.Colony != nil && bot.ConnnectedToColony && farm.Colony == bot.Colony {
		amount += max(0, g.config.ColonyFarmChargeBonus)
	}
	return amount
}

func (g *Game) farmFoodOutputs(pos core.Position, farm core.Farm) int {
	outputs := 1
	if g.Board.BiomeAt(pos) == core.BiomeFertile {
		outputs = 2
	}
	if farm.Colony != nil {
		outputs += max(0, g.config.ColonyFarmOutputBonus)
	}
	return outputs
}

func (g *Game) initializeColonySpawnerGenome(colony *core.Colony, founder *core.Bot) {
	if colony == nil || founder == nil {
		return
	}
	colony.SpawnerGenome = normalizedEvolutionGenome(founder.Genome)
	colony.HasSpawnerGenome = true
	colony.SpawnerGenomeScore = g.BotEvolutionScore(founder)
}

func (g *Game) refreshColonySpawnerGenome(colony *core.Colony) bool {
	if g == nil || g.Board == nil || colony == nil {
		return false
	}
	found := false
	bestScore := 0
	bestIdx := util.Cells
	var bestGenome core.Genome
	for _, member := range colony.Members {
		if member == nil || member.Colony != colony || !member.ConnnectedToColony {
			continue
		}
		if g.Board.GetBot(member.Pos) != member {
			continue
		}
		score := g.BotEvolutionScore(member)
		memberIdx := util.Idx(member.Pos)
		if !found || score > bestScore || (score == bestScore && memberIdx < bestIdx) {
			found = true
			bestScore = score
			bestIdx = memberIdx
			bestGenome = normalizedEvolutionGenome(member.Genome)
		}
	}
	if !found {
		return false
	}
	colony.SpawnerGenome = bestGenome
	colony.HasSpawnerGenome = true
	colony.SpawnerGenomeScore = bestScore
	return true
}

func (g *Game) spawnerChildGenome(spawner core.Spawner) (core.Genome, bool) {
	if spawner.Colony == nil || !spawner.Colony.HasSpawnerGenome {
		return core.Genome{}, false
	}
	return spawner.Colony.SpawnerGenome, true
}

func (g *Game) tryColonySpawnerAutoBirth(pos core.Position, spawner *core.Spawner) bool {
	if g == nil || g.Board == nil || g.config == nil || spawner == nil || spawner.Colony == nil || !spawner.AutoBirth {
		return false
	}
	period := g.config.ColonySpawnerBirthPeriod
	if period <= 0 || (g.logicTick+idx(pos))%period != 0 {
		return false
	}
	colony := spawner.Colony
	if limit := g.config.ColonySpawnerLocalLimit; limit > 0 && g.connectedColonyBotsNear(colony, pos, 2) >= limit {
		return false
	}
	if !colony.HasSpawnerGenome {
		g.refreshColonySpawnerGenome(colony)
	}
	genome, ok := g.spawnerChildGenome(*spawner)
	if !ok {
		return false
	}
	childPos, ok := g.findColonySpawnerBirthPos(pos, colony)
	if !ok {
		return false
	}

	parent := g.liveColonySpawnerParent(spawner)
	if parent != nil {
		child := parent.NewChildWithMutationRate(childPos, g.config.ShouldMutateColor, g.baseMutationRate())
		child.Genome = genome
		child.Colony = colony
		child.ConnnectedToColony = g.hasActiveColonySupportNear(colony, childPos, g.colonyNestRadius())
		parent.Evolution.SpawnerBirths++
		g.Board.AddBot(childPos, child)
		g.Board.MarkDirty(idx(parent.Pos))
	} else {
		child := core.NewBot(childPos)
		child.Genome = genome
		child.Colony = colony
		child.ConnnectedToColony = g.hasActiveColonySupportNear(colony, childPos, g.colonyNestRadius())
		child.Color = colony.Color
		child.PrevColor = child.Color
		colony.AddMember(&child)
		g.Board.AddBot(childPos, &child)
	}

	g.successfulDivisions++
	g.totalSpawnerBirths++
	g.emitConnectedColonyHome(childPos, g.Board.GetBot(childPos))
	g.emitEventPheromone(pos, core.PheromoneFood)
	return true
}

func (g *Game) connectedColonyBotsNear(colony *core.Colony, center core.Position, radius int) int {
	if g == nil || g.Board == nil || colony == nil || radius < 0 {
		return 0
	}
	count := 0
	for r := center.R - radius; r <= center.R+radius; r++ {
		if r < 0 || r >= core.Rows {
			continue
		}
		for dc := -radius; dc <= radius; dc++ {
			bot := g.Board.GetBot(util.NewPos(r, center.C+dc))
			if bot != nil && bot.Colony == colony && bot.ConnnectedToColony {
				count++
			}
		}
	}
	return count
}

func (g *Game) canFoundNewColony() bool {
	if g == nil || g.config == nil {
		return true
	}
	limit := g.config.ColonyMaxActive
	return limit <= 0 || g.activeColonyCount() < limit
}

func (g *Game) activeColonyCount() int {
	return len(g.activeColonies())
}

func (g *Game) activeColonies() map[*core.Colony]struct{} {
	active := map[*core.Colony]struct{}{}
	if g == nil || g.Board == nil {
		return active
	}
	for _, cell := range *g.Board.GetGrid() {
		switch v := cell.(type) {
		case core.Controller:
			if v.Colony != nil && g.controllerOwnerAlive(&v) {
				active[v.Colony] = struct{}{}
			}
		case *core.Controller:
			if v != nil && v.Colony != nil && g.controllerOwnerAlive(v) {
				active[v.Colony] = struct{}{}
			}
		}
	}
	for _, id := range g.Board.ActiveBotIDs() {
		bot := g.Board.BotByID(id)
		if bot != nil && bot.Colony != nil && bot.ConnnectedToColony {
			active[bot.Colony] = struct{}{}
		}
	}
	return active
}

func (g *Game) colonyHeartHpFloor(pos core.Position, bot *core.Bot) (int, bool) {
	if g == nil || g.config == nil || bot == nil || bot.Colony == nil || !bot.ConnnectedToColony {
		return 0, false
	}
	radius := max(0, g.config.ColonyHeartRadius)
	if radius <= 0 {
		return 0, false
	}
	dist, ok := g.colonyHeartDistance(pos, bot.Colony)
	if !ok || dist > radius {
		return 0, false
	}
	low := max(1, g.config.ColonyHeartMinHp)
	high := max(low, g.config.ColonyHeartMaxHp)
	floor := low
	if radius > 0 {
		floor += (high - low) * (radius - dist) / radius
	}
	return floor, true
}

func (g *Game) applyColonyHeartProtection(pos core.Position, bot *core.Bot) bool {
	floor, ok := g.colonyHeartHpFloor(pos, bot)
	if !ok {
		return false
	}
	if bot.Hp < floor {
		bot.Hp = floor
	}
	return true
}

func (g *Game) colonyHeartAgeProtected(pos core.Position, bot *core.Bot) bool {
	if g == nil || g.config == nil || bot == nil || bot.Colony == nil || !bot.ConnnectedToColony {
		return false
	}
	radius := max(0, g.config.ColonyHeartImmortalRadius)
	if radius <= 0 {
		return false
	}
	dist, ok := g.colonyHeartDistance(pos, bot.Colony)
	return ok && dist <= radius
}

func (g *Game) colonyHeartDistance(pos core.Position, colony *core.Colony) (int, bool) {
	if colony == nil {
		return 0, false
	}
	return boardDistance(pos, colony.Center), true
}

func (g *Game) findColonySpawnerBirthPos(center core.Position, colony *core.Colony) (core.Position, bool) {
	if colony == nil {
		return core.Position{}, false
	}
	found := false
	best := colonyMoveCandidate{}
	for _, dir := range core.PosClock {
		pos := center.AddDir(dir)
		if !g.canColonyCohesionMoveInto(pos) {
			continue
		}
		if !g.hasActiveColonySupportNear(colony, pos, g.colonyNestRadius()) {
			continue
		}
		candidate := colonyMoveCandidate{
			pos:   pos,
			dir:   dir,
			score: g.colonyAnchorProximity(colony, pos)*1000 + int(g.ownedHomePheromone(colony, pos))*4 - boardDistance(pos, colony.Center),
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

func (g *Game) liveColonySpawnerParent(spawner *core.Spawner) *core.Bot {
	if spawner == nil || spawner.Colony == nil {
		return nil
	}
	if owner := spawner.Owner; owner != nil && owner.Colony == spawner.Colony && g.Board.GetBot(owner.Pos) == owner {
		return owner
	}
	parent := g.liveColonyMember(spawner.Colony)
	if parent != nil {
		spawner.Owner = parent
	}
	return parent
}

func (g *Game) buildInitialColonyInfrastructure(center core.Position, founder *core.Bot, colony *core.Colony) {
	if g == nil || g.Board == nil || g.config == nil || founder == nil || colony == nil {
		return
	}
	radius := max(0, g.config.ColonyAutoWallRadius)
	if radius > 0 && g.config.ColonyAutoWallHp > 0 {
		for dr := -radius; dr <= radius; dr++ {
			for dc := -radius; dc <= radius; dc++ {
				if max(util.Abs(dr), util.Abs(dc)) != radius {
					continue
				}
				if colonyAutoWallGateCell(dr, dc, radius) {
					continue
				}
				pos := center.AddRowCol(dr, dc)
				if !g.canPlaceFreeColonyInfrastructure(pos) {
					continue
				}
				g.Board.Set(pos, core.Building{
					Pos:   pos,
					Owner: founder,
					Hp:    g.config.ColonyAutoWallHp,
				})
			}
		}
	}

	g.placeInitialColonySpawners(center, founder, colony, max(0, g.config.ColonyInitialSpawners), radius)
}

func colonyAutoWallGateCell(dr, dc, radius int) bool {
	if radius <= 0 {
		return false
	}
	if util.Abs(dr) == radius && util.Abs(dc) <= 1 {
		return true
	}
	return util.Abs(dc) == radius && util.Abs(dr) <= 1
}

func (g *Game) placeInitialColonySpawners(center core.Position, founder *core.Bot, colony *core.Colony, count, wallRadius int) int {
	if count <= 0 {
		return 0
	}
	limitRadius := wallRadius - 1
	if wallRadius <= 1 {
		limitRadius = 1
	}
	placed := 0
	for radius := 0; radius <= limitRadius && placed < count; radius++ {
		for dr := -radius; dr <= radius && placed < count; dr++ {
			for dc := -radius; dc <= radius && placed < count; dc++ {
				if max(util.Abs(dr), util.Abs(dc)) != radius {
					continue
				}
				pos := center.AddRowCol(dr, dc)
				if !g.canPlaceFreeColonyInfrastructure(pos) {
					continue
				}
				g.Board.Set(pos, core.Spawner{
					Pos:       pos,
					Owner:     founder,
					Colony:    colony,
					Amount:    max(0, g.config.SpawnerInitialAmount),
					AutoBirth: true,
				})
				placed++
			}
		}
	}
	return placed
}

func (g *Game) canPlaceFreeColonyInfrastructure(pos core.Position) bool {
	return core.Inside(pos) &&
		!g.Board.IsWall(pos) &&
		!g.Board.IsFrozen(pos) &&
		g.Board.IsEmpty(pos) &&
		g.Board.GetBot(pos) == nil
}
