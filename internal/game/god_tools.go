package game

import (
	"fmt"
	"golab/internal/core"
	"golab/internal/ui"
	"golab/internal/util"
)

func (g *Game) ApplyGodTool(tool ui.GodTool, pos core.Position, radius int) ui.GodReport {
	radius = max(0, min(radius, 20))

	switch tool {
	case ui.GodToolInspect:
		return g.inspectGodCell(pos)
	case ui.GodToolWater:
		groupID := 700000 + g.logicTick
		applied := g.applyGodBrush(pos, radius, func(p util.Position) bool {
			if !g.canGodReplaceSoftCell(p) {
				return false
			}
			g.Board.Set(p, core.Water{GroupId: groupID, Amount: 10000})
			return true
		})
		return ui.GodReport{Message: fmt.Sprintf("Painted water: %d", applied)}
	case ui.GodToolPoison:
		applied := g.applyGodBrush(pos, radius, func(p util.Position) bool {
			if !g.canGodReplaceSoftCell(p) {
				return false
			}
			g.Board.Set(p, core.Poison{Pos: p})
			g.emitEventPheromone(p, core.PheromoneDanger)
			return true
		})
		return ui.GodReport{Message: fmt.Sprintf("Painted poison: %d", applied)}
	case ui.GodToolFood:
		applied := g.applyGodBrush(pos, radius, func(p util.Position) bool {
			if !g.canGodReplaceSoftCell(p) {
				return false
			}
			g.Board.Set(p, core.Food{Pos: p, Amount: 1})
			g.emitEventPheromone(p, core.PheromoneFood)
			return true
		})
		return ui.GodReport{Message: fmt.Sprintf("Dropped food: %d", applied)}
	case ui.GodToolColony:
		return g.spawnGodColony(pos)
	case ui.GodToolFreeze:
		applied := g.applyGodBrush(pos, radius, func(p util.Position) bool {
			return g.Board.SetFrozen(p, true)
		})
		return ui.GodReport{Message: fmt.Sprintf("Frozen cells: %d", applied)}
	case ui.GodToolUnfreeze:
		applied := g.applyGodBrush(pos, radius, func(p util.Position) bool {
			return g.Board.SetFrozen(p, false)
		})
		return ui.GodReport{Message: fmt.Sprintf("Thawed cells: %d", applied)}
	case ui.GodToolBless:
		colony := g.selectedOrClickedColony(pos)
		if colony == nil {
			return ui.GodReport{Message: "No colony selected"}
		}
		applied := g.blessColony(colony, radius)
		return ui.GodReport{Message: fmt.Sprintf("Blessed colony: %d", applied), Lines: g.colonyInspectLines(colony)}
	case ui.GodToolCurse:
		colony := g.selectedOrClickedColony(pos)
		if colony == nil {
			return ui.GodReport{Message: "No colony selected"}
		}
		applied := g.curseColony(colony, radius)
		return ui.GodReport{Message: fmt.Sprintf("Cursed colony: %d", applied), Lines: g.colonyInspectLines(colony)}
	default:
		return ui.GodReport{Message: "Unknown god tool"}
	}
}

func (g *Game) SelectedColonyLabel() string {
	if g.selectedColony == nil {
		return "none"
	}
	return fmt.Sprintf("R%d C%d M%d", g.selectedColony.Center.R, g.selectedColony.Center.C, g.liveSelectedColonyMembers())
}

func (g *Game) applyGodBrush(center util.Position, radius int, apply func(util.Position) bool) int {
	applied := 0
	for r := center.R - radius; r <= center.R+radius; r++ {
		if r < 0 || r >= core.Rows {
			continue
		}
		for dc := -radius; dc <= radius; dc++ {
			pos := util.NewPos(r, center.C+dc)
			if apply(pos) {
				applied++
			}
		}
	}
	return applied
}

func (g *Game) canGodReplaceSoftCell(pos util.Position) bool {
	if util.OutOfBounds(pos) || g.Board.IsFrozen(pos) || g.Board.GetBot(pos) != nil {
		return false
	}
	switch g.Board.At(pos).(type) {
	case nil, core.Resource, core.Food, core.Organics, core.Poison, core.Water:
		return true
	default:
		return false
	}
}

