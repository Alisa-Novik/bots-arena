package main

import (
	"golab/bot"
	"math/rand"
	"runtime"
	"time"

	"github.com/go-gl/gl/v2.1/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
)

const (
	rows = 20
	cols = 40
)

var directionMap = map[Position]bot.Direction{
	{0, 1}:  bot.Up,
	{1, 0}:  bot.Right,
	{0, -1}: bot.Down,
	{-1, 0}: bot.Left,
}

const logicStep = 100 * time.Millisecond

var lastLogic = time.Now()

var (
	camX, camY float32 // world‑space origin of the view
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
	if action == glfw.Press {
		dragging = true
		dragStartX, dragStartY = w.GetCursorPos()
		camStartX, camStartY = camX, camY
	} else if action == glfw.Release {
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

type Position struct{ X, Y int }

var bots = map[Position]bot.Bot{}

func init() { runtime.LockOSThread() }

func main() {
	rand.Seed(time.Now().UnixNano())
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

	// NEW: set 1 world-unit = 1 grid cell, square cells guaranteed
	gl.MatrixMode(gl.PROJECTION)
	gl.LoadIdentity()
	gl.Ortho(0, float64(cols), 0, float64(rows), -1, 1)
	gl.MatrixMode(gl.MODELVIEW)
	gl.LoadIdentity()

	gl.ClearColor(0.1, 0.1, 0.1, 1.0)
	gameLoop(window)
}

func gameLoop(window *glfw.Window) {
	generateBots()
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

func generateBots() {
	for r := range rows {
		for c := range cols {
			pos := Position{c, r}
			if isWall(pos) || rand.Intn(100) > 2 {
				continue
			}
			bots[pos] = bot.NewBot("Bot 1")
		}
	}
}

func botsActions() {
	newBots := make(map[Position]bot.Bot)
	for pos, b := range bots {
		np := Position{pos.X + b.Dir[0], pos.Y + b.Dir[1]}

		_, occ := bots[np]
		_, plan := newBots[np]

		if isWall(np) || occ || plan {
			b.Dir = bot.RandomDir()
			newBots[pos] = b
			continue
		}
		b.Dir = bot.RandomDir()
		newBots[np] = b
	}
	bots = newBots
}

func drawGrid() {
	for r := range rows {
		for c := range cols {
			x := float32(c)
			y := float32(r)
			pos := Position{c, r}

			if isWall(pos) {
				drawCell(x, y, 0.7, 0.7, 0.7, 1, 1)
				continue
			}

			if b, ok := bots[pos]; ok {
				drawBot(x, y, 0.3, 0.3, 1.0, 1, 1, b.Dir)
				continue
			}

			// empty space
			drawCell(x, y, 0.2, 0.2, 0.2, 1, 1)
		}
	}
}

func drawBot(x, y, r, g, b, w, h float32, dir bot.Direction) {
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
}

func drawFin(w, h, y, r, g, b, x float32) {
	finW := w * 0.6
	finH := h * 0.2
	finY := y - h*0.1
	gl.Color3f(r, g, b)
	gl.Begin(gl.QUADS)
	gl.Vertex2f(x+w*0.2, finY)
	gl.Vertex2f(x+w*0.2+finW, finY)
	gl.Vertex2f(x+w*0.2+finW, finY+finH)
	gl.Vertex2f(x+w*0.2, finY+finH)
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

func isWall(pos Position) bool {
	return pos.X == 0 || pos.Y == 0 || pos.X == cols-1 || pos.Y == rows-1
}
