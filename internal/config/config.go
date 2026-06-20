package config

import (
	"encoding/json"
	"os"
	"time"
)

type GameState struct {
	LastLogic           time.Time
	LogicTick           int
	LogicTicksPerSecond float64
	GameMaster          GameMasterState
}

type GameMasterState struct {
	Enabled      bool
	Name         string
	Tick         int
	Interval     int
	LastObserved int

	LiveBots  int
	Colonies  int
	Resources int
	Food      int
	Poison    int
	Water     int

	LastEventTick int
	LastEventKind string
	LastThought   string
	LastReason    string
	LastApplied   int
	LastCenterRow int
	LastCenterCol int
}

type ColoringStrategy int

const (
	ColonyConnectionColoring ColoringStrategy = iota
	DefaultColoring
	numColoringStrats
)

type Config struct {
	ColoringStrategy               ColoringStrategy `json:"coloring"`
	EnableResourceBasedColorChange bool             `json:"enableResourceBasedColorChange"`
	ShouldMutateColor              bool             `json:"shouldMutateColor"`

	HpThreshold          int     `json:"hpThreshold"`
	OceansCount          int     `json:"oceansCount"`
	ColorDelta           float32 `json:"colorDelta"`
	MutationRate         int     `json:"mutationRate"`
	BotChance            int     `json:"botChance"`
	ResourceChance       int     `json:"resourceChance"`
	PoisonChance         int     `json:"poisonChance"`
	NewGenThreshold      int     `json:"newGenThreshold"`
	ChildrenByBot        int     `json:"childrenByBot"`
	SmartEvolution       bool    `json:"smartEvolution"`
	EvolutionEliteCount  int     `json:"evolutionEliteCount"`
	EvolutionSeedPercent int     `json:"evolutionSeedPercent"`
	ImmigrationInterval  int     `json:"immigrationInterval"`
	ImmigrationBots      int     `json:"immigrationBots"`
	DivisionCost         int     `json:"divisionCost"`
	DivisionFoodCost     int     `json:"divisionFoodCost"`
	DivisionOreCost      int     `json:"divisionOreCost"`
	DivisionMinHp        int     `json:"divisionMinHp"`
	MaxBotAge            int     `json:"maxBotAge"`
	DisableFarms         bool    `json:"disableFarms"`
	UseInitialGenome     bool    `json:"useInitialGenome"`

	PhotoHpGain          int `json:"photoHpGain"`
	OrganicInitialAmount int `json:"organicInitialAmount"`

	ColonyPhotoFoodGain       int `json:"colonyPhotoFoodGain"`
	ColonyFarmChargeBonus     int `json:"colonyFarmChargeBonus"`
	ColonyFarmOutputBonus     int `json:"colonyFarmOutputBonus"`
	ColonyShareInventoryBonus int `json:"colonyShareInventoryBonus"`
	ColonyShareHpBonus        int `json:"colonyShareHpBonus"`
	ColonyAutoWallRadius      int `json:"colonyAutoWallRadius"`
	ColonyAutoWallHp          int `json:"colonyAutoWallHp"`
	ColonyInitialSpawners     int `json:"colonyInitialSpawners"`
	ColonySpawnerBirthPeriod  int `json:"colonySpawnerBirthPeriod"`
	ColonySpawnerLocalLimit   int `json:"colonySpawnerLocalLimit"`
	ColonyMaxActive           int `json:"colonyMaxActive"`
	ColonyHeartRadius         int `json:"colonyHeartRadius"`
	ColonyHeartImmortalRadius int `json:"colonyHeartImmortalRadius"`
	ColonyHeartMinHp          int `json:"colonyHeartMinHp"`
	ColonyHeartMaxHp          int `json:"colonyHeartMaxHp"`

	ControllerInitialAmount int `json:"controllerInitialAmount"`
	ControllerBuildCost     int `json:"controllerBuildCost"`
	ControllerHpGain        int `json:"controllerHpGain"`
	ControllerGrabHpGain    int `json:"controllerGrabHpGain"`
	ControllerGrabCost      int `json:"controllerGrabCost"`

	SpawnerBuildCost     int `json:"spawnerBuildCost"`
	SpawnerInitialAmount int `json:"spawnerInitialAmount"`
	SpawnerMaxAmount     int `json:"spawnerMaxAmount"`
	SpawnerAccessRadius  int `json:"spawnerAccessRadius"`
	SpawnerDivisionMinHp int `json:"spawnerDivisionMinHp"`
	SpawnerGrabCost      int `json:"spawnerGrabCost"`
	ResourceGrabGain     int `json:"resourceGrabGain"`
	ResourceGrabHpGain   int `json:"resourceGrabHpGain"`

	BuildingGrabHpGain  int `json:"buildingGrabHpGain"`
	BuildingGrabGain    int `json:"buildingGrabGain"`
	BuildingBuildCost   int `json:"buildingBuildCost"`
	BuildingBuildHpGain int `json:"buildingBuildHpGain"`

	FoodGrabHpGain          int `json:"foodGrabHpGain"`
	FarmGrabCost            int `json:"farmGrabCost"`
	FarmBuildHpGain         int `json:"farmBuildHpGain"`
	FarmBuildCost           int `json:"farmBuildCost"`
	FarmInitialAmount       int `json:"farmInitialAmount"`
	FertileFoodRegrowPeriod int `json:"fertileFoodRegrowPeriod"`

	MineBuildCost  int `json:"mineBuildCost"`
	MineGrabGain   int `json:"mineGrabGain"`
	MineGrabHpCost int `json:"mineGrabHpCost"`

	DepotBuildCost     int `json:"depotBuildCost"`
	DepotFoodCapacity  int `json:"depotFoodCapacity"`
	DepotOreCapacity   int `json:"depotOreCapacity"`
	DepotAccessRadius  int `json:"depotAccessRadius"`
	DepotRaidFoodLimit int `json:"depotRaidFoodLimit"`
	DepotRaidOreLimit  int `json:"depotRaidOreLimit"`

	PheromonesEnabled       bool `json:"pheromonesEnabled"`
	PheromoneDecayPeriod    int  `json:"pheromoneDecayPeriod"`
	PheromoneDecay          int  `json:"pheromoneDecay"`
	PheromoneDiffusePeriod  int  `json:"pheromoneDiffusePeriod"`
	PheromoneDiffuseAmount  int  `json:"pheromoneDiffuseAmount"`
	PheromoneEventDeposit   int  `json:"pheromoneEventDeposit"`
	PheromoneHomeDeposit    int  `json:"pheromoneHomeDeposit"`
	PheromoneBotDeposit     int  `json:"pheromoneBotDeposit"`
	PheromoneEmitHpCost     int  `json:"pheromoneEmitHpCost"`
	PheromoneSenseThreshold int  `json:"pheromoneSenseThreshold"`

	ColonyOrganismEnabled     bool `json:"colonyOrganismEnabled"`
	ColonyCohesionChance      int  `json:"colonyCohesionChance"`
	ColonyForageChance        int  `json:"colonyForageChance"`
	ColonyFrontierChance      int  `json:"colonyFrontierChance"`
	ColonyNestRadius          int  `json:"colonyNestRadius"`
	ColonyMemberHomeDeposit   int  `json:"colonyMemberHomeDeposit"`
	ColonyHomeFollowThreshold int  `json:"colonyHomeFollowThreshold"`
	ControllerCrowdThreshold  int  `json:"controllerCrowdThreshold"`

	LogicStep time.Duration `json:"logicStep"`
	Pause     bool          `json:"pause"`
	LiveBots  int           `json:"liveBots"`

	// UI
	RenderPaths        bool `json:"renderPaths"`
	RenderUnreachables bool `json:"renderUnreachables"`
	RenderTaskTargets  bool `json:"renderTaskTargets"`
}