func (g *Game) spawnGodColony(pos util.Position) ui.GodReport {
	if !g.canFoundNewColony() {
		return ui.GodReport{
			Message: fmt.Sprintf("Colony limit reached (%d active)", max(0, g.config.ColonyMaxActive)),
			Lines:   []string{"New colony rejected by active colony cap."},
		}
	}
	controllerPos, ok := g.firstGodEmptyNear(pos, 6)
	if !ok {
		return ui.GodReport{Message: "No empty cell for colony"}
	}
	ownerPos, ok := g.firstGodEmptyAround(controllerPos)
	if !ok {
		return ui.GodReport{Message: "No empty cell for founder"}
	}

	bot := core.NewBot(ownerPos)
	if g.InitialGenome != nil {
		bot.Genome = *g.InitialGenome
	}
	colony := core.NewColony(controllerPos)
	colony.Color = bot.Color
	colony.AddFamily(&bot)
	bot.ConnnectedToColony = true

	g.Board.AddBot(ownerPos, &bot)
	g.Board.Set(controllerPos, core.Controller{
		Pos:         controllerPos,
		Owner:       &bot,
		Colony:      &colony,
		Amount:      g.config.ControllerInitialAmount,
		WaterAmount: 0,
	})
	g.emitHomePheromone(controllerPos, &colony, g.config.PheromoneHomeDeposit)
	g.emitHomePheromone(ownerPos, &colony, g.config.PheromoneHomeDeposit/3)
	g.initializeColonySpawnerGenome(&colony, &bot)
	g.buildInitialColonyInfrastructure(controllerPos, &bot, &colony)
	g.Colonies = append(g.Colonies, &colony)
	g.selectColony(&colony)
	g.config.LiveBots = g.liveBotCount()

	return ui.GodReport{
		Message: fmt.Sprintf("Spawned colony R%d C%d", controllerPos.R, controllerPos.C),
		Lines:   g.colonyInspectLines(&colony),
	}
}

func (g *Game) firstGodEmptyNear(center util.Position, radius int) (util.Position, bool) {
	if g.canPlaceGodStructure(center) {
		return center, true
	}
	for r := center.R - radius; r <= center.R+radius; r++ {
		if r < 0 || r >= core.Rows {
			continue
		}
		for dc := -radius; dc <= radius; dc++ {
			pos := util.NewPos(r, center.C+dc)
			if g.canPlaceGodStructure(pos) {
				return pos, true
			}
		}
	}
	return util.Position{}, false
}

func (g *Game) firstGodEmptyAround(center util.Position) (util.Position, bool) {
	for _, dir := range core.PosClock {
		pos := center.AddDir(dir)
		if g.canPlaceGodStructure(pos) {
			return pos, true
		}
	}
	return util.Position{}, false
}

func (g *Game) canPlaceGodStructure(pos util.Position) bool {
	return !util.OutOfBounds(pos) && !g.Board.IsFrozen(pos) && g.Board.GetBot(pos) == nil && g.Board.IsEmpty(pos)
}

