package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"golab/internal/config"
	"golab/internal/core"
	"golab/internal/game"
	"golab/internal/render"

	expRand "golang.org/x/exp/rand"
)

const (
	defaultLeaderboardMatches = 3
	defaultReplayTicks        = 120
	defaultReplaySampleEvery  = 5
	defaultSmartnessEvalTicks = 2000
	defaultStatusTicks        = 20
	defaultTopBots            = 5
	defaultMatchTicks         = 300
	defaultScaleTargetBots    = 100000
	defaultScaleTicks         = 300
	defaultScaleWarmupTicks   = 20
)

func runCommand(args []string) bool {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return false
	}

	switch args[0] {
	case "status":
		runStatus(args[1:])
		return true
	case "match":
		runMatch(args[1:])
		return true
	case "leaderboard":
		runLeaderboard(args[1:])
		return true
	case "replay":
		runReplay(args[1:])
		return true
	case "gamemaster", "game-master", "gm":
		runGameMaster(args[1:])
		return true
	case "run", "seed-roulette":
		runSeedRoulette(args[1:])
		return true
	case "render":
		runRender(args[1:])
		return true
	case "scale-test":
		runScaleTest(args[1:])
		return true
	case "smartness-eval":
		runSmartnessEval(args[1:])
		return true
	default:
		return false
	}
}

func runStatus(args []string) {
	flags := commandFlagSet("status")
	seed := flags.Int64("seed", 1, "Deterministic PRNG seed.")
	ticks := flags.Int("ticks", defaultStatusTicks, "Simulation ticks to execute.")
	topBots := flags.Int("top-bots", defaultTopBots, "Number of top bots to include in output.")
	pretty := flags.Bool("pretty", false, "Pretty-print JSON output.")
	usage := "status [--seed N] [--ticks N] [--top-bots N] [--pretty]"
	if err := parseCommandFlags(flags, args, usage); err != nil {
		os.Exit(2)
	}

	tickCount := normalizeNonNegativeInt(*ticks)
	topBotsCount := normalizeNonNegativeInt(*topBots)
	summary := runMatchSummary(*seed, tickCount, topBotsCount)
	payload := map[string]any{
		"command": "status",
		"summary": summary,
	}
	printJSON(payload, *pretty)
}

func runMatch(args []string) {
	flags := commandFlagSet("match")
	seed := flags.Int64("seed", 1, "Deterministic PRNG seed.")
	ticks := flags.Int("ticks", defaultMatchTicks, "Simulation ticks to execute.")
	topBots := flags.Int("top-bots", defaultTopBots, "Number of top bots to include in output.")
	pretty := flags.Bool("pretty", false, "Pretty-print JSON output.")
	usage := "match [--seed N] [--ticks N] [--top-bots N] [--pretty]"
	if err := parseCommandFlags(flags, args, usage); err != nil {
		os.Exit(2)
	}

	tickCount := normalizeNonNegativeInt(*ticks)
	topBotsCount := normalizeNonNegativeInt(*topBots)
	summary := runMatchSummary(*seed, tickCount, topBotsCount)
	winner := winningBot(summary.TopBots)
	payload := map[string]any{
		"command":   "match",
		"match_id":  fmt.Sprintf("match-%d", *seed),
		"seed":      *seed,
		"ticks":     tickCount,
		"summary":   summary,
		"winner":    winner,
		"winner_hp": winnerValue(winner),
	}
	printJSON(payload, *pretty)
}

func runLeaderboard(args []string) {
	flags := commandFlagSet("leaderboard")
	baseSeed := flags.Int64("seed", 1, "Deterministic PRNG seed for the first match.")
	matches := flags.Int("matches", defaultLeaderboardMatches, "Number of matches to run.")
	seedStep := flags.Int64("seed-step", 1, "Seed increment between matches.")
	ticks := flags.Int("ticks", defaultMatchTicks, "Simulation ticks per match.")
	topBots := flags.Int("top-bots", defaultTopBots, "Number of top bots to include in each match summary.")
	pretty := flags.Bool("pretty", false, "Pretty-print JSON output.")
	usage := "leaderboard [--seed N] [--matches M] [--seed-step S] [--ticks T] [--top-bots N] [--pretty]"
	if err := parseCommandFlags(flags, args, usage); err != nil {
		os.Exit(2)
	}

	matchCount := normalizePositiveInt(*matches)
	tickCount := normalizeNonNegativeInt(*ticks)
	seedDelta := *seedStep
	if seedDelta == 0 {
		seedDelta = 1
	}

	type leaderboardEntry struct {
		MatchID       string       `json:"match_id"`
		Seed          int64        `json:"seed"`
		Summary       matchSummary `json:"summary"`
		Winner        *botSummary  `json:"winner"`
		WinnerScore   int          `json:"winner_score"`
		LiveBotCount  int          `json:"live_bot_count"`
		ColonyCount   int          `json:"colony_count"`
		HasReplayHint bool         `json:"replay_hint"`
	}

	entries := make([]leaderboardEntry, 0, matchCount)
	for i := 0; i < matchCount; i++ {
		matchSeed := *baseSeed + int64(i)*seedDelta
		summary := runMatchSummary(matchSeed, tickCount, normalizeNonNegativeInt(*topBots))
		winner := winningBot(summary.TopBots)
		entries = append(entries, leaderboardEntry{
			MatchID:       fmt.Sprintf("match-%d", matchSeed),
			Seed:          matchSeed,
			Summary:       summary,
			Winner:        winner,
			WinnerScore:   winnerValue(winner),
			LiveBotCount:  summary.LiveBots,
			ColonyCount:   summary.ColonyCount,
			HasReplayHint: true,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].WinnerScore != entries[j].WinnerScore {
			return entries[i].WinnerScore > entries[j].WinnerScore
		}
		if entries[i].LiveBotCount != entries[j].LiveBotCount {
			return entries[i].LiveBotCount > entries[j].LiveBotCount
		}
		return entries[i].Seed < entries[j].Seed
	})

	payload := map[string]any{
		"command":        "leaderboard",
		"base_seed":      *baseSeed,
		"seed_step":      seedDelta,
		"matches":        matchCount,
		"ticks":          tickCount,
		"leaderboard":    entries,
		"run_order":      make([]string, matchCount),
		"updated_at_utc": time.Now().UTC().Format(time.RFC3339),
	}
	for i := range entries {
		payload["run_order"].([]string)[i] = entries[i].MatchID
	}

	printJSON(payload, *pretty)
}

func runReplay(args []string) {
	flags := commandFlagSet("replay")
	seed := flags.Int64("seed", 1, "Deterministic PRNG seed.")
	ticks := flags.Int("ticks", defaultReplayTicks, "Simulation ticks to execute.")
	sampleEvery := flags.Int("sample-every", defaultReplaySampleEvery, "Sample interval for frame output.")
	topBots := flags.Int("top-bots", defaultTopBots, "Number of top bots to include per frame.")
	pretty := flags.Bool("pretty", false, "Pretty-print JSON output.")
	usage := "replay [--seed N] [--ticks T] [--sample-every N] [--top-bots M] [--pretty]"
	if err := parseCommandFlags(flags, args, usage); err != nil {
		os.Exit(2)
	}

	tickCount := normalizeNonNegativeInt(*ticks)
	interval := normalizePositiveInt(*sampleEvery)
	summary := runReplaySummary(*seed, tickCount, interval, normalizeNonNegativeInt(*topBots))
	payload := map[string]any{
		"command":       "replay",
		"match_id":      fmt.Sprintf("match-%d", *seed),
		"seed":          *seed,
		"ticks":         tickCount,
		"sample_every":  interval,
		"frames":        summary,
		"final_summary": summary[len(summary)-1],
		"winner":        winningBot(summary[len(summary)-1].TopBots),
	}
	printJSON(payload, *pretty)
}

