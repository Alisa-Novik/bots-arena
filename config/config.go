package config

import (
	"golab/bot"
	"golab/util"
	"time"
)

type Config struct {
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

		LogicStep: 1000000 * time.Nanosecond,
		Pause:     false,
	}
}

func (c *Config) SlowDown() {
	c.LogicStep += 10 * time.Millisecond
}

func (c *Config) SpeedUp() {
	c.LogicStep -= 10 * time.Millisecond
}

func (c *Config) Speed() int {
	return int(c.LogicStep)
}

func getInitialGenome(enabled bool) *bot.Genome {
	if !enabled {
		return nil
	}
	return util.ReadGenome()
}