func (g *Game) inspectGodCell(pos util.Position) ui.GodReport {
	if bot := g.Board.GetBot(pos); bot != nil {
		if bot.Colony != nil {
			g.selectColony(bot.Colony)
		}
		task := "none"
		if bot.CurrTask != nil {
			task = bot.CurrTask.Type.String()
		}
		return ui.GodReport{
			Message: fmt.Sprintf("Inspected bot R%d C%d", pos.R, pos.C),
			Lines: []string{
				fmt.Sprintf("Bot HP %d F %d O %d", bot.Hp, bot.Inventory.Food, bot.Inventory.Ore),
				fmt.Sprintf("Task %s", task),
				fmt.Sprintf("Colony %s", g.SelectedColonyLabel()),
				pheromoneInspectLine(g.Board, pos),
			},
		}
	}

	switch v := g.Board.At(pos).(type) {
	case core.Controller:
		if v.Colony != nil {
			g.selectColony(v.Colony)
		}
		return ui.GodReport{
			Message: fmt.Sprintf("Inspected controller R%d C%d", pos.R, pos.C),
			Lines: []string{
				fmt.Sprintf("Controller amount %d", v.Amount),
				fmt.Sprintf("Water %d", v.WaterAmount),
				colonyBankLine(v.Colony),
				fmt.Sprintf("Colony %s", g.SelectedColonyLabel()),
				pheromoneInspectLine(g.Board, pos),
			},
		}
	case *core.Controller:
		if v.Colony != nil {
			g.selectColony(v.Colony)
		}
		return ui.GodReport{
			Message: fmt.Sprintf("Inspected controller R%d C%d", pos.R, pos.C),
			Lines: []string{
				fmt.Sprintf("Controller amount %d", v.Amount),
				fmt.Sprintf("Water %d", v.WaterAmount),
				colonyBankLine(v.Colony),
				fmt.Sprintf("Colony %s", g.SelectedColonyLabel()),
				pheromoneInspectLine(g.Board, pos),
			},
		}
	case core.Depot:
		if v.Colony != nil {
			g.selectColony(v.Colony)
		}
		return ui.GodReport{
			Message: fmt.Sprintf("Inspected depot R%d C%d", pos.R, pos.C),
			Lines: []string{
				fmt.Sprintf("Depot F %d O %d", v.Food, v.Ore),
				fmt.Sprintf("Owner %v", v.Owner != nil),
				fmt.Sprintf("Colony %s", g.SelectedColonyLabel()),
				pheromoneInspectLine(g.Board, pos),
			},
		}
	case *core.Depot:
		if v == nil {
			return ui.GodReport{
				Message: fmt.Sprintf("Inspected cell R%d C%d", pos.R, pos.C),
				Lines: []string{
					fmt.Sprintf("Cell %T", g.Board.At(pos)),
					fmt.Sprintf("Frozen %v", g.Board.IsFrozen(pos)),
					pheromoneInspectLine(g.Board, pos),
				},
			}
		}
		if v.Colony != nil {
			g.selectColony(v.Colony)
		}
		return ui.GodReport{
			Message: fmt.Sprintf("Inspected depot R%d C%d", pos.R, pos.C),
			Lines: []string{
				fmt.Sprintf("Depot F %d O %d", v.Food, v.Ore),
				fmt.Sprintf("Owner %v", v.Owner != nil),
				fmt.Sprintf("Colony %s", g.SelectedColonyLabel()),
				pheromoneInspectLine(g.Board, pos),
			},
		}
	default:
		return ui.GodReport{
			Message: fmt.Sprintf("Inspected cell R%d C%d", pos.R, pos.C),
			Lines: []string{
				fmt.Sprintf("Cell %T", g.Board.At(pos)),
				fmt.Sprintf("Frozen %v", g.Board.IsFrozen(pos)),
				pheromoneInspectLine(g.Board, pos),
			},
		}
	}
}

func (g *Game) selectedOrClickedColony(pos util.Position) *core.Colony {
	if colony := g.colonyAt(pos); colony != nil {
		g.selectColony(colony)
		return colony
	}
	return g.selectedColony
}

func (g *Game) colonyAt(pos util.Position) *core.Colony {
	if bot := g.Board.GetBot(pos); bot != nil {
		return bot.Colony
	}
	switch v := g.Board.At(pos).(type) {
	case core.Controller:
		return v.Colony
	case *core.Controller:
		return v.Colony
	case core.Depot:
		return v.Colony
	case *core.Depot:
		if v == nil {
			return nil
		}
		return v.Colony
	}
	for r := pos.R - 4; r <= pos.R+4; r++ {
		if r < 0 || r >= core.Rows {
			continue
		}
		for dc := -4; dc <= 4; dc++ {
			near := util.NewPos(r, pos.C+dc)
			if bot := g.Board.GetBot(near); bot != nil && bot.Colony != nil {
				return bot.Colony
			}
			switch v := g.Board.At(near).(type) {
			case core.Controller:
				if v.Colony != nil {
					return v.Colony
				}
			case *core.Controller:
				if v.Colony != nil {
					return v.Colony
				}
			case core.Depot:
				if v.Colony != nil {
					return v.Colony
				}
			case *core.Depot:
				if v != nil && v.Colony != nil {
					return v.Colony
				}
			}
		}
	}
	return nil
}

func (g *Game) selectColony(colony *core.Colony) {
	g.selectedColony = colony
	for _, id := range g.Board.ActiveBotIDs() {
		bot := g.Board.BotByID(id)
		if bot == nil {
			continue
		}
		selected := colony != nil && bot.Colony == colony
		if bot.IsSelected != selected {
			bot.IsSelected = selected
			if cell := g.Board.BotCell(id); cell >= 0 {
				g.Board.MarkDirty(cell)
			}
		}
	}
}

