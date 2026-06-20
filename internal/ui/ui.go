package ui

import (
	"fmt"
	"golab/internal/config"
	"golab/internal/core"
	"golab/internal/util"
	"image"
	"os"

	"github.com/go-gl/gl/v2.1/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/go-gl/gltext"
)

type Position = core.Position

const (
	rows = core.Rows
	cols = core.Cols
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
	uvFlag    = [4]float32{10 * tile, 0, 11 * tile, 1}
)

var (
	uvEmpty    = uvDark
	clrDefault = clrGrey
	// uvEmpty    = uvLight
	// clrDefault = clrLight
)

var conf *config.Config
var brd *core.Board // for marking as dirty
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
var SmallFont *gltext.Font

const tile = 1.0 / 11.0

type v = struct{ x, y, u, v, r, g, b, a float32 }

var buf []v

var (
	atlasTex      uint32
	vbo           uint32
	vboStatic     uint32
	vboDynamic    uint32
	vboDensity    uint32
	vertsStat     []v
	vertsDyn      []v
	vertsDensity  []v
	densityChunks []DensityChunk
)

const (
	vPerQuad         = 4
	maxCells         = core.Rows * core.Cols
	maxVerts         = maxCells * vPerQuad
	maxDensityChunks = ((core.Rows + DensityChunkSize - 1) / DensityChunkSize) * ((core.Cols + DensityChunkSize - 1) / DensityChunkSize)
)

