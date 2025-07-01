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
	BotChance        int
	ResourceChance   int
	NewGenThreshold  int
	ControllerAmount int
	ChildrenByBot    int
	InitialGenome    *bot.Genome
}

type Game struct {
	Board        *board.Board
	Bots         map[board.Position]bot.Bot
	GenConf      GenerationConfig
	MutableState map[string]int
	lastLogic    time.Time
}

func NewGame(conf GenerationConfig) *Game {
	return &Game{
		Board:        board.NewBoard(),
		Bots:         make(map[board.Position]bot.Bot),
		GenConf:      conf,
		MutableState: make(map[string]int),
		lastLogic:    time.Now(),
	}
}

const logicStep = 100000 * time.Nanosecond

var generation = 0

func (g *Game) HeadlessRun() {
	g.newGeneration()
	maxMaxHp := 0
	for {
		for k, v := range g.MutableState {
			if k != "maxHp" || v <= maxMaxHp {
				continue
			}
			maxMaxHp = v
			fmt.Println()
			gen := g.MutableState["generation"]
			fmt.Printf("Generation: %d; Max HP: %d;", gen, v)
		}
		if len(g.Bots) < g.GenConf.NewGenThreshold {
			// sometimes not enough bots are generated btw
			generation++
			g.newGeneration()
			g.MutableState["generation"] = generation
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
	for time.Since(g.lastLogic) >= logicStep {
		// g.printDebugInfo()
		if len(g.Bots) == 0 {
			generation = 0
			g.initialBotsGeneration(g.GenConf.InitialGenome)
			g.populateBoard()
			return
		}
		if len(g.Bots) <= g.GenConf.NewGenThreshold {
			generation++
			g.MutableState["generation"] = generation
			g.newGeneration()
		}
		g.botsActions()
		g.lastLogic = g.lastLogic.Add(logicStep)
	}
}

func (g *Game) printDebugInfo() {
	maxMaxHp := 0
	for k, v := range g.MutableState {
		if k != "maxHp" || v <= maxMaxHp {
			continue
		}
		maxMaxHp = v
		gen := g.MutableState["generation"]
		fmt.Println()
		fmt.Printf("Generation: %d; Max HP: %d;", gen, v)
		fmt.Println()
		fmt.Printf("Bots amount: %d", len(g.Bots))
		fmt.Println()
	}
}

func (g *Game) rollBotGen() bool {
	return rand.Intn(1000) < g.GenConf.BotChance
}

func (g *Game) rollResourceGen() bool {
	return rand.Intn(100) < g.GenConf.ResourceChance
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
			if g.rollResourceGen() {
				g.Board.Set(pos, board.Resource{Pos: pos, Amount: 1})
				continue
			}
		}
	}
}

func (g *Game) newGeneration() {
	g.generateChildren()
	g.populateBoard()
}

func (g *Game) generateChildren() {
	children := make(map[board.Position]bot.Bot)
	g.Bots = make(map[board.Position]bot.Bot)

	for _, parent := range g.Bots {
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
			if _, ok := g.Bots[pos]; ok {
				continue
			}
			if !g.Board.IsEmpty(pos) || !g.rollBotGen() {
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
		if b.Hp > g.MutableState["maxHp"] {
			g.MutableState["maxHp"] = b.Hp
			// util.ExportGenome(b)
		}

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
		// b.Inventory.Amount -= 1000
		// b.Hp -= 1000
		// g.Board.Set(grabPos, nil)
	case board.Controller:
		b.Inventory.Amount += 100
		v.Amount -= 100
		if v.Amount == 0 {
			g.Board.Set(grabPos, nil)
		} else {
			g.Board.Set(grabPos, v)
		}
	case board.Resource:
		b.Inventory.Amount += 5000
		b.Hp += 10
		v.Amount -= 1
		if v.Amount == 0 {
			g.Board.Set(grabPos, nil)
		} else {
			g.Board.Set(grabPos, v)
		}
	default:
		panic(fmt.Sprintf("unexpected. Type: %T", v))
	}
	newBots[pos] = b
}

func (g *Game) buildStructure(pos board.Position, b bot.Bot) bot.Bot {
	if b.Inventory.Amount < 5 {
		return b
	}

	ptr := b.Genome.Pointer
	d := board.PosClock[b.Genome.Pointer%8]
	dx, dy := d[0], d[1]
	buildPos := board.NewPosition(pos.Y+dy, pos.X+dx)
	if g.Board.IsWall(buildPos) || !g.Board.IsEmpty(buildPos) {
		return b
	}
	if g.Board.IsResource(buildPos) {
		controller := g.Board.At(buildPos).(board.Controller)
		controller.Amount -= g.GenConf.ControllerAmount
		b.Inventory.Amount += g.GenConf.ControllerAmount
	}

	switch {
	case ptr < 24:
		g.Board.Set(buildPos, board.Building{
			Pos:   buildPos,
			Owner: &b,
			Hp:    20,
		})
		b.Inventory.Amount -= 1
		b.Hp += 300
	case ptr < 32:
		g.Board.Set(buildPos, board.Controller{
			Pos:    buildPos,
			Owner:  &b,
			Amount: g.GenConf.ControllerAmount,
		})
		b.Inventory.Amount = 0
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
