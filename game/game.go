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
}

type Game struct {
	Board     *board.Board
	Bots      map[board.Position]bot.Bot
	GenConf   GenerationConfig
	lastLogic time.Time
}

func NewGame(conf GenerationConfig) *Game {
	return &Game{
		Board:     board.NewBoard(),
		Bots:      make(map[board.Position]bot.Bot),
		GenConf:   conf,
		lastLogic: time.Now(),
	}
}

const logicStep = 200_000 * time.Nanosecond

func (g *Game) Run() {
	g.newGeneration()
	for !ui.Window.ShouldClose() {
		now := time.Now()
		for now.Sub(g.lastLogic) >= logicStep {
			if len(g.Bots) < g.GenConf.NewGenThreshold {
				g.newGeneration()
			}
			g.botsActions()
			g.lastLogic = g.lastLogic.Add(logicStep)
		}
		ui.DrawGrid(*g.Board, g.Bots)
	}
}

func (g *Game) rollBotGen() bool {
	return rand.Intn(100) < g.GenConf.BotChance
}

func (g *Game) rollResourceGen() bool {
	return rand.Intn(100) < g.GenConf.ResourceChance
}

func (g *Game) populateBoard() {
	g.Board = board.NewBoard()
	for r := range board.Rows {
		for c := range board.Cols {
			pos := board.Position{X: c, Y: r}
			if g.Board.IsWall(pos) {
				g.Board.Set(pos, board.Wall{Pos: pos})
				continue
			}
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
	if len(g.Bots) == 0 {
		g.initialBotsGeneration()
		g.populateBoard()
		return
	}
	g.generateChildren()
	g.populateBoard()
}

func (g *Game) generateChildren() {
	children := make(map[board.Position]bot.Bot)
	for _, parent := range g.Bots {
		for range g.GenConf.ChildrenByBot {
			var pos board.Position
			for {
				pos = board.NewRandomPosition()
				if g.Board.IsWall(pos) {
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

func (g *Game) initialBotsGeneration() {
	for r := range board.Rows {
		for c := range board.Cols {
			pos := board.Position{X: c, Y: r}
			if g.Board.IsWall(pos) {
				g.Board.Set(pos, board.Wall{Pos: pos})
				continue
			}
			if !g.Board.IsEmpty(pos) || !g.rollBotGen() {
				continue
			}
			if _, ok := g.Bots[pos]; ok {
				continue
			}
			b := bot.NewBot()
			g.Board.Set(pos, b)
			g.Bots[pos] = b
		}
	}
}

func (g *Game) botsActions() {
	newBots := make(map[board.Position]bot.Bot)
	for startPos, b := range g.Bots {
		b.Hp -= 1
		if b.Hp <= 0 {
			continue
		}
		g.botAction(startPos, b, newBots)
	}
	g.Bots = newBots
}

func (g *Game) botAction(startPos board.Position, b bot.Bot, newBots map[board.Position]bot.Bot) {
	curPos := startPos
	shouldStop := false
	cmds := 0
	for !shouldStop {
		ptr := b.Genome.Pointer
		cmds++
		switch {
		case ptr < 8:
			b.PointerJump()
			curPos = g.tryMove(newBots, curPos, b)
			shouldStop = true
		case ptr < 16:
			g.lookAround(newBots, curPos, b)
			if cmds > 8 {
				shouldStop = true
			}
		case ptr < 24:
			b.PointerJump()
			g.grab(newBots, curPos, b)
			shouldStop = true
		case ptr < 32:
			b.PointerJump()
			g.buildStructure(newBots, curPos, b)
			if cmds > 8 {
				shouldStop = true
			}
		default:
			if cmds > 8 {
				shouldStop = true
			}
			newBots[curPos] = b
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
	if !g.Board.IsResource(grabPos) && !g.Board.IsBuilding(grabPos) {
		newBots[pos] = b
		return
	}
	switch v := g.Board.At(grabPos).(type) {
	case board.Building:
		if v.Owner == &b {
			return
		}
		b.Hp += 10
		b.Inventory.Amount -= 1
		g.Board.Set(grabPos, nil)
	case board.Resource:
		b.Inventory.Amount += 10
		b.Hp += 100
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

func (g *Game) buildStructure(newBots map[board.Position]bot.Bot, pos board.Position, b bot.Bot) {
	if b.Inventory.Amount < 5 {
		newBots[pos] = b
		return
	}
	d := board.PosClock[b.Genome.Pointer%8]
	dx, dy := d[0], d[1]
	buildPos := board.NewPosition(pos.Y+dy, pos.X+dx)
	if g.Board.IsWall(buildPos) || !g.Board.IsEmpty(buildPos) {
		newBots[pos] = b
		return
	}
	g.Board.Set(buildPos, board.Building{
		Pos:   buildPos,
		Owner: &b,
		Hp:    20,
	})
	b.Inventory.Amount -= 5
	b.Hp += 10
	newBots[pos] = b
}

func (g *Game) lookAround(newBots map[board.Position]bot.Bot, pos board.Position, b bot.Bot) {
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
	newBots[pos] = b
}