func BuildStaticLayer(brd *core.Board) {
	if vboStatic != 0 {
		gl.DeleteBuffers(1, &vboStatic)
		vboStatic = 0
	}

	vertsStat = make([]v, maxVerts)
	statPos := 0

	for idx, occ := range *brd.GetGrid() {
		pos := core.Position{R: idx / core.Cols, C: idx % core.Cols}
		writeQuad(vertsDyn, idx*vPerQuad, pos, clrDefault, uvEmpty)
		col, uv := pickSprite(occ, idx)
		if occ == nil || mayVanish(occ) {
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

func writeQuad(buf []v, base int, p core.Position, col [3]float32, uv [4]float32) {
	x, y := float32(p.C), float32(p.R)
	writeRectQuad(buf, base, x, y, 1, 1, col, uv, 1)
}

func writeRectQuad(buf []v, base int, x, y, w, h float32, col [3]float32, uv [4]float32, alpha float32) {
	buf[base+0] = v{x, y, uv[0], uv[1], col[0], col[1], col[2], alpha}
	buf[base+1] = v{x + w, y, uv[2], uv[1], col[0], col[1], col[2], alpha}
	buf[base+2] = v{x + w, y + h, uv[2], uv[3], col[0], col[1], col[2], alpha}
	buf[base+3] = v{x, y + h, uv[0], uv[3], col[0], col[1], col[2], alpha}
}

func mayVanish(o core.Occupant) bool {
	switch o.(type) {
	case *core.Bot, core.Food, core.Resource, core.Poison,
		core.Organics, core.Farm, core.Controller, *core.Controller, core.Depot, *core.Depot, core.Spawner, core.Mine, core.ColonyFlag:
		return true
	default:
		return false
	}
}

func SetGameState(s *config.GameState) {
	if s == nil {
		panic("config is nil")
	}
	gameState = s
}

func SetBoard(theBoard *core.Board) {
	if theBoard == nil {
		panic("brd is nil")
	}
	brd = theBoard
}

func SetConfig(config *config.Config) {
	if config == nil {
		panic("config is nil")
	}
	conf = config
}

var ctrlState ControlState

func PrepareUi() {
	if err := glfw.Init(); err != nil {
		panic(err)
	}

	ctrlState = ControlState{
		HoveredIdx:     -1,
		LastClickIdx:   -1,
		ActiveTool:     GodToolInspect,
		BrushRadius:    3,
		LastGodMessage: "Ready",
		RenderMode:     RenderModeColony,
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

	window.SetFramebufferSizeCallback(func(w *glfw.Window, pxW, pxH int) {
		gl.Viewport(0, 0, int32(pxW), int32(pxH))
	})

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

	const fontPath = "/usr/share/fonts/truetype/msttcorefonts/Arial.ttf"
	Font = LoadFont(fontPath, 20)
	SmallFont = LoadFont(fontPath, 16)
	const path = "/home/alice/projects/golab/assests/sprites/"
	BotTexture = loadTexture(path + "bot.jpg")
	atlasTex = loadTexture(path + "atlas.png")

	vertsDyn = make([]v, maxVerts)
	for idx := range maxCells {
		p := core.Position{R: idx / core.Cols, C: idx % core.Cols}
		writeQuad(vertsDyn, idx*vPerQuad, p, clrDefault, uvEmpty)
	}
	dynVertCount = len(vertsDyn)

	gl.GenBuffers(1, &vboDynamic)
	gl.BindBuffer(gl.ARRAY_BUFFER, vboDynamic)
	gl.BufferData(gl.ARRAY_BUFFER, len(vertsDyn)*int(stride), gl.Ptr(vertsDyn), gl.DYNAMIC_DRAW)

	vertsDensity = make([]v, maxDensityChunks*vPerQuad)
	densityChunks = make([]DensityChunk, 0, maxDensityChunks)
	gl.GenBuffers(1, &vboDensity)
	gl.BindBuffer(gl.ARRAY_BUFFER, vboDensity)
	gl.BufferData(gl.ARRAY_BUFFER, len(vertsDensity)*int(stride), gl.Ptr(vertsDensity), gl.DYNAMIC_DRAW)
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

func LoadFont(name string, size int) *gltext.Font {
	f, err := os.Open(name)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	ft, err := gltext.LoadTruetype(f, int32(size), 32, 127, gltext.LeftToRight)
	if err != nil {
		panic(err)
	}

	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	return ft
}

func ApplyCamera() {
	gl.MatrixMode(gl.MODELVIEW)
	gl.LoadIdentity()
	f := float32(1)
	gl.Scalef(camScale*f, camScale*f, 1)
	gl.Translatef(-camX*f, -camY*f, 0)
}

func pickSprite(o core.Occupant, idx int) (color [3]float32, uv [4]float32) {
	pos := util.PosOf(idx)
	if idx == ctrlState.HoveredIdx {
		switch ctrlState.ActiveTool {
		case GodToolWater:
			return [3]float32{0.2, 0.55, 1}, uvLight
		case GodToolPoison:
			return [3]float32{0.75, 0.2, 0.95}, uvPoison
		case GodToolFood:
			return [3]float32{1, 0.2, 0.8}, uvFood
		case GodToolColony:
			return util.OrangeColor(), uvChest
		case GodToolFreeze, GodToolUnfreeze:
			return util.LightBlueColor(), uvLight
		case GodToolBless:
			return util.GreenColor(), uvLight
		case GodToolCurse:
			return util.RedColor(), uvPoison
		case GodToolBuild:
			return clrLight, uvWall
		}
		if _, ok := o.(*core.Bot); ok {
			return util.YellowColor(), uvBot
		}
	}
	if ctrlState.RenderMode == RenderModePheromone && brd != nil {
		color, uv = pheromoneSprite(brd.PheromoneAt(pos))
		if brd.IsFrozen(pos) {
			return frozenTint(color), uvLight
		}
		return color, uv
	}
	switch o := o.(type) {
	case *core.Bot:
		color, uv = botRenderColor(o), uvBot
	case core.Food:
		color, uv = [3]float32{1, 0, 0.8}, uvFood
	case core.Water:
		color, uv = [3]float32{0, 0, 0.8}, uvLight
	case core.Organics:
		color, uv = [3]float32{0, 0.8, 0}, uvLight
	case core.Building:
		color, uv = clrLight, uvWall
	case core.Wall:
		color, uv = clrLight, uvWall
	case core.Controller:
		color, uv = clrLight, uvChest
	case *core.Controller:
		color, uv = clrLight, uvChest
	case core.Depot:
		color, uv = [3]float32{0.20, 0.95, 0.85}, uvChest
	case *core.Depot:
		color, uv = [3]float32{0.20, 0.95, 0.85}, uvChest
	case core.Mine:
		color, uv = clrLight, uvSpawner
	case core.Resource:
		color, uv = clrLight, uvOre
	case core.Farm:
		color, uv = clrLight, uvFarm
	case core.Poison:
		color, uv = clrLight, uvPoison
	case core.ColonyFlag:
		color, uv = clrLight, uvFlag
	case nil:
		color, uv = clrDefault, uvEmpty
	default:
		color, uv = clrDefault, uvEmpty
	}
	if ctrlState.RenderMode == RenderModeColony && brd != nil {
		color, uv = colonySprite(o, pos, color, uv)
	}
	if ctrlState.RenderMode == RenderModeBiome && brd != nil {
		color, uv = biomeSprite(o, brd.BiomeAt(pos), color, uv)
	}
	if brd != nil && brd.IsFrozen(pos) {
		return frozenTint(color), uvLight
	}
	return color, uv
}

func botRenderColor(bot *core.Bot) [3]float32 {
	if bot == nil {
		return clrDefault
	}
	if ctrlState.RenderMode == RenderModeNormal {
		if bot.IsSelected {
			return util.YellowColor()
		}
		return colonyBotColor(bot, bot.Color)
	}

	switch ctrlState.RenderMode {
	case RenderModeGenome:
		return genomeColor(bot.Genome)
	case RenderModeHealth:
		return healthColor(bot.Hp)
	case RenderModeInventory:
		return inventoryColor(min(bot.Inventory.Food, bot.Inventory.Ore))
	case RenderModeColony:
		return colonyModeBotColor(bot)
	case RenderModeTask:
		return taskColor(bot)
	case RenderModeBiome:
		if bot.IsSelected {
			return util.YellowColor()
		}
		return bot.Color
	default:
		return bot.Color
	}
}

func colonySprite(o core.Occupant, pos core.Position, color [3]float32, uv [4]float32) ([3]float32, [4]float32) {
	switch v := o.(type) {
	case *core.Bot:
		return colonyModeBotColor(v), uvBot
	case core.Water:
		return [3]float32{0.02, 0.04, 0.10}, uvLight
	case core.Controller:
		return colonyStructureColor(v.Colony, color), uvChest
	case *core.Controller:
		if v != nil {
			return colonyStructureColor(v.Colony, color), uvChest
		}
	case core.Depot:
		return colonyStructureColor(v.Colony, color), uvChest
	case *core.Depot:
		if v != nil {
			return colonyStructureColor(v.Colony, color), uvChest
		}
	case core.Farm:
		return colonyStructureColor(colonyForOwnedCell(v.Colony, v.Owner), color), uvFarm
	case core.Spawner:
		return colonyStructureColor(colonyForOwner(v.Owner), color), uvSpawner
	case core.ColonyFlag:
		return colonyStructureColor(brd.PheromoneHomeOwnerAt(pos), color), uvFlag
	case nil:
		if owner := brd.PheromoneHomeOwnerAt(pos); owner != nil && brd.PheromoneAt(pos).Home > 0 {
			return colonyTissueColor(owner.Color, brd.PheromoneAt(pos).Home), uvLight
		}
		return clrDefault, uvEmpty
	}
	return lerpColor(clrDefault, color, 0.18), uv
}

func colonyBotColor(bot *core.Bot, base [3]float32) [3]float32 {
	if bot == nil || bot.Colony == nil {
		return base
	}
	if bot.ConnnectedToColony {
		return lerpColor(base, bot.Colony.Color, 0.82)
	}
	return lerpColor(clrDefault, bot.Colony.Color, 0.28)
}

func colonyModeBotColor(bot *core.Bot) [3]float32 {
	if bot == nil || bot.Colony == nil {
		return [3]float32{0.22, 0.24, 0.24}
	}
	if bot.ConnnectedToColony {
		return lerpColor(bot.Colony.Color, clrLight, 0.16)
	}
	return lerpColor(clrDefault, bot.Colony.Color, 0.34)
}

func colonyStructureColor(colony *core.Colony, fallback [3]float32) [3]float32 {
	if colony == nil {
		return fallback
	}
	return lerpColor(fallback, colony.Color, 0.92)
}

func colonyTissueColor(colonyColor [3]float32, home uint8) [3]float32 {
	intensity := 0.36 + 0.46*clamp01(float32(home)/255)
	return lerpColor(clrDefault, colonyColor, intensity)
}

func colonyForOwnedCell(colony *core.Colony, owner *core.Bot) *core.Colony {
	if colony != nil {
		return colony
	}
	return colonyForOwner(owner)
}

func colonyForOwner(owner *core.Bot) *core.Colony {
	if owner == nil {
		return nil
	}
	return owner.Colony
}

func biomeSprite(o core.Occupant, biome core.Biome, color [3]float32, uv [4]float32) ([3]float32, [4]float32) {
	tint := biomeRenderColor(biome)
	switch o.(type) {
	case nil:
		return tint, uvLight
	case core.Food, core.Organics:
		return lerpColor(color, tint, 0.28), uv
	case core.Water:
		return lerpColor(color, tint, 0.16), uv
	default:
		return color, uv
	}
}

func biomeRenderColor(biome core.Biome) [3]float32 {
	switch biome {
	case core.BiomeFertile:
		return [3]float32{0.20, 0.54, 0.31}
	case core.BiomeMineral:
		return [3]float32{0.58, 0.48, 0.24}
	case core.BiomeToxic:
		return [3]float32{0.52, 0.22, 0.58}
	default:
		return [3]float32{0.23, 0.25, 0.27}
	}
}

func pheromoneSprite(values core.PheromoneValues) ([3]float32, [4]float32) {
	if values.IsZero() {
		return [3]float32{0.03, 0.035, 0.035}, uvDark
	}
	return pheromoneColor(values), uvLight
}

func pheromoneColor(values core.PheromoneValues) [3]float32 {
	weights := []struct {
		value uint8
		color [3]float32
	}{
		{values.Food, [3]float32{1.00, 0.00, 0.82}},
		{values.Ore, [3]float32{1.00, 0.86, 0.05}},
		{values.Home, [3]float32{0.00, 0.90, 1.00}},
		{values.Danger, [3]float32{1.00, 0.05, 0.02}},
	}
	var sum, maxValue float32
	var out [3]float32
	for _, weight := range weights {
		value := float32(weight.value)
		if value == 0 {
			continue
		}
		sum += value
		if value > maxValue {
			maxValue = value
		}
		out[0] += weight.color[0] * value
		out[1] += weight.color[1] * value
		out[2] += weight.color[2] * value
	}
	if sum == 0 {
		return clrDefault
	}
	intensity := 0.25 + 0.75*clamp01(maxValue/255)
	return [3]float32{
		clamp01(out[0] / sum * intensity),
		clamp01(out[1] / sum * intensity),
		clamp01(out[2] / sum * intensity),
	}
}

func genomeColor(genome core.Genome) [3]float32 {
	var hash uint32 = 2166136261
	for i, gene := range genome.Matrix {
		hash ^= uint32(gene + i*131)
		hash *= 16777619
	}
	hue := float32(hash%360) / 360
	return hsvColor(hue, 0.82, 0.96)
}

func healthColor(hp int) [3]float32 {
	if hp >= 500 {
		return [3]float32{0.20, 0.92, 1.00}
	}
	ratio := clamp01(float32(hp) / 500)
	if ratio < 0.5 {
		return lerpColor([3]float32{0.95, 0.12, 0.08}, [3]float32{1.00, 0.82, 0.10}, ratio*2)
	}
	return lerpColor([3]float32{1.00, 0.82, 0.10}, [3]float32{0.18, 0.90, 0.30}, (ratio-0.5)*2)
}

func inventoryColor(amount int) [3]float32 {
	if amount <= 0 {
		return [3]float32{0.42, 0.44, 0.46}
	}
	ratio := clamp01(float32(amount) / 50)
	return lerpColor([3]float32{0.18, 0.38, 0.95}, [3]float32{1.00, 0.36, 0.08}, ratio)
}

func taskColor(bot *core.Bot) [3]float32 {
	if bot.CurrTask != nil {
		if bot.CurrTask.IsDone {
			return [3]float32{0.25, 0.95, 0.32}
		}
		return [3]float32{0.08, 0.88, 1.00}
	}
	if bot.ConnnectedToColony {
		return [3]float32{0.95, 0.80, 0.18}
	}
	if bot.Colony != nil {
		return [3]float32{1.00, 0.42, 0.08}
	}
	return [3]float32{0.62, 0.66, 0.66}
}

func hsvColor(h, s, val float32) [3]float32 {
	h = h - float32(int(h))
	if h < 0 {
		h += 1
	}
	sector := h * 6
	i := int(sector)
	f := sector - float32(i)
	p := val * (1 - s)
	q := val * (1 - s*f)
	t := val * (1 - s*(1-f))

	switch i % 6 {
	case 0:
		return [3]float32{val, t, p}
	case 1:
		return [3]float32{q, val, p}
	case 2:
		return [3]float32{p, val, t}
	case 3:
		return [3]float32{p, q, val}
	case 4:
		return [3]float32{t, p, val}
	default:
		return [3]float32{val, p, q}
	}
}

func lerpColor(a, b [3]float32, t float32) [3]float32 {
	t = clamp01(t)
	return [3]float32{
		a[0] + (b[0]-a[0])*t,
		a[1] + (b[1]-a[1])*t,
		a[2] + (b[2]-a[2])*t,
	}
}

func clamp01(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func frozenTint(color [3]float32) [3]float32 {
	return [3]float32{
		(color[0] + 0.35) / 2,
		(color[1] + 0.9) / 2,
		(color[2] + 1.0) / 2,
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
var densityVertCount int
var lastDensityMode bool

func idx(p Position) int {
	return util.Idx(p)
}

func DrawGrid(brd *core.Board, bots []*core.Bot) {
	gl.Clear(gl.COLOR_BUFFER_BIT)
	ApplyCamera()

	densityMode := useDensityRendering()
	if densityMode != lastDensityMode {
		brd.MarkAllDirty()
		lastDensityMode = densityMode
	}

	if !densityMode {
		for _, id := range brd.ActiveBotIDs() {
			if cell := brd.BotCell(id); cell >= 0 {
				brd.MarkDirty(cell)
			}
		}
	}

	bindVertexBuffer(vboDynamic)

	runStart := -1
	prevIdx := -2

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

	for _, p := range brd.PathsToRenderR {
		brd.MarkDirty(idx(p))
	}
	for idx, dirty := range brd.DirtyBitmap() {
		if !dirty {
			if runStart >= 0 {
				flushRun(runStart, prevIdx)
				runStart = -1
			}
			continue
		}

		p := Position{R: idx / core.Cols, C: idx % core.Cols}
		occupant := brd.At(p)
		if densityMode {
			if _, ok := occupant.(*core.Bot); ok {
				occupant = nil
			}
		}
		col, uv := pickSprite(occupant, idx)
		if conf.RenderPaths && brd.IsPathToRender(p) {
			col, uv = util.CyanColor(), uvLight
		}
		if conf.RenderTaskTargets && brd.IsTaskTargetToRender(p) {
			col, uv = util.PinkColor(), uvLight
		}
		if conf.RenderUnreachables && brd.IsUnreachableToRender(p) {
			col, uv = util.RedColor(), uvLight
		}
		base := idx * vPerQuad
		writeQuad(vertsDyn, base, p, col, uv)
		if base+vPerQuad > dynVertCount {
			dynVertCount = base + vPerQuad
		}
		brd.MarkClean(idx)

		if idx == prevIdx+1 && runStart >= 0 {
		} else {
			if runStart >= 0 {
				flushRun(runStart, prevIdx)
			}
			runStart = idx
		}
		prevIdx = idx
	}
	flushRun(runStart, prevIdx)

	gl.BindTexture(gl.TEXTURE_2D, atlasTex)

	bindVertexBuffer(vboStatic)
	gl.DrawArrays(gl.QUADS, 0, int32(len(vertsStat)))

	bindVertexBuffer(vboDynamic)
	gl.DrawArrays(gl.QUADS, 0, int32(dynVertCount))

	if densityMode {
		densityVertCount = rebuildDensityVerts(brd)
		if densityVertCount > 0 {
			bindVertexBuffer(vboDensity)
			gl.BufferSubData(gl.ARRAY_BUFFER, 0, densityVertCount*int(stride), gl.Ptr(vertsDensity[:densityVertCount]))
			gl.DrawArrays(gl.QUADS, 0, int32(densityVertCount))
		}
	} else {
		densityVertCount = 0
	}

	drawOverlay()
	Window.SwapBuffers()
	glfw.PollEvents()
}

func useDensityRendering() bool {
	if brd == nil || AppWindow == nil || brd.ActiveBotCount() == 0 {
		return false
	}
	winW, _ := AppWindow.GetSize()
	cellPx := float32(winW) / float32(cols) * camScale
	return camScale <= 0.85 || cellPx < 3.0
}

func rebuildDensityVerts(brd *core.Board) int {
	densityChunks = buildDensityChunksInto(densityChunks[:0], brd, DensityChunkSize, ctrlState.RenderMode)
	needed := len(densityChunks) * vPerQuad
	if needed > len(vertsDensity) {
		vertsDensity = make([]v, needed)
		gl.BindBuffer(gl.ARRAY_BUFFER, vboDensity)
		gl.BufferData(gl.ARRAY_BUFFER, len(vertsDensity)*int(stride), gl.Ptr(vertsDensity), gl.DYNAMIC_DRAW)
	}
	for i, chunk := range densityChunks {
		w := float32(min(DensityChunkSize, core.Cols-chunk.Col))
		h := float32(min(DensityChunkSize, core.Rows-chunk.Row))
		writeRectQuad(
			vertsDensity,
			i*vPerQuad,
			float32(chunk.Col),
			float32(chunk.Row),
			w,
			h,
			chunk.Color,
			uvLight,
			0.86,
		)
	}
	return needed
}

func bindVertexBuffer(buffer uint32) {
	gl.BindBuffer(gl.ARRAY_BUFFER, buffer)
	enableAttribs()
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
	winW, winH := AppWindow.GetSize()
	layout := overlayLayout(float32(winW))

	beginHUD(winW, winH)
	drawSimPanel(float32(winH), layout.simX, layout.simY)
	drawWorldPanel(float32(winH), layout.worldX, layout.worldY, layout.worldW)
	drawGodPanel(float32(winH), layout.godX, layout.godY, layout.godW)
	endHUD()
}

type hudRGBA struct {
	r, g, b, a float32
}

type hudLayout struct {
	simX, simY     float32
	worldX, worldY float32
	worldW         float32
	godX, godY     float32
	godW           float32
}

var (
	hudOrange = hudRGBA{1.00, 0.34, 0.02, 1.00}
	hudBlue   = hudRGBA{0.17, 0.54, 1.00, 1.00}
	hudGreen  = hudRGBA{0.25, 0.92, 0.39, 1.00}
	hudRed    = hudRGBA{1.00, 0.18, 0.12, 1.00}
	hudText   = hudRGBA{0.93, 0.94, 0.90, 1.00}
	hudMuted  = hudRGBA{0.56, 0.62, 0.62, 1.00}
	hudDim    = hudRGBA{0.23, 0.27, 0.27, 1.00}
)

var hudTools = []GodTool{
	GodToolInspect,
	GodToolWater,
	GodToolPoison,
	GodToolFood,
	GodToolColony,
	GodToolFreeze,
	GodToolUnfreeze,
	GodToolBless,
	GodToolCurse,
	GodToolBuild,
}

func overlayLayout(winW float32) hudLayout {
	const (
		margin = 12
		gap    = 14
		simW   = 420
		worldW = 480
		godW   = 690
	)

	if winW >= margin*2+simW+worldW+godW+gap*2 {
		return hudLayout{
			simX:   margin,
			simY:   margin,
			worldX: margin + simW + gap,
			worldY: margin,
			worldW: worldW,
			godX:   margin + simW + gap + worldW + gap,
			godY:   margin,
			godW:   godW,
		}
	}
	if winW >= margin*2+simW+worldW+gap {
		return hudLayout{
			simX:   margin,
			simY:   margin,
			worldX: margin + simW + gap,
			worldY: margin,
			worldW: worldW,
			godX:   margin,
			godY:   214,
			godW:   min(godW, winW-margin*2),
		}
	}
	return hudLayout{
		simX:   margin,
		simY:   margin,
		worldX: margin,
		worldY: 132,
		worldW: min(worldW, winW-margin*2),
		godX:   margin,
		godY:   338,
		godW:   min(godW, winW-margin*2),
	}
}

func beginHUD(winW, winH int) {
	gl.MatrixMode(gl.PROJECTION)
	gl.PushMatrix()
	gl.LoadIdentity()
	gl.Ortho(0, float64(winW), 0, float64(winH), -1, 1)

	gl.MatrixMode(gl.MODELVIEW)
	gl.PushMatrix()
	gl.LoadIdentity()

	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
}

func endHUD() {
	gl.Enable(gl.TEXTURE_2D)
	gl.Color4f(1, 1, 1, 1)

	gl.MatrixMode(gl.MODELVIEW)
	gl.PopMatrix()
	gl.MatrixMode(gl.PROJECTION)
	gl.PopMatrix()
	gl.MatrixMode(gl.MODELVIEW)
}

func drawSimPanel(winH, x, y float32) {
	const w, h = 420, 108
	drawPanel(winH, x, y, w, h, "SIM", hudOrange)

	state := "RUNNING"
	stateColor := hudGreen
	if conf.Pause {
		state = "PAUSED"
		stateColor = hudRed
	}
	drawText(Font, x+16, y+34, stateColor, state)
	drawText(SmallFont, x+16, y+64, hudText, "Speed %d", conf.Speed())
	drawPill(winH, x+158, y+31, 52, 25, "J/K", hudOrange, false)
	drawPill(winH, x+218, y+31, 58, 25, "P", hudOrange, conf.Pause)
	drawPill(winH, x+284, y+31, 34, 25, "R", hudRed, false)
	drawPill(winH, x+326, y+31, 78, 25, "V View", hudBlue, false)
	drawText(SmallFont, x+158, y+71, hudMuted, "speed")
	drawText(SmallFont, x+218, y+71, hudMuted, "pause")
	drawText(SmallFont, x+282, y+71, hudMuted, "reset")
	drawText(SmallFont, x+326, y+71, hudMuted, "%s", ctrlState.RenderMode.Label())
}

func drawWorldPanel(winH, x, y, w float32) {
	const h = 188
	drawPanel(winH, x, y, w, h, "WORLD", hudBlue)

	drawText(Font, x+16, y+34, hudText, "Live %d", conf.LiveBots)
	if gameState != nil {
		drawText(SmallFont, x+176, y+38, hudMuted, "Tick %d  %.1f TPS", gameState.LogicTick, gameState.LogicTicksPerSecond)
	}
	drawText(SmallFont, x+16, y+62, hudMuted, "View %s  %s", ctrlState.RenderMode.Label(), densityModeLabel())
	renderGameMasterOverlay(winH, x+16, y+88, w-32)
}

func drawGodPanel(winH, x, y, w float32) {
	const h = 188
	drawPanel(winH, x, y, w, h, "GOD TOOLS", hudOrange)

	activeColor := godToolColor(ctrlState.ActiveTool)
	drawPill(winH, x+16, y+31, 130, 25, ctrlState.ActiveTool.Label(), activeColor, true)
	drawPill(winH, x+154, y+31, 82, 25, fmt.Sprintf("Brush %d", ctrlState.BrushRadius), hudBlue, false)
	drawPill(winH, x+244, y+31, min(190, w-260), 25, fmt.Sprintf("Colony %s", selectedColonyLabel()), hudGreen, false)
	if w >= 650 {
		drawPill(winH, x+442, y+31, 86, 25, "G Gene", hudGreen, false)
		drawPill(winH, x+536, y+31, 74, 25, "M Map", hudBlue, false)
	}

	chipW := (w - 48) / 5
	chipH := float32(25)
	for i, tool := range hudTools {
		row := i / 5
		col := i % 5
		chipX := x + 16 + float32(col)*(chipW+4)
		chipY := y + 67 + float32(row)*(chipH+6)
		drawToolChip(winH, chipX, chipY, chipW, chipH, tool)
	}

	readoutY := y + 132
	drawRect(winH, x+16, readoutY, w-32, 43, hudRGBA{0.02, 0.025, 0.025, 0.62})
	drawRectBorder(winH, x+16, readoutY, w-32, 43, hudRGBA{0.30, 0.34, 0.33, 0.75})
	drawText(SmallFont, x+28, readoutY+8, hudText, "%s", trimOverlayText(ctrlState.LastGodMessage, int((w-70)/8)))
	for i, line := range ctrlState.InspectLines {
		if i >= 1 {
			break
		}
		drawText(SmallFont, x+28, readoutY+28, hudMuted, "%s", trimOverlayText(line, int((w-70)/8)))
	}
}

func drawPanel(winH, x, y, w, h float32, title string, accent hudRGBA) {
	drawRect(winH, x+4, y+5, w, h, hudRGBA{0.00, 0.00, 0.00, 0.22})
	drawRect(winH, x, y, w, h, hudRGBA{0.025, 0.030, 0.030, 0.72})
	drawRect(winH, x, y, w, 3, accent.withAlpha(0.92))
	drawRect(winH, x, y, 3, h, accent.withAlpha(0.68))
	drawRectBorder(winH, x, y, w, h, accent.withAlpha(0.36))
	drawText(SmallFont, x+14, y+8, accent, title)
}

func drawToolChip(winH, x, y, w, h float32, tool GodTool) {
	active := tool == ctrlState.ActiveTool
	accent := godToolColor(tool)
	bg := hudRGBA{0.08, 0.10, 0.10, 0.58}
	border := hudRGBA{0.30, 0.34, 0.33, 0.62}
	text := hudMuted
	if active {
		bg = accent.withAlpha(0.25)
		border = accent.withAlpha(0.95)
		text = hudText
	}
	drawRect(winH, x, y, w, h, bg)
	drawRect(winH, x, y, 4, h, accent.withAlpha(0.85))
	drawRectBorder(winH, x, y, w, h, border)
	drawText(SmallFont, x+10, y+6, text, "%s %s", godToolHotkey(tool), godToolDisplayLabel(tool))
}

func drawPill(winH, x, y, w, h float32, label string, accent hudRGBA, active bool) {
	bg := hudRGBA{0.05, 0.065, 0.065, 0.70}
	border := accent.withAlpha(0.42)
	if active {
		bg = accent.withAlpha(0.24)
		border = accent.withAlpha(0.92)
	}
	drawRect(winH, x, y, w, h, bg)
	drawRectBorder(winH, x, y, w, h, border)
	drawText(SmallFont, x+9, y+6, hudText, "%s", trimOverlayText(label, int((w-16)/8)))
}

func drawRect(winH, x, y, w, h float32, c hudRGBA) {
	gl.Disable(gl.TEXTURE_2D)
	gl.Color4f(c.r, c.g, c.b, c.a)
	gl.Begin(gl.QUADS)
	gl.Vertex2f(x, winH-y)
	gl.Vertex2f(x+w, winH-y)
	gl.Vertex2f(x+w, winH-y-h)
	gl.Vertex2f(x, winH-y-h)
	gl.End()
}

func drawRectBorder(winH, x, y, w, h float32, c hudRGBA) {
	gl.Disable(gl.TEXTURE_2D)
	gl.Color4f(c.r, c.g, c.b, c.a)
	gl.LineWidth(1)
	gl.Begin(gl.LINE_LOOP)
	gl.Vertex2f(x, winH-y)
	gl.Vertex2f(x+w, winH-y)
	gl.Vertex2f(x+w, winH-y-h)
	gl.Vertex2f(x, winH-y-h)
	gl.End()
}

func drawText(font *gltext.Font, x, y float32, c hudRGBA, format string, args ...interface{}) {
	gl.Color4f(c.r, c.g, c.b, c.a)
	_ = font.Printf(x, y, format, args...)
}

func (c hudRGBA) withAlpha(alpha float32) hudRGBA {
	c.a = alpha
	return c
}

func selectedColonyLabel() string {
	if godActions == nil {
		return "none"
	}
	return trimOverlayText(godActions.SelectedColonyLabel(), 18)
}

func densityModeLabel() string {
	if useDensityRendering() {
		return "Density 4x4"
	}
	return "Sprites"
}

func renderGameMasterOverlay(winH, x, y, w float32) {
	if gameState == nil || !gameState.GameMaster.Enabled {
		drawText(SmallFont, x, y, hudMuted, "GM sleeping")
		return
	}

	gm := gameState.GameMaster
	drawRect(winH, x, y-5, w, 1, hudDim.withAlpha(0.85))
	drawText(SmallFont, x, y+10, hudText, "GM %s  tick %d/%d", trimOverlayText(gm.Name, 16), gm.Tick, gm.Interval)
	drawText(SmallFont, x, y+32, hudMuted, "Obs C%d R%d F%d P%d W%d", gm.Colonies, gm.Resources, gm.Food, gm.Poison, gm.Water)
	if gm.LastEventKind == "" {
		drawText(SmallFont, x, y+54, hudText, "Event watching")
		return
	}
	drawText(SmallFont, x, y+54, hudText, "Event %s +%d @%d", trimOverlayText(gm.LastEventKind, 18), gm.LastApplied, gm.LastEventTick)
	drawText(SmallFont, x, y+76, hudMuted, "%s", trimOverlayText(gm.LastReason, int(w/8)))
	drawText(SmallFont, x, y+98, hudMuted, "%s", trimOverlayText(gm.LastThought, int(w/8)))
}

func trimOverlayText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	if maxLen <= 3 {
		return text[:maxLen]
	}
	return text[:maxLen-3] + "..."
}

func godToolColor(tool GodTool) hudRGBA {
	switch tool {
	case GodToolInspect:
		return hudRGBA{0.86, 0.88, 0.82, 1}
	case GodToolWater:
		return hudRGBA{0.16, 0.50, 1.00, 1}
	case GodToolPoison:
		return hudRGBA{0.82, 0.24, 1.00, 1}
	case GodToolFood:
		return hudRGBA{1.00, 0.18, 0.68, 1}
	case GodToolColony:
		return hudOrange
	case GodToolFreeze:
		return hudRGBA{0.38, 0.88, 1.00, 1}
	case GodToolUnfreeze:
		return hudRGBA{0.38, 0.96, 0.78, 1}
	case GodToolBless:
		return hudGreen
	case GodToolCurse:
		return hudRed
	case GodToolBuild:
		return hudRGBA{0.76, 0.80, 0.82, 1}
	default:
		return hudText
	}
}

func godToolHotkey(tool GodTool) string {
	switch tool {
	case GodToolInspect:
		return "1"
	case GodToolWater:
		return "2"
	case GodToolPoison:
		return "3"
	case GodToolFood:
		return "4"
	case GodToolColony:
		return "5"
	case GodToolFreeze:
		return "6"
	case GodToolUnfreeze:
		return "7"
	case GodToolBless:
		return "8"
	case GodToolCurse:
		return "9"
	case GodToolBuild:
		return "0"
	default:
		return "?"
	}
}

func godToolDisplayLabel(tool GodTool) string {
	switch tool {
	case GodToolUnfreeze:
		return "Thaw"
	case GodToolBuild:
		return "Wall"
	default:
		return tool.Label()
	}
}
