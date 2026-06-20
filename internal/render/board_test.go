package render

import (
	"golab/internal/core"
	"golab/internal/util"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestSaveBoardPNGPheromoneStyleRendersScent(t *testing.T) {
	brd := core.NewBoard()
	pos := util.NewPos(10, 10)
	brd.DepositPheromone(pos, core.PheromoneFood, 255, nil)
	out := filepath.Join(t.TempDir(), "pheromone.png")

	result, err := SaveBoardPNG(brd, Options{
		AtlasPath: filepath.Join("..", "..", "assests", "sprites", "atlas.png"),
		Output:    out,
		Style:     "pheromone",
		CellSize:  3,
		Padding:   2,
		Border:    true,
		Legend:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Width <= 0 || result.Height <= 0 {
		t.Fatalf("invalid render dimensions: %+v", result)
	}

	file, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	img, err := png.Decode(file)
	if err != nil {
		t.Fatal(err)
	}

	x := 2 + pos.C*3 + 1
	y := 2 + (core.Rows-1-pos.R)*3 + 1
	r, g, b, _ := img.At(x, y).RGBA()
	if r <= g || b <= g {
		t.Fatalf("pheromone pixel rgb16 = %d/%d/%d, want magenta-dominant food scent", r, g, b)
	}
}

func TestDepotHasRenderableVisual(t *testing.T) {
	tile, tint := visualFor(core.Depot{Food: 1, Ore: 2})

	if tile != tileChest {
		t.Fatalf("depot tile = %d, want chest tile %d", tile, tileChest)
	}
	if tint == clrGrey {
		t.Fatalf("depot tint = %+v, want visible non-empty tint", tint)
	}
}

func TestSaveBoardPNGBiomeStyleRendersBiomeTint(t *testing.T) {
	brd := core.NewBoard()
	fertile := firstRenderBiomeCell(t, brd, core.BiomeFertile)
	mineral := firstRenderBiomeCell(t, brd, core.BiomeMineral)
	out := filepath.Join(t.TempDir(), "biome.png")

	result, err := SaveBoardPNG(brd, Options{
		AtlasPath: filepath.Join("..", "..", "assests", "sprites", "atlas.png"),
		Output:    out,
		Style:     "biome",
		CellSize:  3,
		Padding:   2,
		Border:    true,
		Legend:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Width <= 0 || result.Height <= 0 {
		t.Fatalf("invalid render dimensions: %+v", result)
	}

	file, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	img, err := png.Decode(file)
	if err != nil {
		t.Fatal(err)
	}

	fertileColor := sampleRenderedCell(img, fertile, 3, 2)
	mineralColor := sampleRenderedCell(img, mineral, 3, 2)
	if fertileColor == mineralColor {
		t.Fatalf("biome style rendered fertile and mineral cells identically: %v", fertileColor)
	}
	if fertileColor.G <= fertileColor.R || mineralColor.R <= mineralColor.G {
		t.Fatalf("unexpected biome colors: fertile=%+v mineral=%+v", fertileColor, mineralColor)
	}
}

func TestSaveBoardPNGDensityStyleRendersBotChunks(t *testing.T) {
	brd := core.NewBoard()
	firstPos := util.NewPos(12, 12)
	secondPos := util.NewPos(13, 13)
	first := core.NewBot(firstPos)
	second := core.NewBot(secondPos)
	first.Color = [3]float32{0.9, 0.1, 0.1}
	second.Color = [3]float32{0.9, 0.1, 0.1}
	brd.AddBot(firstPos, &first)
	brd.AddBot(secondPos, &second)
	out := filepath.Join(t.TempDir(), "density.png")

	result, err := SaveBoardPNG(brd, Options{
		AtlasPath: filepath.Join("..", "..", "assests", "sprites", "atlas.png"),
		Output:    out,
		Style:     "density",
		CellSize:  3,
		Padding:   2,
		Border:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Width <= 0 || result.Height <= 0 {
		t.Fatalf("invalid render dimensions: %+v", result)
	}

	file, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	img, err := png.Decode(file)
	if err != nil {
		t.Fatal(err)
	}

	denseColor := sampleRenderedCell(img, firstPos, 3, 2)
	emptyColor := sampleRenderedCell(img, util.NewPos(80, 80), 3, 2)
	if denseColor == emptyColor {
		t.Fatalf("density chunk color matched empty cell: dense=%+v empty=%+v", denseColor, emptyColor)
	}
	if denseColor.R <= denseColor.G {
		t.Fatalf("density chunk did not preserve bot color bias: %+v", denseColor)
	}
}

func firstRenderBiomeCell(t *testing.T, brd *core.Board, biome core.Biome) util.Position {
	t.Helper()
	for idx := 0; idx < util.Cells; idx++ {
		pos := util.PosOf(idx)
		if pos.R <= 0 || pos.R >= core.Rows-1 {
			continue
		}
		if brd.BiomeAt(pos) == biome {
			return pos
		}
	}
	t.Fatalf("no %s biome cell found", biome)
	return util.Position{}
}

func sampleRenderedCell(img image.Image, pos util.Position, cellSize, padding int) color.RGBA {
	x := padding + pos.C*cellSize + cellSize/2
	y := padding + (core.Rows-1-pos.R)*cellSize + cellSize/2
	r, g, b, a := img.At(x, y).RGBA()
	return color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
}
