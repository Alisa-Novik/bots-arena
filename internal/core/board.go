package core

import (
	"golab/internal/util"
	"sync"

	"golang.org/x/exp/rand"
)

type Occupant any
type Position = util.Position
type BotID int32

const NoBotID BotID = -1

type Wall struct{ Pos Position }

type Resource struct {
	Pos    Position
	Amount int
}
type Water struct {
	GroupId int
	Amount  int
}
type Organics struct {
	Pos    Position
	Amount int
}
type Food struct {
	Pos    Position
	Amount int
}
type Farm struct {
	Pos    Position
	Owner  *Bot
	Colony *Colony
	Amount int
}
type Spawner struct {
	Pos       Position
	Owner     *Bot
	Colony    *Colony
	Amount    int
	AutoBirth bool
}
type Mine struct {
	Pos    Position
	Owner  *Bot
	Amount int
}
type Poison struct {
	Pos Position
}
type Building struct {
	Pos   Position
	Owner *Bot
	Hp    int
}
type Depot struct {
	Pos    Position
	Owner  *Bot
	Colony *Colony
	Food   int
	Ore    int
}
type Board struct {
	TaskTargetsR   []util.Position
	PathsToRenderR []util.Position
	UnreachablesR  []util.Position

	taskTargetsMask   []bool
	pathsToRenderMask []bool
	unreachablesMask  []bool

	grid                []Occupant
	occupied            []bool
	dirty               []bool
	frozen              []bool
	biomes              []Biome
	pheromones          []PheromoneCell
	pheromoneHomeOwner  []*Colony
	pheromoneActive     []int
	pheromoneActiveMask []bool
	Bots                []*Bot
	activeBotIDs        []BotID
	botAtCell           []BotID
	botSlots            []*Bot
	botCell             []int
	botActiveIndex      []int
	freeBotIDs          []BotID
	activeEnvCells      []int
	activeEnvSorted     []int
	envActiveIndex      []int
	activeEnvOrderDirty bool
	// colonyCells []ColonyCell
	patch []int
}

const (
	Rows = util.Rows
	Cols = util.Cols
)

var PosClock = util.PosClock

var PathToPt = make(map[[2]int][]Position)

var neighbourIdx [Rows * Cols][8]int
var neighbourOnce sync.Once

func (b *Board) PullPatch() []int {
	p := b.patch
	b.patch = b.patch[:0]
	for _, i := range p {
		b.dirty[i] = false
	}
	return p
}

func (b *Board) MarkClean(i int) {
	b.dirty[i] = false
}

func (b *Board) MarkDirty(i int) {
	if !b.dirty[i] {
		b.dirty[i] = true
		b.patch = append(b.patch, i)
	}
}

func (b *Board) MarkAllDirty() {
	for i := range b.dirty {
		b.MarkDirty(i)
	}
}

func (b *Board) DirtyBitmap() []bool {
	return b.dirty
}

func (b *Board) IsFrozen(pos Position) bool {
	if !Inside(pos) {
		return false
	}
	return b.frozen[idx(pos)]
}

func (b *Board) IsFrozenIdx(i int) bool {
	return i >= 0 && i < len(b.frozen) && b.frozen[i]
}

func (b *Board) SetFrozen(pos Position, frozen bool) bool {
	if !Inside(pos) {
		return false
	}
	i := idx(pos)
	if b.frozen[i] == frozen {
		return false
	}
	b.frozen[i] = frozen
	b.MarkDirty(i)
	return true
}

func (b *Board) CopyFrozenFrom(other *Board) {
	if other == nil || len(other.frozen) != len(b.frozen) {
		return
	}
	copy(b.frozen, other.frozen)
	for i, frozen := range b.frozen {
		if frozen {
			b.MarkDirty(i)
		}
	}
}

func (b *Board) BiomeAt(pos Position) Biome {
	if !Inside(pos) || len(b.biomes) == 0 {
		return BiomeNeutral
	}
	col := (pos.C + Cols) % Cols
	return b.biomes[pos.R*Cols+col]
}

func (b *Board) BiomeAtIdx(i int) Biome {
	if i < 0 || i >= len(b.biomes) {
		return BiomeNeutral
	}
	return b.biomes[i]
}

func (b *Board) CopyBiomesFrom(other *Board) {
	if other == nil || len(other.biomes) != len(b.biomes) {
		return
	}
	copy(b.biomes, other.biomes)
}

func initNeighbourTable() {
	for r := range Rows {
		for c := range Cols {
			idx := r*Cols + c
			for n, d := range PosClock {
				nr, nc := r+d[1], c+d[0]
				if nr < 0 || nr >= Rows || nc < 0 || nc >= Cols {
					neighbourIdx[idx][n] = -1
					continue
				}
				neighbourIdx[idx][n] = nr*Cols + ((nc + Cols) % Cols)
			}
		}
	}
}