type smartnessEvalRun struct {
	Seed                       int64   `json:"seed"`
	Ticks                      int     `json:"ticks"`
	LiveBots                   int     `json:"live_bots"`
	SuccessfulDivisions        int     `json:"successful_divisions"`
	MaxLineageDepth            int     `json:"max_lineage_depth"`
	ActiveColonies             int     `json:"active_colonies"`
	NonSoloActiveColonies      int     `json:"non_solo_active_colonies"`
	MaxConnectedMembers        int     `json:"max_connected_members"`
	MaxColonyComponent         int     `json:"max_colony_component"`
	MaxConnectedComponent      int     `json:"max_connected_component"`
	LongestColonyRun           int     `json:"longest_colony_run"`
	LongestConnectedRun        int     `json:"longest_connected_run"`
	ColonyTissueCells          int     `json:"colony_tissue_cells"`
	FoodGathered               int     `json:"food_gathered"`
	OreGathered                int     `json:"ore_gathered"`
	EliteCount                 int     `json:"elite_count"`
	BestScore                  int     `json:"best_score"`
	TopColonyLinkedBots        int     `json:"top_colony_linked_bots"`
	TopActiveColonyBots        int     `json:"top_active_colony_bots"`
	StolenFood                 int     `json:"stolen_food"`
	StolenOre                  int     `json:"stolen_ore"`
	CombatKills                int     `json:"combat_kills"`
	ControllerRaids            int     `json:"controller_raids"`
	Depots                     int     `json:"depots"`
	Spawners                   int     `json:"spawners"`
	SpawnerBirths              int     `json:"spawner_births"`
	TotalSpawnerCharges        int     `json:"total_spawner_charges"`
	TopSpawnerActiveBots       int     `json:"top_spawner_active_bots"`
	NonColonySpawnerBots       int     `json:"non_colony_spawner_top_bots"`
	TotalDepotFood             int     `json:"total_depot_food"`
	TotalDepotOre              int     `json:"total_depot_ore"`
	DepotRaids                 int     `json:"depot_raids"`
	TopNonColonyDirectionShare float64 `json:"top_non_colony_direction_share"`
}

type smartnessEvalAggregate struct {
	Seeds                            int     `json:"seeds"`
	Ticks                            int     `json:"ticks"`
	MedianLiveBots                   int     `json:"median_live_bots"`
	MedianSuccessfulDivisions        int     `json:"median_successful_divisions"`
	MedianMaxLineageDepth            int     `json:"median_max_lineage_depth"`
	SeedsWithNonSoloActiveColony     int     `json:"seeds_with_non_solo_active_colony"`
	MedianMaxConnectedMembers        int     `json:"median_max_connected_members"`
	MedianMaxColonyComponent         int     `json:"median_max_colony_component"`
	MedianMaxConnectedComponent      int     `json:"median_max_connected_component"`
	MedianLongestColonyRun           int     `json:"median_longest_colony_run"`
	MedianLongestConnectedRun        int     `json:"median_longest_connected_run"`
	MedianColonyTissueCells          int     `json:"median_colony_tissue_cells"`
	TotalFoodGathered                int     `json:"total_food_gathered"`
	TotalOreGathered                 int     `json:"total_ore_gathered"`
	TotalStolenFood                  int     `json:"total_stolen_food"`
	TotalStolenOre                   int     `json:"total_stolen_ore"`
	TotalDepotRaids                  int     `json:"total_depot_raids"`
	SeedsWithDepots                  int     `json:"seeds_with_depots"`
	SeedsWithSpawners                int     `json:"seeds_with_spawners"`
	TotalSpawnerBirths               int     `json:"total_spawner_births"`
	SeedsWithNonColonySpawnerTop     int     `json:"seeds_with_non_colony_spawner_top_bot"`
	SeedsWithColonyLinkedTopBot      int     `json:"seeds_with_colony_linked_top_bot"`
	SeedsWithActiveColonyTopBot      int     `json:"seeds_with_active_colony_top_bot"`
	MedianBestScore                  int     `json:"median_best_score"`
	MaxBestScore                     int     `json:"max_best_score"`
	MedianTopNonColonyDirectionShare float64 `json:"median_top_non_colony_direction_share"`
}

func runSmartnessEval(args []string) {
	flags := commandFlagSet("smartness-eval")
	seedsArg := flags.String("seeds", "1 2 3", "Space- or comma-separated deterministic seeds.")
	ticks := flags.Int("ticks", defaultSmartnessEvalTicks, "Simulation ticks per seed.")
	smartEvolution := flags.Bool("smart-evolution", true, "Enable smart evolution during the eval.")
	pretty := flags.Bool("pretty", false, "Pretty-print JSON output.")
	usage := "smartness-eval [--seeds \"1 2 3\"] [--ticks N] [--smart-evolution=true|false] [--pretty]"
	if err := parseCommandFlags(flags, args, usage); err != nil {
		os.Exit(2)
	}

	seeds, err := parseSeedList(*seedsArg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		flags.Usage()
		os.Exit(2)
	}

	tickCount := normalizeNonNegativeInt(*ticks)
	runs := make([]smartnessEvalRun, 0, len(seeds))
	for _, seed := range seeds {
		summary := runMatchSummaryWithSmartEvolution(seed, tickCount, 3, *smartEvolution)
		runs = append(runs, smartnessEvalRun{
			Seed:                       seed,
			Ticks:                      tickCount,
			LiveBots:                   summary.LiveBots,
			SuccessfulDivisions:        summary.SuccessfulDivisions,
			MaxLineageDepth:            summary.MaxLineageDepth,
			ActiveColonies:             summary.ActiveColonies,
			NonSoloActiveColonies:      max(0, summary.ActiveColonies-summary.SoloActiveColonies),
			MaxConnectedMembers:        summary.MaxConnectedMembers,
			MaxColonyComponent:         summary.MaxColonyComponent,
			MaxConnectedComponent:      summary.MaxConnectedComponent,
			LongestColonyRun:           summary.LongestColonyRun,
			LongestConnectedRun:        summary.LongestConnectedRun,
			ColonyTissueCells:          summary.ColonyTissueCells,
			FoodGathered:               summary.FoodGathered,
			OreGathered:                summary.OreGathered,
			EliteCount:                 summary.EliteCount,
			BestScore:                  summary.BestScore,
			TopColonyLinkedBots:        summary.TopColonyLinkedBots,
			TopActiveColonyBots:        summary.TopActiveColonyBots,
			StolenFood:                 summary.StolenFood,
			StolenOre:                  summary.StolenOre,
			CombatKills:                summary.CombatKills,
			ControllerRaids:            summary.ControllerRaids,
			Depots:                     summary.Depots,
			Spawners:                   summary.Spawners,
			SpawnerBirths:              summary.SpawnerBirths,
			TotalSpawnerCharges:        summary.TotalSpawnerCharges,
			TopSpawnerActiveBots:       summary.TopSpawnerActiveBots,
			NonColonySpawnerBots:       nonColonySpawnerTopBots(summary.TopBots),
			TotalDepotFood:             summary.TotalDepotFood,
			TotalDepotOre:              summary.TotalDepotOre,
			DepotRaids:                 summary.DepotRaids,
			TopNonColonyDirectionShare: summary.TopNonColonyDirectionShare,
		})
	}

	payload := map[string]any{
		"command":         "smartness-eval",
		"ticks":           tickCount,
		"smart_evolution": *smartEvolution,
		"runs":            runs,
		"aggregate":       aggregateSmartnessEval(runs, tickCount),
	}
	printJSON(payload, *pretty)
}

