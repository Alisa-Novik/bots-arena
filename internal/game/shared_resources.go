package game

import (
	"golab/internal/core"
	"golab/internal/util"
	"sort"
)

type sharedDepotRef struct {
	pos      core.Position
	depot    core.Depot
	distance int
	index    int
}

func (g *Game) accessibleInventory(bot *core.Bot) core.Inventory {
	if bot == nil {
		return core.Inventory{}
	}
	out := bot.Inventory
	if !g.canUseSharedResources(bot) {
		return out
	}
	radius := g.sharedAccessRadius()
	for _, ref := range g.nearbySameColonyDepots(bot.Colony, bot.Pos, radius) {
		out.Food += max(0, ref.depot.Food)
		out.Ore += max(0, ref.depot.Ore)
	}
	if g.hasNearbySameColonyController(bot.Colony, bot.Pos, radius) {
		out.Food += bot.Colony.FoodBank
		out.Ore += bot.Colony.OreBank
	}
	return out
}

func (g *Game) canPayShared(bot *core.Bot, food, ore int) bool {
	return g.accessibleInventory(bot).CanPay(food, ore)
}

func (g *Game) spendShared(bot *core.Bot, food, ore int) bool {
	if bot == nil {
		return false
	}
	food = max(0, food)
	ore = max(0, ore)
	if !g.canPayShared(bot, food, ore) {
		return false
	}

	foodFromInventory := min(bot.Inventory.Food, food)
	bot.Inventory.Food -= foodFromInventory
	food -= foodFromInventory

	oreFromInventory := min(bot.Inventory.Ore, ore)
	bot.Inventory.Ore -= oreFromInventory
	ore -= oreFromInventory

	if food == 0 && ore == 0 {
		return true
	}
	if !g.canUseSharedResources(bot) {
		return food == 0 && ore == 0
	}

	for _, ref := range g.nearbySameColonyDepots(bot.Colony, bot.Pos, g.sharedAccessRadius()) {
		changed := false
		if food > 0 {
			spent := min(max(0, ref.depot.Food), food)
			ref.depot.Food -= spent
			food -= spent
			changed = changed || spent > 0
		}
		if ore > 0 {
			spent := min(max(0, ref.depot.Ore), ore)
			ref.depot.Ore -= spent
			ore -= spent
			changed = changed || spent > 0
		}
		if changed {
			g.Board.Set(ref.pos, ref.depot)
		}
		if food == 0 && ore == 0 {
			return true
		}
	}

	if (food > 0 || ore > 0) && g.hasNearbySameColonyController(bot.Colony, bot.Pos, g.sharedAccessRadius()) {
		if !bot.Colony.SpendBank(food, ore) {
			return false
		}
		food = 0
		ore = 0
	}
	return food == 0 && ore == 0
}

func (g *Game) depositConnectedMemberSurplusToBank(colony *core.Colony, center core.Position) {
	if colony == nil {
		return
	}
	g.forEachLiveBotInRadius(center, g.sharedAccessRadius(), func(bot *core.Bot) bool {
		if bot.Colony != colony || !bot.ConnnectedToColony {
			return true
		}
		foodSurplus := max(0, bot.Inventory.Food-max(0, g.config.DivisionFoodCost))
		oreSurplus := max(0, bot.Inventory.Ore-max(0, g.config.DivisionOreCost))
		if foodSurplus == 0 && oreSurplus == 0 {
			return true
		}
		bot.Inventory.Food -= foodSurplus
		bot.Inventory.Ore -= oreSurplus
		colony.Deposit(foodSurplus, oreSurplus)
		g.Board.MarkDirty(util.Idx(bot.Pos))
		return true
	})
}

