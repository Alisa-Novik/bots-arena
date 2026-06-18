package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
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
	defaultStatusTicks        = 20
	defaultTopBots            = 5
	defaultMatchTicks         = 300
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
	case "run", "seed-roulette":
		runSeedRoulette(args[1:])
		return true
	case "render":
		runRender(args[1:])
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

func runRender(args []string) {
	flags := commandFlagSet("render")
	seed := flags.Int64("seed", 1, "Deterministic PRNG seed.")
	ticks := flags.Int("ticks", defaultStatusTicks, "Simulation ticks to execute before rendering.")
	output := flags.String("output", "golab-render.png", "PNG output path.")
	cellSize := flags.Int("cell-size", 2, "Output pixels per board cell.")
	padding := flags.Int("padding", 0, "Outer image padding in pixels.")
	atlasPath := flags.String("atlas", "assests/sprites/atlas.png", "Sprite atlas path.")
	style := flags.String("style", "game", "Render style: game, atlas, or flat.")
	border := flags.Bool("border", false, "Draw a border around the board.")
	legend := flags.Bool("legend", false, "Draw a compact visual legend below the board.")
	pretty := flags.Bool("pretty", false, "Pretty-print JSON output.")
	usage := "render [--seed N] [--ticks N] [--output path] [--cell-size N] [--padding N] [--style game|atlas|flat] [--border=true|false] [--legend=true|false] [--pretty]"
	if err := parseCommandFlags(flags, args, usage); err != nil {
		os.Exit(2)
	}

	tickCount := normalizeNonNegativeInt(*ticks)
	gameRunner := newDeterministicGame(*seed)
	gameRunner.InitializeForCommands()
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
		"command":   "render",
		"seed":      *seed,
		"ticks":     tickCount,
		"output":    result.Output,
		"width":     result.Width,
		"height":    result.Height,
		"cell_size": normalizePositiveInt(*cellSize),
		"style":     *style,
	}
	printJSON(payload, *pretty)
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
		winnerDesc = fmt.Sprintf("Bot #%d — HP %d, Inv %d", winner.Index, winner.Hp, winner.Inventory)
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
	Index        int     `json:"index"`
	Hp           int     `json:"hp"`
	Inventory    int     `json:"inventory"`
	ColonyID     *int    `json:"colony_id"`
	X            int     `json:"x"`
	Y            int     `json:"y"`
	HasTask      bool    `json:"has_task"`
	CooldownLeft float64 `json:"cooldown_left_seconds"`
}

type matchSummary struct {
	Command     string       `json:"command"`
	Seed        int64        `json:"seed"`
	Ticks       int          `json:"ticks"`
	Timestamp   string       `json:"timestamp"`
	LiveBots    int          `json:"live_bots"`
	ColonyCount int          `json:"colony_count"`
	Controller  int          `json:"controllers"`
	FarmCount   int          `json:"farms"`
	Spawners    int          `json:"spawners"`
	Mines       int          `json:"mines"`
	Buildings   int          `json:"buildings"`
	Food        int          `json:"food"`
	Resources   int          `json:"resources"`
	Poison      int          `json:"poison"`
	Organics    int          `json:"organics"`
	Water       int          `json:"water"`
	Wall        int          `json:"wall"`
	TotalHP     int          `json:"total_bot_hp"`
	TotalInv    int          `json:"total_bot_inventory"`
	TopBots     []botSummary `json:"top_bots"`
}

func runMatchSummary(seed int64, ticks, topBots int) matchSummary {
	gameRunner := newDeterministicGame(seed)
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
	conf := config.NewConfig()
	conf.LogicStep = 0
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
		}
	}
	summary.ColonyCount = len(colonyIDByRef)

	botSummaries := make([]botSummary, 0, len(g.Board.Bots))
	for idx, bot := range g.Board.Bots {
		if bot == nil {
			continue
		}
		summary.LiveBots++
		summary.TotalHP += bot.Hp
		summary.TotalInv += bot.Inventory.Amount

		entry := botSummary{
			Index:     idx,
			Hp:        bot.Hp,
			Inventory: bot.Inventory.Amount,
			X:         bot.Pos.C,
			Y:         bot.Pos.R,
			HasTask:   bot.HasTask(),
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
		}
		botSummaries = append(botSummaries, entry)
	}

	sort.Slice(botSummaries, func(i, j int) bool {
		if botSummaries[i].Hp != botSummaries[j].Hp {
			return botSummaries[i].Hp > botSummaries[j].Hp
		}
		if botSummaries[i].Inventory != botSummaries[j].Inventory {
			return botSummaries[i].Inventory > botSummaries[j].Inventory
		}
		return botSummaries[i].Index < botSummaries[j].Index
	})

	limit := normalizeNonNegativeInt(topBots)
	if limit > len(botSummaries) {
		limit = len(botSummaries)
	}
	summary.TopBots = botSummaries[:limit]

	for _, cell := range *g.Board.GetGrid() {
		switch cell.(type) {
		case core.Wall:
			summary.Wall++
		case core.Controller:
			summary.Controller++
		case core.Farm:
			summary.FarmCount++
		case core.Spawner:
			summary.Spawners++
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
		case core.Water:
			summary.Water++
		}
	}

	return summary
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
	return bot.Hp + bot.Inventory
}

func normalizePositiveInt(value int) int {
	if value <= 0 {
		return 1
	}
	return value
}
