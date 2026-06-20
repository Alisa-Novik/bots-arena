package game

import (
	"encoding/json"
	"golab/internal/config"
	"golab/internal/core"
	"golab/internal/ui"
	"golab/internal/util"
	"math/rand"
	"os"
	"slices"
	"strings"
	"testing"

	expRand "golang.org/x/exp/rand"
)

func firstBiomeCell(t *testing.T, brd *core.Board, biome core.Biome) core.Position {
	t.Helper()
	for r := 3; r < core.Rows-3; r++ {
		for c := 0; c < core.Cols; c++ {
			pos := util.NewPos(r, c)
			if brd.BiomeAt(pos) == biome {
				return pos
			}
		}
	}
	t.Fatalf("no %s biome cell found", biome)
	return core.Position{}
}

func firstBuildTargetForBiome(t *testing.T, brd *core.Board, biome core.Biome) (core.Position, core.Position) {
	t.Helper()
	for r := 3; r < core.Rows-3; r++ {
		for c := 0; c < core.Cols; c++ {
			buildPos := util.NewPos(r, c)
			botPos := buildPos.AddRowCol(-1, 0)
			if brd.BiomeAt(buildPos) == biome && brd.IsEmpty(buildPos) && brd.IsEmpty(botPos) {
				return botPos, buildPos
			}
		}
	}
	t.Fatalf("no build target for %s biome found", biome)
	return core.Position{}, core.Position{}
}

func countFoodAround(brd *core.Board, center core.Position) int {
	count := 0
	for _, dir := range core.PosClock {
		if _, ok := brd.At(center.AddDir(dir)).(core.Food); ok {
			count++
		}
	}
	return count
}

func addTestBot(g *Game, bot *core.Bot) {
	g.Board.Bots[util.Idx(bot.Pos)] = bot
	g.Board.Set(bot.Pos, bot)
}

func makeForeignGenome(base core.Genome) core.Genome {
	foreign := base
	for i := 0; i < 5; i++ {
		foreign.Matrix[i] = base.Matrix[i] + 100 + i
	}
	return foreign
}

func TestGrabControllerRequiresFood(t *testing.T) {
	cfg := config.NewConfig()
	cfg.ControllerGrabCost = 1
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	botPos := util.NewPos(10, 10)
	bot := core.NewBot(botPos)
	bot.Dir = core.Up
	ctrlPos := botPos.AddRowCol(bot.Dir[0], bot.Dir[1])
	colony := core.NewColony(ctrlPos)
	colony.AddFamily(&bot)

	g.Board.Bots[util.Idx(botPos)] = &bot
	g.Board.Set(botPos, &bot)
	g.Board.Set(ctrlPos, core.Controller{
		Pos:    ctrlPos,
		Owner:  &bot,
		Colony: &colony,
		Amount: 0,
	})

	g.grab(botPos, &bot)

	if bot.Inventory.Total() != 0 {
		t.Fatalf("inventory = %+v, want empty", bot.Inventory)
	}
	ctrl, ok := g.Board.At(ctrlPos).(core.Controller)
	if !ok {
		t.Fatalf("controller missing at %v", ctrlPos)
	}
	if ctrl.Amount != 0 {
		t.Fatalf("controller amount = %d, want 0", ctrl.Amount)
	}
}

func TestGrabControllerSpendsFood(t *testing.T) {
	cfg := config.NewConfig()
	cfg.ControllerGrabCost = 1
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	botPos := util.NewPos(10, 10)
	bot := core.NewBot(botPos)
	bot.Dir = core.Up
	ctrlPos := botPos.AddRowCol(bot.Dir[0], bot.Dir[1])
	bot.Inventory.Food = 2
	bot.Inventory.Ore = 2
	colony := core.NewColony(ctrlPos)
	colony.AddFamily(&bot)

	g.Board.Bots[util.Idx(botPos)] = &bot
	g.Board.Set(botPos, &bot)
	g.Board.Set(ctrlPos, core.Controller{
		Pos:    ctrlPos,
		Owner:  &bot,
		Colony: &colony,
		Amount: 0,
	})

	g.grab(botPos, &bot)

	if bot.Inventory.Food != 1 {
		t.Fatalf("food inventory = %d, want 1", bot.Inventory.Food)
	}
	if bot.Inventory.Ore != 2 {
		t.Fatalf("ore inventory = %d, want unchanged 2", bot.Inventory.Ore)
	}
	ctrl, ok := g.Board.At(ctrlPos).(core.Controller)
	if !ok {
		t.Fatalf("controller missing at %v", ctrlPos)
	}
	if ctrl.Amount != 1 {
		t.Fatalf("controller amount = %d, want 1", ctrl.Amount)
	}
}

func TestGrabControllerSpendsBankFoodForConnectedMember(t *testing.T) {
	cfg := config.NewConfig()
	cfg.ControllerGrabCost = 1
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	botPos := util.NewPos(10, 10)
	bot := core.NewBot(botPos)
	bot.Dir = core.Up
	ctrlPos := botPos.AddRowCol(bot.Dir[0], bot.Dir[1])
	colony := core.NewColony(ctrlPos)
	colony.FoodBank = 1
	colony.AddFamily(&bot)
	bot.ConnnectedToColony = true

	g.Board.Bots[util.Idx(botPos)] = &bot
	g.Board.Set(botPos, &bot)
	g.Board.Set(ctrlPos, core.Controller{
		Pos:    ctrlPos,
		Owner:  &bot,
		Colony: &colony,
		Amount: 0,
	})

	g.grab(botPos, &bot)

	if bot.Inventory.Total() != 0 {
		t.Fatalf("personal inventory after bank controller grab = %+v, want empty", bot.Inventory)
	}
	if colony.FoodBank != 0 {
		t.Fatalf("food bank after controller grab = %d, want 0", colony.FoodBank)
	}
	ctrl, ok := g.Board.At(ctrlPos).(core.Controller)
	if !ok {
		t.Fatalf("controller missing at %v", ctrlPos)
	}
	if ctrl.Amount != 1 {
		t.Fatalf("controller amount = %d, want 1", ctrl.Amount)
	}
}

func TestGrabFoodAddsFoodInventoryAndHp(t *testing.T) {
	cfg := config.NewConfig()
	cfg.FoodGrabHpGain = 25
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	botPos := util.NewPos(10, 10)
	bot := core.NewBot(botPos)
	bot.Dir = core.Up
	bot.Hp = 100
	bot.Genome.Pointer = 1
	foodPos := botPos.AddRowCol(bot.Dir[0], bot.Dir[1])

	g.Board.Bots[util.Idx(botPos)] = &bot
	g.Board.Set(botPos, &bot)
	g.Board.Set(foodPos, core.Food{Pos: foodPos, Amount: 2})

	g.grab(botPos, &bot)

	if bot.Inventory.Food != 2 {
		t.Fatalf("food inventory = %d, want 2", bot.Inventory.Food)
	}
	if bot.Inventory.Ore != 0 {
		t.Fatalf("ore inventory = %d, want 0", bot.Inventory.Ore)
	}
	if bot.Hp != 125 {
		t.Fatalf("hp after food grab = %d, want 125", bot.Hp)
	}
	if bot.Genome.Pointer != 9 {
		t.Fatalf("pointer after food grab = %d, want 9", bot.Genome.Pointer)
	}
	if got := g.Board.At(foodPos); got != nil {
		t.Fatalf("food cell after grab = %T, want nil", got)
	}
}

func TestGrabResourceAddsOreInventoryAndHp(t *testing.T) {
	cfg := config.NewConfig()
	cfg.ResourceGrabGain = 7
	cfg.ResourceGrabHpGain = 25
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	botPos := util.NewPos(10, 10)
	bot := core.NewBot(botPos)
	bot.Dir = core.Up
	bot.Hp = 100
	resourcePos := botPos.AddRowCol(bot.Dir[0], bot.Dir[1])

	g.Board.Bots[util.Idx(botPos)] = &bot
	g.Board.Set(botPos, &bot)
	g.Board.Set(resourcePos, core.Resource{Pos: resourcePos, Amount: 20})

	g.grab(botPos, &bot)

	if bot.Inventory.Food != 0 {
		t.Fatalf("food inventory = %d, want 0", bot.Inventory.Food)
	}
	if bot.Inventory.Ore != 7 {
		t.Fatalf("ore inventory = %d, want 7", bot.Inventory.Ore)
	}
	if bot.Hp != 125 {
		t.Fatalf("hp after resource grab = %d, want 125", bot.Hp)
	}
	resource, ok := g.Board.At(resourcePos).(core.Resource)
	if !ok {
		t.Fatalf("resource missing after partial grab")
	}
	if resource.Amount != 10 {
		t.Fatalf("resource amount after grab = %d, want 10", resource.Amount)
	}
}

func TestMoveIntoFoodCollectsFoodInsteadOfBlocking(t *testing.T) {
	cfg := config.NewConfig()
	cfg.FoodGrabHpGain = 25
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	botPos := util.NewPos(10, 10)
	bot := core.NewBot(botPos)
	bot.Dir = core.Up
	bot.Hp = 100
	foodPos := botPos.AddDir(bot.Dir)

	g.Board.Bots[util.Idx(botPos)] = &bot
	g.Board.Set(botPos, &bot)
	g.Board.Set(foodPos, core.Food{Pos: foodPos, Amount: 1})

	g.tryMove(botPos, &bot)

	if bot.Pos != botPos {
		t.Fatalf("bot position after bump pickup = %v, want %v", bot.Pos, botPos)
	}
	if bot.Inventory.Food != 1 {
		t.Fatalf("food inventory after bump pickup = %d, want 1", bot.Inventory.Food)
	}
	if bot.Hp != 125 {
		t.Fatalf("hp after bump pickup = %d, want 125", bot.Hp)
	}
	if got := g.Board.At(foodPos); got != nil {
		t.Fatalf("food cell after bump pickup = %T, want nil", got)
	}
}

func TestMoveIntoResourceCollectsOreInsteadOfBlocking(t *testing.T) {
	cfg := config.NewConfig()
	cfg.ResourceGrabGain = 5
	cfg.ResourceGrabHpGain = 25
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	botPos := util.NewPos(10, 10)
	bot := core.NewBot(botPos)
	bot.Dir = core.Up
	bot.Hp = 100
	resourcePos := botPos.AddDir(bot.Dir)

	g.Board.Bots[util.Idx(botPos)] = &bot
	g.Board.Set(botPos, &bot)
	g.Board.Set(resourcePos, core.Resource{Pos: resourcePos, Amount: 20})

	g.tryMove(botPos, &bot)

	if bot.Pos != botPos {
		t.Fatalf("bot position after bump pickup = %v, want %v", bot.Pos, botPos)
	}
	if bot.Inventory.Ore != 5 {
		t.Fatalf("ore inventory after bump pickup = %d, want 5", bot.Inventory.Ore)
	}
	if bot.Hp != 125 {
		t.Fatalf("hp after bump pickup = %d, want 125", bot.Hp)
	}
	resource, ok := g.Board.At(resourcePos).(core.Resource)
	if !ok {
		t.Fatalf("resource missing after partial bump pickup")
	}
	if resource.Amount != 10 {
		t.Fatalf("resource amount after bump pickup = %d, want 10", resource.Amount)
	}
}

func TestFoodAndOrePickupEmitPheromones(t *testing.T) {
	cfg := config.NewConfig()
	cfg.PheromoneEventDeposit = 48
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	foodBotPos := util.NewPos(10, 10)
	foodBot := core.NewBot(foodBotPos)
	foodBot.Dir = core.Up
	foodPos := foodBotPos.AddRowCol(foodBot.Dir[0], foodBot.Dir[1])
	g.Board.Bots[util.Idx(foodBotPos)] = &foodBot
	g.Board.Set(foodBotPos, &foodBot)
	g.Board.Set(foodPos, core.Food{Pos: foodPos, Amount: 1})

	g.grab(foodBotPos, &foodBot)

	if got := g.Board.PheromoneAt(foodPos).Food; got != 48 {
		t.Fatalf("food pickup pheromone = %d, want 48", got)
	}

	oreBotPos := util.NewPos(12, 10)
	oreBot := core.NewBot(oreBotPos)
	oreBot.Dir = core.Up
	orePos := oreBotPos.AddRowCol(oreBot.Dir[0], oreBot.Dir[1])
	g.Board.Bots[util.Idx(oreBotPos)] = &oreBot
	g.Board.Set(oreBotPos, &oreBot)
	g.Board.Set(orePos, core.Resource{Pos: orePos, Amount: 20})

	g.grab(oreBotPos, &oreBot)

	if got := g.Board.PheromoneAt(orePos).Ore; got != 48 {
		t.Fatalf("ore pickup pheromone = %d, want 48", got)
	}
}

func TestDivideRequiresFoodAndOre(t *testing.T) {
	cases := []struct {
		name      string
		inventory core.Inventory
	}{
		{name: "only hp", inventory: core.Inventory{}},
		{name: "only food", inventory: core.Inventory{Food: 1}},
		{name: "only ore", inventory: core.Inventory{Ore: 1}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.NewConfig()
			cfg.DivisionFoodCost = 1
			cfg.DivisionOreCost = 1
			g := NewGame(&cfg)
			g.Board = core.NewBoard()

			botPos := util.NewPos(10, 10)
			bot := core.NewBot(botPos)
			bot.Hp = cfg.DivisionMinHp
			bot.Inventory = tc.inventory
			bot.Genome.Matrix[0] = int(core.OpDivide)

			g.Board.Bots[util.Idx(botPos)] = &bot
			g.Board.Set(botPos, &bot)

			g.botAction(botPos, &bot)

			if got := g.liveBotCount(); got != 1 {
				t.Fatalf("live bots after failed divide = %d, want 1", got)
			}
			if bot.Hp != cfg.DivisionMinHp {
				t.Fatalf("hp after failed divide = %d, want %d", bot.Hp, cfg.DivisionMinHp)
			}
			if bot.Inventory != tc.inventory {
				t.Fatalf("inventory after failed divide = %+v, want %+v", bot.Inventory, tc.inventory)
			}
			if bot.Genome.Pointer != 5 {
				t.Fatalf("pointer after failed divide = %d, want 5", bot.Genome.Pointer)
			}
		})
	}
}

func TestDivideSpendsFoodOreHpAndChildInventoryEmpty(t *testing.T) {
	cfg := config.NewConfig()
	cfg.DivisionCost = 7
	cfg.DivisionFoodCost = 2
	cfg.DivisionOreCost = 3
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	botPos := util.NewPos(10, 10)
	bot := core.NewBot(botPos)
	bot.Hp = cfg.DivisionMinHp + 20
	bot.Inventory = core.Inventory{Food: 5, Ore: 7}
	bot.Genome.Matrix[0] = int(core.OpDivide)

	g.Board.Bots[util.Idx(botPos)] = &bot
	g.Board.Set(botPos, &bot)

	if !g.DivisionReady(&bot) {
		t.Fatalf("bot should be division-ready before action")
	}

	g.botAction(botPos, &bot)

	if got := g.liveBotCount(); got != 2 {
		t.Fatalf("live bots after divide = %d, want 2", got)
	}
	if got := g.SuccessfulDivisions(); got != 1 {
		t.Fatalf("successful divisions = %d, want 1", got)
	}
	if bot.Divisions != 1 {
		t.Fatalf("parent divisions = %d, want 1", bot.Divisions)
	}
	if bot.Hp != cfg.DivisionMinHp+20-cfg.DivisionCost {
		t.Fatalf("parent hp after divide = %d, want %d", bot.Hp, cfg.DivisionMinHp+20-cfg.DivisionCost)
	}
	if bot.Inventory.Food != 3 || bot.Inventory.Ore != 4 {
		t.Fatalf("parent inventory after divide = %+v, want food 3 ore 4", bot.Inventory)
	}
	if bot.Genome.Pointer != 6 {
		t.Fatalf("pointer after divide = %d, want 6", bot.Genome.Pointer)
	}

	var child *core.Bot
	for _, candidate := range g.Board.Bots {
		if candidate != nil && candidate != &bot {
			child = candidate
			break
		}
	}
	if child == nil {
		t.Fatalf("child bot not found")
	}
	if child.Inventory.Total() != 0 {
		t.Fatalf("child inventory = %+v, want empty", child.Inventory)
	}
	if child.LineageDepth != 1 {
		t.Fatalf("child lineage depth = %d, want 1", child.LineageDepth)
	}
	if child.Divisions != 0 {
		t.Fatalf("child divisions = %d, want 0", child.Divisions)
	}
}

func TestDivideUsesColonyBankForConnectedMember(t *testing.T) {
	cfg := config.NewConfig()
	cfg.DivisionCost = 7
	cfg.DivisionFoodCost = 2
	cfg.DivisionOreCost = 3
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	botPos := util.NewPos(10, 10)
	bot := core.NewBot(botPos)
	bot.Hp = cfg.DivisionMinHp + 20
	bot.Inventory = core.Inventory{Food: 1, Ore: 1}
	bot.Genome.Matrix[0] = int(core.OpDivide)
	colony := core.NewColony(util.NewPos(10, 9))
	colony.FoodBank = 1
	colony.OreBank = 2
	colony.AddFamily(&bot)
	bot.ConnnectedToColony = true

	g.Board.Bots[util.Idx(botPos)] = &bot
	g.Board.Set(botPos, &bot)
	g.Board.Set(colony.Center, core.Controller{Pos: colony.Center, Owner: &bot, Colony: &colony, Amount: 10})

	if !g.DivisionReady(&bot) {
		t.Fatalf("connected bot should be division-ready with personal+bank resources")
	}

	g.botAction(botPos, &bot)

	if got := g.liveBotCount(); got != 2 {
		t.Fatalf("live bots after bank-funded divide = %d, want 2", got)
	}
	if bot.Inventory.Food != 0 || bot.Inventory.Ore != 0 {
		t.Fatalf("parent inventory after bank-funded divide = %+v, want empty", bot.Inventory)
	}
	if colony.FoodBank != 0 || colony.OreBank != 0 {
		t.Fatalf("bank after divide = F%d O%d, want empty", colony.FoodBank, colony.OreBank)
	}

	var child *core.Bot
	for _, candidate := range g.Board.Bots {
		if candidate != nil && candidate != &bot {
			child = candidate
			break
		}
	}
	if child == nil {
		t.Fatalf("child bot not found")
	}
	if child.Inventory.Total() != 0 {
		t.Fatalf("child inventory = %+v, want empty", child.Inventory)
	}
}

func TestChildOfConnectedMemberStartsConnectedNearActiveController(t *testing.T) {
	cfg := config.NewConfig()
	cfg.DivisionCost = 7
	cfg.DivisionFoodCost = 1
	cfg.DivisionOreCost = 1
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	ctrlPos := util.NewPos(10, 10)
	botPos := ctrlPos.AddDir(core.Right)
	bot := core.NewBot(botPos)
	bot.Hp = cfg.DivisionMinHp + 20
	bot.Inventory = core.Inventory{Food: 1, Ore: 1}
	bot.Genome.Matrix[0] = int(core.OpDivide)
	bot.ConnnectedToColony = true
	colony := core.NewColony(ctrlPos)
	colony.AddFamily(&bot)

	g.Board.Bots[util.Idx(botPos)] = &bot
	g.Board.Set(botPos, &bot)
	g.Board.Set(ctrlPos, core.Controller{Pos: ctrlPos, Owner: &bot, Colony: &colony, Amount: 10})

	g.botAction(botPos, &bot)

	var child *core.Bot
	for _, candidate := range g.Board.Bots {
		if candidate != nil && candidate != &bot {
			child = candidate
			break
		}
	}
	if child == nil {
		t.Fatalf("child bot not found")
	}
	if child.Colony != &colony {
		t.Fatalf("child colony = %p, want %p", child.Colony, &colony)
	}
	if !child.ConnnectedToColony {
		t.Fatalf("child born near active controller was not connected")
	}
}

func TestConnectedColonyDivisionPrefersColonyTissue(t *testing.T) {
	cfg := config.NewConfig()
	cfg.ColonyCohesionChance = 0
	cfg.DivisionCost = 7
	cfg.DivisionFoodCost = 1
	cfg.DivisionOreCost = 1
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	parentPos := util.NewPos(24, 24)
	targetPos := parentPos.AddDir(core.Right)
	parent := core.NewBot(parentPos)
	parent.Hp = cfg.DivisionMinHp + 20
	parent.Inventory = core.Inventory{Food: 1, Ore: 1}
	parent.Genome.Matrix[0] = int(core.OpDivide)
	parent.ConnnectedToColony = true
	colony := core.NewColony(parentPos.AddRowCol(0, -3))
	colony.AddFamily(&parent)
	addTestBot(g, &parent)
	g.Board.Set(colony.Center, core.Controller{Pos: colony.Center, Owner: &parent, Colony: &colony, Amount: 10})
	g.Board.DepositPheromone(targetPos, core.PheromoneHome, 80, &colony)
	for _, pos := range []core.Position{targetPos.AddDir(core.Right), targetPos.AddDir(core.Up)} {
		member := core.NewBot(pos)
		member.ConnnectedToColony = true
		colony.AddFamily(&member)
		addTestBot(g, &member)
	}

	g.botAction(parentPos, &parent)

	child := findChildBot(t, g, &parent)
	if child.Pos != targetPos {
		t.Fatalf("child position = %v, want colony tissue target %v", child.Pos, targetPos)
	}
}

func TestChildBornInOwnedHomeTissueStartsConnected(t *testing.T) {
	cfg := config.NewConfig()
	cfg.ColonyCohesionChance = 0
	cfg.DivisionCost = 7
	cfg.DivisionFoodCost = 1
	cfg.DivisionOreCost = 1
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	parentPos := util.NewPos(28, 28)
	targetPos := parentPos.AddDir(core.Right)
	parent := core.NewBot(parentPos)
	parent.Hp = cfg.DivisionMinHp + 20
	parent.Inventory = core.Inventory{Food: 1, Ore: 1}
	parent.Genome.Matrix[0] = int(core.OpDivide)
	parent.ConnnectedToColony = true
	colony := core.NewColony(util.NewPos(28, 20))
	colony.AddFamily(&parent)
	addTestBot(g, &parent)
	g.Board.DepositPheromone(targetPos, core.PheromoneHome, 80, &colony)

	g.botAction(parentPos, &parent)

	child := findChildBot(t, g, &parent)
	if child.Colony != &colony {
		t.Fatalf("child colony = %p, want %p", child.Colony, &colony)
	}
	if !child.ConnnectedToColony {
		t.Fatalf("child born in owned home tissue was not connected")
	}
}

func findChildBot(t *testing.T, g *Game, parent *core.Bot) *core.Bot {
	t.Helper()
	for _, candidate := range g.Board.Bots {
		if candidate != nil && candidate.Parent == parent {
			return candidate
		}
	}
	t.Fatalf("child bot not found")
	return nil
}