func aggregateSmartnessEval(runs []smartnessEvalRun, ticks int) smartnessEvalAggregate {
	liveBots := make([]int, 0, len(runs))
	divisions := make([]int, 0, len(runs))
	depths := make([]int, 0, len(runs))
	connected := make([]int, 0, len(runs))
	colonyComponents := make([]int, 0, len(runs))
	connectedComponents := make([]int, 0, len(runs))
	colonyRuns := make([]int, 0, len(runs))
	connectedRuns := make([]int, 0, len(runs))
	tissueCells := make([]int, 0, len(runs))
	directionShares := make([]float64, 0, len(runs))
	bestScores := make([]int, 0, len(runs))
	out := smartnessEvalAggregate{Seeds: len(runs), Ticks: ticks}
	for _, run := range runs {
		liveBots = append(liveBots, run.LiveBots)
		divisions = append(divisions, run.SuccessfulDivisions)
		depths = append(depths, run.MaxLineageDepth)
		connected = append(connected, run.MaxConnectedMembers)
		colonyComponents = append(colonyComponents, run.MaxColonyComponent)
		connectedComponents = append(connectedComponents, run.MaxConnectedComponent)
		colonyRuns = append(colonyRuns, run.LongestColonyRun)
		connectedRuns = append(connectedRuns, run.LongestConnectedRun)
		tissueCells = append(tissueCells, run.ColonyTissueCells)
		directionShares = append(directionShares, run.TopNonColonyDirectionShare)
		bestScores = append(bestScores, run.BestScore)
		if run.NonSoloActiveColonies > 0 {
			out.SeedsWithNonSoloActiveColony++
		}
		out.TotalFoodGathered += run.FoodGathered
		out.TotalOreGathered += run.OreGathered
		out.TotalStolenFood += run.StolenFood
		out.TotalStolenOre += run.StolenOre
		out.TotalDepotRaids += run.DepotRaids
		if run.Depots > 0 {
			out.SeedsWithDepots++
		}
		if run.Spawners > 0 {
			out.SeedsWithSpawners++
		}
		out.TotalSpawnerBirths += run.SpawnerBirths
		if run.NonColonySpawnerBots > 0 {
			out.SeedsWithNonColonySpawnerTop++
		}
		if run.TopColonyLinkedBots > 0 {
			out.SeedsWithColonyLinkedTopBot++
		}
		if run.TopActiveColonyBots > 0 {
			out.SeedsWithActiveColonyTopBot++
		}
		if run.BestScore > out.MaxBestScore {
			out.MaxBestScore = run.BestScore
		}
	}
	out.MedianLiveBots = medianInt(liveBots)
	out.MedianSuccessfulDivisions = medianInt(divisions)
	out.MedianMaxLineageDepth = medianInt(depths)
	out.MedianMaxConnectedMembers = medianInt(connected)
	out.MedianMaxColonyComponent = medianInt(colonyComponents)
	out.MedianMaxConnectedComponent = medianInt(connectedComponents)
	out.MedianLongestColonyRun = medianInt(colonyRuns)
	out.MedianLongestConnectedRun = medianInt(connectedRuns)
	out.MedianColonyTissueCells = medianInt(tissueCells)
	out.MedianBestScore = medianInt(bestScores)
	out.MedianTopNonColonyDirectionShare = medianFloat64(directionShares)
	return out
}

func medianInt(values []int) int {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]int(nil), values...)
	sort.Ints(sorted)
	return sorted[len(sorted)/2]
}

func medianFloat64(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	return sorted[len(sorted)/2]
}

type gameMasterFrame struct {
	Tick    int          `json:"tick"`
	Summary matchSummary `json:"summary"`
}

type gameMasterRun struct {
	Command      string             `json:"command"`
	Seed         int64              `json:"seed"`
	Ticks        int                `json:"ticks"`
	Interval     int                `json:"interval"`
	Master       string             `json:"master"`
	Events       []game.MasterEvent `json:"events"`
	Frames       []gameMasterFrame  `json:"frames"`
	FinalSummary matchSummary       `json:"final_summary"`
	Winner       *botSummary        `json:"winner"`
}

func runGameMaster(args []string) {
	flags := commandFlagSet("gamemaster")
	seed := flags.Int64("seed", 1, "Deterministic PRNG seed.")
	ticks := flags.Int("ticks", defaultMatchTicks, "Simulation ticks to execute.")
	interval := flags.Int("interval", 25, "Ticks between mock game-master observations.")
	topBots := flags.Int("top-bots", defaultTopBots, "Number of top bots to include per sampled frame.")
	advisor := flags.String("advisor", "mock", "Game-master advisor: mock or external.")
	gmCommand := flags.String("gm-command", "", "External advisor command; receives observation JSON on stdin.")
	gmTimeout := flags.Duration("gm-timeout", 750*time.Millisecond, "External advisor timeout.")
	pretty := flags.Bool("pretty", false, "Pretty-print JSON output.")
	usage := "gamemaster [--seed N] [--ticks T] [--interval N] [--top-bots M] [--advisor mock|external] [--gm-command CMD] [--gm-timeout 750ms] [--pretty]"
	if err := parseCommandFlags(flags, args, usage); err != nil {
		os.Exit(2)
	}

	gameMaster, err := newCommandGameMaster(*advisor, *gmCommand, *gmTimeout)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	result := runGameMasterSummary(
		*seed,
		normalizeNonNegativeInt(*ticks),
		normalizePositiveInt(*interval),
		normalizeNonNegativeInt(*topBots),
		gameMaster,
	)
	printJSON(result, *pretty)
}

func newCommandGameMaster(advisor string, command string, timeout time.Duration) (game.GameMasterAdvisor, error) {
	switch strings.ToLower(strings.TrimSpace(advisor)) {
	case "", "mock":
		return game.NewMockGameMaster(), nil
	case "external", "coolio":
		argv := game.SplitGameMasterCommand(command)
		if len(argv) == 0 {
			return nil, fmt.Errorf("--gm-command is required when --advisor external is used")
		}
		return game.NewExternalGameMaster(argv, timeout, game.NewMockGameMaster()), nil
	default:
		return nil, fmt.Errorf("unknown --advisor: %s", advisor)
	}
}

func runGameMasterSummary(seed int64, ticks, interval, topBots int, master game.GameMasterAdvisor) gameMasterRun {
	gameRunner := newDeterministicGame(seed)
	gameRunner.InitializeForCommands()

	frames := []gameMasterFrame{
		{Tick: 0, Summary: summarizeMatch(gameRunner, seed, 0, topBots)},
	}
	events := []game.MasterEvent{}

	for tick := 1; tick <= ticks; tick++ {
		gameRunner.RunHeadlessFrames(1)
		if tick%interval == 0 {
			obs := gameRunner.ObserveMaster(tick)
			if event, ok := master.Decide(obs); ok {
				events = append(events, gameRunner.ApplyMasterEvent(event))
			}
		}
		if tick%interval == 0 || tick == ticks {
			frames = append(frames, gameMasterFrame{
				Tick:    tick,
				Summary: summarizeMatch(gameRunner, seed, tick, topBots),
			})
		}
	}

	finalSummary := frames[len(frames)-1].Summary
	return gameMasterRun{
		Command:      "gamemaster",
		Seed:         seed,
		Ticks:        ticks,
		Interval:     interval,
		Master:       master.AdvisorName(),
		Events:       events,
		Frames:       frames,
		FinalSummary: finalSummary,
		Winner:       winningBot(finalSummary.TopBots),
	}
}

