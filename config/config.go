package config

import (
	"golab/bot"
	"time"
)

type GameState struct {
	LastLogic time.Time
}

type Config struct {
	HpThreshold     int
	MutationRate    int
	BotChance       int
	ResourceChance  int
	NewGenThreshold int
	ChildrenByBot   int
	InitialGenome   *bot.Genome

	ControllerInitialAmount int
	ControllerHpGain        int
	ControllerGain          int

	SpawnerGrabCost int

	ResourceGrabHpGain int
	ResourceGrabGain   int

	BuildingGrabHpGain int
	BuildingGrabCost   int
	BuildingBuildCost  int

	FoodGrabHpGain int
	FarmGrabGain   int
	FarmGrabHpGain int
	FarmBuildCost  int

	LogicStep time.Duration
	Pause     bool
}

func NewConfig(useGenome *bool) Config {
	return Config{
		HpThreshold:     90,
		MutationRate:    2,
		BotChance:       1,
		ResourceChance:  20,
		NewGenThreshold: 3,
		ChildrenByBot:   10,
		InitialGenome:   getInitialGenome(*useGenome),

		ControllerInitialAmount: 10,
		ControllerHpGain:        0,
		ControllerGain:          -100,

		SpawnerGrabCost: 100,

		ResourceGrabHpGain: 1,
		ResourceGrabGain:   10,

		BuildingGrabHpGain: 0,
		BuildingGrabCost:   4,
		BuildingBuildCost:  10,

		FoodGrabHpGain: 200,
		FarmGrabGain:   -1,
		FarmGrabHpGain: -10,
		FarmBuildCost:  -10,

		LogicStep: 100000000 * time.Nanosecond * 3,
		Pause:     false,
	}
}

func (c *Config) SlowDown() {
	c.LogicStep *= 2
}

func (c *Config) SpeedUp() {
	c.LogicStep /= 2
}

func (c *Config) Speed() int {
	return int(c.LogicStep)
}

func getInitialGenome(enabled bool) *bot.Genome {
	if !enabled {
		return nil
	}
	return bot.ReadGenome()
}