func TestDivideFailsWhenPersonalAndBankResourcesInsufficient(t *testing.T) {
	cfg := config.NewConfig()
	cfg.DivisionFoodCost = 2
	cfg.DivisionOreCost = 3
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	botPos := util.NewPos(10, 10)
	bot := core.NewBot(botPos)
	bot.Hp = cfg.DivisionMinHp + 20
	bot.Inventory = core.Inventory{Food: 1}
	bot.Genome.Matrix[0] = int(core.OpDivide)
	colony := core.NewColony(util.NewPos(10, 9))
	colony.OreBank = 2
	colony.AddFamily(&bot)
	bot.ConnnectedToColony = true

	g.Board.Bots[util.Idx(botPos)] = &bot
	g.Board.Set(botPos, &bot)

	g.botAction(botPos, &bot)

	if got := g.liveBotCount(); got != 1 {
		t.Fatalf("live bots after failed bank divide = %d, want 1", got)
	}
	if bot.Inventory.Food != 1 || bot.Inventory.Ore != 0 {
		t.Fatalf("inventory after failed bank divide = %+v, want unchanged food 1 ore 0", bot.Inventory)
	}
	if colony.FoodBank != 0 || colony.OreBank != 2 {
		t.Fatalf("bank after failed divide = F%d O%d, want F0 O2", colony.FoodBank, colony.OreBank)
	}
	if bot.Genome.Pointer != 5 {
		t.Fatalf("pointer after failed bank divide = %d, want 5", bot.Genome.Pointer)
	}
}

func TestCheckInventoryCountsBankForConnectedBot(t *testing.T) {
	cfg := config.NewConfig()
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	botPos := util.NewPos(10, 10)
	bot := core.NewBot(botPos)
	bot.Inventory.Food = 2
	bot.Genome.Matrix[0] = int(core.OpCheckInventory)
	bot.Genome.Matrix[1] = int(core.OpEatOther)
	colony := core.NewColony(util.NewPos(10, 9))
	colony.FoodBank = 71
	colony.AddFamily(&bot)
	bot.ConnnectedToColony = true

	g.Board.Bots[util.Idx(botPos)] = &bot
	g.Board.Set(botPos, &bot)
	g.Board.Set(colony.Center, core.Controller{Pos: colony.Center, Owner: &bot, Colony: &colony, Amount: 10})

	g.botAction(botPos, &bot)

	if bot.Genome.NextArg != 73 {
		t.Fatalf("check inventory next arg = %d, want personal+bank food 73", bot.Genome.NextArg)
	}
}

func TestAttackTransfersFoodAndOre(t *testing.T) {
	cfg := config.NewConfig()
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	attackerPos := util.NewPos(10, 10)
	victimPos := attackerPos.AddDir(core.PosClock[0])
	attacker := core.NewBot(attackerPos)
	attacker.Hp = 200
	victim := core.NewBot(victimPos)
	victim.Hp = 10
	victim.Inventory = core.Inventory{Food: 2, Ore: 3}
	for i := range attacker.Genome.Matrix {
		attacker.Genome.Matrix[i] = 0
		victim.Genome.Matrix[i] = core.OpcodeCount() + i
	}
	attacker.Genome.Matrix[0] = int(core.OpAttack)
	attacker.Genome.Matrix[1] = 0

	g.Board.Bots[util.Idx(attackerPos)] = &attacker
	g.Board.Bots[util.Idx(victimPos)] = &victim
	g.Board.Set(attackerPos, &attacker)
	g.Board.Set(victimPos, &victim)

	g.botAction(attackerPos, &attacker)

	if got := g.liveBotCount(); got != 1 {
		t.Fatalf("live bots after attack = %d, want 1", got)
	}
	if attacker.Inventory.Food != 2 || attacker.Inventory.Ore != 3 {
		t.Fatalf("attacker inventory after attack = %+v, want food 2 ore 3", attacker.Inventory)
	}
	if attacker.Evolution.FoodGathered != 0 || attacker.Evolution.OreGathered != 0 {
		t.Fatalf("attack loot counted as gathered: %+v", attacker.Evolution)
	}
	if attacker.Evolution.StolenFood != 2 || attacker.Evolution.StolenOre != 3 || attacker.Evolution.CombatKills != 1 {
		t.Fatalf("attack loot telemetry = %+v, want stolen F2 O3 kills 1", attacker.Evolution)
	}
	if g.FoodGathered() != 0 || g.OreGathered() != 0 {
		t.Fatalf("game gathered after attack = F%d O%d, want zero", g.FoodGathered(), g.OreGathered())
	}
	if g.StolenFood() != 2 || g.StolenOre() != 3 || g.CombatKills() != 1 {
		t.Fatalf("game stolen after attack = F%d O%d kills %d, want F2 O3 kills 1", g.StolenFood(), g.StolenOre(), g.CombatKills())
	}
	if got := g.Board.GetBot(victimPos); got != nil {
		t.Fatalf("victim still on board: %v", got)
	}
}

func TestAttackDoesNotDamageFriendlyBots(t *testing.T) {
	cfg := config.NewConfig()
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	attackerPos := util.NewPos(10, 10)
	friendPos := attackerPos.AddDir(core.PosClock[0])
	attacker := core.NewBot(attackerPos)
	attacker.Hp = 200
	friend := core.NewBot(friendPos)
	friend.Hp = 10
	friend.Inventory = core.Inventory{Food: 2, Ore: 3}
	for i := range attacker.Genome.Matrix {
		attacker.Genome.Matrix[i] = 0
		friend.Genome.Matrix[i] = core.OpcodeCount() + i
	}
	attacker.Genome.Matrix[0] = int(core.OpAttack)
	attacker.Genome.Matrix[1] = 0
	colony := core.NewColony(util.NewPos(10, 9))
	colony.AddFamily(&attacker)
	colony.AddFamily(&friend)

	g.Board.Bots[util.Idx(attackerPos)] = &attacker
	g.Board.Bots[util.Idx(friendPos)] = &friend
	g.Board.Set(attackerPos, &attacker)
	g.Board.Set(friendPos, &friend)

	g.botAction(attackerPos, &attacker)

	if got := g.liveBotCount(); got != 2 {
		t.Fatalf("live bots after friendly attack = %d, want 2", got)
	}
	if attacker.Hp != 200 || attacker.Inventory.Total() != 0 {
		t.Fatalf("attacker changed after friendly attack: hp=%d inv=%+v", attacker.Hp, attacker.Inventory)
	}
	if friend.Hp != 10 || friend.Inventory.Food != 2 || friend.Inventory.Ore != 3 {
		t.Fatalf("friend changed after friendly attack: hp=%d inv=%+v", friend.Hp, friend.Inventory)
	}
	if got := g.Board.GetBot(friendPos); got != &friend {
		t.Fatalf("friend missing after friendly attack: %v", got)
	}
}

func TestGrabForeignControllerRaidsCappedBank(t *testing.T) {
	cfg := config.NewConfig()
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	attackerPos := util.NewPos(10, 10)
	attacker := core.NewBot(attackerPos)
	attacker.Dir = core.Up
	attackerColony := core.NewColony(attackerPos)
	attackerColony.AddFamily(&attacker)
	ctrlPos := attackerPos.AddRowCol(attacker.Dir[0], attacker.Dir[1])
	owner := core.NewBot(util.NewPos(10, 12))
	colony := core.NewColony(ctrlPos)
	colony.FoodBank = ControllerRaidFoodLimit + 7
	colony.OreBank = ControllerRaidOreLimit + 9
	colony.AddFamily(&owner)

	g.Board.Bots[util.Idx(attackerPos)] = &attacker
	g.Board.Set(attackerPos, &attacker)
	g.Board.Set(ctrlPos, core.Controller{
		Pos:    ctrlPos,
		Owner:  &owner,
		Colony: &colony,
		Amount: 42,
	})

	g.grab(attackerPos, &attacker)

	if attacker.Inventory.Food != ControllerRaidFoodLimit || attacker.Inventory.Ore != ControllerRaidOreLimit {
		t.Fatalf("attacker inventory after controller raid = %+v, want capped F%d O%d", attacker.Inventory, ControllerRaidFoodLimit, ControllerRaidOreLimit)
	}
	if attacker.Evolution.FoodGathered != 0 || attacker.Evolution.OreGathered != 0 {
		t.Fatalf("controller raid counted as gathered: %+v", attacker.Evolution)
	}
	if attacker.Evolution.StolenFood != ControllerRaidFoodLimit ||
		attacker.Evolution.StolenOre != ControllerRaidOreLimit ||
		attacker.Evolution.ControllerRaids != 1 {
		t.Fatalf("controller raid telemetry = %+v, want stolen F%d O%d raids 1", attacker.Evolution, ControllerRaidFoodLimit, ControllerRaidOreLimit)
	}
	if colony.FoodBank != 7 || colony.OreBank != 9 {
		t.Fatalf("bank after controller raid = F%d O%d, want F7 O9", colony.FoodBank, colony.OreBank)
	}
	ctrl, ok := g.Board.At(ctrlPos).(core.Controller)
	if !ok {
		t.Fatalf("controller after raid = %T, want Controller", g.Board.At(ctrlPos))
	}
	if ctrl.Amount != 42 || ctrl.Owner != &owner || ctrl.Colony != &colony {
		t.Fatalf("controller mutated after raid: %+v", ctrl)
	}
}

func TestAttackForeignControllerRaidsCappedBank(t *testing.T) {
	cfg := config.NewConfig()
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	attackerPos := util.NewPos(10, 10)
	ctrlPos := attackerPos.AddDir(core.PosClock[0])
	attacker := core.NewBot(attackerPos)
	attackerColony := core.NewColony(attackerPos)
	attackerColony.AddFamily(&attacker)
	attacker.Genome.Matrix[0] = int(core.OpAttack)
	attacker.Genome.Matrix[1] = 0
	owner := core.NewBot(util.NewPos(10, 12))
	colony := core.NewColony(ctrlPos)
	colony.FoodBank = ControllerRaidFoodLimit + 3
	colony.OreBank = ControllerRaidOreLimit + 4
	colony.AddFamily(&owner)

	g.Board.Bots[util.Idx(attackerPos)] = &attacker
	g.Board.Set(attackerPos, &attacker)
	g.Board.Set(ctrlPos, core.Controller{
		Pos:    ctrlPos,
		Owner:  &owner,
		Colony: &colony,
		Amount: 42,
	})

	g.botAction(attackerPos, &attacker)

	if attacker.Inventory.Food != ControllerRaidFoodLimit || attacker.Inventory.Ore != ControllerRaidOreLimit {
		t.Fatalf("attacker inventory after controller attack = %+v, want capped F%d O%d", attacker.Inventory, ControllerRaidFoodLimit, ControllerRaidOreLimit)
	}
	if g.FoodGathered() != 0 || g.OreGathered() != 0 {
		t.Fatalf("controller attack gathered totals = F%d O%d, want zero", g.FoodGathered(), g.OreGathered())
	}
	if g.StolenFood() != ControllerRaidFoodLimit || g.StolenOre() != ControllerRaidOreLimit || g.ControllerRaids() != 1 {
		t.Fatalf("controller attack stolen totals = F%d O%d raids %d, want F%d O%d raids 1",
			g.StolenFood(), g.StolenOre(), g.ControllerRaids(), ControllerRaidFoodLimit, ControllerRaidOreLimit)
	}
	if colony.FoodBank != 3 || colony.OreBank != 4 {
		t.Fatalf("bank after controller attack = F%d O%d, want F3 O4", colony.FoodBank, colony.OreBank)
	}
	if _, ok := g.Board.At(ctrlPos).(core.Controller); !ok {
		t.Fatalf("controller after attack = %T, want Controller", g.Board.At(ctrlPos))
	}
}

func TestColonylessBotCannotRaidControllerBank(t *testing.T) {
	cfg := config.NewConfig()
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	attackerPos := util.NewPos(10, 10)
	attacker := core.NewBot(attackerPos)
	ctrlPos := attackerPos.AddDir(core.Up)
	owner := core.NewBot(util.NewPos(10, 12))
	colony := core.NewColony(ctrlPos)
	colony.FoodBank = ControllerRaidFoodLimit
	colony.OreBank = ControllerRaidOreLimit
	colony.AddFamily(&owner)
	ctrl := core.Controller{
		Pos:    ctrlPos,
		Owner:  &owner,
		Colony: &colony,
		Amount: 42,
	}

	if g.raidController(&attacker, &ctrl) {
		t.Fatalf("colonyless bot raided controller bank")
	}
	if attacker.Inventory.Total() != 0 {
		t.Fatalf("attacker inventory = %+v, want empty", attacker.Inventory)
	}
	if colony.FoodBank != ControllerRaidFoodLimit || colony.OreBank != ControllerRaidOreLimit {
		t.Fatalf("bank after blocked raid = F%d O%d, want unchanged", colony.FoodBank, colony.OreBank)
	}
}

func TestControllerAndConnectedMemberEmitHomePheromone(t *testing.T) {
	cfg := config.NewConfig()
	cfg.PheromoneHomeDeposit = 24
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	ctrlPos := util.NewPos(10, 10)
	ownerPos := ctrlPos.AddDir(core.Up)
	owner := core.NewBot(ownerPos)
	owner.ConnnectedToColony = true
	colony := core.NewColony(ctrlPos)
	colony.AddFamily(&owner)
	g.Board.Bots[util.Idx(ownerPos)] = &owner
	g.Board.Set(ownerPos, &owner)
	ctrl := core.Controller{
		Pos:    ctrlPos,
		Owner:  &owner,
		Colony: &colony,
		Amount: 10,
	}
	g.Board.Set(ctrlPos, ctrl)

	g.handleController(&ctrl, ctrlPos)

	if got := g.Board.PheromoneAt(ctrlPos).Home; got != 24 {
		t.Fatalf("controller home pheromone = %d, want 24", got)
	}
	if got := g.Board.PheromoneAt(ownerPos).Home; got == 0 {
		t.Fatalf("connected member home pheromone = 0, want nonzero")
	}
	if owner := g.Board.PheromoneHomeOwnerAt(ctrlPos); owner != &colony {
		t.Fatalf("home owner = %p, want %p", owner, &colony)
	}
}

func TestConnectedColonyBotFollowsOwnedHomePheromoneBeforeGenomeMove(t *testing.T) {
	cfg := config.NewConfig()
	cfg.ColonyCohesionChance = 100
	cfg.ColonyHomeFollowThreshold = 8
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	botPos := util.NewPos(20, 20)
	bot := core.NewBot(botPos)
	bot.Hp = 200
	bot.Dir = core.Right
	bot.Genome.Matrix[0] = int(core.OpMove)
	bot.ConnnectedToColony = true
	colony := core.NewColony(util.NewPos(20, 10))
	colony.AddFamily(&bot)
	homePos := botPos.AddDir(core.Up)
	g.Board.DepositPheromone(homePos, core.PheromoneHome, 80, &colony)
	addTestBot(g, &bot)

	g.botsActions()

	if bot.Pos != homePos {
		t.Fatalf("bot position = %v, want owned home pheromone at %v", bot.Pos, homePos)
	}
}

func TestConnectedColonyBotFillsHigherAdjacencyNestCell(t *testing.T) {
	cfg := config.NewConfig()
	cfg.ColonyCohesionChance = 100
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	ctrlPos := util.NewPos(20, 20)
	botPos := ctrlPos.AddDir(core.Right)
	targetPos := botPos.AddDir(core.Right)
	bot := core.NewBot(botPos)
	bot.ConnnectedToColony = true
	colony := core.NewColony(ctrlPos)
	colony.AddFamily(&bot)
	addTestBot(g, &bot)
	g.Board.Set(ctrlPos, core.Controller{Pos: ctrlPos, Owner: &bot, Colony: &colony, Amount: 10})

	for _, pos := range []core.Position{
		targetPos.AddDir(core.Right),
		targetPos.AddDir(core.Up),
		targetPos.AddDir(core.Down),
	} {
		member := core.NewBot(pos)
		member.ConnnectedToColony = true
		colony.AddFamily(&member)
		addTestBot(g, &member)
	}

	if !g.tryColonyCohesion(botPos, &bot) {
		t.Fatalf("cohesion step did not act")
	}
	if bot.Pos != targetPos {
		t.Fatalf("bot position = %v, want dense target %v", bot.Pos, targetPos)
	}
}

func TestForeignHomePheromoneDoesNotGuideColonyCohesion(t *testing.T) {
	cfg := config.NewConfig()
	cfg.ColonyCohesionChance = 100
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	botPos := util.NewPos(20, 20)
	bot := core.NewBot(botPos)
	bot.ConnnectedToColony = true
	homeColony := core.NewColony(util.NewPos(20, 10))
	foreignColony := core.NewColony(util.NewPos(20, 30))
	homeColony.AddFamily(&bot)
	ownHome := botPos.AddDir(core.Left)
	foreignHome := botPos.AddDir(core.Right)
	g.Board.DepositPheromone(ownHome, core.PheromoneHome, 30, &homeColony)
	g.Board.DepositPheromone(foreignHome, core.PheromoneHome, 200, &foreignColony)
	addTestBot(g, &bot)

	if !g.tryColonyCohesion(botPos, &bot) {
		t.Fatalf("cohesion step did not act")
	}
	if bot.Pos != ownHome {
		t.Fatalf("bot followed %v, want own home %v instead of foreign %v", bot.Pos, ownHome, foreignHome)
	}
}

func TestConnectedColonyBotForagesBeforeReturningHome(t *testing.T) {
	cfg := config.NewConfig()
	cfg.ColonyCohesionChance = 100
	cfg.ColonyForageChance = 100
	cfg.ColonyFrontierChance = 0
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	botPos := util.NewPos(24, 24)
	foodPos := botPos.AddDir(core.Right)
	bot := core.NewBot(botPos)
	bot.ConnnectedToColony = true
	colony := core.NewColony(util.NewPos(24, 8))
	colony.AddFamily(&bot)
	g.Board.DepositPheromone(botPos, core.PheromoneHome, 40, &colony)
	addTestBot(g, &bot)
	g.Board.Set(foodPos, core.Food{Pos: foodPos, Amount: 2})

	if !g.tryColonyCohesion(botPos, &bot) {
		t.Fatalf("cohesion forage step did not act")
	}
	if bot.Inventory.Food != 2 {
		t.Fatalf("food inventory after forage = %d, want 2", bot.Inventory.Food)
	}
	if got := g.Board.At(foodPos); got != nil {
		t.Fatalf("food cell after forage = %T, want nil", got)
	}
}

func TestConnectedColonyBotPushesCrowdedFrontierOutward(t *testing.T) {
	cfg := config.NewConfig()
	cfg.ColonyCohesionChance = 100
	cfg.ColonyForageChance = 0
	cfg.ColonyFrontierChance = 100
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	ctrlPos := util.NewPos(30, 30)
	botPos := ctrlPos.AddDir(core.Right)
	targetPos := botPos.AddDir(core.Right)
	bot := core.NewBot(botPos)
	bot.ConnnectedToColony = true
	colony := core.NewColony(ctrlPos)
	colony.AddFamily(&bot)
	addTestBot(g, &bot)
	g.Board.Set(ctrlPos, core.Controller{Pos: ctrlPos, Owner: &bot, Colony: &colony, Amount: 10})
	g.Board.DepositPheromone(botPos, core.PheromoneHome, 80, &colony)

	for _, pos := range []core.Position{
		botPos.AddDir(core.Up),
		botPos.AddDir(core.Down),
	} {
		member := core.NewBot(pos)
		member.ConnnectedToColony = true
		colony.AddFamily(&member)
		addTestBot(g, &member)
	}

	if !g.tryColonyCohesion(botPos, &bot) {
		t.Fatalf("frontier cohesion step did not act")
	}
	if bot.Pos != targetPos {
		t.Fatalf("bot position = %v, want outward frontier %v", bot.Pos, targetPos)
	}
	if got := g.Board.PheromoneAt(targetPos).Home; got == 0 {
		t.Fatalf("frontier home pheromone = 0, want nonzero")
	}
	if owner := g.Board.PheromoneHomeOwnerAt(targetPos); owner != &colony {
		t.Fatalf("frontier home owner = %p, want %p", owner, &colony)
	}
}

func TestAttackAndControllerRaidEmitDangerPheromone(t *testing.T) {
	cfg := config.NewConfig()
	cfg.PheromoneEventDeposit = 48
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	attackerPos := util.NewPos(10, 10)
	victimPos := attackerPos.AddDir(core.PosClock[0])
	attacker := core.NewBot(attackerPos)
	attacker.Hp = 200
	victim := core.NewBot(victimPos)
	victim.Hp = 10
	for i := range attacker.Genome.Matrix {
		attacker.Genome.Matrix[i] = 0
		victim.Genome.Matrix[i] = core.OpcodeCount() + i
	}
	attacker.Genome.Matrix[0] = int(core.OpAttack)
	attacker.Genome.Matrix[1] = 0
	g.Board.Bots[util.Idx(attackerPos)] = &attacker
	g.Board.Bots[util.Idx(victimPos)] = &victim
	g.Board.Set(attackerPos, &attacker)
	g.Board.Set(victimPos, &victim)

	g.botAction(attackerPos, &attacker)

	if got := g.Board.PheromoneAt(attackerPos).Danger; got != 48 {
		t.Fatalf("attacker danger pheromone = %d, want 48", got)
	}
	if got := g.Board.PheromoneAt(victimPos).Danger; got != 48 {
		t.Fatalf("victim danger pheromone = %d, want 48", got)
	}

	raidPos := util.NewPos(14, 10)
	raider := core.NewBot(raidPos)
	raiderColony := core.NewColony(raidPos)
	raiderColony.AddFamily(&raider)
	ctrlPos := raidPos.AddDir(core.Up)
	owner := core.NewBot(util.NewPos(14, 12))
	colony := core.NewColony(ctrlPos)
	colony.FoodBank = 1
	colony.AddFamily(&owner)
	ctrl := core.Controller{Pos: ctrlPos, Owner: &owner, Colony: &colony}

	if !g.raidController(&raider, &ctrl) {
		t.Fatalf("expected controller raid")
	}
	if got := g.Board.PheromoneAt(ctrlPos).Danger; got != 48 {
		t.Fatalf("controller raid danger pheromone = %d, want 48", got)
	}
}

