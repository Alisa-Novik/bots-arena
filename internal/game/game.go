package game

import (
	"errors"
	"fmt"
	"golab/internal/config"
	conf "golab/internal/config"
	"golab/internal/core"
	"golab/internal/tasking"
	"golab/internal/ui"
	"golab/internal/util"
	"math/rand"
	"sort"
	"time"
)

type Game struct {
	Board         *core.Board
	Colonies      []*core.Colony
	InitialGenome *core.Genome

	config *conf.Config
	State  *conf.GameState

	// aux
	maxHp                   int
	currGen                 int
	latestImprovement       int
	generationSeedGenome    core.Genome
	generationSeedRank      generationChampionRank
	hasGenerationSeedGenome bool
	eliteGenomes            []eliteGenome
	eliteImmigrantCursor    int

	gameMaster           GameMasterAdvisor
	gameMasterEnabled    bool
	gameMasterInterval   int
	logicTick            int
	successfulDivisions  int
	totalFoodGathered    int
	totalOreGathered     int
	totalStolenFood      int
	totalStolenOre       int
	totalCombatKills     int
	totalControllerRaids int
	totalDepotRaids      int
	totalSpawnerBirths   int
	selectedColony       *core.Colony
	botIterationIDs      []core.BotID
	envIterationCells    []int
	tpsWindowStart       time.Time
	tpsWindowTick        int
	scaleMode            bool
}

const (
	controllerCrowdRadius    = 8
	controllerCrowdHpTax     = 1
	controllerBuildMinRadius = 16
	controllerRecruitRadius  = 12
	interactiveFrameInterval = time.Second / 30
	ControllerRaidFoodLimit  = 5
	ControllerRaidOreLimit   = 10
	smartImmigrationTarget   = 200
	nonColonyScoreCap        = 60000
	spawnerNonColonyScoreCap = 260000
	soloColonyScoreCap       = 95000
	colonyEliteCohortSize    = 32
)

type biomeSpawnProfile struct {
	PoisonChance   int
	ResourceChance int
	FoodChance     int
	ResourceAmount int
}

type BotEvolutionProfile struct {
	ColonyLinked          bool
	ActiveColony          bool
	ActiveNonSoloColony   bool
	ActiveControllerOwner bool
	ColonyMemberCount     int
	ConnectedMemberCount  int
}

type generationChampionRank struct {
	score             int
	divisions         int
	lineageDepth      int
	balancedInventory int
	hp                int
	boardIdx          int
	colonyLinked      bool
	activeNonSolo     bool
	connectedMembers  int
}

type eliteGenome struct {
	genome core.Genome
	rank   generationChampionRank
}

func NewGame(config *conf.Config) *Game {
	useInitialGenome := config.UseInitialGenome
	return &Game{
		Board:         core.NewBoard(),
		InitialGenome: core.GetInitialGenome(useInitialGenome),
		config:        config,
		State:         &conf.GameState{LastLogic: time.Now()},
		maxHp:         0,
		currGen:       0,
		gameMaster:    NewMockGameMaster(),
	}
}

func (g *Game) Initialize() {
	g.initialBotsGeneration()
	g.generateWater()
	g.populateBoard()
	g.config.LiveBots = g.liveBotCount()
	g.State.LastLogic = time.Now()
}

func (g *Game) ResetSimulation() {
	g.Board = core.NewBoard()
	g.Colonies = nil
	g.InitialGenome = core.GetInitialGenome(g.config.UseInitialGenome)
	g.maxHp = 0
	g.currGen = 0
	g.latestImprovement = 0
	g.generationSeedGenome = core.Genome{}
	g.generationSeedRank = generationChampionRank{}
	g.hasGenerationSeedGenome = false
	g.eliteGenomes = nil
	g.eliteImmigrantCursor = 0
	g.logicTick = 0
	g.successfulDivisions = 0
	g.totalFoodGathered = 0
	g.totalOreGathered = 0
	g.totalStolenFood = 0
	g.totalStolenOre = 0
	g.totalCombatKills = 0
	g.totalControllerRaids = 0
	g.totalDepotRaids = 0
	g.totalSpawnerBirths = 0
	g.selectedColony = nil
	g.tpsWindowStart = time.Time{}
	g.tpsWindowTick = 0
	g.config.LiveBots = 0
	g.State = &conf.GameState{LastLogic: time.Now()}
	if g.gameMasterEnabled {
		g.State.GameMaster = conf.GameMasterState{
			Enabled:  true,
			Name:     g.gameMaster.AdvisorName(),
			Interval: g.gameMasterInterval,
		}
	}
	g.Initialize()
}

func (g *Game) RunHeadless(ticks int) {
	fmt.Println("Running headless simulation...")
	g.Initialize()
	if ticks > 0 {
		g.RunHeadlessFrames(ticks)
		fmt.Printf("Headless simulation complete: ticks=%d live_bots=%d\n", ticks, g.liveBotCount())
		return
	}

	for {
		g.runLogicTick()
	}
}

func (g *Game) InitializeForCommands() {
	g.Initialize()
}

func (g *Game) InitializeForScale(targetBots int, seed int64) error {
	if targetBots < 0 {
		return errors.New("target bot count must be non-negative")
	}
	g.scaleMode = true
	previousBotChance := g.config.BotChance
	g.config.BotChance = 0
	g.Initialize()
	g.config.BotChance = previousBotChance
	if err := g.SeedTargetBots(targetBots, seed); err != nil {
		return err
	}
	return nil
}

func (g *Game) SeedTargetBots(targetBots int, seed int64) error {
	if targetBots < 0 {
		return errors.New("target bot count must be non-negative")
	}
	candidates := make([]int, 0, util.Cells)
	for cellIdx := 0; cellIdx < util.Cells; cellIdx++ {
		pos := util.PosOf(cellIdx)
		if g.Board.IsWall(pos) || g.Board.IsFrozen(pos) || !g.Board.IsEmpty(pos) || g.Board.GetBot(pos) != nil {
			continue
		}
		candidates = append(candidates, cellIdx)
	}
	if targetBots > len(candidates) {
		return fmt.Errorf("target bot count %d exceeds empty spawn capacity %d", targetBots, len(candidates))
	}

	rng := rand.New(rand.NewSource(seed ^ 0x5eed5eed))
	rng.Shuffle(len(candidates), func(i, j int) {
		candidates[i], candidates[j] = candidates[j], candidates[i]
	})
	for _, cellIdx := range candidates[:targetBots] {
		pos := util.PosOf(cellIdx)
		b := core.NewBot(pos)
		if g.scaleMode {
			b.Genome = core.Genome{}
		} else if g.InitialGenome != nil {
			b.Genome = *g.InitialGenome
		}
		b.Hp = 500
		g.Board.AddBot(pos, &b)
	}
	g.config.LiveBots = g.liveBotCount()
	return nil
}

func (g *Game) RunHeadlessFrames(count int) {
	for range count {
		g.runLogicTick()
	}
}

func (g *Game) SuccessfulDivisions() int {
	return g.successfulDivisions
}

func (g *Game) FoodGathered() int {
	return g.totalFoodGathered
}

func (g *Game) OreGathered() int {
	return g.totalOreGathered
}

func (g *Game) StolenFood() int {
	return g.totalStolenFood
}

func (g *Game) StolenOre() int {
	return g.totalStolenOre
}

func (g *Game) CombatKills() int {
	return g.totalCombatKills
}

func (g *Game) ControllerRaids() int {
	return g.totalControllerRaids
}

func (g *Game) DepotRaids() int {
	return g.totalDepotRaids
}

func (g *Game) SpawnerBirths() int {
	return g.totalSpawnerBirths
}

func (g *Game) BotEvolutionScore(bot *core.Bot) int {
	return g.botEvolutionScoreWithProfile(bot, g.BotEvolutionProfile(bot))
}

func (g *Game) BotEvolutionProfile(bot *core.Bot) BotEvolutionProfile {
	profile := BotEvolutionProfile{}
	if bot == nil || bot.Colony == nil {
		return profile
	}

	colony := bot.Colony
	profile.ColonyLinked = true
	for _, member := range colony.Members {
		if member == nil || member.Colony != colony {
			continue
		}
		if g.Board != nil && g.Board.GetBot(member.Pos) != member {
			continue
		}
		profile.ColonyMemberCount++
		if member.ConnnectedToColony {
			profile.ConnectedMemberCount++
		}
	}

	if active, owner := g.colonyCenterController(colony); active {
		profile.ActiveColony = true
		profile.ActiveControllerOwner = owner == bot
	}
	if profile.ConnectedMemberCount > 0 {
		profile.ActiveColony = true
	}
	profile.ActiveNonSoloColony = profile.ActiveColony && profile.ColonyMemberCount > 1
	return profile
}

func (g *Game) botEvolutionScoreWithProfile(bot *core.Bot, profile BotEvolutionProfile) int {
	if bot == nil {
		return 0
	}
	score := bot.EvolutionScore()
	score += bot.Evolution.ControllerBuilds * 8000
	score += bot.Evolution.DepotBuilds * 7000
	score += bot.Evolution.SpawnerBuilds * 6000
	score += bot.Evolution.SpawnerBirths * 9000
	score += min(bot.Evolution.DepotDepositedFood*120+bot.Evolution.DepotDepositedOre*75, 14000)
	score += bot.Evolution.TaskCompletions * 2500
	score += bot.Evolution.SuccessfulDivisions * 1000

	switch {
	case profile.ActiveNonSoloColony:
		score += 55000
		score += min(profile.ColonyMemberCount, 64) * 2500
		score += min(profile.ConnectedMemberCount, 64) * 10000
		if bot.ConnnectedToColony {
			score += 35000
		}
		if profile.ActiveControllerOwner {
			score += 18000
		}
	case profile.ColonyLinked:
		score += 12000
		if profile.ActiveColony {
			score += 20000
		}
		if bot.ConnnectedToColony {
			score += 12000
		}
		score += min(profile.ConnectedMemberCount, 8) * 3000
		score = min(score, soloColonyScoreCap)
	default:
		if botSpawnerActive(bot) {
			score += 25000
			score = min(score, spawnerNonColonyScoreCap)
		} else {
			score = min(score, nonColonyScoreCap)
		}
	}

	if score < 0 {
		return 0
	}
	return score
}

func botSpawnerActive(bot *core.Bot) bool {
	return bot != nil && (bot.Evolution.SpawnerBuilds > 0 || bot.Evolution.SpawnerBirths > 0)
}

func (g *Game) colonyCenterController(colony *core.Colony) (bool, *core.Bot) {
	if g == nil || g.Board == nil || colony == nil {
		return false, nil
	}
	switch ctrl := g.Board.At(colony.Center).(type) {
	case core.Controller:
		if ctrl.Colony == colony && g.controllerOwnerAlive(&ctrl) {
			return true, ctrl.Owner
		}
	case *core.Controller:
		if ctrl != nil && ctrl.Colony == colony && g.controllerOwnerAlive(ctrl) {
			return true, ctrl.Owner
		}
	}
	return false, nil
}

func (g *Game) EliteCount() int {
	return len(g.eliteGenomes)
}

func (g *Game) BestEvolutionScore() int {
	if len(g.eliteGenomes) > 0 {
		return g.eliteGenomes[0].rank.score
	}
	if g.hasGenerationSeedGenome {
		return g.generationSeedRank.score
	}
	return 0
}

func (g *Game) EnableGameMaster(interval int) {
	if interval <= 0 {
		interval = 1
	}
	g.gameMaster = NewMockGameMaster()
	g.gameMasterInterval = interval
	g.gameMasterEnabled = true
	g.State.GameMaster = conf.GameMasterState{
		Enabled:  true,
		Name:     g.gameMaster.AdvisorName(),
		Interval: interval,
	}
}

func (g *Game) EnableExternalGameMaster(interval int, command []string, timeout time.Duration) {
	if interval <= 0 {
		interval = 1
	}
	g.gameMaster = NewExternalGameMaster(command, timeout, NewMockGameMaster())
	g.gameMasterInterval = interval
	g.gameMasterEnabled = true
	g.State.GameMaster = conf.GameMasterState{
		Enabled:  true,
		Name:     g.gameMaster.AdvisorName(),
		Interval: interval,
	}
}

