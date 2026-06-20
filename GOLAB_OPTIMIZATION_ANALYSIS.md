# Golab Optimization Analysis And AI-Ready Checklist

Prepared: 2026-06-18

Scope of this file:

- Static repository analysis only.
- No Golab run, replay, render, build, test, profiling, benchmark, screenshot, or runtime data collection was performed for this preparation pass.
- Treat every performance claim below as a hypothesis until the measurement checklist is executed.
- Existing worktree was already dirty before this file was created. Do not overwrite or revert unrelated local changes.

Project anchors:

- Root: `/home/alice/projects/golab`
- Main binary: `cmd/golab`
- Simulation core: `internal/game`, `internal/core`, `internal/tasking`
- Interactive UI: `internal/ui`
- PNG renderer and deterministic CLI render path: `internal/render`
- Current board size from `internal/util/util.go`: `Rows = 40 * 7 = 280`, `Cols = 60 * 7 = 420`, `Cells = 117600`

## Optimization Goal

Make Golab faster and more scalable without changing simulation semantics unless the change is explicitly chosen and covered by deterministic tests.

Primary target surfaces:

- Interactive FPS and frame stability.
- Headless deterministic CLI throughput for `status`, `match`, `leaderboard`, `replay`, `gamemaster`, and `render`.
- Per-tick simulation throughput as live bot counts rise.
- Allocation pressure from pathfinding, task assignment, board resets, summaries, and render updates.

## Current Static Hotspot Hypotheses

These are ordered by likely impact and implementation value. Validate with the measurement phase before making broad changes.

### P0: Establish Baselines Before Changing Code

Do not optimize blind. The repo already contains `cpu.out` and `cpu1.out`, but this prep pass did not inspect or regenerate them. They may be stale or from a different code shape.

Next phase must collect:

- CPU profiles for representative interactive and headless scenarios.
- Allocation profiles for headless simulation, pathfinding/task-heavy cases, and render command.
- Deterministic replay/status output before and after any optimization.
- Benchmarks for isolated hot functions before refactors.

Acceptance rule: every optimization PR should name the measured bottleneck, show before/after numbers, and include semantic guard tests.

### P1: Full-Board And Full-Bot Scans Are Everywhere

The grid has 117,600 cells. Several paths scan all cells or all bot slots frequently:

- `internal/game/game.go:288`: `runLogicTick` calls `g.liveBotCount()` every tick.
- `internal/game/game.go:1142`: `liveBotCount` scans all entries in `g.Board.Bots`.
- `internal/game/game.go:419`: `botsActions` scans the full `Bots` slice, not just live bots.
- `internal/game/game.go:107`: `environmentActions` scans every board cell on active ticks.
- `internal/ui/ui.go:413`: `DrawGrid` scans all bot slots to mark bots dirty every frame.
- `internal/ui/ui.go:440`: `DrawGrid` scans the full dirty bitmap every frame.
- `internal/game/gamemaster.go:115`: `ObserveMaster` scans bots and grid.
- `cmd/golab/commands.go:589`: `summarizeMatch` scans bots and grid, and sorts all live bots.

Likely direction:

- Add explicit live bot count maintenance to `Board` or `Game`.
- Add active bot index storage so bot processing and render dirty marking do not scan empty slots.
- Add active cell sets by cell class for dynamic environment work: organics, farms, controllers, food/resources/poison if needed.
- Maintain board summary counters incrementally inside `Board.Set` and `Board.Clear`, or add a stats subsystem with tests.
- Keep full-grid scans available as debug assertions and benchmark baselines.

### P1: Dirty Rendering Has A Patch List But Still Scans The Bitmap

`Board` already tracks dirty cells with `patch`:

- `internal/core/board.go:79`: `PullPatch` returns dirty indices and clears dirty flags.
- `internal/core/board.go:92`: `MarkDirty` appends dirty indices once.

But interactive rendering uses a full dirty bitmap scan:

- `internal/ui/ui.go:440`: loops over `brd.DirtyBitmap()`.
- `internal/ui/ui.go:465`: cleans each dirty index.

Likely direction:

