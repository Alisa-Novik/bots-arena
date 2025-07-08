package game

import (
	"fmt"
	"golab/board"
	"golab/bot"
	conf "golab/config"
	"golab/ui"
	"golab/util"
	"time"
)

type Game struct {
	Board  *board.Board
	Bots   map[board.Position]bot.Bot
	config *conf.Config
	State  *conf.GameState

	maxHp   int
	currGen int
}

func NewGame(config *conf.Config) *Game {
	return &Game{
		Board:  board.NewBoard(),
		Bots:   make(map[board.Position]bot.Bot),
		config: config,
		State:  &conf.GameState{LastLogic: time.Now()},

		maxHp:   0,
		currGen: 0,
	}
}

func (g *Game) RunHeadless() {
	g.newGeneration()
	for {
		if len(g.Bots) < g.config.NewGenThreshold {
			g.newGeneration()
		}
		g.botsActions()
		g.environmentActions()
	}
}

func (g *Game) environmentActions() {
	for r := range board.Rows {
		for c := range board.Cols {
			pos := board.Position{C: c, R: r}
			switch v := g.Board.At(pos).(type) {
			case board.Farm:
				if v.Amount <= 0 {
					continue
				}
				foodPos, ok := g.Board.FindEmptyPosAround(pos)
				if !ok {
					continue
				}
				g.Board.Set(foodPos, board.Food{Pos: foodPos, Amount: 1})
				v.Amount -= 1
				g.Board.Set(pos, v)
			}
		}
	}
}

func (g *Game) Run() {
	g.initialBotsGeneration(g.config.InitialGenome)
	g.populateBoard()

	for !ui.Window.ShouldClose() {
		if g.config.Pause {
			ui.DrawGrid(*g.Board, g.Bots)
			continue
		}
		g.step()
		ui.DrawGrid(*g.Board, g.Bots)
	}
}

func (g *Game) step() {
	for time.Since(g.State.LastLogic) >= g.config.LogicStep {
		// g.printDebugInfo()
		if len(g.Bots) == 0 {
			g.initialBotsGeneration(g.config.InitialGenome)
			g.populateBoard()
			return
		}
		if len(g.Bots) <= g.config.NewGenThreshold {
			g.newGeneration()
		}
		g.botsActions()
		g.environmentActions()
		g.State.LastLogic = g.State.LastLogic.Add(g.config.LogicStep)
	}
}

func (g *Game) printDebugInfo() {
	fmt.Printf("\nGeneration: %d; Max HP: %d;", g.currGen, g.maxHp)
	fmt.Printf("\nBots amount: %d", len(g.Bots))
}

func (g *Game) populateBoard() {
	oldBoard := g.Board
	g.Board = board.NewBoard()
	for r := range board.Rows {
		for c := range board.Cols {
			pos := board.Position{C: c, R: r}
			if g.Board.IsWall(pos) {
				g.Board.Set(pos, board.Wall{Pos: pos})
				continue
			}
			// if spawner, ok := oldBoard.At(pos).(board.Spawner); ok {
			// 	g.Board.Set(pos, spawner)
			// 	continue
			// }
			// if f, ok := oldBoard.At(pos).(board.Farm); ok {
			// 	g.Board.Set(pos, f)
			// 	continue
			// }
			// if fd, ok := oldBoard.At(pos).(board.Food); ok {
			// 	g.Board.Set(pos, fd)
			// 	continue
			// }
			// if bld, ok := oldBoard.At(pos).(board.Building); ok {
			// 	g.Board.Set(pos, bld)
			// 	continue
			// }
			if c, ok := oldBoard.At(pos).(board.Controller); ok {
				g.Board.Set(pos, c)
				continue
			}
			if bot, hasBot := g.Bots[pos]; hasBot {
				g.Board.Set(pos, bot)
				continue
			}
			if util.RollChance(g.config.ResourceChance) {
				g.Board.Set(pos, board.Resource{Pos: pos, Amount: 1})
				continue
			}
		}
	}
}

func (g *Game) newGeneration() {
	g.currGen += 1
	g.generateChildren()
	g.populateBoard()
}