func runRender(args []string) {
	flags := commandFlagSet("render")
	seed := flags.Int64("seed", 1, "Deterministic PRNG seed.")
	ticks := flags.Int("ticks", defaultStatusTicks, "Simulation ticks to execute before rendering.")
	targetBots := flags.Int("target-bots", 0, "Exact scale-seeded bot count before rendering; 0 uses normal initialization.")
	output := flags.String("output", "golab-render.png", "PNG output path.")
	cellSize := flags.Int("cell-size", 2, "Output pixels per board cell.")
	padding := flags.Int("padding", 0, "Outer image padding in pixels.")
	atlasPath := flags.String("atlas", "assests/sprites/atlas.png", "Sprite atlas path.")
	style := flags.String("style", "game", "Render style: game, atlas, flat, pheromone, biome, density, or colony.")
	border := flags.Bool("border", false, "Draw a border around the board.")
	legend := flags.Bool("legend", false, "Draw a compact visual legend below the board.")
	pretty := flags.Bool("pretty", false, "Pretty-print JSON output.")
	usage := "render [--seed N] [--ticks N] [--target-bots N] [--output path] [--cell-size N] [--padding N] [--style game|atlas|flat|pheromone|biome|density|colony] [--border=true|false] [--legend=true|false] [--pretty]"
	if err := parseCommandFlags(flags, args, usage); err != nil {
		os.Exit(2)
	}

	tickCount := normalizeNonNegativeInt(*ticks)
	gameRunner := newDeterministicGame(*seed)
	target := normalizeNonNegativeInt(*targetBots)
	if target > 0 {
		gameRunner = newScaleGame(*seed)
		if err := gameRunner.InitializeForScale(target, *seed); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	} else {
		gameRunner.InitializeForCommands()
	}
	gameRunner.RunHeadlessFrames(tickCount)

	result, err := render.SaveBoardPNG(gameRunner.Board, render.Options{
		AtlasPath:          *atlasPath,
		Output:             *output,
		CellSize:           normalizePositiveInt(*cellSize),
		Padding:            normalizeNonNegativeInt(*padding),
		Border:             *border,
		Legend:             *legend,
		Style:              *style,
		RenderPaths:        true,
		RenderTaskTargets:  true,
		RenderUnreachables: true,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	payload := map[string]any{
		"command":     "render",
		"seed":        *seed,
		"ticks":       tickCount,
		"target_bots": target,
		"output":      result.Output,
		"width":       result.Width,
		"height":      result.Height,
		"cell_size":   normalizePositiveInt(*cellSize),
		"style":       *style,
	}
	printJSON(payload, *pretty)
}

type scaleTestResult struct {
	Command             string  `json:"command"`
	Seed                int64   `json:"seed"`
	Rows                int     `json:"rows"`
	Cols                int     `json:"cols"`
	TargetBots          int     `json:"target_bots"`
	InitialLiveBots     int     `json:"initial_live_bots"`
	FinalLiveBots       int     `json:"final_live_bots"`
	Ticks               int     `json:"ticks"`
	WarmupTicks         int     `json:"warmup_ticks,omitempty"`
	ElapsedMS           int64   `json:"elapsed_ms"`
	LogicTicksPerSecond float64 `json:"logic_ticks_per_second"`
	BotStepsPerSecond   float64 `json:"bot_steps_per_second"`
	HeapMB              float64 `json:"heap_mb"`
	AllocMB             float64 `json:"alloc_mb"`
	GCCount             uint32  `json:"gc_count"`
}

func runScaleTest(args []string) {
	flags := commandFlagSet("scale-test")
	seed := flags.Int64("seed", 42, "Deterministic PRNG seed.")
	targetBots := flags.Int("target-bots", defaultScaleTargetBots, "Exact number of bots to seed.")
	ticks := flags.Int("ticks", defaultScaleTicks, "Measured simulation ticks to execute.")
	warmupTicks := flags.Int("warmup-ticks", defaultScaleWarmupTicks, "Warmup ticks before measuring.")
	pretty := flags.Bool("pretty", false, "Pretty-print JSON output.")
	usage := "scale-test [--seed N] [--target-bots N] [--ticks N] [--warmup-ticks N] [--pretty]"
	if err := parseCommandFlags(flags, args, usage); err != nil {
		os.Exit(2)
	}

	target := normalizeNonNegativeInt(*targetBots)
	tickCount := normalizeNonNegativeInt(*ticks)
	warmup := normalizeNonNegativeInt(*warmupTicks)

	gameRunner := newScaleGame(*seed)
	if err := gameRunner.InitializeForScale(target, *seed); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	initialLive := gameRunner.Board.ActiveBotCount()
	if initialLive != target {
		fmt.Fprintf(os.Stderr, "scale seed produced %d live bots, want %d\n", initialLive, target)
		os.Exit(1)
	}

	if warmup > 0 {
		gameRunner.RunHeadlessFrames(warmup)
	}
	runtime.GC()

	start := time.Now()
	if tickCount > 0 {
		gameRunner.RunHeadlessFrames(tickCount)
	}
	elapsed := time.Since(start)
	if elapsed <= 0 {
		elapsed = time.Nanosecond
	}

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	seconds := elapsed.Seconds()
	result := scaleTestResult{
		Command:             "scale-test",
		Seed:                *seed,
		Rows:                core.Rows,
		Cols:                core.Cols,
		TargetBots:          target,
		InitialLiveBots:     initialLive,
		FinalLiveBots:       gameRunner.Board.ActiveBotCount(),
		Ticks:               tickCount,
		WarmupTicks:         warmup,
		ElapsedMS:           elapsed.Milliseconds(),
		LogicTicksPerSecond: float64(tickCount) / seconds,
		BotStepsPerSecond:   float64(initialLive*tickCount) / seconds,
		HeapMB:              bytesToMB(mem.HeapInuse),
		AllocMB:             bytesToMB(mem.Alloc),
		GCCount:             mem.NumGC,
	}
	printJSON(result, *pretty)
}

func bytesToMB(bytes uint64) float64 {
	return float64(bytes) / 1024.0 / 1024.0
}

type discordField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

type discordCard struct {
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Color       int            `json:"color"`
	Fields      []discordField `json:"fields"`
	Footer      map[string]any `json:"footer"`
}

func runSeedRoulette(args []string) {
	flags := commandFlagSet("seed-roulette")
	seed := flags.String("seed", "random", "Deterministic PRNG seed or random.")
	ticks := flags.Int("ticks", defaultMatchTicks, "Simulation ticks to execute.")
	topBots := flags.Int("top-bots", defaultTopBots, "Number of top bots to include in output.")
	pretty := flags.Bool("pretty", false, "Pretty-print JSON output.")
	usage := "seed-roulette [--seed N|random] [--ticks N] [--top-bots N] [--pretty]"
	if err := parseCommandFlags(flags, args, usage); err != nil {
		os.Exit(2)
	}

	matchSeed, rngSource, err := resolveRouletteSeed(*seed)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		flags.Usage()
		os.Exit(2)
	}

	tickCount := normalizeNonNegativeInt(*ticks)
	topBotsCount := normalizeNonNegativeInt(*topBots)
	summary := runMatchSummary(matchSeed, tickCount, topBotsCount)
	winner := winningBot(summary.TopBots)
	winnerHP := winnerValue(winner)

	winnerDesc := "none"
	if winner != nil {
		winnerDesc = fmt.Sprintf("Bot #%d — HP %d, F %d O %d", winner.Index, winner.Hp, winner.FoodInventory, winner.OreInventory)
	}

	card := discordCard{
		Title:       fmt.Sprintf("🎰 Seed Roulette — Match #%d", matchSeed),
		Description: fmt.Sprintf("Rolled seed **%d** (%s). Simulated %d ticks.", matchSeed, rngSource, tickCount),
		Color:       0x5865F2,
		Fields: []discordField{
			{Name: "Seed", Value: fmt.Sprintf("%d", matchSeed), Inline: true},
			{Name: "Ticks", Value: fmt.Sprintf("%d", tickCount), Inline: true},
			{Name: "Live Bots", Value: fmt.Sprintf("%d", summary.LiveBots), Inline: true},
			{Name: "Colonies", Value: fmt.Sprintf("%d", summary.ColonyCount), Inline: true},
			{Name: "Winner Score", Value: fmt.Sprintf("%d", winnerHP), Inline: true},
			{Name: "Winner", Value: winnerDesc, Inline: false},
		},
		Footer: map[string]any{"text": fmt.Sprintf("bots-arena • %s", time.Now().UTC().Format(time.RFC3339))},
	}

	actions := []string{"rerun", "mutate", "timeline", "sweep-similar"}
	verdict := seedRouletteVerdict(summary)

	payload := map[string]any{
		"command":   "seed-roulette",
		"match_id":  fmt.Sprintf("match-%d", matchSeed),
		"seed":      matchSeed,
		"ticks":     tickCount,
		"summary":   summary,
		"winner":    winner,
		"winner_hp": winnerHP,
		"card":      card,
		"verdict":   verdict,
		"actions":   actions,
	}
	printJSON(payload, *pretty)
}

func parseCommandFlags(flags *flag.FlagSet, args []string, usage string) error {
	flags.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: bots-arena %s\n", usage)
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() > 0 {
		flags.Usage()
		return fmt.Errorf("unexpected positional arguments: %v", flags.Args())
	}
	return nil
}

func seedRouletteVerdict(summary matchSummary) string {
	switch {
	case summary.LiveBots >= 5000 && summary.Poison < 300:
		return "thriving wild swarm"
	case summary.LiveBots >= 5000:
		return "chaotic population boom"
	case summary.LiveBots < 500:
		return "near-extinction spiral"
	case summary.Water > summary.Resources:
		return "water-heavy strange world"
	default:
		return "unstable frontier"
	}
}

func resolveRouletteSeed(value string) (int64, string, error) {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if trimmed == "" || trimmed == "random" || trimmed == "roll" {
		return time.Now().UnixNano()%900000 + 100000, "random", nil
	}
	seed, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil {
		return 0, "", fmt.Errorf("invalid seed %q: use an integer or random", value)
	}
	return seed, "provided", nil
}

func parseSeedList(value string) ([]int64, error) {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\t' || r == '\n'
	})
	seeds := make([]int64, 0, len(fields))
	for _, field := range fields {
		seed, err := strconv.ParseInt(field, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid seed %q: %w", field, err)
		}
		seeds = append(seeds, seed)
	}
	if len(seeds) == 0 {
		return nil, fmt.Errorf("--seeds must include at least one integer seed")
	}
	return seeds, nil
}