func (g *Game) environmentActions() {
	grid := *g.Board.GetGrid()
	g.envIterationCells = g.Board.SortedActiveEnvironmentCells(g.envIterationCells[:0])
	for _, cellIdx := range g.envIterationCells {
		if cellIdx < 0 || cellIdx >= len(grid) {
			continue
		}
		if g.Board.IsFrozenIdx(cellIdx) {
			continue
		}
		cell := grid[cellIdx]
		switch v := cell.(type) {
		case core.Organics:
			pos := util.PosOf(cellIdx)
			if v.Amount <= 1 {
				g.Board.Clear(pos)
				continue
			}
			v.Amount--
			g.Board.Set(pos, v)
			continue
		case core.Controller:
			pos := util.PosOf(cellIdx)
			g.handleController(&v, pos)
			if g.Board.At(pos) != nil {
				g.Board.Set(pos, v)
			}
			continue
		case *core.Controller:
			if v == nil {
				continue
			}
			pos := util.PosOf(cellIdx)
			g.handleController(v, pos)
			if g.Board.At(pos) != nil {
				g.Board.Set(pos, *v)
			}
			continue
		case core.Depot:
			pos := util.PosOf(cellIdx)
			g.handleDepot(&v, pos)
			if g.Board.At(pos) != nil {
				g.Board.Set(pos, v)
			}
			continue
		case *core.Depot:
			if v == nil {
				continue
			}
			pos := util.PosOf(cellIdx)
			g.handleDepot(v, pos)
			if g.Board.At(pos) != nil {
				g.Board.Set(pos, *v)
			}
			continue
		case core.Farm:
			if v.Amount <= 0 {
				continue
			}
			pos := util.PosOf(cellIdx)
			produced := 0
			for range g.farmFoodOutputs(pos, v) {
				foodPos, ok := g.Board.FindEmptyPosAround(pos)
				if !ok {
					break
				}
				if g.Board.IsFrozen(foodPos) {
					continue
				}
				g.Board.Set(foodPos, core.Food{Pos: foodPos, Amount: 1})
				g.emitEventPheromone(foodPos, core.PheromoneFood)
				produced++
			}
			if produced > 0 {
				v.Amount--
				g.Board.Set(pos, v)
				g.emitEventPheromone(pos, core.PheromoneFood)
			}
			continue
		case core.Spawner:
			pos := util.PosOf(cellIdx)
			if g.tryColonySpawnerAutoBirth(pos, &v) {
				g.Board.Set(pos, v)
			}
			continue
		case core.Poison:
			g.emitEventPheromone(util.PosOf(cellIdx), core.PheromoneDanger)
			continue
		}
	}
	g.regrowBiomeFoodForTick()
}

func (g *Game) regrowBiomeFoodForTick() {
	period := g.config.FertileFoodRegrowPeriod
	if period <= 0 {
		return
	}
	start := positiveModulo(-g.logicTick*7919, period)
	for cellIdx := start; cellIdx < util.Cells; cellIdx += period {
		g.regrowBiomeFood(cellIdx)
	}
}

func positiveModulo(value, mod int) int {
	if mod <= 0 {
		return 0
	}
	value %= mod
	if value < 0 {
		value += mod
	}
	return value
}

func (g *Game) regrowBiomeFood(cellIdx int) {
	if g.config.FertileFoodRegrowPeriod <= 0 || g.Board.BiomeAtIdx(cellIdx) != core.BiomeFertile {
		return
	}
	if (cellIdx+g.logicTick*7919)%g.config.FertileFoodRegrowPeriod != 0 {
		return
	}
	pos := util.PosOf(cellIdx)
	g.Board.Set(pos, core.Food{Pos: pos, Amount: 1})
	g.emitEventPheromone(pos, core.PheromoneFood)
}

func (g *Game) killBot(b *core.Bot, botIdx int) {
	// if b.Path != nil {
	// 	b.Path = nil
	// }
	if b.CurrTask != nil {
		b.CurrTask.Owner = nil
	}
	if c := b.Colony; c != nil {
		c.RemoveMember(b)
	}
	if p := b.Parent; p != nil {
		p.RemoveOffspring(b)
	}
	g.Board.RemoveBotAt(util.PosOf(botIdx))
	*b = core.Bot{}
}

func (g *Game) handleController(ctrl *core.Controller, pos util.Position) {
	c := ctrl.Colony
	if c == nil {
		g.Board.Clear(pos)
		return
	}

	if !g.ensureControllerOwner(ctrl) {
		for _, m := range c.Members {
			m.DisconnectFromColony()
		}
		for _, f := range c.Flags {
			g.Board.Clear(f.Pos)
		}
		g.Board.Clear(ctrl.Pos)
		return
	}

	g.emitControllerHomePheromones(ctrl, pos)
	g.recruitFriendlyBots(c, pos, controllerRecruitRadius)
	g.claimNearbyColonyFarms(c, pos, controllerRecruitRadius)
	g.connectNearbyColonyMembers(c, pos, controllerRecruitRadius)
	g.refreshColonySpawnerGenome(c)

	for _, d := range core.Dirs {
		g.connectBots(pos.AddDir(d), map[*core.Bot]struct{}{}, ctrl.Colony)
	}
	g.depositConnectedMemberSurplusToBank(c, pos)

	if g.colonyOrganismEnabled() {
		g.releaseColonyWaterBridgeTasks(c)
		g.processColonyRoleTasks(c, pos)
	}

	for _, m := range c.Members {
		c.HealMember(m, ctrl)
		if task := m.CurrTask; task != nil {
			if !task.IsDone {
				m.SetColor(util.CyanColor(), g.Board.MarkDirty)
			} else {
				m.SetColor(util.GreenColor(), g.Board.MarkDirty)
			}
		}
	}

	c.HealBotsInFlagRadius(5, g.calcHpChange(), ctrl)
	g.applyControllerCrowdingPressure(ctrl, pos)

	if g.colonyOrganismEnabled() {
		return
	}

	if len(c.WaterPositions) > 0 && !c.HasTaskOfType(core.ConnectToPosTask) {
		task := c.NewConnectionTask(c.WaterPositions[0])
		c.AddTask(task)
	}

	tasking.ProcessColonyTasks(ctrl, g.Board)
}

func (g *Game) releaseColonyWaterBridgeTasks(colony *core.Colony) {
	if colony == nil {
		return
	}
	now := time.Now()
	tasks := colony.Tasks[:0]
	for _, task := range colony.Tasks {
		if task == nil {
			continue
		}
		if task.Type != core.ConnectToPosTask && task.Type != core.MaintainConnectionTask {
			tasks = append(tasks, task)
			continue
		}
		if owner := task.Owner; owner != nil {
			if owner.CurrTask == task {
				owner.CurrTask = nil
				if colony.AssignedTasksCount > 0 {
					colony.AssignedTasksCount--
				}
				owner.StartCooldown(now)
				if g != nil && g.Board != nil {
					g.Board.MarkDirty(util.Idx(owner.Pos))
				}
			}
			task.Owner = nil
		}
	}
	colony.Tasks = tasks
	colony.SetPathToWater(nil)
	colony.WaterPathFlowField = nil
}

func (g *Game) handleDepot(depot *core.Depot, pos util.Position) {
	if depot == nil || depot.Colony == nil {
		return
	}
	depot.Pos = pos
	if g.depositConnectedMemberSurplusToDepot(depot, pos) {
		g.emitHomePheromone(pos, depot.Colony, g.config.PheromoneHomeDeposit/4)
	}
}

func (g *Game) controllerOwnerAlive(ctrl *core.Controller) bool {
	if g.liveControllerOwner(ctrl) {
		return true
	}
	return ctrl != nil && ctrl.Colony != nil && g.liveColonyMember(ctrl.Colony) != nil
}

func (g *Game) ensureControllerOwner(ctrl *core.Controller) bool {
	if ctrl == nil || ctrl.Colony == nil {
		return false
	}
	if g.liveControllerOwner(ctrl) {
		return true
	}
	replacement := g.liveColonyMember(ctrl.Colony)
	if replacement == nil {
		return false
	}
	ctrl.Owner = replacement
	return true
}

func (g *Game) liveControllerOwner(ctrl *core.Controller) bool {
	if ctrl == nil || ctrl.Owner == nil || ctrl.Colony == nil {
		return false
	}
	owner := ctrl.Owner
	return owner.Colony == ctrl.Colony && g.Board.GetBot(owner.Pos) == owner
}

func (g *Game) liveColonyMember(colony *core.Colony) *core.Bot {
	if colony == nil {
		return nil
	}
	for _, m := range colony.Members {
		if m == nil || m.Colony != colony {
			continue
		}
		if g.Board.GetBot(m.Pos) == m {
			return m
		}
	}
	return nil
}

func (g *Game) liveColonyMembers(colony *core.Colony) []*core.Bot {
	if colony == nil {
		return nil
	}
	members := make([]*core.Bot, 0, len(colony.Members))
	for _, m := range colony.Members {
		if m == nil || m.Colony != colony {
			continue
		}
		if g.Board.GetBot(m.Pos) == m {
			members = append(members, m)
		}
	}
	return members
}

func (g *Game) forEachLiveBotInRadius(center core.Position, radius int, visit func(*core.Bot) bool) {
	if g == nil || g.Board == nil || radius < 0 || visit == nil {
		return
	}
	for r := center.R - radius; r <= center.R+radius; r++ {
		if r < 0 || r >= core.Rows {
			continue
		}
		for dc := -radius; dc <= radius; dc++ {
			pos := util.NewPos(r, center.C+dc)
			bot := g.Board.GetBot(pos)
			if bot == nil {
				continue
			}
			if !visit(bot) {
				return
			}
		}
	}
}

func (g *Game) liveLineageRoot(bot *core.Bot) *core.Bot {
	if bot == nil {
		return nil
	}
	root := bot
	for root.Parent != nil && g.Board.GetBot(root.Parent.Pos) == root.Parent {
		root = root.Parent
	}
	return root
}

func (g *Game) recruitFriendlyBots(colony *core.Colony, center util.Position, radius int) int {
	if colony == nil || g == nil || g.Board == nil || radius < 0 {
		return 0
	}
	seeds := g.liveColonyMembers(colony)
	if len(seeds) == 0 {
		return 0
	}
	recruited := 0
	g.forEachLiveBotInRadius(center, radius, func(candidate *core.Bot) bool {
		if candidate == nil || candidate.Colony != nil {
			return true
		}
		for _, member := range seeds {
			if !core.BotsFriendly(member, candidate) {
				continue
			}
			colony.AddFamily(candidate)
			seeds = append(seeds, candidate)
			recruited++
			break
		}
		return true
	})
	return recruited
}

func (g *Game) connectNearbyColonyMembers(colony *core.Colony, center util.Position, radius int) int {
	if colony == nil || g == nil || g.Board == nil || radius < 0 {
		return 0
	}
	connected := 0
	g.forEachLiveBotInRadius(center, radius, func(member *core.Bot) bool {
		if member.Colony != colony {
			return true
		}
		if !member.ConnnectedToColony {
			member.ConnnectedToColony = true
			g.Board.MarkDirty(util.Idx(member.Pos))
		}
		connected++
		return true
	})
	return connected
}

func (g *Game) claimNearbyColonyFarms(colony *core.Colony, center util.Position, radius int) int {
	if colony == nil || radius < 0 {
		return 0
	}
	claimed := 0
	for r := center.R - radius; r <= center.R+radius; r++ {
		if r < 0 || r >= core.Rows {
			continue
		}
		for dc := -radius; dc <= radius; dc++ {
			pos := util.NewPos(r, center.C+dc)
			farm, ok := g.Board.At(pos).(core.Farm)
			if !ok || farm.Colony == colony || farm.Colony != nil {
				continue
			}
			if farm.Owner == nil || farm.Owner.Colony != colony {
				continue
			}
			farm.Colony = colony
			g.Board.Set(pos, farm)
			claimed++
		}
	}
	return claimed
}

func (g *Game) applyControllerCrowdingPressure(ctrl *core.Controller, pos util.Position) {
	if ctrl == nil || ctrl.Colony == nil || g == nil || g.Board == nil {
		return
	}
	threshold := g.config.ControllerCrowdThreshold
	if threshold <= 0 {
		return
	}
	nearby := make([]*core.Bot, 0, threshold+1)
	g.forEachLiveBotInRadius(pos, controllerCrowdRadius, func(m *core.Bot) bool {
		if m.Colony == ctrl.Colony {
			nearby = append(nearby, m)
		}
		return true
	})
	if len(nearby) <= threshold {
		return
	}
	for _, m := range nearby {
		m.Hp = max(1, m.Hp-controllerCrowdHpTax)
		g.Board.MarkDirty(util.Idx(m.Pos))
	}
}

func (g *Game) connectBots(currPos util.Position, visited map[*core.Bot]struct{}, colony *core.Colony) bool {
	if util.OutOfBounds(currPos) {
		return false
	}
	b := g.Board.GetBot(currPos)
	if b == nil || b.Colony != colony {
		return false
	}
	if _, yes := visited[b]; yes {
		return true
	}

	b.ConnnectedToColony = true
	visited[b] = struct{}{}

	isOnBorder := false
	for _, d := range core.Dirs {
		if !g.connectBots(currPos.AddDir(d), visited, colony) {
			isOnBorder = true
		}
	}
	if b.CurrTask == nil {
		if g.config.ColoringStrategy == config.ColonyConnectionColoring {
			if isOnBorder {
				b.Color = blendColor(b.Color, b.Colony.Color, 0.65)
			} else {
				b.Color = b.Colony.Color
			}
		}
	}
	return true
}

