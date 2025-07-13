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
	Board         *board.Board
	Bots          []*bot.Bot
	Colonies      []*bot.Colony
	InitialGenome *bot.Genome
	config        *conf.Config
	State         *conf.GameState

	maxHp             int
	currGen           int
	latestImprovement int
}

func idx(p board.Position) int {
	return util.Idx(p)
}

func NewGame(config *conf.Config) *Game {
	useInitialGenome := config.UseInitialGenome
	return &Game{
		Board:         board.NewBoard(),
		Bots:          make([]*bot.Bot, util.Cells),
		InitialGenome: bot.GetInitialGenome(useInitialGenome),
		config:        config,
		State:         &conf.GameState{LastLogic: time.Now()},
		maxHp:         0,
		currGen:       0,
	}
}

func (g *Game) RunHeadless() {
	g.initialBotsGeneration()
	for {
		if g.liveBotCount() < g.config.NewGenThreshold {
			g.initialBotsGeneration()
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
			case board.Organics:
				if v.Amount <= 1 {
					g.Board.Clear(pos)
					continue
				}
				v.Amount--
				g.Board.Set(pos, v)
				continue
			case board.Controller:
				g.handleController(&v, pos, 10)
				g.Board.Set(pos, v)
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

func (g *Game) killBot(b *bot.Bot, botIdx int) {
	if c := b.Colony; c != nil {
		c.RemoveMember(b)
	}
	if p := b.Parent; p != nil {
		p.RemoveOffspring(b)
	}
	g.Bots[botIdx] = nil
	g.Board.MarkDirty(botIdx)
	*b = bot.Bot{}
	bot.BotPool.Put(b)
}

// TODO: subject to optimization
func (g *Game) handleController(ctrl *board.Controller, pos util.Position, radius int) {
	if ctrl.Owner == nil {
		g.Board.Clear(pos)
		return
	}
	for r := -radius; r <= radius; r++ {
		for c := -radius; c <= radius; c++ {
			botPos := pos.Add(r, c)
			if botPos.R < 0 || botPos.R >= util.Rows {
				continue
			}
			b := g.Bots[idx(botPos)]
			if b == nil {
				continue
			}
			if ctrl.Amount <= 0 && b.Inventory.Amount > 0 {
				ctrl.Amount++
				b.Inventory.Amount--
			}
			if ctrl.Amount <= 0 {
				if b.Colony == ctrl.Colony {
					// b.Hp += g.calcHpChange()
					// b.Hp += 1
				} else {
					b.Hp -= 0
				}
				continue
			}
			if b.Colony == ctrl.Colony {
				b.Hp += 25
				ctrl.Amount--
			} else {
				b.Hp -= 5
			}
		}
	}
}

func (g *Game) Run() {
	g.initialBotsGeneration()
	g.generateWater()
	g.populateBoard()
	ui.BuildStaticLayer(g.Board)

	for !ui.Window.ShouldClose() {
		if g.config.Pause {
			ui.DrawGrid(g.Board, g.Bots)
			continue
		}
		g.step()
		ui.DrawGrid(g.Board, g.Bots)
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
	var maxLogicPerFrame int
	if g.config.LiveBots < 2000 {
		maxLogicPerFrame = 64
	} else if g.config.LiveBots < 5000 {
		maxLogicPerFrame = 32
	} else if g.config.LiveBots < 25000 {
		maxLogicPerFrame = 16
	} else {
		maxLogicPerFrame = 8
	}

	for executed := 0; executed < maxLogicPerFrame &&
		time.Since(g.State.LastLogic) >= g.config.LogicStep; executed++ {

		g.config.LiveBots = g.liveBotCount()
		switch n := g.config.LiveBots; {
		case n == 0, n <= g.config.NewGenThreshold:
			g.initialBotsGeneration()
			g.populateBoard()
		default:
			g.botsActions()
			g.environmentActions()
		}

		g.State.LastLogic = g.State.LastLogic.Add(g.config.LogicStep)
	}
}

func (g *Game) printDebugInfo() {
	fmt.Printf("\nGeneration: %d; Max HP: %d;", g.currGen, g.maxHp)
	fmt.Printf(" Latest improvement: %d;", g.latestImprovement)
	fmt.Printf("\nBots amount: %d", g.liveBotCount())
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
			if _, ok := oldBoard.At(pos).(board.Organics); ok {
				g.Board.Clear(pos)
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
			if b := g.Bots[idx(pos)]; b != nil {
				g.Board.Set(pos, b)
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
			if rand.Intn(100) < g.config.ResourceChance {
				g.Board.Set(pos, board.Resource{Pos: pos, Amount: 1})
				continue
			}
			// if util.RollChance(g.config.ResourceChance) {
			// 	g.Board.Set(pos, board.Resource{Pos: pos, Amount: 1})
			// 	continue
			// }
		}
	}
}

var newGenerations = 0

func (g *Game) initialBotsGeneration() {
	fmt.Printf("initialBotsGeneration, %v\n", newGenerations)
	newGenerations += 1
	for r := range board.Rows {
		for c := range board.Cols {
			pos := board.Position{C: c, R: r}
			if !g.Board.IsEmpty(pos) || !util.RollChance(g.config.BotChance) {
				continue
			}
			b := bot.NewBot()
			if g.InitialGenome != nil {
				b.Genome = *g.InitialGenome
			}
			g.Bots[idx(pos)] = &b
			g.Board.Set(pos, &b)
		}
	}
}

func (g *Game) botsActions() {
	for i, b := range g.Bots {
		if b == nil {
			continue
		}
		pos := util.PosOf(i)
		// b.Hp -= g.calcHpChange()
		b.Hp -= 1
		b.Hp = min(b.Hp, 500)
		if b.Hp <= 0 {
			g.killBot(b, i)
			if rand.Intn(100) < 33 {
				g.Board.Set(pos, board.Organics{Pos: pos, Amount: g.config.OrganicInitialAmount})
			} else {
				g.Board.Clear(pos)
			}
			continue
		}
		g.botAction(pos, b)
	}
}

func (g *Game) calcHpChange() int {
	var hpChange int
	if g.config.LiveBots > 20000 {
		hpChange = 3
	} else if g.config.LiveBots > 15000 {
		hpChange = 2
	} else if g.config.LiveBots > 10000 {
		hpChange = 2
	} else if g.config.LiveBots > 5000 {
		hpChange = 1
	} else if g.config.LiveBots > 3000 {
		hpChange = 1
	} else if g.config.LiveBots > 1000 {
		hpChange = 1
	} else {
		hpChange = 1
	}
	return hpChange
}

func (g *Game) botAction(pos board.Position, b *bot.Bot) {
	for range 5 {
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
			b.Hp -= g.config.DivisionCost
			g.Bots[idx(newPos)] = child
			g.Board.Set(newPos, child)
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
				// fmt.Printf("I attack %v. Hp: %d; other.Hp: %d\n", attackPos, b.Hp, other.Hp)
				b.Hp -= other.Hp
				if other.Inventory.Amount > 0 {
					b.Inventory.Amount += other.Inventory.Amount
				}
				g.Board.Clear(attackPos)
				g.killBot(other, idx(attackPos))
			}
			b.PointerJumpBy(2)
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
				if other.FromSameColony(b) {
					b.PointerJumpBy(1)
					return
				}
				// fmt.Printf("I eat. Hp: %d; other.Hp: %d\n. Is bro? %v\n", b.Hp, other.Hp, b.IsBro(other))
				b.Hp += other.Hp
				color := b.Color

				dc := g.config.ColorDelta
				b.Color = [3]float32{color[0] + dc, color[1] - dc, color[2] - dc}

				g.Board.Clear(eatPos)
				g.killBot(other, idx(eatPos))
			}
			b.PointerJumpBy(2)
			return

		case bot.OpPhoto:
			row := pos.R
			rows := util.Rows

			var photoChance int
			switch {
			case row < rows*1/10:
				photoChance = 100
			case row < rows*2/10:
				photoChance = 80
			case row < rows*3/10:
				photoChance = 50
			default:
				photoChance = 25
			}

			if util.RollChance(photoChance) {
				// b.Hp += g.config.PhotoHpGain
				b.Hp += 1
				dc := g.config.ColorDelta
				color := b.Color
				b.Color = [3]float32{color[0] - dc, color[1] + dc, color[2] - dc}
			}
			b.PointerJumpBy(1)
			return

		case bot.OpLook:
			g.lookAround(pos, b)
			continue

		case bot.OpCheckInventory:
			amt := b.Inventory.Amount
			b.Genome.NextArg = amt
			if amt > 70 {
				b.PointerJumpBy(1)
			} else {
				b.PointerJumpBy(2)
			}
			continue

		case bot.OpExecuteInstr:
			b.PointerJumpBy(b.CmdArg(1))
			continue

		case bot.OpHpToResource:
			// arg := b.CmdArg(1) % 4
			// hpChange := arg * 10
			// if b.Hp < hpChange {
			// 	b.PointerJumpBy(3)
			// 	return
			// }
			// b.Hp -= hpChange
			// b.Inventory.Amount += arg
			b.PointerJumpBy(2)
			return

		case bot.OpCheckHp:
			b.Genome.NextArg = b.Hp
			if b.Hp > 100 {
				b.PointerJumpBy(3)
			} else {
				b.PointerJumpBy(5)
			}
			continue

		case bot.OpEatOrganicsAbs:
			nextPos := b.CmdArgDir(1, pos)
			o, ok := g.Board.At(nextPos).(board.Organics)
			if !ok {
				b.PointerJumpBy(2)
				continue
			}
			// fmt.Printf("I eat organics abs. Hp after: %d; \n", b.Hp)
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
			// fmt.Printf("I eat organics . Hp after: %d; \n", b.Hp)
			b.Hp += o.Amount
			g.Board.Clear(nextPos)
			b.PointerJumpBy(1)
			return

		case bot.OpGrab:
			g.grab(pos, b)
			return

		case bot.OpShareHp:
			if b.Hp < 20 {
				b.PointerJumpBy(6)
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
			// fmt.Printf("I share %d HP with %v. Is bro? %v\n", shareAmount, sharePos, b.IsBro(other))
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
			// fmt.Printf("I share %d Inventory with %v. Is bro? %v\n", shareAmount, sharePos, b.IsBro(other))
			b.PointerJumpBy(2)
			return

		case bot.OpBuild:
			g.build(pos, b)
			g.Board.Set(pos, b)
			continue

		case bot.OpSetReg:
			reg := b.CmdArg(1) % 4
			b.Genome.Registers[reg] = b.Genome.NextArg
			b.PointerJumpBy(3)
			return

		case bot.OpIncReg:
			reg := b.CmdArg(1) % 4
			b.Genome.Registers[reg]++
			b.PointerJumpBy(2)
			continue

		case bot.OpDecReg:
			reg := b.CmdArg(1) % 4
			b.Genome.Registers[reg]--
			b.PointerJumpBy(2)
			continue

		case bot.OpCmpReg:
			regA := b.CmdArg(1) % 4
			regB := b.CmdArg(2) % 4
			aVal := b.Genome.Registers[regA]
			bVal := b.Genome.Registers[regB]
			if aVal == bVal {
				b.PointerJumpBy(3)
			} else if aVal < bVal {
				b.PointerJumpBy(4)
			} else {
				b.PointerJumpBy(5)
			}
			continue

		case bot.OpJumpIfZero:
			reg := b.CmdArg(1) % 4
			// fmt.Printf("I jump if reg %v is zero\n", reg)
			jump := b.CmdArg(2)
			if b.Genome.Registers[reg] == 0 {
				b.PointerJumpBy(3 + jump)
			} else {
				b.PointerJumpBy(3)
			}
			continue

		case bot.OpSendSignal:
			dir := b.CmdArgDir(1, pos)
			sendPos := pos.AddPos(dir)
			if util.RowInside(sendPos) {
				return
			}
			i := idx(sendPos)
			other := g.Bots[i]
			if other == nil {
				continue
			}
			commands := 4 // arbitrary value
			sigVal := b.CmdArg(2) % commands
			other.Genome.Signal = sigVal
			b.PointerJumpBy(2)
			continue

		case bot.OpCheckSignal:
			sigVal := b.Genome.Signal
			b.Genome.Signal = 0
			b.PointerJumpBy(sigVal + 1)
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
	g.Bots[idx(oldPos)] = nil
	g.Bots[idx(newPos)] = b

	g.Board.Clear(oldPos)
	g.Board.Set(newPos, b)

	g.Board.MarkDirty(util.Idx(newPos))
	g.Board.MarkDirty(util.Idx(oldPos))
	return newPos
}

func (g *Game) grab(pos board.Position, b *bot.Bot) {
	c := *g.config

	// dir := util.PosClock[b.CmdArg(1)%8]
	// TODO: decide. this is test one
	dir := b.Dir
	grabPos := pos.Add(dir[0], dir[1])
	// fmt.Printf("grabbing %v\n", grabPos)

	// if !g.Board.IsGrabable(grabPos) {
	// 	return
	// }

	switch v := g.Board.At(grabPos).(type) {
	case board.Building:
		if v.Owner == b {
			return
		}
		b.Inventory.Amount += c.BuildingGrabGain
		b.Hp += c.BuildingGrabHpGain
		g.Board.Set(grabPos, nil)
		b.PointerJumpBy(1)
		b.Genome.NextArg = 1
		return
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
		g.Bots[idx(spawnPos)] = child
		b.PointerJumpBy(2)
		b.Genome.NextArg = 2
		return
	case board.Farm:
		if v.Owner != b {
			b.Inventory.Amount = 0
			g.Board.Clear(grabPos)
			return
		}
		if b.Inventory.Amount <= 0 {
			return
		}
		b.Inventory.Amount -= c.FarmGrabCost
		v.Amount += 1
		g.Board.Set(grabPos, v)
		b.PointerJumpBy(3)
		b.Genome.NextArg = 3
		return
	case board.Food:
		b.Hp += c.FoodGrabHpGain
		g.Board.Clear(grabPos)
		return
	case board.Poison:
		b.Hp = 0
		g.Board.Clear(pos)
		g.Board.Clear(grabPos)
		b.PointerJumpBy(4)
		b.Genome.NextArg = 4
		return
	case board.Controller:
		if !v.Owner.FromSameColony(b) {
			// arg := b.CmdArg(1) % 2
			// if arg == 0 {
			// 	v.Colony = b.Colony
			// 	v.Owner = b
			// 	b.PointerJumpBy(9)
			// } else {
			g.Board.Set(grabPos, nil)
			v.Owner.Colony = nil
			b.PointerJumpBy(10)
			// }
			return
		}
		b.Inventory.Amount -= c.ControllerGrabCost
		b.Hp += c.ControllerHpGain
		v.Amount += 1
		g.Board.Set(grabPos, v)
		b.PointerJumpBy(5)
		b.Genome.NextArg = 5
		return
	case board.Resource:
		// if b.Inventory.Amount > 10 {
		// 	b.Hp -= 10
		// }
		b.Inventory.Amount += c.ResourceGrabGain
		b.Hp += c.ResourceGrabHpGain
		// Todo:  adjust
		v.Amount -= 10
		if v.Amount <= 0 {
			g.Board.Set(grabPos, nil)
		} else {
			g.Board.Set(grabPos, v)
		}
		b.PointerJumpBy(6)
		b.Genome.NextArg = 6
		return
	case board.Mine:
		// fmt.Println("MineMine")
		if b.Hp < c.MineGrabHpCost {
			return
		}
		b.Inventory.Amount += c.MineGrabGain
		b.Hp -= c.ResourceGrabHpGain
		b.PointerJumpBy(7)
		b.Genome.NextArg = 7
		return
	default:
		b.PointerJumpBy(8)
		b.Genome.NextArg = 8
		return
	}
}

func (g *Game) build(botPos board.Position, b *bot.Bot) {
	c := g.config
	dir := util.PosClock[b.CmdArg(1)%8]
	buildPos := botPos.Add(dir[0], dir[1])

	if !g.Board.IsEmpty(buildPos) {
		return
	}

	buildTypes := bot.BuildTypesCount()
	buildType := bot.BuildType(b.CmdArg(2) % buildTypes)

	switch buildType {
	case bot.BuildWall:
		// if b.Inventory.Amount < c.BuildingBuildCost {
		// 	return
		// }
		g.Board.Set(buildPos, board.Building{
			Pos:   buildPos,
			Owner: b,
			Hp:    20,
		})
		b.Inventory.Amount -= c.BuildingBuildCost
		b.Hp += c.BuildingBuildHpGain
		b.PointerJumpBy(1)
		b.Genome.NextArg = 1
		return
	case bot.BuildSpawner:
		return
		// if b.HasSpawner {
		// 	return
		// }
		// g.Board.Set(buildPos, board.Spawner{
		// 	Pos:    buildPos,
		// 	Owner:  b,
		// 	Amount: 10,
		// })
		// b.HasSpawner = true
		// b.PointerJumpBy(2)
	case bot.BuildController:
		if b.Colony != nil {
			return
		}

		colony := bot.NewColony(buildPos)
		colony.AddFamily(b)

		g.Board.Set(buildPos, board.Controller{
			Pos:    buildPos,
			Owner:  b,
			Colony: &colony,
			Amount: c.ControllerInitialAmount,
		})
		b.ReassignColor()
		b.Hp += c.ControllerHpGain
		b.PointerJumpBy(3)
		b.Genome.NextArg = 3
		return
	case bot.BuildMine:
		if b.Inventory.Amount < c.MineBuildCost {
			return
		}
		g.Board.Set(buildPos, board.Mine{
			Pos:    buildPos,
			Owner:  b,
			Amount: 0,
		})
		b.Inventory.Amount -= c.MineBuildCost
		b.PointerJumpBy(4)
		b.Genome.NextArg = 4
		return
	case bot.BuildFarm:
		if c.DisableFarms {
			return
		}
		if b.Inventory.Amount < c.FarmBuildCost {
			return
		}
		g.Board.Set(buildPos, board.Farm{
			Pos:    buildPos,
			Owner:  b,
			Amount: c.FarmInitialAmount,
		})
		b.Hp += c.FarmBuildHpGain
		b.Inventory.Amount -= c.FarmBuildCost
		b.PointerJumpBy(5)
		b.Genome.NextArg = 5
		return
	}
}

func (g *Game) lookAround(botPos board.Position, b *bot.Bot) {
	dir := util.PosClock[b.CmdArg(1)%8]
	lookPos := botPos.Add(dir[0], dir[1])
	// reg := b.CmdArg(2) % 4
	// b.Registers[reg] = 0
	switch g.Board.At(lookPos).(type) {
	case *bot.Bot:
		other := g.Bots[idx(lookPos)]
		if other != nil {
			b.PointerJumpBy(13)
			b.Genome.NextArg = 13
			return
		}
		if b.IsBro(other) {
			b.PointerJumpBy(20)
			b.Genome.NextArg = 20
			return
		}
	case board.Building:
		b.PointerJumpBy(4)
		b.Genome.NextArg = 4
	case board.Wall:
		b.PointerJumpBy(8)
		b.Genome.NextArg = 8
	case board.Resource:
		b.PointerJumpBy(11)
		b.Genome.NextArg = 11
	case board.Controller:
		b.PointerJumpBy(50)
		b.Genome.NextArg = 50
	case board.Spawner:
		b.PointerJumpBy(61)
		b.Genome.NextArg = 61
	case board.Farm:
		b.PointerJumpBy(7)
		b.Genome.NextArg = 7
	case board.Food:
		b.PointerJumpBy(9)
		b.Genome.NextArg = 9
	case board.Poison:
		b.PointerJumpBy(14)
		b.Genome.NextArg = 14
	case board.Water:
		b.PointerJumpBy(15)
		b.Genome.NextArg = 15
	case board.Organics:
		b.PointerJumpBy(3)
		b.Genome.NextArg = 3
	default:
		b.PointerJumpBy(12)
		b.Genome.NextArg = 12
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