func normalizeNonNegativeInt(value int) int {
	if value < 0 {
		return 0
	}
	return value
}

func commandFlagSet(name string) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	return flags
}

func printJSON(payload any, pretty bool) {
	var (
		output []byte
		err    error
	)
	if pretty {
		output, err = json.MarshalIndent(payload, "", "  ")
	} else {
		output, err = json.Marshal(payload)
	}
	if err != nil {
		panic(err)
	}
	fmt.Println(string(output))
}

type botSummary struct {
	Index              int     `json:"index"`
	Hp                 int     `json:"hp"`
	Inventory          int     `json:"inventory"`
	FoodInventory      int     `json:"food_inventory"`
	OreInventory       int     `json:"ore_inventory"`
	Divisions          int     `json:"divisions"`
	LineageDepth       int     `json:"lineage_depth"`
	ReproductionScore  int     `json:"reproduction_score"`
	EvolutionScore     int     `json:"evolution_score"`
	FoodGathered       int     `json:"food_gathered"`
	OreGathered        int     `json:"ore_gathered"`
	StolenFood         int     `json:"stolen_food"`
	StolenOre          int     `json:"stolen_ore"`
	CombatKills        int     `json:"combat_kills"`
	ControllerRaids    int     `json:"controller_raids"`
	DepotRaids         int     `json:"depot_raids"`
	DepotBuilds        int     `json:"depot_builds"`
	SpawnerBuilds      int     `json:"spawner_builds"`
	SpawnerBirths      int     `json:"spawner_births"`
	DepotDepositedFood int     `json:"depot_deposited_food"`
	DepotDepositedOre  int     `json:"depot_deposited_ore"`
	TaskCompletions    int     `json:"task_completions"`
	ColonyID           *int    `json:"colony_id"`
	ColonyLinked       bool    `json:"colony_linked"`
	ActiveColony       bool    `json:"active_colony"`
	ActiveNonSolo      bool    `json:"active_non_solo_colony"`
	ConnectedMembers   int     `json:"connected_member_count"`
	ColonyFoodBank     *int    `json:"colony_food_bank,omitempty"`
	ColonyOreBank      *int    `json:"colony_ore_bank,omitempty"`
	X                  int     `json:"x"`
	Y                  int     `json:"y"`
	HasTask            bool    `json:"has_task"`
	CooldownLeft       float64 `json:"cooldown_left_seconds"`
}

type matchSummary struct {
	Command                    string       `json:"command"`
	Seed                       int64        `json:"seed"`
	Ticks                      int          `json:"ticks"`
	Timestamp                  string       `json:"timestamp"`
	LiveBots                   int          `json:"live_bots"`
	ColonyCount                int          `json:"colony_count"`
	Controller                 int          `json:"controllers"`
	FarmCount                  int          `json:"farms"`
	Spawners                   int          `json:"spawners"`
	SpawnerBirths              int          `json:"spawner_births"`
	TotalSpawnerCharges        int          `json:"total_spawner_charges"`
	Mines                      int          `json:"mines"`
	Buildings                  int          `json:"buildings"`
	Depots                     int          `json:"depots"`
	TotalDepotFood             int          `json:"total_depot_food"`
	TotalDepotOre              int          `json:"total_depot_ore"`
	Food                       int          `json:"food"`
	Resources                  int          `json:"resources"`
	Poison                     int          `json:"poison"`
	Organics                   int          `json:"organics"`
	Water                      int          `json:"water"`
	Wall                       int          `json:"wall"`
	TotalHP                    int          `json:"total_bot_hp"`
	TotalInv                   int          `json:"total_bot_inventory"`
	TotalFoodInv               int          `json:"total_food_inventory"`
	TotalOreInv                int          `json:"total_ore_inventory"`
	TotalColonyFoodBank        int          `json:"total_colony_food_bank"`
	TotalColonyOreBank         int          `json:"total_colony_ore_bank"`
	ColonyMemberBots           int          `json:"colony_member_bots"`
	ConnectedColonyBots        int          `json:"connected_colony_bots"`
	ActiveColonies             int          `json:"active_colonies"`
	SoloActiveColonies         int          `json:"solo_active_colonies"`
	MaxColonyMembers           int          `json:"max_colony_members"`
	MaxConnectedMembers        int          `json:"max_connected_members"`
	MaxColonyComponent         int          `json:"max_colony_component"`
	MaxConnectedComponent      int          `json:"max_connected_component"`
	LongestColonyRun           int          `json:"longest_colony_run"`
	LongestConnectedRun        int          `json:"longest_connected_run"`
	ColonyTissueCells          int          `json:"colony_tissue_cells"`
	FriendlyAdjacencies        int          `json:"friendly_adjacencies"`
	ForeignAdjacencies         int          `json:"foreign_adjacencies"`
	PheromoneActiveCells       int          `json:"pheromone_active_cells"`
	TotalFoodPheromone         int          `json:"total_food_pheromone"`
	TotalOrePheromone          int          `json:"total_ore_pheromone"`
	TotalHomePheromone         int          `json:"total_home_pheromone"`
	TotalDangerPheromone       int          `json:"total_danger_pheromone"`
	DivisionReadyBots          int          `json:"division_ready_bots"`
	SuccessfulDivisions        int          `json:"successful_divisions"`
	LivingBotDivisions         int          `json:"living_bot_divisions"`
	MaxLineageDepth            int          `json:"max_lineage_depth"`
	FoodGathered               int          `json:"food_gathered"`
	OreGathered                int          `json:"ore_gathered"`
	StolenFood                 int          `json:"stolen_food"`
	StolenOre                  int          `json:"stolen_ore"`
	CombatKills                int          `json:"combat_kills"`
	ControllerRaids            int          `json:"controller_raids"`
	DepotRaids                 int          `json:"depot_raids"`
	EliteCount                 int          `json:"elite_count"`
	BestScore                  int          `json:"best_score"`
	TopColonyLinkedBots        int          `json:"top_colony_linked_bots"`
	TopActiveColonyBots        int          `json:"top_active_colony_bots"`
	TopSpawnerActiveBots       int          `json:"top_spawner_active_bots"`
	TopNonColonyDirectionShare float64      `json:"top_non_colony_direction_share"`
	TopBots                    []botSummary `json:"top_bots"`
}

