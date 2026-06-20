package game

import (
	"encoding/json"
	"fmt"
	"golab/internal/core"
	"golab/internal/ui"
	"golab/internal/util"
	"os"
	"path/filepath"
	"time"
)

const savesRoot = "data/saves"

type savePosition struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

type genomeSaveFile struct {
	Version   int               `json:"version"`
	Kind      string            `json:"kind"`
	CreatedAt string            `json:"created_at"`
	Tick      int               `json:"tick"`
	Source    string            `json:"source"`
	Bot       genomeBotSave     `json:"bot"`
	Genome    core.Genome       `json:"genome"`
	Config    genomeConfigSave  `json:"config"`
	Colony    *genomeColonySave `json:"colony,omitempty"`
}

type genomeBotSave struct {
	Position       savePosition `json:"position"`
	HP             int          `json:"hp"`
	Inventory      int          `json:"inventory"`
	FoodInventory  int          `json:"food_inventory"`
	OreInventory   int          `json:"ore_inventory"`
	OffspringCount int          `json:"offspring_count"`
	Color          [3]float32   `json:"color"`
}

type genomeColonySave struct {
	Center   savePosition `json:"center"`
	Members  int          `json:"members"`
	FoodBank int          `json:"food_bank"`
	OreBank  int          `json:"ore_bank"`
}

type genomeConfigSave struct {
	UseInitialGenome  bool `json:"use_initial_genome"`
	ShouldMutateColor bool `json:"should_mutate_color"`
	MutationRate      int  `json:"mutation_rate"`
}

type mapSaveFile struct {
	Version     int            `json:"version"`
	Kind        string         `json:"kind"`
	CreatedAt   string         `json:"created_at"`
	Tick        int            `json:"tick"`
	Rows        int            `json:"rows"`
	Cols        int            `json:"cols"`
	Cells       []mapCellSave  `json:"cells"`
	Frozen      []savePosition `json:"frozen"`
	Biomes      []mapBiomeSave `json:"biomes,omitempty"`
	Counts      map[string]int `json:"counts"`
	BiomeCounts map[string]int `json:"biome_counts"`
	Config      mapConfigSave  `json:"config"`
}

type mapConfigSave struct {
	OceansCount    int `json:"oceans_count"`
	ResourceChance int `json:"resource_chance"`
	PoisonChance   int `json:"poison_chance"`
}

type mapCellSave struct {
	Position    savePosition `json:"position"`
	Kind        string       `json:"kind"`
	Amount      int          `json:"amount,omitempty"`
	Food        int          `json:"food,omitempty"`
	Ore         int          `json:"ore,omitempty"`
	GroupID     int          `json:"group_id,omitempty"`
	HP          int          `json:"hp,omitempty"`
	WaterAmount int          `json:"water_amount,omitempty"`
	HasOwner    bool         `json:"has_owner,omitempty"`
	HasColony   bool         `json:"has_colony,omitempty"`
	AutoBirth   bool         `json:"auto_birth,omitempty"`
}

type mapBiomeSave struct {
	Position savePosition `json:"position"`
	Kind     string       `json:"kind"`
}

func (g *Game) SaveGenome(pos core.Position) ui.GodReport {
	path, source, err := g.saveGenomeToDir(filepath.Join(savesRoot, "genomes"), pos)
	if err != nil {
		return ui.GodReport{Message: fmt.Sprintf("Genome save failed: %v", err)}
	}
	return ui.GodReport{
		Message: fmt.Sprintf("Saved genome: %s", filepath.Base(path)),
		Lines:   []string{source, path},
	}
}

func (g *Game) SaveMap() ui.GodReport {
	path, cells, frozen, biomes, err := g.saveMapToDir(filepath.Join(savesRoot, "maps"))
	if err != nil {
		return ui.GodReport{Message: fmt.Sprintf("Map save failed: %v", err)}
	}
	return ui.GodReport{
		Message: fmt.Sprintf("Saved map: %s", filepath.Base(path)),
		Lines:   []string{fmt.Sprintf("%d cells, %d frozen, %d biomes", cells, frozen, biomes), path},
	}
}

