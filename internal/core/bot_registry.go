package core

import (
	"golab/internal/util"
	"sort"
)

func (b *Board) ActiveBotIDs() []BotID {
	return b.activeBotIDs
}

func (b *Board) ActiveBotCount() int {
	return len(b.activeBotIDs)
}

func (b *Board) BotByID(id BotID) *Bot {
	if !b.validBotID(id) {
		return nil
	}
	return b.botSlots[int(id)]
}

func (b *Board) BotCell(id BotID) int {
	if !b.validBotID(id) {
		return -1
	}
	return b.botCell[int(id)]
}

func (b *Board) BotPosition(id BotID) (Position, bool) {
	cell := b.BotCell(id)
	if cell < 0 {
		return Position{}, false
	}
	return util.PosOf(cell), true
}

func (b *Board) AddBot(pos Position, bot *Bot) bool {
	if !Inside(pos) || bot == nil {
		return false
	}
	b.setBotAtIdx(idx(pos), pos, bot)
	return true
}

func (b *Board) RemoveBotAt(pos Position) *Bot {
	if !Inside(pos) {
		return nil
	}
	cellIdx := idx(pos)
	bot := b.unregisterBotAtIdx(cellIdx)
	if _, ok := b.grid[cellIdx].(*Bot); ok {
		b.grid[cellIdx] = nil
		b.occupied[cellIdx] = false
	}
	b.MarkDirty(cellIdx)
	return bot
}

func (b *Board) MoveBot(oldPos, newPos Position, bot *Bot) bool {
	if !Inside(oldPos) || !Inside(newPos) || bot == nil {
		return false
	}
	oldIdx := idx(oldPos)
	newIdx := idx(newPos)
	if oldIdx == newIdx {
		b.setBotAtIdx(newIdx, newPos, bot)
		return true
	}
	if existing := b.GetBot(newPos); existing != nil && existing != bot {
		return false
	}

	id := b.botAtCell[oldIdx]
	if !b.validBotID(id) || b.botSlots[int(id)] != bot {
		if b.Bots[oldIdx] == bot {
			b.setBotAtIdx(oldIdx, oldPos, bot)
			id = b.botAtCell[oldIdx]
		}
	}
	if !b.validBotID(id) || b.botSlots[int(id)] != bot {
		return false
	}

	b.moveRegisteredBot(id, oldIdx, newIdx, newPos, bot)
	return true
}

func (b *Board) ActiveEnvironmentCells() []int {
	return b.activeEnvCells
}

func (b *Board) SortedActiveEnvironmentCells(dst []int) []int {
	if b == nil {
		return dst
	}
	if b.activeEnvOrderDirty {
		b.activeEnvSorted = append(b.activeEnvSorted[:0], b.activeEnvCells...)
		sort.Ints(b.activeEnvSorted)
		b.activeEnvOrderDirty = false
	}
	return append(dst, b.activeEnvSorted...)
}

func (b *Board) validBotID(id BotID) bool {
	i := int(id)
	return id != NoBotID && i >= 0 && i < len(b.botSlots) && b.botSlots[i] != nil
}

func (b *Board) setBotAtIdx(cellIdx int, pos Position, bot *Bot) {
	if bot == nil {
		b.Clear(pos)
		return
	}

	if existingID := b.botAtCell[cellIdx]; b.validBotID(existingID) {
		if b.botSlots[int(existingID)] == bot {
			b.writeRegisteredBotCell(existingID, cellIdx, pos, bot)
			return
		}
		b.unregisterBotAtIdx(cellIdx)
	}

	if Inside(bot.Pos) {
		oldIdx := idx(bot.Pos)
		if oldIdx != cellIdx {
			if oldID := b.botAtCell[oldIdx]; b.validBotID(oldID) && b.botSlots[int(oldID)] == bot {
				b.moveRegisteredBot(oldID, oldIdx, cellIdx, pos, bot)
				return
			}
		}
	}

	id := b.allocateBotID()
	idIdx := int(id)
	b.botSlots[idIdx] = bot
	b.botCell[idIdx] = cellIdx
	b.botActiveIndex[idIdx] = len(b.activeBotIDs)
	b.botAtCell[cellIdx] = id
	b.activeBotIDs = append(b.activeBotIDs, id)
	b.writeRegisteredBotCell(id, cellIdx, pos, bot)
}

