package game

import (
	"golab/internal/core"
	"golab/internal/util"
	"math/rand"
)

const mockGameMasterName = "mock-coolio"

type MasterPosition struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

type MasterObservation struct {
	Tick                      int `json:"tick"`
	LiveBots                  int `json:"live_bots"`
	Colonies                  int `json:"colonies"`
	Controllers               int `json:"controllers"`
	ColonyMemberBots          int `json:"colony_member_bots"`
	ConnectedColonyBots       int `json:"connected_colony_bots"`
	MaxActiveColonyMembers    int `json:"max_active_colony_members"`
	MaxActiveConnectedMembers int `json:"max_active_connected_members"`
	SoloActiveColonies        int `json:"solo_active_colonies"`
	Resources                 int `json:"resources"`
	Food                      int `json:"food"`
	Poison                    int `json:"poison"`
	Organics                  int `json:"organics"`
	Water                     int `json:"water"`
	Wall                      int `json:"wall"`
	TotalHP                   int `json:"total_bot_hp"`
	TotalInventory            int `json:"total_bot_inventory"`
	TotalFood                 int `json:"total_food_inventory"`
	TotalOre                  int `json:"total_ore_inventory"`
}

type MasterEvent struct {
	Tick    int            `json:"tick"`
	Kind    string         `json:"kind"`
	Thought string         `json:"thought"`
	Reason  string         `json:"reason"`
	Center  MasterPosition `json:"center"`
	Radius  int            `json:"radius"`
	Amount  int            `json:"amount"`
	Applied int            `json:"applied"`
}

type MockGameMaster struct {
	Name string `json:"name"`
}

type GameMasterAdvisor interface {
	AdvisorName() string
	Decide(MasterObservation) (MasterEvent, bool)
}

func NewMockGameMaster() MockGameMaster {
	return MockGameMaster{Name: mockGameMasterName}
}

func (m MockGameMaster) AdvisorName() string {
	return m.Name
}

func (m MockGameMaster) Decide(obs MasterObservation) (MasterEvent, bool) {
	center := randomMasterPosition()
	event := MasterEvent{
		Tick:   obs.Tick,
		Center: center,
	}

	switch {
	case obs.LiveBots == 0:
		event.Kind = "spark_bots"
		event.Thought = "Everyone vanished. I am flicking the lights back on."
		event.Reason = "no live bots"
		event.Radius = 12
		event.Amount = 120
	case obs.Controllers == 0 && obs.LiveBots > 0:
		event.Kind = "settlement_support"
		event.Thought = "No anchors left. Feed likely founders and add only enough metal to rebuild."
		event.Reason = "live bots without active controllers"
		event.Radius = 9
		event.Amount = 90
	case obs.Controllers > 0 && (obs.MaxActiveColonyMembers <= 2 || obs.MaxActiveConnectedMembers < 3):
		event.Kind = "colony_support"
		event.Thought = "Stop feeding lone raiders. Feed the roots."
		event.Reason = "active colonies are solo or nearly solo"
		event.Radius = 10
		event.Amount = 80
	case obs.LiveBots < 400:
		event.Kind = "food_rain"
		event.Thought = "The little ones are fading. Drop calories, not metal."
		event.Reason = "low live bot count"
		event.Radius = 14
		event.Amount = 180
	case obs.Water < 1200 && obs.Resources > 3500:
		event.Kind = "cooling_rain"
		event.Thought = "There is too much sun happening."
		event.Reason = "resource-heavy dry board"
		event.Radius = 12
		event.Amount = 140
	case obs.Poison > obs.Resources && obs.LiveBots < 2500:
		event.Kind = "cleansing_rain"
		event.Thought = "The map is getting mean. Rinse some venom away."
		event.Reason = "poison pressure exceeds resources"
		event.Radius = 14
		event.Amount = 160
	case obs.LiveBots > 8000 && obs.Food+obs.Resources > 1000 && masterHasStableColony(obs):
		event.Kind = "famine_wind"
		event.Thought = "Too many mouths. Make the feast move."
		event.Reason = "population boom with abundant food"
		event.Radius = 18
		event.Amount = 220
	case shouldAddOre(obs):
		event.Kind = "resource_rain"
		event.Thought = "Ore is actually scarce. Add a small seam."
		event.Reason = "ore scarcity without food scarcity"
		event.Radius = 8
		event.Amount = 45
	case obs.Resources < 250 && obs.Food < 250:
		event.Kind = "forage_rain"
		event.Thought = "The pantry is empty. Grow food first, ore second."
		event.Reason = "scarce food and resources"
		event.Radius = 12
		event.Amount = 140
	case obs.LiveBots > 3500 && obs.Poison < 300:
		event.Kind = "poison_bloom"
		event.Thought = "This is too peaceful. Add a bad neighborhood."
		event.Reason = "large calm swarm"
		event.Radius = 8
		event.Amount = 70
	default:
		return MasterEvent{}, false
	}

	return event, true
}

