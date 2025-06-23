package main

import (
	"fmt"
	"golab/board"
	"golab/bot"
	"math/rand"
	"os"
	"runtime"
	"time"

	"github.com/go-gl/gl/v2.1/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/go-gl/gltext"
)

type Position = board.Position
type Board = board.Board

const (
	rows = 20
	cols = 40
)

var directionMap = map[board.Position]bot.Direction{
	{X: 0, Y: 1}:  bot.Up,
	{X: 1, Y: 0}:  bot.Right,
	{X: 0, Y: -1}: bot.Down,
	{X: -1, Y: 0}: bot.Left,
}

const logicStep = 300 * time.Millisecond

var lastLogic = time.Now()
var font *gltext.Font
var brd Board = *board.NewBoard()

// Camera
var (
	camX, camY float32
	camScale   float32 = 1.0

	dragging               bool
	dragStartX, dragStartY float64
	camStartX, camStartY   float32
	appWindow              *glfw.Window
)

func scrollCallback(_ *glfw.Window, _ float64, yoff float64) {
	f := 1 + float32(yoff)*0.1
	camScale *= f
	if camScale < 0.5 {
		camScale = 0.5
	}
	if camScale > 4.0 {
		camScale = 4.0
	}
}

func mouseButtonCallback(w *glfw.Window, button glfw.MouseButton, action glfw.Action, mods glfw.ModifierKey) {
	if button != glfw.MouseButtonLeft {
		return
	}
	switch action {
	case glfw.Press:
		dragging = true
		dragStartX, dragStartY = w.GetCursorPos()
		camStartX, camStartY = camX, camY
	case glfw.Release:
		dragging = false
	}
}

func cursorPosCallback(w *glfw.Window, xpos, ypos float64) {
	if !dragging {
		return
	}
	winW, winH := w.GetSize()
	dx := xpos - dragStartX
	dy := ypos - dragStartY
	camX = camStartX - float32(dx)*float32(cols)/float32(winW)/camScale
	camY = camStartY + float32(dy)*float32(rows)/float32(winH)/camScale // y axis is flipped
}

func applyCamera() {
	gl.MatrixMode(gl.MODELVIEW)
	gl.LoadIdentity()
	gl.Scalef(camScale, camScale, 1)
	gl.Translatef(-camX, -camY, 0)
}

var bots = map[Position]bot.Bot{}

func init() { runtime.LockOSThread() }

func main() {
	if err := glfw.Init(); err != nil {
		panic(err)
	}
	defer glfw.Terminate()

	mon := glfw.GetPrimaryMonitor()
	mode := mon.GetVideoMode()
	screenW, screenH := mode.Width, mode.Height
	glfw.WindowHint(glfw.ContextVersionMinor, 1)
	window, err := glfw.CreateWindow(screenW, screenH, "Bot Arena", nil, nil)
	appWindow = window
	window.SetScrollCallback(scrollCallback)
	window.SetMouseButtonCallback(mouseButtonCallback)
	window.SetCursorPosCallback(cursorPosCallback)

	if err != nil {
		panic(err)
	}
	window.MakeContextCurrent()
	glfw.SwapInterval(1)
	if err := gl.Init(); err != nil {
		panic(err)
	}

	gl.Enable(gl.TEXTURE_2D) // lets glyph quads use the atlas
	gl.Enable(gl.BLEND)      // allow alpha
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	font = loadFont("/usr/share/fonts/truetype/msttcorefonts/Arial.ttf")
	Assert(font != nil, "Font can't be nil")

	gl.MatrixMode(gl.PROJECTION)
	gl.LoadIdentity()
	gl.Ortho(0, float64(cols), 0, float64(rows), -1, 1)
	gl.MatrixMode(gl.MODELVIEW)
	gl.LoadIdentity()

	gl.ClearColor(0.1, 0.1, 0.1, 1.0)
	gameLoop(window)
}

