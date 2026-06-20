package core

import (
	"fmt"
	"sort"
)

type PheromoneChannel uint8

const (
	PheromoneFood PheromoneChannel = iota
	PheromoneOre
	PheromoneHome
	PheromoneDanger
	pheromoneChannelCount
)

type PheromoneCell [pheromoneChannelCount]uint8

type PheromoneValues struct {
	Food   uint8
	Ore    uint8
	Home   uint8
	Danger uint8
}

type PheromoneTotals struct {
	ActiveCells int
	Food        int
	Ore         int
	Home        int
	Danger      int
}

func PheromoneChannelCount() int {
	return int(pheromoneChannelCount)
}

func DecodePheromoneChannel(value int) PheromoneChannel {
	if value < 0 {
		value = -value
	}
	return PheromoneChannel(value % int(pheromoneChannelCount))
}

func (c PheromoneChannel) Valid() bool {
	return c < pheromoneChannelCount
}

func (c PheromoneChannel) String() string {
	switch c {
	case PheromoneFood:
		return "food"
	case PheromoneOre:
		return "ore"
	case PheromoneHome:
		return "home"
	case PheromoneDanger:
		return "danger"
	default:
		return "unknown"
	}
}

func (v PheromoneValues) Channel(channel PheromoneChannel) uint8 {
	switch channel {
	case PheromoneFood:
		return v.Food
	case PheromoneOre:
		return v.Ore
	case PheromoneHome:
		return v.Home
	case PheromoneDanger:
		return v.Danger
	default:
		return 0
	}
}

func (v PheromoneValues) IsZero() bool {
	return v.Food == 0 && v.Ore == 0 && v.Home == 0 && v.Danger == 0
}

func (v PheromoneValues) InspectString() string {
	return fmt.Sprintf("Phero F %d O %d H %d D %d", v.Food, v.Ore, v.Home, v.Danger)
}

func (b *Board) DepositPheromone(pos Position, channel PheromoneChannel, amount int, owner *Colony) bool {
	if !Inside(pos) || !channel.Valid() || amount <= 0 {
		return false
	}
	i := idx(pos)
	old := b.pheromones[i][channel]
	next := cappedPheromone(int(old) + amount)
	if next == old && (channel != PheromoneHome || owner == nil || b.pheromoneHomeOwner[i] == owner) {
		return false
	}
	b.pheromones[i][channel] = next
	if channel == PheromoneHome {
		b.assignHomeOwner(i, old, amount, owner)
	}
	b.markPheromoneActive(i)
	b.MarkDirty(i)
	return true
}

func (b *Board) PheromoneAt(pos Position) PheromoneValues {
	if !Inside(pos) {
		return PheromoneValues{}
	}
	return b.PheromoneAtIdx(idx(pos))
}

func (b *Board) PheromoneAtIdx(i int) PheromoneValues {
	if i < 0 || i >= len(b.pheromones) {
		return PheromoneValues{}
	}
	return pheromoneValues(b.pheromones[i])
}

func (b *Board) PheromoneHomeOwnerAt(pos Position) *Colony {
	if !Inside(pos) {
		return nil
	}
	return b.pheromoneHomeOwner[idx(pos)]
}

func (b *Board) PheromoneValueForBot(pos Position, channel PheromoneChannel, bot *Bot) uint8 {
	if !Inside(pos) || !channel.Valid() {
		return 0
	}
	i := idx(pos)
	values := b.pheromones[i]
	switch channel {
	case PheromoneHome:
		if b.isForeignHome(i, bot) {
			return 0
		}
		return values[PheromoneHome]
	case PheromoneDanger:
		danger := int(values[PheromoneDanger])
		if b.isForeignHome(i, bot) {
			danger += int(values[PheromoneHome])
		}
		return cappedPheromone(danger)
	default:
		return values[channel]
	}
}

func (b *Board) CopyPheromonesFrom(other *Board) {
	if other == nil || len(other.pheromones) != len(b.pheromones) {
		return
	}
	copy(b.pheromones, other.pheromones)
	copy(b.pheromoneHomeOwner, other.pheromoneHomeOwner)
	b.pheromoneActive = b.pheromoneActive[:0]
	for i := range b.pheromoneActiveMask {
		b.pheromoneActiveMask[i] = false
	}
	for _, i := range other.pheromoneActive {
		if i < 0 || i >= len(other.pheromones) || !other.pheromoneActiveMask[i] || !b.pheromoneCellNonZero(i) {
			continue
		}
		b.markPheromoneActive(i)
		b.MarkDirty(i)
	}
}

