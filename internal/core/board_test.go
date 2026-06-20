package core

import (
	"golab/internal/util"
	"slices"
	"testing"
)

func firstCoreBiomeCell(t *testing.T, brd *Board, biome Biome) util.Position {
	t.Helper()
	for idx := 0; idx < util.Cells; idx++ {
		pos := util.PosOf(idx)
		if pos.R <= 0 || pos.R >= Rows-1 {
			continue
		}
		if brd.BiomeAtIdx(idx) == biome {
			return pos
		}
	}
	t.Fatalf("no %s biome cell found", biome)
	return util.Position{}
}

func TestSetNilClearsOccupancy(t *testing.T) {
	brd := NewBoard()
	center := util.NewPos(20, 20)
	target := center.AddDir(Up)

	for _, dir := range PosClock {
		pos := center.AddDir(dir)
		if pos == target {
			brd.Set(pos, Resource{Pos: pos, Amount: 1})
			brd.Set(pos, nil)
			continue
		}
		brd.Set(pos, Wall{Pos: pos})
	}

	got, ok := brd.FindEmptyPosAround(center)
	if !ok {
		t.Fatal("expected cleared cell to be reusable")
	}
	if got != target {
		t.Fatalf("empty position = %v, want %v", got, target)
	}
}

func TestActiveBotRegistryTracksAddMoveAndRemove(t *testing.T) {
	brd := NewBoard()
	start := util.NewPos(20, 20)
	next := util.NewPos(20, 21)
	bot := NewBot(start)

	if !brd.AddBot(start, &bot) {
		t.Fatalf("AddBot returned false")
	}
	if got := brd.ActiveBotCount(); got != 1 {
		t.Fatalf("active bot count after add = %d, want 1", got)
	}
	if got := brd.GetBot(start); got != &bot {
		t.Fatalf("GetBot(start) = %p, want %p", got, &bot)
	}
	if brd.Bots[util.Idx(start)] != &bot {
		t.Fatalf("legacy bot slot was not populated")
	}
	if _, ok := brd.At(start).(*Bot); !ok {
		t.Fatalf("grid occupant at start = %T, want *Bot", brd.At(start))
	}

	if !brd.MoveBot(start, next, &bot) {
		t.Fatalf("MoveBot returned false")
	}
	if got := brd.ActiveBotCount(); got != 1 {
		t.Fatalf("active bot count after move = %d, want 1", got)
	}
	if got := brd.GetBot(start); got != nil {
		t.Fatalf("GetBot(start) after move = %p, want nil", got)
	}
	if got := brd.GetBot(next); got != &bot {
		t.Fatalf("GetBot(next) = %p, want %p", got, &bot)
	}
	if bot.Pos != next {
		t.Fatalf("bot position after move = %v, want %v", bot.Pos, next)
	}

	removed := brd.RemoveBotAt(next)
	if removed != &bot {
		t.Fatalf("removed bot = %p, want %p", removed, &bot)
	}
	if got := brd.ActiveBotCount(); got != 0 {
		t.Fatalf("active bot count after remove = %d, want 0", got)
	}
	if got := brd.GetBot(next); got != nil {
		t.Fatalf("GetBot(next) after remove = %p, want nil", got)
	}
	if !brd.IsEmpty(next) {
		t.Fatalf("removed bot cell should be empty")
	}
}

func TestSortedActiveEnvironmentCellsInvalidatesOnSetAndClear(t *testing.T) {
	brd := NewBoard()
	low := util.NewPos(10, 10)
	mid := util.NewPos(20, 20)
	high := util.NewPos(30, 30)

	brd.Set(high, Poison{Pos: high})
	brd.Set(low, Farm{Pos: low})
	got := brd.SortedActiveEnvironmentCells(nil)
	want := []int{util.Idx(low), util.Idx(high)}
	if !slices.Equal(got, want) {
		t.Fatalf("sorted active cells after initial set = %v, want %v", got, want)
	}

	brd.Set(mid, Depot{Pos: mid})
	got = brd.SortedActiveEnvironmentCells(nil)
	want = []int{util.Idx(low), util.Idx(mid), util.Idx(high)}
	if !slices.Equal(got, want) {
		t.Fatalf("sorted active cells after add = %v, want %v", got, want)
	}

	brd.Clear(low)
	got = brd.SortedActiveEnvironmentCells(nil)
	want = []int{util.Idx(mid), util.Idx(high)}
	if !slices.Equal(got, want) {
		t.Fatalf("sorted active cells after clear = %v, want %v", got, want)
	}
}

