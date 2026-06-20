package game

import (
	"golab/internal/config"
	"golab/internal/core"
	"golab/internal/util"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMockGameMasterComplainsAboutTooMuchSun(t *testing.T) {
	master := NewMockGameMaster()
	event, ok := master.Decide(MasterObservation{
		Tick:                      25,
		LiveBots:                  1200,
		Controllers:               1,
		MaxActiveColonyMembers:    3,
		MaxActiveConnectedMembers: 3,
		Resources:                 5000,
		Water:                     200,
	})
	if !ok {
		t.Fatalf("expected mock master event")
	}
	if event.Kind != "cooling_rain" {
		t.Fatalf("event kind = %q, want cooling_rain", event.Kind)
	}
	if !strings.Contains(event.Thought, "too much sun") {
		t.Fatalf("thought = %q, want too much sun complaint", event.Thought)
	}
}

func TestMockGameMasterLowPopulationDropsFoodNotOre(t *testing.T) {
	master := NewMockGameMaster()
	event, ok := master.Decide(MasterObservation{
		Tick:                      25,
		LiveBots:                  200,
		Controllers:               1,
		MaxActiveColonyMembers:    3,
		MaxActiveConnectedMembers: 3,
		Resources:                 2000,
		Food:                      40,
	})
	if !ok {
		t.Fatalf("expected mock master event")
	}
	if event.Kind != "food_rain" {
		t.Fatalf("event kind = %q, want food_rain", event.Kind)
	}
	if strings.Contains(event.Reason, "ore") {
		t.Fatalf("low-pop reason should not be ore-focused: %q", event.Reason)
	}
}

func TestMockGameMasterScarcePantryUsesForageInsteadOfOreSpam(t *testing.T) {
	master := NewMockGameMaster()
	event, ok := master.Decide(MasterObservation{
		Tick:                      25,
		LiveBots:                  1000,
		Controllers:               1,
		MaxActiveColonyMembers:    3,
		MaxActiveConnectedMembers: 3,
		Resources:                 100,
		Food:                      120,
		Water:                     2000,
	})
	if !ok {
		t.Fatalf("expected mock master event")
	}
	if event.Kind != "forage_rain" {
		t.Fatalf("event kind = %q, want forage_rain", event.Kind)
	}
}

func TestMockGameMasterOnlyAddsOreWhenOreIsActuallyScarce(t *testing.T) {
	master := NewMockGameMaster()
	event, ok := master.Decide(MasterObservation{
		Tick:                      25,
		LiveBots:                  1000,
		Controllers:               1,
		MaxActiveColonyMembers:    3,
		MaxActiveConnectedMembers: 3,
		Resources:                 40,
		Food:                      500,
		Water:                     2000,
		TotalFood:                 200,
		TotalOre:                  10,
	})
	if !ok {
		t.Fatalf("expected mock master event")
	}
	if event.Kind != "resource_rain" {
		t.Fatalf("event kind = %q, want resource_rain for true ore scarcity", event.Kind)
	}
	if event.Amount >= 100 {
		t.Fatalf("ore event amount = %d, want small top-up", event.Amount)
	}
}

func TestMockGameMasterSupportsSoloActiveColonies(t *testing.T) {
	master := NewMockGameMaster()
	event, ok := master.Decide(MasterObservation{
		Tick:                   25,
		LiveBots:               1000,
		Controllers:            1,
		MaxActiveColonyMembers: 1,
		Resources:              2000,
		Food:                   2000,
		Water:                  2000,
	})
	if !ok {
		t.Fatalf("expected mock master event")
	}
	if event.Kind != "colony_support" {
		t.Fatalf("event kind = %q, want colony_support", event.Kind)
	}
}

func TestMockGameMasterSupportsControllerlessSettlements(t *testing.T) {
	master := NewMockGameMaster()
	event, ok := master.Decide(MasterObservation{
		Tick:        25,
		LiveBots:    120,
		Controllers: 0,
		Resources:   2000,
		Food:        2000,
		Water:       2000,
	})
	if !ok {
		t.Fatalf("expected mock master event")
	}
	if event.Kind != "settlement_support" {
		t.Fatalf("event kind = %q, want settlement_support", event.Kind)
	}
}

func TestApplyMasterEventResourceRainAddsResources(t *testing.T) {
	cfg := config.NewConfig()
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	event := g.ApplyMasterEvent(MasterEvent{
		Kind:   "resource_rain",
		Center: MasterPosition{Row: 10, Col: 10},
		Radius: 3,
		Amount: 20,
	})
	if event.Applied == 0 {
		t.Fatalf("applied = 0, want resources placed")
	}
	if obs := g.ObserveMaster(1); obs.Resources == 0 {
		t.Fatalf("resources = 0 after resource rain")
	}
}

func TestApplyMasterEventFoodRainAddsFood(t *testing.T) {
	cfg := config.NewConfig()
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	event := g.ApplyMasterEvent(MasterEvent{
		Kind:   "food_rain",
		Center: MasterPosition{Row: 10, Col: 10},
		Radius: 3,
		Amount: 20,
	})
	if event.Applied == 0 {
		t.Fatalf("applied = 0, want food placed")
	}
	obs := g.ObserveMaster(1)
	if obs.Food == 0 {
		t.Fatalf("food = 0 after food rain")
	}
	if obs.Resources != 0 {
		t.Fatalf("resources = %d after food rain, want no ore", obs.Resources)
	}
}

func TestApplyMasterEventForageRainIsFoodBiased(t *testing.T) {
	cfg := config.NewConfig()
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	event := g.ApplyMasterEvent(MasterEvent{
		Kind:   "forage_rain",
		Tick:   7,
		Center: MasterPosition{Row: 20, Col: 20},
		Radius: 8,
		Amount: 80,
	})
	if event.Applied == 0 {
		t.Fatalf("applied = 0, want forage placed")
	}
	obs := g.ObserveMaster(1)
	if obs.Food <= obs.Resources {
		t.Fatalf("forage rain food/resources = %d/%d, want food-biased", obs.Food, obs.Resources)
	}
}

func TestApplyMasterEventColonySupportBoostsColonyBank(t *testing.T) {
	cfg := config.NewConfig()
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	ctrlPos := util.NewPos(20, 20)
	ownerPos := ctrlPos.AddDir(core.Right)
	owner := core.NewBot(ownerPos)
	owner.ConnnectedToColony = true
	colony := core.NewColony(ctrlPos)
	colony.AddFamily(&owner)
	g.Board.Bots[util.Idx(ownerPos)] = &owner
	g.Board.Set(ownerPos, &owner)
	g.Board.Set(ctrlPos, core.Controller{
		Pos:    ctrlPos,
		Owner:  &owner,
		Colony: &colony,
		Amount: 1,
	})

	event := g.ApplyMasterEvent(MasterEvent{
		Kind:   "colony_support",
		Center: MasterPosition{Row: 20, Col: 20},
		Radius: 4,
		Amount: 20,
	})
	if event.Applied == 0 {
		t.Fatalf("applied = 0, want colony support")
	}
	if colony.FoodBank == 0 || colony.OreBank == 0 {
		t.Fatalf("colony bank after support = F%d O%d, want food and small ore", colony.FoodBank, colony.OreBank)
	}
	ctrl, ok := g.Board.At(ctrlPos).(core.Controller)
	if !ok {
		t.Fatalf("controller after support = %T, want Controller", g.Board.At(ctrlPos))
	}
	if ctrl.Amount <= 1 {
		t.Fatalf("controller amount after support = %d, want boosted", ctrl.Amount)
	}
}

func TestApplyMasterEventSettlementSupportIsFoodBiasedAroundLiveBots(t *testing.T) {
	cfg := config.NewConfig()
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	botPos := util.NewPos(20, 20)
	bot := core.NewBot(botPos)
	kin := core.NewBot(botPos.AddDir(core.Right))
	kin.Genome = bot.Genome
	g.Board.Bots[util.Idx(botPos)] = &bot
	g.Board.Bots[util.Idx(kin.Pos)] = &kin
	g.Board.Set(botPos, &bot)
	g.Board.Set(kin.Pos, &kin)

	event := g.ApplyMasterEvent(MasterEvent{
		Kind:   "settlement_support",
		Tick:   11,
		Center: MasterPosition{Row: 1, Col: 1},
		Radius: 6,
		Amount: 60,
	})
	if event.Applied == 0 {
		t.Fatalf("applied = 0, want settlement support")
	}
	obs := g.ObserveMaster(1)
	if obs.Controllers != 1 {
		t.Fatalf("controllers after settlement support = %d, want 1", obs.Controllers)
	}
	if obs.Food <= obs.Resources {
		t.Fatalf("settlement support food/resources = %d/%d, want food-biased", obs.Food, obs.Resources)
	}
	if got := g.Board.GetBot(botPos); got != &bot {
		t.Fatalf("bot was overwritten by settlement support")
	}
}

func TestApplyMasterEventFamineWindSkippedUntilStableColony(t *testing.T) {
	cfg := config.NewConfig()
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	ctrlPos := util.NewPos(20, 20)
	ownerPos := ctrlPos.AddDir(core.Right)
	owner := core.NewBot(ownerPos)
	owner.ConnnectedToColony = true
	colony := core.NewColony(ctrlPos)
	colony.AddFamily(&owner)
	resourcePos := util.NewPos(10, 10)

	g.Board.Bots[util.Idx(ownerPos)] = &owner
	g.Board.Set(ownerPos, &owner)
	g.Board.Set(ctrlPos, core.Controller{Pos: ctrlPos, Owner: &owner, Colony: &colony, Amount: 10})
	g.Board.Set(resourcePos, core.Resource{Pos: resourcePos, Amount: 1})

	event := g.ApplyMasterEvent(MasterEvent{
		Kind:   "famine_wind",
		Center: MasterPosition{Row: resourcePos.R, Col: resourcePos.C},
		Radius: 0,
		Amount: 1,
	})

	if event.Applied != 0 {
		t.Fatalf("famine wind applied = %d, want skipped", event.Applied)
	}
	if _, ok := g.Board.At(resourcePos).(core.Resource); !ok {
		t.Fatalf("resource was removed by famine wind despite fragile colony")
	}
}

func TestApplyMasterEventDoesNotOverwriteBots(t *testing.T) {
	cfg := config.NewConfig()
	g := NewGame(&cfg)
	g.Board = core.NewBoard()

	pos := util.NewPos(10, 10)
	bot := core.NewBot(pos)
	g.Board.Bots[util.Idx(pos)] = &bot
	g.Board.Set(pos, &bot)

	event := g.ApplyMasterEvent(MasterEvent{
		Kind:   "resource_rain",
		Center: MasterPosition{Row: pos.R, Col: pos.C},
		Radius: 0,
		Amount: 1,
	})
	if event.Applied != 0 {
		t.Fatalf("applied = %d, want 0 over occupied bot cell", event.Applied)
	}
	if got := g.Board.GetBot(pos); got != &bot {
		t.Fatalf("bot was overwritten: got %p want %p", got, &bot)
	}
	if _, ok := g.Board.At(pos).(*core.Bot); !ok {
		t.Fatalf("grid occupant = %T, want *core.Bot", g.Board.At(pos))
	}
}

func TestEnabledGameMasterUpdatesSharedState(t *testing.T) {
	cfg := config.NewConfig()
	g := NewGame(&cfg)
	g.Board = core.NewBoard()
	g.EnableGameMaster(1)
	g.logicTick = 1

	g.runGameMasterTick()

	state := g.State.GameMaster
	if !state.Enabled {
		t.Fatalf("game master state is not enabled")
	}
	if state.Name != mockGameMasterName {
		t.Fatalf("game master name = %q, want %q", state.Name, mockGameMasterName)
	}
	if state.LastEventKind != "spark_bots" {
		t.Fatalf("last event = %q, want spark_bots", state.LastEventKind)
	}
	if state.LastApplied == 0 {
		t.Fatalf("last applied = 0, want spawned bots")
	}
}

func TestExternalGameMasterUsesCommand(t *testing.T) {
	script := filepath.Join(t.TempDir(), "coolio-gm")
	if err := os.WriteFile(script, []byte(`#!/bin/sh
cat >/dev/null
printf '%s\n' '{"kind":"cooling_rain","thought":"There is too much sun happening.","reason":"test external","center":{"row":10,"col":11},"radius":3,"amount":4}'
`), 0o755); err != nil {
		t.Fatal(err)
	}

	master := NewExternalGameMaster([]string{script}, time.Second, nil)
	event, ok := master.Decide(MasterObservation{Tick: 42, LiveBots: 1000})
	if !ok {
		t.Fatalf("expected external event")
	}
	if event.Kind != "cooling_rain" {
		t.Fatalf("event kind = %q, want cooling_rain", event.Kind)
	}
	if event.Tick != 42 {
		t.Fatalf("event tick = %d, want observation tick fallback", event.Tick)
	}
	if event.Center.Row != 10 || event.Center.Col != 11 {
		t.Fatalf("center = %+v, want row=10 col=11", event.Center)
	}
}

func TestExternalGameMasterFallsBackToMock(t *testing.T) {
	master := NewExternalGameMaster([]string{"/missing/coolio-gm"}, time.Millisecond, NewMockGameMaster())
	event, ok := master.Decide(MasterObservation{Tick: 12, LiveBots: 0})
	if !ok {
		t.Fatalf("expected fallback event")
	}
	if event.Kind != "spark_bots" {
		t.Fatalf("event kind = %q, want spark_bots", event.Kind)
	}
	if !strings.Contains(event.Reason, "external game master failed") {
		t.Fatalf("fallback reason = %q, want external failure context", event.Reason)
	}
}
