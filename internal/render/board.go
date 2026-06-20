package render

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"

	"golab/internal/core"
	"golab/internal/util"
)

const (
	tileFood = iota
	tileWall
	tileOre
	tilePoison
	tileChest
	tileFarm
	tileSpawner
	tileLight
	tileBot
	tileDark
	tileFlag
)

const densityChunkSize = 4

var (
	pageBackground = color.RGBA{R: 13, G: 17, B: 23, A: 255}
	boardBorder    = color.RGBA{R: 190, G: 200, B: 210, A: 255}
	clrGrey        = [3]float32{0.2, 0.2, 0.2}
	clrWhite       = [3]float32{1, 1, 1}
)

type Options struct {
	AtlasPath string
	Output    string

	CellSize int
	Padding  int

	Border bool
	Legend bool

	Style string

	RenderPaths        bool
	RenderTaskTargets  bool
	RenderUnreachables bool
}

type Result struct {
	Output string `json:"output"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

func SaveBoardPNG(brd *core.Board, opts Options) (Result, error) {
	if opts.AtlasPath == "" {
		opts.AtlasPath = "assests/sprites/atlas.png"
	}
	if opts.Output == "" {
		opts.Output = "golab-render.png"
	}
	if opts.CellSize <= 0 {
		opts.CellSize = 2
	}
	if opts.Padding < 0 {
		opts.Padding = 0
	}
	if opts.Style == "" {
		opts.Style = "game"
	}
	if opts.Style != "flat" && opts.Style != "atlas" && opts.Style != "game" && opts.Style != "pheromone" && opts.Style != "biome" && opts.Style != "density" && opts.Style != "colony" {
		return Result{}, fmt.Errorf("unknown render style %q: use flat, atlas, game, pheromone, biome, density, or colony", opts.Style)
	}

	atlas, err := loadAtlas(opts.AtlasPath)
	if err != nil {
		return Result{}, err
	}

	tileSize := atlas.Bounds().Dy()
	if tileSize <= 0 || atlas.Bounds().Dx()%tileSize != 0 {
		return Result{}, fmt.Errorf("atlas must contain square tiles in a single row: %s", opts.AtlasPath)
	}

	boardW := core.Cols * opts.CellSize
	boardH := core.Rows * opts.CellSize
	legendH := 0
	if opts.Legend {
		legendH = opts.Padding + 24
	}

	img := image.NewRGBA(image.Rect(0, 0, boardW+opts.Padding*2, boardH+opts.Padding*2+legendH))
	draw.Draw(img, img.Bounds(), &image.Uniform{pageBackground}, image.Point{}, draw.Src)

	boardRect := image.Rect(opts.Padding, opts.Padding, opts.Padding+boardW, opts.Padding+boardH)
	drawBoard(img, boardRect, brd, atlas, tileSize, opts)

	if opts.Border {
		drawBorder(img, boardRect)
	}
	if opts.Legend {
		drawLegend(img, opts.Padding, boardRect.Max.Y+opts.Padding, opts.CellSize, atlas, tileSize, opts.Style)
	}

	file, err := os.Create(opts.Output)
	if err != nil {
		return Result{}, err
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		return Result{}, err
	}

	return Result{Output: opts.Output, Width: img.Bounds().Dx(), Height: img.Bounds().Dy()}, nil
}

func loadAtlas(path string) (*image.RGBA, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	src, _, err := image.Decode(file)
	if err != nil {
		return nil, err
	}

	bounds := src.Bounds()
	atlas := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(atlas, atlas.Bounds(), src, bounds.Min, draw.Src)
	return atlas, nil
}

func drawBoard(dst *image.RGBA, rect image.Rectangle, brd *core.Board, atlas *image.RGBA, tileSize int, opts Options) {
	for row := 0; row < core.Rows; row++ {
		for col := 0; col < core.Cols; col++ {
			pos := util.Position{R: row, C: col}
			occupant := brd.At(pos)
			if opts.Style == "density" {
				if _, ok := occupant.(*core.Bot); ok {
					occupant = nil
				}
			}
			tile, tint := visualFor(occupant)
			if opts.Style == "pheromone" {
				tile, tint = pheromoneVisual(brd.PheromoneAt(pos))
			}
			if opts.Style == "biome" {
				tile, tint = biomeVisual(brd.At(pos), brd.BiomeAt(pos), tile, tint)
			}
			if opts.Style == "colony" {
				tile, tint = colonyVisual(brd, pos, occupant, tile, tint)
			}
			if opts.Style != "pheromone" && opts.RenderPaths && brd.IsPathToRender(pos) {
				tile, tint = tileLight, util.CyanColor()
			}
			if opts.Style != "pheromone" && opts.RenderTaskTargets && brd.IsTaskTargetToRender(pos) {
				tile, tint = tileLight, util.PinkColor()
			}
			if opts.Style != "pheromone" && opts.RenderUnreachables && brd.IsUnreachableToRender(pos) {
				tile, tint = tileLight, util.RedColor()
			}
			if brd.IsFrozen(pos) {
				tile, tint = tileLight, frozenTint(tint)
			}

			x0 := rect.Min.X + col*opts.CellSize
			y0 := rect.Min.Y + (core.Rows-1-row)*opts.CellSize
			drawCell(dst, image.Rect(x0, y0, x0+opts.CellSize, y0+opts.CellSize), atlas, tileSize, tile, tint, opts.Style)
		}
	}
	if opts.Style == "density" {
		drawDensityOverlay(dst, rect, brd, opts.CellSize)
	}
}

func frozenTint(tint [3]float32) [3]float32 {
	return [3]float32{
		(tint[0] + 0.35) / 2,
		(tint[1] + 0.9) / 2,
		(tint[2] + 1.0) / 2,
	}
}

func visualFor(o core.Occupant) (int, [3]float32) {
	switch o := o.(type) {
	case *core.Bot:
		if o.IsSelected {
			return tileBot, util.YellowColor()
		}
		return tileBot, colonyBotColor(o, o.Color)
	case core.Food:
		return tileFood, [3]float32{1, 0, 0.8}
	case core.Water:
		return tileLight, [3]float32{0, 0, 0.8}
	case core.Organics:
		return tileLight, [3]float32{0, 0.8, 0}
	case core.Building:
		return tileWall, clrWhite
	case core.Wall:
		return tileWall, clrWhite
	case core.Controller:
		return tileChest, clrWhite
	case *core.Controller:
		return tileChest, clrWhite
	case core.Depot:
		return tileChest, [3]float32{0.20, 0.95, 0.85}
	case *core.Depot:
		return tileChest, [3]float32{0.20, 0.95, 0.85}
	case core.Mine:
		return tileSpawner, clrWhite
	case core.Resource:
		return tileOre, clrWhite
	case core.Farm:
		return tileFarm, clrWhite
	case core.Poison:
		return tilePoison, clrWhite
	case core.ColonyFlag:
		return tileFlag, clrWhite
	case nil:
		return tileDark, clrGrey
	default:
		return tileDark, clrGrey
	}
}

func colonyVisual(brd *core.Board, pos core.Position, o core.Occupant, tile int, tint [3]float32) (int, [3]float32) {
	switch v := o.(type) {
	case *core.Bot:
		return tileBot, colonyModeBotColor(v)
	case core.Water:
		return tileLight, [3]float32{0.02, 0.04, 0.10}
	case core.Controller:
		return tileChest, colonyStructureColor(v.Colony, tint)
	case *core.Controller:
		if v != nil {
			return tileChest, colonyStructureColor(v.Colony, tint)
		}
	case core.Depot:
		return tileChest, colonyStructureColor(v.Colony, tint)
	case *core.Depot:
		if v != nil {
			return tileChest, colonyStructureColor(v.Colony, tint)
		}
	case core.Farm:
		return tileFarm, colonyStructureColor(colonyForOwnedCell(v.Colony, v.Owner), tint)
	case core.Spawner:
		return tileSpawner, colonyStructureColor(colonyForOwner(v.Owner), tint)
	case core.ColonyFlag:
		return tileFlag, colonyStructureColor(brd.PheromoneHomeOwnerAt(pos), tint)
	case nil:
		if owner := brd.PheromoneHomeOwnerAt(pos); owner != nil && brd.PheromoneAt(pos).Home > 0 {
			return tileLight, colonyTissueColor(owner.Color, brd.PheromoneAt(pos).Home)
		}
		return tileDark, clrGrey
	}
	return tile, lerpColor(clrGrey, tint, 0.18)
}

func colonyBotColor(bot *core.Bot, base [3]float32) [3]float32 {
	if bot == nil || bot.Colony == nil {
		return base
	}
	if bot.ConnnectedToColony {
		return lerpColor(base, bot.Colony.Color, 0.82)
	}
	return lerpColor(clrGrey, bot.Colony.Color, 0.28)
}

func colonyModeBotColor(bot *core.Bot) [3]float32 {
	if bot == nil || bot.Colony == nil {
		return [3]float32{0.22, 0.24, 0.24}
	}
	if bot.ConnnectedToColony {
		return lerpColor(bot.Colony.Color, clrWhite, 0.16)
	}
	return lerpColor(clrGrey, bot.Colony.Color, 0.34)
}

func colonyStructureColor(colony *core.Colony, fallback [3]float32) [3]float32 {
	if colony == nil {
		return fallback
	}
	return lerpColor(fallback, colony.Color, 0.92)
}

func colonyTissueColor(colonyColor [3]float32, home uint8) [3]float32 {
	intensity := 0.36 + 0.46*clamp01(float32(home)/255)
	return lerpColor(clrGrey, colonyColor, intensity)
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

func drawCell(dst *image.RGBA, rect image.Rectangle, atlas *image.RGBA, tileSize, tile int, tint [3]float32, style string) {
	if style == "pheromone" || style == "density" {
		draw.Draw(dst, rect, &image.Uniform{flatColor(tile, tint)}, image.Point{}, draw.Src)
		return
	}
	if style == "flat" && tile != tileBot {
		draw.Draw(dst, rect, &image.Uniform{flatColor(tile, tint)}, image.Point{}, draw.Src)
		return
	}
	drawTile(dst, rect, atlas, tileSize, tile, tint)
}

func drawDensityOverlay(dst *image.RGBA, rect image.Rectangle, brd *core.Board, cellSize int) {
	chunkRows := (core.Rows + densityChunkSize - 1) / densityChunkSize
	chunkCols := (core.Cols + densityChunkSize - 1) / densityChunkSize
	counts := make([]int, chunkRows*chunkCols)
	sums := make([][3]float32, chunkRows*chunkCols)
	touched := make([]int, 0, len(counts))

	for _, id := range brd.ActiveBotIDs() {
		bot := brd.BotByID(id)
		if bot == nil {
			continue
		}
		cell := brd.BotCell(id)
		if cell < 0 {
			continue
		}
		row := cell / core.Cols
		col := cell % core.Cols
		chunkIdx := (row/densityChunkSize)*chunkCols + col/densityChunkSize
		if counts[chunkIdx] == 0 {
			touched = append(touched, chunkIdx)
		}
		counts[chunkIdx]++
		color := bot.Color
		sums[chunkIdx][0] += color[0]
		sums[chunkIdx][1] += color[1]
		sums[chunkIdx][2] += color[2]
	}

	chunkArea := densityChunkSize * densityChunkSize
	for _, chunkIdx := range touched {
		count := counts[chunkIdx]
		chunkRow := chunkIdx / chunkCols
		chunkCol := chunkIdx % chunkCols
		row := chunkRow * densityChunkSize
		col := chunkCol * densityChunkSize
		chunkH := min(densityChunkSize, core.Rows-row)
		chunkW := min(densityChunkSize, core.Cols-col)
		x0 := rect.Min.X + col*cellSize
		y0 := rect.Min.Y + (core.Rows-row-chunkH)*cellSize
		chunkRect := image.Rect(x0, y0, x0+chunkW*cellSize, y0+chunkH*cellSize)
		draw.Draw(dst, chunkRect, &image.Uniform{flatColor(tileLight, densityColor(count, sums[chunkIdx], chunkArea))}, image.Point{}, draw.Over)
	}
}

func densityColor(count int, sum [3]float32, chunkArea int) [3]float32 {
	if count <= 0 {
		return clrGrey
	}
	invCount := 1 / float32(count)
	avg := [3]float32{sum[0] * invCount, sum[1] * invCount, sum[2] * invCount}
	occupancy := clamp01(float32(count) / float32(max(1, chunkArea)))
	return lerpColor(lerpColor(clrGrey, avg, 0.68), clrWhite, occupancy*0.22)
}

func pheromoneVisual(values core.PheromoneValues) (int, [3]float32) {
	if values.IsZero() {
		return tileDark, clrGrey
	}
	return tileLight, pheromoneColor(values)
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
		return clrGrey
	}
	intensity := 0.25 + 0.75*clamp01(maxValue/255)
	return [3]float32{
		clamp01(out[0] / sum * intensity),
		clamp01(out[1] / sum * intensity),
		clamp01(out[2] / sum * intensity),
	}
}

func biomeVisual(o core.Occupant, biome core.Biome, tile int, tint [3]float32) (int, [3]float32) {
	biomeTint := biomeColor(biome)
	switch o.(type) {
	case nil:
		return tileLight, biomeTint
	case core.Food, core.Organics:
		return tile, lerpColor(tint, biomeTint, 0.28)
	case core.Water:
		return tile, lerpColor(tint, biomeTint, 0.16)
	default:
		return tile, tint
	}
}

func biomeColor(biome core.Biome) [3]float32 {
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

func lerpColor(a, b [3]float32, t float32) [3]float32 {
	return [3]float32{
		a[0] + (b[0]-a[0])*t,
		a[1] + (b[1]-a[1])*t,
		a[2] + (b[2]-a[2])*t,
	}
}

func drawTile(dst *image.RGBA, rect image.Rectangle, atlas *image.RGBA, tileSize, tile int, tint [3]float32) {
	srcX := tile * tileSize
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		sy := (y - rect.Min.Y) * tileSize / rect.Dy()
		for x := rect.Min.X; x < rect.Max.X; x++ {
			sx := srcX + (x-rect.Min.X)*tileSize/rect.Dx()
			src := atlas.RGBAAt(sx, sy)
			r := uint8(float32(src.R) * tint[0])
			g := uint8(float32(src.G) * tint[1])
			b := uint8(float32(src.B) * tint[2])
			dst.SetRGBA(x, y, over(dst.RGBAAt(x, y), color.RGBA{R: r, G: g, B: b, A: src.A}))
		}
	}
}

func flatColor(tile int, tint [3]float32) color.RGBA {
	switch tile {
	case tileDark:
		return color.RGBA{R: 38, G: 49, B: 40, A: 255}
	case tileLight:
		switch tint {
		case util.CyanColor():
			return color.RGBA{R: 66, G: 190, B: 205, A: 255}
		case util.PinkColor():
			return color.RGBA{R: 202, G: 113, B: 157, A: 255}
		case util.RedColor():
			return color.RGBA{R: 190, G: 62, B: 60, A: 255}
		case [3]float32{0, 0, 0.8}:
			return color.RGBA{R: 25, G: 93, B: 126, A: 255}
		case [3]float32{0, 0.8, 0}:
			return color.RGBA{R: 91, G: 138, B: 89, A: 255}
		default:
			return color.RGBA{R: toByte(tint[0]), G: toByte(tint[1]), B: toByte(tint[2]), A: 255}
		}
	case tileWall, tileChest, tileSpawner:
		return color.RGBA{R: 86, G: 84, B: 79, A: 255}
	case tileOre:
		return color.RGBA{R: 196, G: 172, B: 70, A: 255}
	case tilePoison:
		return color.RGBA{R: 137, G: 63, B: 108, A: 255}
	case tileFood:
		return color.RGBA{R: 210, G: 66, B: 152, A: 255}
	case tileFarm, tileFlag:
		return color.RGBA{R: 91, G: 138, B: 89, A: 255}
	default:
		return color.RGBA{R: toByte(tint[0]), G: toByte(tint[1]), B: toByte(tint[2]), A: 255}
	}
}

func toByte(v float32) uint8 {
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return 255
	}
	return uint8(v * 255)
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

func over(bg, fg color.RGBA) color.RGBA {
	if fg.A == 255 {
		return fg
	}
	if fg.A == 0 {
		return bg
	}
	a := uint32(fg.A)
	inv := 255 - a
	return color.RGBA{
		R: uint8((uint32(fg.R)*a + uint32(bg.R)*inv) / 255),
		G: uint8((uint32(fg.G)*a + uint32(bg.G)*inv) / 255),
		B: uint8((uint32(fg.B)*a + uint32(bg.B)*inv) / 255),
		A: 255,
	}
}

func drawBorder(dst *image.RGBA, rect image.Rectangle) {
	for x := rect.Min.X - 1; x <= rect.Max.X; x++ {
		setSafe(dst, x, rect.Min.Y-1, boardBorder)
		setSafe(dst, x, rect.Max.Y, boardBorder)
	}
	for y := rect.Min.Y - 1; y <= rect.Max.Y; y++ {
		setSafe(dst, rect.Min.X-1, y, boardBorder)
		setSafe(dst, rect.Max.X, y, boardBorder)
	}
}

func drawLegend(dst *image.RGBA, x, y, cellSize int, atlas *image.RGBA, tileSize int, style string) {
	size := max(14, cellSize*7)
	gap := size + 18
	if style == "pheromone" {
		items := []core.PheromoneValues{
			{Food: 255},
			{Ore: 255},
			{Home: 255},
			{Danger: 255},
		}
		for i, item := range items {
			tile, tint := pheromoneVisual(item)
			x0 := x + i*gap
			drawCell(dst, image.Rect(x0, y, x0+size, y+size), atlas, tileSize, tile, tint, "pheromone")
		}
		return
	}
	if style == "biome" {
		items := []struct {
			tile int
			tint [3]float32
		}{
			{tileLight, biomeColor(core.BiomeNeutral)},
			{tileLight, biomeColor(core.BiomeFertile)},
			{tileLight, biomeColor(core.BiomeMineral)},
			{tileLight, biomeColor(core.BiomeToxic)},
			{tileLight, [3]float32{0, 0, 0.8}},
			{tileOre, clrWhite},
			{tileFood, [3]float32{1, 0, 0.8}},
			{tileBot, util.LightBlueColor()},
		}
		for i, item := range items {
			x0 := x + i*gap
			drawCell(dst, image.Rect(x0, y, x0+size, y+size), atlas, tileSize, item.tile, item.tint, "flat")
		}
		return
	}
	items := []struct {
		tile int
		tint [3]float32
	}{
		{tileDark, clrGrey},
		{tileLight, [3]float32{0, 0, 0.8}},
		{tileWall, clrWhite},
		{tilePoison, clrWhite},
		{tileOre, clrWhite},
		{tileLight, [3]float32{0, 0.8, 0}},
		{tileBot, util.LightBlueColor()},
		{tileBot, util.YellowColor()},
	}
	for i, item := range items {
		x0 := x + i*gap
		drawCell(dst, image.Rect(x0, y, x0+size, y+size), atlas, tileSize, item.tile, item.tint, "flat")
	}
}

func setSafe(dst *image.RGBA, x, y int, c color.RGBA) {
	if image.Pt(x, y).In(dst.Bounds()) {
		dst.SetRGBA(x, y, c)
	}
}
