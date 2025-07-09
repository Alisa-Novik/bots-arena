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
	Bots   map[board.Position]*bot.Bot
	config *conf.Config
	State  *conf.GameState

	maxHp             int
	currGen           int
	latestImprovement int
}

func NewGame(config *conf.Config) *Game {
	return &Game{
		Board:  board.NewBoard(),
		Bots:   make(map[board.Position]*bot.Bot),
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
				v.Amount--
				g.Board.Set(pos, v)
			}
		}
	}
}

var newGenerations = 0

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
	fmt.Printf(" Latest improvement: %d;", g.latestImprovement)
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
	children := make(map[board.Position]*bot.Bot)

	for _, parent := range g.Bots {
		for i := 0; i < g.config.ChildrenByBot; i++ {
			pos := board.NewRandomPosition()
			if !g.Board.IsEmpty(pos) || children[pos] != nil {
				continue
			}
			child := parent.NewChild()
			children[pos] = &child
			g.Board.Set(pos, &child)
		}
	}
	g.Bots = children
}

func (g *Game) initialBotsGeneration(initialGenome *bot.Genome) {
	fmt.Printf("initialBotsGeneration, %v\n", newGenerations)
	newGenerations += 1
	for r := range board.Rows {
		for c := range board.Cols {
			pos := board.Position{C: c, R: r}
			if !g.Board.IsEmpty(pos) || !util.RollChance(g.config.BotChance) {
				continue
			}
			b := bot.NewBot()
			if initialGenome != nil {
				b.Genome = *initialGenome
			}
			g.Bots[pos] = &b
			g.Board.Set(pos, &b)
		}
	}
}