func TestPheromoneOpcodesEmitSenseAndFollow(t *testing.T) {
	cfg := config.NewConfig()
	cfg.PheromoneBotDeposit = 16
	cfg.PheromoneEmitHpCost = 1
	cfg.PheromoneSenseThreshold = 16
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	emitPos := util.NewPos(10, 10)
	emitter := core.NewBot(emitPos)
	emitter.Hp = 50
	emitter.Genome.Matrix[0] = int(core.OpEmitPheromone)
	emitter.Genome.Matrix[1] = int(core.PheromoneFood)
	g.Board.Bots[util.Idx(emitPos)] = &emitter
	g.Board.Set(emitPos, &emitter)

	g.botAction(emitPos, &emitter)

	if emitter.Hp != 49 {
		t.Fatalf("emitter hp after pheromone emit = %d, want 49", emitter.Hp)
	}
	if emitter.Genome.Pointer != 2 || emitter.Genome.NextArg != 16 {
		t.Fatalf("emitter genome pointer/next = %d/%d, want 2/16", emitter.Genome.Pointer, emitter.Genome.NextArg)
	}
	if got := g.Board.PheromoneAt(emitPos).Food; got != 16 {
		t.Fatalf("emitted food pheromone = %d, want 16", got)
	}

	sensePos := util.NewPos(12, 10)
	sensor := core.NewBot(sensePos)
	senseDirIdx := int(core.OpEatOther) % 8
	targetPos := sensePos.AddDir(util.PosClock[senseDirIdx])
	g.Board.DepositPheromone(targetPos, core.PheromoneFood, 20, nil)
	for i := range sensor.Genome.Matrix {
		sensor.Genome.Matrix[i] = int(core.OpEatOther)
	}
	sensor.Genome.Matrix[0] = int(core.OpSensePheromone)
	sensor.Genome.Matrix[1] = int(core.PheromoneFood)
	sensor.Genome.Matrix[2] = int(core.OpEatOther)
	g.Board.Bots[util.Idx(sensePos)] = &sensor
	g.Board.Set(sensePos, &sensor)

	g.botAction(sensePos, &sensor)

	if sensor.Genome.NextArg != 20 {
		t.Fatalf("sensed pheromone value = %d, want 20", sensor.Genome.NextArg)
	}
	if sensor.Genome.Pointer != 3 {
		t.Fatalf("sensor pointer after high branch and stop = %d, want 3", sensor.Genome.Pointer)
	}

	followPos := util.NewPos(16, 10)
	follower := core.NewBot(followPos)
	follower.Dir = core.Down
	follower.Genome.Matrix[0] = int(core.OpFollowPheromone)
	follower.Genome.Matrix[1] = int(core.PheromoneFood)
	follower.Genome.Matrix[2] = 0
	foodTrailPos := followPos.AddDir(core.Up)
	g.Board.DepositPheromone(foodTrailPos, core.PheromoneFood, 80, nil)
	g.Board.Bots[util.Idx(followPos)] = &follower
	g.Board.Set(followPos, &follower)

	g.botAction(followPos, &follower)

	if follower.Dir != core.Up {
		t.Fatalf("follower dir = %v, want up", follower.Dir)
	}
	if follower.Pos != foodTrailPos {
		t.Fatalf("follower pos = %v, want %v", follower.Pos, foodTrailPos)
	}
	if follower.Genome.NextArg != 80 {
		t.Fatalf("follow next arg = %d, want 80", follower.Genome.NextArg)
	}
}

func TestFollowDangerPheromoneAvoidModeChoosesLowerDanger(t *testing.T) {
	cfg := config.NewConfig()
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	botPos := util.NewPos(20, 20)
	bot := core.NewBot(botPos)
	bot.Genome.Matrix[0] = int(core.OpFollowPheromone)
	bot.Genome.Matrix[1] = int(core.PheromoneDanger)
	bot.Genome.Matrix[2] = 1
	targetPos := botPos.AddDir(core.Right)
	for _, dir := range core.PosClock {
		pos := botPos.AddDir(dir)
		if pos == targetPos {
			continue
		}
		g.Board.DepositPheromone(pos, core.PheromoneDanger, 50, nil)
	}
	g.Board.Bots[util.Idx(botPos)] = &bot
	g.Board.Set(botPos, &bot)

	g.botAction(botPos, &bot)

	if bot.Dir != core.Right {
		t.Fatalf("avoid danger dir = %v, want right", bot.Dir)
	}
	if bot.Pos != targetPos {
		t.Fatalf("avoid danger pos = %v, want %v", bot.Pos, targetPos)
	}
	if bot.Genome.NextArg != 0 {
		t.Fatalf("avoid danger next arg = %d, want lowest 0", bot.Genome.NextArg)
	}
}

func TestPheromonesCopyAcrossPopulateBoardAndResetClears(t *testing.T) {
	cfg := config.NewConfig()
	cfg.BotChance = 0
	cfg.ResourceChance = 0
	cfg.PoisonChance = 0
	cfg.OceansCount = 0
	g := NewGame(&cfg)
	g.Board = core.NewBoard()
	pos := util.NewPos(10, 10)
	g.Board.DepositPheromone(pos, core.PheromoneFood, 44, nil)

	g.populateBoard()

	if got := g.Board.PheromoneAt(pos).Food; got != 44 {
		t.Fatalf("pheromone after populateBoard = %d, want copied 44", got)
	}

	g.ResetSimulation()

	if got := g.Board.PheromoneTotals(); got.ActiveCells != 0 {
		t.Fatalf("pheromones after reset = %+v, want none", got)
	}
}

func TestInspectIncludesPheromoneValues(t *testing.T) {
	cfg := config.NewConfig()
	g := NewGame(&cfg)
	g.Board = core.NewBoard()
	pos := util.NewPos(10, 10)
	g.Board.DepositPheromone(pos, core.PheromoneFood, 12, nil)
	g.Board.DepositPheromone(pos, core.PheromoneDanger, 7, nil)

	report := g.inspectGodCell(pos)

	found := false
	for _, line := range report.Lines {
		if strings.Contains(line, "Phero F 12 O 0 H 0 D 7") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("inspect lines missing pheromones: %v", report.Lines)
	}
}

func TestShareInventorySelectsFoodOrOre(t *testing.T) {
	cases := []struct {
		name         string
		selector     int
		start        core.Inventory
		wantSender   core.Inventory
		wantReceiver core.Inventory
	}{
		{
			name:         "food",
			selector:     0,
			start:        core.Inventory{Food: 6, Ore: 9},
			wantSender:   core.Inventory{Food: 2, Ore: 9},
			wantReceiver: core.Inventory{Food: 4},
		},
		{
			name:         "ore",
			selector:     1,
			start:        core.Inventory{Food: 9, Ore: 6},
			wantSender:   core.Inventory{Food: 9, Ore: 2},
			wantReceiver: core.Inventory{Ore: 4},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.NewConfig()
			g := NewGame(&cfg)
			g.Board = core.NewBoard()

			senderPos := util.NewPos(10, 10)
			receiverPos := senderPos.AddDir(core.PosClock[0])
			sender := core.NewBot(senderPos)
			sender.Inventory = tc.start
			sender.Genome.Matrix[0] = int(core.OpShareInventory)
			sender.Genome.Matrix[1] = 0
			sender.Genome.Matrix[2] = 4
			sender.Genome.Matrix[3] = tc.selector
			receiver := core.NewBot(receiverPos)
			receiver.Genome = sender.Genome

			g.Board.Bots[util.Idx(senderPos)] = &sender
			g.Board.Bots[util.Idx(receiverPos)] = &receiver
			g.Board.Set(senderPos, &sender)
			g.Board.Set(receiverPos, &receiver)

			g.botAction(senderPos, &sender)

			if sender.Inventory != tc.wantSender {
				t.Fatalf("sender inventory = %+v, want %+v", sender.Inventory, tc.wantSender)
			}
			if receiver.Inventory != tc.wantReceiver {
				t.Fatalf("receiver inventory = %+v, want %+v", receiver.Inventory, tc.wantReceiver)
			}
		})
	}
}

func TestShareInventoryDoesNotFeedForeignBot(t *testing.T) {
	cfg := config.NewConfig()
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	senderPos := util.NewPos(10, 10)
	receiverPos := senderPos.AddDir(core.PosClock[0])
	sender := core.NewBot(senderPos)
	sender.Inventory = core.Inventory{Food: 9}
	sender.Genome.Matrix[0] = int(core.OpShareInventory)
	sender.Genome.Matrix[1] = 0
	sender.Genome.Matrix[2] = 4
	sender.Genome.Matrix[3] = 0
	receiver := core.NewBot(receiverPos)
	for i := range sender.Genome.Matrix {
		sender.Genome.Matrix[i] = 0
		receiver.Genome.Matrix[i] = core.OpcodeCount() + i
	}
	sender.Genome.Matrix[0] = int(core.OpShareInventory)
	sender.Genome.Matrix[1] = 0
	sender.Genome.Matrix[2] = 4
	sender.Genome.Matrix[3] = 0

	g.Board.Bots[util.Idx(senderPos)] = &sender
	g.Board.Bots[util.Idx(receiverPos)] = &receiver
	g.Board.Set(senderPos, &sender)
	g.Board.Set(receiverPos, &receiver)

	g.botAction(senderPos, &sender)

	if sender.Inventory.Food != 9 {
		t.Fatalf("sender food after foreign share = %d, want unchanged 9", sender.Inventory.Food)
	}
	if receiver.Inventory.Total() != 0 {
		t.Fatalf("foreign receiver inventory = %+v, want empty", receiver.Inventory)
	}
}

func TestShareHpDoesNotHealForeignBot(t *testing.T) {
	cfg := config.NewConfig()
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	senderPos := util.NewPos(10, 10)
	receiverPos := senderPos.AddDir(core.PosClock[0])
	sender := core.NewBot(senderPos)
	sender.Hp = 100
	receiver := core.NewBot(receiverPos)
	receiver.Hp = 50
	for i := range sender.Genome.Matrix {
		sender.Genome.Matrix[i] = 0
		receiver.Genome.Matrix[i] = core.OpcodeCount() + i
	}
	sender.Genome.Matrix[0] = int(core.OpShareHp)
	sender.Genome.Matrix[1] = 0
	sender.Genome.Matrix[2] = 10

	g.Board.Bots[util.Idx(senderPos)] = &sender
	g.Board.Bots[util.Idx(receiverPos)] = &receiver
	g.Board.Set(senderPos, &sender)
	g.Board.Set(receiverPos, &receiver)

	g.botAction(senderPos, &sender)

	if sender.Hp != 100 {
		t.Fatalf("sender hp after foreign share = %d, want unchanged 100", sender.Hp)
	}
	if receiver.Hp != 50 {
		t.Fatalf("foreign receiver hp = %d, want unchanged 50", receiver.Hp)
	}
}

func TestConnectedColonySharingAppliesResourceAndHpBonuses(t *testing.T) {
	cfg := config.NewConfig()
	cfg.ColonyShareInventoryBonus = 2
	cfg.ColonyShareHpBonus = 7
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	colony := core.NewColony(util.NewPos(10, 10))
	senderPos := util.NewPos(10, 11)
	receiverPos := senderPos.AddDir(core.Right)
	sender := core.NewBot(senderPos)
	receiver := core.NewBot(receiverPos)
	colony.AddFamily(&sender)
	colony.AddFamily(&receiver)
	sender.ConnnectedToColony = true
	receiver.ConnnectedToColony = true
	addTestBot(g, &sender)
	addTestBot(g, &receiver)

	sender.Inventory.Food = 6
	sender.Genome.Matrix[0] = int(core.OpShareInventory)
	sender.Genome.Matrix[1] = 2
	sender.Genome.Matrix[2] = 4
	sender.Genome.Matrix[3] = 0
	g.botAction(senderPos, &sender)

	if sender.Inventory.Food != 2 {
		t.Fatalf("sender food after colony share = %d, want 2", sender.Inventory.Food)
	}
	if receiver.Inventory.Food != 6 {
		t.Fatalf("receiver food after colony share = %d, want normal 4 plus bonus 2", receiver.Inventory.Food)
	}

	sender.Genome.Pointer = 0
	sender.Genome.Matrix[0] = int(core.OpShareHp)
	sender.Genome.Matrix[1] = 2
	sender.Genome.Matrix[2] = 10
	sender.Hp = 100
	receiver.Hp = 50
	g.botAction(senderPos, &sender)

	if sender.Hp != 90 {
		t.Fatalf("sender hp after colony share = %d, want 90", sender.Hp)
	}
	if receiver.Hp != 67 {
		t.Fatalf("receiver hp after colony share = %d, want normal 10 plus bonus 7", receiver.Hp)
	}
}

func TestSendSignalTargetsAdjacentBot(t *testing.T) {
	cfg := config.NewConfig()
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	senderPos := util.NewPos(10, 10)
	receiverPos := senderPos.AddDir(core.PosClock[0])
	sender := core.NewBot(senderPos)
	sender.Genome.Matrix[0] = int(core.OpSendSignal)
	sender.Genome.Matrix[1] = 0
	sender.Genome.Matrix[2] = int(core.OpEatOther)
	receiver := core.NewBot(receiverPos)

	g.Board.Bots[util.Idx(senderPos)] = &sender
	g.Board.Bots[util.Idx(receiverPos)] = &receiver
	g.Board.Set(senderPos, &sender)
	g.Board.Set(receiverPos, &receiver)

	g.botAction(senderPos, &sender)

	wantSignal := int(core.OpEatOther) % 4
	if receiver.Genome.Signal != wantSignal {
		t.Fatalf("receiver signal = %d, want %d", receiver.Genome.Signal, wantSignal)
	}
	if sender.Genome.NextArg != wantSignal {
		t.Fatalf("sender next arg = %d, want delivered signal %d", sender.Genome.NextArg, wantSignal)
	}
}

func TestGrabMineConsumesFiniteAmount(t *testing.T) {
	cfg := config.NewConfig()
	cfg.MineGrabGain = 30
	cfg.MineGrabHpCost = 10
	cfg.ResourceGrabHpGain = 150
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	botPos := util.NewPos(10, 10)
	bot := core.NewBot(botPos)
	bot.Dir = core.Up
	bot.Hp = 100
	minePos := botPos.AddRowCol(bot.Dir[0], bot.Dir[1])

	g.Board.Bots[util.Idx(botPos)] = &bot
	g.Board.Set(botPos, &bot)
	g.Board.Set(minePos, core.Mine{
		Pos:    minePos,
		Owner:  &bot,
		Amount: 45,
	})

	g.grab(botPos, &bot)

	if bot.Inventory.Ore != 30 {
		t.Fatalf("ore inventory after first mine grab = %d, want 30", bot.Inventory.Ore)
	}
	if bot.Hp != 90 {
		t.Fatalf("hp after first mine grab = %d, want 90", bot.Hp)
	}
	mine, ok := g.Board.At(minePos).(core.Mine)
	if !ok {
		t.Fatalf("mine missing after partial extraction")
	}
	if mine.Amount != 15 {
		t.Fatalf("mine amount after first grab = %d, want 15", mine.Amount)
	}

	g.grab(botPos, &bot)

	if bot.Inventory.Ore != 45 {
		t.Fatalf("ore inventory after second mine grab = %d, want 45", bot.Inventory.Ore)
	}
	if bot.Hp != 80 {
		t.Fatalf("hp after second mine grab = %d, want 80", bot.Hp)
	}
	if got := g.Board.At(minePos); got != nil {
		t.Fatalf("mine cell after depletion = %T, want nil", got)
	}
}

func TestBuildMineCreatesFiniteOre(t *testing.T) {
	cfg := config.NewConfig()
	cfg.MineBuildCost = 1
	cfg.MineGrabGain = 30
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	botPos, buildPos := firstBuildTargetForBiome(t, g.Board, core.BiomeNeutral)
	bot := core.NewBot(botPos)
	bot.Inventory.Ore = 1
	bot.Genome.Matrix[1] = 2
	bot.Genome.Matrix[2] = int(core.BuildMine)

	g.Board.Bots[util.Idx(botPos)] = &bot
	g.Board.Set(botPos, &bot)

	g.build(botPos, &bot)

	mine, ok := g.Board.At(buildPos).(core.Mine)
	if !ok {
		t.Fatalf("built cell = %T, want Mine", g.Board.At(buildPos))
	}
	if mine.Amount != 30 {
		t.Fatalf("built mine amount = %d, want 30", mine.Amount)
	}
	if bot.Inventory.Ore != 0 {
		t.Fatalf("ore inventory after mine build = %d, want 0", bot.Inventory.Ore)
	}
}

func TestBuildMineUsesBankOreForConnectedMember(t *testing.T) {
	cfg := config.NewConfig()
	cfg.MineBuildCost = 1
	cfg.MineGrabGain = 30
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	botPos, buildPos := firstBuildTargetForBiome(t, g.Board, core.BiomeNeutral)
	bot := core.NewBot(botPos)
	bot.Genome.Matrix[1] = 2
	bot.Genome.Matrix[2] = int(core.BuildMine)
	colony := core.NewColony(util.NewPos(10, 9))
	colony.OreBank = 1
	colony.AddFamily(&bot)
	bot.ConnnectedToColony = true

	g.Board.Bots[util.Idx(botPos)] = &bot
	g.Board.Set(botPos, &bot)
	g.Board.Set(botPos.AddRowCol(0, -2), core.Controller{
		Pos:    botPos.AddRowCol(0, -2),
		Owner:  &bot,
		Colony: &colony,
		Amount: 10,
	})

	g.build(botPos, &bot)

	if _, ok := g.Board.At(buildPos).(core.Mine); !ok {
		t.Fatalf("built cell = %T, want Mine", g.Board.At(buildPos))
	}
	if bot.Inventory.Ore != 0 {
		t.Fatalf("personal ore after bank mine build = %d, want 0", bot.Inventory.Ore)
	}
	if colony.OreBank != 0 {
		t.Fatalf("ore bank after mine build = %d, want 0", colony.OreBank)
	}
}

func TestBiomeSpawnProfilesBiasEcology(t *testing.T) {
	cfg := config.NewConfig()
	cfg.ResourceChance = 6
	cfg.PoisonChance = 4
	g := NewGame(&cfg)

	neutral := g.biomeSpawnProfile(core.BiomeNeutral)
	fertile := g.biomeSpawnProfile(core.BiomeFertile)
	mineral := g.biomeSpawnProfile(core.BiomeMineral)
	toxic := g.biomeSpawnProfile(core.BiomeToxic)

	if fertile.FoodChance <= neutral.FoodChance {
		t.Fatalf("fertile food chance = %d, want above neutral %d", fertile.FoodChance, neutral.FoodChance)
	}
	if fertile.PoisonChance >= neutral.PoisonChance {
		t.Fatalf("fertile poison chance = %d, want below neutral %d", fertile.PoisonChance, neutral.PoisonChance)
	}
	if fertile.ResourceChance >= neutral.ResourceChance {
		t.Fatalf("fertile resource chance = %d, want below neutral %d", fertile.ResourceChance, neutral.ResourceChance)
	}
	if mineral.ResourceChance <= neutral.ResourceChance || mineral.ResourceAmount <= neutral.ResourceAmount {
		t.Fatalf("mineral profile = %+v, want denser/larger than neutral %+v", mineral, neutral)
	}
	if toxic.PoisonChance <= neutral.PoisonChance || toxic.FoodChance > fertile.FoodChance {
		t.Fatalf("toxic profile = %+v, neutral=%+v fertile=%+v", toxic, neutral, fertile)
	}
}

func TestWaterGenerationIsDeterministicGroupedAndIrregular(t *testing.T) {
	cfg := config.NewConfig()
	cfg.OceansCount = 15

	first := generatedWaterSnapshot(&cfg, 42)
	second := generatedWaterSnapshot(&cfg, 42)

	if !slices.Equal(first.Cells, second.Cells) {
		t.Fatalf("water generation changed for same seed")
	}
	if first.Total == 0 {
		t.Fatalf("water generation produced no water")
	}
	if first.Total < 1500 || first.Total > 15000 {
		t.Fatalf("water cells = %d, want bounded default-like coverage", first.Total)
	}
	if len(first.Groups) < 4 {
		t.Fatalf("water group count = %d, want multiple bodies", len(first.Groups))
	}
	if irregularWaterGroups(first.Groups) < 3 {
		t.Fatalf("water groups did not produce enough ragged bodies: %+v", first.Groups)
	}
}

func TestOreSpawnDistributionFavorsMineralVeins(t *testing.T) {
	cfg := config.NewConfig()
	cfg.BotChance = 0
	cfg.OceansCount = 0
	cfg.PoisonChance = 0
	cfg.ResourceChance = 8
	seedTerrainRNG(42)

	g := NewGame(&cfg)
	g.InitializeForCommands()

	type biomeCounts struct {
		cells int
		ore   int
	}
	counts := map[core.Biome]biomeCounts{}
	mineralVeinCells := 0
	mineralVeinOre := 0
	totalOre := 0
	for idx, cell := range *g.Board.GetGrid() {
		pos := util.PosOf(idx)
		biome := g.Board.BiomeAt(pos)
		count := counts[biome]
		count.cells++
		if _, ok := cell.(core.Resource); ok {
			count.ore++
			totalOre++
			if biome == core.BiomeMineral && core.OreVeinScore(pos) >= 52 {
				mineralVeinOre++
			}
		}
		counts[biome] = count
		if biome == core.BiomeMineral && core.OreVeinScore(pos) >= 52 {
			mineralVeinCells++
		}
	}

	mineral := counts[core.BiomeMineral]
	fertile := counts[core.BiomeFertile]
	if totalOre == 0 || mineral.ore == 0 || fertile.ore == 0 || mineralVeinCells == 0 {
		t.Fatalf("unexpected ore distribution: total=%d mineral=%+v fertile=%+v mineralVeinCells=%d", totalOre, mineral, fertile, mineralVeinCells)
	}

	mineralDensity := mineral.ore * 10000 / mineral.cells
	fertileDensity := fertile.ore * 10000 / fertile.cells
	mineralVeinDensity := mineralVeinOre * 10000 / mineralVeinCells
	if mineralDensity < fertileDensity*8 {
		t.Fatalf("mineral ore density = %d, fertile = %d, want at least 8x", mineralDensity, fertileDensity)
	}
	if mineralVeinDensity < fertileDensity*20 {
		t.Fatalf("mineral vein ore density = %d, fertile = %d, want at least 20x", mineralVeinDensity, fertileDensity)
	}
	if mineralVeinOre*2 <= totalOre {
		t.Fatalf("mineral vein ore = %d of total %d, want majority", mineralVeinOre, totalOre)
	}
}

type waterSnapshot struct {
	Cells  []int
	Total  int
	Groups map[int]*waterGroupStats
}

type waterGroupStats struct {
	Cells     int
	MinR      int
	MaxR      int
	MinC      int
	MaxC      int
	RowCounts map[int]int
}

func generatedWaterSnapshot(cfg *config.Config, seed int64) waterSnapshot {
	seedTerrainRNG(seed)
	g := NewGame(cfg)
	g.Board = core.NewBoard()
	g.generateWater()

	out := waterSnapshot{
		Cells:  make([]int, util.Cells),
		Groups: map[int]*waterGroupStats{},
	}
	for i := range out.Cells {
		out.Cells[i] = -1
	}
	for idx, cell := range *g.Board.GetGrid() {
		water, ok := cell.(core.Water)
		if !ok {
			continue
		}
		pos := util.PosOf(idx)
		out.Cells[idx] = water.GroupId
		out.Total++
		group := out.Groups[water.GroupId]
		if group == nil {
			group = &waterGroupStats{
				MinR:      core.Rows,
				MaxR:      -1,
				MinC:      core.Cols,
				MaxC:      -1,
				RowCounts: map[int]int{},
			}
			out.Groups[water.GroupId] = group
		}
		group.Cells++
		group.MinR = min(group.MinR, pos.R)
		group.MaxR = max(group.MaxR, pos.R)
		group.MinC = min(group.MinC, pos.C)
		group.MaxC = max(group.MaxC, pos.C)
		group.RowCounts[pos.R]++
	}
	return out
}

