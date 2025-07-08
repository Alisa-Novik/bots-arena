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

	FoodGrabHpGain  int
	FarmGrabGain    int
	FarmBuildHpGain int
	FarmBuildCost   int

	LogicStep time.Duration
	Pause     bool
}

func NewConfig(useGenome *bool) Config {
	return Config{
		HpThreshold:     90,
		MutationRate:    1,
		BotChance:       3,
		ResourceChance:  5,
		NewGenThreshold: 3,
		ChildrenByBot:   15,
		InitialGenome:   bot.GetInitialGenome(*useGenome),

		ControllerInitialAmount: 10,
		ControllerHpGain:        0,
		ControllerGain:          -100,

		SpawnerGrabCost: 20,

		ResourceGrabHpGain: 100,
		ResourceGrabGain:   20,

		BuildingGrabHpGain: 20,
		BuildingGrabCost:   3,
		BuildingBuildCost:  10,

		FoodGrabHpGain:  100,
		FarmGrabGain:    -1,
		FarmBuildHpGain: 20,
		FarmBuildCost:   -10,

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
