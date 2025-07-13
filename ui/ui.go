package ui

import (
	"fmt"
	"golab/board"
	"golab/bot"
	"golab/config"
	"image"
	"os"
	"time"

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

var BotTexture uint32

var (
	clrGrey  = [3]float32{0.2, 0.2, 0.2}
	clrWhite = [3]float32{1, 1, 1}
	clrLight = clrWhite
	clrDark  = clrGrey
)

var (
	uvFood    = [4]float32{0 * tile, 0, 1 * tile, 1}
	uvWall    = [4]float32{1 * tile, 0, 2 * tile, 1}
	uvOre     = [4]float32{2 * tile, 0, 3 * tile, 1}
	uvPoison  = [4]float32{3 * tile, 0, 4 * tile, 1}
	uvChest   = [4]float32{4 * tile, 0, 5 * tile, 1}
	uvFarm    = [4]float32{5 * tile, 0, 6 * tile, 1}
	uvSpawner = [4]float32{6 * tile, 0, 7 * tile, 1}
	uvLight   = [4]float32{7 * tile, 0, 8 * tile, 1}
	uvBot     = [4]float32{8 * tile, 0, 9 * tile, 1}
	uvDark    = [4]float32{9 * tile, 0, 10 * tile, 1}
)

var (
	uvEmpty    = uvDark
	clrDefault = clrGrey
	// uvEmpty    = uvLight
	// clrDefault = clrLight
)

var conf *config.Config
var gameState *config.GameState

// Camera
var (
	camX, camY float32
	camScale   float32 = 1.0

	dragging               bool
	dragStartX, dragStartY float64
	camStartX, camStartY   float32
	AppWindow              *glfw.Window
)

var drawShark = true
var Font *gltext.Font

const tile = 1.0 / 10.0

type v = struct{ x, y, u, v, r, g, b, a float32 }

func makeVert(p board.Position, col [3]float32, uv [4]float32) v {
	return v{
		float32(p.C), float32(p.R),
		uv[0], uv[1],
		col[0], col[1], col[2], 1,
	}
}

var buf []v

var (
	atlasTex   uint32
	vbo        uint32
	vboStatic  uint32
	vboDynamic uint32
	vertsStat  []v
	vertsDyn   []v
)

const (
	vPerQuad = 4
	maxCells = board.Rows * board.Cols
	maxVerts = maxCells * vPerQuad
)

func BuildStaticLayer(brd *board.Board) {
	vertsStat = make([]v, maxVerts)
	statPos := 0

	for idx, occ := range *brd.GetGrid() {
		pos := board.Position{R: idx / board.Cols, C: idx % board.Cols}
		col, uv := pickSprite(occ)
		if occ == nil || mayVanish(occ) {
			writeQuad(vertsDyn, idx*vPerQuad, pos, clrDefault, uvEmpty)
			continue
		}
		writeQuad(vertsStat, statPos, pos, col, uv)
		statPos += vPerQuad
	}
	vertsStat = vertsStat[:statPos]
	gl.GenBuffers(1, &vboStatic)
	gl.BindBuffer(gl.ARRAY_BUFFER, vboStatic)
	gl.BufferData(gl.ARRAY_BUFFER, len(vertsStat)*int(stride), gl.Ptr(vertsStat), gl.STATIC_DRAW)
}

func writeQuad(buf []v, base int, p board.Position, col [3]float32, uv [4]float32) {
	x, y := float32(p.C), float32(p.R)
	buf[base+0] = v{x, y, uv[0], uv[1], col[0], col[1], col[2], 1}
	buf[base+1] = v{x + 1, y, uv[2], uv[1], col[0], col[1], col[2], 1}
	buf[base+2] = v{x + 1, y + 1, uv[2], uv[3], col[0], col[1], col[2], 1}
	buf[base+3] = v{x, y + 1, uv[0], uv[3], col[0], col[1], col[2], 1}
}

func mayVanish(o board.Occupant) bool {
	switch o.(type) {
	case *bot.Bot, board.Food, board.Resource, board.Poison,
		board.Organics, board.Farm, board.Controller, board.Spawner:
		return true
	default:
		return false
	}
}

func appendQuad(buf *[]v, x, y float32, c [3]float32, uv [4]float32) {
	*buf = append(*buf,
		v{x, y, uv[0], uv[1], c[0], c[1], c[2], 1},
		v{x + 1, y, uv[2], uv[1], c[0], c[1], c[2], 1},
		v{x + 1, y + 1, uv[2], uv[3], c[0], c[1], c[2], 1},
		v{x, y + 1, uv[0], uv[3], c[0], c[1], c[2], 1},
	)
}

func keyCallback(w *glfw.Window, key glfw.Key, scancode int, action glfw.Action, mods glfw.ModifierKey) {
	if action != glfw.Press {
		return
	}
	switch key {
	case glfw.KeyK:
		conf.SpeedUp()
	case glfw.KeyJ:
		conf.SlowDown()
	case glfw.KeyP:
		conf.Pause = !conf.Pause
		if !conf.Pause {
			gameState.LastLogic = time.Now()
		}
	case glfw.KeyEscape:
		w.SetShouldClose(true)
	}
}

func SetGameState(s *config.GameState) {
	if s == nil {
		panic("config is nil")
	}
	gameState = s
}

func SetConfig(config *config.Config) {
	if config == nil {
		panic("config is nil")
	}
	conf = config
}

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
	window.SetKeyCallback(keyCallback)
	window.SetCursorPosCallback(cursorPosCallback)

	if err != nil {
		panic(err)
	}
	window.MakeContextCurrent()
	glfw.SwapInterval(1)
	if err := gl.Init(); err != nil {
		panic(err)
	}

	gl.Enable(gl.TEXTURE_2D)
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)

	Font = LoadFont("/usr/share/fonts/truetype/msttcorefonts/Arial.ttf")

	const path = "/home/alice/projects/golab/"
	BotTexture = loadTexture(path + "bot.jpg")
	vertsDyn = make([]v, maxVerts)
	for idx := range maxCells {
		p := board.Position{R: idx / board.Cols, C: idx % board.Cols}
		writeQuad(vertsDyn, idx*vPerQuad, p, clrDefault, uvEmpty)
	}
	dynVertCount = len(vertsDyn)

	gl.GenBuffers(1, &vboDynamic)
	gl.BindBuffer(gl.ARRAY_BUFFER, vboDynamic)
	gl.BufferData(gl.ARRAY_BUFFER, len(vertsDyn)*int(stride), gl.Ptr(vertsDyn), gl.DYNAMIC_DRAW)
	atlasTex = loadTexture(path + "sprites/atlas.png")
	enableAttribs()

	gl.MatrixMode(gl.PROJECTION)
	gl.LoadIdentity()
	gl.Ortho(0, float64(cols), 0, float64(rows), -1, 1)
	gl.MatrixMode(gl.MODELVIEW)
	gl.LoadIdentity()

	r, g, b := clrDefault[0], clrDefault[1], clrDefault[2]
	gl.ClearColor(r, g, b, 1)
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