func runMatchSummary(seed int64, ticks, topBots int) matchSummary {
	return runMatchSummaryWithSmartEvolution(seed, ticks, topBots, true)
}

func runMatchSummaryWithSmartEvolution(seed int64, ticks, topBots int, smartEvolution bool) matchSummary {
	gameRunner := newDeterministicGameWithSmartEvolution(seed, smartEvolution)
	tickCount := normalizeNonNegativeInt(ticks)
	gameRunner.InitializeForCommands()
	gameRunner.RunHeadlessFrames(tickCount)
	return summarizeMatch(gameRunner, seed, tickCount, topBots)
}

func runReplaySummary(seed int64, ticks, sampleEvery, topBots int) []matchSummary {
	gameRunner := newDeterministicGame(seed)
	gameRunner.InitializeForCommands()

	frames := []matchSummary{
		summarizeMatch(gameRunner, seed, 0, topBots),
	}
	for tick := 1; tick <= ticks; tick++ {
		gameRunner.RunHeadlessFrames(1)
		if tick%sampleEvery == 0 || tick == ticks {
			frames = append(frames, summarizeMatch(gameRunner, seed, tick, topBots))
		}
	}
	return frames
}

func newDeterministicGame(seed int64) *game.Game {
	return newDeterministicGameWithSmartEvolution(seed, true)
}

func newDeterministicGameWithSmartEvolution(seed int64, smartEvolution bool) *game.Game {
	conf := config.NewConfig()
	conf.LogicStep = 0
	conf.SmartEvolution = smartEvolution
	rand.Seed(seed)
	expRand.Seed(uint64(seed))
	return game.NewGame(&conf)
}

func newScaleGame(seed int64) *game.Game {
	conf := config.NewConfig()
	conf.LogicStep = 0
	conf.NewGenThreshold = 0
	conf.ImmigrationBots = 0
	conf.ImmigrationInterval = 0
	conf.SmartEvolution = false
	rand.Seed(seed)
	expRand.Seed(uint64(seed))
	return game.NewGame(&conf)
}

func registerColonyID(colonyIDByRef map[*core.Colony]int, colony *core.Colony) {
	if colony == nil {
		return
	}
	if _, ok := colonyIDByRef[colony]; !ok {
		colonyIDByRef[colony] = len(colonyIDByRef)
	}
}

