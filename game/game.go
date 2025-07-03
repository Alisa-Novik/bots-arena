package game

import (
	"fmt"
	"golab/board"
	"golab/bot"
	"golab/ui"
	"math/rand"
	"time"
)

type GenerationConfig struct {
	BotChance       int
	ResourceChance  int
	NewGenThreshold int
	ChildrenByBot   int
	InitialGenome   *bot.Genome

	ControllerInitialAmount int
	HpFromController        int
	InventoryFromController int

	HpFromResource        int
	InventoryFromResource int

	HpFromBuilding        int
	InventoryFromBuilding int

	LogicStep time.Duration
}

type Game struct {
	Board     *board.Board
	Bots      map[board.Position]bot.Bot
	GenConf   GenerationConfig
	lastLogic time.Time

	maxHp   int
	currGen int
}

func NewGame(conf GenerationConfig) *Game {
	return &Game{
		Board:     board.NewBoard(),
		Bots:      make(map[board.Position]bot.Bot),
		GenConf:   conf,
		lastLogic: time.Now(),
		maxHp:     0,
		currGen:   0,
	}
}

func (g *Game) RunHeadless() {
	g.newGeneration()
	for {
		if len(g.Bots) < g.GenConf.NewGenThreshold {
			g.newGeneration()
		}
		g.botsActions()
	}
}

func (g *Game) Run() {
	g.initialBotsGeneration(g.GenConf.InitialGenome)
	g.populateBoard()
	for !ui.Window.ShouldClose() {
		g.step()
		ui.DrawGrid(*g.Board, g.Bots)
	}
}

func (g *Game) step() {
	for time.Since(g.lastLogic) >= g.GenConf.LogicStep {
		g.printDebugInfo()
		if len(g.Bots) == 0 {
			g.initialBotsGeneration(g.GenConf.InitialGenome)
			g.populateBoard()
			return
		}
		if len(g.Bots) <= g.GenConf.NewGenThreshold {
			g.newGeneration()
		}
		g.botsActions()
		g.lastLogic = g.lastLogic.Add(g.GenConf.LogicStep)
	}
}

func (g *Game) printDebugInfo() {
	fmt.Printf("\nGeneration: %d; Max HP: %d;", g.currGen, g.maxHp)
	fmt.Printf("\nBots amount: %d", len(g.Bots))
}

func (g *Game) rollChance(percent int) bool {
	return rand.Intn(100) < percent
}

