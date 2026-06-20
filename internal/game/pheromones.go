package game

import (
	"golab/internal/core"
	"golab/internal/util"
)

func (g *Game) updatePheromones() {
	if !g.pheromonesEnabled() {
		return
	}
	if g.config.PheromoneDecayPeriod > 0 &&
		g.config.PheromoneDecay > 0 &&
		g.logicTick%g.config.PheromoneDecayPeriod == 0 {
		g.Board.DecayPheromones(g.config.PheromoneDecay)
	}
	if g.config.PheromoneDiffusePeriod > 0 &&
		g.config.PheromoneDiffuseAmount > 0 &&
		g.logicTick%g.config.PheromoneDiffusePeriod == 0 {
		g.Board.DiffusePheromones(g.config.PheromoneDiffuseAmount)
	}
}

func (g *Game) emitEventPheromone(pos core.Position, channel core.PheromoneChannel) bool {
	if !g.pheromonesEnabled() {
		return false
	}
	return g.Board.DepositPheromone(pos, channel, g.config.PheromoneEventDeposit, nil)
}

func (g *Game) emitHomePheromone(pos core.Position, colony *core.Colony, amount int) bool {
	if !g.pheromonesEnabled() || colony == nil {
		return false
	}
	return g.Board.DepositPheromone(pos, core.PheromoneHome, amount, colony)
}

func (g *Game) emitBotPheromone(pos core.Position, bot *core.Bot, channel core.PheromoneChannel) bool {
	if !g.pheromonesEnabled() || bot == nil {
		return false
	}
	var owner *core.Colony
	if channel == core.PheromoneHome {
		owner = bot.Colony
	}
	return g.Board.DepositPheromone(pos, channel, g.config.PheromoneBotDeposit, owner)
}

func (g *Game) emitControllerHomePheromones(ctrl *core.Controller, pos core.Position) {
	if !g.pheromonesEnabled() || ctrl == nil || ctrl.Colony == nil {
		return
	}
	homeDeposit := g.config.PheromoneHomeDeposit
	if homeDeposit <= 0 {
		return
	}
	g.emitHomePheromone(pos, ctrl.Colony, homeDeposit)

	lowDeposit := max(1, homeDeposit/2)
	for _, flag := range ctrl.Colony.Flags {
		if flag != nil {
			g.emitHomePheromone(flag.Pos, ctrl.Colony, lowDeposit)
		}
	}
	memberDeposit := max(1, homeDeposit/3)
	for _, member := range ctrl.Colony.Members {
		if member == nil || !member.ConnnectedToColony || member.Colony != ctrl.Colony {
			continue
		}
		if g.Board.GetBot(member.Pos) != member {
			continue
		}
		g.emitHomePheromone(member.Pos, ctrl.Colony, memberDeposit)
	}
}

func (g *Game) pheromonesEnabled() bool {
	return g != nil && g.config != nil && g.config.PheromonesEnabled && g.Board != nil
}

func (g *Game) sensePheromone(pos core.Position, bot *core.Bot, channel core.PheromoneChannel) uint8 {
	if !g.pheromonesEnabled() {
		return 0
	}
	return g.Board.PheromoneValueForBot(pos, channel, bot)
}

func (g *Game) selectPheromoneDirection(pos core.Position, bot *core.Bot, channel core.PheromoneChannel, avoid bool) (core.Direction, uint8, bool) {
	if !g.pheromonesEnabled() {
		return core.Direction{}, 0, false
	}
	bestDir := core.Direction{}
	bestValue := uint8(0)
	found := false
	for _, dir := range core.PosClock {
		next := pos.AddDir(dir)
		if !g.canFollowPheromoneInto(next) {
			continue
		}
		value := g.sensePheromone(next, bot, channel)
		if !found ||
			(!avoid && value > bestValue) ||
			(avoid && value < bestValue) {
			bestDir = dir
			bestValue = value
			found = true
		}
	}
	return bestDir, bestValue, found
}

func (g *Game) canFollowPheromoneInto(pos core.Position) bool {
	if !core.Inside(pos) || g.Board.IsFrozen(pos) {
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

func pheromoneInspectLine(brd *core.Board, pos util.Position) string {
	if brd == nil {
		return core.PheromoneValues{}.InspectString()
	}
	return brd.PheromoneAt(pos).InspectString()
}