func (g *Game) saveGenomeToDir(dir string, pos core.Position) (string, string, error) {
	bot, source := g.genomeSaveCandidate(pos)
	if bot == nil {
		return "", "", fmt.Errorf("no live bot genome available")
	}

	save := genomeSaveFile{
		Version:   1,
		Kind:      "golab_genome",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Tick:      g.logicTick,
		Source:    source,
		Bot: genomeBotSave{
			Position:       savePos(bot.Pos),
			HP:             bot.Hp,
			Inventory:      bot.Inventory.Total(),
			FoodInventory:  bot.Inventory.Food,
			OreInventory:   bot.Inventory.Ore,
			OffspringCount: bot.CountOffsprings() - 1,
			Color:          bot.Color,
		},
		Genome: bot.Genome,
		Config: genomeConfigSave{
			UseInitialGenome:  g.config.UseInitialGenome,
			ShouldMutateColor: g.config.ShouldMutateColor,
			MutationRate:      g.config.MutationRate,
		},
	}
	if bot.Colony != nil {
		save.Colony = &genomeColonySave{
			Center:   savePos(bot.Colony.Center),
			Members:  g.liveMembers(bot.Colony),
			FoodBank: bot.Colony.FoodBank,
			OreBank:  bot.Colony.OreBank,
		}
	}

	path := filepath.Join(dir, fmt.Sprintf("genome-%s-t%d.json", saveTimestamp(), g.logicTick))
	return path, source, writeJSON(path, save)
}

func (g *Game) genomeSaveCandidate(pos core.Position) (*core.Bot, string) {
	if g.selectedColony != nil {
		if bot := g.bestColonyBot(g.selectedColony); bot != nil {
			return bot, "selected colony champion"
		}
	}
	if core.Inside(pos) {
		if bot := g.Board.GetBot(pos); bot != nil {
			return bot, "hovered bot"
		}
	}
	if bot := g.bestLiveBot(); bot != nil {
		return bot, "top live bot"
	}
	return nil, ""
}

func (g *Game) bestColonyBot(colony *core.Colony) *core.Bot {
	var best *core.Bot
	for _, id := range g.Board.ActiveBotIDs() {
		bot := g.Board.BotByID(id)
		if bot != nil && bot.Colony == colony && betterGenomeCandidate(bot, best) {
			best = bot
		}
	}
	return best
}

func (g *Game) bestLiveBot() *core.Bot {
	var best *core.Bot
	for _, id := range g.Board.ActiveBotIDs() {
		bot := g.Board.BotByID(id)
		if bot != nil && betterGenomeCandidate(bot, best) {
			best = bot
		}
	}
	return best
}

func betterGenomeCandidate(candidate, current *core.Bot) bool {
	if current == nil {
		return true
	}
	if candidate.Hp != current.Hp {
		return candidate.Hp > current.Hp
	}
	if candidate.Inventory.Total() != current.Inventory.Total() {
		return candidate.Inventory.Total() > current.Inventory.Total()
	}
	if candidate.CountOffsprings() != current.CountOffsprings() {
		return candidate.CountOffsprings() > current.CountOffsprings()
	}
	return util.Idx(candidate.Pos) < util.Idx(current.Pos)
}