func (g *Game) Run() {
	fmt.Println("Running simulation...")
	g.ResetSimulation()
	ui.SetBoard(g.Board)
	ui.SetGameState(g.State)
	ui.SetGodActions(g)
	ui.BuildStaticLayer(g.Board)

	for !ui.Window.ShouldClose() {
		frameStart := time.Now()
		if ui.ConsumeSimulationResetRequest() {
			g.ResetSimulation()
			ui.SetBoard(g.Board)
			ui.SetGameState(g.State)
			ui.SetGodActions(g)
			ui.BuildStaticLayer(g.Board)
			ui.MarkSimulationResetComplete()
		}
		if g.config.Pause {
			ui.DrawGrid(g.Board, g.Board.Bots)
			sleepUntilNextInteractiveFrame(frameStart)
			continue
		}
		g.step()
		ui.DrawGrid(g.Board, g.Board.Bots)
		sleepUntilNextInteractiveFrame(frameStart)
	}
}

func sleepUntilNextInteractiveFrame(frameStart time.Time) {
	if remaining := interactiveFrameInterval - time.Since(frameStart); remaining > 0 {
		time.Sleep(remaining)
	}
}

func (g *Game) generateWater() {
	if g.config.OceansCount <= 0 {
		return
	}
	for groupID := 0; groupID < g.config.OceansCount; groupID++ {
		g.generateWaterBody(groupID)
	}
}

func (g *Game) generateWaterBody(groupID int) {
	center := core.NewRandomPosition()
	if center.R <= 1 || center.R >= core.Rows-2 {
		center.R = 2 + rand.Intn(core.Rows-4)
	}

	isRiver := rand.Intn(100) >= 35
	steps := 16 + rand.Intn(18)
	radius := 2 + rand.Intn(3)
	if isRiver {
		steps = 34 + rand.Intn(38)
		radius = 1 + rand.Intn(2)
	}

	dirIdx := rand.Intn(len(core.PosClock))
	for step := 0; step < steps; step++ {
		stampRadius := radius
		if rand.Intn(100) < 28 {
			stampRadius++
		}
		if !isRiver && rand.Intn(100) < 20 {
			stampRadius++
		}
		g.stampWaterBrush(center, groupID, stampRadius)

		turn := rand.Intn(3) - 1
		if !isRiver {
			turn = rand.Intn(5) - 2
		}
		dirIdx = (dirIdx + turn + len(core.PosClock)) % len(core.PosClock)

		stride := 1
		if isRiver && rand.Intn(100) < 35 {
			stride = 2
		}
		for range stride {
			next := center.AddDir(core.PosClock[dirIdx])
			if next.R <= 0 || next.R >= core.Rows-1 {
				dirIdx = (dirIdx + 4) % len(core.PosClock)
				next = center.AddDir(core.PosClock[dirIdx])
			}
			if next.R > 0 && next.R < core.Rows-1 {
				center = next
			}
		}
	}
}

func (g *Game) stampWaterBrush(center core.Position, groupID, radius int) {
	if radius < 1 {
		radius = 1
	}
	inner := max(0, radius-1)
	inner2 := inner * inner
	outer2 := radius * radius
	for dr := -radius; dr <= radius; dr++ {
		for dc := -radius; dc <= radius; dc++ {
			dist2 := dr*dr + dc*dc
			if dist2 > outer2 {
				continue
			}
			if dist2 > inner2 && rand.Intn(100) < 35 {
				continue
			}
			pos := center.AddRowCol(dr, dc)
			if !g.Board.IsWall(pos) {
				g.Board.Set(pos, core.Water{GroupId: groupID, Amount: 10000})
			}
		}
	}
}

func (g *Game) step() {
	var maxLogicPerFrame int
	if g.config.LiveBots < 2000 {
		maxLogicPerFrame = 64
	} else if g.config.LiveBots < 5000 {
		maxLogicPerFrame = 32
	} else if g.config.LiveBots < 55000 {
		maxLogicPerFrame = 16
	} else {
		maxLogicPerFrame = 8
	}

	for executed := 0; executed < maxLogicPerFrame &&
		time.Since(g.State.LastLogic) >= g.config.LogicStep; executed++ {

		g.runLogicTick()
		g.State.LastLogic = g.State.LastLogic.Add(g.config.LogicStep)
	}
}

func (g *Game) runLogicTick() {
	g.logicTick++
	liveBots, champion := g.liveBotCountAndGenerationChampion()
	g.config.LiveBots = liveBots
	if champion != nil {
		g.rememberGenerationChampion(champion)
	}
	switch n := g.config.LiveBots; {
	case n == 0:
		g.currGen++
		g.initialBotsGeneration()
		g.populateBoard()
	default:
		g.botsActions()
		g.environmentActions()
		liveAfterActions := g.liveBotCount()
		if liveAfterActions <= g.immigrationThreshold() {
			g.spawnRandomImmigrants(g.immigrationThreshold() - liveAfterActions)
		}
	}
	g.config.LiveBots = g.liveBotCount()
	g.updatePheromones()
	g.runGameMasterTick()
	g.updateLogicRate()
}

func (g *Game) updateLogicRate() {
	if g.State == nil {
		return
	}
	g.State.LogicTick = g.logicTick
	now := time.Now()
	if g.tpsWindowStart.IsZero() {
		g.tpsWindowStart = now
		g.tpsWindowTick = g.logicTick
		return
	}
	ticks := g.logicTick - g.tpsWindowTick
	if ticks < 10 {
		return
	}
	elapsed := now.Sub(g.tpsWindowStart)
	if elapsed <= 0 {
		return
	}
	g.State.LogicTicksPerSecond = float64(ticks) / elapsed.Seconds()
	g.tpsWindowStart = now
	g.tpsWindowTick = g.logicTick
}

func (g *Game) spawnRandomImmigrants(maxSpawn int) int {
	count := g.immigrationCount()
	if maxSpawn >= 0 && count > maxSpawn {
		count = maxSpawn
	}
	if count <= 0 || g.config.ImmigrationInterval <= 0 || g.logicTick%g.config.ImmigrationInterval != 0 {
		return 0
	}
	spawned := 0
	attempts := count * 100
	for attempts > 0 && spawned < count {
		attempts--
		spawned += g.spawnRandomImmigrantAtWithBudget(core.NewRandomPosition(), count-spawned)
	}
	for cellIdx := 0; spawned < count && cellIdx < core.Rows*core.Cols; cellIdx++ {
		spawned += g.spawnRandomImmigrantAtWithBudget(util.PosOf(cellIdx), count-spawned)
	}
	return spawned
}

func (g *Game) immigrationThreshold() int {
	threshold := g.config.NewGenThreshold
	if g.smartEvolutionEnabled() {
		threshold = max(threshold, smartImmigrationTarget)
	}
	return threshold
}

func (g *Game) immigrationCount() int {
	count := g.config.ImmigrationBots
	if g.smartEvolutionEnabled() && len(g.eliteGenomes) > 0 {
		count = max(count, min(32, max(colonyEliteCohortSize, g.evolutionEliteLimit())))
	}
	return count
}

func (g *Game) spawnRandomImmigrantAt(pos core.Position) bool {
	return g.spawnRandomImmigrantAtWithBudget(pos, 1) > 0
}

func (g *Game) spawnRandomImmigrantAtWithBudget(pos core.Position, budget int) int {
	if budget <= 0 {
		return 0
	}
	if g.Board.IsFrozen(pos) || !g.Board.IsEmpty(pos) || g.Board.GetBot(pos) != nil || g.Board.IsWall(pos) {
		return 0
	}
	b, elite, hasElite := g.newImmigrantBotWithElite(pos)
	g.Board.AddBot(pos, &b)
	spawned := 1
	if hasElite && elite.rank.colonyLinked && budget > spawned {
		spawned += g.seedEliteCohortAround(pos, elite, min(colonyEliteCohortSize-1, budget-spawned))
	}
	return spawned
}

func (g *Game) newImmigrantBot(pos core.Position) core.Bot {
	b, _, _ := g.newImmigrantBotWithElite(pos)
	return b
}

func (g *Game) newImmigrantBotWithElite(pos core.Position) (core.Bot, eliteGenome, bool) {
	b := core.NewBot(pos)
	if !g.smartEvolutionEnabled() || len(g.eliteGenomes) == 0 {
		return b, eliteGenome{}, false
	}
	eliteIdx := g.eliteImmigrantCursor % len(g.eliteGenomes)
	elite := g.preferredEliteSeed(eliteIdx)
	if g.eliteImmigrantCursor < len(g.eliteGenomes) {
		b.Genome = elite.genome
	} else {
		b.Genome = core.NewMutatedGenomeWithRate(elite.genome, g.eliteMutationRate(eliteIdx))
	}
	g.eliteImmigrantCursor++
	return b, elite, true
}

func (g *Game) liveBotCountAndGenerationChampion() (int, *core.Bot) {
	count := 0
	var best *core.Bot
	g.botIterationIDs = g.sortedActiveBotIDs(g.botIterationIDs[:0])
	for _, id := range g.botIterationIDs {
		bot := g.Board.BotByID(id)
		if bot == nil {
			continue
		}
		count++
		if g.smartEvolutionEnabled() && !g.scaleMode {
			g.rememberEliteCandidate(bot)
		}
		if g.generationChampionRanksBefore(bot, best) {
			best = bot
		}
	}
	return count, best
}

func (g *Game) generationChampionRanksBefore(candidate, current *core.Bot) bool {
	if current == nil {
		return true
	}
	return generationChampionRankBefore(
		g.generationChampionRankForBot(candidate),
		g.generationChampionRankForBot(current),
	)
}

func generationChampionRanksBefore(candidate, current *core.Bot) bool {
	if current == nil {
		return true
	}
	return generationChampionRankBefore(
		generationChampionRankForBot(candidate),
		generationChampionRankForBot(current),
	)
}

func (g *Game) generationChampionRankForBot(bot *core.Bot) generationChampionRank {
	if bot == nil {
		return generationChampionRank{boardIdx: util.Cells}
	}
	profile := g.BotEvolutionProfile(bot)
	return generationChampionRank{
		score:             g.botEvolutionScoreWithProfile(bot, profile),
		divisions:         bot.Divisions,
		lineageDepth:      bot.LineageDepth,
		balancedInventory: min(bot.Inventory.Food, bot.Inventory.Ore),
		hp:                bot.Hp,
		boardIdx:          util.Idx(bot.Pos),
		colonyLinked:      profile.ColonyLinked,
		activeNonSolo:     profile.ActiveNonSoloColony,
		connectedMembers:  profile.ConnectedMemberCount,
	}
}

func generationChampionRankForBot(bot *core.Bot) generationChampionRank {
	if bot == nil {
		return generationChampionRank{boardIdx: util.Cells}
	}
	return generationChampionRank{
		score:             bot.EvolutionScore(),
		divisions:         bot.Divisions,
		lineageDepth:      bot.LineageDepth,
		balancedInventory: min(bot.Inventory.Food, bot.Inventory.Ore),
		hp:                bot.Hp,
		boardIdx:          util.Idx(bot.Pos),
	}
}

func generationChampionRankBefore(candidate, current generationChampionRank) bool {
	if candidate.score != current.score {
		return candidate.score > current.score
	}
	if candidate.activeNonSolo != current.activeNonSolo {
		return candidate.activeNonSolo
	}
	if candidate.colonyLinked != current.colonyLinked {
		return candidate.colonyLinked
	}
	if candidate.connectedMembers != current.connectedMembers {
		return candidate.connectedMembers > current.connectedMembers
	}
	if candidate.divisions != current.divisions {
		return candidate.divisions > current.divisions
	}
	if candidate.lineageDepth != current.lineageDepth {
		return candidate.lineageDepth > current.lineageDepth
	}
	if candidate.balancedInventory != current.balancedInventory {
		return candidate.balancedInventory > current.balancedInventory
	}
	if candidate.hp != current.hp {
		return candidate.hp > current.hp
	}
	return candidate.boardIdx < current.boardIdx
}

func (g *Game) rememberGenerationChampion(bot *core.Bot) {
	if bot == nil {
		return
	}
	rank := g.generationChampionRankForBot(bot)
	if g.smartEvolutionEnabled() {
		g.rememberEliteGenome(normalizedEvolutionGenome(bot.Genome), rank)
		return
	}
	if g.hasGenerationSeedGenome && !generationChampionRankBefore(rank, g.generationSeedRank) {
		return
	}
	g.generationSeedGenome = normalizedEvolutionGenome(bot.Genome)
	g.generationSeedRank = rank
	g.hasGenerationSeedGenome = true
	g.latestImprovement = g.logicTick
}

func (g *Game) rememberEliteCandidate(bot *core.Bot) {
	if bot == nil {
		return
	}
	g.rememberEliteGenome(normalizedEvolutionGenome(bot.Genome), g.generationChampionRankForBot(bot))
}

