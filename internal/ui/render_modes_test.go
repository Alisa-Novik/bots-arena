package ui

import (
	"golab/internal/core"
	"testing"
)

func TestRenderModeLabels(t *testing.T) {
	cases := map[RenderMode]string{
		RenderModeNormal:    "Normal",
		RenderModeGenome:    "Genome",
		RenderModeHealth:    "Health",
		RenderModeInventory: "Inventory",
		RenderModeColony:    "Colony",
		RenderModeTask:      "Task",
		RenderModeBiome:     "Biome",
		RenderModePheromone: "Pheromone",
	}
	for mode, want := range cases {
		if got := mode.Label(); got != want {
			t.Fatalf("mode %d label = %q, want %q", mode, got, want)
		}
	}
}

func TestCycleBiomeRenderModeMarksAllCellsDirty(t *testing.T) {
	brd = core.NewBoard()
	defer func() {
		brd = nil
		ctrlState = ControlState{}
	}()

	ctrlState.RenderMode = RenderModeTask
	cycleRenderMode()
	if ctrlState.RenderMode != RenderModeBiome {
		t.Fatalf("render mode = %s, want Biome", ctrlState.RenderMode.Label())
	}
	assertAllCellsDirty(t, brd)

	brd.PullPatch()
	cycleRenderMode()
	if ctrlState.RenderMode != RenderModePheromone {
		t.Fatalf("render mode = %s, want Pheromone", ctrlState.RenderMode.Label())
	}
	assertAllCellsDirty(t, brd)

	brd.PullPatch()
	cycleRenderMode()
	if ctrlState.RenderMode != RenderModeNormal {
		t.Fatalf("render mode = %s, want Normal", ctrlState.RenderMode.Label())
	}
	assertAllCellsDirty(t, brd)
}

func TestBiomeRenderModeTintsSoftCells(t *testing.T) {
	brd = core.NewBoard()
	defer func() {
		brd = nil
		ctrlState = ControlState{}
	}()

	ctrlState.RenderMode = RenderModeBiome
	ctrlState.HoveredIdx = -1

	var fertileIdx, mineralIdx int
	for idx := range brd.DirtyBitmap() {
		switch brd.BiomeAtIdx(idx) {
		case core.BiomeFertile:
			if fertileIdx == 0 {
				fertileIdx = idx
			}
		case core.BiomeMineral:
			if mineralIdx == 0 {
				mineralIdx = idx
			}
		}
	}
	if fertileIdx == 0 || mineralIdx == 0 {
		t.Fatalf("expected fertile and mineral biome cells")
	}

	fertileColor, fertileUV := pickSprite(nil, fertileIdx)
	mineralColor, mineralUV := pickSprite(nil, mineralIdx)
	if fertileColor == clrDefault || mineralColor == clrDefault {
		t.Fatalf("biome mode did not tint empty cells: fertile=%v mineral=%v", fertileColor, mineralColor)
	}
	if fertileColor == mineralColor {
		t.Fatalf("biome colors were not distinct: %v", fertileColor)
	}
	if fertileUV != uvLight || mineralUV != uvLight {
		t.Fatalf("biome empty cells should use light tile: fertile=%v mineral=%v", fertileUV, mineralUV)
	}

	pos := core.Position{R: fertileIdx / core.Cols, C: fertileIdx % core.Cols}
	bot := core.NewBot(pos)
	bot.Color = [3]float32{0.11, 0.22, 0.33}
	botColor, botUV := pickSprite(&bot, fertileIdx)
	if botColor != bot.Color || botUV != uvBot {
		t.Fatalf("biome mode changed bot readability: color=%v uv=%v", botColor, botUV)
	}
}

func assertAllCellsDirty(t *testing.T, brd *core.Board) {
	t.Helper()
	for idx, dirty := range brd.DirtyBitmap() {
		if !dirty {
			t.Fatalf("cell %d was not marked dirty", idx)
		}
	}
}