func NewConfig() Config {
	return Config{
		ColoringStrategy:               DefaultColoring,
		ShouldMutateColor:              true,
		EnableResourceBasedColorChange: true,

		HpThreshold:          90,
		OceansCount:          15,
		ColorDelta:           float32(0.05),
		MutationRate:         1,
		BotChance:            5,
		ResourceChance:       5,
		PoisonChance:         3,
		NewGenThreshold:      50,
		ChildrenByBot:        20,
		SmartEvolution:       true,
		EvolutionEliteCount:  16,
		EvolutionSeedPercent: 35,
		ImmigrationInterval:  20,
		ImmigrationBots:      5,
		DivisionCost:         25,
		DivisionFoodCost:     1,
		DivisionOreCost:      1,
		DivisionMinHp:        120,
		MaxBotAge:            2500,
		DisableFarms:         false,
		UseInitialGenome:     false,

		PhotoHpGain:          2,
		OrganicInitialAmount: 3,

		ColonyPhotoFoodGain:       1,
		ColonyFarmChargeBonus:     1,
		ColonyFarmOutputBonus:     1,
		ColonyShareInventoryBonus: 1,
		ColonyShareHpBonus:        5,
		ColonyAutoWallRadius:      6,
		ColonyAutoWallHp:          35,
		ColonyInitialSpawners:     3,
		ColonySpawnerBirthPeriod:  16,
		ColonySpawnerLocalLimit:   3,
		ColonyMaxActive:           5,
		ColonyHeartRadius:         12,
		ColonyHeartImmortalRadius: 4,
		ColonyHeartMinHp:          180,
		ColonyHeartMaxHp:          500,

		ControllerInitialAmount: 1000,
		ControllerBuildCost:     5,
		ControllerHpGain:        1,
		ControllerGrabHpGain:    15,
		ControllerGrabCost:      1,

		SpawnerBuildCost:     2,
		SpawnerInitialAmount: 2,
		SpawnerMaxAmount:     6,
		SpawnerAccessRadius:  2,
		SpawnerDivisionMinHp: 80,
		SpawnerGrabCost:      2,
		ResourceGrabGain:     5,
		ResourceGrabHpGain:   150,

		BuildingGrabHpGain:  10,
		BuildingGrabGain:    1,
		BuildingBuildCost:   1,
		BuildingBuildHpGain: 15,

		FoodGrabHpGain:          250,
		FarmGrabCost:            1,
		FarmBuildHpGain:         100,
		FarmBuildCost:           1,
		FarmInitialAmount:       1,
		FertileFoodRegrowPeriod: 60000,

		MineBuildCost:  1,
		MineGrabGain:   30,
		MineGrabHpCost: 30,

		DepotBuildCost:     2,
		DepotFoodCapacity:  20,
		DepotOreCapacity:   20,
		DepotAccessRadius:  6,
		DepotRaidFoodLimit: 4,
		DepotRaidOreLimit:  4,

		PheromonesEnabled:       true,
		PheromoneDecayPeriod:    4,
		PheromoneDecay:          2,
		PheromoneDiffusePeriod:  8,
		PheromoneDiffuseAmount:  1,
		PheromoneEventDeposit:   48,
		PheromoneHomeDeposit:    24,
		PheromoneBotDeposit:     16,
		PheromoneEmitHpCost:     1,
		PheromoneSenseThreshold: 16,

		ColonyOrganismEnabled:     true,
		ColonyCohesionChance:      95,
		ColonyForageChance:        75,
		ColonyFrontierChance:      28,
		ColonyNestRadius:          14,
		ColonyMemberHomeDeposit:   24,
		ColonyHomeFollowThreshold: 8,
		ControllerCrowdThreshold:  64,

		LogicStep: 100000000 * time.Nanosecond * 3,
		Pause:     false,
		LiveBots:  0,

		RenderPaths:        true,
		RenderUnreachables: true,
		RenderTaskTargets:  true,
	}
}

func (c *Config) SlowDown() {
	c.LogicStep *= 2
}

func (c *Config) ToggleTaskTargets() {
	c.RenderTaskTargets = !c.RenderTaskTargets
}

func (c *Config) ToggleUnreachables() {
	c.RenderUnreachables = !c.RenderUnreachables
}

func (c *Config) TogglePaths() {
	c.RenderPaths = !c.RenderPaths
}

func (c *Config) SpeedUp() {
	if c.LogicStep <= time.Millisecond {
		c.LogicStep = time.Millisecond
		return
	}
	c.LogicStep /= 2
	if c.LogicStep < time.Millisecond {
		c.LogicStep = time.Millisecond
	}
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
	conf := NewConfig()
	json.NewDecoder(confJson).Decode(&conf)
	return conf
}