func TestNewBoardInitializesNeighbourTableOnlyOnce(t *testing.T) {
	NewBoard()

	center := util.NewPos(20, 20)
	tableIdx := idx(center)
	original := neighbourIdx[tableIdx][0]
	const sentinel = -12345
	neighbourIdx[tableIdx][0] = sentinel
	defer func() {
		neighbourIdx[tableIdx][0] = original
	}()

	NewBoard()

	if got := neighbourIdx[tableIdx][0]; got != sentinel {
		t.Fatalf("neighbor table entry after repeated NewBoard = %d, want sentinel %d", got, sentinel)
	}
}

func TestDirtyPatchSuppressesDuplicatesAndClears(t *testing.T) {
	brd := NewBoard()
	first := util.Idx(util.NewPos(10, 10))
	second := util.Idx(util.NewPos(10, 11))

	brd.MarkDirty(first)
	brd.MarkDirty(first)
	brd.MarkDirty(second)

	if got, want := brd.PullPatch(), []int{first, second}; !slices.Equal(got, want) {
		t.Fatalf("patch = %v, want %v", got, want)
	}
	if brd.DirtyBitmap()[first] || brd.DirtyBitmap()[second] {
		t.Fatalf("dirty bitmap was not cleared for pulled patch cells")
	}
	if got := brd.PullPatch(); len(got) != 0 {
		t.Fatalf("patch after clearing = %v, want empty", got)
	}

	brd.MarkDirty(first)
	if got, want := brd.PullPatch(), []int{first}; !slices.Equal(got, want) {
		t.Fatalf("patch after re-mark = %v, want %v", got, want)
	}
}

func TestFrozenCellsMarkDirtyAndCopyToNewBoard(t *testing.T) {
	brd := NewBoard()
	pos := util.NewPos(12, 14)
	cellIdx := util.Idx(pos)

	if brd.IsFrozen(pos) {
		t.Fatalf("new board cell starts frozen")
	}
	if !brd.SetFrozen(pos, true) {
		t.Fatalf("expected freezing cell to report a change")
	}
	if !brd.IsFrozen(pos) || !brd.IsFrozenIdx(cellIdx) {
		t.Fatalf("cell was not frozen")
	}
	if got := brd.PullPatch(); !slices.Equal(got, []int{cellIdx}) {
		t.Fatalf("freeze patch = %v, want [%d]", got, cellIdx)
	}
	if brd.SetFrozen(pos, true) {
		t.Fatalf("freezing an already frozen cell reported a change")
	}

	next := NewBoard()
	next.CopyFrozenFrom(brd)
	if !next.IsFrozen(pos) {
		t.Fatalf("copied board did not preserve frozen cell")
	}
	if !next.SetFrozen(pos, false) {
		t.Fatalf("expected thawing cell to report a change")
	}
	if next.IsFrozen(pos) {
		t.Fatalf("cell was not thawed")
	}
}

func TestBiomeGenerationIsDeterministicAndDiverse(t *testing.T) {
	first := NewBoard()
	second := NewBoard()
	counts := map[Biome]int{}

	for idx := 0; idx < util.Cells; idx++ {
		biome := first.BiomeAtIdx(idx)
		if biome != second.BiomeAtIdx(idx) {
			t.Fatalf("biome at idx %d changed between boards: %s vs %s", idx, biome, second.BiomeAtIdx(idx))
		}
		if got := first.BiomeAt(util.PosOf(idx)); got != biome {
			t.Fatalf("BiomeAt idx %d = %s, want %s", idx, got, biome)
		}
		counts[biome]++
	}

	for _, biome := range []Biome{BiomeNeutral, BiomeFertile, BiomeMineral, BiomeToxic} {
		if counts[biome] == 0 {
			t.Fatalf("biome %s was not generated; counts=%v", biome, counts)
		}
	}
}