func loadTexture(filename string) uint32 {
	file, err := os.Open(filename)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		panic(err)
	}

	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	data := make([]uint8, 0, w*h*4)

	for y := bounds.Max.Y - 1; y >= bounds.Min.Y; y-- {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			data = append(data, uint8(r>>8), uint8(g>>8), uint8(b>>8), uint8(a>>8))
		}
	}

	var tex uint32
	gl.GenTextures(1, &tex)
	gl.BindTexture(gl.TEXTURE_2D, tex)

	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
	gl.PixelStorei(gl.UNPACK_ALIGNMENT, 1)

	gl.TexImage2D(
		gl.TEXTURE_2D, 0, gl.RGBA,
		int32(w), int32(h), 0,
		gl.RGBA, gl.UNSIGNED_BYTE,
		gl.Ptr(data),
	)
	return tex
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

	gl.Color3f(1, 0, 0)
	// gl.Color4f(0, 0, 0, 1)
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

	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
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
	f := float32(1)
	gl.Scalef(camScale*f, camScale*f, 1)
	gl.Translatef(-camX*f, -camY*f, 0)
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

var overlayW float32 = 230 // pixels
var overlayH float32 = 90  // pixels

func drawFloatingPane(offsetX float32, renderText func()) {
	winW, winH := AppWindow.GetSize()

	gl.MatrixMode(gl.PROJECTION)
	gl.PushMatrix()
	gl.LoadIdentity()
	gl.Ortho(0, float64(winW), 0, float64(winH), -1, 1)

	gl.MatrixMode(gl.MODELVIEW)
	gl.PushMatrix()
	gl.LoadIdentity()

	gl.Disable(gl.TEXTURE_2D)
	gl.Enable(gl.BLEND)
	gl.Color4f(0.05, 0.05, 0.05, 0.8)
	gl.Begin(gl.QUADS)
	gl.Vertex2f(offsetX+10, float32(winH)-10)
	gl.Vertex2f(offsetX+230+overlayW, float32(winH)-10)
	gl.Vertex2f(offsetX+230+overlayW, float32(winH)-100-overlayH)
	gl.Vertex2f(offsetX+10, float32(winH)-100-overlayH)
	gl.End()

	gl.Enable(gl.TEXTURE_2D)
	gl.Color4f(1, 1, 1, 1)

	renderText()
}

func pickSprite(o board.Occupant) (color [3]float32, uv [4]float32) {
	switch o := o.(type) {
	case *bot.Bot:
		r, g, b := o.Color[0], o.Color[1], o.Color[2]
		clr := [3]float32{r, g, b}
		return clr, uvBot
	case board.Food:
		return [3]float32{1, 0, 0.8}, uvFood
	case board.Water:
		return [3]float32{0, 0, 0.8}, uvLight
	case board.Organics:
		return [3]float32{0, 0.8, 0}, uvLight
	case board.Building:
		return clrLight, uvWall
	case board.Wall:
		return clrLight, uvWall
	case board.Controller:
		return clrLight, uvChest
	case board.Mine:
		return clrLight, uvSpawner
	case board.Resource:
		return clrLight, uvOre
	case board.Farm:
		return clrLight, uvFarm
	case board.Poison:
		return clrLight, uvPoison
	case nil:
		return clrDefault, uvEmpty
	default:
		return clrDefault, uvEmpty
	}
}

