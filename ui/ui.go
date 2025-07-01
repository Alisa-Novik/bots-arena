package ui

import (
	"fmt"
	"golab/board"
	"golab/bot"
	"os"

	"github.com/go-gl/gl/v2.1/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/go-gl/gltext"
)

type Position = board.Position

const (
	rows = board.Rows
	cols = board.Cols
)

var Window *glfw.Window

// Camera
var (
	camX, camY float32
	camScale   float32 = 1.0

	dragging               bool
	dragStartX, dragStartY float64
	camStartX, camStartY   float32
	AppWindow              *glfw.Window
)

var Font *gltext.Font

var paused bool

func PrepareUi() {
	if err := glfw.Init(); err != nil {
		panic(err)
	}
	mon := glfw.GetPrimaryMonitor()
	mode := mon.GetVideoMode()
	screenW, screenH := mode.Width, mode.Height
	glfw.WindowHint(glfw.ContextVersionMinor, 1)
	window, err := glfw.CreateWindow(screenW, screenH, "Bot Arena", nil, nil)
	AppWindow = window
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
	Font = LoadFont("/usr/share/fonts/truetype/msttcorefonts/Arial.ttf")

	gl.MatrixMode(gl.PROJECTION)
	gl.LoadIdentity()
	gl.Ortho(0, float64(cols), 0, float64(rows), -1, 1)
	gl.MatrixMode(gl.MODELVIEW)
	gl.LoadIdentity()

	gl.ClearColor(0.1, 0.1, 0.1, 1.0)
	Window = window
}

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

func textAtWorld(wx, wy float32, s string) {
	winW, winH := AppWindow.GetSize()

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
	Font.Printf(px, py, s)

	gl.PopMatrix() // MODELVIEW
	gl.MatrixMode(gl.PROJECTION)
	gl.PopMatrix()
	gl.MatrixMode(gl.MODELVIEW)
}

func LoadFont(name string) *gltext.Font {
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

func ApplyCamera() {
	gl.MatrixMode(gl.MODELVIEW)
	gl.LoadIdentity()
	gl.Scalef(camScale, camScale, 1)
	gl.Translatef(-camX, -camY, 0)
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

func DrawGrid(brd board.Board, bots map[Position]bot.Bot) {
	gl.Clear(gl.COLOR_BUFFER_BIT)
	ApplyCamera()
	for r := range rows {
		for c := range cols {
			x := float32(c)
			y := float32(r)
			pos := Position{X: c, Y: r}

			if brd.IsBuilding(pos) {
				drawBuilding(x, y, 0.2, 0.9, 0.2, 1, 1)
				continue
			}

			if brd.IsResource(pos) {
				drawCell(x, y, 0.8, 0, 0, 1, 1)
				continue
			}

			if brd.IsWall(pos) {
				drawCell(x, y, 0.7, 0.7, 0.7, 1, 1)
				continue
			}

			if b, ok := bots[pos]; ok {
				drawBot(x, y, 0, 0, 0.8, 1, 1, b.Dir, b.Hp)
				continue
			}

			if brd.IsController(pos) {
				c := brd.At(pos).(board.Controller)
				drawController(x, y, 0.9, 0.9, 0.9, 1, 1, c.Amount)
				continue
			}

			// empty space
			drawCell(x, y, 0.2, 0.2, 0.2, 1, 1)
		}
	}
	Window.SwapBuffers()
	glfw.PollEvents()
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

func drawResource(x, y, r, g, b, w, h float32) {
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

func drawController(x, y, r, g, b, w, h float32, hp int) {
	gl.Begin(gl.QUADS)
	gl.Color3f(r, g, b)
	gl.Vertex2f(x, y)
	gl.Vertex2f(x+w, y)
	gl.Vertex2f(x+w, y+h)
	gl.Vertex2f(x, y+h)
	gl.End()

	textAtWorld(x+0.05, y+0.05, fmt.Sprintf("%d", hp))
}

func drawBuilding(x, y, r, g, b, w, h float32) {
	gl.Begin(gl.QUADS)
	gl.Color3f(r, g, b)
	gl.Vertex2f(x, y)
	gl.Vertex2f(x+w, y)
	gl.Vertex2f(x+w, y+h)
	gl.Vertex2f(x, y+h)
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
