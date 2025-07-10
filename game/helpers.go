package game

import (
	"golab/board"
	"golab/bot"
)

// func (g *Game) botAt(p board.Position) *bot.Bot     { return g.Bots[idx(p)] }
// func (g *Game) setBot(p board.Position, b *bot.Bot) { g.Bots[idx(p)] = b }
// func (g *Game) clearBot(p board.Position)           { g.Bots[idx(p)] = nil }
//
// func (g *Game) liveBotCount() int {
// 	n := 0
// 	for _, b := range g.Bots {
// 		if b != nil {
// 			n++
// 		}
// 	}
// 	return n
// }

func (g *Game) unload(b bot.Bot, pos board.Position, newBots map[board.Position]bot.Bot) {
	t1 := board.NewPosition(15, 40)
	if !b.Unloading {
		b.Unloading = true
		b.Usp = [2]int{pos.R, pos.C}
		board.PathToPt[b.Usp] = g.Board.FindPath(pos, t1)
		newBots[pos] = b
		return
	}
	path := board.PathToPt[b.Usp]
	if len(path) == 0 {
		b.Hp -= 30
		b.Unloading = false
		newBots[pos] = b
		return
	}
	nextMove := board.NewPosition(path[0].R-pos.R, path[0].C-pos.C)
	g.move(newBots, pos, nextMove, b)
	board.PathToPt[b.Usp] = path[1:]
}

func (g *Game) move(next map[board.Position]bot.Bot, pos board.Position, dir board.Position, b bot.Bot) {
	target := pos.AddPos(dir)

	blocked := g.Board.IsWall(target) ||
		!g.Board.IsEmpty(target) ||
		next[target] != (bot.Bot{})

	if blocked {
		next[pos] = b
	}

	g.Board.Clear(pos)
	g.Board.Set(target, b)
	next[target] = b
}