func TestAnalyticalBotRenderColors(t *testing.T) {
	bot := core.NewBot(core.Position{R: 10, C: 10})

	ctrlState.RenderMode = RenderModeHealth
	bot.Hp = 0
	lowHP := botRenderColor(&bot)
	bot.Hp = 500
	highHP := botRenderColor(&bot)
	if lowHP == highHP {
		t.Fatalf("health mode did not distinguish low and high HP")
	}

	ctrlState.RenderMode = RenderModeInventory
	bot.Inventory = core.Inventory{Food: 0, Ore: 50}
	empty := botRenderColor(&bot)
	bot.Inventory = core.Inventory{Food: 50, Ore: 50}
	full := botRenderColor(&bot)
	if empty == full {
		t.Fatalf("inventory mode did not distinguish unbalanced and division-ready inventory")
	}

	ctrlState.RenderMode = RenderModeGenome
	first := botRenderColor(&bot)
	second := botRenderColor(&bot)
	if first != second {
		t.Fatalf("genome mode is not stable: %v != %v", first, second)
	}
}

func TestColonyRenderModeColorsMembersStructuresAndHomeTissue(t *testing.T) {
	brd = core.NewBoard()
	defer func() {
		brd = nil
		ctrlState = ControlState{}
	}()

	ctrlState.RenderMode = RenderModeColony
	ctrlState.HoveredIdx = -1

	colony := core.NewColony(core.Position{R: 10, C: 10})
	colony.Color = [3]float32{0.12, 0.78, 0.34}
	connectedPos := core.Position{R: 10, C: 11}
	connected := core.NewBot(connectedPos)
	connected.Color = [3]float32{0.90, 0.10, 0.10}
	connected.ConnnectedToColony = true
	colony.AddFamily(&connected)
	disconnectedPos := core.Position{R: 10, C: 12}
	disconnected := core.NewBot(disconnectedPos)
	disconnected.Color = connected.Color
	colony.AddFamily(&disconnected)

	brd.AddBot(connectedPos, &connected)
	brd.AddBot(disconnectedPos, &disconnected)
	ctrlPos := core.Position{R: 10, C: 10}
	depotPos := core.Position{R: 10, C: 13}
	farmPos := core.Position{R: 10, C: 14}
	spawnerPos := core.Position{R: 10, C: 15}
	flagPos := core.Position{R: 10, C: 16}
	tissuePos := core.Position{R: 10, C: 17}
	brd.Set(ctrlPos, core.Controller{Pos: ctrlPos, Owner: &connected, Colony: &colony})
	brd.Set(depotPos, core.Depot{Pos: depotPos, Owner: &connected, Colony: &colony})
	brd.Set(farmPos, core.Farm{Pos: farmPos, Owner: &connected, Colony: &colony})
	brd.Set(spawnerPos, core.Spawner{Pos: spawnerPos, Owner: &connected, Amount: 1})
	brd.Set(flagPos, core.ColonyFlag{Pos: flagPos})
	brd.DepositPheromone(flagPos, core.PheromoneHome, 80, &colony)
	brd.DepositPheromone(tissuePos, core.PheromoneHome, 80, &colony)

	connectedColor, connectedUV := pickSprite(&connected, core.Cols*connectedPos.R+connectedPos.C)
	disconnectedColor, _ := pickSprite(&disconnected, core.Cols*disconnectedPos.R+disconnectedPos.C)
	if connectedUV != uvBot {
		t.Fatalf("connected colony member uv = %v, want bot", connectedUV)
	}
	if colorDistance(connectedColor, colony.Color) >= colorDistance(disconnectedColor, colony.Color) {
		t.Fatalf("connected color %v should be closer to colony %v than disconnected %v", connectedColor, colony.Color, disconnectedColor)
	}

	for _, tc := range []struct {
		name string
		pos  core.Position
		uv   [4]float32
	}{
		{name: "controller", pos: ctrlPos, uv: uvChest},
		{name: "depot", pos: depotPos, uv: uvChest},
		{name: "farm", pos: farmPos, uv: uvFarm},
		{name: "spawner", pos: spawnerPos, uv: uvSpawner},
		{name: "flag", pos: flagPos, uv: uvFlag},
	} {
		color, uv := pickSprite(brd.At(tc.pos), core.Cols*tc.pos.R+tc.pos.C)
		if uv != tc.uv {
			t.Fatalf("%s uv = %v, want %v", tc.name, uv, tc.uv)
		}
		if colorDistance(color, colony.Color) >= colorDistance(clrLight, colony.Color) {
			t.Fatalf("%s color = %v, want tinted toward colony %v", tc.name, color, colony.Color)
		}
	}

	tissueColor, tissueUV := pickSprite(nil, core.Cols*tissuePos.R+tissuePos.C)
	if tissueUV != uvLight {
		t.Fatalf("home tissue uv = %v, want light tile", tissueUV)
	}
	if tissueColor == clrDefault || colorDistance(tissueColor, colony.Color) >= colorDistance(clrDefault, colony.Color) {
		t.Fatalf("home tissue color = %v, want faint colony tint %v", tissueColor, colony.Color)
	}
}

