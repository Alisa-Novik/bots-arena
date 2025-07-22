package game

import (
	"golab/core"
	"golab/util"
)

func (g *Game) liveBotCount() int {
	n := 0
	for _, b := range g.Bots {
		if b != nil {
			n++
		}
	}
	return n
}

func assert(cond bool, msg string) {
	if !cond {
		panic(msg)
	}
}

func idx(p core.Position) int {
	return util.Idx(p)
}

func (g *Game) GetBot(pos util.Position) *core.Bot {
	if !core.Inside(pos) {
		return nil
	}
	return g.Bots[idx(pos)]
}

// func (g *Game) unload(b bot.Bot, pos board.Position, newBots map[board.Position]bot.Bot) {
// 	t1 := util.Position{R: 15, C: 40}
// 	if !b.Unloading {
// 		b.Unloading = true
// 		b.Usp = [2]int{pos.R, pos.C}
// 		board.PathToPt[b.Usp] = util.FindPath(pos, t1, g.Board.IsEmpty)
// 		newBots[pos] = b
// 		return
// 	}
// 	path := board.PathToPt[b.Usp]
// 	if len(path) == 0 {
// 		b.Hp -= 30
// 		b.Unloading = false
// 		newBots[pos] = b
// 		return
// 	}
// 	nextMove := util.Position{R: path[0].R - pos.R, C: path[0].C - pos.C}
// 	g.move(newBots, pos, nextMove, b)
// 	board.PathToPt[b.Usp] = path[1:]
// }

// func (g *Game) move(next map[board.Position]bot.Bot, pos board.Position, dir board.Position, b bot.Bot) {
// 	target := pos.AddPos(dir)
//
// 	blocked := g.Board.IsWall(target) ||
// 		!g.Board.IsEmpty(target) ||
// 		next[target] != (bot.Bot{})
//
// 	if blocked {
// 		next[pos] = b
// 	}
//
// 	g.Board.Clear(pos)
// 	g.Board.Set(target, b)
// 	next[target] = b
// }