func summarizeMatch(g *game.Game, seed int64, tick int, topBots int) matchSummary {
	summary := matchSummary{
		Command:   "status",
		Seed:      seed,
		Ticks:     tick,
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
	}

	colonyIDByRef := map[*core.Colony]int{}
	for _, colony := range g.Colonies {
		registerColonyID(colonyIDByRef, colony)
	}
	for _, cell := range *g.Board.GetGrid() {
		switch v := cell.(type) {
		case core.Controller:
			registerColonyID(colonyIDByRef, v.Colony)
		case *core.Controller:
			registerColonyID(colonyIDByRef, v.Colony)
		case core.Depot:
			registerColonyID(colonyIDByRef, v.Colony)
		case *core.Depot:
			if v != nil {
				registerColonyID(colonyIDByRef, v.Colony)
			}
		case core.Farm:
			registerColonyID(colonyIDByRef, colonyForSummaryOwnedCell(v.Colony, v.Owner))
		case core.Spawner:
			registerColonyID(colonyIDByRef, colonyForSummaryOwnedCell(v.Colony, v.Owner))
		}
	}
	for _, id := range g.Board.ActiveBotIDs() {
		bot := g.Board.BotByID(id)
		if bot != nil {
			registerColonyID(colonyIDByRef, bot.Colony)
		}
	}
	summary.ColonyCount = len(colonyIDByRef)
	for colony := range colonyIDByRef {
		summary.TotalColonyFoodBank += colony.FoodBank
		summary.TotalColonyOreBank += colony.OreBank
	}
	pheromones := g.Board.PheromoneTotals()
	summary.PheromoneActiveCells = pheromones.ActiveCells
	summary.TotalFoodPheromone = pheromones.Food
	summary.TotalOrePheromone = pheromones.Ore
	summary.TotalHomePheromone = pheromones.Home
	summary.TotalDangerPheromone = pheromones.Danger

	topSelector := newTopBotSelector(normalizeNonNegativeInt(topBots))
	for _, id := range g.Board.ActiveBotIDs() {
		bot := g.Board.BotByID(id)
		if bot == nil {
			continue
		}
		idx := g.Board.BotCell(id)
		if idx < 0 {
			continue
		}
		summary.LiveBots++
		summary.TotalHP += bot.Hp
		totalInventory := bot.Inventory.Total()
		summary.TotalInv += totalInventory
		summary.TotalFoodInv += bot.Inventory.Food
		summary.TotalOreInv += bot.Inventory.Ore
		summary.LivingBotDivisions += bot.Divisions
		if bot.Colony != nil {
			summary.ColonyMemberBots++
		}
		if bot.ConnnectedToColony {
			summary.ConnectedColonyBots++
		}
		if bot.LineageDepth > summary.MaxLineageDepth {
			summary.MaxLineageDepth = bot.LineageDepth
		}
		if g.DivisionReady(bot) {
			summary.DivisionReadyBots++
		}

		reproductionScore := bot.ReproductionScore()
		evolutionScore := g.BotEvolutionScore(bot)
		evolutionProfile := g.BotEvolutionProfile(bot)
		entry := botSummary{
			Index:              idx,
			Hp:                 bot.Hp,
			Inventory:          totalInventory,
			FoodInventory:      bot.Inventory.Food,
			OreInventory:       bot.Inventory.Ore,
			Divisions:          bot.Divisions,
			LineageDepth:       bot.LineageDepth,
			ReproductionScore:  reproductionScore,
			EvolutionScore:     evolutionScore,
			FoodGathered:       bot.Evolution.FoodGathered,
			OreGathered:        bot.Evolution.OreGathered,
			StolenFood:         bot.Evolution.StolenFood,
			StolenOre:          bot.Evolution.StolenOre,
			CombatKills:        bot.Evolution.CombatKills,
			ControllerRaids:    bot.Evolution.ControllerRaids,
			DepotRaids:         bot.Evolution.DepotRaids,
			DepotBuilds:        bot.Evolution.DepotBuilds,
			SpawnerBuilds:      bot.Evolution.SpawnerBuilds,
			SpawnerBirths:      bot.Evolution.SpawnerBirths,
			DepotDepositedFood: bot.Evolution.DepotDepositedFood,
			DepotDepositedOre:  bot.Evolution.DepotDepositedOre,
			TaskCompletions:    bot.Evolution.TaskCompletions,
			ColonyLinked:       evolutionProfile.ColonyLinked,
			ActiveColony:       evolutionProfile.ActiveColony,
			ActiveNonSolo:      evolutionProfile.ActiveNonSoloColony,
			ConnectedMembers:   evolutionProfile.ConnectedMemberCount,
			X:                  bot.Pos.C,
			Y:                  bot.Pos.R,
			HasTask:            bot.HasTask(),
		}
		if bot.CooldownUntil.IsZero() {
			entry.CooldownLeft = 0
		} else {
			entry.CooldownLeft = time.Until(bot.CooldownUntil).Seconds()
			if entry.CooldownLeft < 0 {
				entry.CooldownLeft = 0
			}
		}

		if id, ok := colonyIDByRef[bot.Colony]; ok {
			idCopy := id
			entry.ColonyID = &idCopy
			foodBank := bot.Colony.FoodBank
			oreBank := bot.Colony.OreBank
			entry.ColonyFoodBank = &foodBank
			entry.ColonyOreBank = &oreBank
		}
		topSelector.Add(entry)
	}
	summary.SuccessfulDivisions = g.SuccessfulDivisions()
	summary.FoodGathered = g.FoodGathered()
	summary.OreGathered = g.OreGathered()
	summary.StolenFood = g.StolenFood()
	summary.StolenOre = g.StolenOre()
	summary.CombatKills = g.CombatKills()
	summary.ControllerRaids = g.ControllerRaids()
	summary.DepotRaids = g.DepotRaids()
	summary.SpawnerBirths = g.SpawnerBirths()
	summary.EliteCount = g.EliteCount()
	summary.BestScore = g.BestEvolutionScore()
	summary.TopBots = topSelector.Top()
	for _, top := range summary.TopBots {
		if top.ColonyLinked {
			summary.TopColonyLinkedBots++
		}
		if top.ActiveColony {
			summary.TopActiveColonyBots++
		}
		if botSummarySpawnerActive(top) {
			summary.TopSpawnerActiveBots++
		}
	}
	colonySizes := activeColonySizes(g.Board)
	summary.ActiveColonies = colonySizes.active
	summary.SoloActiveColonies = colonySizes.solo
	summary.MaxColonyMembers = colonySizes.maxMembers
	summary.MaxConnectedMembers = colonySizes.maxConnected
	summary.FriendlyAdjacencies, summary.ForeignAdjacencies = botAdjacencyCounts(g.Board)
	components := colonyComponentMetrics(g.Board)
	summary.MaxColonyComponent = components.maxColonyComponent
	summary.MaxConnectedComponent = components.maxConnectedComponent
	summary.LongestColonyRun = components.longestColonyRun
	summary.LongestConnectedRun = components.longestConnectedRun
	summary.ColonyTissueCells = colonyTissueCells(g.Board)
	summary.TopNonColonyDirectionShare = topNonColonyDirectionShare(g.Board)

	for _, cell := range *g.Board.GetGrid() {
		switch v := cell.(type) {
		case core.Wall:
			summary.Wall++
		case core.Controller:
			summary.Controller++
		case core.Farm:
			summary.FarmCount++
		case core.Spawner:
			summary.Spawners++
			summary.TotalSpawnerCharges += max(0, v.Amount)
		case core.Mine:
			summary.Mines++
		case core.Poison:
			summary.Poison++
		case core.Organics:
			summary.Organics++
		case core.Food:
			summary.Food++
		case core.Resource:
			summary.Resources++
		case core.Building:
			summary.Buildings++
		case core.Depot:
			summary.Depots++
			summary.TotalDepotFood += v.Food
			summary.TotalDepotOre += v.Ore
		case *core.Depot:
			if v != nil {
				summary.Depots++
				summary.TotalDepotFood += v.Food
				summary.TotalDepotOre += v.Ore
			}
		case core.Water:
			summary.Water++
		}
	}

	return summary
}

type colonySizeSummary struct {
	active       int
	solo         int
	maxMembers   int
	maxConnected int
}

func activeColonySizes(brd *core.Board) colonySizeSummary {
	if brd == nil {
		return colonySizeSummary{}
	}
	active := map[*core.Colony]struct{}{}
	for idx, cell := range *brd.GetGrid() {
		switch ctrl := cell.(type) {
		case core.Controller:
			if ctrl.Colony != nil {
				active[ctrl.Colony] = struct{}{}
			}
		case *core.Controller:
			if ctrl != nil && ctrl.Colony != nil {
				active[ctrl.Colony] = struct{}{}
			}
		case core.Depot:
			if ctrl.Colony != nil {
				active[ctrl.Colony] = struct{}{}
			}
		case *core.Depot:
			if ctrl != nil && ctrl.Colony != nil {
				active[ctrl.Colony] = struct{}{}
			}
		case core.Farm:
			if colony := colonyForSummaryOwnedCell(ctrl.Colony, ctrl.Owner); colony != nil {
				active[colony] = struct{}{}
			}
		case core.Spawner:
			if colony := colonyForSummaryOwner(ctrl.Owner); colony != nil {
				active[colony] = struct{}{}
			}
		case core.ColonyFlag:
			if colony := brd.PheromoneHomeOwnerAt(core.Position{R: idx / core.Cols, C: idx % core.Cols}); colony != nil {
				active[colony] = struct{}{}
			}
		}
	}

	members := map[*core.Colony]int{}
	connected := map[*core.Colony]int{}
	for _, id := range brd.ActiveBotIDs() {
		bot := brd.BotByID(id)
		if bot == nil || bot.Colony == nil {
			continue
		}
		if _, ok := active[bot.Colony]; !ok {
			continue
		}
		members[bot.Colony]++
		if bot.ConnnectedToColony {
			connected[bot.Colony]++
		}
	}

	out := colonySizeSummary{active: len(active)}
	for colony := range active {
		if members[colony] <= 1 {
			out.solo++
		}
		if members[colony] > out.maxMembers {
			out.maxMembers = members[colony]
		}
		if connected[colony] > out.maxConnected {
			out.maxConnected = connected[colony]
		}
	}
	return out
}

func botAdjacencyCounts(brd *core.Board) (friendly, foreign int) {
	if brd == nil {
		return 0, 0
	}
	for _, id := range brd.ActiveBotIDs() {
		bot := brd.BotByID(id)
		if bot == nil {
			continue
		}
		pos, ok := brd.BotPosition(id)
		if !ok {
			continue
		}
		for _, dir := range []core.Direction{core.Right, core.Up} {
			neighborPos := pos.AddDir(dir)
			if !core.Inside(neighborPos) {
				continue
			}
			other := brd.GetBot(neighborPos)
			if other == nil {
				continue
			}
			if core.BotsFriendly(bot, other) {
				friendly++
			} else {
				foreign++
			}
		}
	}
	return friendly, foreign
}

type colonyComponentSummary struct {
	maxColonyComponent    int
	maxConnectedComponent int
	longestColonyRun      int
	longestConnectedRun   int
}

func colonyComponentMetrics(brd *core.Board) colonyComponentSummary {
	return colonyComponentSummary{
		maxColonyComponent:    largestVisibleColonyComponent(brd, false),
		maxConnectedComponent: largestVisibleColonyComponent(brd, true),
		longestColonyRun:      longestColonyBotRun(brd, false),
		longestConnectedRun:   longestColonyBotRun(brd, true),
	}
}