func normalizedEvolutionGenome(genome core.Genome) core.Genome {
	genome.Pointer = 0
	genome.NextArg = 0
	genome.Signal = 0
	genome.Registers = [4]int{}
	return genome
}

func (g *Game) rememberEliteGenome(genome core.Genome, rank generationChampionRank) {
	limit := g.evolutionEliteLimit()
	if limit <= 0 {
		return
	}

	improvedBest := len(g.eliteGenomes) == 0 || generationChampionRankBefore(rank, g.eliteGenomes[0].rank)
	for i := range g.eliteGenomes {
		if g.eliteGenomes[i].genome.Matrix != genome.Matrix {
			continue
		}
		if generationChampionRankBefore(rank, g.eliteGenomes[i].rank) {
			g.eliteGenomes[i] = eliteGenome{genome: genome, rank: rank}
			g.sortEliteGenomes()
			g.pruneEliteGenomes(limit)
			g.syncGenerationSeedFromElite()
			if improvedBest {
				g.latestImprovement = g.logicTick
			}
		}
		return
	}

	if len(g.eliteGenomes) >= limit &&
		!generationChampionRankBefore(rank, g.eliteGenomes[len(g.eliteGenomes)-1].rank) &&
		!(rank.colonyLinked && g.colonyLinkedEliteCount() < eliteColonyQuota(limit)) {
		return
	}

	g.eliteGenomes = append(g.eliteGenomes, eliteGenome{genome: genome, rank: rank})
	g.sortEliteGenomes()
	g.pruneEliteGenomes(limit)
	g.syncGenerationSeedFromElite()
	if improvedBest {
		g.latestImprovement = g.logicTick
	}
}

func (g *Game) sortEliteGenomes() {
	for i := 1; i < len(g.eliteGenomes); i++ {
		curr := g.eliteGenomes[i]
		j := i - 1
		for ; j >= 0 && generationChampionRankBefore(curr.rank, g.eliteGenomes[j].rank); j-- {
			g.eliteGenomes[j+1] = g.eliteGenomes[j]
		}
		g.eliteGenomes[j+1] = curr
	}
}

func (g *Game) pruneEliteGenomes(limit int) {
	if limit <= 0 {
		g.eliteGenomes = nil
		return
	}
	if len(g.eliteGenomes) <= limit {
		return
	}
	quota := eliteColonyQuota(limit)
	availableColony := g.colonyLinkedEliteCount()
	preserveColony := min(quota, availableColony)
	selected := make([]eliteGenome, 0, limit)
	selectedIdx := make(map[int]struct{}, limit)
	colonySelected := 0

	for i, elite := range g.eliteGenomes {
		if len(selected) == limit {
			break
		}
		remainingSlots := limit - len(selected)
		remainingColonyNeeded := preserveColony - colonySelected
		if remainingColonyNeeded > 0 && remainingSlots <= remainingColonyNeeded && !elite.rank.colonyLinked {
			continue
		}
		selected = append(selected, elite)
		selectedIdx[i] = struct{}{}
		if elite.rank.colonyLinked {
			colonySelected++
		}
	}

	for i, elite := range g.eliteGenomes {
		if len(selected) == limit {
			break
		}
		if _, ok := selectedIdx[i]; ok {
			continue
		}
		selected = append(selected, elite)
	}
	g.eliteGenomes = selected
}

func (g *Game) colonyLinkedEliteCount() int {
	count := 0
	for _, elite := range g.eliteGenomes {
		if elite.rank.colonyLinked {
			count++
		}
	}
	return count
}

func eliteColonyQuota(limit int) int {
	if limit <= 0 {
		return 0
	}
	pool := min(limit, 16)
	return max(1, (pool+1)/2)
}

func (g *Game) syncGenerationSeedFromElite() {
	if len(g.eliteGenomes) == 0 {
		return
	}
	elite := g.preferredEliteSeed(0)
	g.generationSeedGenome = elite.genome
	g.generationSeedRank = elite.rank
	g.hasGenerationSeedGenome = true
}

func (g *Game) preferredEliteSeed(seedIdx int) eliteGenome {
	if len(g.eliteGenomes) == 0 {
		return eliteGenome{}
	}
	colonyLinked := make([]eliteGenome, 0, len(g.eliteGenomes))
	general := make([]eliteGenome, 0, len(g.eliteGenomes))
	for _, elite := range g.eliteGenomes {
		if elite.rank.colonyLinked {
			colonyLinked = append(colonyLinked, elite)
		} else {
			general = append(general, elite)
		}
	}
	if seedIdx < len(colonyLinked) {
		return colonyLinked[seedIdx]
	}
	seedIdx -= len(colonyLinked)
	if seedIdx < len(general) {
		return general[seedIdx]
	}
	combinedIdx := seedIdx % len(g.eliteGenomes)
	if combinedIdx < len(colonyLinked) {
		return colonyLinked[combinedIdx]
	}
	return general[combinedIdx-len(colonyLinked)]
}

func (g *Game) smartEvolutionEnabled() bool {
	return g.config != nil && g.config.SmartEvolution
}

func (g *Game) evolutionEliteLimit() int {
	if g.config == nil {
		return 0
	}
	limit := g.config.EvolutionEliteCount
	if limit < 0 {
		return 0
	}
	return limit
}

func (g *Game) runGameMasterTick() {
	if !g.gameMasterEnabled || g.gameMasterInterval <= 0 || g.logicTick%g.gameMasterInterval != 0 {
		return
	}

	obs := g.ObserveMaster(g.logicTick)
	state := conf.GameMasterState{
		Enabled:      true,
		Name:         g.gameMaster.AdvisorName(),
		Tick:         g.logicTick,
		Interval:     g.gameMasterInterval,
		LastObserved: obs.Tick,
		LiveBots:     obs.LiveBots,
		Colonies:     obs.Colonies,
		Resources:    obs.Resources,
		Food:         obs.Food,
		Poison:       obs.Poison,
		Water:        obs.Water,
	}

	if event, ok := g.gameMaster.Decide(obs); ok {
		applied := g.ApplyMasterEvent(event)
		state.LastEventTick = applied.Tick
		state.LastEventKind = applied.Kind
		state.LastThought = applied.Thought
		state.LastReason = applied.Reason
		state.LastApplied = applied.Applied
		state.LastCenterRow = applied.Center.Row
		state.LastCenterCol = applied.Center.Col
	} else {
		previous := g.State.GameMaster
		state.LastEventTick = previous.LastEventTick
		state.LastEventKind = previous.LastEventKind
		state.LastThought = previous.LastThought
		state.LastReason = "watching"
		state.LastApplied = previous.LastApplied
		state.LastCenterRow = previous.LastCenterRow
		state.LastCenterCol = previous.LastCenterCol
	}

	g.State.GameMaster = state
}

func (g *Game) populateBoard() {
	oldBoard := g.Board
	g.Board = core.NewBoard()
	g.Board.CopyFrozenFrom(oldBoard)
	g.Board.CopyBiomesFrom(oldBoard)
	g.Board.CopyPheromonesFrom(oldBoard)
	g.Board.MarkAllDirty()
	ui.SetBoard(g.Board)
	for r := range core.Rows {
		for c := range core.Cols {
			pos := core.Position{C: c, R: r}
			if g.Board.IsWall(pos) {
				g.Board.Set(pos, core.Wall{Pos: pos})
				continue
			}
			if spawner, ok := oldBoard.At(pos).(core.Spawner); ok {
				g.Board.Set(pos, spawner)
				continue
			}
			if _, ok := oldBoard.At(pos).(core.Organics); ok {
				g.Board.Clear(pos)
				continue
			}
			if f, ok := oldBoard.At(pos).(core.Farm); ok {
				g.Board.Set(pos, f)
				continue
			}
			if fd, ok := oldBoard.At(pos).(core.Food); ok {
				g.Board.Set(pos, fd)
				continue
			}
			if bld, ok := oldBoard.At(pos).(core.Building); ok {
				g.Board.Set(pos, bld)
				continue
			}
			if c, ok := oldBoard.At(pos).(core.Water); ok {
				g.Board.Set(pos, c)
				continue
			}
			if d, ok := oldBoard.At(pos).(core.Depot); ok {
				g.Board.Set(pos, d)
				continue
			}
			if d, ok := oldBoard.At(pos).(*core.Depot); ok && d != nil {
				g.Board.Set(pos, *d)
				continue
			}
			if c, ok := oldBoard.At(pos).(core.Controller); ok {
				g.Board.Set(pos, c)
				continue
			}
			if c, ok := oldBoard.At(pos).(*core.Controller); ok && c != nil {
				g.Board.Set(pos, *c)
				continue
			}
			if b := oldBoard.GetBot(pos); b != nil {
				g.Board.AddBot(pos, b)
				continue
			}
			if g.Board.IsFrozen(pos) {
				continue
			}
			spawn := g.biomeSpawnProfile(g.Board.BiomeAt(pos))
			if util.RollChanceOf(1000, spawn.PoisonChance) {
				g.Board.Set(pos, core.Poison{Pos: pos})
				continue
			}
			if spawn.FoodChance > 0 && util.RollChanceOf(1000, spawn.FoodChance) {
				g.Board.Set(pos, core.Food{Pos: pos, Amount: 1})
				continue
			}
			if g.shouldSpawnOre(pos, spawn) {
				g.Board.Set(pos, core.Resource{Pos: pos, Amount: spawn.ResourceAmount})
				continue
			}
		}
	}
}

func (g *Game) biomeSpawnProfile(biome core.Biome) biomeSpawnProfile {
	poisonChance := clampChance(g.config.PoisonChance, 1000)
	resourceChance := clampChance(g.config.ResourceChance, 100)
	profile := biomeSpawnProfile{
		PoisonChance:   poisonChance,
		ResourceChance: resourceChance,
		ResourceAmount: 1,
	}

	switch biome {
	case core.BiomeFertile:
		profile.PoisonChance = scaledPositiveChance(poisonChance, 1, 2, 1000)
		profile.ResourceChance = scaledPositiveChance(resourceChance, 1, 4, 100)
		profile.FoodChance = fertileFoodChance(resourceChance, poisonChance)
	case core.BiomeMineral:
		profile.ResourceChance = scaledPositiveChance(resourceChance, 2, 1, 100)
		profile.ResourceAmount = 2
	case core.BiomeToxic:
		profile.PoisonChance = clampChance(poisonChance*3+positiveBonus(poisonChance, 2), 1000)
		profile.ResourceChance = scaledPositiveChance(resourceChance, 1, 2, 100)
		profile.ResourceAmount = 2
	}
	return profile
}

func (g *Game) shouldSpawnOre(pos core.Position, profile biomeSpawnProfile) bool {
	chance := g.oreSpawnChancePerMille(pos, profile)
	return chance > 0 && rand.Intn(1000) < chance
}

func (g *Game) oreSpawnChancePerMille(pos core.Position, profile biomeSpawnProfile) int {
	base := clampChance(profile.ResourceChance, 100) * 10
	if base <= 0 {
		return 0
	}

	score := core.OreVeinScore(pos)
	chance := base
	switch g.Board.BiomeAt(pos) {
	case core.BiomeMineral:
		switch {
		case score >= 72:
			chance = base * 6
		case score >= 52:
			chance = base * 3
		case score >= 34:
			chance = base * 2
		default:
			chance = base / 2
		}
	case core.BiomeFertile:
		switch {
		case score >= 90:
			chance = base / 6
		case score >= 70:
			chance = base / 10
		default:
			chance = base / 20
		}
	case core.BiomeToxic:
		switch {
		case score >= 75:
			chance = base
		case score >= 50:
			chance = base / 2
		default:
			chance = base / 5
		}
	default:
		switch {
		case score >= 80:
			chance = base
		case score >= 55:
			chance = base / 2
		default:
			chance = base / 4
		}
	}
	return clampChance(chance, 1000)
}

func fertileFoodChance(resourceChance, poisonChance int) int {
	if resourceChance <= 0 && poisonChance <= 0 {
		return 0
	}
	return clampChance(max(2, resourceChance), 1000)
}

func scaledPositiveChance(value, numerator, denominator, maxValue int) int {
	if value <= 0 {
		return 0
	}
	scaled := value * numerator / denominator
	if scaled <= 0 {
		scaled = 1
	}
	return clampChance(scaled, maxValue)
}

func positiveBonus(value, bonus int) int {
	if value <= 0 {
		return 0
	}
	return bonus
}