func TestCopyBiomesPreservesLayerWithoutChangingFrozen(t *testing.T) {
	source := NewBoard()
	frozenPos := util.NewPos(12, 14)
	source.SetFrozen(frozenPos, true)

	target := NewBoard()
	target.CopyBiomesFrom(source)

	for idx := 0; idx < util.Cells; idx++ {
		if got, want := target.BiomeAtIdx(idx), source.BiomeAtIdx(idx); got != want {
			t.Fatalf("copied biome at idx %d = %s, want %s", idx, got, want)
		}
	}
	if target.IsFrozen(frozenPos) {
		t.Fatalf("CopyBiomesFrom changed frozen state")
	}
}

func TestPheromoneDepositCapsAndIgnoresInvalid(t *testing.T) {
	brd := NewBoard()
	pos := util.NewPos(10, 10)
	cellIdx := util.Idx(pos)

	if !brd.DepositPheromone(pos, PheromoneFood, 300, nil) {
		t.Fatalf("expected food pheromone deposit")
	}
	if got := brd.PheromoneAt(pos).Food; got != 255 {
		t.Fatalf("food pheromone = %d, want capped 255", got)
	}
	if brd.DepositPheromone(pos, PheromoneOre, 0, nil) {
		t.Fatalf("zero deposit reported a change")
	}
	if brd.DepositPheromone(pos, PheromoneChannel(99), 10, nil) {
		t.Fatalf("invalid channel deposit reported a change")
	}
	if brd.DepositPheromone(util.NewPos(-1, 10), PheromoneFood, 10, nil) {
		t.Fatalf("out-of-bounds deposit reported a change")
	}
	if got, want := brd.PullPatch(), []int{cellIdx}; !slices.Equal(got, want) {
		t.Fatalf("pheromone dirty patch = %v, want %v", got, want)
	}
}

func TestPheromoneDecayPrunesInactiveCellsAndMarksDirty(t *testing.T) {
	brd := NewBoard()
	pos := firstCoreBiomeCell(t, brd, BiomeNeutral)
	cellIdx := util.Idx(pos)
	brd.DepositPheromone(pos, PheromoneFood, 3, nil)
	brd.DepositPheromone(pos, PheromoneDanger, 2, nil)
	brd.PullPatch()

	if changed := brd.DecayPheromones(2); changed != 1 {
		t.Fatalf("first decay changed cells = %d, want 1", changed)
	}
	values := brd.PheromoneAt(pos)
	if values.Food != 1 || values.Danger != 0 {
		t.Fatalf("pheromones after first decay = %+v, want food 1 danger 0", values)
	}
	if totals := brd.PheromoneTotals(); totals.ActiveCells != 1 {
		t.Fatalf("active cells after first decay = %d, want 1", totals.ActiveCells)
	}
	if got, want := brd.PullPatch(), []int{cellIdx}; !slices.Equal(got, want) {
		t.Fatalf("first decay patch = %v, want %v", got, want)
	}

	if changed := brd.DecayPheromones(2); changed != 1 {
		t.Fatalf("second decay changed cells = %d, want 1", changed)
	}
	if values := brd.PheromoneAt(pos); !values.IsZero() {
		t.Fatalf("pheromones after second decay = %+v, want zero", values)
	}
	if totals := brd.PheromoneTotals(); totals.ActiveCells != 0 {
		t.Fatalf("active cells after prune = %d, want 0", totals.ActiveCells)
	}
	if got, want := brd.PullPatch(), []int{cellIdx}; !slices.Equal(got, want) {
		t.Fatalf("second decay patch = %v, want %v", got, want)
	}
}

func TestPheromoneDiffusionReachesCardinalNeighborsDeterministically(t *testing.T) {
	brd := NewBoard()
	pos := util.NewPos(10, 10)
	brd.DepositPheromone(pos, PheromoneFood, 8, nil)
	brd.PullPatch()

	if changed := brd.DiffusePheromones(1); changed != 5 {
		t.Fatalf("diffusion changed cells = %d, want 5", changed)
	}
	if got := brd.PheromoneAt(pos).Food; got != 4 {
		t.Fatalf("source food after diffusion = %d, want 4", got)
	}
	for _, dir := range Dirs {
		neighbor := pos.AddDir(dir)
		if got := brd.PheromoneAt(neighbor).Food; got != 1 {
			t.Fatalf("neighbor %v food after diffusion = %d, want 1", neighbor, got)
		}
	}
	if totals := brd.PheromoneTotals(); totals.ActiveCells != 5 || totals.Food != 8 {
		t.Fatalf("diffusion totals = %+v, want 5 active and total food 8", totals)
	}
}