func masterHasStableColony(obs MasterObservation) bool {
	return obs.Controllers > 0 && obs.MaxActiveColonyMembers >= 3 && obs.MaxActiveConnectedMembers >= 3
}

func shouldAddOre(obs MasterObservation) bool {
	return obs.Resources < 120 &&
		obs.Food > obs.Resources*2 &&
		obs.TotalOre < max(1, obs.TotalFood/2)
}

func (g *Game) ObserveMaster(tick int) MasterObservation {
	obs := MasterObservation{Tick: tick}
	colonies := map[*core.Colony]struct{}{}
	activeColonies := map[*core.Colony]struct{}{}
	activeMembers := map[*core.Colony]int{}
	activeConnected := map[*core.Colony]int{}

	for _, colony := range g.Colonies {
		if colony != nil {
			colonies[colony] = struct{}{}
		}
	}
	for _, cell := range *g.Board.GetGrid() {
		switch v := cell.(type) {
		case core.Controller:
			if v.Colony != nil {
				colonies[v.Colony] = struct{}{}
				activeColonies[v.Colony] = struct{}{}
				obs.Controllers++
			}
		case *core.Controller:
			if v.Colony != nil {
				colonies[v.Colony] = struct{}{}
				activeColonies[v.Colony] = struct{}{}
				obs.Controllers++
			}
		}
	}
	for _, id := range g.sortedActiveBotIDs(nil) {
		bot := g.Board.BotByID(id)
		if bot == nil {
			continue
		}
		obs.LiveBots++
		obs.TotalHP += bot.Hp
		obs.TotalInventory += bot.Inventory.Total()
		obs.TotalFood += bot.Inventory.Food
		obs.TotalOre += bot.Inventory.Ore
		if bot.Colony != nil {
			colonies[bot.Colony] = struct{}{}
			obs.ColonyMemberBots++
			if _, active := activeColonies[bot.Colony]; active {
				activeMembers[bot.Colony]++
				if bot.ConnnectedToColony {
					activeConnected[bot.Colony]++
				}
			}
		}
		if bot.ConnnectedToColony {
			obs.ConnectedColonyBots++
		}
	}
	for _, cell := range *g.Board.GetGrid() {
		switch cell.(type) {
		case core.Controller:
		case *core.Controller:
		case core.Resource:
			obs.Resources++
		case core.Food:
			obs.Food++
		case core.Poison:
			obs.Poison++
		case core.Organics:
			obs.Organics++
		case core.Water:
			obs.Water++
		case core.Wall:
			obs.Wall++
		}
	}
	obs.Colonies = len(colonies)
	for colony := range activeColonies {
		members := activeMembers[colony]
		connected := activeConnected[colony]
		if members > obs.MaxActiveColonyMembers {
			obs.MaxActiveColonyMembers = members
		}
		if connected > obs.MaxActiveConnectedMembers {
			obs.MaxActiveConnectedMembers = connected
		}
		if members <= 1 {
			obs.SoloActiveColonies++
		}
	}
	return obs
}

