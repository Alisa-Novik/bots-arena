package config

import (
	"encoding/json"
	"os"
	"time"
)

type GameState struct {
	LastLogic time.Time
}

type Config struct {
	HpThreshold      int     `json:"hpThreshold"`
	ColorDelta       float32 `json:"colorDelta"`
	MutationRate     int     `json:"mutationRate"`
	BotChance        int     `json:"botChance"`
	ResourceChance   int     `json:"resourceChance"`
	PoisonChance     int     `json:"poisonChance"`
	NewGenThreshold  int     `json:"newGenThreshold"`
	ChildrenByBot    int     `json:"childrenByBot"`
	DivisionCost     int     `json:"divisionCost"`
	DisableFarms     bool    `json:"disableFarms"`
	UseInitialGenome bool    `json:"useInitialGenome"`

	PhotoHpGain          int `json:"photoHpGain"`
	OrganicInitialAmount int `json:"organicInitialAmount"`

	ControllerInitialAmount int `json:"controllerInitialAmount"`
	ControllerHpGain        int `json:"controllerHpGain"`
	ControllerGrabHpGain    int `json:"controllerGrabHpGain"`
	ControllerGrabCost      int `json:"controllerGrabCost"`

	SpawnerGrabCost    int `json:"spawnerGrabCost"`
	ResourceGrabGain   int `json:"resourceGrabGain"`
	ResourceGrabHpGain int `json:"resourceGrabHpGain"`

	BuildingGrabHpGain  int `json:"buildingGrabHpGain"`
	BuildingGrabGain    int `json:"buildingGrabGain"`
	BuildingBuildCost   int `json:"buildingBuildCost"`
	BuildingBuildHpGain int `json:"buildingBuildHpGain"`

	FoodGrabHpGain    int `json:"foodGrabHpGain"`
	FarmGrabCost      int `json:"farmGrabCost"`
	FarmBuildHpGain   int `json:"farmBuildHpGain"`
	FarmBuildCost     int `json:"farmBuildCost"`
	FarmInitialAmount int `json:"farmInitialAmount"`

	MineBuildCost  int `json:"mineBuildCost"`
	MineGrabGain   int `json:"mineGrabGain"`
	MineGrabHpCost int `json:"mineGrabHpCost"`

	LogicStep time.Duration `json:"logicStep"`
	Pause     bool          `json:"pause"`
	LiveBots  int           `json:"liveBots"`
}

func NewConfig() Config {
	return Config{
		HpThreshold:      90,
		ColorDelta:       float32(0.05),
		MutationRate:     1,
		BotChance:        5,
		ResourceChance:   5,
		PoisonChance:     3,
		NewGenThreshold:  5,
		ChildrenByBot:    20,
		DivisionCost:     10,
		DisableFarms:     false,
		UseInitialGenome: false,

		PhotoHpGain:          1,
		OrganicInitialAmount: 5,

		ControllerInitialAmount: 1000,
		ControllerHpGain:        1,
		ControllerGrabHpGain:    15,
		ControllerGrabCost:      1,

		SpawnerGrabCost:    20,
		ResourceGrabGain:   5,
		ResourceGrabHpGain: 150,

		BuildingGrabHpGain:  100,
		BuildingGrabGain:    1,
		BuildingBuildCost:   1,
		BuildingBuildHpGain: 25,

		FoodGrabHpGain:    250,
		FarmGrabCost:      -1,
		FarmBuildHpGain:   100,
		FarmBuildCost:     1,
		FarmInitialAmount: 0,

		MineBuildCost:  1,
		MineGrabGain:   30,
		MineGrabHpCost: 30,

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

func LoadFromJson(file string) Config {
	confJson, err := os.Open(file)
	if err != nil {
		panic(err)
	}
	defer confJson.Close()
	var conf Config
	json.NewDecoder(confJson).Decode(&conf)
	return conf
}
