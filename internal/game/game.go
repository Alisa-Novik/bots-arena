package game

import (
	"fmt"
	"golab/internal/config"
	conf "golab/internal/config"
	"golab/internal/core"
	"golab/internal/tasking"
	"golab/internal/ui"
	"golab/internal/util"
	"math/rand"
	"time"
)

type Game struct {
	Board         *core.Board
	Colonies      []*core.Colony
	InitialGenome *core.Genome

	config *conf.Config
	State  *conf.GameState

	// aux
	maxHp             int
	currGen           int
	latestImprovement int
}

func NewGame(config *conf.Config) *Game {
	useInitialGenome := config.UseInitialGenome
	return &Game{
		Board:         core.NewBoard(),
		InitialGenome: core.GetInitialGenome(useInitialGenome),
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
	for r := range core.Rows {
		for c := range core.Cols {
			pos := core.Position{C: c, R: r}
			switch v := g.Board.At(pos).(type) {
			case core.Organics:
				if v.Amount <= 1 {
					g.Board.Clear(pos)
					continue
				}
				v.Amount--
				g.Board.Set(pos, v)
				continue
			case core.Controller:
				g.handleController(&v, pos)
				g.Board.Set(pos, v)
				continue
			case core.Farm:
				if v.Amount <= 0 {
					continue
				}
				foodPos, ok := g.Board.FindEmptyPosAround(pos)
				if !ok {
					continue
				}
				g.Board.Set(foodPos, core.Food{Pos: foodPos, Amount: 1})
				v.Amount--
				g.Board.Set(pos, v)
				continue
			}
		}
	}
}

func (g *Game) killBot(b *core.Bot, botIdx int) {
	if b.Path != nil {
		b.Path = nil
	}
	if b.CurrTask != nil {
		b.CurrTask.Owner = nil
	}
	if c := b.Colony; c != nil {
		c.RemoveMember(b)
	}
	if p := b.Parent; p != nil {
		p.RemoveOffspring(b)
	}
	g.Board.Bots[botIdx] = nil
	g.Board.MarkDirty(botIdx)
	*b = core.Bot{}
	core.BotPool.Put(b)
}

func (g *Game) handleController(ctrl *core.Controller, pos util.Position) {
	c := ctrl.Colony

	if ctrl.Owner == nil {
		for _, m := range c.Members {
			m.DisconnectFromColony()
		}
		for _, f := range c.Flags {
			g.Board.Clear(f.Pos)
		}
		g.Board.Clear(ctrl.Pos)
		return
	}

	for _, d := range core.Dirs {
		g.connectBots(pos.AddDir(d), map[*core.Bot]struct{}{}, ctrl.Colony)
	}

	for _, m := range c.Members {
		c.HealMember(m, ctrl)

		if task := m.CurrTask; task != nil {
			m.Hp += 1
			if !task.IsDone {
				m.SetColor(util.CyanColor(), g.Board.MarkDirty)
			} else {
				m.SetColor(util.GreenColor(), g.Board.MarkDirty)
			}
		}
	}

	c.HealBotsInFlagRadius(5, g.calcHpChange())

	if len(c.WaterPositions) > 0 && !c.HasTaskOfType(core.ConnectToPosTask) {
		task := c.NewConnectionTask(c.WaterPositions[0])
		// task := c.NewConnectionTask(util.NewPos(1, 1))
		c.AddTask(task)
	}

	tasking.ProcessColonyTasks(ctrl, g.Board)
}

func (g *Game) connectBots(currPos util.Position, visited map[*core.Bot]struct{}, colony *core.Colony) bool {
	if util.OutOfBounds(currPos) {
		return false
	}
	b := g.Board.GetBot(currPos)
	if b == nil || b.Colony != colony {
		return false
	}
	if _, yes := visited[b]; yes {
		return true
	}

	b.ConnnectedToColony = true
	if g.config.ColoringStrategy == config.ColonyConnectionColoring {
		b.Color = util.RedColor()
	}
	visited[b] = struct{}{}

	isOnBorder := false
	for _, d := range core.Dirs {
		if !g.connectBots(currPos.AddDir(d), visited, colony) {
			isOnBorder = true
		}
	}
	if b.CurrTask == nil {
		if isOnBorder {
			b.Color = util.YellowColor()
		} else {
			b.Color = b.Colony.Color
		}
	}
	return true
}

func (g *Game) Run() {
	fmt.Println("Running simulation...")
	g.initialBotsGeneration()
	g.generateWater()
	g.populateBoard()

	ui.BuildStaticLayer(g.Board)
	ui.GameCallbacks.PrintPathToTask = g.Board.GetBot

	for !ui.Window.ShouldClose() {
		if g.config.Pause {
			ui.DrawGrid(g.Board, g.Board.Bots)
			continue
		}
		g.step()
		ui.DrawGrid(g.Board, g.Board.Bots)
	}
}

func (g *Game) generateWater() {
	waterSpread := g.config.OceansCount * util.ScaleFactor
	for i := range waterSpread {
		center := core.NewRandomPosition()
		radius := 1 + rand.Intn(4)
		for dr := -radius; dr <= radius; dr++ {
			for dc := -radius; dc <= radius+15; dc++ {
				if rand.Float64() > 0.6 {
					continue
				}
				pos := center.AddRowCol(dc, dr)
				if !g.Board.IsWall(pos) {
					g.Board.Set(pos, core.Water{GroupId: i, Amount: 10000})
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
	} else if g.config.LiveBots < 55000 {
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

func (g *Game) populateBoard() {
	oldBoard := g.Board
	g.Board = core.NewBoard()
	for r := range core.Rows {
		for c := range core.Cols {
			pos := core.Position{C: c, R: r}
			if g.Board.IsWall(pos) {
				g.Board.Set(pos, core.Wall{Pos: pos})
				continue
			}
			if spawner, ok := oldBoard.At(pos).(core.Spawner); ok {
				g.Board.Set(pos, spawner)
				continue
			}
			if _, ok := oldBoard.At(pos).(core.Organics); ok {
				g.Board.Clear(pos)
				continue
			}
			if f, ok := oldBoard.At(pos).(core.Farm); ok {
				g.Board.Set(pos, f)
				continue
			}
			if fd, ok := oldBoard.At(pos).(core.Food); ok {
				g.Board.Set(pos, fd)
				continue
			}
			if bld, ok := oldBoard.At(pos).(core.Building); ok {
				g.Board.Set(pos, bld)
				continue
			}
			if c, ok := oldBoard.At(pos).(core.Water); ok {
				g.Board.Set(pos, c)
				continue
			}
			if c, ok := oldBoard.At(pos).(core.Controller); ok {
				g.Board.Set(pos, c)
				continue
			}
			if b := g.Board.Bots[idx(pos)]; b != nil {
				g.Board.Set(pos, b)
				continue
			}
			if util.RollChanceOf(1000, g.config.PoisonChance) {
				g.Board.Set(pos, core.Poison{Pos: pos})
				continue
			}
			if rand.Intn(100) < g.config.ResourceChance {
				g.Board.Set(pos, core.Resource{Pos: pos, Amount: 1})
				continue
			}
		}
	}
}

func (g *Game) initialBotsGeneration() {
	for r := range core.Rows {
		for c := range core.Cols {
			pos := core.Position{C: c, R: r}
			if !g.Board.IsEmpty(pos) || !util.RollChance(g.config.BotChance) {
				continue
			}
			b := core.NewBot(pos)
			if g.InitialGenome != nil {
				b.Genome = *g.InitialGenome
			}
			g.Board.Bots[idx(pos)] = &b
			g.Board.Set(pos, &b)
		}
	}
}

func (g *Game) botsActions() {
	for i, b := range g.Board.Bots {
		if b == nil {
			continue
		}
		pos := util.PosOf(i)
		b.Hp -= g.calcHpChange()
		// b.Hp -= 1
		b.Hp = min(b.Hp, 500)
		if b.Hp <= 0 {
			g.killBot(b, i)
			if rand.Intn(100) < 33 {
				g.Board.Set(pos, core.Organics{Pos: pos, Amount: g.config.OrganicInitialAmount})
			} else {
				g.Board.Clear(pos)
			}
			continue
		}
		if b.CurrTask != nil && b.CurrTask.Type == core.MaintainConnectionTask && b.CurrTask.IsDone {
			continue
		}
		g.botAction(pos, b)
	}
}

func (g *Game) calcHpChange() int {
	var hpChange int
	if g.config.LiveBots > 20000 {
		hpChange = 15
	} else if g.config.LiveBots > 15000 {
		hpChange = 13
	} else if g.config.LiveBots > 10000 {
		hpChange = 13
	} else if g.config.LiveBots > 5000 {
		hpChange = 13
	} else if g.config.LiveBots > 3000 {
		hpChange = 2
	} else if g.config.LiveBots > 1000 {
		hpChange = 1
	} else {
		hpChange = 1
	}
	return hpChange
}

func (g *Game) botAction(pos core.Position, b *core.Bot) {
	for range 5 {
		op := core.Opcode(b.Genome.Matrix[b.Genome.Pointer])
		if b.MaintainingConn() {
			op = core.OpMove
		}
		// fmt.Printf("opcode: %v\n", op)
		switch op {
		case core.OpDivide:
			divMin := g.config.DivisionMinHp
			if b.Colony != nil {
				divMin = 80
			}
			if b.Hp < divMin {
				b.PointerJumpBy(5)
				return
			}

			newPos, ok := g.Board.FindEmptyPosAround(pos)
			if !ok {
				b.PointerJumpBy(4)
				continue
			}
			child := b.NewChild(newPos, g.config.ShouldMutateColor)
			b.Hp -= g.config.DivisionCost
			g.Board.Bots[idx(newPos)] = child
			g.Board.Set(newPos, child)
			b.PointerJumpBy(6)
			return

		case core.OpCheckConnection:
			if b.ConnnectedToColony {
				b.PointerJumpBy(1)
				b.Genome.NextArg = 1
			} else {
				b.PointerJumpBy(2)
				b.Genome.NextArg = 2
			}
			continue

		case core.OpMoveAbs:
			b.Dir = util.PosClock[b.CmdArg(1)%8]
			g.tryMove(pos, b)
			b.PointerJumpBy(1)
			return

		case core.OpCheckIfBro:
			checkPos := b.CmdArgDir(1, pos)
			other, ok := g.Board.At(checkPos).(*core.Bot)
			if !ok {
				b.PointerJumpBy(1)
				continue
			}
			if b.IsBro(other) || (other.ConnnectedToColony && b.SameColony(other)) {
				b.PointerJumpBy(2)
				continue
			}
			b.PointerJumpBy(3)
			continue

		case core.OpCheckColony:
			checkPos := b.CmdArgDir(1, pos)
			other, ok := g.Board.At(checkPos).(*core.Bot)
			if !ok {
				b.PointerJumpBy(1)
				continue
			}
			if b.SameColony(other) {
				b.PointerJumpBy(2)
				continue
			}
			b.PointerJumpBy(3)
			continue

		case core.OpAttack:
			attackPos := b.CmdArgDir(1, pos)
			other, ok := g.Board.At(attackPos).(*core.Bot)
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

		case core.OpMove:
			g.tryMove(pos, b)
			b.PointerJumpBy(1)
			return

		case core.OpTurn:
			b.Dir = util.PosClock[b.CmdArg(1)%8]
			b.PointerJumpBy(1)
			continue

		case core.OpEatOther:
			arg := b.CmdArg(1)
			dir := util.PosClock[arg%8]
			eatPos := pos.AddRowCol(dir[0], dir[1])
			other, ok := g.Board.At(eatPos).(*core.Bot)

			if ok && b.Hp >= other.Hp {
				if other.SameColony(b) {
					b.PointerJumpBy(1)
					return
				}
				b.Hp += other.Hp
				if g.config.EnableResourceBasedColorChange {
					color := b.Color
					dc := g.config.ColorDelta
					b.Color = [3]float32{color[0] + dc, color[1] - dc, color[2] - dc}
				}
				g.Board.Clear(eatPos)
				g.killBot(other, idx(eatPos))
			}
			b.PointerJumpBy(2)
			return

		case core.OpPhoto:
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
				if g.config.EnableResourceBasedColorChange {
					dc := g.config.ColorDelta
					color := b.Color
					b.Color = [3]float32{color[0] - dc, color[1] + dc, color[2] - dc}
				}
			}
			b.PointerJumpBy(1)
			return

		case core.OpLook:
			g.lookAround(pos, b)
			continue

		case core.OpCheckInventory:
			amt := b.Inventory.Amount
			b.Genome.NextArg = amt
			if amt > 70 {
				b.PointerJumpBy(1)
			} else {
				b.PointerJumpBy(2)
			}
			continue

		case core.OpExecuteInstr:
			b.PointerJumpBy(b.CmdArg(1))
			continue

		case core.OpHpToResource:
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

		case core.OpCheckHp:
			b.Genome.NextArg = b.Hp
			if b.Hp > 100 {
				b.PointerJumpBy(3)
			} else {
				b.PointerJumpBy(5)
			}
			continue

		case core.OpEatOrganicsAbs:
			nextPos := b.CmdArgDir(1, pos)
			o, ok := g.Board.At(nextPos).(core.Organics)
			if !ok {
				b.PointerJumpBy(2)
				continue
			}
			// fmt.Printf("I eat organics abs. Hp after: %d; \n", b.Hp)
			b.Hp += o.Amount
			g.Board.Clear(nextPos)
			b.PointerJumpBy(1)
			return

		case core.OpEatOrganics:
			nextPos := pos.AddRowCol(b.Dir[0], b.Dir[1])
			o, ok := g.Board.At(nextPos).(core.Organics)
			if !ok {
				b.PointerJumpBy(2)
				continue
			}
			// fmt.Printf("I eat organics . Hp after: %d; \n", b.Hp)
			b.Hp += o.Amount
			g.Board.Clear(nextPos)
			b.PointerJumpBy(1)
			return

		case core.OpGrab:
			g.grab(pos, b)
			return

		case core.OpShareHp:
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
			other, ok := g.Board.At(sharePos).(*core.Bot)
			if !ok {
				b.PointerJumpBy(4)
				continue
			}
			other.Hp += shareAmount
			b.Hp -= shareAmount
			// fmt.Printf("I share %d HP with %v. Is bro? %v\n", shareAmount, sharePos, b.IsBro(other))
			b.PointerJumpBy(3)
			return

		case core.OpShareInventory:
			if b.Inventory.Amount < 5 {
				b.PointerJumpBy(5)
				continue
			}
			sharePos := b.CmdArgDir(1, pos)
			shareAmount := b.CmdArg(2)
			other, ok := g.Board.At(sharePos).(*core.Bot)
			if !ok {
				b.PointerJumpBy(3)
				continue
			}
			other.Inventory.Amount += shareAmount
			b.Inventory.Amount -= shareAmount
			// fmt.Printf("I share %d Inventory with %v. Is bro? %v\n", shareAmount, sharePos, b.IsBro(other))
			b.PointerJumpBy(2)
			return

		case core.OpBuild:
			g.build(pos, b)
			g.Board.Set(pos, b)
			continue

		case core.OpSetReg:
			reg := b.CmdArg(1) % 4
			b.Genome.Registers[reg] = b.Genome.NextArg
			b.PointerJumpBy(3)
			return

		case core.OpIncReg:
			reg := b.CmdArg(1) % 4
			b.Genome.Registers[reg]++
			b.PointerJumpBy(2)
			continue

		case core.OpDecReg:
			reg := b.CmdArg(1) % 4
			b.Genome.Registers[reg]--
			b.PointerJumpBy(2)
			continue

		case core.OpCmpReg:
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

		case core.OpJumpIfZero:
			reg := b.CmdArg(1) % 4
			// fmt.Printf("I jump if reg %v is zero\n", reg)
			jump := b.CmdArg(2)
			if b.Genome.Registers[reg] == 0 {
				b.PointerJumpBy(3 + jump)
			} else {
				b.PointerJumpBy(3)
			}
			continue

		case core.OpSendSignal:
			dir := b.CmdArgDir(1, pos)
			sendPos := pos.AddPos(dir)
			if util.OutOfBounds(sendPos) {
				return
			}
			i := idx(sendPos)
			other := g.Board.Bots[i]
			if other == nil {
				continue
			}
			commands := 4 // arbitrary value
			sigVal := b.CmdArg(2) % commands
			other.Genome.Signal = sigVal
			b.PointerJumpBy(2)
			continue

		case core.OpCheckSignal:
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

func (g *Game) tryMove(oldPos core.Position, b *core.Bot) {
	g.Board.MarkDirty(idx(oldPos))
	newPos := oldPos.AddDir(b.Dir)

	if b.HasTask() {
		// fmt.Printf("Pos %v, Target %v, Path %v\n", b.Pos, b.CurrTask.Pos, b.Path)
		if b.Pos == b.CurrTask.Pos {
			b.CurrTask.MarkDone()
			return
		}
		if g.Board.IsEmpty(b.PeekNextPos()) {
			// fmt.Printf("Popping. Pos %v, Target %v, Path %v\n", b.Pos, b.CurrTask.Pos, b.Path)
			newPos = b.PopNextPos()
		}
	}

	if !g.Board.IsEmpty(newPos) {
		return
	}

	b.Pos = newPos

	g.Board.Bots[idx(oldPos)] = nil
	g.Board.Bots[idx(newPos)] = b

	g.Board.Set(newPos, b)
	g.Board.Clear(oldPos)
}

var i = 0

func (g *Game) grab(pos core.Position, b *core.Bot) {
	c := *g.config

	// dir := util.PosClock[b.CmdArg(1)%8]
	// TODO: decide. this is test one
	dir := b.Dir
	grabPos := pos.AddRowCol(dir[0], dir[1])
	// fmt.Printf("grabbing %v\n", grabPos)

	// if !g.Board.IsGrabable(grabPos) {
	// 	return
	// }

	switch v := g.Board.At(grabPos).(type) {
	case core.Building:
		if v.Owner == b {
			return
		}
		b.Inventory.Amount += c.BuildingGrabGain
		b.Hp += c.BuildingGrabHpGain
		g.Board.Set(grabPos, nil)
		b.PointerJumpBy(1)
		b.Genome.NextArg = 1
		return
	case core.Spawner:
		if b.Inventory.Amount < c.SpawnerGrabCost {
			return
		}
		b.Inventory.Amount -= c.SpawnerGrabCost
		spawnPos, found := g.Board.FindEmptyPosAround(pos)
		if !found {
			return
		}
		child := b.NewChild(spawnPos, g.config.ShouldMutateColor)
		g.Board.Set(spawnPos, &child)
		g.Board.Bots[idx(spawnPos)] = child
		b.PointerJumpBy(2)
		b.Genome.NextArg = 2
		return
	case core.Farm:
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
	case core.Food:
		b.Hp += c.FoodGrabHpGain
		g.Board.Clear(grabPos)
		return
	case core.Poison:
		b.Hp = 0
		g.Board.Clear(pos)
		g.Board.Clear(grabPos)
		b.PointerJumpBy(4)
		b.Genome.NextArg = 4
		return
	case core.Controller:
		if !v.Owner.SameColony(b) {
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
	case core.Resource:
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
	case core.Mine:
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

func (g *Game) build(botPos core.Position, b *core.Bot) {
	c := g.config
	dir := util.PosClock[b.CmdArg(1)%8]
	buildPos := botPos.AddRowCol(dir[0], dir[1])

	if !g.Board.IsEmpty(buildPos) {
		return
	}

	buildTypes := core.BuildTypesCount()
	buildType := core.BuildType(b.CmdArg(2) % buildTypes)

	switch buildType {
	case core.BuildWall:
		// if b.Inventory.Amount < c.BuildingBuildCost {
		// 	return
		// }
		g.Board.Set(buildPos, core.Building{
			Pos:   buildPos,
			Owner: b,
			Hp:    20,
		})
		b.Inventory.Amount -= c.BuildingBuildCost
		b.Hp += c.BuildingBuildHpGain
		b.PointerJumpBy(1)
		b.Genome.NextArg = 1
		return
	case core.BuildColonyFlag:
		if b.Colony == nil || !b.ConnnectedToColony || b.Colony.FlagsCount() >= 3 {
			b.PointerJumpBy(2)
			return
		}
		flag := core.ColonyFlag{
			Pos: buildPos,
		}
		g.Board.Set(buildPos, flag)
		b.Colony.AddFlag(&flag)
		b.PointerJumpBy(1)
		return
	case core.BuildSpawner:
		b.PointerJumpBy(2)
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
	case core.BuildController:
		if b.Colony != nil {
			b.PointerJumpBy(6)
			return
		}

		cln := core.NewColony(buildPos)
		cln.AddFamily(b)

		g.Board.Set(buildPos, core.Controller{
			Pos:         buildPos,
			Owner:       b,
			Colony:      &cln,
			Amount:      c.ControllerInitialAmount,
			WaterAmount: 0,
		})
		b.AssignRandomColor()
		b.Hp += c.ControllerHpGain
		b.PointerJumpBy(3)
		b.Genome.NextArg = 3
		return
	case core.BuildMine:
		if b.Inventory.Amount < c.MineBuildCost {
			b.PointerJumpBy(7)
			return
		}
		g.Board.Set(buildPos, core.Mine{
			Pos:    buildPos,
			Owner:  b,
			Amount: 0,
		})
		b.Inventory.Amount -= c.MineBuildCost
		b.PointerJumpBy(4)
		b.Genome.NextArg = 4
		return
	case core.BuildFarm:
		if c.DisableFarms {
			b.PointerJumpBy(9)
			return
		}
		if b.Inventory.Amount < c.FarmBuildCost {
			b.PointerJumpBy(8)
			return
		}
		g.Board.Set(buildPos, core.Farm{
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

func (g *Game) lookAround(botPos core.Position, b *core.Bot) {
	lookPos := b.CmdArgDir(2, botPos)
	switch v := g.Board.At(lookPos).(type) {
	case *core.Bot:
		other := g.Board.Bots[idx(lookPos)]
		if other != nil {
			if b.IsBro(other) {
				b.PointerJumpBy(20)
				b.Genome.NextArg = 20
				return
			}
			b.PointerJumpBy(13)
			b.Genome.NextArg = 13
			return
		}
	case core.Building:
		b.PointerJumpBy(4)
		b.Genome.NextArg = 4
	case core.Wall:
		b.PointerJumpBy(8)
		b.Genome.NextArg = 8
	case core.Resource:
		b.PointerJumpBy(11)
		b.Genome.NextArg = 11
	case core.Controller:
		b.PointerJumpBy(50)
		b.Genome.NextArg = 50
	case core.Spawner:
		b.PointerJumpBy(61)
		b.Genome.NextArg = 61
	case core.Farm:
		b.PointerJumpBy(7)
		b.Genome.NextArg = 7
	case core.Food:
		b.PointerJumpBy(9)
		b.Genome.NextArg = 9
	case core.Poison:
		b.PointerJumpBy(14)
		b.Genome.NextArg = 14
	case core.Water:
		if b.Colony != nil && !b.Colony.KnowsWaterGroupId(v.GroupId) {
			b.Colony.AddWaterPosition(lookPos, v.GroupId)
		}
		// if b.Colony != nil {
		// 	marker := b.Colony.NewMarker(lookPos, bot.WaterMarker)
		// 	b.Colony.AddMarker(marker)
		// }
		b.Genome.NextArg = 15
		b.PointerJumpBy(15)
	case core.Organics:
		b.PointerJumpBy(3)
		b.Genome.NextArg = 3
	default:
		b.PointerJumpBy(12)
		b.Genome.NextArg = 12
	}
}

func (g *Game) liveBotCount() int {
	n := 0
	for _, b := range g.Board.Bots {
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