func seedTerrainRNG(seed int64) {
	rand.Seed(seed)
	expRand.Seed(uint64(seed))
}

func irregularWaterGroups(groups map[int]*waterGroupStats) int {
	irregular := 0
	for _, group := range groups {
		if group.Cells < 35 {
			continue
		}
		boundsArea := (group.MaxR - group.MinR + 1) * (group.MaxC - group.MinC + 1)
		if boundsArea <= 0 {
			continue
		}
		minRowWidth := core.Cols
		maxRowWidth := 0
		for _, rowWidth := range group.RowCounts {
			minRowWidth = min(minRowWidth, rowWidth)
			maxRowWidth = max(maxRowWidth, rowWidth)
		}
		fillPermille := group.Cells * 1000 / boundsArea
		if fillPermille < 700 && maxRowWidth-minRowWidth >= 3 {
			irregular++
		}
	}
	return irregular
}

func TestBuildMineMineralBiomeCreatesLargerDeposit(t *testing.T) {
	cfg := config.NewConfig()
	cfg.MineBuildCost = 1
	cfg.MineGrabGain = 30
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	botPos, buildPos := firstBuildTargetForBiome(t, g.Board, core.BiomeMineral)
	bot := core.NewBot(botPos)
	bot.Inventory.Ore = 1
	bot.Genome.Matrix[1] = 2
	bot.Genome.Matrix[2] = int(core.BuildMine)

	g.Board.Bots[util.Idx(botPos)] = &bot
	g.Board.Set(botPos, &bot)

	g.build(botPos, &bot)

	mine, ok := g.Board.At(buildPos).(core.Mine)
	if !ok {
		t.Fatalf("built cell = %T, want Mine", g.Board.At(buildPos))
	}
	if mine.Amount != 60 {
		t.Fatalf("mineral mine amount = %d, want 60", mine.Amount)
	}
	if bot.Inventory.Ore != 0 {
		t.Fatalf("ore inventory after mine build = %d, want 0", bot.Inventory.Ore)
	}
}

func TestFertileFarmProducesExtraFood(t *testing.T) {
	cfg := config.NewConfig()
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	farmPos := firstBiomeCell(t, g.Board, core.BiomeFertile)
	g.Board.Set(farmPos, core.Farm{Pos: farmPos, Amount: 1})

	g.environmentActions()

	if got := countFoodAround(g.Board, farmPos); got != 2 {
		t.Fatalf("fertile farm produced %d food cells, want 2", got)
	}
	farm, ok := g.Board.At(farmPos).(core.Farm)
	if !ok {
		t.Fatalf("farm missing after production")
	}
	if farm.Amount != 0 {
		t.Fatalf("farm amount after production = %d, want 0", farm.Amount)
	}
}

func TestColonyFarmBuildAndOutputBonuses(t *testing.T) {
	cfg := config.NewConfig()
	cfg.FarmBuildCost = 1
	cfg.FarmInitialAmount = 1
	cfg.ColonyFarmChargeBonus = 2
	cfg.ColonyFarmOutputBonus = 2
	cfg.FertileFoodRegrowPeriod = 0
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	botPos, farmPos := firstBuildTargetForBiome(t, g.Board, core.BiomeNeutral)
	colony := core.NewColony(farmPos.AddRowCol(0, -2))
	bot := core.NewBot(botPos)
	bot.Inventory.Ore = cfg.FarmBuildCost
	bot.ConnnectedToColony = true
	bot.Genome.Matrix[1] = 2
	bot.Genome.Matrix[2] = int(core.BuildFarm)
	colony.AddFamily(&bot)
	addTestBot(g, &bot)

	g.build(botPos, &bot)

	farm, ok := g.Board.At(farmPos).(core.Farm)
	if !ok {
		t.Fatalf("built cell = %T, want Farm", g.Board.At(farmPos))
	}
	if farm.Amount != 3 {
		t.Fatalf("farm initial amount = %d, want base 1 plus bonus 2", farm.Amount)
	}
	if farm.Colony != &colony {
		t.Fatalf("farm colony = %p, want %p", farm.Colony, &colony)
	}

	g.environmentActions()

	if got := countFoodAround(g.Board, farmPos); got != 3 {
		t.Fatalf("colony farm produced %d food cells, want base 1 plus bonus 2", got)
	}
	farm, ok = g.Board.At(farmPos).(core.Farm)
	if !ok {
		t.Fatalf("farm missing after production")
	}
	if farm.Amount != 2 {
		t.Fatalf("farm amount after production = %d, want 2", farm.Amount)
	}
}

func TestFertileBiomeRegrowsFood(t *testing.T) {
	cfg := config.NewConfig()
	cfg.FertileFoodRegrowPeriod = 1
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	fertilePos := firstBiomeCell(t, g.Board, core.BiomeFertile)
	g.environmentActions()

	if _, ok := g.Board.At(fertilePos).(core.Food); !ok {
		t.Fatalf("fertile biome cell = %T, want Food", g.Board.At(fertilePos))
	}
}

func TestNeutralBiomeDoesNotRegrowFood(t *testing.T) {
	cfg := config.NewConfig()
	cfg.FertileFoodRegrowPeriod = 1
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	neutralPos := firstBiomeCell(t, g.Board, core.BiomeNeutral)
	g.environmentActions()

	if got := g.Board.At(neutralPos); got != nil {
		t.Fatalf("neutral biome cell = %T, want nil", got)
	}
}

func TestLowPopulationDoesNotGenerationSeedFromStrongestSurvivor(t *testing.T) {
	cfg := config.NewConfig()
	cfg.BotChance = 100
	cfg.NewGenThreshold = 5
	cfg.ImmigrationInterval = 0
	cfg.PoisonChance = 0
	cfg.ResourceChance = 0
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	spawnPos := util.NewPos(10, 10)
	weakerPos := util.NewPos(15, 15)
	championPos := util.NewPos(20, 20)
	freezeAllExcept(t, g.Board, spawnPos, weakerPos, championPos)

	weaker := core.NewBot(weakerPos)
	weaker.Hp = 250
	weaker.Genome = testGenerationGenome(11)
	weaker.Genome.Matrix[0] = int(core.OpPhoto)
	champion := core.NewBot(championPos)
	champion.Hp = 250
	champion.Inventory = core.Inventory{Food: 4, Ore: 4}
	champion.Genome = testGenerationGenome(42)
	champion.Genome.Matrix[0] = int(core.OpPhoto)

	g.Board.Bots[util.Idx(weakerPos)] = &weaker
	g.Board.Bots[util.Idx(championPos)] = &champion
	g.Board.Set(weakerPos, &weaker)
	g.Board.Set(championPos, &champion)

	g.runLogicTick()

	if spawned := g.Board.GetBot(spawnPos); spawned != nil {
		t.Fatalf("low population spawned generation bot at %v: %+v", spawnPos, spawned)
	}
	if g.Board.GetBot(championPos) != &champion {
		t.Fatalf("champion survivor was replaced during low-population tick")
	}
	if !g.hasGenerationSeedGenome {
		t.Fatalf("low-population survivor should still be remembered as a future extinction seed")
	}
}

func TestLowPopulationSpawnsRandomImmigrants(t *testing.T) {
	cfg := config.NewConfig()
	cfg.NewGenThreshold = 5
	cfg.ImmigrationInterval = 1
	cfg.ImmigrationBots = 1
	cfg.PoisonChance = 0
	cfg.ResourceChance = 0
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	survivorPos := util.NewPos(20, 20)
	immigrantPos := util.NewPos(10, 10)
	freezeAllExcept(t, g.Board, survivorPos, immigrantPos)

	survivor := core.NewBot(survivorPos)
	survivor.Hp = 250
	survivor.Genome.Matrix[0] = int(core.OpPhoto)
	g.Board.Bots[util.Idx(survivorPos)] = &survivor
	g.Board.Set(survivorPos, &survivor)

	g.runLogicTick()

	if g.Board.GetBot(survivorPos) != &survivor {
		t.Fatalf("survivor was replaced during immigration")
	}
	immigrant := g.Board.GetBot(immigrantPos)
	if immigrant == nil {
		t.Fatalf("expected random immigrant at %v", immigrantPos)
	}
	if immigrant.Parent != nil || immigrant.LineageDepth != 0 || immigrant.Divisions != 0 {
		t.Fatalf("immigrant should be a fresh random bot, got parent=%p depth=%d divisions=%d", immigrant.Parent, immigrant.LineageDepth, immigrant.Divisions)
	}
}

func TestConnectedColonyPhotoUsesConfiguredHpAndAddsFood(t *testing.T) {
	cfg := config.NewConfig()
	cfg.PhotoHpGain = 7
	cfg.ColonyPhotoFoodGain = 3
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	ctrlPos := util.NewPos(20, 20)
	botPos := util.NewPos(20, 21)
	colony := core.NewColony(ctrlPos)
	bot := core.NewBot(botPos)
	bot.Hp = 100
	bot.Genome.Matrix[0] = int(core.OpPhoto)
	colony.AddFamily(&bot)
	bot.ConnnectedToColony = true
	addTestBot(g, &bot)
	g.Board.Set(ctrlPos, core.Controller{Pos: ctrlPos, Owner: &bot, Colony: &colony, Amount: 10})

	g.botAction(botPos, &bot)

	if bot.Hp != 107 {
		t.Fatalf("bot hp after photo = %d, want configured gain to 107", bot.Hp)
	}
	if bot.Inventory.Food != 3 || bot.Evolution.FoodGathered != 3 || g.FoodGathered() != 3 {
		t.Fatalf("photo food accounting = inv %+v evo %+v total %d, want 3", bot.Inventory, bot.Evolution, g.FoodGathered())
	}
}

func TestBotDiesWhenPastMaxBotAge(t *testing.T) {
	cfg := config.NewConfig()
	cfg.MaxBotAge = 1
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	pos := util.NewPos(20, 20)
	bot := core.NewBot(pos)
	bot.Age = cfg.MaxBotAge
	bot.Hp = 500
	bot.Genome.Matrix[0] = int(core.OpPhoto)
	g.Board.Bots[util.Idx(pos)] = &bot
	g.Board.Set(pos, &bot)

	g.botsActions()

	if got := g.Board.GetBot(pos); got != nil {
		t.Fatalf("aged bot still live at %v: %+v", pos, got)
	}
	if _, ok := g.Board.At(pos).(*core.Bot); ok {
		t.Fatalf("aged bot still occupies grid cell")
	}
}

func TestColonyHeartProtectionFloorsHpAndSuppressesCoreAgeDeath(t *testing.T) {
	cfg := config.NewConfig()
	cfg.MaxBotAge = 1
	cfg.ColonyHeartRadius = 5
	cfg.ColonyHeartImmortalRadius = 2
	cfg.ColonyHeartMinHp = 100
	cfg.ColonyHeartMaxHp = 500
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	ctrlPos := util.NewPos(20, 20)
	botPos := ctrlPos.AddRowCol(0, 1)
	colony := core.NewColony(ctrlPos)
	bot := core.NewBot(botPos)
	bot.Age = cfg.MaxBotAge
	bot.Hp = 1
	bot.ConnnectedToColony = true
	bot.CurrTask = &core.ColonyTask{Type: core.MaintainConnectionTask, IsDone: true}
	colony.AddFamily(&bot)
	addTestBot(g, &bot)
	g.Board.Set(ctrlPos, core.Controller{Pos: ctrlPos, Owner: &bot, Colony: &colony, Amount: 10})

	g.botsActions()

	live := g.Board.GetBot(botPos)
	if live != &bot {
		t.Fatalf("heart-protected bot was removed: got %p want %p", live, &bot)
	}
	if bot.Age != cfg.MaxBotAge+1 {
		t.Fatalf("bot age after protected tick = %d, want %d", bot.Age, cfg.MaxBotAge+1)
	}
	if bot.Hp != 420 {
		t.Fatalf("heart-protected bot hp = %d, want distance-scaled floor 420", bot.Hp)
	}
}

func TestGenerationChampionPrefersBalancedProgressOverHpOnly(t *testing.T) {
	highHP := core.NewBot(util.NewPos(10, 10))
	highHP.Hp = 500
	highHP.Inventory = core.Inventory{Ore: 20}

	balanced := core.NewBot(util.NewPos(10, 11))
	balanced.Hp = 120
	balanced.Inventory = core.Inventory{Food: 1, Ore: 1}
	balanced.Evolution.FoodGathered = 10
	balanced.Evolution.OreGathered = 10
	balanced.Evolution.FarmBuilds = 1

	if !generationChampionRanksBefore(&balanced, &highHP) {
		t.Fatalf("balanced progress should outrank high-HP ore-only survivor")
	}
	if generationChampionRanksBefore(&highHP, &balanced) {
		t.Fatalf("high-HP ore-only survivor should not outrank balanced progress")
	}
}

func TestGenerationChampionPrefersReproductiveLineage(t *testing.T) {
	highHPBalanced := core.NewBot(util.NewPos(10, 10))
	highHPBalanced.Hp = 500
	highHPBalanced.Inventory = core.Inventory{Food: 20, Ore: 20}

	reproductive := core.NewBot(util.NewPos(10, 11))
	reproductive.Hp = 80
	reproductive.Divisions = 1

	if !generationChampionRanksBefore(&reproductive, &highHPBalanced) {
		t.Fatalf("reproductive lineage should outrank high-HP balanced bot")
	}
	if generationChampionRanksBefore(&highHPBalanced, &reproductive) {
		t.Fatalf("high-HP balanced bot should not outrank reproductive lineage")
	}
}

func TestBotEvolutionScoreRewardsConnectedColonyOverSoloDestroyer(t *testing.T) {
	cfg := config.NewConfig()
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	solo := core.NewBot(util.NewPos(10, 10))
	solo.Divisions = 50
	solo.LineageDepth = 25
	solo.Inventory = core.Inventory{Food: 100, Ore: 100}

	ctrlPos := util.NewPos(20, 20)
	owner := core.NewBot(ctrlPos.AddDir(core.Right))
	owner.ConnnectedToColony = true
	member := core.NewBot(ctrlPos.AddDir(core.Left))
	member.ConnnectedToColony = true
	member.Divisions = 1
	member.Evolution.ControllerBuilds = 1
	member.Evolution.TaskCompletions = 1
	colony := core.NewColony(ctrlPos)
	colony.AddFamily(&owner)
	colony.AddFamily(&member)

	for _, bot := range []*core.Bot{&solo, &owner, &member} {
		g.Board.Bots[util.Idx(bot.Pos)] = bot
		g.Board.Set(bot.Pos, bot)
	}
	g.Board.Set(ctrlPos, core.Controller{Pos: ctrlPos, Owner: &owner, Colony: &colony, Amount: 100})

	if soloScore, colonyScore := g.BotEvolutionScore(&solo), g.BotEvolutionScore(&member); colonyScore <= soloScore {
		t.Fatalf("colony score = %d, solo score = %d; want colony to outrank capped solo destroyer", colonyScore, soloScore)
	}
}

func TestRememberGenerationChampionKeepsBestReproductiveSeed(t *testing.T) {
	cfg := config.NewConfig()
	g := NewGame(&cfg)

	reproductive := core.NewBot(util.NewPos(10, 10))
	reproductive.Hp = 80
	reproductive.Divisions = 2
	reproductive.LineageDepth = 3
	reproductive.Genome = testGenerationGenome(21)

	walker := core.NewBot(util.NewPos(10, 11))
	walker.Hp = 500
	walker.Inventory = core.Inventory{Food: 20, Ore: 20}
	walker.Genome = testGenerationGenome(42)

	g.rememberGenerationChampion(&reproductive)
	g.rememberGenerationChampion(&walker)

	if g.generationSeedGenome.Matrix != reproductive.Genome.Matrix {
		t.Fatalf("reproductive seed was overwritten by non-reproductive walker")
	}
}

func TestRememberGenerationChampionReplacesSeedWithBetterReproductiveLineage(t *testing.T) {
	cfg := config.NewConfig()
	g := NewGame(&cfg)

	weaker := core.NewBot(util.NewPos(10, 10))
	weaker.Divisions = 1
	weaker.Genome = testGenerationGenome(21)

	stronger := core.NewBot(util.NewPos(10, 11))
	stronger.LineageDepth = 4
	stronger.Genome = testGenerationGenome(42)

	g.rememberGenerationChampion(&weaker)
	g.rememberGenerationChampion(&stronger)

	if g.generationSeedGenome.Matrix != stronger.Genome.Matrix {
		t.Fatalf("better reproductive lineage did not replace seed")
	}
}

func TestElitePoolDeduplicatesAndKeepsDeterministicOrder(t *testing.T) {
	cfg := config.NewConfig()
	cfg.EvolutionEliteCount = 2
	g := NewGame(&cfg)

	sharedGenome := testGenerationGenome(21)
	weakDuplicate := core.NewBot(util.NewPos(10, 20))
	weakDuplicate.Hp = 50
	weakDuplicate.Genome = sharedGenome

	strongDuplicate := core.NewBot(util.NewPos(10, 21))
	strongDuplicate.Divisions = 1
	strongDuplicate.Genome = sharedGenome

	earlierTie := core.NewBot(util.NewPos(10, 5))
	earlierTie.Divisions = 1
	earlierTie.Genome = testGenerationGenome(42)

	lateTie := core.NewBot(util.NewPos(10, 25))
	lateTie.Divisions = 1
	lateTie.Genome = testGenerationGenome(57)

	g.rememberGenerationChampion(&weakDuplicate)
	g.rememberGenerationChampion(&strongDuplicate)
	g.rememberGenerationChampion(&lateTie)
	g.rememberGenerationChampion(&earlierTie)

	if len(g.eliteGenomes) != 2 {
		t.Fatalf("elite count = %d, want 2", len(g.eliteGenomes))
	}
	if g.eliteGenomes[0].genome.Matrix != earlierTie.Genome.Matrix {
		t.Fatalf("first elite did not use deterministic board-index tie-break")
	}
	if g.eliteGenomes[1].genome.Matrix != strongDuplicate.Genome.Matrix {
		t.Fatalf("duplicate genome was not retained with its better rank")
	}
	if g.generationSeedGenome.Matrix != earlierTie.Genome.Matrix {
		t.Fatalf("generation seed was not synced to best elite")
	}
}

func TestElitePoolPreservesColonyLinkedGenomesUnderQuota(t *testing.T) {
	cfg := config.NewConfig()
	cfg.EvolutionEliteCount = 4
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	for n := 0; n < 4; n++ {
		solo := core.NewBot(util.NewPos(10, 10+n))
		solo.Divisions = 20 + n
		solo.Genome = testGenerationGenome(100 + n*10)
		g.rememberGenerationChampion(&solo)
	}

	for n := 0; n < 2; n++ {
		colonyBot := core.NewBot(util.NewPos(20, 20+n))
		colony := core.NewColony(util.NewPos(20, 30+n))
		colony.AddFamily(&colonyBot)
		colonyBot.Genome = testGenerationGenome(200 + n*10)
		g.rememberGenerationChampion(&colonyBot)
	}

	if len(g.eliteGenomes) != 4 {
		t.Fatalf("elite count = %d, want 4", len(g.eliteGenomes))
	}
	colonyLinked := 0
	for _, elite := range g.eliteGenomes {
		if elite.rank.colonyLinked {
			colonyLinked++
		}
	}
	if colonyLinked < 2 {
		t.Fatalf("colony-linked elites = %d, want at least 2 in top-4 pool: %+v", colonyLinked, g.eliteGenomes)
	}
	if !g.preferredEliteSeed(0).rank.colonyLinked || !g.preferredEliteSeed(1).rank.colonyLinked {
		t.Fatalf("preferred elite seeds should start with colony-linked genomes")
	}
}

func TestSmartGenerationSeedingUsesElitePercentRoundRobin(t *testing.T) {
	rand.Seed(7)

	cfg := config.NewConfig()
	cfg.BotChance = 100
	cfg.EvolutionEliteCount = 2
	cfg.EvolutionSeedPercent = 75
	cfg.MutationRate = 8
	cfg.PoisonChance = 0
	cfg.ResourceChance = 0
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	spawnPositions := []core.Position{
		util.NewPos(10, 10),
		util.NewPos(10, 11),
		util.NewPos(10, 12),
		util.NewPos(10, 13),
	}
	freezeAllExcept(t, g.Board, spawnPositions...)

	firstElite := core.NewBot(util.NewPos(20, 20))
	firstElite.Divisions = 2
	firstElite.Genome = testGenerationGenome(11)
	secondElite := core.NewBot(util.NewPos(20, 21))
	secondElite.Divisions = 1
	secondElite.Genome = testGenerationGenome(31)
	g.rememberGenerationChampion(&firstElite)
	g.rememberGenerationChampion(&secondElite)

	g.initialBotsGeneration()

	firstSpawn := g.Board.GetBot(spawnPositions[0])
	secondSpawn := g.Board.GetBot(spawnPositions[1])
	thirdSpawn := g.Board.GetBot(spawnPositions[2])
	fourthSpawn := g.Board.GetBot(spawnPositions[3])
	if firstSpawn == nil || secondSpawn == nil || thirdSpawn == nil || fourthSpawn == nil {
		t.Fatalf("expected four generation spawns, got %v %v %v %v", firstSpawn, secondSpawn, thirdSpawn, fourthSpawn)
	}
	if firstSpawn.Genome.Matrix != firstElite.Genome.Matrix {
		t.Fatalf("first elite copy was not exact")
	}
	if secondSpawn.Genome.Matrix != secondElite.Genome.Matrix {
		t.Fatalf("second elite copy was not exact")
	}
	if thirdSpawn.Genome.Matrix == firstElite.Genome.Matrix {
		t.Fatalf("third elite round-robin copy should be mutated")
	}
	if fourthSpawn.Genome.Matrix == firstElite.Genome.Matrix || fourthSpawn.Genome.Matrix == secondElite.Genome.Matrix {
		t.Fatalf("non-elite seed slot reused elite genome exactly")
	}
}

func TestSmartGenerationSeedsColonyLinkedEliteCohort(t *testing.T) {
	rand.Seed(7)

	cfg := config.NewConfig()
	cfg.BotChance = 100
	cfg.EvolutionEliteCount = 2
	cfg.EvolutionSeedPercent = 100
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	cluster := []core.Position{
		util.NewPos(10, 10),
		util.NewPos(10, 11),
		util.NewPos(10, 12),
		util.NewPos(11, 10),
		util.NewPos(11, 11),
		util.NewPos(11, 12),
	}
	freezeAllExcept(t, g.Board, cluster...)

	colonyElite := core.NewBot(util.NewPos(20, 20))
	colonyElite.Genome = testGenerationGenome(71)
	colony := core.NewColony(util.NewPos(20, 21))
	colony.AddFamily(&colonyElite)
	g.rememberGenerationChampion(&colonyElite)

	soloElite := core.NewBot(util.NewPos(22, 20))
	soloElite.Divisions = 2
	soloElite.Genome = testGenerationGenome(91)
	g.rememberGenerationChampion(&soloElite)

	g.initialBotsGeneration()

	matches := exactGenomeBotsNear(g.Board, cluster[0], colonyElite.Genome, controllerRecruitRadius)
	if matches < 2 {
		t.Fatalf("colony-linked elite nearby exact copies = %d, want at least founder support", matches)
	}
}