func (g *Game) blessColony(colony *core.Colony, radius int) int {
	applied := 0
	for _, bot := range colony.Members {
		if bot == nil {
			continue
		}
		bot.Hp = min(500, bot.Hp+120)
		bot.Inventory.AddFood(5)
		bot.Inventory.AddOre(5)
		bot.Color = blendColor(bot.Color, util.GreenColor(), 0.35)
		g.Board.MarkDirty(util.Idx(bot.Pos))
		applied++
	}
	for _, pos := range g.controllerPositions(colony) {
		switch ctrl := g.Board.At(pos).(type) {
		case core.Controller:
			ctrl.Amount += 250
			g.Board.Set(pos, ctrl)
			applied++
		case *core.Controller:
			ctrl.Amount += 250
			g.Board.MarkDirty(util.Idx(pos))
			applied++
		}
	}
	applied += g.applyGodBrush(colony.Center, min(radius+2, 8), func(p util.Position) bool {
		if !g.canGodReplaceSoftCell(p) {
			return false
		}
		g.Board.Set(p, core.Food{Pos: p, Amount: 1})
		g.emitEventPheromone(p, core.PheromoneFood)
		return true
	})
	g.selectColony(colony)
	return applied
}

func (g *Game) curseColony(colony *core.Colony, radius int) int {
	applied := 0
	for _, bot := range colony.Members {
		if bot == nil {
			continue
		}
		bot.Hp = max(1, bot.Hp-120)
		bot.Inventory.Clear()
		bot.Color = blendColor(bot.Color, util.RedColor(), 0.45)
		g.Board.MarkDirty(util.Idx(bot.Pos))
		applied++
	}
	for _, pos := range g.controllerPositions(colony) {
		switch ctrl := g.Board.At(pos).(type) {
		case core.Controller:
			ctrl.Amount = max(0, ctrl.Amount-250)
			g.Board.Set(pos, ctrl)
			applied++
		case *core.Controller:
			ctrl.Amount = max(0, ctrl.Amount-250)
			g.Board.MarkDirty(util.Idx(pos))
			applied++
		}
	}
	applied += g.applyGodBrush(colony.Center, min(radius+2, 8), func(p util.Position) bool {
		if !g.canGodReplaceSoftCell(p) {
			return false
		}
		g.Board.Set(p, core.Poison{Pos: p})
		g.emitEventPheromone(p, core.PheromoneDanger)
		return true
	})
	g.selectColony(colony)
	return applied
}

func (g *Game) controllerPositions(colony *core.Colony) []util.Position {
	positions := []util.Position{}
	for i, cell := range *g.Board.GetGrid() {
		pos := util.PosOf(i)
		switch ctrl := cell.(type) {
		case core.Controller:
			if ctrl.Colony == colony {
				positions = append(positions, pos)
			}
		case *core.Controller:
			if ctrl.Colony == colony {
				positions = append(positions, pos)
			}
		}
	}
	return positions
}

func (g *Game) colonyInspectLines(colony *core.Colony) []string {
	return []string{
		fmt.Sprintf("Colony R%d C%d", colony.Center.R, colony.Center.C),
		fmt.Sprintf("Members %d", g.liveSelectedColonyMembers()),
		colonyBankLine(colony),
		fmt.Sprintf("Flags %d Tasks %d", len(colony.Flags), len(colony.Tasks)),
	}
}

func colonyBankLine(colony *core.Colony) string {
	food, ore := colonyBank(colony)
	return fmt.Sprintf("Bank F %d O %d", food, ore)
}

func colonyBank(colony *core.Colony) (int, int) {
	if colony == nil {
		return 0, 0
	}
	return colony.FoodBank, colony.OreBank
}

func (g *Game) liveSelectedColonyMembers() int {
	if g.selectedColony == nil {
		return 0
	}
	count := 0
	for _, id := range g.Board.ActiveBotIDs() {
		bot := g.Board.BotByID(id)
		if bot != nil && bot.Colony == g.selectedColony {
			count++
		}
	}
	return count
}

func blendColor(a, b [3]float32, weight float32) [3]float32 {
	return [3]float32{
		a[0]*(1-weight) + b[0]*weight,
		a[1]*(1-weight) + b[1]*weight,
		a[2]*(1-weight) + b[2]*weight,
	}
}