func textAtWorld(wx, wy float32, s string) {
	winW, winH := appWindow.GetSize()

	cellPxX := float32(winW) / float32(cols) * camScale
	cellPxY := float32(winH) / float32(rows) * camScale

	px := (wx - camX) * cellPxX
	py := (wy - camY) * cellPxY

	px += cellPxX * 0.30 // horizontal offset in the cell
	py += cellPxY * 0.55 // vertical offset

	py = float32(winH) - py // <-- flip Y so 0 = top edge

	gl.MatrixMode(gl.PROJECTION)
	gl.PushMatrix()
	gl.LoadIdentity()
	gl.Ortho(0, float64(winW), 0, float64(winH), -1, 1)

	gl.MatrixMode(gl.MODELVIEW)
	gl.PushMatrix()
	gl.LoadIdentity()

	gl.Color3f(1, 1, 1)
	font.Printf(px, py, s)

	gl.PopMatrix() // MODELVIEW
	gl.MatrixMode(gl.PROJECTION)
	gl.PopMatrix()
	gl.MatrixMode(gl.MODELVIEW)
}

func loadFont(name string) *gltext.Font {
	f, err := os.Open(name)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	ft, err := gltext.LoadTruetype(f, 24, 32, 127, gltext.LeftToRight)
	if err != nil {
		panic(err)
	}
	return ft
}

func gameLoop(window *glfw.Window) {
	generateBots()
	generateFood()
	populateBoard()
	for !window.ShouldClose() {
		now := time.Now()

		for now.Sub(lastLogic) >= logicStep {
			botsActions()
			lastLogic = lastLogic.Add(logicStep)
		}

		gl.Clear(gl.COLOR_BUFFER_BIT)
		applyCamera()
		drawGrid()
		window.SwapBuffers()
		glfw.PollEvents()
	}
}

func generateFood() {
	for r := range rows {
		for c := range cols {
			pos := Position{X: c, Y: r}
			if !brd.IsEmpty(pos) || brd.IsWall(pos) || rand.Intn(100) > 10 {
				continue
			}
			brd.Set(pos, board.Food{Pos: pos})
		}
	}
}

func populateBoard() {
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
		}
	}
}

func generateBots() {
	for r := range rows {
		for c := range cols {
			pos := Position{X: c, Y: r}
			if brd.IsWall(pos) || rand.Intn(100) > 1 {
				continue
			}
			b := bot.NewBot("bot")
			brd.Set(pos, b)
			bots[pos] = b
		}
	}
}

func tryMove(dst map[Position]bot.Bot, oldPos Position, b bot.Bot) Position {
	b.Dir = bot.RandomDir()
	newPos := Position{X: oldPos.X + b.Dir[0], Y: oldPos.Y + b.Dir[1]}

	blocked := brd.IsWall(newPos) ||
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
			delete(bots, startPos)
			continue
		}
		botAction(startPos, b, newBots)
	}

	bots = newBots
}

func botAction(startPos Position, b bot.Bot, newBots map[Position]bot.Bot) {
	cmd := 9
	// cmd := rand.Intn(25)
	cmds := 1
	curPos := startPos

	for cmds > 0 {
		switch {
		case cmd < 8:
			curPos = tryMove(newBots, curPos, b)
		// case cmd < 16:
		// 	curPos = lookAround(newBots, curPos, b)
		case cmd < 24:
			grab(newBots, curPos, b)
			// case cmd < 64:
			// 	curPos = other()
		}
		cmds--
	}
}

func grab(newBots map[Position]bot.Bot, pos Position, b bot.Bot) {
	positions := [8][2]int{
		// x, y clockwise
		{0, 1}, {1, 1}, {1, 0}, {1, -1},
		{0, -1}, {-1, -1}, {-1, 0}, {-1, 1},
	}
	for _, d := range positions {
		dx, dy := d[0], d[1]
		grabPos := board.NewPosition(pos.Y+dy, pos.X+dx)
		if brd.IsFood(grabPos) {
			fmt.Printf("Bot position: %v; ", pos)
			fmt.Printf("Food position: %v; ", grabPos)
			fmt.Printf("dx: %d; ", dx)
			fmt.Printf("dy: %d; ", dy)
			fmt.Println("isFood")
			b.Hp += 10
			brd.Set(grabPos, nil)
		}
	}
	newBots[pos] = b
}