func NewBoard() *Board {
	neighbourOnce.Do(initNeighbourTable)
	b := &Board{
		taskTargetsMask:     make([]bool, util.Cells),
		pathsToRenderMask:   make([]bool, util.Cells),
		unreachablesMask:    make([]bool, util.Cells),
		grid:                make([]Occupant, Rows*Cols),
		occupied:            make([]bool, Rows*Cols),
		dirty:               make([]bool, Rows*Cols),
		frozen:              make([]bool, Rows*Cols),
		biomes:              make([]Biome, Rows*Cols),
		pheromones:          make([]PheromoneCell, Rows*Cols),
		pheromoneHomeOwner:  make([]*Colony, Rows*Cols),
		pheromoneActiveMask: make([]bool, Rows*Cols),
		Bots:                make([]*Bot, util.Cells),
		botAtCell:           make([]BotID, util.Cells),
		envActiveIndex:      make([]int, util.Cells),
	}
	for i := range b.botAtCell {
		b.botAtCell[i] = NoBotID
	}
	for i := range b.envActiveIndex {
		b.envActiveIndex[i] = -1
	}
	b.populateDeterministicBiomes()
	return b
}

func (b *Board) AddPathsToRender(path ...Position) {
	for _, p := range path {
		if !Inside(p) {
			continue
		}
		i := idx(p)
		if !b.pathsToRenderMask[i] {
			b.pathsToRenderMask[i] = true
			b.PathsToRenderR = append(b.PathsToRenderR, p)
		}
		b.MarkDirty(i)
	}
}

func (b *Board) IsPathToRender(pos Position) bool {
	return b.isMarked(b.pathsToRenderMask, pos)
}

func (b *Board) IsTaskTargetToRender(pos Position) bool {
	return b.isMarked(b.taskTargetsMask, pos)
}

func (b *Board) IsUnreachableToRender(pos Position) bool {
	return b.isMarked(b.unreachablesMask, pos)
}

func (b *Board) isMarked(mask []bool, pos Position) bool {
	if !Inside(pos) {
		return false
	}
	i := idx(pos)
	return i < len(mask) && mask[i]
}

func NewRandomPosition() Position {
	return Position{C: rand.Intn(Cols), R: rand.Intn(Rows)}
}

func (b *Board) GetGrid() *[]Occupant {
	return &b.grid
}

func idx(p Position) int {
	return util.Idx(p)
}

func (b *Board) Clear(pos Position) {
	if !Inside(pos) {
		return
	}
	i := idx(pos)
	b.unregisterBotAtIdx(i)
	b.unmarkEnvironmentActive(i)
	b.occupied[i] = false
	b.MarkDirty(i)
	b.grid[i] = nil
}

func (b *Board) Set(pos Position, o Occupant) {
	if !Inside(pos) {
		return
	}
	if o == nil {
		b.Clear(pos)
		return
	}
	i := idx(pos)
	if bot, ok := o.(*Bot); ok {
		b.setBotAtIdx(i, pos, bot)
		return
	}
	b.unregisterBotAtIdx(i)
	b.updateEnvironmentActive(i, o)
	b.occupied[i] = true
	b.MarkDirty(i)
	b.grid[i] = o
}

func (b *Board) IsEmptyNoBot(pos Position) bool {
	if !(pos.R >= 0 && pos.R < Rows) {
		return false
	}

	return b.grid[idx(pos)] == nil
}

func (b *Board) IsEmpty(pos Position) bool {
	if !(pos.R >= 0 && pos.R < Rows) {
		return false
	}

	return b.grid[idx(pos)] == nil
}

func (b *Board) At(pos Position) Occupant {
	if pos.R < 0 || pos.R >= Rows {
		return nil
	}
	return b.grid[idx(pos)]
}

func (b *Board) IsPreserved(o Occupant) bool {
	switch o.(type) {
	case Controller, *Controller, Farm, Food, Poison, Building, Water, Depot, *Depot:
		return true
	default:
		return false
	}
}

func Inside(p Position) bool {
	return p.R >= 0 && p.R < Rows
}

func (b *Board) firstEmptyAround(idx int) int {
	start := rand.Intn(8)
	for i := range 8 {
		n := neighbourIdx[idx][(start+i)&7]
		if n >= 0 && !b.occupied[n] {
			return n
		}
	}
	return -1
}

func (b *Board) FindEmptyPosAround(p Position) (Position, bool) {
	n := b.firstEmptyAround(idx(p))
	if n < 0 {
		return Position{}, false
	}
	return Position{R: n / Cols, C: n % Cols}, true
}

func (b *Board) IsGrabable(pos Position) bool {
	switch b.At(pos).(type) {
	case Farm, Food, Poison, Controller, *Controller, Resource, Building, Spawner, Depot, *Depot:
		return true
	default:
		return false
	}
}

func (b *Board) IsWall(pos Position) bool {
	if !Inside(pos) {
		return true
	}
	return pos.R == 0 || pos.R == Rows-1
}

func (b *Board) GetBot(pos util.Position) *Bot {
	if !Inside(pos) {
		return nil
	}
	i := idx(pos)
	if id := b.botAtCell[i]; b.validBotID(id) {
		return b.botSlots[id]
	}
	if bot := b.Bots[i]; bot != nil {
		b.setBotAtIdx(i, pos, bot)
		return bot
	}
	return nil
}

func (b *Board) IsSurrounded(bp util.Position) bool {
	for _, dir := range Dirs {
		dPos := bp.AddDir(dir)
		if b.GetBot(dPos) == nil {
			return false
		}
	}
	return true
}

func (b *Board) IsEmptyOrBot(p util.Position) bool {
	return !util.OutOfBounds(p) && (b.IsEmpty(p) || b.GetBot(p) != nil)
}

func (b *Board) IsEmptyOrBotIdx(i int) bool {
	if i < Cols || i >= (Rows-1)*Cols {
		return false
	}
	return b.grid[i] == nil || b.Bots[i] != nil
}