func (g *Game) ApplyMasterEvent(event MasterEvent) MasterEvent {
	if event.Radius < 0 {
		event.Radius = 0
	}
	if event.Amount < 0 {
		event.Amount = 0
	}

	switch event.Kind {
	case "resource_rain":
		event.Applied = g.scatterMasterEvent(event, func(pos util.Position) bool {
			if !g.canMasterReplaceSoftCell(pos) {
				return false
			}
			g.Board.Set(pos, core.Resource{Pos: pos, Amount: 1 + rand.Intn(3)})
			g.emitEventPheromone(pos, core.PheromoneOre)
			return true
		})
	case "food_rain":
		event.Applied = g.scatterMasterEvent(event, func(pos util.Position) bool {
			if !g.canMasterReplaceSoftCell(pos) {
				return false
			}
			g.Board.Set(pos, core.Food{Pos: pos, Amount: 1})
			g.emitEventPheromone(pos, core.PheromoneFood)
			return true
		})
	case "forage_rain":
		event.Applied = g.scatterMasterEvent(event, func(pos util.Position) bool {
			if !g.canMasterReplaceSoftCell(pos) {
				return false
			}
			if forageDropsOre(pos, event.Tick) {
				g.Board.Set(pos, core.Resource{Pos: pos, Amount: 1})
				g.emitEventPheromone(pos, core.PheromoneOre)
				return true
			}
			g.Board.Set(pos, core.Food{Pos: pos, Amount: 1})
			g.emitEventPheromone(pos, core.PheromoneFood)
			return true
		})
	case "colony_support":
		event.Applied = g.applyColonySupport(event)
	case "settlement_support":
		event.Applied = g.applySettlementSupport(event)
	case "poison_bloom":
		event.Applied = g.scatterMasterEvent(event, func(pos util.Position) bool {
			if !g.canMasterReplaceSoftCell(pos) {
				return false
			}
			g.Board.Set(pos, core.Poison{Pos: pos})
			g.emitEventPheromone(pos, core.PheromoneDanger)
			return true
		})
	case "cooling_rain":
		groupID := 900000 + event.Tick
		event.Applied = g.scatterMasterEvent(event, func(pos util.Position) bool {
			if !g.canMasterReplaceSoftCell(pos) {
				return false
			}
			g.Board.Set(pos, core.Water{GroupId: groupID, Amount: 10000})
			return true
		})
	case "cleansing_rain":
		event.Applied = g.scatterMasterEvent(event, func(pos util.Position) bool {
			switch g.Board.At(pos).(type) {
			case core.Poison, core.Organics:
				g.Board.Set(pos, core.Food{Pos: pos, Amount: 1})
				g.emitEventPheromone(pos, core.PheromoneFood)
				return true
			default:
				return false
			}
		})
	case "famine_wind":
		if !g.hasStableActiveColony() {
			if event.Reason == "" {
				event.Reason = "skipped until a stable connected colony exists"
			} else {
				event.Reason += "; skipped until a stable connected colony exists"
			}
			event.Applied = 0
			return event
		}
		event.Applied = g.scatterMasterEvent(event, func(pos util.Position) bool {
			switch g.Board.At(pos).(type) {
			case core.Resource, core.Food:
				g.Board.Clear(pos)
				return true
			default:
				return false
			}
		})
	case "spark_bots":
		event.Applied = g.scatterMasterEvent(event, func(pos util.Position) bool {
			if !g.Board.IsEmpty(pos) || g.Board.GetBot(pos) != nil {
				return false
			}
			bot := core.NewBot(pos)
			if g.InitialGenome != nil {
				bot.Genome = *g.InitialGenome
			}
			g.Board.AddBot(pos, &bot)
			return true
		})
		g.config.LiveBots = g.liveBotCount()
	}
	return event
}

func (g *Game) hasStableActiveColony() bool {
	obs := g.ObserveMaster(g.logicTick)
	return masterHasStableColony(obs)
}

func forageDropsOre(pos util.Position, tick int) bool {
	return (pos.R+pos.C+tick)%5 == 0
}

type controllerSupportTarget struct {
	pos  util.Position
	ctrl core.Controller
}

