package config

import (
	"golab/bot"
	"time"
)

type GameState struct {
	LastLogic time.Time
}

type Config struct {
	MutationRate    int
	BotChance       int
	ResourceChance  int
	NewGenThreshold int
	ChildrenByBot   int
	InitialGenome   *bot.Genome

	ControllerInitialAmount int
	HpFromController        int
	InventoryFromController int

	HpFromResource        int
	InventoryFromResource int

	HpFromBuilding        int
	InventoryFromBuilding int

	LogicStep time.Duration
	Pause     bool
}

func NewConfig(useGenome *bool) Config {
	return Config{
		MutationRate:    2,
		BotChance:       1,
		ResourceChance:  1,
		NewGenThreshold: 3,
		ChildrenByBot:   10,
		InitialGenome:   getInitialGenome(*useGenome),

		ControllerInitialAmount: 10,
		HpFromController:        10000,
		InventoryFromController: -100,

		HpFromResource:        300,
		InventoryFromResource: 1000,

		HpFromBuilding:        300,
		InventoryFromBuilding: 300,

		LogicStep: 10000000 * time.Nanosecond,
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