func (g *Game) saveMapToDir(dir string) (string, int, int, int, error) {
	save := mapSaveFile{
		Version:     1,
		Kind:        "golab_map",
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
		Tick:        g.logicTick,
		Rows:        core.Rows,
		Cols:        core.Cols,
		Counts:      map[string]int{},
		BiomeCounts: map[string]int{},
		Config: mapConfigSave{
			OceansCount:    g.config.OceansCount,
			ResourceChance: g.config.ResourceChance,
			PoisonChance:   g.config.PoisonChance,
		},
	}

	for idx, cell := range *g.Board.GetGrid() {
		pos := util.PosOf(idx)
		biome := g.Board.BiomeAtIdx(idx)
		save.BiomeCounts[biome.String()]++
		if biome != core.BiomeNeutral {
			save.Biomes = append(save.Biomes, mapBiomeSave{
				Position: savePos(pos),
				Kind:     biome.String(),
			})
		}
		if g.Board.IsFrozenIdx(idx) {
			save.Frozen = append(save.Frozen, savePos(pos))
		}
		cellSave, ok := mapCellFromOccupant(pos, cell)
		if !ok {
			continue
		}
		save.Cells = append(save.Cells, cellSave)
		save.Counts[cellSave.Kind]++
	}
	if len(save.Frozen) > 0 {
		save.Counts["frozen"] = len(save.Frozen)
	}

	path := filepath.Join(dir, fmt.Sprintf("map-%s-t%d.json", saveTimestamp(), g.logicTick))
	err := writeJSON(path, save)
	return path, len(save.Cells), len(save.Frozen), len(save.Biomes), err
}

func mapCellFromOccupant(pos core.Position, cell core.Occupant) (mapCellSave, bool) {
	out := mapCellSave{Position: savePos(pos)}
	switch v := cell.(type) {
	case nil, *core.Bot:
		return mapCellSave{}, false
	case core.Wall:
		out.Kind = "wall"
	case core.Building:
		out.Kind = "building"
		out.HP = v.Hp
		out.HasOwner = v.Owner != nil
	case core.Resource:
		out.Kind = "resource"
		out.Amount = v.Amount
	case core.Food:
		out.Kind = "food"
		out.Amount = v.Amount
	case core.Organics:
		out.Kind = "organics"
		out.Amount = v.Amount
	case core.Poison:
		out.Kind = "poison"
	case core.Water:
		out.Kind = "water"
		out.GroupID = v.GroupId
		out.Amount = v.Amount
	case core.Farm:
		out.Kind = "farm"
		out.Amount = v.Amount
		out.HasOwner = v.Owner != nil
	case core.Spawner:
		out.Kind = "spawner"
		out.Amount = v.Amount
		out.HasOwner = v.Owner != nil
		out.HasColony = v.Colony != nil
		out.AutoBirth = v.AutoBirth
	case core.Mine:
		out.Kind = "mine"
		out.Amount = v.Amount
		out.HasOwner = v.Owner != nil
	case core.Depot:
		out.Kind = "depot"
		out.Food = v.Food
		out.Ore = v.Ore
		out.HasOwner = v.Owner != nil
		out.HasColony = v.Colony != nil
	case *core.Depot:
		if v == nil {
			return mapCellSave{}, false
		}
		out.Kind = "depot"
		out.Food = v.Food
		out.Ore = v.Ore
		out.HasOwner = v.Owner != nil
		out.HasColony = v.Colony != nil
	case core.Controller:
		out.Kind = "controller"
		out.Amount = v.Amount
		out.WaterAmount = v.WaterAmount
		out.HasOwner = v.Owner != nil
		out.HasColony = v.Colony != nil
	case *core.Controller:
		out.Kind = "controller"
		out.Amount = v.Amount
		out.WaterAmount = v.WaterAmount
		out.HasOwner = v.Owner != nil
		out.HasColony = v.Colony != nil
	case core.ColonyFlag:
		out.Kind = "colony_flag"
	case *core.ColonyFlag:
		out.Kind = "colony_flag"
	default:
		out.Kind = fmt.Sprintf("%T", cell)
	}
	return out, true
}

func (g *Game) liveMembers(colony *core.Colony) int {
	count := 0
	for _, id := range g.Board.ActiveBotIDs() {
		bot := g.Board.BotByID(id)
		if bot != nil && bot.Colony == colony {
			count++
		}
	}
	return count
}

func savePos(pos core.Position) savePosition {
	return savePosition{Row: pos.R, Col: pos.C}
}

func saveTimestamp() string {
	return time.Now().UTC().Format("20060102-150405.000000000")
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
}