func TestLowPopulationImmigrantsUseEliteGenomeWhenAvailable(t *testing.T) {
	cfg := config.NewConfig()
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	elite := core.NewBot(util.NewPos(20, 20))
	elite.Divisions = 1
	elite.Genome = testGenerationGenome(43)
	g.rememberGenerationChampion(&elite)

	immigrantPos := util.NewPos(10, 10)
	if !g.spawnRandomImmigrantAt(immigrantPos) {
		t.Fatalf("failed to spawn elite immigrant")
	}
	immigrant := g.Board.GetBot(immigrantPos)
	if immigrant == nil {
		t.Fatalf("immigrant missing at %v", immigrantPos)
	}
	if immigrant.Genome.Matrix != elite.Genome.Matrix {
		t.Fatalf("first elite immigrant was not an exact elite genome")
	}
}

func TestLowPopulationColonyEliteImmigrantsSpawnCohortWithinBudget(t *testing.T) {
	cfg := config.NewConfig()
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	elite := core.NewBot(util.NewPos(20, 20))
	elite.Genome = testGenerationGenome(73)
	colony := core.NewColony(util.NewPos(20, 21))
	colony.AddFamily(&elite)
	g.rememberGenerationChampion(&elite)

	immigrantPos := util.NewPos(10, 10)
	spawned := g.spawnRandomImmigrantAtWithBudget(immigrantPos, 4)
	if spawned < 2 || spawned > 4 {
		t.Fatalf("colony elite immigrant cohort spawned %d, want 2..4", spawned)
	}
	matches := exactGenomeBotsNear(g.Board, immigrantPos, elite.Genome, controllerRecruitRadius)
	if matches < 2 {
		t.Fatalf("colony elite immigrant nearby exact copies = %d, want founder support", matches)
	}
}

func TestLowPopulationImmigrantsUseRandomGenomeWithoutElite(t *testing.T) {
	rand.Seed(9)

	cfg := config.NewConfig()
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	immigrantPos := util.NewPos(10, 10)
	if !g.spawnRandomImmigrantAt(immigrantPos) {
		t.Fatalf("failed to spawn random immigrant")
	}
	immigrant := g.Board.GetBot(immigrantPos)
	if immigrant == nil {
		t.Fatalf("immigrant missing at %v", immigrantPos)
	}
	if immigrant.Parent != nil || immigrant.LineageDepth != 0 || immigrant.Divisions != 0 {
		t.Fatalf("immigrant should be a fresh random bot, got parent=%p depth=%d divisions=%d", immigrant.Parent, immigrant.LineageDepth, immigrant.Divisions)
	}
	if immigrant.Genome.Matrix == (core.Genome{}).Matrix {
		t.Fatalf("random immigrant genome was zero-valued")
	}
}

func TestImmigrationCapsSpawnCountToPopulationGap(t *testing.T) {
	cfg := config.NewConfig()
	cfg.ImmigrationInterval = 1
	cfg.ImmigrationBots = 16
	g := NewGame(&cfg)
	g.Board = core.NewBoard()
	g.logicTick = 1

	firstSpawn := util.NewPos(10, 10)
	secondSpawn := util.NewPos(10, 11)
	thirdSpawn := util.NewPos(10, 12)
	freezeAllExcept(t, g.Board, firstSpawn, secondSpawn, thirdSpawn)

	if spawned := g.spawnRandomImmigrants(2); spawned != 2 {
		t.Fatalf("spawned immigrants = %d, want cap of 2", spawned)
	}
	if got := g.liveBotCount(); got != 2 {
		t.Fatalf("live bots after capped immigration = %d, want 2", got)
	}
}

func TestGenerationSeedingUsesChildrenByBotLimit(t *testing.T) {
	cfg := config.NewConfig()
	cfg.BotChance = 100
	cfg.ChildrenByBot = 1
	cfg.NewGenThreshold = 0
	cfg.PoisonChance = 0
	cfg.ResourceChance = 0
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	firstSpawnPos := util.NewPos(10, 10)
	secondSpawnPos := util.NewPos(10, 11)
	championPos := util.NewPos(20, 20)
	freezeAllExcept(t, g.Board, firstSpawnPos, secondSpawnPos, championPos)

	champion := core.NewBot(championPos)
	champion.Hp = 250
	champion.Inventory = core.Inventory{Food: 1, Ore: 1}
	champion.Genome = testGenerationGenome(42)
	g.rememberGenerationChampion(&champion)

	g.runLogicTick()

	firstSpawn := g.Board.GetBot(firstSpawnPos)
	if firstSpawn == nil {
		t.Fatalf("expected first generation seed at %v", firstSpawnPos)
	}
	if firstSpawn.Genome.Matrix != champion.Genome.Matrix {
		t.Fatalf("first seeded bot did not inherit champion genome")
	}

	secondSpawn := g.Board.GetBot(secondSpawnPos)
	if secondSpawn == nil {
		t.Fatalf("expected random generation bot at %v", secondSpawnPos)
	}
	if secondSpawn.Genome.Matrix == champion.Genome.Matrix {
		t.Fatalf("second spawned bot inherited champion despite ChildrenByBot=1")
	}
}

func TestExtinctionGenerationUsesLastObservedChampion(t *testing.T) {
	cfg := config.NewConfig()
	cfg.BotChance = 100
	cfg.NewGenThreshold = 0
	cfg.PoisonChance = 0
	cfg.ResourceChance = 0
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	spawnPos := util.NewPos(10, 10)
	dyingPos := util.NewPos(20, 20)
	freezeAllExcept(t, g.Board, spawnPos, dyingPos)

	dyingChampion := core.NewBot(dyingPos)
	dyingChampion.Hp = 1
	dyingChampion.Genome = testGenerationGenome(57)
	wantGenome := dyingChampion.Genome
	g.Board.Bots[util.Idx(dyingPos)] = &dyingChampion
	g.Board.Set(dyingPos, &dyingChampion)

	g.runLogicTick()
	if got := g.liveBotCount(); got != 0 {
		t.Fatalf("live bots after death tick = %d, want 0", got)
	}

	g.runLogicTick()

	spawned := g.Board.GetBot(spawnPos)
	if spawned == nil {
		t.Fatalf("expected generation bot at %v after extinction", spawnPos)
	}
	if spawned.Genome.Matrix != wantGenome.Matrix {
		t.Fatalf("spawned genome after extinction did not use last champion matrix")
	}
}

func TestGrabOwnedFarmSpendsOreToChargeProduction(t *testing.T) {
	cfg := config.NewConfig()
	cfg.FarmGrabCost = 1
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	botPos := util.NewPos(10, 10)
	bot := core.NewBot(botPos)
	bot.Dir = core.Up
	bot.Inventory = core.Inventory{Food: 2, Ore: 2}
	farmPos := botPos.AddRowCol(bot.Dir[0], bot.Dir[1])

	g.Board.Bots[util.Idx(botPos)] = &bot
	g.Board.Set(botPos, &bot)
	g.Board.Set(farmPos, core.Farm{Pos: farmPos, Owner: &bot, Amount: 0})

	g.grab(botPos, &bot)

	if bot.Inventory.Food != 2 {
		t.Fatalf("food inventory after farm charge = %d, want unchanged 2", bot.Inventory.Food)
	}
	if bot.Inventory.Ore != 1 {
		t.Fatalf("ore inventory after farm charge = %d, want 1", bot.Inventory.Ore)
	}
	farm, ok := g.Board.At(farmPos).(core.Farm)
	if !ok {
		t.Fatalf("farm missing after charge")
	}
	if farm.Amount != 1 {
		t.Fatalf("farm amount after charge = %d, want 1", farm.Amount)
	}
}

func TestGrabOwnedFarmSpendsBankOreToChargeProduction(t *testing.T) {
	cfg := config.NewConfig()
	cfg.FarmGrabCost = 1
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	botPos := util.NewPos(10, 10)
	bot := core.NewBot(botPos)
	bot.Dir = core.Up
	farmPos := botPos.AddRowCol(bot.Dir[0], bot.Dir[1])
	colony := core.NewColony(util.NewPos(10, 9))
	colony.OreBank = 1
	colony.AddFamily(&bot)
	bot.ConnnectedToColony = true

	g.Board.Bots[util.Idx(botPos)] = &bot
	g.Board.Set(botPos, &bot)
	g.Board.Set(colony.Center, core.Controller{Pos: colony.Center, Owner: &bot, Colony: &colony, Amount: 10})
	g.Board.Set(farmPos, core.Farm{Pos: farmPos, Owner: &bot, Amount: 0})

	g.grab(botPos, &bot)

	if bot.Inventory.Ore != 0 {
		t.Fatalf("personal ore after bank farm charge = %d, want 0", bot.Inventory.Ore)
	}
	if colony.OreBank != 0 {
		t.Fatalf("ore bank after farm charge = %d, want 0", colony.OreBank)
	}
	farm, ok := g.Board.At(farmPos).(core.Farm)
	if !ok {
		t.Fatalf("farm missing after charge")
	}
	wantCharge := 1 + cfg.ColonyFarmChargeBonus
	if farm.Amount != wantCharge {
		t.Fatalf("farm amount after bank charge = %d, want %d", farm.Amount, wantCharge)
	}
}

func TestDescendantCanMaintainParentFarm(t *testing.T) {
	cfg := config.NewConfig()
	cfg.FarmGrabCost = 1
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	parent := core.NewBot(util.NewPos(10, 12))
	botPos := util.NewPos(10, 10)
	child := parent.NewChild(botPos, false)
	child.Dir = core.Up
	child.Inventory.Ore = 1
	farmPos := botPos.AddRowCol(child.Dir[0], child.Dir[1])

	g.Board.Bots[util.Idx(botPos)] = child
	g.Board.Set(botPos, child)
	g.Board.Set(farmPos, core.Farm{Pos: farmPos, Owner: &parent, Amount: 0})

	g.grab(botPos, child)

	farm, ok := g.Board.At(farmPos).(core.Farm)
	if !ok {
		t.Fatalf("descendant destroyed parent farm")
	}
	if farm.Amount != 1 {
		t.Fatalf("farm amount after descendant charge = %d, want 1", farm.Amount)
	}
	if farm.Owner != child {
		t.Fatalf("farm owner after stale-owner claim = %p, want child %p", farm.Owner, child)
	}
	if child.Inventory.Ore != 0 {
		t.Fatalf("child ore after farm charge = %d, want 0", child.Inventory.Ore)
	}
}

func TestColonyMemberCanMaintainColonyFarm(t *testing.T) {
	cfg := config.NewConfig()
	cfg.FarmGrabCost = 1
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	owner := core.NewBot(util.NewPos(10, 12))
	botPos := util.NewPos(10, 10)
	member := core.NewBot(botPos)
	member.Dir = core.Up
	farmPos := botPos.AddRowCol(member.Dir[0], member.Dir[1])
	colony := core.NewColony(util.NewPos(10, 9))
	colony.OreBank = 1
	colony.AddFamily(&owner)
	colony.AddFamily(&member)
	member.ConnnectedToColony = true

	g.Board.Bots[util.Idx(botPos)] = &member
	g.Board.Set(botPos, &member)
	g.Board.Set(colony.Center, core.Controller{Pos: colony.Center, Owner: &owner, Colony: &colony, Amount: 10})
	g.Board.Set(farmPos, core.Farm{Pos: farmPos, Owner: &owner, Colony: &colony, Amount: 0})

	g.grab(botPos, &member)

	if colony.OreBank != 0 {
		t.Fatalf("ore bank after colony farm charge = %d, want 0", colony.OreBank)
	}
	farm, ok := g.Board.At(farmPos).(core.Farm)
	if !ok {
		t.Fatalf("colony member destroyed colony farm")
	}
	wantCharge := 1 + cfg.ColonyFarmChargeBonus
	if farm.Amount != wantCharge {
		t.Fatalf("farm amount after colony charge = %d, want %d", farm.Amount, wantCharge)
	}
	if farm.Colony != &colony {
		t.Fatalf("farm colony = %p, want %p", farm.Colony, &colony)
	}
}

func TestGrabForeignFarmDoesNotDestroyIt(t *testing.T) {
	cfg := config.NewConfig()
	cfg.FarmGrabCost = 1
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	botPos := util.NewPos(10, 10)
	raider := core.NewBot(botPos)
	raider.Dir = core.Up
	raider.Inventory = core.Inventory{Food: 2, Ore: 3}
	farmPos := botPos.AddRowCol(raider.Dir[0], raider.Dir[1])
	owner := core.NewBot(util.NewPos(10, 12))
	ownerColony := core.NewColony(farmPos)
	ownerColony.AddFamily(&owner)
	raiderColony := core.NewColony(botPos)
	raiderColony.AddFamily(&raider)

	g.Board.Bots[util.Idx(botPos)] = &raider
	g.Board.Set(botPos, &raider)
	g.Board.Set(farmPos, core.Farm{Pos: farmPos, Owner: &owner, Colony: &ownerColony, Amount: 2})

	g.grab(botPos, &raider)

	if raider.Inventory.Food != 2 || raider.Inventory.Ore != 3 {
		t.Fatalf("raider inventory after foreign farm grab = %+v, want unchanged F2 O3", raider.Inventory)
	}
	farm, ok := g.Board.At(farmPos).(core.Farm)
	if !ok {
		t.Fatalf("foreign farm was destroyed")
	}
	if farm.Amount != 2 || farm.Owner != &owner || farm.Colony != &ownerColony {
		t.Fatalf("foreign farm mutated after grab: %+v", farm)
	}
}

func TestStaleControllerOwnerClearsController(t *testing.T) {
	cfg := config.NewConfig()
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	ctrlPos := util.NewPos(20, 20)
	owner := core.NewBot(util.NewPos(20, 21))
	owner.ConnnectedToColony = true
	colony := core.NewColony(ctrlPos)
	colony.AddFamily(&owner)
	ctrl := core.Controller{
		Pos:    ctrlPos,
		Owner:  &owner,
		Colony: &colony,
		Amount: 10,
	}
	g.Board.Set(ctrlPos, ctrl)

	g.handleController(&ctrl, ctrlPos)

	if got := g.Board.At(ctrlPos); got != nil {
		t.Fatalf("stale controller cell = %T, want nil", got)
	}
	if owner.ConnnectedToColony {
		t.Fatalf("stale controller member should be disconnected")
	}
}

func TestControllerTransfersToLiveColonyMember(t *testing.T) {
	cfg := config.NewConfig()
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	ctrlPos := util.NewPos(20, 20)
	owner := core.NewBot(util.NewPos(20, 21))
	successor := core.NewBot(ctrlPos.AddDir(core.Right))
	colony := core.NewColony(ctrlPos)
	colony.AddFamily(&owner)
	colony.AddFamily(&successor)
	ctrl := core.Controller{
		Pos:    ctrlPos,
		Owner:  &owner,
		Colony: &colony,
		Amount: 10,
	}
	g.Board.Set(ctrlPos, ctrl)
	g.Board.Bots[util.Idx(successor.Pos)] = &successor
	g.Board.Set(successor.Pos, &successor)

	g.handleController(&ctrl, ctrlPos)

	if got := g.Board.At(ctrlPos); got == nil {
		t.Fatalf("controller was cleared despite live colony successor")
	}
	if ctrl.Owner != &successor {
		t.Fatalf("controller owner = %p, want live successor %p", ctrl.Owner, &successor)
	}
	if !successor.ConnnectedToColony {
		t.Fatalf("successor was not connected to colony")
	}
}

func TestControllerRecruitsNearbyKinOnly(t *testing.T) {
	cfg := config.NewConfig()
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	ctrlPos := util.NewPos(20, 20)
	owner := core.NewBot(ctrlPos.AddDir(core.Right))
	kin := core.NewBot(util.NewPos(20, 23))
	foreign := core.NewBot(util.NewPos(20, 24))
	for i := range owner.Genome.Matrix {
		owner.Genome.Matrix[i] = 0
		kin.Genome.Matrix[i] = 0
		foreign.Genome.Matrix[i] = core.OpcodeCount() + i
	}
	colony := core.NewColony(ctrlPos)
	colony.AddFamily(&owner)
	ctrl := core.Controller{
		Pos:    ctrlPos,
		Owner:  &owner,
		Colony: &colony,
		Amount: 10,
	}
	for _, bot := range []*core.Bot{&owner, &kin, &foreign} {
		g.Board.Bots[util.Idx(bot.Pos)] = bot
		g.Board.Set(bot.Pos, bot)
	}
	g.Board.Set(ctrlPos, ctrl)

	g.handleController(&ctrl, ctrlPos)

	if kin.Colony != &colony {
		t.Fatalf("nearby kin colony = %p, want %p", kin.Colony, &colony)
	}
	if !kin.ConnnectedToColony {
		t.Fatalf("nearby recruited kin was not connected to colony")
	}
	if foreign.Colony != nil {
		t.Fatalf("foreign bot was recruited into colony %p", foreign.Colony)
	}
}

func TestEnvironmentActionsDoesNotReinsertStaleController(t *testing.T) {
	cfg := config.NewConfig()
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	ctrlPos := util.NewPos(20, 20)
	owner := core.NewBot(util.NewPos(20, 21))
	colony := core.NewColony(ctrlPos)
	colony.AddFamily(&owner)
	g.Board.Set(ctrlPos, core.Controller{
		Pos:    ctrlPos,
		Owner:  &owner,
		Colony: &colony,
		Amount: 10,
	})

	g.environmentActions()

	if got := g.Board.At(ctrlPos); got != nil {
		t.Fatalf("stale controller after environment action = %T, want nil", got)
	}
}

func TestHandleControllerDepositsConnectedMemberSurplus(t *testing.T) {
	cfg := config.NewConfig()
	cfg.DivisionFoodCost = 1
	cfg.DivisionOreCost = 1
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	ctrlPos := util.NewPos(20, 20)
	botPos := ctrlPos.AddDir(core.Right)
	bot := core.NewBot(botPos)
	bot.Inventory = core.Inventory{Food: 4, Ore: 3}
	colony := core.NewColony(ctrlPos)
	colony.AddFamily(&bot)
	ctrl := core.Controller{
		Pos:    ctrlPos,
		Owner:  &bot,
		Colony: &colony,
		Amount: 10,
	}

	g.Board.Bots[util.Idx(botPos)] = &bot
	g.Board.Set(botPos, &bot)
	g.Board.Set(ctrlPos, ctrl)

	g.handleController(&ctrl, ctrlPos)

	if !bot.ConnnectedToColony {
		t.Fatalf("adjacent colony bot was not connected")
	}
	if bot.Inventory.Food != 1 || bot.Inventory.Ore != 1 {
		t.Fatalf("bot inventory after controller deposit = %+v, want reserve food 1 ore 1", bot.Inventory)
	}
	if colony.FoodBank != 3 || colony.OreBank != 2 {
		t.Fatalf("colony bank after controller deposit = F%d O%d, want F3 O2", colony.FoodBank, colony.OreBank)
	}
}

func TestColonyBuildingTaskOverridesOpcodeAndBuildsAssignedTarget(t *testing.T) {
	cfg := config.NewConfig()
	cfg.FarmBuildCost = 0
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	botPos := util.NewPos(40, 40)
	buildPos := botPos.AddDir(core.Right)
	bot := core.NewBot(botPos)
	bot.Genome.Matrix[0] = int(core.OpMove)
	bot.ConnnectedToColony = true
	colony := core.NewColony(botPos.AddRowCol(0, -3))
	colony.AddFamily(&bot)
	addTestBot(g, &bot)

	task := colony.NewBuildingTask(buildPos, core.BuildFarm)
	colony.AddTask(task)
	bot.AssignTask(task)

	g.botAction(botPos, &bot)

	if _, ok := g.Board.At(buildPos).(core.Farm); !ok {
		t.Fatalf("assigned build target = %T, want Farm", g.Board.At(buildPos))
	}
	if !task.IsDone {
		t.Fatalf("building task was not marked done")
	}
	if bot.CurrTask != nil {
		t.Fatalf("bot still has task %+v", bot.CurrTask)
	}
}

func TestColonyFoodGatheringTaskOverridesOpcodeAndGrabsAssignedFood(t *testing.T) {
	cfg := config.NewConfig()
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	botPos := util.NewPos(44, 44)
	foodPos := botPos.AddDir(core.Right)
	bot := core.NewBot(botPos)
	bot.Dir = core.Left
	bot.Genome.Matrix[0] = int(core.OpTurn)
	bot.ConnnectedToColony = true
	colony := core.NewColony(botPos.AddRowCol(0, -3))
	colony.AddFamily(&bot)
	addTestBot(g, &bot)
	g.Board.Set(foodPos, core.Food{Pos: foodPos, Amount: 2})

	task := colony.NewFoodGatheringTask(foodPos)
	colony.AddTask(task)
	bot.AssignTask(task)

	g.botAction(botPos, &bot)

	if bot.Inventory.Food != 2 {
		t.Fatalf("food inventory = %d, want 2", bot.Inventory.Food)
	}
	if g.Board.At(foodPos) != nil {
		t.Fatalf("food target still occupied by %T", g.Board.At(foodPos))
	}
	if !task.IsDone || bot.CurrTask != nil {
		t.Fatalf("food gathering task done/current = %v/%v, want done/nil", task.IsDone, bot.CurrTask)
	}
}

func TestColonyScoutTaskOverridesOpcodeAndMovesTowardTarget(t *testing.T) {
	cfg := config.NewConfig()
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	botPos := util.NewPos(48, 48)
	targetPos := botPos.AddRowCol(0, 6)
	bot := core.NewBot(botPos)
	bot.Genome.Matrix[0] = int(core.OpGrab)
	bot.ConnnectedToColony = true
	colony := core.NewColony(botPos.AddRowCol(0, -3))
	colony.AddFamily(&bot)
	addTestBot(g, &bot)

	task := colony.NewScoutTask(targetPos)
	colony.AddTask(task)
	bot.AssignTask(task)
	before := boardDistance(bot.Pos, targetPos)

	g.botAction(botPos, &bot)

	after := boardDistance(bot.Pos, targetPos)
	if after >= before {
		t.Fatalf("scout distance = %d, want less than %d; pos=%v target=%v", after, before, bot.Pos, targetPos)
	}
	if bot.CurrTask != task {
		t.Fatalf("scout task current = %p, want %p", bot.CurrTask, task)
	}
	if got := g.Board.PheromoneAt(bot.Pos).Home; got == 0 {
		t.Fatalf("scout did not leave home pheromone at %v", bot.Pos)
	}
}