func (b *Board) ClearPheromones() {
	for _, i := range b.pheromoneActive {
		if i < 0 || i >= len(b.pheromones) || !b.pheromoneActiveMask[i] {
			continue
		}
		b.pheromones[i] = PheromoneCell{}
		b.pheromoneHomeOwner[i] = nil
		b.pheromoneActiveMask[i] = false
		b.MarkDirty(i)
	}
	b.pheromoneActive = b.pheromoneActive[:0]
}

func (b *Board) DecayPheromones(amount int) int {
	if amount <= 0 {
		return 0
	}
	changed := 0
	write := 0
	active := b.pheromoneActive
	for _, i := range active {
		if i < 0 || i >= len(b.pheromones) || !b.pheromoneActiveMask[i] {
			continue
		}
		before := b.pheromones[i]
		for channel := PheromoneChannel(0); channel < pheromoneChannelCount; channel++ {
			b.pheromones[i][channel] = decayPheromoneValue(
				b.pheromones[i][channel],
				b.adjustedPheromoneDecay(i, channel, amount),
			)
		}
		if b.pheromones[i][PheromoneHome] == 0 {
			b.pheromoneHomeOwner[i] = nil
		}
		if before != b.pheromones[i] {
			changed++
			b.MarkDirty(i)
		}
		if b.pheromoneCellNonZero(i) {
			b.pheromoneActive[write] = i
			write++
			continue
		}
		b.pheromoneActiveMask[i] = false
		b.pheromoneHomeOwner[i] = nil
	}
	b.pheromoneActive = b.pheromoneActive[:write]
	return changed
}

func (b *Board) DiffusePheromones(amount int) int {
	if amount <= 0 {
		return 0
	}
	type deltaCell [pheromoneChannelCount]int

	active := append([]int(nil), b.pheromoneActive...)
	sort.Ints(active)
	deltas := make(map[int]deltaCell, len(active)*2)
	homeOwners := make(map[int]*Colony, len(active))
	for _, i := range active {
		if i < 0 || i >= len(b.pheromones) || !b.pheromoneActiveMask[i] {
			continue
		}
		neighbors, neighborCount := cardinalNeighborIndexes(i)
		if neighborCount == 0 {
			continue
		}
		source := b.pheromones[i]
		for channel := PheromoneChannel(0); channel < pheromoneChannelCount; channel++ {
			totalOut := amount * neighborCount
			if int(source[channel]) <= totalOut {
				continue
			}
			sourceDelta := deltas[i]
			sourceDelta[channel] -= totalOut
			deltas[i] = sourceDelta
			for neighborIdx := 0; neighborIdx < neighborCount; neighborIdx++ {
				n := neighbors[neighborIdx]
				neighborDelta := deltas[n]
				neighborDelta[channel] += amount
				deltas[n] = neighborDelta
				if channel == PheromoneHome && b.pheromoneHomeOwner[i] != nil {
					homeOwners[n] = b.pheromoneHomeOwner[i]
				}
			}
		}
	}

	changed := 0
	deltaCells := make([]int, 0, len(deltas))
	for i := range deltas {
		deltaCells = append(deltaCells, i)
	}
	sort.Ints(deltaCells)
	for _, i := range deltaCells {
		delta := deltas[i]
		if i < 0 || i >= len(b.pheromones) {
			continue
		}
		before := b.pheromones[i]
		for channel := PheromoneChannel(0); channel < pheromoneChannelCount; channel++ {
			b.pheromones[i][channel] = cappedPheromone(int(b.pheromones[i][channel]) + delta[channel])
		}
		if owner := homeOwners[i]; owner != nil && before[PheromoneHome] == 0 {
			b.pheromoneHomeOwner[i] = owner
		}
		if b.pheromones[i][PheromoneHome] == 0 {
			b.pheromoneHomeOwner[i] = nil
		}
		if before == b.pheromones[i] {
			continue
		}
		changed++
		if b.pheromoneCellNonZero(i) {
			b.markPheromoneActive(i)
		} else {
			b.pheromoneActiveMask[i] = false
			b.pheromoneHomeOwner[i] = nil
		}
		b.MarkDirty(i)
	}
	b.compactPheromoneActive()
	return changed
}