func (g *Game) depositConnectedMemberSurplusToDepot(depot *core.Depot, center core.Position) bool {
	if depot == nil || depot.Colony == nil {
		return false
	}
	foodCapacity := max(0, g.config.DepotFoodCapacity)
	oreCapacity := max(0, g.config.DepotOreCapacity)
	foodRoom := max(0, foodCapacity-depot.Food)
	oreRoom := max(0, oreCapacity-depot.Ore)
	if foodRoom == 0 && oreRoom == 0 {
		return false
	}

	changed := false
	g.forEachLiveBotInRadius(center, g.sharedAccessRadius(), func(bot *core.Bot) bool {
		if bot.Colony != depot.Colony || !bot.ConnnectedToColony {
			return true
		}
		if foodRoom == 0 && oreRoom == 0 {
			return false
		}
		foodSurplus := max(0, bot.Inventory.Food-max(0, g.config.DivisionFoodCost))
		oreSurplus := max(0, bot.Inventory.Ore-max(0, g.config.DivisionOreCost))
		foodDeposit := min(foodSurplus, foodRoom)
		oreDeposit := min(oreSurplus, oreRoom)
		if foodDeposit == 0 && oreDeposit == 0 {
			return true
		}
		bot.Inventory.Food -= foodDeposit
		bot.Inventory.Ore -= oreDeposit
		depot.Food += foodDeposit
		depot.Ore += oreDeposit
		foodRoom -= foodDeposit
		oreRoom -= oreDeposit
		bot.RecordDepotDeposit(foodDeposit, oreDeposit)
		g.Board.MarkDirty(util.Idx(bot.Pos))
		changed = true
		return true
	})
	return changed
}

func (g *Game) canUseSharedResources(bot *core.Bot) bool {
	return g != nil && g.Board != nil && bot != nil && bot.Colony != nil && bot.ConnnectedToColony
}

func (g *Game) sharedAccessRadius() int {
	if g == nil || g.config == nil {
		return 0
	}
	return max(0, g.config.DepotAccessRadius)
}

func (g *Game) nearbySameColonyDepots(colony *core.Colony, center core.Position, radius int) []sharedDepotRef {
	if g == nil || g.Board == nil || colony == nil || radius < 0 {
		return nil
	}
	refs := make([]sharedDepotRef, 0, 8)
	for r := center.R - radius; r <= center.R+radius; r++ {
		if r < 0 || r >= core.Rows {
			continue
		}
		for dc := -radius; dc <= radius; dc++ {
			pos := util.NewPos(r, center.C+dc)
			depot, ok := depotAt(g.Board.At(pos))
			if !ok || depot.Colony != colony {
				continue
			}
			refs = append(refs, sharedDepotRef{
				pos:      pos,
				depot:    depot,
				distance: boardDistance(center, pos),
				index:    util.Idx(pos),
			})
		}
	}
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].distance != refs[j].distance {
			return refs[i].distance < refs[j].distance
		}
		return refs[i].index < refs[j].index
	})
	return refs
}

func (g *Game) hasNearbySameColonyController(colony *core.Colony, center core.Position, radius int) bool {
	if g == nil || g.Board == nil || colony == nil || radius < 0 {
		return false
	}
	for r := center.R - radius; r <= center.R+radius; r++ {
		if r < 0 || r >= core.Rows {
			continue
		}
		for dc := -radius; dc <= radius; dc++ {
			pos := util.NewPos(r, center.C+dc)
			switch ctrl := g.Board.At(pos).(type) {
			case core.Controller:
				if ctrl.Colony == colony {
					return true
				}
			case *core.Controller:
				if ctrl != nil && ctrl.Colony == colony {
					return true
				}
			}
		}
	}
	return false
}

func depotAt(cell core.Occupant) (core.Depot, bool) {
	switch depot := cell.(type) {
	case core.Depot:
		return depot, true
	case *core.Depot:
		if depot == nil {
			return core.Depot{}, false
		}
		return *depot, true
	default:
		return core.Depot{}, false
	}
}

func boardDistance(a, b core.Position) int {
	dr := util.Abs(a.R - b.R)
	dc := util.Abs(a.C - b.C)
	if wrap := core.Cols - dc; wrap < dc {
		dc = wrap
	}
	return max(dr, dc)
}