func (g *Game) generateChildren() {
	children := make(map[board.Position]bot.Bot)
	oldBots := g.Bots
	g.Bots = make(map[board.Position]bot.Bot)

	for _, parent := range oldBots {
		for range g.config.ChildrenByBot {
			var pos board.Position
			for range 1000 {
				pos = board.NewRandomPosition()
				if !g.Board.IsEmpty(pos) || children[pos] != (bot.Bot{}) {
					continue
				}
				break
			}
			child := parent.NewChild()
			children[pos] = child
			g.Board.Set(pos, child)
		}
	}
	g.Bots = children
}

func (g *Game) initialBotsGeneration(initialGenome *bot.Genome) {
	fmt.Printf("initialBotsGeneration\n")
	for r := range board.Rows {
		for c := range board.Cols {
			pos := board.Position{C: c, R: r}
			if g.Board.IsWall(pos) {
				g.Board.Set(pos, board.Wall{Pos: pos})
				continue
			}
			if !g.Board.IsEmpty(pos) || !util.RollChance(g.config.BotChance) {
				continue
			}
			b := bot.NewBot()
			if initialGenome != nil {
				b.Genome = *initialGenome
			}
			g.Board.Set(pos, b)
			g.Bots[pos] = b
		}
	}
}

func (g *Game) botsActions() {
	newBots := make(map[board.Position]bot.Bot)
	for startPos, b := range g.Bots {
		g.maxHp = max(b.Hp, g.maxHp)
		b.Hp -= 1
		if b.Hp <= 0 {
			continue
		}
		// if b.Hp > g.config.HpThreshold {
		// 	g.unload(b, startPos, newBots)
		// 	g.Bots = newBots
		// 	continue
		// }
		g.botAction(startPos, b, newBots)
	}
	g.Bots = newBots
}

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

func (g *Game) botAction(startPos board.Position, b bot.Bot, newBots map[board.Position]bot.Bot) {
	pos := startPos

	for steps := 0; ; steps++ {
		ptr := b.Genome.Pointer
		switch {
		case ptr < 8:
			b.PointerJump()
			pos = g.tryMove(newBots, pos, b)
			return
		case ptr < 16:
			b = g.lookAround(pos, b)
			newBots[pos] = b
			if steps >= 8 {
				return
			}
		case ptr < 24:
			b.PointerJump()
			g.grab(newBots, pos, b)
			return
		case ptr < 48:
			b.PointerJump()
			b = g.build(pos, b)
			newBots[pos] = b
			if steps >= 8 {
				return
			}
		// case ptr < 64:
		// 	b.PointerJump()
		// 	b = g.specialAction(pos, b)
		// 	newBots[pos] = b
		// 	if steps >= 8 {
		// 		return
		// 	}
		default:
			if steps >= 8 {
				newBots[pos] = b
				b.PointerJump()
				return
			}
			newBots[pos] = b
			b.PointerJump()
		}
	}
}

func (g *Game) specialAction(pos board.Position, b bot.Bot) bot.Bot {
	// ptr range 48 - 64
	// higher level actions like "unload to nearest chest"

	return b
}

func (g *Game) tryMove(newBots map[board.Position]bot.Bot, oldPos board.Position, b bot.Bot) board.Position {
	ptr := b.Genome.Pointer
	dir := board.PosClock[ptr%8]
	b.Dir = dir
	newPos := oldPos.Add(b.Dir[0], b.Dir[1])

	blocked := g.Board.IsWall(newPos) ||
		!g.Board.IsEmpty(newPos) ||
		newBots[newPos] != (bot.Bot{}) ||
		(g.Bots[newPos] != (bot.Bot{}) && newPos != oldPos)

	if blocked {
		newBots[oldPos] = b
		return oldPos
	}
	delete(newBots, oldPos)
	newBots[newPos] = b
	return newPos
}