func (b *Board) allocateBotID() BotID {
	if n := len(b.freeBotIDs); n > 0 {
		id := b.freeBotIDs[n-1]
		b.freeBotIDs = b.freeBotIDs[:n-1]
		return id
	}
	id := BotID(len(b.botSlots))
	b.botSlots = append(b.botSlots, nil)
	b.botCell = append(b.botCell, -1)
	b.botActiveIndex = append(b.botActiveIndex, -1)
	return id
}

func (b *Board) writeRegisteredBotCell(id BotID, cellIdx int, pos Position, bot *Bot) {
	b.unmarkEnvironmentActive(cellIdx)
	b.botAtCell[cellIdx] = id
	b.Bots[cellIdx] = bot
	b.grid[cellIdx] = bot
	b.occupied[cellIdx] = true
	bot.Pos = pos
	b.MarkDirty(cellIdx)
}

func (b *Board) moveRegisteredBot(id BotID, oldIdx, newIdx int, newPos Position, bot *Bot) {
	b.unregisterDestinationBotIfDifferent(newIdx, bot)
	b.botAtCell[oldIdx] = NoBotID
	b.Bots[oldIdx] = nil
	if _, ok := b.grid[oldIdx].(*Bot); ok {
		b.grid[oldIdx] = nil
		b.occupied[oldIdx] = false
	}
	b.botCell[int(id)] = newIdx
	b.writeRegisteredBotCell(id, newIdx, newPos, bot)
	b.MarkDirty(oldIdx)
}

func (b *Board) unregisterDestinationBotIfDifferent(cellIdx int, bot *Bot) {
	if existingID := b.botAtCell[cellIdx]; b.validBotID(existingID) && b.botSlots[int(existingID)] != bot {
		b.unregisterBotAtIdx(cellIdx)
	}
}

func (b *Board) unregisterBotAtIdx(cellIdx int) *Bot {
	if cellIdx < 0 || cellIdx >= len(b.Bots) {
		return nil
	}

	id := b.botAtCell[cellIdx]
	if !b.validBotID(id) {
		bot := b.Bots[cellIdx]
		b.botAtCell[cellIdx] = NoBotID
		b.Bots[cellIdx] = nil
		return bot
	}

	idIdx := int(id)
	bot := b.botSlots[idIdx]
	activeIdx := b.botActiveIndex[idIdx]
	lastIdx := len(b.activeBotIDs) - 1
	if activeIdx >= 0 && activeIdx <= lastIdx {
		lastID := b.activeBotIDs[lastIdx]
		b.activeBotIDs[activeIdx] = lastID
		b.botActiveIndex[int(lastID)] = activeIdx
		b.activeBotIDs = b.activeBotIDs[:lastIdx]
	}

	b.botSlots[idIdx] = nil
	b.botCell[idIdx] = -1
	b.botActiveIndex[idIdx] = -1
	b.freeBotIDs = append(b.freeBotIDs, id)
	b.botAtCell[cellIdx] = NoBotID
	b.Bots[cellIdx] = nil
	return bot
}

func (b *Board) updateEnvironmentActive(cellIdx int, o Occupant) {
	if environmentNeedsTick(o) {
		b.markEnvironmentActive(cellIdx)
		return
	}
	b.unmarkEnvironmentActive(cellIdx)
}

func environmentNeedsTick(o Occupant) bool {
	switch v := o.(type) {
	case Organics, Farm, Poison, Controller, *Controller, Depot, *Depot:
		return true
	case Spawner:
		return v.Colony != nil && v.AutoBirth
	default:
		return false
	}
}

func (b *Board) markEnvironmentActive(cellIdx int) {
	if cellIdx < 0 || cellIdx >= len(b.envActiveIndex) || b.envActiveIndex[cellIdx] >= 0 {
		return
	}
	b.envActiveIndex[cellIdx] = len(b.activeEnvCells)
	b.activeEnvCells = append(b.activeEnvCells, cellIdx)
	b.activeEnvOrderDirty = true
}

func (b *Board) unmarkEnvironmentActive(cellIdx int) {
	if cellIdx < 0 || cellIdx >= len(b.envActiveIndex) {
		return
	}
	activeIdx := b.envActiveIndex[cellIdx]
	if activeIdx < 0 {
		return
	}
	lastIdx := len(b.activeEnvCells) - 1
	lastCell := b.activeEnvCells[lastIdx]
	b.activeEnvCells[activeIdx] = lastCell
	b.envActiveIndex[lastCell] = activeIdx
	b.activeEnvCells = b.activeEnvCells[:lastIdx]
	b.envActiveIndex[cellIdx] = -1
	b.activeEnvOrderDirty = true
}