- Replace the full bitmap scan with `brd.PullPatch()` in `DrawGrid`.
- Preserve contiguous-run `gl.BufferSubData` batching by sorting patch indices or by accepting unordered smaller uploads after measuring.
- Mark only visually changed bot cells dirty: old position, new position, color/selection changes, hover changes, path overlays.
- Stop marking every live bot dirty each frame unless profiling proves it is necessary.
- Add tests around patch uniqueness, clearing, and expected dirty cells after `Set`, `Clear`, movement, hover, and path overlay changes.

### P1: Pathfinding And Colony Tasking Allocate Heavily

Likely allocation-heavy code:

- `internal/tasking/pathfinding.go:88`: A* allocates maps for `gScore`, `prev`, and `closed`.
- `internal/tasking/pathfinding.go:100`: path reconstruction prepends with `append([]Position{p}, path...)`, which is O(path length squared).
- `internal/tasking/pathfinding.go:46`: `CalcFlowField` allocates a full `[]int16` per call.
- `internal/tasking/tasking.go:17`: each colony recalculates flow field every fifth counter.
- `internal/tasking/tasking.go:111`: `SortedFreeBots` allocates pairs and output slices and sorts free bots.
- `internal/core/colony.go:113`: `SetPathToWater` allocates full-size mask and index slices for each path.

Likely direction:

- Replace A* maps with fixed-size arrays indexed by `util.Idx(pos)` using generation stamps.
- Reconstruct paths by appending backward then reversing once.
- Reuse flow-field buffers per colony or from a scratch pool, with clear invalidation rules.
- Consider path/flow cache invalidation keyed by board topology version if water path tasks repeat.
- Replace full free-bot sort with nearest-N or bucketed assignment if only a sequential task fill is needed.
- Add benchmark coverage for `CalcPath`, `CalcFlowField`, and `ProcessColonyTasks`.

### P1: Environment Work Should Be Event-Set Based

`environmentActions` visits every cell every logic tick:

- `internal/game/game.go:107`: nested `Rows x Cols` loop.
- It only acts on `Organics`, `Controller`, and `Farm`.

Likely direction:

- Maintain active sets for environment actors:
  - organic cells that decay.
  - farm cells that can produce food.
  - controller cells that drive colony logic.
- Update these sets in `Board.Set` and `Board.Clear`.
- Iterate those sets each tick instead of scanning the full grid.
- Retain a debug-only full scan to verify set consistency.

### P2: Board Reset And Initialization Churn

Generation reset paths recreate the board:

- `internal/game/game.go:345`: `populateBoard` assigns `g.Board = core.NewBoard()`.
- `internal/game/game.go:348`: marks the full board dirty.
- `internal/core/board.go:125`: `NewBoard` calls `initNeighbourTable` every time.
- `internal/core/board.go:109`: neighbor table initialization is deterministic global work.

Likely direction:

- Guard neighbor table initialization with `sync.Once`.
- Reuse board slices in a `Reset` method instead of allocating a new board on generation resets.
- Preserve UI board pointer update semantics carefully if board reuse is introduced.
- Split static wall initialization from dynamic cell repopulation.
- Treat `MarkAllDirty` as necessary only for visual board replacement, not for headless command mode.

### P2: CLI Summaries Sort All Bots To Return Top-N

`summarizeMatch` builds every live bot summary and sorts the whole list:

- `cmd/golab/commands.go:611`: `botSummaries := make([]botSummary, 0, len(g.Board.Bots))`.
- `cmd/golab/commands.go:644`: sorts all bot summaries.
- `cmd/golab/commands.go:654`: slices to top-N.

Likely direction:

- Use a fixed-size min-heap or selection algorithm for top-N.
- Combine bot summary counting and top-N selection in one pass.
- Combine grid object counting and colony registration in one grid pass.
- Use cached board counters if an incremental stats layer is added.

### P2: Game-Master Observation Duplicates Summary Work

`ObserveMaster` scans bot slots and grid:

- `internal/game/gamemaster.go:124`: bot pass.
- `internal/game/gamemaster.go:135`: grid pass.

Likely direction:

- Reuse the same board stats system used by command summaries.
- Maintain colony counts by controller/colony lifecycle, or compute from a small colony/controller list.
- Keep one tested canonical summary function so command and game-master stats cannot drift.

### P2: Occupant Interface Grid Has Type-Assertion Costs

