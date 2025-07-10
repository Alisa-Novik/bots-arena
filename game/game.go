package game

import (
	"fmt"
	"golab/board"
	"golab/bot"
	conf "golab/config"
	"golab/ui"
	"golab/util"
	"math/rand"
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
	g.generateWater()
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

func (g *Game) generateWater() {
	waterSpread := 3 * util.ScaleFactor
	for range waterSpread {
		center := board.NewRandomPosition()
		radius := 1 + rand.Intn(4)
		for dr := -radius; dr <= radius; dr++ {
			for dc := -radius; dc <= radius+15; dc++ {
				if rand.Float64() > 0.6 {
					continue
				}
				pos := center.Add(dc, dr)
				if !g.Board.IsWall(pos) {
					g.Board.Set(pos, board.Water{})
				}
			}
		}
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
			// g.newGeneration()
			g.initialBotsGeneration(g.config.InitialGenome)
			g.populateBoard()
			return
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
			if spawner, ok := oldBoard.At(pos).(board.Spawner); ok {
				g.Board.Set(pos, spawner)
				continue
			}
			if f, ok := oldBoard.At(pos).(board.Farm); ok {
				g.Board.Set(pos, f)
				continue
			}
			if fd, ok := oldBoard.At(pos).(board.Food); ok {
				g.Board.Set(pos, fd)
				continue
			}
			if bld, ok := oldBoard.At(pos).(board.Building); ok {
				g.Board.Set(pos, bld)
				continue
			}
			if c, ok := oldBoard.At(pos).(board.Water); ok {
				g.Board.Set(pos, c)
				continue
			}
			if c, ok := oldBoard.At(pos).(board.Controller); ok {
				g.Board.Set(pos, c)
				continue
			}
			if bot, hasBot := g.Bots[pos]; hasBot {
				g.Board.Set(pos, bot)
				continue
			}
			// if util.RollChanceOf(1000, g.config.PoisonChance) {
			// 	g.Board.Set(pos, board.Food{Pos: pos, Amount: 1})
			// 	continue
			// }
			if util.RollChanceOf(1000, g.config.PoisonChance) {
				g.Board.Set(pos, board.Poison{Pos: pos})
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
		b.Hp = min(b.Hp, 500)
		if b.Hp <= 0 {
			delete(g.Bots, pos)
			g.Board.Set(pos, board.Organics{Pos: pos, Amount: 10})
			continue
		}
		g.botAction(pos, b)
	}
}

func (g *Game) botAction(pos board.Position, b *bot.Bot) {
	for range 15 {
		op := bot.Opcode(b.Genome.Matrix[b.Genome.Pointer])
		// fmt.Printf("opcode: %v\n", op)

		switch op {
		case bot.OpDivide:
			if b.Hp < 100 {
				b.PointerJumpBy(5)
				return
			}

			newPos, ok := g.Board.FindEmptyPosAround(pos)
			if !ok {
				continue
			}
			child := b.NewChild()
			b.Hp -= 15
			g.Bots[newPos] = &child
			g.Board.Set(newPos, &child)
			return

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

		case bot.OpAttack:
			attackPos := b.CmdArgDir(1, pos)
			other, ok := g.Board.At(attackPos).(*bot.Bot)
			if !ok {
				b.PointerJumpBy(1)
				continue
			}
			if b.Hp > other.Hp {
				fmt.Printf("I attack %v. Hp: %d; other.Hp: %d\n", attackPos, b.Hp, other.Hp)
				b.Hp -= other.Hp
				other.Hp = 0
				g.Board.Clear(attackPos)
				delete(g.Bots, attackPos)
			}
			b.PointerJumpBy(5)
			return

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
				// fmt.Printf("I eat. Hp: %d; other.Hp: %d\n. Is bro? %v\n", b.Hp, other.Hp, b.IsBro(other))
				b.Hp += other.Hp
				color := b.Color
				b.Color = [3]float32{color[0] + 0.4, color[1], color[2]}
				g.Board.Clear(eatPos)
				delete(g.Bots, eatPos)
			}
			return

		case bot.OpPhoto:
			row := pos.R
			rows := util.Rows

			var photoChance int
			switch {
			case row < rows*1/10:
				photoChance = 90
			case row < rows*2/10:
				photoChance = 50
			case row < rows*3/10:
				photoChance = 30
			default:
				photoChance = 10
			}

			if util.RollChance(photoChance) {
				b.Hp += g.config.PhotoHpGain
			}
			b.PointerJumpBy(1)
			return

		case bot.OpLook:
			g.lookAround(pos, b)
			continue

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

		case bot.OpEatOrganicsAbs:
			nextPos := b.CmdArgDir(1, pos)
			o, ok := g.Board.At(nextPos).(board.Organics)
			if !ok {
				b.PointerJumpBy(2)
				continue
			}
			fmt.Printf("I eat organics abs. Hp after: %d; \n", b.Hp)
			b.Hp += o.Amount
			g.Board.Clear(nextPos)
			b.PointerJumpBy(1)
			return

		case bot.OpEatOrganics:
			nextPos := pos.Add(b.Dir[0], b.Dir[1])
			o, ok := g.Board.At(nextPos).(board.Organics)
			if !ok {
				b.PointerJumpBy(2)
				continue
			}
			fmt.Printf("I eat organics . Hp after: %d; \n", b.Hp)
			b.Hp += o.Amount
			g.Board.Clear(nextPos)
			b.PointerJumpBy(1)
			return

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
			shareAmount := b.CmdArg(2)
			if b.Hp < shareAmount {
				b.PointerJumpBy(5)
				continue
			}
			other, ok := g.Board.At(sharePos).(*bot.Bot)
			if !ok {
				b.PointerJumpBy(4)
				continue
			}
			other.Hp += shareAmount
			b.Hp -= shareAmount
			fmt.Printf("I share %d HP with %v. Is bro? %v\n", shareAmount, sharePos, b.IsBro(other))
			b.PointerJumpBy(3)
			return

		case bot.OpShareInventory:
			if b.Inventory.Amount < 5 {
				b.PointerJumpBy(5)
				continue
			}
			sharePos := b.CmdArgDir(1, pos)
			shareAmount := b.CmdArg(2)
			other, ok := g.Board.At(sharePos).(*bot.Bot)
			if !ok {
				b.PointerJumpBy(3)
				continue
			}
			other.Inventory.Amount += shareAmount
			b.Inventory.Amount -= shareAmount
			fmt.Printf("I share %d Inventory with %v. Is bro? %v\n", shareAmount, sharePos, b.IsBro(other))
			b.PointerJumpBy(2)
			return

		case bot.OpBuild:
			g.build(pos, b)
			b.PointerJumpBy(1)
			continue

		case bot.OpSetReg:
			reg := b.CmdArg(1) % 4
			val := b.CmdArg(2)
			// fmt.Printf("I set reg %v to %v\n", reg, b.CmdArg(2))
			b.Registers[reg] = val
			b.PointerJumpBy(3)
			return

		case bot.OpIncReg:
			reg := b.CmdArg(1) % 4
			// fmt.Printf("I inc reg %v\n", reg)
			b.Registers[reg]++
			b.PointerJumpBy(2)
			continue

		case bot.OpDecReg:
			reg := b.CmdArg(1) % 4
			// fmt.Printf("I dec reg %v\n", reg)
			b.Registers[reg]--
			b.PointerJumpBy(2)
			continue

		case bot.OpCmpReg:
			regA := b.CmdArg(1) % 4
			regB := b.CmdArg(2) % 4
			// fmt.Printf("I cmp reg %v to %v\n", regA, regB)
			if b.Registers[regA] == b.Registers[regB] {
				b.PointerJumpBy(3)
			} else {
				b.PointerJumpBy(3)
			}
			continue

		case bot.OpJumpIfZero:
			reg := b.CmdArg(1) % 4
			// fmt.Printf("I jump if reg %v is zero\n", reg)
			jump := b.CmdArg(2)
			if b.Registers[reg] == 0 {
				b.PointerJumpBy(3 + jump)
			} else {
				b.PointerJumpBy(3)
			}
			continue

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

	// dir := util.PosClock[b.CmdArg(1)%8]
	// TODO: decide. this is test one
	dir := b.Dir
	grabPos := pos.Add(dir[0], dir[1])

	if !g.Board.IsGrabable(grabPos) {
		return
	}

	switch v := g.Board.At(grabPos).(type) {
	case board.Building:
		if v.Owner == b {
			return
		}
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
			g.Board.Clear(grabPos)
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
		g.Board.Clear(grabPos)
	case board.Poison:
		b.Hp = 0
		g.Board.Clear(pos)
		g.Board.Clear(grabPos)
	case board.Controller:
		b.Inventory.Amount += c.ControllerGain
		v.Amount -= 1
		if v.Amount <= 0 {
			g.Board.Set(grabPos, nil)
		} else {
			g.Board.Set(grabPos, v)
		}
	case board.Resource:
		if b.Inventory.Amount > 10 {
			b.Hp -= 10
		}
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
			Amount: c.FarmInitialAmount,
		})
		b.Hp += c.FarmBuildHpGain
		b.Inventory.Amount += c.FarmBuildCost
	}
}

func (g *Game) lookAround(botPos board.Position, b *bot.Bot) {
	dir := util.PosClock[b.CmdArg(1)%8]
	lookPos := botPos.Add(dir[0], dir[1])
	// reg := b.CmdArg(2) % 4
	// b.Registers[reg] = 0
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
	case board.Poison:
		b.PointerJumpBy(14)
	case board.Water:
		b.PointerJumpBy(15)
	case board.Organics:
		b.PointerJumpBy(3)
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