func TestProcessColonyRoleTasksCreatesDiverseTaskTypes(t *testing.T) {
	cfg := config.NewConfig()
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	center := util.NewPos(100, 100)
	colony := core.NewColony(center)
	for i, offset := range []core.Position{
		{R: 0, C: -2}, {R: 0, C: -1}, {R: 0, C: 1}, {R: 0, C: 2},
		{R: 1, C: -2}, {R: 1, C: -1}, {R: 1, C: 1}, {R: 1, C: 2},
		{R: -1, C: -2}, {R: -1, C: -1}, {R: 18, C: 0}, {R: 18, C: 2},
	} {
		pos := center.AddRowCol(offset.R, offset.C)
		bot := core.NewBot(pos)
		bot.ConnnectedToColony = true
		bot.Hp = 300 + i
		colony.AddFamily(&bot)
		addTestBot(g, &bot)
	}
	g.Board.Set(center.AddRowCol(2, 0), core.Food{Pos: center.AddRowCol(2, 0), Amount: 1})
	g.Board.Set(center.AddRowCol(3, 0), core.Farm{Pos: center.AddRowCol(3, 0), Colony: &colony, Amount: 1})

	g.processColonyRoleTasks(&colony, center)

	seen := map[core.ColonyTaskType]bool{}
	for _, task := range colony.Tasks {
		if task != nil && isColonyRoleTask(task) {
			seen[task.Type] = true
		}
	}
	for _, taskType := range []core.ColonyTaskType{
		core.BuildingTask,
		core.FoodGatheringTask,
		core.ScoutTask,
		core.FarmingTask,
	} {
		if !seen[taskType] {
			t.Fatalf("missing role task %s; tasks=%+v", taskType, colony.Tasks)
		}
	}
}

func TestConnectedColonyMemberCanBuildDepot(t *testing.T) {
	cfg := config.NewConfig()
	cfg.DepotBuildCost = 2
	cfg.PheromoneHomeDeposit = 24
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	buildPos := util.NewPos(20, 20)
	botPos := buildPos.AddRowCol(-1, 0)
	bot := core.NewBot(botPos)
	bot.Inventory.Ore = cfg.DepotBuildCost
	bot.Genome.Matrix[1] = 2
	bot.Genome.Matrix[2] = int(core.BuildDepot)
	colony := core.NewColony(buildPos.AddRowCol(0, -2))
	colony.AddFamily(&bot)
	bot.ConnnectedToColony = true
	addTestBot(g, &bot)

	g.build(botPos, &bot)

	depot, ok := g.Board.At(buildPos).(core.Depot)
	if !ok {
		t.Fatalf("built cell = %T, want Depot", g.Board.At(buildPos))
	}
	if depot.Owner != &bot || depot.Colony != &colony {
		t.Fatalf("depot owner/colony = %p/%p, want bot/colony", depot.Owner, depot.Colony)
	}
	if bot.Inventory.Ore != 0 {
		t.Fatalf("ore after depot build = %d, want 0", bot.Inventory.Ore)
	}
	if bot.Evolution.DepotBuilds != 1 {
		t.Fatalf("depot builds = %d, want 1", bot.Evolution.DepotBuilds)
	}
	if got := g.Board.PheromoneAt(buildPos).Home; got != 8 {
		t.Fatalf("depot home pheromone = %d, want 8", got)
	}
}

func TestBuildDepotRequiresConnectedColonyMember(t *testing.T) {
	cfg := config.NewConfig()
	cfg.DepotBuildCost = 2
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	buildPos := util.NewPos(20, 20)
	botPos := buildPos.AddRowCol(-1, 0)
	colonyless := core.NewBot(botPos)
	colonyless.Inventory.Ore = cfg.DepotBuildCost
	colonyless.Genome.Matrix[1] = 2
	colonyless.Genome.Matrix[2] = int(core.BuildDepot)
	addTestBot(g, &colonyless)

	g.build(botPos, &colonyless)

	if got := g.Board.At(buildPos); got != nil {
		t.Fatalf("colonyless depot build cell = %T, want nil", got)
	}
	if colonyless.Inventory.Ore != cfg.DepotBuildCost {
		t.Fatalf("ore after colonyless depot build = %d, want unchanged", colonyless.Inventory.Ore)
	}

	g.Board.Clear(botPos)
	disconnected := core.NewBot(botPos)
	disconnected.Inventory.Ore = cfg.DepotBuildCost
	disconnected.Genome.Matrix[1] = 2
	disconnected.Genome.Matrix[2] = int(core.BuildDepot)
	colony := core.NewColony(buildPos.AddRowCol(0, -2))
	colony.AddFamily(&disconnected)
	addTestBot(g, &disconnected)

	g.build(botPos, &disconnected)

	if got := g.Board.At(buildPos); got != nil {
		t.Fatalf("disconnected depot build cell = %T, want nil", got)
	}
	if disconnected.Inventory.Ore != cfg.DepotBuildCost {
		t.Fatalf("ore after disconnected depot build = %d, want unchanged", disconnected.Inventory.Ore)
	}
}

func TestBuildSpawnerCreatesChargedSpawnerForColonylessBot(t *testing.T) {
	cfg := config.NewConfig()
	cfg.SpawnerBuildCost = 2
	cfg.SpawnerInitialAmount = 3
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	buildPos := util.NewPos(20, 20)
	botPos := buildPos.AddRowCol(-1, 0)
	bot := core.NewBot(botPos)
	bot.Inventory.Ore = cfg.SpawnerBuildCost
	bot.Genome.Matrix[1] = 2
	bot.Genome.Matrix[2] = int(core.BuildSpawner)
	addTestBot(g, &bot)

	g.build(botPos, &bot)

	spawner, ok := g.Board.At(buildPos).(core.Spawner)
	if !ok {
		t.Fatalf("built cell = %T, want Spawner", g.Board.At(buildPos))
	}
	if spawner.Owner != &bot || spawner.Amount != cfg.SpawnerInitialAmount {
		t.Fatalf("spawner = %+v, want owner bot amount %d", spawner, cfg.SpawnerInitialAmount)
	}
	if bot.Inventory.Ore != 0 {
		t.Fatalf("ore after spawner build = %d, want 0", bot.Inventory.Ore)
	}
	if bot.Evolution.SpawnerBuilds != 1 {
		t.Fatalf("spawner builds = %d, want 1", bot.Evolution.SpawnerBuilds)
	}
}

func TestSpawnerAssistedDivisionUsesLowerHpAndCharge(t *testing.T) {
	cfg := config.NewConfig()
	cfg.DivisionCost = 7
	cfg.DivisionFoodCost = 1
	cfg.DivisionOreCost = 1
	cfg.DivisionMinHp = 120
	cfg.SpawnerDivisionMinHp = 80
	cfg.SpawnerAccessRadius = 2
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	botPos := util.NewPos(20, 20)
	spawnerPos := botPos.AddRowCol(0, 2)
	bot := core.NewBot(botPos)
	bot.Hp = cfg.SpawnerDivisionMinHp
	bot.Inventory = core.Inventory{Food: 1, Ore: 1}
	bot.Genome.Matrix[0] = int(core.OpDivide)
	addTestBot(g, &bot)
	g.Board.Set(spawnerPos, core.Spawner{Pos: spawnerPos, Owner: &bot, Amount: 2})

	if !g.DivisionReady(&bot) {
		t.Fatalf("bot should be division-ready through nearby spawner")
	}

	g.botAction(botPos, &bot)

	if got := g.liveBotCount(); got != 2 {
		t.Fatalf("live bots after spawner-assisted divide = %d, want 2", got)
	}
	if got := g.SuccessfulDivisions(); got != 1 {
		t.Fatalf("successful divisions = %d, want 1", got)
	}
	if got := g.SpawnerBirths(); got != 1 {
		t.Fatalf("spawner births = %d, want 1", got)
	}
	if bot.Divisions != 1 || bot.Evolution.SuccessfulDivisions != 1 || bot.Evolution.SpawnerBirths != 1 {
		t.Fatalf("parent division stats = divisions %d evo %+v, want successful spawner birth", bot.Divisions, bot.Evolution)
	}
	if bot.Hp != cfg.SpawnerDivisionMinHp-cfg.DivisionCost {
		t.Fatalf("parent hp after spawner divide = %d, want %d", bot.Hp, cfg.SpawnerDivisionMinHp-cfg.DivisionCost)
	}
	if bot.Inventory.Total() != 0 {
		t.Fatalf("parent inventory after spawner divide = %+v, want empty", bot.Inventory)
	}
	spawner, ok := g.Board.At(spawnerPos).(core.Spawner)
	if !ok || spawner.Amount != 1 {
		t.Fatalf("spawner after assisted divide = %+v ok=%v, want amount 1", spawner, ok)
	}

	var child *core.Bot
	for _, candidate := range g.Board.Bots {
		if candidate != nil && candidate != &bot {
			child = candidate
			break
		}
	}
	if child == nil {
		t.Fatalf("child bot not found")
	}
	if !child.Pos.InRadius(spawnerPos, 1) {
		t.Fatalf("child position = %v, want adjacent to spawner %v", child.Pos, spawnerPos)
	}
}

func TestColonySpawnerAssistedDivisionUsesCachedBestGenome(t *testing.T) {
	cfg := config.NewConfig()
	cfg.DivisionCost = 1
	cfg.DivisionFoodCost = 1
	cfg.DivisionOreCost = 1
	cfg.DivisionMinHp = 120
	cfg.SpawnerDivisionMinHp = 80
	cfg.SpawnerAccessRadius = 3
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	ctrlPos := util.NewPos(20, 20)
	activatorPos := util.NewPos(20, 21)
	championPos := util.NewPos(22, 20)
	spawnerPos := util.NewPos(20, 23)
	colony := core.NewColony(ctrlPos)
	activator := core.NewBot(activatorPos)
	activator.Hp = cfg.SpawnerDivisionMinHp
	activator.Inventory = core.Inventory{Food: 1, Ore: 1}
	activator.Genome = testGenerationGenome(3)
	activator.Genome.Pointer = 0
	activator.Genome.Matrix[0] = int(core.OpDivide)
	champion := core.NewBot(championPos)
	champion.Genome = testGenerationGenome(42)
	champion.Genome.Matrix[0] = int(core.OpPhoto)
	champion.Evolution.FoodGathered = 500
	colony.AddFamily(&activator)
	colony.AddFamily(&champion)
	activator.ConnnectedToColony = true
	champion.ConnnectedToColony = true
	addTestBot(g, &activator)
	addTestBot(g, &champion)
	ctrl := core.Controller{Pos: ctrlPos, Owner: &activator, Colony: &colony, Amount: 100}
	g.Board.Set(ctrlPos, ctrl)
	g.Board.Set(spawnerPos, core.Spawner{Pos: spawnerPos, Owner: &activator, Colony: &colony, Amount: 1})

	g.handleController(&ctrl, ctrlPos)

	if !colony.HasSpawnerGenome {
		t.Fatalf("colony did not cache a spawner genome")
	}
	if colony.SpawnerGenome.Matrix != champion.Genome.Matrix {
		t.Fatalf("cached spawner genome did not use connected champion")
	}

	g.botAction(activatorPos, &activator)

	var child *core.Bot
	for _, candidate := range g.Board.Bots {
		if candidate != nil && candidate != &activator && candidate != &champion {
			child = candidate
			break
		}
	}
	if child == nil {
		t.Fatalf("child bot not found")
	}
	if child.Genome.Matrix != champion.Genome.Matrix {
		t.Fatalf("child genome matrix = %v, want cached champion genome", child.Genome.Matrix)
	}
	if child.Genome.Matrix == activator.Genome.Matrix {
		t.Fatalf("child used activator genome instead of colony cached genome")
	}
}

func TestColonyOwnedSpawnerAutoBirthsCachedGenomeWithoutCharge(t *testing.T) {
	cfg := config.NewConfig()
	cfg.ColonySpawnerBirthPeriod = 1
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	ctrlPos := util.NewPos(20, 20)
	parentPos := util.NewPos(20, 21)
	spawnerPos := util.NewPos(20, 23)
	colony := core.NewColony(ctrlPos)
	parent := core.NewBot(parentPos)
	parent.Genome = testGenerationGenome(55)
	parent.ConnnectedToColony = true
	colony.AddFamily(&parent)
	colony.SpawnerGenome = normalizedEvolutionGenome(parent.Genome)
	colony.HasSpawnerGenome = true
	addTestBot(g, &parent)
	g.Board.Set(ctrlPos, core.Controller{Pos: ctrlPos, Owner: &parent, Colony: &colony, Amount: 100})
	g.Board.Set(spawnerPos, core.Spawner{Pos: spawnerPos, Owner: &parent, Colony: &colony, Amount: 0, AutoBirth: true})

	g.environmentActions()

	if got := g.liveBotCount(); got != 2 {
		t.Fatalf("live bots after auto spawner birth = %d, want 2", got)
	}
	if got := g.SpawnerBirths(); got != 1 {
		t.Fatalf("spawner births after auto birth = %d, want 1", got)
	}
	if parent.Evolution.SpawnerBirths != 1 {
		t.Fatalf("parent spawner birth credit = %d, want 1", parent.Evolution.SpawnerBirths)
	}
	spawner, ok := g.Board.At(spawnerPos).(core.Spawner)
	if !ok {
		t.Fatalf("spawner missing after auto birth")
	}
	if spawner.Amount != 0 {
		t.Fatalf("auto birth consumed charge: amount = %d, want 0", spawner.Amount)
	}

	var child *core.Bot
	for _, candidate := range g.Board.Bots {
		if candidate != nil && candidate != &parent {
			child = candidate
			break
		}
	}
	if child == nil {
		t.Fatalf("auto-born child not found")
	}
	if child.Colony != &colony || !child.ConnnectedToColony {
		t.Fatalf("auto-born child colony/connection = %p/%v, want connected colony", child.Colony, child.ConnnectedToColony)
	}
	if child.Genome.Matrix != parent.Genome.Matrix {
		t.Fatalf("auto-born child genome matrix = %v, want cached parent genome", child.Genome.Matrix)
	}
}

func TestColonyOwnedSpawnerAutoBirthRespectsLocalLimit(t *testing.T) {
	cfg := config.NewConfig()
	cfg.ColonySpawnerBirthPeriod = 1
	cfg.ColonySpawnerLocalLimit = 1
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	ctrlPos := util.NewPos(20, 20)
	parentPos := util.NewPos(20, 21)
	spawnerPos := util.NewPos(20, 23)
	colony := core.NewColony(ctrlPos)
	parent := core.NewBot(parentPos)
	parent.ConnnectedToColony = true
	colony.AddFamily(&parent)
	colony.SpawnerGenome = normalizedEvolutionGenome(parent.Genome)
	colony.HasSpawnerGenome = true
	addTestBot(g, &parent)
	g.Board.Set(ctrlPos, core.Controller{Pos: ctrlPos, Owner: &parent, Colony: &colony, Amount: 100})
	g.Board.Set(spawnerPos, core.Spawner{Pos: spawnerPos, Owner: &parent, Colony: &colony, Amount: 0, AutoBirth: true})

	g.environmentActions()

	if got := g.liveBotCount(); got != 1 {
		t.Fatalf("live bots after capped auto spawner tick = %d, want unchanged 1", got)
	}
	if got := g.SpawnerBirths(); got != 0 {
		t.Fatalf("spawner births after capped auto birth = %d, want 0", got)
	}
}

func TestColonyOwnedSpawnerWithoutAutoBirthDoesNotTick(t *testing.T) {
	cfg := config.NewConfig()
	cfg.ColonySpawnerBirthPeriod = 1
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	spawnerPos := util.NewPos(20, 23)
	colony := core.NewColony(util.NewPos(20, 20))
	parent := core.NewBot(util.NewPos(20, 21))
	parent.ConnnectedToColony = true
	colony.AddFamily(&parent)
	colony.SpawnerGenome = normalizedEvolutionGenome(parent.Genome)
	colony.HasSpawnerGenome = true
	g.Board.Set(spawnerPos, core.Spawner{Pos: spawnerPos, Owner: &parent, Colony: &colony, Amount: 0})

	if got := len(g.Board.ActiveEnvironmentCells()); got != 0 {
		t.Fatalf("active environment cells after passive colony spawner = %d, want 0", got)
	}
	g.environmentActions()
	if got := g.SpawnerBirths(); got != 0 {
		t.Fatalf("passive colony spawner births = %d, want 0", got)
	}
}

func TestSpawnerAssistedDivisionRejectsForeignActiveOwner(t *testing.T) {
	cfg := config.NewConfig()
	cfg.DivisionFoodCost = 1
	cfg.DivisionOreCost = 1
	cfg.DivisionMinHp = 120
	cfg.SpawnerDivisionMinHp = 80
	cfg.SpawnerAccessRadius = 2
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	botPos := util.NewPos(20, 20)
	spawnerPos := botPos.AddRowCol(0, 2)
	bot := core.NewBot(botPos)
	bot.Hp = cfg.SpawnerDivisionMinHp
	bot.Inventory = core.Inventory{Food: 1, Ore: 1}
	bot.Genome.Matrix[0] = int(core.OpDivide)
	owner := core.NewBot(util.NewPos(22, 22))
	owner.Genome = makeForeignGenome(bot.Genome)
	addTestBot(g, &bot)
	addTestBot(g, &owner)
	g.Board.Set(spawnerPos, core.Spawner{Pos: spawnerPos, Owner: &owner, Amount: 1})

	g.botAction(botPos, &bot)

	if got := g.liveBotCount(); got != 2 {
		t.Fatalf("live bots after foreign-spawner divide attempt = %d, want 2", got)
	}
	if got := g.SpawnerBirths(); got != 0 {
		t.Fatalf("spawner births = %d, want 0", got)
	}
	if bot.Hp != cfg.SpawnerDivisionMinHp || bot.Inventory.Food != 1 || bot.Inventory.Ore != 1 {
		t.Fatalf("bot mutated after rejected foreign spawner: hp %d inv %+v", bot.Hp, bot.Inventory)
	}
	spawner, ok := g.Board.At(spawnerPos).(core.Spawner)
	if !ok || spawner.Amount != 1 || spawner.Owner != &owner {
		t.Fatalf("foreign spawner after rejected divide = %+v ok=%v, want unchanged", spawner, ok)
	}
}

func TestGrabRechargesFriendlySpawnersAndRaidsForeignSpawners(t *testing.T) {
	cfg := config.NewConfig()
	cfg.SpawnerGrabCost = 2
	cfg.SpawnerMaxAmount = 3
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	botPos := util.NewPos(20, 20)
	spawnerPos := botPos.AddRowCol(1, 0)
	bot := core.NewBot(botPos)
	bot.Dir = core.Right
	bot.Inventory.Food = cfg.SpawnerGrabCost
	addTestBot(g, &bot)
	g.Board.Set(spawnerPos, core.Spawner{Pos: spawnerPos, Amount: 1})

	g.grab(botPos, &bot)

	spawner, ok := g.Board.At(spawnerPos).(core.Spawner)
	if !ok || spawner.Amount != 2 || spawner.Owner != &bot {
		t.Fatalf("friendly spawner after recharge = %+v ok=%v, want amount 2 claimed by bot", spawner, ok)
	}
	if bot.Inventory.Food != 0 {
		t.Fatalf("food after spawner recharge = %d, want 0", bot.Inventory.Food)
	}

	attackerPos := util.NewPos(24, 20)
	raidPos := attackerPos.AddRowCol(1, 0)
	attacker := core.NewBot(attackerPos)
	attacker.Dir = core.Right
	owner := core.NewBot(util.NewPos(24, 22))
	owner.Genome = makeForeignGenome(attacker.Genome)
	addTestBot(g, &attacker)
	addTestBot(g, &owner)
	g.Board.Set(raidPos, core.Spawner{Pos: raidPos, Owner: &owner, Amount: 3})

	g.grab(attackerPos, &attacker)

	raided, ok := g.Board.At(raidPos).(core.Spawner)
	if !ok || raided.Amount != 2 || raided.Owner != &owner {
		t.Fatalf("foreign spawner after raid = %+v ok=%v, want amount 2 same owner", raided, ok)
	}
	if attacker.Inventory.Food != 1 || attacker.Evolution.StolenFood != 1 || g.StolenFood() != 1 {
		t.Fatalf("attacker after spawner raid = inv %+v evo %+v game stolen %d, want stolen food 1", attacker.Inventory, attacker.Evolution, g.StolenFood())
	}
	if g.Board.PheromoneAt(raidPos).Danger == 0 || g.Board.PheromoneAt(attackerPos).Danger == 0 {
		t.Fatalf("spawner raid should emit danger pheromones")
	}
}

func TestConnectedControllerBuildInsideExclusionFallsBackToDepot(t *testing.T) {
	cfg := config.NewConfig()
	cfg.ControllerBuildCost = 5
	cfg.DepotBuildCost = 2
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	existingPos := util.NewPos(20, 20)
	buildPos := existingPos.AddRowCol(2, 0)
	botPos := buildPos.AddRowCol(-1, 0)
	owner := core.NewBot(existingPos.AddDir(core.Right))
	member := core.NewBot(botPos)
	member.Inventory.Ore = cfg.DepotBuildCost
	member.ConnnectedToColony = true
	member.Genome.Matrix[1] = 2
	member.Genome.Matrix[2] = int(core.BuildController)
	colony := core.NewColony(existingPos)
	colony.AddFamily(&owner)
	colony.AddFamily(&member)

	addTestBot(g, &owner)
	addTestBot(g, &member)
	g.Board.Set(existingPos, core.Controller{Pos: existingPos, Owner: &owner, Colony: &colony, Amount: 10})

	g.build(botPos, &member)

	depot, ok := g.Board.At(buildPos).(core.Depot)
	if !ok {
		t.Fatalf("fallback build cell = %T, want Depot", g.Board.At(buildPos))
	}
	if depot.Colony != &colony || depot.Owner != &member {
		t.Fatalf("fallback depot = %+v, want member-owned same-colony depot", depot)
	}
	if member.Inventory.Ore != 0 || member.Evolution.DepotBuilds != 1 {
		t.Fatalf("member after fallback depot build = inv %+v evo %+v, want spent ore and depot credit", member.Inventory, member.Evolution)
	}
}