func colorDistance(a, b [3]float32) float32 {
	dr := a[0] - b[0]
	dg := a[1] - b[1]
	db := a[2] - b[2]
	return dr*dr + dg*dg + db*db
}

func TestPheromoneRenderModeUsesChannelBlend(t *testing.T) {
	brd = core.NewBoard()
	defer func() {
		brd = nil
		ctrlState = ControlState{}
	}()

	pos := core.Position{R: 10, C: 10}
	cellIdx := core.Cols*pos.R + pos.C
	brd.DepositPheromone(pos, core.PheromoneFood, 255, nil)
	ctrlState.RenderMode = RenderModePheromone
	ctrlState.HoveredIdx = -1

	color, uv := pickSprite(nil, cellIdx)
	if uv != uvLight {
		t.Fatalf("pheromone cell uv = %v, want light tile", uv)
	}
	if color[0] <= 0.9 || color[2] <= 0.7 {
		t.Fatalf("food pheromone color = %v, want bright magenta blend", color)
	}

	emptyColor, emptyUV := pheromoneSprite(core.PheromoneValues{})
	if emptyUV != uvDark || emptyColor == color {
		t.Fatalf("empty pheromone sprite = %v/%v, want dark distinct cell", emptyColor, emptyUV)
	}
}

func TestDensityChunksCountBotsAndUpdate(t *testing.T) {
	brd := core.NewBoard()
	firstPos := core.Position{R: 12, C: 12}
	secondPos := core.Position{R: 13, C: 13}
	movePos := core.Position{R: 40, C: 40}
	first := core.NewBot(firstPos)
	second := core.NewBot(secondPos)
	brd.AddBot(firstPos, &first)
	brd.AddBot(secondPos, &second)

	chunks := BuildDensityChunks(brd, DensityChunkSize)
	if got := densityCountAt(chunks, 12, 12); got != 2 {
		t.Fatalf("initial density count = %d, want 2; chunks=%+v", got, chunks)
	}

	if !brd.MoveBot(firstPos, movePos, &first) {
		t.Fatalf("MoveBot returned false")
	}
	brd.RemoveBotAt(secondPos)
	chunks = BuildDensityChunks(brd, DensityChunkSize)
	if got := densityCountAt(chunks, 12, 12); got != 0 {
		t.Fatalf("old density count = %d, want 0; chunks=%+v", got, chunks)
	}
	if got := densityCountAt(chunks, 40, 40); got != 1 {
		t.Fatalf("moved density count = %d, want 1; chunks=%+v", got, chunks)
	}
}

func densityCountAt(chunks []DensityChunk, row, col int) int {
	for _, chunk := range chunks {
		if chunk.Row == row && chunk.Col == col {
			return chunk.Count
		}
	}
	return 0
}

var benchmarkDensityChunks []DensityChunk

func BenchmarkDensityRender100k(b *testing.B) {
	brd := core.NewBoard()
	seedBenchmarkDensityBots(b, brd, 100000)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		benchmarkDensityChunks = buildDensityChunksInto(benchmarkDensityChunks[:0], brd, DensityChunkSize, RenderModeNormal)
	}
}

func seedBenchmarkDensityBots(tb testing.TB, brd *core.Board, target int) {
	tb.Helper()
	seeded := 0
	for cellIdx := 0; cellIdx < core.Rows*core.Cols && seeded < target; cellIdx++ {
		pos := core.Position{R: cellIdx / core.Cols, C: cellIdx % core.Cols}
		if brd.IsWall(pos) || !brd.IsEmpty(pos) {
			continue
		}
		bot := core.NewBot(pos)
		brd.AddBot(pos, &bot)
		seeded++
	}
	if seeded != target {
		tb.Fatalf("seeded %d density bots, want %d", seeded, target)
	}
}