func TestPheromoneHomeReadsForSameColonyAndForeignDanger(t *testing.T) {
	brd := NewBoard()
	pos := util.NewPos(10, 10)
	homeColony := NewColony(pos)
	foreignColony := NewColony(util.NewPos(12, 12))
	homeBot := NewBot(util.NewPos(10, 11))
	foreignBot := NewBot(util.NewPos(10, 12))
	homeColony.AddFamily(&homeBot)
	foreignColony.AddFamily(&foreignBot)

	brd.DepositPheromone(pos, PheromoneHome, 40, &homeColony)
	brd.DepositPheromone(pos, PheromoneDanger, 5, nil)

	if got := brd.PheromoneValueForBot(pos, PheromoneHome, &homeBot); got != 40 {
		t.Fatalf("same-colony home read = %d, want 40", got)
	}
	if got := brd.PheromoneValueForBot(pos, PheromoneHome, &foreignBot); got != 0 {
		t.Fatalf("foreign home read = %d, want hidden 0", got)
	}
	if got := brd.PheromoneValueForBot(pos, PheromoneDanger, &homeBot); got != 5 {
		t.Fatalf("same-colony danger read = %d, want raw 5", got)
	}
	if got := brd.PheromoneValueForBot(pos, PheromoneDanger, &foreignBot); got != 45 {
		t.Fatalf("foreign danger read = %d, want danger plus foreign home 45", got)
	}
}

func TestPheromoneDecayUsesBiomeAndWaterModifiers(t *testing.T) {
	brd := NewBoard()
	fertilePos := firstCoreBiomeCell(t, brd, BiomeFertile)
	mineralPos := firstCoreBiomeCell(t, brd, BiomeMineral)
	toxicPos := firstCoreBiomeCell(t, brd, BiomeToxic)
	waterPos := firstCoreBiomeCell(t, brd, BiomeNeutral)

	brd.DepositPheromone(fertilePos, PheromoneFood, 4, nil)
	brd.DepositPheromone(mineralPos, PheromoneOre, 4, nil)
	brd.DepositPheromone(toxicPos, PheromoneDanger, 4, nil)
	brd.Set(waterPos, Water{GroupId: 1, Amount: 1})
	brd.DepositPheromone(waterPos, PheromoneHome, 5, nil)

	brd.DecayPheromones(2)

	if got := brd.PheromoneAt(fertilePos).Food; got != 3 {
		t.Fatalf("fertile food decay = %d, want 3", got)
	}
	if got := brd.PheromoneAt(mineralPos).Ore; got != 3 {
		t.Fatalf("mineral ore decay = %d, want 3", got)
	}
	if got := brd.PheromoneAt(toxicPos).Danger; got != 3 {
		t.Fatalf("toxic danger decay = %d, want 3", got)
	}
	if got := brd.PheromoneAt(waterPos).Home; got != 1 {
		t.Fatalf("water home decay = %d, want 1", got)
	}
}

func TestCopyPheromonesPreservesLayerAndOwners(t *testing.T) {
	source := NewBoard()
	target := NewBoard()
	pos := util.NewPos(10, 10)
	colony := NewColony(pos)
	source.DepositPheromone(pos, PheromoneHome, 24, &colony)
	source.DepositPheromone(pos, PheromoneOre, 11, nil)

	target.CopyPheromonesFrom(source)

	if got := target.PheromoneAt(pos); got.Home != 24 || got.Ore != 11 {
		t.Fatalf("copied pheromones = %+v, want home 24 ore 11", got)
	}
	if owner := target.PheromoneHomeOwnerAt(pos); owner != &colony {
		t.Fatalf("copied home owner = %p, want %p", owner, &colony)
	}
	if totals := target.PheromoneTotals(); totals.ActiveCells != 1 || totals.Home != 24 || totals.Ore != 11 {
		t.Fatalf("copied totals = %+v", totals)
	}
}

func BenchmarkBoardDirtyPatch(b *testing.B) {
	brd := NewBoard()
	indices := make([]int, 1024)
	for i := range indices {
		indices[i] = (i * 97) % util.Cells
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		for _, cellIdx := range indices {
			brd.MarkDirty(cellIdx)
			brd.MarkDirty(cellIdx)
		}
		if patch := brd.PullPatch(); len(patch) == 0 {
			b.Fatal("expected dirty patch")
		}
	}
}