func (g *Game) applyColonySupport(event MasterEvent) int {
	targets := g.controllerSupportTargets()
	if len(targets) == 0 || event.Amount == 0 {
		return 0
	}
	applied := 0
	for i, target := range targets {
		if applied >= event.Amount {
			break
		}
		target.ctrl.Amount += 40
		if target.ctrl.Colony != nil {
			target.ctrl.Colony.Deposit(6, 1)
		}
		g.Board.Set(target.pos, target.ctrl)
		applied += 8

		targets[i].ctrl = target.ctrl
	}
	maxAttempts := event.Amount*20 + 20
	for attempts := 0; attempts < maxAttempts && applied < event.Amount; attempts++ {
		target := targets[attempts%len(targets)]
		pos := randomPositionNear(target.pos, event.Radius)
		if util.OutOfBounds(pos) || g.Board.IsFrozen(pos) || !g.canMasterReplaceSoftCell(pos) {
			continue
		}
		if forageDropsOre(pos, event.Tick+attempts) {
			g.Board.Set(pos, core.Resource{Pos: pos, Amount: 1})
			g.emitEventPheromone(pos, core.PheromoneOre)
		} else {
			g.Board.Set(pos, core.Food{Pos: pos, Amount: 1})
			g.emitEventPheromone(pos, core.PheromoneFood)
		}
		applied++
	}
	return min(applied, event.Amount)
}

func (g *Game) applySettlementSupport(event MasterEvent) int {
	targets := g.liveBotPositions()
	if len(targets) == 0 || event.Amount == 0 {
		return 0
	}
	applied := g.seedSettlementController(event)
	maxAttempts := event.Amount*20 + 20
	for attempts := 0; attempts < maxAttempts && applied < event.Amount; attempts++ {
		target := targets[attempts%len(targets)]
		pos := randomPositionNear(target, event.Radius)
		if util.OutOfBounds(pos) || g.Board.IsFrozen(pos) || !g.canMasterReplaceSoftCell(pos) {
			continue
		}
		if (pos.R+pos.C+event.Tick+attempts)%6 == 0 {
			g.Board.Set(pos, core.Resource{Pos: pos, Amount: 1})
			g.emitEventPheromone(pos, core.PheromoneOre)
		} else {
			g.Board.Set(pos, core.Food{Pos: pos, Amount: 1})
			g.emitEventPheromone(pos, core.PheromoneFood)
		}
		applied++
	}
	return applied
}

func (g *Game) seedSettlementController(event MasterEvent) int {
	if len(g.controllerSupportTargets()) > 0 {
		return 0
	}
	founder, ctrlPos, ok := g.bestSettlementFounder()
	if !ok {
		return 0
	}
	colony := founder.Colony
	newColony := colony == nil
	if newColony && !g.canFoundNewColony() {
		return 0
	}
	if colony == nil {
		cln := core.NewColony(ctrlPos)
		colony = &cln
		g.Colonies = append(g.Colonies, colony)
	}
	colony.AddFamily(g.liveLineageRoot(founder))
	colony.Deposit(4, 1)
	g.recruitFriendlyBots(colony, ctrlPos, controllerRecruitRadius)
	g.claimNearbyColonyFarms(colony, ctrlPos, controllerRecruitRadius)
	g.connectNearbyColonyMembers(colony, ctrlPos, controllerRecruitRadius)
	g.Board.Set(ctrlPos, core.Controller{
		Pos:    ctrlPos,
		Owner:  founder,
		Colony: colony,
		Amount: max(1, g.config.ControllerInitialAmount/2),
	})
	g.emitHomePheromone(ctrlPos, colony, g.config.PheromoneHomeDeposit)
	if newColony {
		g.initializeColonySpawnerGenome(colony, founder)
		g.buildInitialColonyInfrastructure(ctrlPos, founder, colony)
	}
	return 10
}