func (g *Game) grab(newBots map[board.Position]bot.Bot, pos board.Position, b bot.Bot) {
	c := *g.config
	brd := *g.Board

	d := board.PosClock[b.Genome.Pointer%8]
	b.PointerJumpBy(b.Genome.Pointer % 8)
	dx, dy := d[0], d[1]
	grabPos := board.NewPosition(pos.R+dy, pos.C+dx)

	if !brd.IsGrabable(grabPos) {
		newBots[pos] = b
		return
	}

	switch v := brd.At(grabPos).(type) {
	case board.Building:
		if v.Owner == &b {
			return
		}
		b.Inventory.Amount += c.BuildingGrabCost
		b.Hp -= c.BuildingGrabHpGain
		brd.Set(grabPos, nil)
	case board.Spawner:
		// if v.Owner != &b {
		// 	// maybe bot should try to "steal" the spawner?
		// 	return
		// }
		if b.Inventory.Amount < c.SpawnerGrabCost {
			return
		}
		b.Inventory.Amount -= c.SpawnerGrabCost
		spawnPos, found := brd.FindEmptyPosAround(pos)
		if !found {
			return
		}
		newBots[spawnPos] = b.NewChild()
	case board.Farm:
		if b.Inventory.Amount <= 0 {
			return
		}
		b.Inventory.Amount += c.FarmGrabGain
		v.Amount += 1
		brd.Set(grabPos, v)
	case board.Food:
		b.Hp += c.FoodGrabHpGain
		g.Board.Set(grabPos, nil)
	case board.Controller:
		b.Inventory.Amount += c.ControllerGain
		v.Amount -= 1
		if v.Amount <= 0 {
			brd.Set(grabPos, nil)
		} else {
			brd.Set(grabPos, v)
		}
	case board.Resource:
		b.Inventory.Amount += c.ResourceGrabGain
		b.Hp += c.ResourceGrabHpGain
		// Todo:  adjust
		v.Amount -= 10
		if v.Amount <= 0 {
			brd.Set(grabPos, nil)
		} else {
			brd.Set(grabPos, v)
		}
	default:
		panic(fmt.Sprintf("unexpected. Type: %T", v))
	}
	newBots[pos] = b
}

func (g *Game) build(botPos board.Position, b bot.Bot) bot.Bot {
	c := g.config

	ptr := b.Genome.Pointer
	d := board.PosClock[b.Genome.Pointer%8]
	dx, dy := d[0], d[1]
	pos := board.NewPosition(botPos.R+dy, botPos.C+dx)
	if !g.Board.IsEmpty(pos) {
		return b
	}

	switch {
	case ptr < 24:
		if b.Inventory.Amount < c.BuildingBuildCost {
			return b
		}
		g.Board.Set(pos, board.Building{
			Pos:   pos,
			Owner: &b,
			Hp:    20,
		})
		b.Inventory.Amount -= c.BuildingBuildCost
	case ptr < 30:
		if b.HasSpawner {
			return b
		}
		g.Board.Set(pos, board.Spawner{
			Pos:    pos,
			Owner:  &b,
			Amount: 10,
		})
		b.HasSpawner = true
	case ptr < 32:
		if g.Board.HasController() {
			return b
		}
		g.Board.Set(pos, board.Controller{
			Pos:    pos,
			Owner:  &b,
			Amount: c.ControllerInitialAmount,
		})
		b.Hp += c.ControllerHpGain
	case ptr < 48:
		if b.Inventory.Amount < -c.FarmBuildCost {
			return b
		}
		g.Board.Set(pos, board.Farm{
			Pos:    pos,
			Amount: 1,
		})
		b.Hp += c.FarmGrabHpGain
		b.Inventory.Amount += c.FarmBuildCost
	}
	return b
}

func (g *Game) lookAround(pos board.Position, b bot.Bot) bot.Bot {
	d := board.PosClock[b.Genome.Pointer%8]
	dx, dy := d[0], d[1]
	lookupPos := board.NewPosition(pos.R+dy, pos.C+dx)

	switch g.Board.At(lookupPos).(type) {
	case bot.Bot:
		b.PointerJumpBy(1)
	case board.Building:
		b.PointerJumpBy(2)
	case board.Wall:
		b.PointerJumpBy(3)
	case board.Resource:
		b.PointerJumpBy(4)
	case board.Controller:
		b.PointerJumpBy(5)
	case board.Spawner:
		b.PointerJumpBy(6)
	case board.Farm:
		b.PointerJumpBy(7)
	case board.Food:
		g.Board.Set(lookupPos, nil)
		b.PointerJumpBy(8)
	default:
		b.PointerJump()
	}
	return b
}
