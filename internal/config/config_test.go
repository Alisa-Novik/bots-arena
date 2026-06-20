package config

import (
	"testing"
	"time"
)

func TestSpeedUpDoesNotCollapseLogicStepToZero(t *testing.T) {
	cfg := NewConfig()
	cfg.LogicStep = time.Nanosecond

	cfg.SpeedUp()

	if cfg.LogicStep != time.Millisecond {
		t.Fatalf("logic step after speed-up = %s, want %s", cfg.LogicStep, time.Millisecond)
	}

	cfg.SpeedUp()

	if cfg.LogicStep != time.Millisecond {
		t.Fatalf("logic step after second speed-up = %s, want %s", cfg.LogicStep, time.Millisecond)
	}
}

func TestPheromoneDefaults(t *testing.T) {
	cfg := NewConfig()

	if !cfg.PheromonesEnabled {
		t.Fatalf("pheromones should be enabled by default")
	}
	if cfg.PheromoneDecayPeriod != 4 || cfg.PheromoneDecay != 2 {
		t.Fatalf("decay defaults = period %d amount %d, want 4/2", cfg.PheromoneDecayPeriod, cfg.PheromoneDecay)
	}
	if cfg.PheromoneDiffusePeriod != 8 || cfg.PheromoneDiffuseAmount != 1 {
		t.Fatalf("diffuse defaults = period %d amount %d, want 8/1", cfg.PheromoneDiffusePeriod, cfg.PheromoneDiffuseAmount)
	}
	if cfg.PheromoneEventDeposit != 48 ||
		cfg.PheromoneHomeDeposit != 24 ||
		cfg.PheromoneBotDeposit != 16 ||
		cfg.PheromoneEmitHpCost != 1 ||
		cfg.PheromoneSenseThreshold != 16 {
		t.Fatalf("unexpected pheromone defaults: %+v", cfg)
	}
}

func TestDepotDefaults(t *testing.T) {
	cfg := NewConfig()

	if cfg.DepotBuildCost != 2 ||
		cfg.DepotFoodCapacity != 20 ||
		cfg.DepotOreCapacity != 20 ||
		cfg.DepotAccessRadius != 6 ||
		cfg.DepotRaidFoodLimit != 4 ||
		cfg.DepotRaidOreLimit != 4 {
		t.Fatalf("unexpected depot defaults: %+v", cfg)
	}
}

func TestSpawnerDefaults(t *testing.T) {
	cfg := NewConfig()

	if cfg.SpawnerBuildCost != 2 ||
		cfg.SpawnerInitialAmount != 2 ||
		cfg.SpawnerMaxAmount != 6 ||
		cfg.SpawnerAccessRadius != 2 ||
		cfg.SpawnerDivisionMinHp != 80 ||
		cfg.SpawnerGrabCost != 2 {
		t.Fatalf("unexpected spawner defaults: %+v", cfg)
	}
}

func TestColonyEconomyDefaults(t *testing.T) {
	cfg := NewConfig()

	if cfg.PhotoHpGain != 2 ||
		cfg.ColonyPhotoFoodGain != 1 ||
		cfg.ColonyFarmChargeBonus != 1 ||
		cfg.ColonyFarmOutputBonus != 1 ||
		cfg.ColonyShareInventoryBonus != 1 ||
		cfg.ColonyShareHpBonus != 5 ||
		cfg.ColonyAutoWallRadius != 6 ||
		cfg.ColonyAutoWallHp != 35 ||
		cfg.ColonyInitialSpawners != 3 ||
		cfg.ColonySpawnerBirthPeriod != 16 ||
		cfg.ColonySpawnerLocalLimit != 3 ||
		cfg.ColonyMaxActive != 5 ||
		cfg.ColonyHeartRadius != 12 ||
		cfg.ColonyHeartImmortalRadius != 4 ||
		cfg.ColonyHeartMinHp != 180 ||
		cfg.ColonyHeartMaxHp != 500 {
		t.Fatalf("unexpected colony economy defaults: %+v", cfg)
	}
}

func TestColonyOrganismDefaults(t *testing.T) {
	cfg := NewConfig()

	if !cfg.ColonyOrganismEnabled ||
		cfg.ColonyCohesionChance != 95 ||
		cfg.ColonyNestRadius != 14 ||
		cfg.ColonyMemberHomeDeposit != 24 ||
		cfg.ColonyHomeFollowThreshold != 8 ||
		cfg.ControllerCrowdThreshold != 64 {
		t.Fatalf("unexpected colony organism defaults: %+v", cfg)
	}
}
