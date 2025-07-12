package config

// TODO: get rid of bot dependency
import (
	"golab/bot"
	"time"
)

type GameState struct {
	LastLogic time.Time
}

type Config struct {
	HpThreshold     int
	ColorDelta      float32
	MutationRate    int
	BotChance       int
	ResourceChance  int
	PoisonChance    int
	NewGenThreshold int
	ChildrenByBot   int
	DisableFarms    bool
	InitialGenome   *bot.Genome

	PhotoHpGain          int
	OrganicInitialAmount int

	ControllerInitialAmount int
	ControllerHpGain        int
	ControllerGrabHpGain    int
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

	MineBuildCost  int
	MineGrabGain   int
	MineGrabHpCost int

	LogicStep time.Duration
	Pause     bool
	LiveBots  int
}

func NewConfig(useGenome *bool) Config {
	return Config{
		HpThreshold:     90,
		ColorDelta:      float32(0.05),
		MutationRate:    1,
		BotChance:       5,
		ResourceChance:  5,
		PoisonChance:    3,
		NewGenThreshold: 5,
		ChildrenByBot:   20,
		DisableFarms:    false,
		InitialGenome:   bot.GetInitialGenome(*useGenome),

		PhotoHpGain:          1,
		OrganicInitialAmount: 10,

		ControllerInitialAmount: 1000,
		ControllerHpGain:        1,
		ControllerGrabHpGain:    15,
		ControllerGain:          1,

		SpawnerGrabCost:    20,
		ResourceGrabGain:   5,
		ResourceGrabHpGain: 5,

		BuildingGrabHpGain:  5,
		BuildingGrabGain:    1,
		BuildingBuildCost:   1,
		BuildingBuildHpGain: 5,

		FoodGrabHpGain:    250,
		FarmGrabGain:      -1,
		FarmBuildHpGain:   1,
		FarmBuildCost:     2,
		FarmInitialAmount: 0,

		MineBuildCost:  1,
		MineGrabGain:   300,
		MineGrabHpCost: 10,

		LogicStep: 100000000 * time.Nanosecond * 3,
		Pause:     false,
		LiveBots:  0,
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
