package main

import (
	"fmt"
	"golab/board"
	"golab/bot"
	"golab/ui"
	"math/rand"
	"runtime"
	"time"

	"github.com/go-gl/glfw/v3.3/glfw"
)

type GenerationConfig struct {
	BotChance       int
	ResourceChance  int
	NewGenThreshold int
	ChildrenByBot   int
}

var genConf GenerationConfig = GenerationConfig{
	BotChance:       5,
	ResourceChance:  5,
	NewGenThreshold: 5,
	ChildrenByBot:   3,
}

type Position = board.Position
type Board = board.Board

const (
	rows = board.Rows
	cols = board.Cols
)

const logicStep = 200000 * time.Nanosecond

var lastLogic = time.Now()
var brd Board = *board.NewBoard()

var bots = map[Position]bot.Bot{}

func init() { runtime.LockOSThread() }

func main() {
	ui.PrepareUi()
	defer glfw.Terminate()
	gameLoop()
}

func gameLoop() {
	newGeneration()

	for !ui.Window.ShouldClose() {
		now := time.Now()

		for now.Sub(lastLogic) >= logicStep {
			if len(bots) < genConf.NewGenThreshold {
				newGeneration()
			}
			botsActions()
			lastLogic = lastLogic.Add(logicStep)
		}

		ui.DrawGrid(brd, bots)
	}
}

func populateBoard() {
	brd = *board.NewBoard()
	for r := range rows {
		for c := range cols {
			pos := Position{X: c, Y: r}

			if brd.IsWall(pos) {
				brd.Set(pos, board.Wall{Pos: pos})
				continue
			}

			if bot, hasBot := bots[pos]; hasBot {
				brd.Set(pos, bot)
				continue
			}

			if rollResourceGen() {
				brd.Set(pos, board.Resource{Pos: pos, Amount: 1})
				continue
			}
		}
	}
}

func rollBotGen() bool {
	return rand.Intn(100) < genConf.BotChance
}

func rollResourceGen() bool {
	return rand.Intn(100) < genConf.ResourceChance
}

func newGeneration() {
	if len(bots) == 0 {
		fmt.Println("initial")
		initialBotsGeneration()
		populateBoard()
		return
	}
	fmt.Println("children")
	generateChildren()
	populateBoard()
}

func generateChildren() {
	children := make(map[Position]bot.Bot)

	for _, parent := range bots {
		for range genConf.ChildrenByBot {
			var pos board.Position
			for {
				pos = board.NewRandomPosition()
				if brd.IsWall(pos) {
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
	bots = children
}

func initialBotsGeneration() {
	i := 0
	for r := range rows {
		for c := range cols {
			pos := Position{X: c, Y: r}
			if brd.IsWall(pos) {
				brd.Set(pos, board.Wall{Pos: pos})
				continue
			}
			if !brd.IsEmpty(pos) || !rollBotGen() {
				continue
			}
			_, ok := bots[pos]
			if ok {
				continue
			}
			fmt.Println("bots generated: %d", i)
			i++
			b := bot.NewBot()
			brd.Set(pos, b)
			bots[pos] = b
		}
	}
}

func tryMove(dst map[Position]bot.Bot, oldPos Position, b bot.Bot) Position {
	b.Dir = bot.RandomDir()
	newPos := Position{X: oldPos.X + b.Dir[0], Y: oldPos.Y + b.Dir[1]}

	blocked := brd.IsWall(newPos) ||
		!brd.IsEmpty(newPos) ||
		dst[newPos] != (bot.Bot{}) ||
		(bots[newPos] != (bot.Bot{}) && newPos != oldPos)

	if blocked {
		dst[oldPos] = b
		return oldPos
	}

	delete(dst, oldPos)
	dst[newPos] = b
	return newPos
}

func botsActions() {
	newBots := make(map[Position]bot.Bot)

	for startPos, b := range bots {
		b.Hp -= 1
		if b.Hp <= 0 {
			continue
		}
		botAction(startPos, b, newBots)
	}

	bots = newBots
}

func botAction(startPos Position, b bot.Bot, newBots map[Position]bot.Bot) {
	curPos := startPos
	shouldStop := false

	cmds := 0
	for !shouldStop {
		ptr := b.Genome.Pointer
		cmds++
		switch {
		case ptr < 8:
			b.PointerJump()
			curPos = tryMove(newBots, curPos, b)
			shouldStop = true
		case ptr < 16:
			lookAround(newBots, curPos, b)
			if cmds > 8 {
				shouldStop = true
			}
		case ptr < 24:
			b.PointerJump()
			grab(newBots, curPos, b)
			shouldStop = true
		case ptr < 32:
			b.PointerJump()
			buildStructure(newBots, curPos, b)
			if cmds > 8 {
				shouldStop = true
			}
		// case ptr < 36:
		// 	dir := bot.IntToDir[b.Genome.Pointer%4]
		// 	b.Dir = dir
		// 	newBots[curPos] = b
		// 	// rotate(newBots, curPos, b)
		// 	b.PointerJump()
		//
		// 	if cmds > 8 {
		// 		shouldStop = true
		// 	}
		case ptr < 64:
			if cmds > 8 {
				shouldStop = true
			}
			newBots[curPos] = b
			b.PointerJump()
		}
	}
}

func rotate(newBots map[Position]bot.Bot, curPos Position, b bot.Bot) {
	// ptr := b.Genome.Pointer
	// d :=
	// 	fmt.Println("ptr: %d; d: %d", ptr, d)
	// b.Dir = bot.Up
	// fmt.Println("dir %d", dir)
	// newBots[curPos] = b
}

func grab(newBots map[Position]bot.Bot, pos Position, b bot.Bot) {
	d := board.PosClock[b.Genome.Pointer%8]
	dx, dy := d[0], d[1]
	grabPos := board.NewPosition(pos.Y+dy, pos.X+dx)
	if !brd.IsResource(grabPos) && !brd.IsBuilding(grabPos) {
		newBots[pos] = b
		return
	}
	switch v := brd.At(grabPos).(type) {
	case board.Building:
		if v.Owner == &b {
			return
		}
		b.Hp += 10
		b.Inventory.Amount -= 1
		brd.Set(grabPos, nil)
	case board.Resource:
		b.Inventory.Amount += 10
		b.Hp += 100
		v.Amount -= 1
		if v.Amount == 0 {
			brd.Set(grabPos, nil)
		} else {
			brd.Set(grabPos, v)
		}
	default:
		panic(fmt.Sprintf("unexpected. Type: %T", v))
	}

	newBots[pos] = b
}

func buildStructure(newBots map[Position]bot.Bot, pos Position, b bot.Bot) {
	if b.Inventory.Amount < 5 {
		newBots[pos] = b
		return
	}

	d := board.PosClock[b.Genome.Pointer%8]
	dx, dy := d[0], d[1]
	buildPos := board.NewPosition(pos.Y+dy, pos.X+dx)
	if brd.IsWall(buildPos) || !brd.IsEmpty(buildPos) {
		newBots[pos] = b
		return
	}
	brd.Set(buildPos, board.Building{
		Pos:   buildPos,
		Owner: &b,
		Hp:    20,
	})
	b.Inventory.Amount -= 5
	b.Hp += 10

	newBots[pos] = b
}

func lookAround(newBots map[Position]bot.Bot, pos Position, b bot.Bot) {
	d := board.PosClock[b.Genome.Pointer%8]
	dx, dy := d[0], d[1]
	lookupPos := board.NewPosition(pos.Y+dy, pos.X+dx)
	switch brd.At(lookupPos).(type) {
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