func lookAround(newBots map[Position]bot.Bot, curPos Position, b bot.Bot) Position {
	panic("unimplemented")
}

func drawGrid() {
	for r := range rows {
		for c := range cols {
			x := float32(c)
			y := float32(r)
			pos := Position{X: c, Y: r}

			if brd.IsFood(pos) {
				drawCell(x, y, 0.2, 0.7, 0.2, 1, 1)
				continue
			}

			if brd.IsWall(pos) {
				drawCell(x, y, 0.7, 0.7, 0.7, 1, 1)
				continue
			}

			if !brd.IsEmpty(pos) {
				switch v := brd.At(pos).(type) {
				case bot.Bot:
					drawBot(x, y, 0.3, 0.3, 1.0, 1, 1, v.Dir, v.Hp)
				case board.Food:
					fmt.Printf("RenFood")
					drawFood(x, y, 0.8, 0.8, 0.2, 0.5, 0.5)
				}
			}

			if b, ok := bots[pos]; ok {
				drawBot(x, y, 0.3, 0.3, 1.0, 1, 1, b.Dir, b.Hp)
				continue
			}

			// empty space
			drawCell(x, y, 0.2, 0.2, 0.2, 1, 1)
		}
	}
}

func drawBot(x, y, r, g, b, w, h float32, dir bot.Direction, hp int) {
	gl.MatrixMode(gl.MODELVIEW)
	gl.PushMatrix()

	// position + rotate around cell center
	gl.Translatef(x+w/2, y+h/2, 0)
	switch dir {
	case bot.Right:
		gl.Rotatef(270, 0, 0, 1)
	case bot.Left:
		gl.Rotatef(90, 0, 0, 1)
	case bot.Down:
		gl.Rotatef(180, 0, 0, 1)
	}
	gl.Translatef(-w/2, -h/2, 0)

	// BODY in local 0..1 coords
	gl.Color3f(r, g, b)
	gl.Begin(gl.QUADS)
	gl.Vertex2f(0, 0)
	gl.Vertex2f(w, 0)
	gl.Vertex2f(w, h)
	gl.Vertex2f(0, h)
	gl.End()

	// EYES in local 0..1 coords
	eyeW := w * 0.2
	eyeH := h * 0.2
	eyeY := h * 0.6

	gl.Color3f(0, 0, 0)
	gl.Begin(gl.QUADS)
	// left eye
	gl.Vertex2f(w*0.2, eyeY)
	gl.Vertex2f(w*0.2+eyeW, eyeY)
	gl.Vertex2f(w*0.2+eyeW, eyeY+eyeH)
	gl.Vertex2f(w*0.2, eyeY+eyeH)
	// right eye
	gl.Vertex2f(w*0.6, eyeY)
	gl.Vertex2f(w*0.6+eyeW, eyeY)
	gl.Vertex2f(w*0.6+eyeW, eyeY+eyeH)
	gl.Vertex2f(w*0.6, eyeY+eyeH)
	gl.End()

	gl.PopMatrix()

	textAtWorld(x+0.05, y+0.05, fmt.Sprintf("%d", hp))
}

func drawFood(x, y, r, g, b, w, h float32) {
	w *= 0.5
	h *= 0.5
	ox := w * 0.5
	oy := h * 0.5

	gl.Begin(gl.QUADS)
	gl.Color3f(r, g, b)
	gl.Vertex2f(x+ox, y+oy)
	gl.Vertex2f(x+ox+w, y+oy)
	gl.Vertex2f(x+ox+w, y+oy+h)
	gl.Vertex2f(x+ox, y+oy+h)
	gl.End()
}

func drawCell(x, y, r, g, b, w, h float32) {
	gl.Begin(gl.QUADS)
	gl.Color3f(r, g, b)
	gl.Vertex2f(x, y)
	gl.Vertex2f(x+w, y)
	gl.Vertex2f(x+w, y+h)
	gl.Vertex2f(x, y+h)
	gl.End()
}

func Assert(cond bool, msg string) {
	if !cond {
		panic(msg)
	}
}