func (g *Game) bestSettlementFounder() (*core.Bot, util.Position, bool) {
	var best *core.Bot
	var bestCtrlPos util.Position
	bestScore := -1
	for _, id := range g.sortedActiveBotIDs(nil) {
		bot := g.Board.BotByID(id)
		if bot == nil {
			continue
		}
		if g.Board.GetBot(bot.Pos) != bot {
			continue
		}
		ctrlPos, ok := g.Board.FindEmptyPosAround(bot.Pos)
		if !ok || g.Board.IsFrozen(ctrlPos) || g.hasControllerNear(ctrlPos, controllerBuildMinRadius) {
			continue
		}
		if bot.Colony == nil && !g.hasNearbyFounderSupport(bot, ctrlPos, controllerRecruitRadius) {
			continue
		}
		score := bot.Hp + bot.Inventory.Total()*20 + g.nearbyFriendlyCount(bot, controllerRecruitRadius)*100
		if score <= bestScore {
			continue
		}
		best = bot
		bestCtrlPos = ctrlPos
		bestScore = score
	}
	return best, bestCtrlPos, best != nil
}

func (g *Game) nearbyFriendlyCount(bot *core.Bot, radius int) int {
	if bot == nil {
		return 0
	}
	count := 0
	for _, id := range g.sortedActiveBotIDs(nil) {
		other := g.Board.BotByID(id)
		if other == nil || other == bot || !other.Pos.InRadius(bot.Pos, radius) {
			continue
		}
		if core.BotsFriendly(bot, other) {
			count++
		}
	}
	return count
}

func (g *Game) liveBotPositions() []util.Position {
	positions := []util.Position{}
	for _, id := range g.sortedActiveBotIDs(nil) {
		bot := g.Board.BotByID(id)
		if bot == nil {
			continue
		}
		pos, ok := g.Board.BotPosition(id)
		if !ok {
			continue
		}
		if g.Board.GetBot(pos) != bot {
			continue
		}
		positions = append(positions, pos)
	}
	return positions
}

func (g *Game) controllerSupportTargets() []controllerSupportTarget {
	targets := []controllerSupportTarget{}
	for idx, cell := range *g.Board.GetGrid() {
		pos := util.PosOf(idx)
		switch ctrl := cell.(type) {
		case core.Controller:
			if ctrl.Colony != nil && g.controllerOwnerAlive(&ctrl) {
				targets = append(targets, controllerSupportTarget{pos: pos, ctrl: ctrl})
			}
		case *core.Controller:
			if ctrl != nil && ctrl.Colony != nil && g.controllerOwnerAlive(ctrl) {
				targets = append(targets, controllerSupportTarget{pos: pos, ctrl: *ctrl})
			}
		}
	}
	return targets
}

func (g *Game) scatterMasterEvent(event MasterEvent, apply func(util.Position) bool) int {
	if event.Amount == 0 {
		return 0
	}
	center := event.Center.toPosition()
	applied := 0
	maxAttempts := event.Amount*20 + 20
	for attempts := 0; attempts < maxAttempts && applied < event.Amount; attempts++ {
		pos := randomPositionNear(center, event.Radius)
		if util.OutOfBounds(pos) || g.Board.IsFrozen(pos) {
			continue
		}
		if apply(pos) {
			applied++
		}
	}
	return applied
}

func (g *Game) canMasterReplaceSoftCell(pos util.Position) bool {
	if util.OutOfBounds(pos) || g.Board.IsFrozen(pos) || g.Board.GetBot(pos) != nil {
		return false
	}
	switch g.Board.At(pos).(type) {
	case nil, core.Resource, core.Food, core.Organics, core.Poison:
		return true
	default:
		return false
	}
}

func randomMasterPosition() MasterPosition {
	return MasterPosition{
		Row: 1 + rand.Intn(core.Rows-2),
		Col: rand.Intn(core.Cols),
	}
}

func randomPositionNear(center util.Position, radius int) util.Position {
	if radius <= 0 {
		return center
	}
	dr := rand.Intn(radius*2+1) - radius
	dc := rand.Intn(radius*2+1) - radius
	return util.NewPos(center.R+dr, center.C+dc)
}

func (p MasterPosition) toPosition() util.Position {
	return util.NewPos(p.Row, p.Col)
}