func (b *Board) PheromoneTotals() PheromoneTotals {
	var totals PheromoneTotals
	for _, i := range b.pheromoneActive {
		if i < 0 || i >= len(b.pheromones) || !b.pheromoneActiveMask[i] || !b.pheromoneCellNonZero(i) {
			continue
		}
		totals.ActiveCells++
		values := b.pheromones[i]
		totals.Food += int(values[PheromoneFood])
		totals.Ore += int(values[PheromoneOre])
		totals.Home += int(values[PheromoneHome])
		totals.Danger += int(values[PheromoneDanger])
	}
	return totals
}

func cardinalNeighborIndexes(i int) ([4]int, int) {
	pos := Position{R: i / Cols, C: i % Cols}
	var out [4]int
	count := 0
	for _, dir := range Dirs {
		next := pos.AddDir(dir)
		if Inside(next) {
			out[count] = idx(next)
			count++
		}
	}
	return out, count
}

func (b *Board) adjustedPheromoneDecay(i int, channel PheromoneChannel, base int) int {
	decay := base
	switch {
	case channel == PheromoneFood && b.BiomeAtIdx(i) == BiomeFertile:
		decay = max(1, decay-1)
	case channel == PheromoneOre && b.BiomeAtIdx(i) == BiomeMineral:
		decay = max(1, decay-1)
	case channel == PheromoneDanger && b.BiomeAtIdx(i) == BiomeToxic:
		decay = max(1, decay-1)
	}
	if _, ok := b.grid[i].(Water); ok && isPositivePheromone(channel) {
		decay += 2
	}
	return decay
}

func (b *Board) assignHomeOwner(i int, old uint8, amount int, owner *Colony) {
	if owner == nil {
		return
	}
	if old == 0 || b.pheromoneHomeOwner[i] == nil || b.pheromoneHomeOwner[i] == owner || amount >= int(old) {
		b.pheromoneHomeOwner[i] = owner
	}
}

func (b *Board) markPheromoneActive(i int) {
	if i < 0 || i >= len(b.pheromoneActiveMask) || b.pheromoneActiveMask[i] {
		return
	}
	b.pheromoneActiveMask[i] = true
	b.pheromoneActive = append(b.pheromoneActive, i)
}

func (b *Board) compactPheromoneActive() {
	write := 0
	for _, i := range b.pheromoneActive {
		if i < 0 || i >= len(b.pheromones) || !b.pheromoneActiveMask[i] || !b.pheromoneCellNonZero(i) {
			if i >= 0 && i < len(b.pheromoneActiveMask) {
				b.pheromoneActiveMask[i] = false
			}
			continue
		}
		b.pheromoneActive[write] = i
		write++
	}
	b.pheromoneActive = b.pheromoneActive[:write]
	sort.Ints(b.pheromoneActive)
}

func (b *Board) pheromoneCellNonZero(i int) bool {
	cell := b.pheromones[i]
	for channel := PheromoneChannel(0); channel < pheromoneChannelCount; channel++ {
		if cell[channel] != 0 {
			return true
		}
	}
	return false
}

func (b *Board) isForeignHome(i int, bot *Bot) bool {
	owner := b.pheromoneHomeOwner[i]
	return b.pheromones[i][PheromoneHome] > 0 && owner != nil && (bot == nil || bot.Colony != owner)
}

func pheromoneValues(cell PheromoneCell) PheromoneValues {
	return PheromoneValues{
		Food:   cell[PheromoneFood],
		Ore:    cell[PheromoneOre],
		Home:   cell[PheromoneHome],
		Danger: cell[PheromoneDanger],
	}
}

func isPositivePheromone(channel PheromoneChannel) bool {
	return channel == PheromoneFood || channel == PheromoneOre || channel == PheromoneHome
}

func cappedPheromone(value int) uint8 {
	if value <= 0 {
		return 0
	}
	if value >= 255 {
		return 255
	}
	return uint8(value)
}

func decayPheromoneValue(value uint8, amount int) uint8 {
	if amount <= 0 {
		return value
	}
	if int(value) <= amount {
		return 0
	}
	return value - uint8(amount)
}