func (g *Game) populateBoard() {
	oldBoard := g.Board
	g.Board = board.NewBoard()
	for r := range board.Rows {
		for c := range board.Cols {
			pos := board.Position{X: c, Y: r}
			if g.Board.IsWall(pos) {
				g.Board.Set(pos, board.Wall{Pos: pos})
				continue
			}
			if oldBoard.IsController(pos) {
				c := oldBoard.At(pos).(board.Controller)
				g.Board.Set(pos, board.Controller{Pos: pos, Owner: nil, Amount: c.Amount})
				continue
			}
			if oldBoard.IsBuilding(pos) {
				g.Board.Set(pos, board.Building{Pos: pos, Owner: nil, Hp: 10})
				continue
			}
			// why? it's populated already
			if bot, hasBot := g.Bots[pos]; hasBot {
				g.Board.Set(pos, bot)
				continue
			}
			if g.rollChance(g.GenConf.ResourceChance) {
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
		for range g.GenConf.ChildrenByBot {
			var pos board.Position
			for {
				pos = board.NewRandomPosition()
				if g.Board.IsEmpty(pos) {
					continue
				}
				if children[pos] != (bot.Bot{}) {
					continue
				}
				break
			}
			children[pos] = parent.NewChild()
		}
	}
	g.Bots = children
}

func (g *Game) initialBotsGeneration(initialGenome *bot.Genome) {
	for r := range board.Rows {
		for c := range board.Cols {
			pos := board.Position{X: c, Y: r}
			if g.Board.IsWall(pos) {
				g.Board.Set(pos, board.Wall{Pos: pos})
				continue
			}
			if !g.Board.IsEmpty(pos) || !g.rollChance(g.GenConf.BotChance) {
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
		g.botAction(startPos, b, newBots)
	}
	g.Bots = newBots
}

func (g *Game) botAction(start board.Position, b bot.Bot, newBots map[board.Position]bot.Bot) {
	pos := start

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
		case ptr < 32:
			b.PointerJump()
			b = g.buildStructure(pos, b)
			newBots[pos] = b
			if steps >= 8 {
				return
			}
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

func (g *Game) tryMove(dst map[board.Position]bot.Bot, oldPos board.Position, b bot.Bot) board.Position {
	b.Dir = bot.RandomDir()
	newPos := board.Position{X: oldPos.X + b.Dir[0], Y: oldPos.Y + b.Dir[1]}
	blocked := g.Board.IsWall(newPos) ||
		!g.Board.IsEmpty(newPos) ||
		dst[newPos] != (bot.Bot{}) ||
		(g.Bots[newPos] != (bot.Bot{}) && newPos != oldPos)
	if blocked {
		dst[oldPos] = b
		return oldPos
	}
	delete(dst, oldPos)
	dst[newPos] = b
	return newPos
}

func (g *Game) grab(newBots map[board.Position]bot.Bot, pos board.Position, b bot.Bot) {
	d := board.PosClock[b.Genome.Pointer%8]
	dx, dy := d[0], d[1]
	grabPos := board.NewPosition(pos.Y+dy, pos.X+dx)
	if !g.Board.IsGrabable(grabPos) {
		newBots[pos] = b
		return
	}
	switch v := g.Board.At(grabPos).(type) {
	case board.Building:
		if v.Owner == &b {
			return
		}
		b.Inventory.Amount += g.GenConf.InventoryFromBuilding
		b.Hp -= g.GenConf.HpFromBuilding
		g.Board.Set(grabPos, nil)
	case board.Controller:
		b.Inventory.Amount += g.GenConf.InventoryFromController
		// b.Hp += g.GenConf.HpFromController
		v.Amount -= 1
		if v.Amount <= 0 {
			g.Board.Set(grabPos, nil)
		} else {
			g.Board.Set(grabPos, v)
		}
	case board.Resource:
		b.Inventory.Amount += g.GenConf.InventoryFromResource
		b.Hp += g.GenConf.HpFromResource
		// Todo:  adjust
		v.Amount -= 1
		if v.Amount <= 0 {
			g.Board.Set(grabPos, nil)
		} else {
			g.Board.Set(grabPos, v)
		}
	default:
		panic(fmt.Sprintf("unexpected. Type: %T", v))
	}
	newBots[pos] = b
}

func (g *Game) buildStructure(botPos board.Position, b bot.Bot) bot.Bot {
	if b.Inventory.Amount < 5 {
		return b
	}

	ptr := b.Genome.Pointer

	d := board.PosClock[b.Genome.Pointer%8]
	dx, dy := d[0], d[1]
	pos := board.NewPosition(botPos.Y+dy, botPos.X+dx)
	if g.Board.IsWall(pos) || !g.Board.IsEmpty(pos) {
		return b
	}

	switch {
	case ptr < 24:
		g.Board.Set(pos, board.Building{
			Pos:   pos,
			Owner: &b,
			Hp:    20,
		})
		b.Inventory.Amount -= 1
		b.Hp += 300
	case ptr < 32:
		// if g.Board.HasController() {
		// 	return b
		// }
		g.Board.Set(pos, board.Controller{
			Pos:    pos,
			Owner:  &b,
			Amount: g.GenConf.ControllerInitialAmount,
		})
		b.Hp += g.GenConf.HpFromController
	}

	return b
}

func (g *Game) lookAround(pos board.Position, b bot.Bot) bot.Bot {
	d := board.PosClock[b.Genome.Pointer%8]
	dx, dy := d[0], d[1]
	lookupPos := board.NewPosition(pos.Y+dy, pos.X+dx)
	switch g.Board.At(lookupPos).(type) {
	case *bot.Bot:
		b.PointerJumpBy(1)
	case *board.Building:
		b.PointerJumpBy(2)
	case *board.Wall:
		b.PointerJumpBy(3)
	case *board.Resource:
		b.PointerJumpBy(4)
	default:
		b.PointerJump()
	}
	return b
}