The board stores `[]Occupant` where `Occupant` is `any`:

- `internal/core/board.go:8`: `type Occupant any`.
- `internal/core/board.go:60`: `grid []Occupant`.

This is convenient but causes many type switches/assertions in hot loops:

- Simulation environment actions.
- Rendering sprite selection.
- CLI summaries.
- Game-master observation.

Likely direction:

- Do not rewrite this first unless measurement proves it matters.
- If needed, introduce a `CellKind` byte slice beside `grid` to make counting and rendering faster without immediately replacing typed payloads.
- Longer-term: split static terrain, dynamic resources, bots, and structures into typed arrays/maps.

### P2: Bot Lifecycle And Lineage May Be Costly Or Bug-Prone

Potential static issues:

- `internal/core/bot.go:141`: `NewChild` has a 0.5 percent branch that returns `BotPool.Get().(*Bot)` without initialization. This looks like a correctness bug, not an optimization.
- `internal/core/bot.go:26`: every bot has an `Offsprings map[*Bot]struct{}`.
- `internal/core/bot.go:178`: `CountOffsprings` recursively walks offspring maps.
- `internal/core/genome.go:153`: `IsBro` scans the genome matrix.

Likely direction:

- Investigate and fix the uninitialized child path before relying on optimization results.
- Only optimize lineage maps if profiling shows meaningful time or allocation pressure.
- Consider replacing recursive lineage checks with colony/family IDs if behavior permits.

### P3: Interactive UI Uses Legacy Immediate-Mode Overlay And Always-Draws Dynamic Buffer

Potential UI costs:

- `internal/ui/ui.go:335`: overlay uses immediate mode every frame.
- `internal/ui/ui.go:480`: static layer draw.
- `internal/ui/ui.go:483`: dynamic layer draws `dynVertCount`, currently initialized to all cells in `PrepareUi`.
- `internal/ui/ui.go:486`: overlay draws every frame.
- `cmd/golab/main.go:27`: interactive main always creates `cpu.out`.
- `cmd/golab/main.go:58`: interactive main always starts CPU profiling.

Likely direction:

- Gate profiling behind a flag or environment variable.
- Use `PullPatch` first, then evaluate whether dynamic layer should draw all cells or only dirty/visible chunks.
- Consider viewport culling only after measuring.
- Batch or simplify overlay text if it appears in profiles.

### P3: PNG Renderer Is Command-Oriented And Pixel-Heavy

`internal/render/board.go` loops over every cell and, for atlas/game styles, every pixel in a cell:

- `internal/render/board.go:139`: full board loop.
- `internal/render/board.go:197`: `drawCell`.
- `internal/render/board.go:205`: `drawTile` per-pixel tinting and alpha blending.

Likely direction:

- Leave this alone unless render command throughput matters.
- Cache tinted tile images for common tile/tint combinations.
- Prefer `--style flat` for diagnostic images where pixel-art fidelity is not required.

## Risk Map

High-risk optimization areas:

- Changing bot iteration order can change deterministic outcomes because random calls and movement conflicts are order-sensitive.
- Changing summary timestamps or cooldown calculations can alter JSON output beyond performance-sensitive fields.
- Dirty rendering changes can create stale visual cells if every visual state change is not marked dirty.
- Reusing flow fields or board stats can create stale pathing or counts if invalidation misses a board mutation.
- Replacing full-board scans with active sets requires strong consistency tests for `Board.Set`, `Board.Clear`, movement, spawn, death, build, grab, master events, and populate/reset.

Low-risk likely first wins:

- Add benchmark files and profiling commands.
- Guard global neighbor-table init with `sync.Once`.
- Gate always-on CPU profiling behind a flag.
- Replace top-N full sort in summaries with a bounded heap if tests pin output order.
- Convert A* path reconstruction to append-and-reverse.

## AI-Ready Execution Checklist

Use this checklist as the next agent's working plan. Do not skip the measurement phase.

### 0. Guardrails

- [ ] Confirm current branch and dirty worktree.
- [ ] Read `AGENTS.md`.
- [ ] Do not revert user changes.
- [ ] Keep deterministic simulation semantics unless explicitly changing behavior.
- [ ] Add or update tests before refactoring shared simulation logic.
- [ ] Record every command run and whether it is data-gathering, validation, or implementation support.