// TODO: rearrange
const (
	attrPos = 0
	attrUV  = 1
	attrCol = 2
)
const stride = int32(8 * 4)

var dynVertCount int

func DrawGrid(brd *board.Board, bots []*bot.Bot) {
	gl.Clear(gl.COLOR_BUFFER_BIT)
	ApplyCamera()

	// 1. any cell with a bot is dirty this frame
	for idx, b := range bots {
		if b != nil {
			brd.MarkDirty(idx)
		}
	}

	// -----------------------------------------------------------------------------
	// Upload only contiguous runs of dirty tiles – FIX #2 (crash-free)
	// -----------------------------------------------------------------------------
	gl.BindBuffer(gl.ARRAY_BUFFER, vboDynamic)

	// helpers --------------------------------------------------------------------
	runStart := -1 // first dirty cell in current run (-1 == no run yet)
	prevIdx := -2  // previous dirty cell index

	flushRun := func(first, last int) {
		if first < 0 {
			return
		}
		nCells := last - first + 1
		baseVert := first * vPerQuad
		nVerts := nCells * vPerQuad
		byteOff := int32(baseVert) * stride

		gl.BufferSubData(gl.ARRAY_BUFFER, int(byteOff), nVerts*int(stride), gl.Ptr(vertsDyn[baseVert:baseVert+nVerts]))
	}
	// -----------------------------------------------------------------------------

	for idx, dirty := range brd.DirtyBitmap() {
		if !dirty {
			if runStart >= 0 { // end of a run
				flushRun(runStart, prevIdx) // push it
				runStart = -1
			}
			continue
		}

		// -------- tile idx IS dirty --------
		p := Position{R: idx / board.Cols, C: idx % board.Cols}
		col, uv := pickSprite(brd.At(p))
		base := idx * vPerQuad
		writeQuad(vertsDyn, base, p, col, uv)
		if base+vPerQuad > dynVertCount {
			dynVertCount = base + vPerQuad
		}
		brd.MarkClean(idx)

		// build / extend the run
		if idx == prevIdx+1 && runStart >= 0 {
			// still contiguous – just extend
		} else {
			// new run; flush previous if any
			if runStart >= 0 {
				flushRun(runStart, prevIdx)
			}
			runStart = idx
		}
		prevIdx = idx
	}
	// any pending run left?
	flushRun(runStart, prevIdx)

	// 4. draw everything
	gl.BindTexture(gl.TEXTURE_2D, atlasTex)

	gl.BindBuffer(gl.ARRAY_BUFFER, vboStatic)
	gl.DrawArrays(gl.QUADS, 0, int32(len(vertsStat))) // static layer

	gl.BindBuffer(gl.ARRAY_BUFFER, vboDynamic)
	gl.DrawArrays(gl.QUADS, 0, int32(dynVertCount)) // dynamic layer

	drawOverlay()
	Window.SwapBuffers()
	glfw.PollEvents()
}

func enableAttribs() {
	gl.EnableClientState(gl.VERTEX_ARRAY)
	gl.VertexPointer(2, gl.FLOAT, stride, gl.PtrOffset(0))

	gl.EnableClientState(gl.TEXTURE_COORD_ARRAY)
	gl.TexCoordPointer(2, gl.FLOAT, stride, gl.PtrOffset(2*4))

	gl.EnableClientState(gl.COLOR_ARRAY)
	gl.ColorPointer(4, gl.FLOAT, stride, gl.PtrOffset(4*4))
}

func drawOverlay() {
	drawFloatingPane(0, func() {
		state := "Running"
		if conf.Pause {
			state = "Paused"
		}
		Font.Printf(20, 20, "State: %s", state)
		Font.Printf(20, 55, "J/K speed   P pause")
		Font.Printf(20, 100, "Speed: %d", conf.Speed())
		textClearfix()
	})

	drawFloatingPane(500, func() {
		Font.Printf(520, 20, "Test")
		Font.Printf(520, 40, fmt.Sprintf("Live bots: %d", conf.LiveBots))
		// Font.Printf(fmt.Sprintf("\nGeneration: %d; Max HP: %d;", g.currGen, g.maxHp))
		// Font.Printf(" Latest improvement: %d;", g.latestImprovement)
		// Font.Printf(fmt.Sprintf("\nBots amount: %d", len(g.Bots)))
		textClearfix()
	})
}

func textClearfix() {
	gl.PopMatrix()
	gl.MatrixMode(gl.PROJECTION)
	gl.PopMatrix()
	gl.MatrixMode(gl.MODELVIEW)
}