func largestVisibleColonyComponent(brd *core.Board, connectedOnly bool) int {
	if brd == nil {
		return 0
	}
	visited := make([]bool, core.Rows*core.Cols)
	maxComponent := 0
	for idx := 0; idx < core.Rows*core.Cols; idx++ {
		if visited[idx] {
			continue
		}
		pos := core.Position{R: idx / core.Cols, C: idx % core.Cols}
		colony := visibleColonyAt(brd, pos, connectedOnly)
		if colony == nil {
			continue
		}
		stack := []core.Position{pos}
		visited[idx] = true
		size := 0
		for len(stack) > 0 {
			curr := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			size++
			for _, dir := range core.PosClock {
				next := curr.AddDir(dir)
				if !core.Inside(next) {
					continue
				}
				nextIdx := next.R*core.Cols + next.C
				if visited[nextIdx] {
					continue
				}
				if visibleColonyAt(brd, next, connectedOnly) != colony {
					continue
				}
				visited[nextIdx] = true
				stack = append(stack, next)
			}
		}
		if size > maxComponent {
			maxComponent = size
		}
	}
	return maxComponent
}

func visibleColonyAt(brd *core.Board, pos core.Position, connectedOnly bool) *core.Colony {
	if bot := brd.GetBot(pos); bot != nil {
		if bot.Colony == nil || (connectedOnly && !bot.ConnnectedToColony) {
			return nil
		}
		return bot.Colony
	}
	switch cell := brd.At(pos).(type) {
	case core.Controller:
		return cell.Colony
	case *core.Controller:
		if cell != nil {
			return cell.Colony
		}
	case core.Depot:
		return cell.Colony
	case *core.Depot:
		if cell != nil {
			return cell.Colony
		}
	case core.Farm:
		return colonyForSummaryOwnedCell(cell.Colony, cell.Owner)
	case core.Spawner:
		return colonyForSummaryOwner(cell.Owner)
	case core.ColonyFlag:
		return brd.PheromoneHomeOwnerAt(pos)
	}
	if brd.PheromoneAt(pos).Home > 0 {
		return brd.PheromoneHomeOwnerAt(pos)
	}
	return nil
}

func longestColonyBotRun(brd *core.Board, connectedOnly bool) int {
	if brd == nil {
		return 0
	}
	dirs := []core.Direction{
		core.Right,
		core.Up,
		{1, 1},
		{1, -1},
	}
	longest := 0
	for _, id := range brd.ActiveBotIDs() {
		bot := brd.BotByID(id)
		if !summaryBotMatchesComponent(bot, nil, connectedOnly) {
			continue
		}
		for _, dir := range dirs {
			prev := bot.Pos.AddDir(core.Direction{-dir[0], -dir[1]})
			if summaryBotMatchesComponent(brd.GetBot(prev), bot.Colony, connectedOnly) {
				continue
			}
			run := 0
			pos := bot.Pos
			for steps := 0; steps < max(core.Rows, core.Cols); steps++ {
				curr := brd.GetBot(pos)
				if !summaryBotMatchesComponent(curr, bot.Colony, connectedOnly) {
					break
				}
				run++
				pos = pos.AddDir(dir)
				if pos == bot.Pos {
					break
				}
				if !core.Inside(pos) {
					break
				}
			}
			if run > longest {
				longest = run
			}
		}
	}
	return longest
}

func summaryBotMatchesComponent(bot *core.Bot, colony *core.Colony, connectedOnly bool) bool {
	if bot == nil || bot.Colony == nil {
		return false
	}
	if colony != nil && bot.Colony != colony {
		return false
	}
	return !connectedOnly || bot.ConnnectedToColony
}

func colonyTissueCells(brd *core.Board) int {
	if brd == nil {
		return 0
	}
	count := 0
	for r := 0; r < core.Rows; r++ {
		for c := 0; c < core.Cols; c++ {
			pos := core.Position{R: r, C: c}
			if brd.PheromoneHomeOwnerAt(pos) != nil && brd.PheromoneAt(pos).Home > 0 {
				count++
			}
		}
	}
	return count
}

func topNonColonyDirectionShare(brd *core.Board) float64 {
	if brd == nil {
		return 0
	}
	var counts [8]int
	total := 0
	for _, id := range brd.ActiveBotIDs() {
		bot := brd.BotByID(id)
		if bot == nil || bot.Colony != nil {
			continue
		}
		total++
		counts[summaryDirectionIndex(bot.Dir)]++
	}
	if total == 0 {
		return 0
	}
	top := 0
	for _, count := range counts {
		if count > top {
			top = count
		}
	}
	return float64(top) / float64(total)
}

func summaryDirectionIndex(dir core.Direction) int {
	for i, candidate := range core.PosClock {
		if dir == candidate {
			return i
		}
	}
	return 0
}

func colonyForSummaryOwnedCell(colony *core.Colony, owner *core.Bot) *core.Colony {
	if colony != nil {
		return colony
	}
	return colonyForSummaryOwner(owner)
}

func colonyForSummaryOwner(owner *core.Bot) *core.Colony {
	if owner == nil {
		return nil
	}
	return owner.Colony
}

func botSummarySpawnerActive(bot botSummary) bool {
	return bot.SpawnerBuilds > 0 || bot.SpawnerBirths > 0
}

func nonColonySpawnerTopBots(top []botSummary) int {
	count := 0
	for _, bot := range top {
		if !bot.ColonyLinked && botSummarySpawnerActive(bot) {
			count++
		}
	}
	return count
}

type topBotSelector struct {
	limit int
	top   []botSummary
}

func newTopBotSelector(limit int) topBotSelector {
	return topBotSelector{
		limit: limit,
		top:   make([]botSummary, 0, limit),
	}
}

func (s *topBotSelector) Add(entry botSummary) {
	if s.limit <= 0 {
		return
	}

	pos := sort.Search(len(s.top), func(i int) bool {
		return botSummaryRanksBefore(entry, s.top[i])
	})
	if pos == len(s.top) {
		if len(s.top) < s.limit {
			s.top = append(s.top, entry)
		}
		return
	}

	if len(s.top) < s.limit {
		s.top = append(s.top, botSummary{})
		copy(s.top[pos+1:], s.top[pos:])
		s.top[pos] = entry
		return
	}

	copy(s.top[pos+1:], s.top[pos:len(s.top)-1])
	s.top[pos] = entry
}

func (s topBotSelector) Top() []botSummary {
	if s.top == nil {
		return []botSummary{}
	}
	return s.top
}

func botSummaryRanksBefore(a, b botSummary) bool {
	if a.EvolutionScore != b.EvolutionScore {
		return a.EvolutionScore > b.EvolutionScore
	}
	if a.Divisions != b.Divisions {
		return a.Divisions > b.Divisions
	}
	if a.LineageDepth != b.LineageDepth {
		return a.LineageDepth > b.LineageDepth
	}
	aBalanced := min(a.FoodInventory, a.OreInventory)
	bBalanced := min(b.FoodInventory, b.OreInventory)
	if aBalanced != bBalanced {
		return aBalanced > bBalanced
	}
	if a.Hp != b.Hp {
		return a.Hp > b.Hp
	}
	return a.Index < b.Index
}

func winningBot(top []botSummary) *botSummary {
	if len(top) == 0 {
		return nil
	}
	out := top[0]
	return &out
}

func winnerValue(bot *botSummary) int {
	if bot == nil {
		return 0
	}
	return bot.EvolutionScore
}

func normalizePositiveInt(value int) int {
	if value <= 0 {
		return 1
	}
	return value
}