func TestDepotDepositsNearbyConnectedMemberSurplus(t *testing.T) {
	cfg := config.NewConfig()
	cfg.DivisionFoodCost = 1
	cfg.DivisionOreCost = 2
	cfg.DepotFoodCapacity = 5
	cfg.DepotOreCapacity = 3
	cfg.DepotAccessRadius = 2
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	depotPos := util.NewPos(20, 20)
	colony := core.NewColony(depotPos)
	member := core.NewBot(depotPos.AddDir(core.Right))
	member.Inventory = core.Inventory{Food: 5, Ore: 5}
	member.ConnnectedToColony = true
	colony.AddFamily(&member)
	disconnected := core.NewBot(depotPos.AddDir(core.Left))
	disconnected.Inventory = core.Inventory{Food: 7, Ore: 7}
	colony.AddFamily(&disconnected)
	foreignColony := core.NewColony(depotPos.AddRowCol(0, 2))
	foreign := core.NewBot(depotPos.AddRowCol(0, 2))
	foreign.Inventory = core.Inventory{Food: 7, Ore: 7}
	foreign.ConnnectedToColony = true
	foreignColony.AddFamily(&foreign)
	distant := core.NewBot(depotPos.AddRowCol(0, 6))
	distant.Inventory = core.Inventory{Food: 7, Ore: 7}
	distant.ConnnectedToColony = true
	colony.AddFamily(&distant)

	for _, bot := range []*core.Bot{&member, &disconnected, &foreign, &distant} {
		addTestBot(g, bot)
	}
	depot := core.Depot{Pos: depotPos, Owner: &member, Colony: &colony, Food: 3, Ore: 1}

	g.handleDepot(&depot, depotPos)

	if depot.Food != 5 || depot.Ore != 3 {
		t.Fatalf("depot after deposit = F%d O%d, want F5 O3", depot.Food, depot.Ore)
	}
	if member.Inventory.Food != 3 || member.Inventory.Ore != 3 {
		t.Fatalf("member inventory after depot deposit = %+v, want F3 O3", member.Inventory)
	}
	if member.Evolution.DepotDepositedFood != 2 || member.Evolution.DepotDepositedOre != 2 {
		t.Fatalf("depot deposit stats = %+v, want F2 O2", member.Evolution)
	}
	if disconnected.Inventory.Food != 7 || disconnected.Inventory.Ore != 7 {
		t.Fatalf("disconnected inventory changed: %+v", disconnected.Inventory)
	}
	if foreign.Inventory.Food != 7 || foreign.Inventory.Ore != 7 {
		t.Fatalf("foreign inventory changed: %+v", foreign.Inventory)
	}
	if distant.Inventory.Food != 7 || distant.Inventory.Ore != 7 {
		t.Fatalf("distant inventory changed: %+v", distant.Inventory)
	}
}

func TestNearbyDepotResourcesEnableDivisionAndBuilding(t *testing.T) {
	cfg := config.NewConfig()
	cfg.DivisionFoodCost = 2
	cfg.DivisionOreCost = 2
	cfg.DivisionCost = 7
	cfg.DepotAccessRadius = 2
	cfg.MineBuildCost = 2
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	colony := core.NewColony(util.NewPos(20, 20))
	botPos := util.NewPos(20, 20)
	bot := core.NewBot(botPos)
	bot.Hp = cfg.DivisionMinHp + 20
	bot.Inventory.Food = 1
	bot.Genome.Matrix[0] = int(core.OpDivide)
	bot.ConnnectedToColony = true
	colony.AddFamily(&bot)
	addTestBot(g, &bot)
	depotPos := botPos.AddDir(core.Right)
	g.Board.Set(depotPos, core.Depot{Pos: depotPos, Owner: &bot, Colony: &colony, Food: 1, Ore: 2})

	if !g.DivisionReady(&bot) {
		t.Fatalf("bot should be division-ready with nearby depot resources")
	}
	g.botAction(botPos, &bot)

	if got := g.liveBotCount(); got != 2 {
		t.Fatalf("live bots after depot-funded divide = %d, want 2", got)
	}
	depot, ok := g.Board.At(depotPos).(core.Depot)
	if !ok || depot.Food != 0 || depot.Ore != 0 {
		t.Fatalf("depot after division = %+v ok=%v, want empty", depot, ok)
	}

	buildPos := util.NewPos(25, 25)
	builderPos := buildPos.AddRowCol(-1, 0)
	builder := core.NewBot(builderPos)
	builder.Genome.Matrix[1] = 2
	builder.Genome.Matrix[2] = int(core.BuildMine)
	builder.ConnnectedToColony = true
	colony.AddFamily(&builder)
	addTestBot(g, &builder)
	buildDepotPos := builderPos.AddRowCol(0, -1)
	g.Board.Set(buildDepotPos, core.Depot{Pos: buildDepotPos, Owner: &builder, Colony: &colony, Ore: cfg.MineBuildCost})

	g.build(builderPos, &builder)

	if _, ok := g.Board.At(buildPos).(core.Mine); !ok {
		t.Fatalf("built cell = %T, want Mine", g.Board.At(buildPos))
	}
}

func TestDepotAndControllerSharedResourcesAreLocal(t *testing.T) {
	cfg := config.NewConfig()
	cfg.DivisionFoodCost = 1
	cfg.DivisionOreCost = 1
	cfg.DepotAccessRadius = 2
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	botPos := util.NewPos(20, 20)
	bot := core.NewBot(botPos)
	bot.ConnnectedToColony = true
	colony := core.NewColony(botPos)
	colony.FoodBank = 1
	colony.OreBank = 1
	colony.AddFamily(&bot)
	addTestBot(g, &bot)

	if g.canPayShared(&bot, 1, 1) {
		t.Fatalf("bot used colony bank without nearby controller")
	}

	foreignColony := core.NewColony(botPos.AddDir(core.Right))
	g.Board.Set(botPos.AddDir(core.Right), core.Controller{Pos: botPos.AddDir(core.Right), Colony: &foreignColony})
	if g.canPayShared(&bot, 1, 1) {
		t.Fatalf("bot used foreign nearby controller bank access")
	}
	g.Board.Clear(botPos.AddDir(core.Right))

	farCtrlPos := botPos.AddRowCol(0, cfg.DepotAccessRadius+1)
	g.Board.Set(farCtrlPos, core.Controller{Pos: farCtrlPos, Colony: &colony})
	if g.canPayShared(&bot, 1, 1) {
		t.Fatalf("bot used out-of-radius controller bank access")
	}
	g.Board.Clear(farCtrlPos)

	ctrlPos := botPos.AddDir(core.Right)
	g.Board.Set(ctrlPos, core.Controller{Pos: ctrlPos, Owner: &bot, Colony: &colony})
	if !g.canPayShared(&bot, 1, 1) {
		t.Fatalf("bot could not use nearby same-colony controller bank")
	}
	if !g.spendShared(&bot, 1, 1) {
		t.Fatalf("nearby controller bank spend failed")
	}
	if colony.FoodBank != 0 || colony.OreBank != 0 {
		t.Fatalf("bank after local spend = F%d O%d, want empty", colony.FoodBank, colony.OreBank)
	}

	depotPos := botPos.AddRowCol(0, cfg.DepotAccessRadius+1)
	g.Board.Set(depotPos, core.Depot{Pos: depotPos, Colony: &colony, Food: 1, Ore: 1})
	if g.accessibleInventory(&bot).Total() != 0 {
		t.Fatalf("out-of-radius depot was counted in accessible inventory")
	}
}

func TestForeignColonyBotsRaidDepotsByGrabAndAttack(t *testing.T) {
	cfg := config.NewConfig()
	cfg.DepotRaidFoodLimit = 4
	cfg.DepotRaidOreLimit = 4
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	attackerPos := util.NewPos(20, 20)
	attacker := core.NewBot(attackerPos)
	attacker.Dir = core.Up
	attackerColony := core.NewColony(attackerPos)
	attackerColony.AddFamily(&attacker)
	owner := core.NewBot(util.NewPos(20, 22))
	colony := core.NewColony(attackerPos.AddDir(core.Up))
	colony.AddFamily(&owner)
	depotPos := attackerPos.AddRowCol(attacker.Dir[0], attacker.Dir[1])
	addTestBot(g, &attacker)
	g.Board.Set(depotPos, core.Depot{Pos: depotPos, Owner: &owner, Colony: &colony, Food: 10, Ore: 9})

	g.grab(attackerPos, &attacker)

	if attacker.Inventory.Food != 4 || attacker.Inventory.Ore != 4 {
		t.Fatalf("attacker inventory after depot grab = %+v, want F4 O4", attacker.Inventory)
	}
	if attacker.Evolution.FoodGathered != 0 || attacker.Evolution.OreGathered != 0 {
		t.Fatalf("depot raid counted as gathered: %+v", attacker.Evolution)
	}
	if attacker.Evolution.StolenFood != 4 || attacker.Evolution.StolenOre != 4 || attacker.Evolution.DepotRaids != 1 {
		t.Fatalf("depot grab telemetry = %+v, want stolen F4 O4 raids 1", attacker.Evolution)
	}
	if g.StolenFood() != 4 || g.StolenOre() != 4 || g.DepotRaids() != 1 {
		t.Fatalf("game depot raid totals = stolen F%d O%d raids %d, want F4 O4 raids 1", g.StolenFood(), g.StolenOre(), g.DepotRaids())
	}
	depot, ok := g.Board.At(depotPos).(core.Depot)
	if !ok || depot.Food != 6 || depot.Ore != 5 {
		t.Fatalf("depot after grab raid = %+v ok=%v, want F6 O5", depot, ok)
	}

	attackPos := util.NewPos(24, 20)
	attacker2 := core.NewBot(attackPos)
	attacker2.Genome.Matrix[0] = int(core.OpAttack)
	attacker2.Genome.Matrix[1] = 0
	attackerColony.AddFamily(&attacker2)
	depot2Pos := attackPos.AddDir(core.PosClock[0])
	addTestBot(g, &attacker2)
	g.Board.Set(depot2Pos, core.Depot{Pos: depot2Pos, Owner: &owner, Colony: &colony, Food: 5, Ore: 5})

	g.botAction(attackPos, &attacker2)

	if attacker2.Inventory.Food != 4 || attacker2.Inventory.Ore != 4 || attacker2.Evolution.DepotRaids != 1 {
		t.Fatalf("attacker2 after depot attack = inv %+v evo %+v, want F4 O4 raid 1", attacker2.Inventory, attacker2.Evolution)
	}

	solo := core.NewBot(util.NewPos(30, 30))
	blockedDepot := core.Depot{Pos: util.NewPos(30, 31), Colony: &colony, Food: 4, Ore: 4}
	if g.raidDepot(&solo, &blockedDepot) {
		t.Fatalf("colonyless bot raided depot")
	}
	if solo.Inventory.Total() != 0 || blockedDepot.Food != 4 || blockedDepot.Ore != 4 {
		t.Fatalf("blocked depot raid mutated solo/depot: inv %+v depot %+v", solo.Inventory, blockedDepot)
	}
}

func TestDepotContributionImprovesEvolutionScore(t *testing.T) {
	cfg := config.NewConfig()
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	contributor := core.NewBot(util.NewPos(20, 20))
	contributor.ConnnectedToColony = true
	contributor.Evolution.DepotBuilds = 1
	contributor.Evolution.DepotDepositedFood = 10
	contributor.Evolution.DepotDepositedOre = 8
	colony := core.NewColony(contributor.Pos)
	colony.AddFamily(&contributor)
	addTestBot(g, &contributor)
	g.Board.Set(contributor.Pos.AddDir(core.Right), core.Depot{Pos: contributor.Pos.AddDir(core.Right), Owner: &contributor, Colony: &colony})

	holder := core.NewBot(util.NewPos(22, 20))
	holder.Inventory = core.Inventory{Food: 20, Ore: 20}
	addTestBot(g, &holder)

	if contributorScore, holderScore := g.BotEvolutionScore(&contributor), g.BotEvolutionScore(&holder); contributorScore <= holderScore {
		t.Fatalf("depot contributor score = %d, holder score = %d, want contributor higher", contributorScore, holderScore)
	}
}

func TestDepotInspectSaveLookAndGrabRecognition(t *testing.T) {
	cfg := config.NewConfig()
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	botPos := util.NewPos(20, 20)
	depotPos := botPos.AddRowCol(1, 1)
	bot := core.NewBot(botPos)
	bot.Dir = core.Direction{1, 1}
	bot.Genome.Matrix[0] = int(core.OpLook)
	bot.Genome.Matrix[2] = 1
	colony := core.NewColony(depotPos)
	colony.AddFamily(&bot)
	bot.ConnnectedToColony = true
	addTestBot(g, &bot)
	g.Board.Set(depotPos, core.Depot{Pos: depotPos, Owner: &bot, Colony: &colony, Food: 3, Ore: 4})

	if !g.Board.IsGrabable(depotPos) {
		t.Fatalf("depot should be grabbable")
	}
	report := g.inspectGodCell(depotPos)
	if !hasReportLine(report.Lines, "Depot F 3 O 4") {
		t.Fatalf("depot inspect lines = %v, want depot storage line", report.Lines)
	}

	g.lookAround(botPos, &bot)
	if bot.Genome.NextArg != 52 {
		t.Fatalf("look depot next arg = %d, want 52", bot.Genome.NextArg)
	}

	bot.Genome.Pointer = 0
	g.grab(botPos, &bot)
	if bot.Genome.NextArg != 9 {
		t.Fatalf("friendly depot grab next arg = %d, want 9", bot.Genome.NextArg)
	}

	path, cells, _, _, err := g.saveMapToDir(t.TempDir())
	if err != nil {
		t.Fatalf("save map with depot: %v", err)
	}
	if cells == 0 {
		t.Fatalf("saved no cells, want depot cell")
	}
	var saved mapSaveFile
	readJSONFile(t, path, &saved)
	if saved.Counts["depot"] != 1 {
		t.Fatalf("saved counts = %+v, want one depot", saved.Counts)
	}
	found := false
	for _, cell := range saved.Cells {
		if cell.Kind == "depot" {
			found = true
			if cell.Food != 3 || cell.Ore != 4 || !cell.HasOwner || !cell.HasColony {
				t.Fatalf("saved depot cell = %+v, want F3 O4 owner colony", cell)
			}
		}
	}
	if !found {
		t.Fatalf("saved cells missing depot: %+v", saved.Cells)
	}
}

func TestBuildControllerSpendsConfiguredOre(t *testing.T) {
	cfg := config.NewConfig()
	cfg.ControllerBuildCost = 5
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	buildPos := util.NewPos(20, 20)
	botPos := buildPos.AddRowCol(-1, 0)
	bot := core.NewBot(botPos)
	bot.Inventory.Ore = cfg.ControllerBuildCost
	bot.Genome.Matrix[1] = 2
	bot.Genome.Matrix[2] = int(core.BuildController)
	kin := core.NewBot(botPos.AddDir(core.Right))
	kin.Genome = bot.Genome

	g.Board.Bots[util.Idx(botPos)] = &bot
	g.Board.Bots[util.Idx(kin.Pos)] = &kin
	g.Board.Set(botPos, &bot)
	g.Board.Set(kin.Pos, &kin)

	g.build(botPos, &bot)

	if _, ok := g.Board.At(buildPos).(core.Controller); !ok {
		t.Fatalf("built cell = %T, want Controller", g.Board.At(buildPos))
	}
	if bot.Inventory.Ore != 0 {
		t.Fatalf("ore after controller build = %d, want 0", bot.Inventory.Ore)
	}
	if bot.Colony == nil {
		t.Fatalf("builder should join new colony")
	}
	if kin.Colony != bot.Colony {
		t.Fatalf("nearby kin colony = %p, want builder colony %p", kin.Colony, bot.Colony)
	}
}

func TestBuildControllerRejectsNewColonyAtActiveLimit(t *testing.T) {
	cfg := config.NewConfig()
	cfg.ControllerBuildCost = 5
	cfg.ColonyMaxActive = 1
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	existingPos := util.NewPos(20, 20)
	existingColony := core.NewColony(existingPos)
	owner := core.NewBot(existingPos.AddDir(core.Right))
	owner.ConnnectedToColony = true
	existingColony.AddFamily(&owner)
	addTestBot(g, &owner)
	g.Board.Set(existingPos, core.Controller{Pos: existingPos, Owner: &owner, Colony: &existingColony, Amount: 10})

	buildPos := util.NewPos(80, 80)
	botPos := buildPos.AddRowCol(-1, 0)
	bot := core.NewBot(botPos)
	bot.Inventory.Ore = cfg.ControllerBuildCost
	bot.Genome.Matrix[1] = 2
	bot.Genome.Matrix[2] = int(core.BuildController)
	kin := core.NewBot(botPos.AddDir(core.Right))
	kin.Genome = bot.Genome
	addTestBot(g, &bot)
	addTestBot(g, &kin)

	g.build(botPos, &bot)

	if got := g.Board.At(buildPos); got != nil {
		t.Fatalf("capped controller build cell = %T, want nil", got)
	}
	if bot.Inventory.Ore != cfg.ControllerBuildCost {
		t.Fatalf("ore after capped controller build = %d, want unchanged %d", bot.Inventory.Ore, cfg.ControllerBuildCost)
	}
	if bot.Colony != nil || kin.Colony != nil {
		t.Fatalf("capped builder/kin joined colonies: %p/%p", bot.Colony, kin.Colony)
	}
}

func TestFirstControllerBuildCreatesFreeWallsGatesAndSpawners(t *testing.T) {
	cfg := config.NewConfig()
	cfg.ControllerBuildCost = 1
	cfg.ColonyAutoWallRadius = 3
	cfg.ColonyAutoWallHp = 44
	cfg.ColonyInitialSpawners = 3
	cfg.SpawnerInitialAmount = 5
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	buildPos := util.NewPos(80, 80)
	botPos := buildPos.AddRowCol(-1, 0)
	bot := core.NewBot(botPos)
	bot.Inventory.Ore = cfg.ControllerBuildCost
	bot.Genome.Matrix[1] = 2
	bot.Genome.Matrix[2] = int(core.BuildController)
	kin := core.NewBot(botPos.AddDir(core.Right))
	kin.Genome = bot.Genome
	addTestBot(g, &bot)
	addTestBot(g, &kin)

	g.build(botPos, &bot)

	if bot.Colony == nil {
		t.Fatalf("builder should found a colony")
	}
	radius := cfg.ColonyAutoWallRadius
	walls := 0
	gates := 0
	for dr := -radius; dr <= radius; dr++ {
		for dc := -radius; dc <= radius; dc++ {
			if max(util.Abs(dr), util.Abs(dc)) != radius {
				continue
			}
			pos := buildPos.AddRowCol(dr, dc)
			if colonyAutoWallGateCell(dr, dc, radius) {
				if _, ok := g.Board.At(pos).(core.Building); ok {
					t.Fatalf("gate cell %v contains a wall building", pos)
				}
				gates++
				continue
			}
			wall, ok := g.Board.At(pos).(core.Building)
			if !ok {
				t.Fatalf("perimeter cell %v = %T, want Building", pos, g.Board.At(pos))
			}
			if wall.Owner != &bot || wall.Hp != cfg.ColonyAutoWallHp {
				t.Fatalf("wall at %v = %+v, want owner founder hp %d", pos, wall, cfg.ColonyAutoWallHp)
			}
			walls++
		}
	}
	if gates != 12 {
		t.Fatalf("gate cells = %d, want four 3-cell gates", gates)
	}
	if walls != 12 {
		t.Fatalf("wall cells = %d, want perimeter minus gates = 12", walls)
	}

	spawners := 0
	for _, cell := range *g.Board.GetGrid() {
		spawner, ok := cell.(core.Spawner)
		if !ok {
			continue
		}
		if spawner.Owner != &bot || spawner.Colony != bot.Colony || spawner.Amount != cfg.SpawnerInitialAmount || !spawner.AutoBirth {
			t.Fatalf("initial spawner = %+v, want founder-owned colony spawner amount %d", spawner, cfg.SpawnerInitialAmount)
		}
		if !spawner.Pos.InRadius(buildPos, radius-1) {
			t.Fatalf("initial spawner at %v outside wall interior around %v", spawner.Pos, buildPos)
		}
		spawners++
	}
	if spawners != cfg.ColonyInitialSpawners {
		t.Fatalf("initial spawners = %d, want %d", spawners, cfg.ColonyInitialSpawners)
	}
}

func TestBuildControllerRejectedForLoneColonylessBot(t *testing.T) {
	cfg := config.NewConfig()
	cfg.ControllerBuildCost = 5
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	buildPos := util.NewPos(20, 20)
	botPos := buildPos.AddRowCol(-1, 0)
	bot := core.NewBot(botPos)
	bot.Inventory.Ore = cfg.ControllerBuildCost
	bot.Genome.Matrix[1] = 2
	bot.Genome.Matrix[2] = int(core.BuildController)

	g.Board.Bots[util.Idx(botPos)] = &bot
	g.Board.Set(botPos, &bot)

	g.build(botPos, &bot)

	if got := g.Board.At(buildPos); got != nil {
		t.Fatalf("lone controller build cell = %T, want nil", got)
	}
	if bot.Inventory.Ore != cfg.ControllerBuildCost {
		t.Fatalf("ore after lone rejected controller build = %d, want unchanged %d", bot.Inventory.Ore, cfg.ControllerBuildCost)
	}
	if bot.Colony != nil {
		t.Fatalf("lone builder joined colony %p", bot.Colony)
	}
}

func TestBuildControllerClaimsNearbyOwnedFarm(t *testing.T) {
	cfg := config.NewConfig()
	cfg.ControllerBuildCost = 5
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	buildPos := util.NewPos(20, 20)
	botPos := buildPos.AddRowCol(-1, 0)
	farmPos := buildPos.AddRowCol(1, 0)
	bot := core.NewBot(botPos)
	bot.Inventory.Ore = cfg.ControllerBuildCost
	bot.Genome.Matrix[1] = 2
	bot.Genome.Matrix[2] = int(core.BuildController)
	kin := core.NewBot(botPos.AddDir(core.Right))
	kin.Genome = bot.Genome

	g.Board.Bots[util.Idx(botPos)] = &bot
	g.Board.Bots[util.Idx(kin.Pos)] = &kin
	g.Board.Set(botPos, &bot)
	g.Board.Set(kin.Pos, &kin)
	g.Board.Set(farmPos, core.Farm{Pos: farmPos, Owner: &bot, Amount: 1})

	g.build(botPos, &bot)

	if bot.Colony == nil {
		t.Fatalf("builder should join new colony")
	}
	farm, ok := g.Board.At(farmPos).(core.Farm)
	if !ok {
		t.Fatalf("nearby farm missing after controller build")
	}
	if farm.Colony != bot.Colony {
		t.Fatalf("farm colony = %p, want builder colony %p", farm.Colony, bot.Colony)
	}
}

func TestConnectedColonyMemberCanBuildAdditionalControllerOutsideExclusionRadius(t *testing.T) {
	cfg := config.NewConfig()
	cfg.ControllerBuildCost = 3
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	existingPos := util.NewPos(20, 20)
	buildPos := existingPos.AddRowCol(0, controllerBuildMinRadius+2)
	botPos := buildPos.AddRowCol(-1, 0)
	owner := core.NewBot(existingPos.AddDir(core.Right))
	member := core.NewBot(botPos)
	member.Inventory.Ore = cfg.ControllerBuildCost
	member.ConnnectedToColony = true
	member.Genome.Matrix[1] = 2
	member.Genome.Matrix[2] = int(core.BuildController)
	colony := core.NewColony(existingPos)
	colony.AddFamily(&owner)
	colony.AddFamily(&member)

	for _, bot := range []*core.Bot{&owner, &member} {
		g.Board.Bots[util.Idx(bot.Pos)] = bot
		g.Board.Set(bot.Pos, bot)
	}
	g.Board.Set(existingPos, core.Controller{Pos: existingPos, Owner: &owner, Colony: &colony, Amount: 10})

	g.build(botPos, &member)

	ctrl, ok := g.Board.At(buildPos).(core.Controller)
	if !ok {
		t.Fatalf("additional controller cell = %T, want Controller", g.Board.At(buildPos))
	}
	if ctrl.Colony != &colony || ctrl.Owner != &member {
		t.Fatalf("additional controller = %+v, want same colony owner member", ctrl)
	}
	if member.Inventory.Ore != 0 {
		t.Fatalf("ore after additional controller build = %d, want 0", member.Inventory.Ore)
	}
}