### 1. Baseline Data Collection

Only perform these when the user permits Golab/data gathering.

- [ ] Run `go test ./...` and record pass/fail.
- [ ] Add focused benchmarks without changing behavior:
  - [ ] `BenchmarkLiveBotCount`
  - [ ] `BenchmarkBotsActionsSparse`
  - [ ] `BenchmarkEnvironmentActions`
  - [ ] `BenchmarkCalcPath`
  - [ ] `BenchmarkCalcFlowField`
  - [ ] `BenchmarkSummarizeMatch`
  - [ ] `BenchmarkDrawGridDirtyPatch` if a headless GL-safe harness exists; otherwise isolate patch preparation.
- [ ] Run `go test -run '^$' -bench . -benchmem ./...`.
- [ ] Capture CPU profile for deterministic headless command mode.
- [ ] Capture allocation profile for deterministic headless command mode.
- [ ] Capture CPU profile for task/path-heavy scenario.
- [ ] Capture interactive profile only with explicit user permission.
- [ ] Save baseline JSON outputs for representative seeds and tick counts.

### 2. Fast Correctness Fixes Before Perf Refactors

- [ ] Investigate `internal/core/bot.go:141` uninitialized child path.
- [ ] Add a regression test proving `NewChild` always initializes position, genome, HP, color, parent/colony fields, and offspring relationship.
- [ ] Decide whether `NewChild` should ever return a raw pooled bot without initialization. Most likely remove that branch.
- [ ] Run tests.

### 3. Low-Risk Performance Wins

- [ ] Change board neighbor-table initialization to `sync.Once`.
- [ ] Add a test or benchmark proving repeated `NewBoard` does not redo neighbor-table work.
- [ ] Gate always-on CPU profiling in `cmd/golab/main.go` behind `--cpuprofile path` or an environment variable.
- [ ] Ensure command modes remain unaffected by the interactive profiling flag.
- [ ] Replace path reconstruction prepend in `findPath` with append-reverse.
- [ ] Add/extend `CalcPath` tests for path order and wrapped columns.
- [ ] Replace full sort in `summarizeMatch` with bounded top-N selection.
- [ ] Preserve exact top-bot ordering for ties: HP desc, inventory desc, index asc.

### 4. Rendering Dirty-Patch Refactor

- [ ] Add tests for `Board.MarkDirty`, `PullPatch`, duplicate suppression, and dirty clearing.
- [ ] Replace `DrawGrid` full dirty bitmap scan with patch iteration.
- [ ] Decide whether patch indices must be sorted before contiguous VBO upload batching.
- [ ] Ensure every visual state mutation calls `MarkDirty`:
  - [ ] Bot move old cell.
  - [ ] Bot move new cell.
  - [ ] Bot color change.
  - [ ] Selection change.
  - [ ] Hover change.
  - [ ] Overlay path/task/unreachable toggles.
  - [ ] Board cell set/clear.
- [ ] Remove or reduce `DrawGrid` marking every bot dirty each frame after confirming no stale visuals.
- [ ] Validate with screenshot/manual visual check when user permits running Golab.

### 5. Live Bot Active Set

- [ ] Add a canonical live bot count maintained on spawn/death/move/reset.
- [ ] Add active bot index list or dense live bot slice.
- [ ] Keep `Board.Bots` direct index lookup for existing code.
- [ ] Rewrite `liveBotCount` to return maintained count or assert maintained count in debug mode.
- [ ] Rewrite `botsActions` to iterate active live bots.
- [ ] Preserve deterministic iteration order. If using a dense slice changes order, either sort indices or intentionally document and test the behavior change.
- [ ] Add tests for spawn, death, attack kill, poison death, division, master `spark_bots`, and populate/reset.

### 6. Environment Active Sets

- [ ] Add active sets for `Organics`, `Farm`, and `Controller`.
- [ ] Maintain sets in `Board.Set` and `Board.Clear`.
- [ ] Rewrite `environmentActions` to iterate active sets.
- [ ] Add consistency assertions comparing active sets to a full scan in tests.
- [ ] Add tests for organics decay/removal, farm production, controller removal, and populate/reset.

### 7. Board Stats Cache

