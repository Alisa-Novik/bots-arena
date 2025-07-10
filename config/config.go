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
	PoisonChance    int
	NewGenThreshold int
	ChildrenByBot   int
	DisableFarms    bool
	InitialGenome   *bot.Genome

	PhotoHpGain int
	PhotoChance int

	ControllerInitialAmount int
	ControllerHpGain        int
	ControllerGain          int

	SpawnerGrabCost int

	ResourceGrabHpGain int
	ResourceGrabGain   int

	BuildingGrabHpGain  int
	BuildingGrabGain    int
	BuildingBuildCost   int
	BuildingBuildHpGain int

	FoodGrabHpGain    int
	FarmGrabGain      int
	FarmBuildHpGain   int
	FarmBuildCost     int
	FarmInitialAmount int

	LogicStep time.Duration
	Pause     bool
}

func NewConfig(useGenome *bool) Config {
	return Config{
		HpThreshold:     90,
		MutationRate:    1,
		BotChance:       5,
		ResourceChance:  5,
		PoisonChance:    10,
		NewGenThreshold: 5,
		ChildrenByBot:   20,
		DisableFarms:    false,
		InitialGenome:   bot.GetInitialGenome(*useGenome),

		PhotoHpGain: 1,
		PhotoChance: 80,

		ControllerInitialAmount: 10,
		ControllerHpGain:        0,
		ControllerGain:          -100,

		SpawnerGrabCost: 20,

		ResourceGrabHpGain: 50,
		ResourceGrabGain:   5,

		BuildingGrabHpGain:  0,
		BuildingGrabGain:    1,
		BuildingBuildCost:   2,
		BuildingBuildHpGain: 5,

		FoodGrabHpGain:    250,
		FarmGrabGain:      -1,
		FarmBuildHpGain:   0,
		FarmBuildCost:     2,
		FarmInitialAmount: 0,

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