func (g *Game) botsActions() {
	for pos, b := range g.Bots {
		if b.Hp >= g.maxHp {
			g.latestImprovement = g.currGen
			g.maxHp = b.Hp
			// b.SaveGenomeIntoFile()
		}
		b.Hp--
		if b.Hp <= 0 {
			delete(g.Bots, pos)
			g.Board.Clear(pos)
			continue
		}
		g.botAction(pos, b)
		if b.Hp > 200 {
			newPos, ok := g.Board.FindEmptyPosAround(pos)
			if !ok {
				continue
			}
			child := b.NewChild()
			b.Hp -= 100
			g.Bots[newPos] = &child
			g.Board.Set(newPos, &child)
		}
	}
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

func (g *Game) botAction(pos board.Position, b *bot.Bot) {
	for range 15 {
		op := bot.Opcode(b.Genome.Matrix[b.Genome.Pointer])
		// fmt.Printf("opcode: %v\n", op)

		switch op {
		case bot.OpMoveAbs:
			b.Dir = util.PosClock[b.CmdArg(1)%8]
			g.tryMove(pos, b)
			b.PointerJumpBy(1)
			return

		case bot.OpCheckIfBro:
			checkPos := b.CmdArgDir(1, pos)
			other, ok := g.Board.At(checkPos).(*bot.Bot)
			if !ok {
				b.PointerJumpBy(1)
				continue
			}
			// fmt.Printf("I check if %v is bro. Is bro? %v\n", checkPos, b.IsBro(other))
			if b.IsBro(other) {
				b.PointerJumpBy(9)
				continue
			}
		case bot.OpMove:
			g.tryMove(pos, b)
			b.PointerJumpBy(1)
			return

		case bot.OpTurn:
			b.Dir = util.PosClock[b.CmdArg(1)%8]
			b.PointerJumpBy(1)
			continue

		case bot.OpEatOther:
			arg := b.CmdArg(1)
			dir := util.PosClock[arg%8]
			eatPos := pos.Add(dir[0], dir[1])
			other, ok := g.Board.At(eatPos).(*bot.Bot)

			if ok && b.Hp >= other.Hp {
				fmt.Printf("I eat. Hp: %d; other.Hp: %d\n. Is bro? %v\n", b.Hp, other.Hp, b.IsBro(other))
				b.Hp += other.Hp
				color := b.Color
				b.Color = [3]float32{color[0] + 0.4, color[1], color[2]}
				g.Board.Clear(eatPos)
				delete(g.Bots, eatPos)
			}
			return

		case bot.OpPhoto:
			if util.RollChance(g.config.PhotoChance) {
				b.Hp += g.config.PhotoHpGain
			}
			return

		case bot.OpLook:
			g.lookAround(pos, b)

		case bot.OpCheckInventory:
			if b.Inventory.Amount > 70 {
				b.PointerJumpBy(3)
			} else {
				b.PointerJumpBy(5)
			}

		case bot.OpCheckHp:
			if b.Hp > 100 {
				b.PointerJumpBy(3)
			} else {
				b.PointerJumpBy(5)
			}

		case bot.OpGrab:
			g.grab(pos, b)
			b.PointerJumpBy(1)
			return

		case bot.OpShareHp:
			if b.Hp < 20 {
				b.PointerJumpBy(5)
				continue
			}
			sharePos := b.CmdArgDir(1, pos)
			other, ok := g.Board.At(sharePos).(*bot.Bot)
			if !ok {
				b.PointerJumpBy(5)
				continue
			}
			other.Hp += 10
			b.Hp -= 10
			fmt.Printf("I share %d HP with %v. Is bro? %v\n", 10, sharePos, b.IsBro(other))
			b.PointerJumpBy(1)
			return
		case bot.OpShareInventory:
			if b.Inventory.Amount < 5 {
				b.PointerJumpBy(5)
				continue
			}
			sharePos := b.CmdArgDir(1, pos)
			other, ok := g.Board.At(sharePos).(*bot.Bot)
			if !ok {
				b.PointerJumpBy(5)
				continue
			}
			other.Inventory.Amount += 3
			b.Inventory.Amount -= 3
			fmt.Printf("I share %d Inventory with %v. Is bro? %v\n", 3, sharePos, b.IsBro(other))
			b.PointerJumpBy(1)
			return

		case bot.OpBuild:
			g.build(pos, b)
			b.PointerJumpBy(1)

		default:
			b.PointerJump()
			return
		}
	}
}

func (g *Game) tryMove(oldPos board.Position, b *bot.Bot) board.Position {
	newPos := oldPos.Add(b.Dir[0], b.Dir[1])
	if !g.Board.IsEmpty(newPos) {
		return newPos
	}
	delete(g.Bots, oldPos)
	g.Board.Clear(oldPos)
	g.Board.Set(newPos, b)
	g.Bots[newPos] = b
	return newPos
}

func (g *Game) grab(pos board.Position, b *bot.Bot) {
	c := *g.config

	dir := util.PosClock[b.CmdArg(1)%8]
	grabPos := pos.Add(dir[0], dir[1])

	if !g.Board.IsGrabable(grabPos) {
		return
	}

	switch v := g.Board.At(grabPos).(type) {
	case board.Building:
		b.Inventory.Amount += c.BuildingGrabGain
		b.Hp += c.BuildingGrabHpGain
		g.Board.Set(grabPos, nil)
	case board.Spawner:
		if b.Inventory.Amount < c.SpawnerGrabCost {
			return
		}
		b.Inventory.Amount -= c.SpawnerGrabCost
		spawnPos, found := g.Board.FindEmptyPosAround(pos)
		if !found {
			return
		}
		child := b.NewChild()
		g.Board.Set(spawnPos, &child)
		g.Bots[spawnPos] = &child
	case board.Farm:
		if v.Owner != b {
			b.Inventory.Amount = 0
			b.Hp += v.Amount + 50
			return
		}
		if b.Inventory.Amount <= 0 {
			return
		}
		b.Inventory.Amount += c.FarmGrabGain
		v.Amount += 1
		g.Board.Set(grabPos, v)
	case board.Food:
		b.Hp += c.FoodGrabHpGain
		g.Board.Set(grabPos, nil)
	case board.Controller:
		b.Inventory.Amount += c.ControllerGain
		v.Amount -= 1
		if v.Amount <= 0 {
			g.Board.Set(grabPos, nil)
		} else {
			g.Board.Set(grabPos, v)
		}
	case board.Resource:
		b.Inventory.Amount += c.ResourceGrabGain
		b.Hp += c.ResourceGrabHpGain
		// Todo:  adjust
		v.Amount -= 10
		if v.Amount <= 0 {
			g.Board.Set(grabPos, nil)
		} else {
			g.Board.Set(grabPos, v)
		}
	default:
		panic(fmt.Sprintf("unexpected. Type: %T", v))
	}
}

func (g *Game) build(botPos board.Position, b *bot.Bot) {
	c := g.config
	dir := util.PosClock[b.CmdArg(1)%8]
	buildPos := botPos.Add(dir[0], dir[1])

	if !g.Board.IsEmpty(buildPos) {
		return
	}

	buildType := bot.BuildType(b.CmdArg(2) % bot.BuildTypesCount())

	switch buildType {
	case bot.BuildWall:
		if b.Inventory.Amount < c.BuildingBuildCost {
			return
		}
		g.Board.Set(buildPos, board.Building{
			Pos:   buildPos,
			Owner: b,
			Hp:    20,
		})
		b.Inventory.Amount -= c.BuildingBuildCost
		b.Hp += c.BuildingBuildHpGain
	case bot.BuildSpawner:
		// if b.HasSpawner {
		// 	return
		// }
		// g.Board.Set(buildPos, board.Spawner{
		// 	Pos:    buildPos,
		// 	Owner:  b,
		// 	Amount: 10,
		// })
		// b.HasSpawner = true
	case bot.BuildController:
		if g.Board.HasController() {
			return
		}
		g.Board.Set(buildPos, board.Controller{
			Pos:    buildPos,
			Owner:  b,
			Amount: c.ControllerInitialAmount,
		})
		b.Hp += c.ControllerHpGain
	case bot.BuildFarm:
		if c.DisableFarms {
			return
		}
		if b.Inventory.Amount < -c.FarmBuildCost {
			return
		}
		g.Board.Set(buildPos, board.Farm{
			Pos:    buildPos,
			Owner:  b,
			Amount: 1,
		})
		b.Hp += c.FarmBuildHpGain
		b.Inventory.Amount += c.FarmBuildCost
	}
}

func (g *Game) lookAround(botPos board.Position, b *bot.Bot) {
	dir := util.PosClock[b.CmdArg(1)%8]
	lookPos := botPos.Add(dir[0], dir[1])

	switch g.Board.At(lookPos).(type) {
	case *bot.Bot:
		other, ok := g.Bots[lookPos]
		if !ok || other != nil {
			b.PointerJumpBy(13)
			return
		}
		if b.IsBro(other) {
			b.PointerJumpBy(20)
			return
		}
	case board.Building:
		b.PointerJumpBy(4)
	case board.Wall:
		b.PointerJumpBy(8)
	case board.Resource:
		b.PointerJumpBy(11)
	case board.Controller:
		b.PointerJumpBy(50)
	case board.Spawner:
		b.PointerJumpBy(61)
	case board.Farm:
		b.PointerJumpBy(7)
	case board.Food:
		b.PointerJumpBy(9)
	default:
		b.PointerJumpBy(12)
	}

	log := false
	if log && g.Board.At(lookPos) != nil {
		nextOp := bot.Opcode(b.Genome.Matrix[b.Genome.Pointer])
		fmt.Printf("I am at %v ", botPos)
		fmt.Printf("I look at %v ", lookPos)
		fmt.Printf("; I see %T; ", g.Board.At(lookPos))
		fmt.Printf("My next action is %v\n", nextOp.String())
	}
}