func clampChance(value, maxValue int) int {
	if value < 0 {
		return 0
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func (g *Game) initialBotsGeneration() {
	spawnPositions := make([]core.Position, 0)
	for r := range core.Rows {
		for c := range core.Cols {
			pos := core.Position{C: c, R: r}
			if g.Board.IsFrozen(pos) || !g.Board.IsEmpty(pos) || !util.RollChance(g.config.BotChance) {
				continue
			}
			spawnPositions = append(spawnPositions, pos)
		}
	}

	if g.smartEvolutionEnabled() && len(g.eliteGenomes) > 0 {
		eliteSlots := g.evolutionSeedSlots(len(spawnPositions))
		for i, pos := range spawnPositions {
			if !g.Board.IsEmpty(pos) || g.Board.GetBot(pos) != nil {
				continue
			}
			b := core.NewBot(pos)
			var seededElite eliteGenome
			hasSeededElite := false
			if i < eliteSlots {
				eliteIdx := i % len(g.eliteGenomes)
				elite := g.preferredEliteSeed(eliteIdx)
				seededElite = elite
				hasSeededElite = true
				if i < len(g.eliteGenomes) {
					b.Genome = elite.genome
				} else {
					b.Genome = core.NewMutatedGenomeWithRate(elite.genome, g.eliteMutationRate(eliteIdx))
				}
			} else if g.InitialGenome != nil {
				b.Genome = *g.InitialGenome
			}
			g.Board.AddBot(pos, &b)
			if hasSeededElite && seededElite.rank.colonyLinked && i < len(g.eliteGenomes) {
				g.seedEliteCohortAround(pos, seededElite, colonyEliteCohortSize-1)
			}
		}
		return
	}

	championSeedLimit := g.config.ChildrenByBot
	if championSeedLimit < 0 {
		championSeedLimit = 0
	}
	seededChampions := 0
	for _, pos := range spawnPositions {
		b := core.NewBot(pos)
		switch {
		case g.hasGenerationSeedGenome && seededChampions < championSeedLimit:
			if seededChampions > 0 {
				if util.RollChance(25) {
					b.Genome = core.NewMutatedGenomeWithRate(g.generationSeedGenome, g.baseMutationRate())
				} else {
					b.Genome = g.generationSeedGenome
				}
			} else {
				b.Genome = g.generationSeedGenome
			}
			seededChampions++
		case g.InitialGenome != nil:
			b.Genome = *g.InitialGenome
		}
		g.Board.AddBot(pos, &b)
	}
}

func (g *Game) evolutionSeedSlots(spawnCount int) int {
	if spawnCount <= 0 || g.config == nil || g.config.EvolutionSeedPercent <= 0 {
		return 0
	}
	percent := g.config.EvolutionSeedPercent
	if percent > 100 {
		percent = 100
	}
	slots := (spawnCount*percent + 99) / 100
	if slots > spawnCount {
		return spawnCount
	}
	return slots
}

func (g *Game) seedEliteCohortAround(anchor core.Position, elite eliteGenome, maxAdditional int) int {
	if maxAdditional <= 0 || !elite.rank.colonyLinked || g == nil || g.Board == nil {
		return 0
	}
	seeded := 0
	for radius := 1; radius <= controllerRecruitRadius && seeded < maxAdditional; radius++ {
		for dr := -radius; dr <= radius && seeded < maxAdditional; dr++ {
			for dc := -radius; dc <= radius && seeded < maxAdditional; dc++ {
				if max(util.Abs(dr), util.Abs(dc)) != radius {
					continue
				}
				pos := anchor.AddRowCol(dr, dc)
				if g.Board.IsWall(pos) || g.Board.IsFrozen(pos) || !g.Board.IsEmpty(pos) || g.Board.GetBot(pos) != nil {
					continue
				}
				b := core.NewBot(pos)
				b.Genome = elite.genome
				g.Board.AddBot(pos, &b)
				seeded++
			}
		}
	}
	return seeded
}

func (g *Game) baseMutationRate() int {
	if g.config == nil || g.config.MutationRate < 0 {
		return 0
	}
	return g.config.MutationRate
}

func (g *Game) eliteMutationRate(eliteIdx int) int {
	base := g.baseMutationRate()
	if base <= 0 {
		return 0
	}
	rate := base
	if eliteIdx < max(1, len(g.eliteGenomes)/4) {
		rate = max(1, base/2)
	}
	if g.logicTick-g.latestImprovement > 500 {
		rate = max(rate, base*2+1)
	}
	return rate
}

func (g *Game) botsActions() {
	g.botIterationIDs = g.sortedActiveBotIDs(g.botIterationIDs[:0])
	for _, id := range g.botIterationIDs {
		b := g.Board.BotByID(id)
		if b == nil {
			continue
		}
		i := g.Board.BotCell(id)
		if i < 0 {
			continue
		}
		pos := util.PosOf(i)
		if g.Board.IsFrozen(pos) {
			continue
		}
		b.Age++
		b.Hp -= g.calcHpChange()
		heartProtected := g.applyColonyHeartProtection(pos, b)
		b.Hp = min(b.Hp, 500)
		ageExpired := g.config.MaxBotAge > 0 && b.Age > g.config.MaxBotAge && !g.colonyHeartAgeProtected(pos, b)
		if b.Hp <= 0 || ageExpired {
			g.emitEventPheromone(pos, core.PheromoneDanger)
			g.killBot(b, i)
			if rand.Intn(100) < 33 {
				g.Board.Set(pos, core.Organics{Pos: pos, Amount: g.config.OrganicInitialAmount})
			} else {
				g.Board.Clear(pos)
			}
			continue
		}
		if b.CurrTask != nil && b.CurrTask.Type == core.MaintainConnectionTask && b.CurrTask.IsDone {
			continue
		}
		if g.tryColonyCohesion(pos, b) {
			if g.Board.GetBot(b.Pos) == b {
				g.applyColonyHeartProtection(b.Pos, b)
				b.Hp = min(b.Hp, 500)
			}
			continue
		}
		g.botAction(pos, b)
		if heartProtected && g.Board.GetBot(b.Pos) == b {
			g.applyColonyHeartProtection(b.Pos, b)
			b.Hp = min(b.Hp, 500)
		}
	}
}

func (g *Game) sortedActiveBotIDs(dst []core.BotID) []core.BotID {
	dst = append(dst, g.Board.ActiveBotIDs()...)
	if g.scaleMode {
		return dst
	}
	sort.Slice(dst, func(i, j int) bool {
		left := g.Board.BotCell(dst[i])
		right := g.Board.BotCell(dst[j])
		if left != right {
			return left < right
		}
		return dst[i] < dst[j]
	})
	return dst
}

func (g *Game) calcHpChange() int {
	if g.scaleMode {
		return 1
	}
	var hpChange int
	if g.config.LiveBots > 20000 {
		hpChange = 5
	} else if g.config.LiveBots > 15000 {
		hpChange = 3
	} else if g.config.LiveBots > 10000 {
		hpChange = 3
	} else if g.config.LiveBots > 5000 {
		hpChange = 3
	} else if g.config.LiveBots > 3000 {
		hpChange = 2
	} else if g.config.LiveBots > 1000 {
		hpChange = 1
	} else {
		hpChange = 1
	}
	return hpChange
}

func (g *Game) divisionThreshold(b *core.Bot) int {
	if b != nil && b.ConnnectedToColony {
		return 100
	}
	return g.config.DivisionMinHp
}

func (g *Game) inheritColonyConnection(parent, child *core.Bot) {
	if parent == nil || child == nil || parent.Colony == nil || child.Colony != parent.Colony {
		return
	}
	if !parent.ConnnectedToColony {
		return
	}
	if !g.hasActiveColonySupportNear(parent.Colony, child.Pos, g.colonyNestRadius()) {
		return
	}
	child.ConnnectedToColony = true
}

func (g *Game) hasActiveColonySupportNear(colony *core.Colony, pos util.Position, radius int) bool {
	if g == nil || g.Board == nil || colony == nil || radius < 0 {
		return false
	}
	if active, _ := g.colonyCenterController(colony); active && pos.InRadius(colony.Center, radius) {
		return true
	}
	for _, flag := range colony.Flags {
		if flag != nil && pos.InRadius(flag.Pos, radius) {
			return true
		}
	}
	if g.nearestColonyAnchorDistance(colony, pos, radius) <= radius {
		return true
	}
	if owner := g.Board.PheromoneHomeOwnerAt(pos); owner == colony && g.Board.PheromoneAt(pos).Home > 0 {
		return true
	}
	return false
}

func (g *Game) DivisionReady(b *core.Bot) bool {
	if b == nil {
		return false
	}
	if !g.canPayShared(b, g.config.DivisionFoodCost, g.config.DivisionOreCost) {
		return false
	}
	if b.Hp >= g.divisionThreshold(b) {
		return true
	}
	if b.Hp < g.config.SpawnerDivisionMinHp {
		return false
	}
	_, ok := g.findSpawnerDivisionTarget(b)
	return ok
}

type spawnerDivisionTarget struct {
	pos      core.Position
	spawner  core.Spawner
	childPos core.Position
	distance int
	index    int
}

func (g *Game) trySpawnerAssistedDivision(parentPos core.Position, b *core.Bot) bool {
	if g == nil || g.Board == nil || g.config == nil || b == nil {
		return false
	}
	if b.Hp < max(0, g.config.SpawnerDivisionMinHp) {
		return false
	}
	if !g.canPayShared(b, g.config.DivisionFoodCost, g.config.DivisionOreCost) {
		return false
	}
	target, ok := g.findSpawnerDivisionTarget(b)
	if !ok {
		return false
	}
	if !g.spendShared(b, g.config.DivisionFoodCost, g.config.DivisionOreCost) {
		return false
	}

	target.spawner.Amount--
	g.claimSpawnerIfClaimable(&target.spawner, b)
	g.Board.Set(target.pos, target.spawner)

	b.Divisions++
	b.Evolution.SuccessfulDivisions++
	b.Evolution.SpawnerBirths++
	child := b.NewChildWithMutationRate(target.childPos, g.config.ShouldMutateColor, g.baseMutationRate())
	if genome, ok := g.spawnerChildGenome(target.spawner); ok {
		child.Genome = genome
	}
	g.inheritColonyConnection(b, child)
	b.Hp -= g.config.DivisionCost
	g.Board.AddBot(target.childPos, child)
	g.successfulDivisions++
	g.totalSpawnerBirths++
	g.emitEventPheromone(target.pos, core.PheromoneFood)
	g.Board.MarkDirty(idx(parentPos))
	b.PointerJumpBy(6)
	return true
}

func (g *Game) findSpawnerDivisionTarget(b *core.Bot) (spawnerDivisionTarget, bool) {
	if g == nil || g.Board == nil || g.config == nil || b == nil {
		return spawnerDivisionTarget{}, false
	}
	radius := max(0, g.config.SpawnerAccessRadius)
	found := false
	best := spawnerDivisionTarget{}
	for r := b.Pos.R - radius; r <= b.Pos.R+radius; r++ {
		if r < 0 || r >= core.Rows {
			continue
		}
		for dc := -radius; dc <= radius; dc++ {
			pos := util.NewPos(r, b.Pos.C+dc)
			spawner, ok := g.Board.At(pos).(core.Spawner)
			if !ok || spawner.Amount <= 0 || !g.spawnerFriendlyToBot(spawner, b) {
				continue
			}
			childPos, ok := g.findDivisionChildPosAround(pos, b)
			if !ok {
				continue
			}
			distance := boardDistance(b.Pos, pos)
			if distance > radius {
				continue
			}
			candidate := spawnerDivisionTarget{
				pos:      pos,
				spawner:  spawner,
				childPos: childPos,
				distance: distance,
				index:    idx(pos),
			}
			if !found ||
				candidate.distance < best.distance ||
				(candidate.distance == best.distance && candidate.index < best.index) {
				best = candidate
				found = true
			}
		}
	}
	return best, found
}

func (g *Game) findEmptyUnfrozenPosAround(center core.Position) (core.Position, bool) {
	if g == nil || g.Board == nil {
		return core.Position{}, false
	}
	for _, dir := range core.PosClock {
		pos := center.AddDir(dir)
		if !core.Inside(pos) {
			continue
		}
		if g.Board.IsEmpty(pos) && !g.Board.IsFrozen(pos) {
			return pos, true
		}
	}
	return core.Position{}, false
}

func (g *Game) botAction(pos core.Position, b *core.Bot) {
	for range 5 {
		op := core.DecodeOpcode(b.Genome.Matrix[b.Genome.Pointer])
		if b.MaintainingConn() {
			op = core.OpMove
		}
		if taskOp, ok := g.colonyTaskOpcode(pos, b, op); ok {
			op = taskOp
		}
		// fmt.Printf("opcode: %v\n", op)
		switch op {
		case core.OpDivide:
			if g.trySpawnerAssistedDivision(pos, b) {
				return
			}
			if b.Hp < g.divisionThreshold(b) {
				b.PointerJumpBy(5)
				return
			}
			if !g.canPayShared(b, g.config.DivisionFoodCost, g.config.DivisionOreCost) {
				b.PointerJumpBy(5)
				return
			}

			newPos, ok := g.findDivisionChildPosAround(pos, b)
			if !ok {
				b.PointerJumpBy(4)
				continue
			}
			if g.Board.IsFrozen(newPos) {
				b.PointerJumpBy(4)
				continue
			}
			b.Divisions++
			b.Evolution.SuccessfulDivisions++
			child := b.NewChildWithMutationRate(newPos, g.config.ShouldMutateColor, g.baseMutationRate())
			g.inheritColonyConnection(b, child)
			g.spendShared(b, g.config.DivisionFoodCost, g.config.DivisionOreCost)
			b.Hp -= g.config.DivisionCost
			g.Board.AddBot(newPos, child)
			g.successfulDivisions++
			b.PointerJumpBy(6)
			return

		case core.OpCheckConnection:
			if b.ConnnectedToColony {
				b.PointerJumpBy(1)
				b.Genome.NextArg = 1
			} else {
				b.PointerJumpBy(2)
				b.Genome.NextArg = 2
			}
			continue

		case core.OpMoveAbs:
			b.Dir = util.PosClock[b.CmdArg(1)%8]
			g.tryMove(pos, b)
			b.PointerJumpBy(1)
			return

		case core.OpCheckIfBro:
			checkPos := b.CmdArgDir(1, pos)
			other, ok := g.Board.At(checkPos).(*core.Bot)
			if !ok {
				b.PointerJumpBy(1)
				continue
			}
			if b.IsBro(other) || (other.ConnnectedToColony && b.SameColony(other)) {
				b.PointerJumpBy(2)
				continue
			}
			b.PointerJumpBy(3)
			continue

		case core.OpCheckColony:
			checkPos := b.CmdArgDir(1, pos)
			other, ok := g.Board.At(checkPos).(*core.Bot)
			if !ok {
				b.PointerJumpBy(1)
				continue
			}
			if b.SameColony(other) {
				b.PointerJumpBy(2)
				continue
			}
			b.PointerJumpBy(3)
			continue

		case core.OpAttack:
			attackPos := b.CmdArgDir(1, pos)
			target := g.Board.At(attackPos)
			other, ok := target.(*core.Bot)
			if !ok {
				switch ctrl := target.(type) {
				case core.Controller:
					if g.raidController(b, &ctrl) {
						g.Board.Set(attackPos, ctrl)
						b.PointerJumpBy(2)
						return
					}
				case *core.Controller:
					if g.raidController(b, ctrl) {
						g.Board.MarkDirty(idx(attackPos))
						b.PointerJumpBy(2)
						return
					}
				case core.Depot:
					if g.raidDepot(b, &ctrl) {
						g.Board.Set(attackPos, ctrl)
						b.PointerJumpBy(2)
						return
					}
				case *core.Depot:
					if g.raidDepot(b, ctrl) {
						g.Board.MarkDirty(idx(attackPos))
						b.PointerJumpBy(2)
						return
					}
				}
				b.PointerJumpBy(1)
				continue
			}
			if core.BotsFriendly(b, other) {
				b.PointerJumpBy(1)
				continue
			}
			g.emitEventPheromone(pos, core.PheromoneDanger)
			g.emitEventPheromone(attackPos, core.PheromoneDanger)
			if b.Hp > other.Hp {
				// fmt.Printf("I attack %v. Hp: %d; other.Hp: %d\n", attackPos, b.Hp, other.Hp)
				b.Hp -= other.Hp
				g.recordFoodStolen(b, other.Inventory.Food)
				g.recordOreStolen(b, other.Inventory.Ore)
				g.recordCombatKill(b)
				g.Board.Clear(attackPos)
				g.killBot(other, idx(attackPos))
			}
			b.PointerJumpBy(2)
			return

		case core.OpMove:
			g.tryMove(pos, b)
			b.PointerJumpBy(1)
			return

		case core.OpTurn:
			b.Dir = util.PosClock[b.CmdArg(1)%8]
			b.PointerJumpBy(1)
			continue

		case core.OpEatOther:
			b.PointerJumpBy(1)
			return

		case core.OpPhoto:
			row := pos.R
			rows := util.Rows

			var photoChance int
			switch {
			case row < rows*1/10:
				photoChance = 100
			case row < rows*2/10:
				photoChance = 80
			case row < rows*3/10:
				photoChance = 50
			default:
				photoChance = 25
			}

			if util.RollChance(photoChance) {
				b.Hp += g.config.PhotoHpGain
				if g.connectedColonyBotInOwnedNestOrTissue(pos, b) {
					g.recordFoodGathered(b, max(0, g.config.ColonyPhotoFoodGain))
				}
				if g.config.EnableResourceBasedColorChange {
					dc := g.config.ColorDelta
					color := b.Color
					b.Color = [3]float32{color[0] - dc, color[1] + dc, color[2] - dc}
				}
			}
			b.PointerJumpBy(1)
			return

		case core.OpLook:
			g.lookAround(pos, b)
			continue

		case core.OpCheckInventory:
			inventory := g.accessibleInventory(b)
			var amt int
			switch b.CmdArg(1) % 3 {
			case 1:
				amt = inventory.Food
			case 2:
				amt = inventory.Ore
			default:
				amt = inventory.Total()
			}
			b.Genome.NextArg = amt
			if amt > 70 {
				b.PointerJumpBy(1)
			} else {
				b.PointerJumpBy(2)
			}
			continue

		case core.OpExecuteInstr:
			b.PointerJumpBy(b.CmdArg(1))
			continue

		case core.OpHpToResource:
			// arg := b.CmdArg(1) % 4
			// hpChange := arg * 10
			// if b.Hp < hpChange {
			// 	b.PointerJumpBy(3)
			// 	return
			// }
			// b.Hp -= hpChange
			// b.Inventory.AddOre(arg)
			b.PointerJumpBy(2)
			return

		case core.OpCheckHp:
			b.Genome.NextArg = b.Hp
			if b.Hp > 100 {
				b.PointerJumpBy(3)
			} else {
				b.PointerJumpBy(5)
			}
			continue

		case core.OpEatOrganicsAbs:
			nextPos := b.CmdArgDir(1, pos)
			o, ok := g.Board.At(nextPos).(core.Organics)
			if !ok {
				b.PointerJumpBy(2)
				continue
			}
			// fmt.Printf("I eat organics abs. Hp after: %d; \n", b.Hp)
			b.Hp += o.Amount
			g.Board.Clear(nextPos)
			b.PointerJumpBy(1)
			return

		case core.OpEatOrganics:
			nextPos := pos.AddRowCol(b.Dir[0], b.Dir[1])
			o, ok := g.Board.At(nextPos).(core.Organics)
			if !ok {
				b.PointerJumpBy(2)
				continue
			}
			// fmt.Printf("I eat organics . Hp after: %d; \n", b.Hp)
			b.Hp += o.Amount
			g.Board.Clear(nextPos)
			b.PointerJumpBy(1)
			return

		case core.OpGrab:
			g.grab(pos, b)
			g.completeColonyTaskAfterGrab(b)
			return

		case core.OpShareHp:
			if b.Hp < 20 {
				b.PointerJumpBy(6)
				continue
			}
			sharePos := b.CmdArgDir(1, pos)
			shareAmount := b.CmdArg(2)
			if b.Hp < shareAmount {
				b.PointerJumpBy(5)
				continue
			}
			other, ok := g.Board.At(sharePos).(*core.Bot)
			if !ok {
				b.PointerJumpBy(4)
				continue
			}
			if !core.BotsFriendly(b, other) {
				b.PointerJumpBy(4)
				continue
			}
			other.Hp += shareAmount
			if shareAmount > 0 && sameConnectedColony(b, other) {
				other.Hp += max(0, g.config.ColonyShareHpBonus)
			}
			b.Hp -= shareAmount
			// fmt.Printf("I share %d HP with %v. Is bro? %v\n", shareAmount, sharePos, b.IsBro(other))
			b.PointerJumpBy(3)
			return

		case core.OpShareInventory:
			shareFood := b.CmdArg(3)%2 == 0
			available := b.Inventory.Ore
			if shareFood {
				available = b.Inventory.Food
			}
			if available < 5 {
				b.PointerJumpBy(5)
				continue
			}
			sharePos := b.CmdArgDir(1, pos)
			shareAmount := b.CmdArg(2)
			if shareFood && !b.Inventory.CanPay(shareAmount, 0) {
				b.PointerJumpBy(5)
				continue
			}
			if !shareFood && !b.Inventory.CanPay(0, shareAmount) {
				b.PointerJumpBy(5)
				continue
			}
			other, ok := g.Board.At(sharePos).(*core.Bot)
			if !ok {
				b.PointerJumpBy(3)
				continue
			}
			if !core.BotsFriendly(b, other) {
				b.PointerJumpBy(3)
				continue
			}
			if shareFood {
				b.Inventory.Spend(shareAmount, 0)
				other.Inventory.AddFood(shareAmount)
				if shareAmount > 0 && sameConnectedColony(b, other) {
					other.Inventory.AddFood(max(0, g.config.ColonyShareInventoryBonus))
				}
			} else {
				b.Inventory.Spend(0, shareAmount)
				other.Inventory.AddOre(shareAmount)
				if shareAmount > 0 && sameConnectedColony(b, other) {
					other.Inventory.AddOre(max(0, g.config.ColonyShareInventoryBonus))
				}
			}
			// fmt.Printf("I share %d Inventory with %v. Is bro? %v\n", shareAmount, sharePos, b.IsBro(other))
			b.PointerJumpBy(2)
			return

		case core.OpBuild:
			g.build(pos, b)
			g.completeColonyTaskAfterBuild(b)
			g.Board.Set(pos, b)
			continue

		case core.OpSetReg:
			reg := b.CmdArg(1) % 4
			b.Genome.Registers[reg] = b.Genome.NextArg
			b.PointerJumpBy(3)
			return

		case core.OpIncReg:
			reg := b.CmdArg(1) % 4
			b.Genome.Registers[reg]++
			b.PointerJumpBy(2)
			continue

		case core.OpDecReg:
			reg := b.CmdArg(1) % 4
			b.Genome.Registers[reg]--
			b.PointerJumpBy(2)
			continue

		case core.OpCmpReg:
			regA := b.CmdArg(1) % 4
			regB := b.CmdArg(2) % 4
			aVal := b.Genome.Registers[regA]
			bVal := b.Genome.Registers[regB]
			if aVal == bVal {
				b.PointerJumpBy(3)
			} else if aVal < bVal {
				b.PointerJumpBy(4)
			} else {
				b.PointerJumpBy(5)
			}
			continue

		case core.OpJumpIfZero:
			reg := b.CmdArg(1) % 4
			// fmt.Printf("I jump if reg %v is zero\n", reg)
			jump := b.CmdArg(2)
			if b.Genome.Registers[reg] == 0 {
				b.PointerJumpBy(3 + jump)
			} else {
				b.PointerJumpBy(3)
			}
			continue

		case core.OpSendSignal:
			sendPos := b.CmdArgDir(1, pos)
			if util.OutOfBounds(sendPos) {
				b.Genome.NextArg = 0
				b.PointerJumpBy(3)
				continue
			}
			other := g.Board.GetBot(sendPos)
			if other == nil {
				b.Genome.NextArg = 0
				b.PointerJumpBy(3)
				continue
			}
			commands := 4 // arbitrary value
			sigVal := b.CmdArg(2) % commands
			other.Genome.Signal = sigVal
			b.Genome.NextArg = sigVal
			b.PointerJumpBy(2)
			continue

		case core.OpCheckSignal:
			sigVal := b.Genome.Signal
			b.Genome.Signal = 0
			b.PointerJumpBy(sigVal + 1)
			continue

		case core.OpEmitPheromone:
			channel := core.DecodePheromoneChannel(b.CmdArg(1))
			if !g.pheromonesEnabled() || g.config.PheromoneBotDeposit <= 0 {
				b.Genome.NextArg = 0
				b.PointerJumpBy(3)
				return
			}
			cost := max(0, g.config.PheromoneEmitHpCost)
			if b.Hp <= cost {
				b.Genome.NextArg = 0
				b.PointerJumpBy(3)
				return
			}
			b.Hp -= cost
			g.Board.MarkDirty(idx(pos))
			if g.emitBotPheromone(pos, b, channel) {
				b.Genome.NextArg = int(g.Board.PheromoneAt(pos).Channel(channel))
				b.PointerJumpBy(2)
			} else {
				b.Genome.NextArg = 0
				b.PointerJumpBy(3)
			}
			return

		case core.OpSensePheromone:
			channel := core.DecodePheromoneChannel(b.CmdArg(1))
			senseDir := util.PosClock[b.CmdArg(2)%8]
			sensePos := pos.AddDir(senseDir)
			value := g.sensePheromone(sensePos, b, channel)
			b.Genome.NextArg = int(value)
			if int(value) >= max(0, g.config.PheromoneSenseThreshold) {
				b.PointerJumpBy(2)
			} else {
				b.PointerJumpBy(3)
			}
			continue

		case core.OpFollowPheromone:
			channel := core.DecodePheromoneChannel(b.CmdArg(1))
			avoid := b.CmdArg(2)%2 == 1
			if dir, value, ok := g.selectPheromoneDirection(pos, b, channel, avoid); ok {
				b.Dir = dir
				b.Genome.NextArg = int(value)
				g.tryMove(pos, b)
			} else {
				b.Genome.NextArg = 0
			}
			b.PointerJumpBy(3)
			return

		default:
			b.PointerJump()
			return
		}
	}
}

func (g *Game) tryMove(oldPos core.Position, b *core.Bot) {
	g.Board.MarkDirty(idx(oldPos))
	newPos := oldPos.AddDir(b.Dir)
	oldIdx := util.Idx(oldPos)

	if b.HasTask() {
		newPos = oldPos
		if isColonyRoleTask(b.CurrTask) {
			next, ok := g.nextColonyRoleTaskStep(oldPos, b)
			if !ok {
				return
			}
			newPos = next
		} else {
			if b.CurrTask.Type == core.MaintainConnectionTask && oldPos == b.CurrTask.Pos {
				b.CurrTask.MarkDone()
				return
			}
			if b.Colony == nil || b.Colony.WaterPathFlowField == nil {
				return
			}
			if next, ok := b.Colony.NextPathStep(oldPos, b.CurrTask.Pos); ok {
				newPos = next
			} else {
				best := b.Colony.WaterPathFlowField[oldIdx]
				for _, dir := range util.PosCross {
					n := oldPos.AddDir(dir)
					if g.Board.GetBot(n) != nil {
						continue
					}
					if v := b.Colony.WaterPathFlowField[util.Idx(n)]; v < best {
						best, newPos = v, n
					}
				}
			}
		}
	}

	if g.Board.IsFrozen(newPos) {
		return
	}
	if g.autoPickupOnMove(newPos, b) {
		g.completeColonyTaskAfterPickup(b, newPos)
		return
	}
	if !g.Board.IsEmpty(newPos) {
		return
	}

	b.Pos = newPos

	g.Board.MoveBot(oldPos, newPos, b)
	g.completeColonyTaskAfterMove(b)
}

func (g *Game) autoPickupOnMove(pos core.Position, b *core.Bot) bool {
	c := g.config
	switch v := g.Board.At(pos).(type) {
	case core.Food:
		foodGain := v.Amount
		if foodGain <= 0 {
			foodGain = 1
		}
		g.recordFoodGathered(b, foodGain)
		b.Hp += c.FoodGrabHpGain
		g.Board.Clear(pos)
		g.emitEventPheromone(pos, core.PheromoneFood)
		return true
	case core.Resource:
		g.recordOreGathered(b, c.ResourceGrabGain)
		b.Hp += c.ResourceGrabHpGain
		v.Amount -= 10
		if v.Amount <= 0 {
			g.Board.Clear(pos)
		} else {
			g.Board.Set(pos, v)
		}
		g.emitEventPheromone(pos, core.PheromoneOre)
		return true
	default:
		return false
	}
}

func (g *Game) recordFoodGathered(b *core.Bot, amount int) {
	if amount <= 0 {
		return
	}
	b.GatherFood(amount)
	g.totalFoodGathered += amount
}

func (g *Game) recordOreGathered(b *core.Bot, amount int) {
	if amount <= 0 {
		return
	}
	b.GatherOre(amount)
	g.totalOreGathered += amount
}

func (g *Game) recordFoodStolen(b *core.Bot, amount int) {
	if amount <= 0 {
		return
	}
	b.StealFood(amount)
	g.totalStolenFood += amount
}

func (g *Game) recordOreStolen(b *core.Bot, amount int) {
	if amount <= 0 {
		return
	}
	b.StealOre(amount)
	g.totalStolenOre += amount
}

func (g *Game) recordCombatKill(b *core.Bot) {
	if b == nil {
		return
	}
	b.RecordCombatKill()
	g.totalCombatKills++
}

func (g *Game) recordControllerRaid(b *core.Bot) {
	if b == nil {
		return
	}
	b.RecordControllerRaid()
	g.totalControllerRaids++
}

func (g *Game) recordDepotRaid(b *core.Bot) {
	if b == nil {
		return
	}
	b.RecordDepotRaid()
	g.totalDepotRaids++
}

var i = 0

func (g *Game) grab(pos core.Position, b *core.Bot) {
	c := *g.config

	// dir := util.PosClock[b.CmdArg(1)%8]
	// TODO: decide. this is test one
	dir := b.Dir
	grabPos := pos.AddRowCol(dir[0], dir[1])
	if taskTarget, ok := g.colonyTaskGrabTarget(pos, b); ok {
		grabPos = taskTarget
	}
	// fmt.Printf("grabbing %v\n", grabPos)

	// if !g.Board.IsGrabable(grabPos) {
	// 	return
	// }

	switch v := g.Board.At(grabPos).(type) {
	case core.Building:
		if v.Owner == b {
			return
		}
		g.recordOreGathered(b, c.BuildingGrabGain)
		b.Hp += c.BuildingGrabHpGain
		g.Board.Set(grabPos, nil)
		g.emitEventPheromone(grabPos, core.PheromoneOre)
		b.PointerJumpBy(1)
		b.Genome.NextArg = 1
		return
	case core.Spawner:
		if !g.spawnerFriendlyToBot(v, b) {
			if g.raidSpawner(b, &v) {
				g.Board.Set(grabPos, v)
				b.PointerJumpBy(2)
				b.Genome.NextArg = 2
			}
			return
		}
		if v.Amount >= max(0, c.SpawnerMaxAmount) {
			b.PointerJumpBy(2)
			b.Genome.NextArg = 2
			return
		}
		if !g.spendShared(b, c.SpawnerGrabCost, 0) {
			return
		}
		g.claimSpawnerIfClaimable(&v, b)
		v.Amount++
		g.Board.Set(grabPos, v)
		g.emitEventPheromone(grabPos, core.PheromoneFood)
		b.PointerJumpBy(2)
		b.Genome.NextArg = 2
		return
	case core.Farm:
		if !g.farmFriendlyToBot(v, b) {
			b.PointerJumpBy(3)
			b.Genome.NextArg = 0
			return
		}
		if !g.spendShared(b, 0, c.FarmGrabCost) {
			b.PointerJumpBy(3)
			b.Genome.NextArg = 0
			return
		}
		if v.Owner == nil || !g.farmOwnerAlive(v.Owner) {
			v.Owner = b
		}
		if v.Colony == nil && b.Colony != nil {
			v.Colony = b.Colony
		}
		v.Amount += g.colonyFarmChargeAmount(v, b)
		g.Board.Set(grabPos, v)
		b.PointerJumpBy(3)
		b.Genome.NextArg = 3
		return
	case core.Food:
		foodGain := v.Amount
		if foodGain <= 0 {
			foodGain = 1
		}
		g.recordFoodGathered(b, foodGain)
		b.Hp += c.FoodGrabHpGain
		g.Board.Clear(grabPos)
		g.emitEventPheromone(grabPos, core.PheromoneFood)
		b.PointerJumpBy(8)
		b.Genome.NextArg = 8
		return
	case core.Poison:
		b.Hp = 0
		g.Board.Clear(pos)
		g.Board.Clear(grabPos)
		g.emitEventPheromone(pos, core.PheromoneDanger)
		g.emitEventPheromone(grabPos, core.PheromoneDanger)
		b.PointerJumpBy(4)
		b.Genome.NextArg = 4
		return
	case core.Controller:
		if !controllerFriendlyToBot(&v, b) {
			if g.raidController(b, &v) {
				g.Board.Set(grabPos, v)
				b.PointerJumpBy(5)
				b.Genome.NextArg = 5
			}
			return
		}
		if !g.spendShared(b, c.ControllerGrabCost, 0) {
			return
		}
		b.Hp += c.ControllerHpGain
		v.Amount += 1
		g.Board.Set(grabPos, v)
		b.PointerJumpBy(5)
		b.Genome.NextArg = 5
		return
	case *core.Controller:
		if !controllerFriendlyToBot(v, b) {
			if g.raidController(b, v) {
				g.Board.MarkDirty(idx(grabPos))
				b.PointerJumpBy(5)
				b.Genome.NextArg = 5
			}
			return
		}
		if !g.spendShared(b, c.ControllerGrabCost, 0) {
			return
		}
		b.Hp += c.ControllerHpGain
		v.Amount += 1
		g.Board.MarkDirty(idx(grabPos))
		b.PointerJumpBy(5)
		b.Genome.NextArg = 5
		return
	case core.Depot:
		if !depotFriendlyToBot(&v, b) {
			if g.raidDepot(b, &v) {
				g.Board.Set(grabPos, v)
				b.PointerJumpBy(9)
				b.Genome.NextArg = 9
			}
			return
		}
		b.PointerJumpBy(9)
		b.Genome.NextArg = 9
		return
	case *core.Depot:
		if !depotFriendlyToBot(v, b) {
			if g.raidDepot(b, v) {
				g.Board.MarkDirty(idx(grabPos))
				b.PointerJumpBy(9)
				b.Genome.NextArg = 9
			}
			return
		}
		b.PointerJumpBy(9)
		b.Genome.NextArg = 9
		return
	case core.Resource:
		// if b.Inventory.Total() > 10 {
		// 	b.Hp -= 10
		// }
		g.recordOreGathered(b, c.ResourceGrabGain)
		b.Hp += c.ResourceGrabHpGain
		// Todo:  adjust
		v.Amount -= 10
		if v.Amount <= 0 {
			g.Board.Set(grabPos, nil)
		} else {
			g.Board.Set(grabPos, v)
		}
		g.emitEventPheromone(grabPos, core.PheromoneOre)
		b.PointerJumpBy(6)
		b.Genome.NextArg = 6
		return
	case core.Mine:
		if v.Amount <= 0 {
			g.Board.Clear(grabPos)
			b.PointerJumpBy(7)
			b.Genome.NextArg = 7
			return
		}
		if b.Hp < c.MineGrabHpCost {
			return
		}
		gain := min(c.MineGrabGain, v.Amount)
		g.recordOreGathered(b, gain)
		b.Hp -= c.MineGrabHpCost
		v.Amount -= gain
		if v.Amount <= 0 {
			g.Board.Clear(grabPos)
		} else {
			g.Board.Set(grabPos, v)
		}
		g.emitEventPheromone(grabPos, core.PheromoneOre)
		b.PointerJumpBy(7)
		b.Genome.NextArg = 7
		return
	default:
		b.PointerJumpBy(8)
		b.Genome.NextArg = 8
		return
	}
}

func controllerFriendlyToBot(ctrl *core.Controller, b *core.Bot) bool {
	return ctrl != nil && b != nil && ctrl.Colony != nil && b.Colony == ctrl.Colony
}

func depotFriendlyToBot(depot *core.Depot, b *core.Bot) bool {
	return depot != nil && b != nil && depot.Colony != nil && b.Colony == depot.Colony
}

func (g *Game) spawnerFriendlyToBot(spawner core.Spawner, b *core.Bot) bool {
	if b == nil {
		return false
	}
	if spawner.Colony != nil {
		return b.Colony == spawner.Colony
	}
	if spawner.Owner == nil || !g.spawnerOwnerAlive(spawner.Owner) {
		return true
	}
	return core.BotsFriendly(spawner.Owner, b)
}

func (g *Game) spawnerOwnerAlive(owner *core.Bot) bool {
	return g != nil && g.Board != nil && owner != nil && core.Inside(owner.Pos) && g.Board.GetBot(owner.Pos) == owner
}

func (g *Game) claimSpawnerIfClaimable(spawner *core.Spawner, b *core.Bot) bool {
	if spawner == nil || b == nil {
		return false
	}
	if spawner.Owner == nil || !g.spawnerOwnerAlive(spawner.Owner) {
		spawner.Owner = b
		if b.Colony != nil && b.ConnnectedToColony {
			spawner.Colony = b.Colony
		}
		return true
	}
	return false
}

func (g *Game) farmFriendlyToBot(farm core.Farm, b *core.Bot) bool {
	if b == nil {
		return false
	}
	if farm.Colony != nil && b.Colony == farm.Colony {
		return true
	}
	if farm.Owner != nil && core.BotsFriendly(farm.Owner, b) {
		return true
	}
	return farm.Colony == nil && !g.farmOwnerAlive(farm.Owner)
}

func (g *Game) farmOwnerAlive(owner *core.Bot) bool {
	return owner != nil && core.Inside(owner.Pos) && g.Board.GetBot(owner.Pos) == owner
}

func (g *Game) raidController(attacker *core.Bot, ctrl *core.Controller) bool {
	if attacker == nil || attacker.Colony == nil || ctrl == nil || ctrl.Colony == nil || attacker.Colony == ctrl.Colony {
		return false
	}
	food := min(ControllerRaidFoodLimit, ctrl.Colony.FoodBank)
	ore := min(ControllerRaidOreLimit, ctrl.Colony.OreBank)
	if food <= 0 && ore <= 0 {
		return false
	}
	if !ctrl.Colony.SpendBank(food, ore) {
		return false
	}
	g.recordFoodStolen(attacker, food)
	g.recordOreStolen(attacker, ore)
	g.recordControllerRaid(attacker)
	g.emitEventPheromone(attacker.Pos, core.PheromoneDanger)
	g.emitEventPheromone(ctrl.Pos, core.PheromoneDanger)
	return true
}

func (g *Game) raidDepot(attacker *core.Bot, depot *core.Depot) bool {
	if attacker == nil || attacker.Colony == nil || depot == nil || depot.Colony == nil || attacker.Colony == depot.Colony {
		return false
	}
	food := min(max(0, g.config.DepotRaidFoodLimit), depot.Food)
	ore := min(max(0, g.config.DepotRaidOreLimit), depot.Ore)
	if food <= 0 && ore <= 0 {
		return false
	}
	depot.Food -= food
	depot.Ore -= ore
	g.recordFoodStolen(attacker, food)
	g.recordOreStolen(attacker, ore)
	g.recordDepotRaid(attacker)
	g.emitEventPheromone(attacker.Pos, core.PheromoneDanger)
	g.emitEventPheromone(depot.Pos, core.PheromoneDanger)
	return true
}

func (g *Game) raidSpawner(attacker *core.Bot, spawner *core.Spawner) bool {
	if attacker == nil || spawner == nil || spawner.Amount <= 0 || g.spawnerFriendlyToBot(*spawner, attacker) {
		return false
	}
	spawner.Amount--
	g.recordFoodStolen(attacker, 1)
	g.emitEventPheromone(attacker.Pos, core.PheromoneDanger)
	g.emitEventPheromone(spawner.Pos, core.PheromoneDanger)
	return true
}

func (g *Game) build(botPos core.Position, b *core.Bot) {
	c := g.config
	dir := util.PosClock[b.CmdArg(1)%8]
	buildPos := botPos.AddRowCol(dir[0], dir[1])
	buildTypes := core.BuildTypesCount()
	buildType := core.BuildType(b.CmdArg(2) % buildTypes)
	if taskPos, taskType, ok := g.colonyTaskBuildDirective(botPos, b); ok {
		buildPos = taskPos
		buildType = taskType
	}

	if !g.Board.IsEmpty(buildPos) {
		return
	}
	if g.Board.IsFrozen(buildPos) {
		return
	}

	switch buildType {
	case core.BuildWall:
		if !g.canPayShared(b, 0, c.BuildingBuildCost) {
			b.PointerJumpBy(2)
			b.Genome.NextArg = 0
			return
		}
		g.Board.Set(buildPos, core.Building{
			Pos:   buildPos,
			Owner: b,
			Hp:    20,
		})
		g.spendShared(b, 0, c.BuildingBuildCost)
		b.Hp += c.BuildingBuildHpGain
		b.PointerJumpBy(1)
		b.Genome.NextArg = 1
		return
	case core.BuildColonyFlag:
		if b.Colony == nil || !b.ConnnectedToColony || b.Colony.FlagsCount() >= 3 {
			if g.tryBuildDepot(buildPos, b) {
				b.PointerJumpBy(10)
				b.Genome.NextArg = 10
				return
			}
			b.PointerJumpBy(2)
			return
		}
		flag := core.ColonyFlag{
			Pos: buildPos,
		}
		g.Board.Set(buildPos, flag)
		b.Colony.AddFlag(&flag)
		g.emitHomePheromone(buildPos, b.Colony, c.PheromoneHomeDeposit/2)
		b.PointerJumpBy(1)
		return
	case core.BuildDepot:
		if !g.tryBuildDepot(buildPos, b) {
			b.PointerJumpBy(10)
			b.Genome.NextArg = 0
			return
		}
		b.PointerJumpBy(10)
		b.Genome.NextArg = 10
		return
	case core.BuildSpawner:
		if !g.canPayShared(b, 0, c.SpawnerBuildCost) {
			b.PointerJumpBy(2)
			b.Genome.NextArg = 0
			return
		}
		g.Board.Set(buildPos, core.Spawner{
			Pos:    buildPos,
			Owner:  b,
			Colony: connectedBuilderColony(b),
			Amount: c.SpawnerInitialAmount,
		})
		g.spendShared(b, 0, c.SpawnerBuildCost)
		b.Evolution.SpawnerBuilds++
		g.emitEventPheromone(buildPos, core.PheromoneFood)
		b.PointerJumpBy(2)
		b.Genome.NextArg = 2
		return
	case core.BuildController:
		if g.hasControllerNear(buildPos, controllerBuildMinRadius) {
			if g.tryBuildDepot(buildPos, b) {
				b.PointerJumpBy(10)
				b.Genome.NextArg = 10
				return
			}
			b.PointerJumpBy(6)
			b.Genome.NextArg = 0
			return
		}
		if !g.canPayShared(b, 0, c.ControllerBuildCost) {
			b.PointerJumpBy(6)
			b.Genome.NextArg = 0
			return
		}
		newColony := b.Colony == nil
		colony, ok := g.controllerBuildColony(b, buildPos)
		if !ok {
			b.PointerJumpBy(6)
			b.Genome.NextArg = 0
			return
		}

		g.Board.Set(buildPos, core.Controller{
			Pos:         buildPos,
			Owner:       b,
			Colony:      colony,
			Amount:      c.ControllerInitialAmount,
			WaterAmount: 0,
		})
		g.emitHomePheromone(buildPos, colony, c.PheromoneHomeDeposit)
		b.AssignRandomColor()
		g.spendShared(b, 0, c.ControllerBuildCost)
		b.Hp += c.ControllerHpGain
		b.Evolution.ControllerBuilds++
		if newColony {
			g.buildInitialColonyInfrastructure(buildPos, b, colony)
		}
		b.PointerJumpBy(3)
		b.Genome.NextArg = 3
		return
	case core.BuildMine:
		if !g.canPayShared(b, 0, c.MineBuildCost) {
			b.PointerJumpBy(7)
			return
		}
		g.Board.Set(buildPos, core.Mine{
			Pos:    buildPos,
			Owner:  b,
			Amount: g.mineInitialAmount(buildPos),
		})
		g.emitEventPheromone(buildPos, core.PheromoneOre)
		g.spendShared(b, 0, c.MineBuildCost)
		b.Evolution.MineBuilds++
		b.PointerJumpBy(4)
		b.Genome.NextArg = 4
		return
	case core.BuildFarm:
		if c.DisableFarms {
			b.PointerJumpBy(9)
			return
		}
		if !g.canPayShared(b, 0, c.FarmBuildCost) {
			b.PointerJumpBy(8)
			return
		}
		g.Board.Set(buildPos, core.Farm{
			Pos:    buildPos,
			Owner:  b,
			Colony: connectedBuilderColony(b),
			Amount: g.colonyFarmInitialAmount(b),
		})
		g.emitEventPheromone(buildPos, core.PheromoneFood)
		b.Hp += c.FarmBuildHpGain
		g.spendShared(b, 0, c.FarmBuildCost)
		b.Evolution.FarmBuilds++
		b.PointerJumpBy(5)
		b.Genome.NextArg = 5
		return
	}
}

func (g *Game) tryBuildDepot(buildPos core.Position, b *core.Bot) bool {
	if b == nil || b.Colony == nil || !b.ConnnectedToColony {
		return false
	}
	if !g.canPayShared(b, 0, g.config.DepotBuildCost) {
		return false
	}
	g.Board.Set(buildPos, core.Depot{
		Pos:    buildPos,
		Owner:  b,
		Colony: b.Colony,
	})
	g.spendShared(b, 0, g.config.DepotBuildCost)
	b.Evolution.DepotBuilds++
	g.emitHomePheromone(buildPos, b.Colony, g.config.PheromoneHomeDeposit/3)
	return true
}

func (g *Game) hasControllerNear(center core.Position, radius int) bool {
	for r := center.R - radius; r <= center.R+radius; r++ {
		if r < 0 || r >= core.Rows {
			continue
		}
		for dc := -radius; dc <= radius; dc++ {
			pos := util.NewPos(r, center.C+dc)
			switch g.Board.At(pos).(type) {
			case core.Controller, *core.Controller:
				return true
			}
		}
	}
	return false
}

func (g *Game) controllerBuildColony(builder *core.Bot, buildPos core.Position) (*core.Colony, bool) {
	if builder == nil {
		return nil, false
	}
	if builder.Colony != nil {
		if !builder.ConnnectedToColony {
			return nil, false
		}
		g.recruitFriendlyBots(builder.Colony, buildPos, controllerRecruitRadius)
		g.claimNearbyColonyFarms(builder.Colony, buildPos, controllerRecruitRadius)
		g.connectNearbyColonyMembers(builder.Colony, buildPos, controllerRecruitRadius)
		return builder.Colony, true
	}
	if !g.hasNearbyFounderSupport(builder, buildPos, controllerRecruitRadius) {
		return nil, false
	}
	if !g.canFoundNewColony() {
		return nil, false
	}
	colony := core.NewColony(buildPos)
	root := g.liveLineageRoot(builder)
	colony.AddFamily(root)
	g.Colonies = append(g.Colonies, &colony)
	g.recruitFriendlyBots(&colony, buildPos, controllerRecruitRadius)
	g.claimNearbyColonyFarms(&colony, buildPos, controllerRecruitRadius)
	g.connectNearbyColonyMembers(&colony, buildPos, controllerRecruitRadius)
	g.initializeColonySpawnerGenome(&colony, builder)
	return &colony, true
}

func (g *Game) hasNearbyFounderSupport(builder *core.Bot, center core.Position, radius int) bool {
	if builder == nil || g == nil || g.Board == nil || radius < 0 {
		return false
	}
	found := false
	g.forEachLiveBotInRadius(center, radius, func(other *core.Bot) bool {
		if other == nil || other == builder {
			return true
		}
		if core.BotsFriendly(builder, other) {
			found = true
			return false
		}
		return true
	})
	return found
}

func connectedBuilderColony(builder *core.Bot) *core.Colony {
	if builder == nil || builder.Colony == nil || !builder.ConnnectedToColony {
		return nil
	}
	return builder.Colony
}

func (g *Game) mineInitialAmount(pos core.Position) int {
	base := g.config.MineGrabGain
	if base <= 0 {
		return 0
	}
	switch g.Board.BiomeAt(pos) {
	case core.BiomeMineral:
		return base * 2
	case core.BiomeToxic:
		return base + base/2
	default:
		return base
	}
}

func (g *Game) lookAround(botPos core.Position, b *core.Bot) {
	lookPos := b.CmdArgDir(2, botPos)
	switch v := g.Board.At(lookPos).(type) {
	case *core.Bot:
		other := g.Board.GetBot(lookPos)
		if other != nil {
			if b.IsBro(other) {
				b.PointerJumpBy(20)
				b.Genome.NextArg = 20
				return
			}
			b.PointerJumpBy(13)
			b.Genome.NextArg = 13
			return
		}
	case core.Building:
		b.PointerJumpBy(4)
		b.Genome.NextArg = 4
	case core.Wall:
		b.PointerJumpBy(8)
		b.Genome.NextArg = 8
	case core.Resource:
		b.PointerJumpBy(11)
		b.Genome.NextArg = 11
	case core.Controller:
		b.PointerJumpBy(50)
		b.Genome.NextArg = 50
	case *core.Controller:
		b.PointerJumpBy(50)
		b.Genome.NextArg = 50
	case core.Depot, *core.Depot:
		b.PointerJumpBy(52)
		b.Genome.NextArg = 52
	case core.Spawner:
		b.PointerJumpBy(61)
		b.Genome.NextArg = 61
	case core.Farm:
		b.PointerJumpBy(7)
		b.Genome.NextArg = 7
	case core.Food:
		b.PointerJumpBy(9)
		b.Genome.NextArg = 9
	case core.Poison:
		b.PointerJumpBy(14)
		b.Genome.NextArg = 14
	case core.Water:
		if b.Colony != nil && !b.Colony.KnowsWaterGroupId(v.GroupId) {
			b.Colony.AddWaterPosition(lookPos, v.GroupId)
		}
		// if b.Colony != nil {
		// 	marker := b.Colony.NewMarker(lookPos, bot.WaterMarker)
		// 	b.Colony.AddMarker(marker)
		// }
		b.Genome.NextArg = 15
		b.PointerJumpBy(15)
	case core.Organics:
		b.PointerJumpBy(3)
		b.Genome.NextArg = 3
	default:
		b.PointerJumpBy(12)
		b.Genome.NextArg = 12
	}
}

func (g *Game) liveBotCount() int {
	return g.Board.ActiveBotCount()
}

func idx(p core.Position) int {
	return util.Idx(p)
}