func TestBuildControllerRejectedNearExistingController(t *testing.T) {
	cfg := config.NewConfig()
	cfg.ControllerBuildCost = 1
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	existingPos := util.NewPos(20, 20)
	existingColony := core.NewColony(existingPos)
	g.Board.Set(existingPos, core.Controller{
		Pos:    existingPos,
		Colony: &existingColony,
		Amount: 10,
	})

	buildPos := existingPos.AddRowCol(2, 0)
	botPos := buildPos.AddRowCol(-1, 0)
	bot := core.NewBot(botPos)
	bot.Inventory.Ore = 5
	bot.Genome.Matrix[1] = 2
	bot.Genome.Matrix[2] = int(core.BuildController)

	g.Board.Bots[util.Idx(botPos)] = &bot
	g.Board.Set(botPos, &bot)

	g.build(botPos, &bot)

	if got := g.Board.At(buildPos); got != nil {
		t.Fatalf("nearby controller build cell = %T, want nil", got)
	}
	if bot.Inventory.Ore != 5 {
		t.Fatalf("ore after rejected controller build = %d, want unchanged 5", bot.Inventory.Ore)
	}
	if bot.Colony != nil {
		t.Fatalf("bot joined colony after rejected build")
	}
}

func TestPopulateBoardDoesNotOverlayResourceOnCopiedBot(t *testing.T) {
	cfg := config.NewConfig()
	cfg.ResourceChance = 100
	cfg.PoisonChance = 0
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	botPos := util.NewPos(10, 10)
	bot := core.NewBot(botPos)
	g.Board.Bots[util.Idx(botPos)] = &bot
	g.Board.Set(botPos, &bot)

	g.populateBoard()

	if got := g.Board.GetBot(botPos); got != &bot {
		t.Fatalf("copied bot = %p, want %p", got, &bot)
	}
	if _, ok := g.Board.At(botPos).(*core.Bot); !ok {
		t.Fatalf("grid occupant at bot pos = %T, want *core.Bot", g.Board.At(botPos))
	}
}

func TestPopulateBoardMarksClearedResourceCellDirty(t *testing.T) {
	cfg := config.NewConfig()
	cfg.ResourceChance = 0
	cfg.PoisonChance = 0
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	resourcePos := util.NewPos(10, 10)
	g.Board.Set(resourcePos, core.Resource{Pos: resourcePos, Amount: 1})

	g.populateBoard()

	if got := g.Board.At(resourcePos); got != nil {
		t.Fatalf("resource cell after repopulate = %T, want nil", got)
	}
	if !g.Board.DirtyBitmap()[util.Idx(resourcePos)] {
		t.Fatalf("cleared resource cell was not marked dirty")
	}
}

func TestGodToolsPaintSoftCellsAndSkipBots(t *testing.T) {
	cfg := config.NewConfig()
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	center := util.NewPos(20, 20)
	bot := core.NewBot(center)
	g.Board.Bots[util.Idx(center)] = &bot
	g.Board.Set(center, &bot)
	target := center.AddRowCol(0, 1)

	report := g.ApplyGodTool(ui.GodToolWater, center, 1)
	if report.Message == "" {
		t.Fatalf("expected water report")
	}
	if got := g.Board.GetBot(center); got != &bot {
		t.Fatalf("god paint overwrote bot")
	}
	if _, ok := g.Board.At(target).(core.Water); !ok {
		t.Fatalf("target cell = %T, want Water", g.Board.At(target))
	}

	g.ApplyGodTool(ui.GodToolPoison, target, 0)
	if _, ok := g.Board.At(target).(core.Poison); !ok {
		t.Fatalf("target cell = %T, want Poison", g.Board.At(target))
	}

	g.ApplyGodTool(ui.GodToolFood, target, 0)
	if _, ok := g.Board.At(target).(core.Food); !ok {
		t.Fatalf("target cell = %T, want Food", g.Board.At(target))
	}
}

func TestGodToolSpawnColonySelectBlessAndCurse(t *testing.T) {
	cfg := config.NewConfig()
	cfg.ControllerInitialAmount = 100
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	pos := util.NewPos(30, 30)
	report := g.ApplyGodTool(ui.GodToolColony, pos, 0)
	if report.Message == "" {
		t.Fatalf("expected colony spawn report")
	}
	if g.selectedColony == nil {
		t.Fatalf("spawned colony was not selected")
	}
	if g.config.LiveBots != 1 {
		t.Fatalf("live bots = %d, want 1", g.config.LiveBots)
	}
	if _, ok := g.Board.At(pos).(core.Controller); !ok {
		t.Fatalf("spawn cell = %T, want Controller", g.Board.At(pos))
	}

	colony := g.selectedColony
	foundSelected := false
	for _, bot := range g.Board.Bots {
		if bot != nil && bot.Colony == colony {
			foundSelected = bot.IsSelected
			bot.Hp = 100
			bot.Inventory.Clear()
			break
		}
	}
	if !foundSelected {
		t.Fatalf("spawned colony founder was not selected")
	}

	g.ApplyGodTool(ui.GodToolBless, pos, 0)
	for _, bot := range g.Board.Bots {
		if bot != nil && bot.Colony == colony {
			if bot.Hp <= 100 {
				t.Fatalf("blessed bot hp = %d, want above 100", bot.Hp)
			}
			if bot.Inventory.Food == 0 || bot.Inventory.Ore == 0 {
				t.Fatalf("blessed bot inventory = %+v, want food and ore", bot.Inventory)
			}
		}
	}

	g.ApplyGodTool(ui.GodToolCurse, pos, 0)
	for _, bot := range g.Board.Bots {
		if bot != nil && bot.Colony == colony {
			if bot.Inventory.Total() != 0 {
				t.Fatalf("cursed bot inventory = %+v, want empty", bot.Inventory)
			}
			if bot.Hp >= 220 {
				t.Fatalf("cursed bot hp = %d, want reduced", bot.Hp)
			}
		}
	}
}

func TestGodFreezePausesBotsAndBlocksMovement(t *testing.T) {
	cfg := config.NewConfig()
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	pos := util.NewPos(40, 40)
	bot := core.NewBot(pos)
	bot.Hp = 100
	bot.Dir = core.Right
	g.Board.Bots[util.Idx(pos)] = &bot
	g.Board.Set(pos, &bot)

	g.ApplyGodTool(ui.GodToolFreeze, pos, 0)
	g.botsActions()
	if bot.Hp != 100 {
		t.Fatalf("frozen bot hp changed to %d, want 100", bot.Hp)
	}

	g.ApplyGodTool(ui.GodToolUnfreeze, pos, 0)
	next := pos.AddDir(core.Right)
	g.ApplyGodTool(ui.GodToolFreeze, next, 0)
	g.tryMove(pos, &bot)
	if bot.Pos != pos {
		t.Fatalf("bot moved into frozen cell: got %v, want %v", bot.Pos, pos)
	}
}

func TestControllerCrowdingPressureAppliesOnlyAboveThreshold(t *testing.T) {
	cfg := config.NewConfig()
	belowGame, belowCtrl, belowBots := controllerCrowdFixture(t, cfg.ControllerCrowdThreshold)
	belowGame.handleController(belowCtrl, belowCtrl.Pos)
	for _, bot := range belowBots {
		if bot.Hp != 103 {
			t.Fatalf("below threshold bot hp = %d, want healed to 103", bot.Hp)
		}
	}

	twentyGame, twentyCtrl, twentyBots := controllerCrowdFixture(t, 20)
	twentyGame.handleController(twentyCtrl, twentyCtrl.Pos)
	for _, bot := range twentyBots {
		if bot.Hp != 103 {
			t.Fatalf("20-member colony bot hp = %d, want healed without crowd tax to 103", bot.Hp)
		}
	}

	crowdedGame, crowdedCtrl, crowdedBots := controllerCrowdFixture(t, cfg.ControllerCrowdThreshold+1)
	crowdedGame.handleController(crowdedCtrl, crowdedCtrl.Pos)
	for _, bot := range crowdedBots {
		if bot.Hp != 102 {
			t.Fatalf("crowded bot hp = %d, want healed then taxed to 102", bot.Hp)
		}
	}
}

func controllerCrowdFixture(t *testing.T, memberCount int) (*Game, *core.Controller, []*core.Bot) {
	t.Helper()
	cfg := config.NewConfig()
	g := NewGame(&cfg)
	g.Board = core.NewBoard()
	ctrlPos := util.NewPos(80, 80)
	colony := core.NewColony(ctrlPos)
	bots := make([]*core.Bot, 0, memberCount)

	for r := ctrlPos.R - controllerCrowdRadius; r <= ctrlPos.R+controllerCrowdRadius && len(bots) < memberCount; r++ {
		for dc := -controllerCrowdRadius; dc <= controllerCrowdRadius && len(bots) < memberCount; dc++ {
			pos := util.NewPos(r, ctrlPos.C+dc)
			if pos == ctrlPos || util.OutOfBounds(pos) {
				continue
			}
			bot := core.NewBot(pos)
			bot.Hp = 100
			bot.ConnnectedToColony = true
			colony.AddFamily(&bot)
			g.Board.Bots[util.Idx(pos)] = &bot
			g.Board.Set(pos, &bot)
			bots = append(bots, &bot)
		}
	}
	if len(bots) != memberCount {
		t.Fatalf("created %d bots, want %d", len(bots), memberCount)
	}

	ctrl := &core.Controller{
		Pos:    ctrlPos,
		Owner:  bots[0],
		Colony: &colony,
		Amount: memberCount * 10,
	}
	g.Board.Set(ctrlPos, *ctrl)
	return g, ctrl, bots
}

func TestResetSimulationClearsWorldAndPreservesRuntimeConfig(t *testing.T) {
	cfg := config.NewConfig()
	cfg.BotChance = 0
	cfg.OceansCount = 0
	cfg.PoisonChance = 0
	cfg.ResourceChance = 0
	cfg.Pause = true
	cfg.LogicStep = 42

	g := NewGame(&cfg)
	g.EnableGameMaster(7)

	pos := util.NewPos(40, 40)
	bot := core.NewBot(pos)
	colony := core.NewColony(pos)
	colony.AddFamily(&bot)
	g.Board.Bots[util.Idx(pos)] = &bot
	g.Board.Set(pos, &bot)
	g.Colonies = []*core.Colony{&colony}
	g.selectedColony = &colony
	g.logicTick = 99
	g.currGen = 3
	g.maxHp = 500
	g.latestImprovement = 12
	g.config.LiveBots = 1

	oldBoard := g.Board
	g.ResetSimulation()

	if g.Board == oldBoard {
		t.Fatalf("board was not replaced")
	}
	if got := g.Board.At(pos); got != nil {
		t.Fatalf("reset cell = %T, want nil", got)
	}
	if len(g.Colonies) != 0 {
		t.Fatalf("colonies = %d, want 0", len(g.Colonies))
	}
	if g.selectedColony != nil {
		t.Fatalf("selected colony was not cleared")
	}
	if g.logicTick != 0 || g.currGen != 0 || g.maxHp != 0 || g.latestImprovement != 0 {
		t.Fatalf("progress counters not reset: tick=%d gen=%d maxHp=%d improvement=%d", g.logicTick, g.currGen, g.maxHp, g.latestImprovement)
	}
	if cfg.LiveBots != 0 {
		t.Fatalf("live bots = %d, want 0", cfg.LiveBots)
	}
	if !cfg.Pause || cfg.LogicStep != 42 {
		t.Fatalf("runtime config changed: pause=%v logicStep=%s", cfg.Pause, cfg.LogicStep)
	}
	if !g.State.GameMaster.Enabled || g.State.GameMaster.Name != mockGameMasterName || g.State.GameMaster.Interval != 7 {
		t.Fatalf("game master state = %+v, want enabled mock interval 7", g.State.GameMaster)
	}
}

func TestInspectControllerAndColonyLinesIncludeBank(t *testing.T) {
	cfg := config.NewConfig()
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	ctrlPos := util.NewPos(20, 20)
	ownerPos := util.NewPos(20, 21)
	owner := core.NewBot(ownerPos)
	colony := core.NewColony(ctrlPos)
	colony.FoodBank = 3
	colony.OreBank = 4
	colony.AddFamily(&owner)

	g.Board.Bots[util.Idx(ownerPos)] = &owner
	g.Board.Set(ownerPos, &owner)
	g.Board.Set(ctrlPos, core.Controller{
		Pos:    ctrlPos,
		Owner:  &owner,
		Colony: &colony,
		Amount: 10,
	})

	report := g.inspectGodCell(ctrlPos)

	if !hasReportLine(report.Lines, "Bank F 3 O 4") {
		t.Fatalf("controller inspect lines = %v, want bank line", report.Lines)
	}
	colonyLines := g.colonyInspectLines(&colony)
	if !hasReportLine(colonyLines, "Bank F 3 O 4") {
		t.Fatalf("colony inspect lines = %v, want bank line", colonyLines)
	}
}

func TestSaveGenomeWritesSelectedColonyChampion(t *testing.T) {
	cfg := config.NewConfig()
	cfg.MutationRate = 9
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	colony := core.NewColony(util.NewPos(10, 10))
	colony.FoodBank = 9
	colony.OreBank = 10
	weakerPos := util.NewPos(10, 11)
	strongerPos := util.NewPos(10, 12)
	weaker := core.NewBot(weakerPos)
	stronger := core.NewBot(strongerPos)
	weaker.Hp = 100
	stronger.Hp = 250
	stronger.Inventory = core.Inventory{Food: 1, Ore: 2}
	colony.AddFamily(&weaker)
	colony.AddFamily(&stronger)
	g.Board.Bots[util.Idx(weakerPos)] = &weaker
	g.Board.Bots[util.Idx(strongerPos)] = &stronger
	g.Board.Set(weakerPos, &weaker)
	g.Board.Set(strongerPos, &stronger)
	g.selectedColony = &colony
	g.logicTick = 77

	path, source, err := g.saveGenomeToDir(t.TempDir(), util.NewPos(20, 20))
	if err != nil {
		t.Fatalf("save genome: %v", err)
	}
	if source != "selected colony champion" {
		t.Fatalf("source = %q, want selected colony champion", source)
	}

	var saved genomeSaveFile
	readJSONFile(t, path, &saved)
	if saved.Kind != "golab_genome" || saved.Tick != 77 {
		t.Fatalf("saved metadata = kind %q tick %d", saved.Kind, saved.Tick)
	}
	if saved.Bot.Position != savePos(strongerPos) {
		t.Fatalf("saved bot pos = %+v, want %+v", saved.Bot.Position, savePos(strongerPos))
	}
	if saved.Bot.HP != 250 || saved.Bot.Inventory != 3 || saved.Bot.FoodInventory != 1 || saved.Bot.OreInventory != 2 {
		t.Fatalf("saved bot stats = hp %d inv %d food %d ore %d", saved.Bot.HP, saved.Bot.Inventory, saved.Bot.FoodInventory, saved.Bot.OreInventory)
	}
	if saved.Config.MutationRate != 9 {
		t.Fatalf("saved mutation rate = %d, want 9", saved.Config.MutationRate)
	}
	if saved.Colony == nil {
		t.Fatalf("saved colony is nil, want colony block")
	}
	if saved.Colony.FoodBank != 9 || saved.Colony.OreBank != 10 {
		t.Fatalf("saved colony bank = F%d O%d, want F9 O10", saved.Colony.FoodBank, saved.Colony.OreBank)
	}
}

func TestSaveMapWritesBoardCellsAndSkipsBots(t *testing.T) {
	cfg := config.NewConfig()
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	botPos := util.NewPos(20, 20)
	resourcePos := util.NewPos(21, 20)
	waterPos := util.NewPos(22, 20)
	frozenPos := util.NewPos(23, 20)
	bot := core.NewBot(botPos)
	g.Board.Bots[util.Idx(botPos)] = &bot
	g.Board.Set(botPos, &bot)
	g.Board.Set(resourcePos, core.Resource{Pos: resourcePos, Amount: 2})
	g.Board.Set(waterPos, core.Water{GroupId: 42, Amount: 10000})
	g.Board.SetFrozen(frozenPos, true)
	g.logicTick = 88

	path, cells, frozen, biomes, err := g.saveMapToDir(t.TempDir())
	if err != nil {
		t.Fatalf("save map: %v", err)
	}
	if cells != 2 {
		t.Fatalf("saved cells = %d, want 2", cells)
	}
	if frozen != 1 {
		t.Fatalf("saved frozen cells = %d, want 1", frozen)
	}
	if biomes == 0 {
		t.Fatalf("saved biome records = 0, want non-neutral biome records")
	}

	var saved mapSaveFile
	readJSONFile(t, path, &saved)
	if saved.Kind != "golab_map" || saved.Tick != 88 {
		t.Fatalf("saved metadata = kind %q tick %d", saved.Kind, saved.Tick)
	}
	if len(saved.Frozen) != 1 || saved.Frozen[0] != savePos(frozenPos) {
		t.Fatalf("saved frozen = %+v, want %+v", saved.Frozen, savePos(frozenPos))
	}
	if saved.Counts["resource"] != 1 || saved.Counts["water"] != 1 {
		t.Fatalf("saved counts = %+v", saved.Counts)
	}
	if saved.BiomeCounts[core.BiomeNeutral.String()]+saved.BiomeCounts[core.BiomeFertile.String()]+saved.BiomeCounts[core.BiomeMineral.String()]+saved.BiomeCounts[core.BiomeToxic.String()] != util.Cells {
		t.Fatalf("saved biome counts = %+v, want total %d", saved.BiomeCounts, util.Cells)
	}
	if len(saved.Biomes) != biomes {
		t.Fatalf("saved biome records = %d, want returned count %d", len(saved.Biomes), biomes)
	}
	for _, biome := range saved.Biomes {
		if biome.Kind == core.BiomeNeutral.String() {
			t.Fatalf("neutral biome should not be saved as compact record: %+v", biome)
		}
	}
	for _, cell := range saved.Cells {
		if cell.Position == savePos(botPos) {
			t.Fatalf("bot cell was saved as map cell: %+v", cell)
		}
	}
}

func readJSONFile(t *testing.T, path string, out any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}

func hasReportLine(lines []string, want string) bool {
	for _, line := range lines {
		if line == want {
			return true
		}
	}
	return false
}

func freezeAllExcept(t *testing.T, brd *core.Board, positions ...core.Position) {
	t.Helper()
	allowed := map[int]struct{}{}
	for _, pos := range positions {
		allowed[util.Idx(pos)] = struct{}{}
	}
	for i := 0; i < util.Cells; i++ {
		if _, ok := allowed[i]; ok {
			continue
		}
		brd.SetFrozen(util.PosOf(i), true)
	}
}

func exactGenomeBotsNear(brd *core.Board, center core.Position, genome core.Genome, radius int) int {
	count := 0
	for _, id := range brd.ActiveBotIDs() {
		bot := brd.BotByID(id)
		if bot == nil || bot.Genome.Matrix != genome.Matrix {
			continue
		}
		if bot.Pos.InRadius(center, radius) {
			count++
		}
	}
	return count
}

func testGenerationGenome(seed int) core.Genome {
	var genome core.Genome
	for i := range genome.Matrix {
		genome.Matrix[i] = (seed + i) % 61
	}
	genome.Pointer = 17
	genome.NextArg = 23
	genome.Signal = 5
	genome.Registers = [4]int{1, 2, 3, 4}
	return genome
}

func TestInitializeForScaleSeedsExactDeterministicTarget(t *testing.T) {
	const seed = 42
	const target = 250

	newSeeded := func() *Game {
		rand.Seed(seed)
		expRand.Seed(seed)
		cfg := config.NewConfig()
		cfg.LogicStep = 0
		g := NewGame(&cfg)
		if err := g.InitializeForScale(target, seed); err != nil {
			t.Fatalf("InitializeForScale: %v", err)
		}
		return g
	}

	first := newSeeded()
	second := newSeeded()

	if got := first.Board.ActiveBotCount(); got != target {
		t.Fatalf("first active bot count = %d, want %d", got, target)
	}
	if got := second.Board.ActiveBotCount(); got != target {
		t.Fatalf("second active bot count = %d, want %d", got, target)
	}
	if got := first.liveBotCount(); got != target {
		t.Fatalf("liveBotCount = %d, want %d", got, target)
	}

	firstCells := activeBotCells(first.Board)
	secondCells := activeBotCells(second.Board)
	if !slices.Equal(firstCells, secondCells) {
		t.Fatalf("scale seed placements differ for identical seed")
	}
}

func activeBotCells(brd *core.Board) []int {
	cells := make([]int, 0, brd.ActiveBotCount())
	for _, id := range brd.ActiveBotIDs() {
		if cell := brd.BotCell(id); cell >= 0 {
			cells = append(cells, cell)
		}
	}
	slices.Sort(cells)
	return cells
}

var benchmarkLiveBots int

func BenchmarkLiveBotCount(b *testing.B) {
	cfg := config.NewConfig()
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	for i := 0; i < util.Cells; i += 3 {
		pos := util.PosOf(i)
		if util.OutOfBounds(pos) {
			continue
		}
		bot := core.NewBot(pos)
		g.Board.Bots[i] = &bot
		g.Board.Set(pos, &bot)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		benchmarkLiveBots = g.liveBotCount()
	}
}

func BenchmarkEnvironmentActions(b *testing.B) {
	cfg := config.NewConfig()
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	for i := 0; i < util.Cells; i += 257 {
		pos := util.PosOf(i)
		if util.OutOfBounds(pos) {
			continue
		}
		g.Board.Set(pos, core.Organics{Pos: pos, Amount: b.N + 100})
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		g.environmentActions()
	}
}

func BenchmarkBotsActions100k(b *testing.B) {
	g := newScaleBenchmarkGame(b, 100000)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		g.botsActions()
	}
	benchmarkLiveBots = g.Board.ActiveBotCount()
}

func BenchmarkScaleTest100k(b *testing.B) {
	for range b.N {
		g := newScaleBenchmarkGame(b, 100000)
		g.RunHeadlessFrames(300)
		benchmarkLiveBots = g.Board.ActiveBotCount()
	}
}

func newScaleBenchmarkGame(tb testing.TB, target int) *Game {
	tb.Helper()
	const seed = 42
	rand.Seed(seed)
	expRand.Seed(seed)
	cfg := config.NewConfig()
	cfg.LogicStep = 0
	cfg.NewGenThreshold = 0
	cfg.ImmigrationBots = 0
	cfg.ImmigrationInterval = 0
	g := NewGame(&cfg)
	if err := g.InitializeForScale(target, seed); err != nil {
		tb.Fatalf("InitializeForScale: %v", err)
	}
	return g
}
