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
	if opts.Style != "flat" && opts.Style != "atlas" && opts.Style != "game" {
		return Result{}, fmt.Errorf("unknown render style %q: use flat, atlas, or game", opts.Style)
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
		drawLegend(img, opts.Padding, boardRect.Max.Y+opts.Padding, opts.CellSize, atlas, tileSize)
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
			tile, tint := visualFor(brd.At(pos))
			if opts.RenderPaths && brd.IsPathToRender(pos) {
				tile, tint = tileLight, util.CyanColor()
			}
			if opts.RenderTaskTargets && brd.IsTaskTargetToRender(pos) {
				tile, tint = tileLight, util.PinkColor()
			}
			if opts.RenderUnreachables && brd.IsUnreachableToRender(pos) {
				tile, tint = tileLight, util.RedColor()
			}

			x0 := rect.Min.X + col*opts.CellSize
			y0 := rect.Min.Y + (core.Rows-1-row)*opts.CellSize
			drawCell(dst, image.Rect(x0, y0, x0+opts.CellSize, y0+opts.CellSize), atlas, tileSize, tile, tint, opts.Style)
		}
	}
}

func visualFor(o core.Occupant) (int, [3]float32) {
	switch o := o.(type) {
	case *core.Bot:
		if o.IsSelected {
			return tileBot, util.YellowColor()
		}
		return tileBot, o.Color
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

func drawCell(dst *image.RGBA, rect image.Rectangle, atlas *image.RGBA, tileSize, tile int, tint [3]float32, style string) {
	if style == "flat" && tile != tileBot {
		draw.Draw(dst, rect, &image.Uniform{flatColor(tile, tint)}, image.Point{}, draw.Src)
		return
	}
	drawTile(dst, rect, atlas, tileSize, tile, tint)
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

func drawLegend(dst *image.RGBA, x, y, cellSize int, atlas *image.RGBA, tileSize int) {
	size := max(14, cellSize*7)
	gap := size + 18
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