- [ ] Define board stats fields: walls, controllers, farms, spawners, mines, buildings, food, resources, poison, organics, water.
- [ ] Update stats through `Set` and `Clear` by decrementing old occupant kind and incrementing new occupant kind.
- [ ] Add a full-scan verification helper for tests.
- [ ] Update `summarizeMatch` and `ObserveMaster` to use cached stats.
- [ ] Keep colony count logic separate unless a reliable colony/controller lifecycle list is added.
- [ ] Add tests for all occupant kind transitions, including setting the same kind and setting nil.

### 8. Pathfinding And Tasking Optimization

- [ ] Benchmark current A*, flow field, and task assignment.
- [ ] Replace A* maps with array scratch buffers and generation stamps.
- [ ] Reuse heap backing arrays.
- [ ] Reuse `prev`, `gScore`, and `closed` scratch buffers.
- [ ] Reuse `CalcFlowField` queue and distance buffers per colony or from a pool.
- [ ] Add board topology versioning if cached fields can become stale after walls/buildings/resources change.
- [ ] Replace `SortedFreeBots` full sort if benchmarks show it matters.
- [ ] Add tests for path correctness, unreachable targets, wrapped columns, bot-passable fields, and task assignment stability.

### 9. Board Reset And Populate

- [ ] Benchmark `populateBoard`.
- [ ] Convert `NewBoard` global work to one-time init.
- [ ] Consider `Board.Reset` that clears slices in place.
- [ ] Avoid `MarkAllDirty` in headless command mode if no renderer consumes dirty state.
- [ ] Preserve current tests:
  - [ ] `TestPopulateBoardDoesNotOverlayResourceOnCopiedBot`
  - [ ] `TestPopulateBoardMarksClearedResourceCellDirty`
- [ ] Add tests for preserved occupant types across populate.

### 10. Command Summary Optimization

- [ ] Add golden-ish deterministic tests for `status`, `match`, and `replay` summary shape.
- [ ] Use top-N selection instead of sorting all bots.
- [ ] Combine grid passes.
- [ ] Replace wall/resource/etc counts with board stats cache if available.
- [ ] Avoid timestamp-sensitive comparisons in deterministic tests unless timestamp is explicitly part of contract.

### 11. Profiling And Validation Gates

- [ ] `go fmt ./...`
- [ ] `go test ./...`
- [ ] `go test -run '^$' -bench . -benchmem ./...`
- [ ] Deterministic CLI before/after comparison for fixed seeds.
- [ ] CPU profile before/after for each modified hot path.
- [ ] Allocation profile before/after for allocation-focused changes.
- [ ] Interactive smoke test and screenshot only when user permits running Golab.

## Suggested Implementation Order

1. Measurement harness and benchmarks.
2. Correctness fix for `NewChild` if confirmed.
3. Neighbor init `sync.Once`, profiling flag, path reconstruction fix.
4. Summary top-N selection.
5. Dirty patch rendering refactor.
6. Active live bot count/set.
7. Environment active sets.
8. Board stats cache.
9. Pathfinding scratch buffers and flow-field reuse.
10. Larger data-layout changes only if profiles still justify them.

## Files To Inspect First In The Next Pass

- `internal/util/util.go`: board dimensions and position indexing.
- `internal/core/board.go`: grid, dirty tracking, set/clear, occupancy, neighbor table.
- `internal/game/game.go`: tick loop, population, bot actions, environment actions.
- `internal/tasking/pathfinding.go`: A*, flow fields.
- `internal/tasking/tasking.go`: colony task assignment and sorting.
- `internal/core/colony.go`: path masks, task state, colony members.
- `internal/core/bot.go`: child creation, pooling, lineage.
- `internal/ui/ui.go`: dirty rendering, VBO updates, overlay.
- `cmd/golab/commands.go`: deterministic command summaries.
- `internal/game/gamemaster.go`: observation and master events.
- `internal/render/board.go`: PNG render command path.

## Notes For Future Agents

- Prefer small, measured changes. The code has intertwined simulation, rendering, and deterministic command behavior.
- Keep full-scan helpers as test or debug verification while replacing hot scans with maintained state.
- Be explicit when an optimization changes iteration order. That can alter random call order and simulation results.
- If profiles contradict this static analysis, trust the profiles and update this file with the measured evidence.
