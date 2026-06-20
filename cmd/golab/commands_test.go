package main

import (
	"encoding/json"
	"golab/internal/config"
	"golab/internal/core"
	"golab/internal/game"
	"golab/internal/util"
	"io"
	"os"
	"reflect"
	"testing"
)

var benchmarkSummary matchSummary

func TestRunSmartnessEvalReportsRequiredFields(t *testing.T) {
	output := captureStdout(t, func() {
		runSmartnessEval([]string{"--seeds", "1 2 3", "--ticks", "5"})
	})

	var payload struct {
		Command        string                 `json:"command"`
		Ticks          int                    `json:"ticks"`
		SmartEvolution bool                   `json:"smart_evolution"`
		Runs           []smartnessEvalRun     `json:"runs"`
		Aggregate      smartnessEvalAggregate `json:"aggregate"`
	}
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("parse smartness-eval JSON: %v\noutput:\n%s", err, output)
	}
	if payload.Command != "smartness-eval" || payload.Ticks != 5 || !payload.SmartEvolution {
		t.Fatalf("smartness-eval header = command %q ticks %d smart %v", payload.Command, payload.Ticks, payload.SmartEvolution)
	}
	if len(payload.Runs) != 3 {
		t.Fatalf("run count = %d, want 3", len(payload.Runs))
	}
	for i, seed := range []int64{1, 2, 3} {
		run := payload.Runs[i]
		if run.Seed != seed || run.Ticks != 5 {
			t.Fatalf("run %d identity = seed %d ticks %d, want seed %d ticks 5", i, run.Seed, run.Ticks, seed)
		}
		if run.EliteCount < 0 || run.BestScore < 0 || run.LiveBots < 0 {
			t.Fatalf("run %d has invalid metrics: %+v", i, run)
		}
		if run.Spawners < 0 || run.SpawnerBirths < 0 || run.TotalSpawnerCharges < 0 || run.TopSpawnerActiveBots < 0 {
			t.Fatalf("run %d has invalid spawner metrics: %+v", i, run)
		}
		if run.MaxColonyComponent < 0 ||
			run.MaxConnectedComponent < 0 ||
			run.LongestColonyRun < 0 ||
			run.LongestConnectedRun < 0 ||
			run.ColonyTissueCells < 0 ||
			run.TopNonColonyDirectionShare < 0 {
			t.Fatalf("run %d has invalid colony organism metrics: %+v", i, run)
		}
	}
	if payload.Aggregate.Seeds != 3 || payload.Aggregate.Ticks != 5 {
		t.Fatalf("aggregate = %+v, want seeds 3 ticks 5", payload.Aggregate)
	}
	if payload.Aggregate.SeedsWithSpawners < 0 ||
		payload.Aggregate.TotalSpawnerBirths < 0 ||
		payload.Aggregate.SeedsWithNonColonySpawnerTop < 0 {
		t.Fatalf("aggregate has invalid spawner metrics: %+v", payload.Aggregate)
	}
	if payload.Aggregate.MedianMaxColonyComponent < 0 ||
		payload.Aggregate.MedianMaxConnectedComponent < 0 ||
		payload.Aggregate.MedianColonyTissueCells < 0 ||
		payload.Aggregate.MedianTopNonColonyDirectionShare < 0 {
		t.Fatalf("aggregate has invalid colony organism metrics: %+v", payload.Aggregate)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	os.Stdout = w
	defer func() {
		os.Stdout = old
	}()

	fn()
	if err := w.Close(); err != nil {
		t.Fatalf("close stdout pipe: %v", err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read stdout pipe: %v", err)
	}
	return string(data)
}

func TestSummarizeMatchTopBotsOrderingAndTieBreak(t *testing.T) {
	cfg := config.NewConfig()
	g := game.NewGame(&cfg)
	g.Board = core.NewBoard()

	lowHPHighInv := addSummaryBot(g, util.NewPos(10, 4), 90, 99)
	tiedLater := addSummaryBot(g, util.NewPos(10, 12), 100, 8)
	tiedEarlier := addSummaryBot(g, util.NewPos(10, 8), 100, 8)
	lowerInventory := addSummaryBot(g, util.NewPos(10, 20), 100, 4)
	addSummaryBot(g, util.NewPos(10, 30), 80, 1)

	summary := summarizeMatch(g, 99, 7, 3)

	if summary.LiveBots != 5 {
		t.Fatalf("live bots = %d, want 5", summary.LiveBots)
	}
	if summary.TotalHP != 470 {
		t.Fatalf("total hp = %d, want 470", summary.TotalHP)
	}
	if summary.TotalInv != 120 {
		t.Fatalf("total inventory = %d, want 120", summary.TotalInv)
	}
	if summary.TotalFoodInv != 59 || summary.TotalOreInv != 61 {
		t.Fatalf("total food/ore inventory = %d/%d, want 59/61", summary.TotalFoodInv, summary.TotalOreInv)
	}
	if summary.DivisionReadyBots != 0 {
		t.Fatalf("division ready bots = %d, want 0", summary.DivisionReadyBots)
	}
	got := summary.TopBots
	want := []int{lowHPHighInv, tiedEarlier, tiedLater}
	if len(got) != len(want) {
		t.Fatalf("top bot count = %d, want %d; top=%v", len(got), len(want), got)
	}
	for i := range want {
		if got[i].Index != want[i] {
			t.Fatalf("top bot index at %d = %d, want %d; top=%v", i, got[i].Index, want[i], got)
		}
	}
	if got[0].EvolutionScore <= got[1].EvolutionScore {
		t.Fatalf("top bot score = %d, want greater than second score %d; top=%v", got[0].EvolutionScore, got[1].EvolutionScore, got)
	}
	if got[1].Inventory != 8 || got[1].FoodInventory != 4 || got[1].OreInventory != 4 {
		t.Fatalf("second bot inventory fields = total %d food %d ore %d, want 8/4/4", got[1].Inventory, got[1].FoodInventory, got[1].OreInventory)
	}
	_ = lowerInventory

	none := summarizeMatch(g, 99, 7, 0)
	if len(none.TopBots) != 0 {
		t.Fatalf("top bots with zero limit = %v, want empty", none.TopBots)
	}
}

func TestSummarizeMatchReportsDivisionReadyBots(t *testing.T) {
	cfg := config.NewConfig()
	g := game.NewGame(&cfg)
	g.Board = core.NewBoard()

	ready := core.NewBot(util.NewPos(10, 4))
	ready.Hp = cfg.DivisionMinHp
	ready.Inventory = core.Inventory{Food: cfg.DivisionFoodCost, Ore: cfg.DivisionOreCost}
	readyIdx := util.Idx(ready.Pos)
	g.Board.Bots[readyIdx] = &ready
	g.Board.Set(ready.Pos, &ready)

	foodOnly := core.NewBot(util.NewPos(10, 8))
	foodOnly.Hp = cfg.DivisionMinHp
	foodOnly.Inventory = core.Inventory{Food: cfg.DivisionFoodCost}
	g.Board.Bots[util.Idx(foodOnly.Pos)] = &foodOnly
	g.Board.Set(foodOnly.Pos, &foodOnly)

	summary := summarizeMatch(g, 99, 7, 1)

	if summary.TotalInv != 3 || summary.TotalFoodInv != 2 || summary.TotalOreInv != 1 {
		t.Fatalf("inventory totals = total %d food %d ore %d, want 3/2/1", summary.TotalInv, summary.TotalFoodInv, summary.TotalOreInv)
	}
	if summary.DivisionReadyBots != 1 {
		t.Fatalf("division ready bots = %d, want 1", summary.DivisionReadyBots)
	}
	if summary.SuccessfulDivisions != 0 {
		t.Fatalf("successful divisions = %d, want 0", summary.SuccessfulDivisions)
	}
	if len(summary.TopBots) != 1 || summary.TopBots[0].Index != readyIdx {
		t.Fatalf("top bots = %+v, want ready bot %d", summary.TopBots, readyIdx)
	}
}

func TestSummarizeMatchReportsColonyBanks(t *testing.T) {
	cfg := config.NewConfig()
	g := game.NewGame(&cfg)
	g.Board = core.NewBoard()

	bot := core.NewBot(util.NewPos(10, 4))
	bot.Hp = cfg.DivisionMinHp
	bot.ConnnectedToColony = true
	colony := core.NewColony(util.NewPos(10, 3))
	colony.FoodBank = 7
	colony.OreBank = 11
	colony.AddFamily(&bot)
	botIdx := util.Idx(bot.Pos)
	g.Board.Bots[botIdx] = &bot
	g.Board.Set(bot.Pos, &bot)
	g.Board.Set(colony.Center, core.Controller{Pos: colony.Center, Owner: &bot, Colony: &colony, Amount: 10})

	summary := summarizeMatch(g, 99, 7, 1)

	if summary.ColonyCount != 1 {
		t.Fatalf("colony count = %d, want 1", summary.ColonyCount)
	}
	if summary.TotalColonyFoodBank != 7 || summary.TotalColonyOreBank != 11 {
		t.Fatalf("total colony bank = F%d O%d, want F7 O11", summary.TotalColonyFoodBank, summary.TotalColonyOreBank)
	}
	if summary.ColonyMemberBots != 1 || summary.ConnectedColonyBots != 1 {
		t.Fatalf("colony bot metrics = members %d connected %d, want 1/1", summary.ColonyMemberBots, summary.ConnectedColonyBots)
	}
	if summary.DivisionReadyBots != 1 {
		t.Fatalf("division ready bots = %d, want 1", summary.DivisionReadyBots)
	}
	if len(summary.TopBots) != 1 || summary.TopBots[0].Index != botIdx {
		t.Fatalf("top bots = %+v, want colony bot %d", summary.TopBots, botIdx)
	}
	top := summary.TopBots[0]
	if top.ColonyFoodBank == nil || *top.ColonyFoodBank != 7 {
		t.Fatalf("top bot colony food bank = %v, want 7", top.ColonyFoodBank)
	}
	if top.ColonyOreBank == nil || *top.ColonyOreBank != 11 {
		t.Fatalf("top bot colony ore bank = %v, want 11", top.ColonyOreBank)
	}
}

func TestSummarizeMatchReportsDepots(t *testing.T) {
	cfg := config.NewConfig()
	g := game.NewGame(&cfg)
	g.Board = core.NewBoard()

	depotPos := util.NewPos(10, 10)
	bot := core.NewBot(util.NewPos(10, 11))
	bot.Evolution.DepotBuilds = 1
	bot.Evolution.DepotDepositedFood = 3
	bot.Evolution.DepotDepositedOre = 2
	bot.Evolution.DepotRaids = 1
	colony := core.NewColony(depotPos)
	colony.AddFamily(&bot)
	bot.ConnnectedToColony = true
	g.Board.Bots[util.Idx(bot.Pos)] = &bot
	g.Board.Set(bot.Pos, &bot)
	g.Board.Set(depotPos, core.Depot{Pos: depotPos, Owner: &bot, Colony: &colony, Food: 5, Ore: 4})

	summary := summarizeMatch(g, 99, 7, 1)

	if summary.Depots != 1 || summary.TotalDepotFood != 5 || summary.TotalDepotOre != 4 {
		t.Fatalf("depot summary = depots %d food %d ore %d, want 1/5/4", summary.Depots, summary.TotalDepotFood, summary.TotalDepotOre)
	}
	if summary.ActiveColonies != 1 {
		t.Fatalf("active colonies with depot = %d, want 1", summary.ActiveColonies)
	}
	if len(summary.TopBots) != 1 {
		t.Fatalf("top bots = %d, want 1", len(summary.TopBots))
	}
	top := summary.TopBots[0]
	if top.DepotBuilds != 1 || top.DepotDepositedFood != 3 || top.DepotDepositedOre != 2 || top.DepotRaids != 1 {
		t.Fatalf("top depot fields = %+v, want build/deposit/raid stats", top)
	}
}

func TestSummarizeMatchReportsSpawners(t *testing.T) {
	cfg := config.NewConfig()
	g := game.NewGame(&cfg)
	g.Board = core.NewBoard()

	spawnerPos := util.NewPos(10, 10)
	bot := core.NewBot(util.NewPos(10, 11))
	bot.Evolution.SpawnerBuilds = 1
	bot.Evolution.SpawnerBirths = 2
	g.Board.Bots[util.Idx(bot.Pos)] = &bot
	g.Board.Set(bot.Pos, &bot)
	g.Board.Set(spawnerPos, core.Spawner{Pos: spawnerPos, Owner: &bot, Amount: 4})

	summary := summarizeMatch(g, 99, 7, 1)

	if summary.Spawners != 1 || summary.TotalSpawnerCharges != 4 {
		t.Fatalf("spawner summary = spawners %d charges %d, want 1/4", summary.Spawners, summary.TotalSpawnerCharges)
	}
	if summary.TopSpawnerActiveBots != 1 {
		t.Fatalf("top spawner-active bots = %d, want 1", summary.TopSpawnerActiveBots)
	}
	if len(summary.TopBots) != 1 {
		t.Fatalf("top bots = %d, want 1", len(summary.TopBots))
	}
	top := summary.TopBots[0]
	if top.SpawnerBuilds != 1 || top.SpawnerBirths != 2 {
		t.Fatalf("top spawner fields = %+v, want build/birth stats", top)
	}
}

func TestReplaySummarySameSeedIsDeterministic(t *testing.T) {
	first := runReplaySummary(7, 20, 10, 1)
	second := runReplaySummary(7, 20, 10, 1)
	clearSummaryTimestamps(first)
	clearSummaryTimestamps(second)

	if !reflect.DeepEqual(first, second) {
		t.Fatalf("same-seed replay summaries differ:\nfirst=%+v\nsecond=%+v", first, second)
	}
}

func clearSummaryTimestamps(frames []matchSummary) {
	for i := range frames {
		frames[i].Timestamp = ""
	}
}

func TestSummarizeMatchReportsActiveColonySizes(t *testing.T) {
	cfg := config.NewConfig()
	g := game.NewGame(&cfg)
	g.Board = core.NewBoard()

	ctrlPos := util.NewPos(10, 10)
	owner := core.NewBot(util.NewPos(10, 11))
	owner.ConnnectedToColony = true
	member := core.NewBot(util.NewPos(10, 12))
	colony := core.NewColony(ctrlPos)
	colony.AddFamily(&owner)
	colony.AddFamily(&member)

	for _, bot := range []*core.Bot{&owner, &member} {
		g.Board.Bots[util.Idx(bot.Pos)] = bot
		g.Board.Set(bot.Pos, bot)
	}
	g.Board.Set(ctrlPos, core.Controller{
		Pos:    ctrlPos,
		Owner:  &owner,
		Colony: &colony,
		Amount: 10,
	})

	summary := summarizeMatch(g, 99, 7, 0)

	if summary.ActiveColonies != 1 || summary.SoloActiveColonies != 0 {
		t.Fatalf("active/solo colonies = %d/%d, want 1/0", summary.ActiveColonies, summary.SoloActiveColonies)
	}
	if summary.MaxColonyMembers != 2 || summary.MaxConnectedMembers != 1 {
		t.Fatalf("max colony sizes = members %d connected %d, want 2/1", summary.MaxColonyMembers, summary.MaxConnectedMembers)
	}
}

func TestSummarizeMatchReportsFriendlyAndForeignAdjacencies(t *testing.T) {
	cfg := config.NewConfig()
	g := game.NewGame(&cfg)
	g.Board = core.NewBoard()

	left := core.NewBot(util.NewPos(10, 10))
	right := core.NewBot(util.NewPos(10, 11))
	foreign := core.NewBot(util.NewPos(10, 12))
	for i := range left.Genome.Matrix {
		left.Genome.Matrix[i] = 0
		right.Genome.Matrix[i] = 0
		foreign.Genome.Matrix[i] = core.OpcodeCount() + i
	}
	colony := core.NewColony(util.NewPos(9, 10))
	colony.AddFamily(&left)
	colony.AddFamily(&right)

	for _, bot := range []*core.Bot{&left, &right, &foreign} {
		g.Board.Bots[util.Idx(bot.Pos)] = bot
		g.Board.Set(bot.Pos, bot)
	}

	summary := summarizeMatch(g, 99, 7, 0)

	if summary.ColonyMemberBots != 2 {
		t.Fatalf("colony member bots = %d, want 2", summary.ColonyMemberBots)
	}
	if summary.FriendlyAdjacencies != 1 {
		t.Fatalf("friendly adjacencies = %d, want 1", summary.FriendlyAdjacencies)
	}
	if summary.ForeignAdjacencies != 1 {
		t.Fatalf("foreign adjacencies = %d, want 1", summary.ForeignAdjacencies)
	}
}

func TestSummarizeMatchReportsColonyComponentsTissueAndSoloWind(t *testing.T) {
	cfg := config.NewConfig()
	g := game.NewGame(&cfg)
	g.Board = core.NewBoard()

	colony := core.NewColony(util.NewPos(9, 10))
	for _, pos := range []core.Position{
		util.NewPos(10, 10),
		util.NewPos(10, 11),
		util.NewPos(10, 12),
	} {
		bot := core.NewBot(pos)
		bot.ConnnectedToColony = true
		colony.AddFamily(&bot)
		g.Board.AddBot(pos, &bot)
	}
	isolated := core.NewBot(util.NewPos(20, 20))
	colony.AddFamily(&isolated)
	g.Board.AddBot(isolated.Pos, &isolated)
	g.Board.DepositPheromone(util.NewPos(11, 10), core.PheromoneHome, 12, &colony)
	g.Board.DepositPheromone(util.NewPos(11, 11), core.PheromoneHome, 12, &colony)

	for i, dir := range []core.Direction{core.Right, core.Right, core.Up} {
		pos := util.NewPos(30, 30+i)
		bot := core.NewBot(pos)
		bot.Dir = dir
		g.Board.AddBot(pos, &bot)
	}

	summary := summarizeMatch(g, 99, 7, 0)

	if summary.MaxColonyComponent != 5 || summary.MaxConnectedComponent != 5 {
		t.Fatalf("component metrics = colony %d connected %d, want 5/5", summary.MaxColonyComponent, summary.MaxConnectedComponent)
	}
	if summary.LongestColonyRun != 3 || summary.LongestConnectedRun != 3 {
		t.Fatalf("run metrics = colony %d connected %d, want 3/3", summary.LongestColonyRun, summary.LongestConnectedRun)
	}
	if summary.ColonyTissueCells != 2 {
		t.Fatalf("colony tissue cells = %d, want 2", summary.ColonyTissueCells)
	}
	if summary.TopNonColonyDirectionShare != float64(2)/float64(3) {
		t.Fatalf("top non-colony direction share = %f, want 2/3", summary.TopNonColonyDirectionShare)
	}
}

func TestSummarizeMatchReportsPheromoneTotals(t *testing.T) {
	cfg := config.NewConfig()
	g := game.NewGame(&cfg)
	g.Board = core.NewBoard()

	g.Board.DepositPheromone(util.NewPos(10, 10), core.PheromoneFood, 12, nil)
	g.Board.DepositPheromone(util.NewPos(10, 11), core.PheromoneOre, 7, nil)
	g.Board.DepositPheromone(util.NewPos(10, 12), core.PheromoneHome, 5, nil)
	g.Board.DepositPheromone(util.NewPos(10, 13), core.PheromoneDanger, 3, nil)

	summary := summarizeMatch(g, 99, 7, 0)

	if summary.PheromoneActiveCells != 4 {
		t.Fatalf("active pheromone cells = %d, want 4", summary.PheromoneActiveCells)
	}
	if summary.TotalFoodPheromone != 12 ||
		summary.TotalOrePheromone != 7 ||
		summary.TotalHomePheromone != 5 ||
		summary.TotalDangerPheromone != 3 {
		t.Fatalf("pheromone totals = F%d O%d H%d D%d, want 12/7/5/3",
			summary.TotalFoodPheromone,
			summary.TotalOrePheromone,
			summary.TotalHomePheromone,
			summary.TotalDangerPheromone,
		)
	}
}

func BenchmarkSummarizeMatchTopN(b *testing.B) {
	g := newSummaryBenchmarkGame()

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		benchmarkSummary = summarizeMatch(g, 42, 100, 5)
	}
}

func addSummaryBot(g *game.Game, pos util.Position, hp, inventory int) int {
	bot := core.NewBot(pos)
	bot.Hp = hp
	bot.Inventory.Food = inventory / 2
	bot.Inventory.Ore = inventory - bot.Inventory.Food
	idx := util.Idx(pos)
	g.Board.Bots[idx] = &bot
	g.Board.Set(pos, &bot)
	return idx
}

func newSummaryBenchmarkGame() *game.Game {
	cfg := config.NewConfig()
	g := game.NewGame(&cfg)
	g.Board = core.NewBoard()

	for i := 0; i < util.Cells; i += 5 {
		pos := util.PosOf(i)
		if util.OutOfBounds(pos) {
			continue
		}
		addSummaryBot(g, pos, 100+i%500, i%50)
	}
	return g
}
