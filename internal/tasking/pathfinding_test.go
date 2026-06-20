package tasking

import (
	"math"
	"testing"

	"golab/internal/core"
	"golab/internal/util"
)

func TestCalcPathUsesWrappedColumnDistance(t *testing.T) {
	start := util.NewPos(10, 1)
	end := util.NewPos(10, util.Cols-2)

	path := CalcPath(start, end, func(pos util.Position) bool {
		return !util.OutOfBounds(pos)
	}, nil)

	if got, want := len(path), 3; got != want {
		t.Fatalf("path length = %d, want %d; path=%v", got, want, path)
	}
	want := []util.Position{
		util.NewPos(10, 0),
		util.NewPos(10, util.Cols-1),
		end,
	}
	for i := range want {
		if path[i] != want[i] {
			t.Fatalf("path[%d] = %v, want %v; path=%v", i, path[i], want[i], path)
		}
	}
	if path[len(path)-1] != end {
		t.Fatalf("last path position = %v, want %v", path[len(path)-1], end)
	}
}

func TestCalcFlowFieldTreatsBotsAsPassable(t *testing.T) {
	brd := core.NewBoard()
	source := util.NewPos(10, 10)
	botPos := util.NewPos(10, 11)
	bot := core.NewBot(botPos)
	brd.Bots[util.Idx(botPos)] = &bot
	brd.Set(botPos, &bot)

	field := CalcFlowField([]util.Position{source}, brd)

	if got := field[util.Idx(botPos)]; got == math.MaxInt16 {
		t.Fatalf("bot cell should be reachable in flow field")
	} else if got != 1 {
		t.Fatalf("bot cell distance = %d, want 1", got)
	}
}

func BenchmarkCalcPath(b *testing.B) {
	start := util.NewPos(10, 10)
	end := util.NewPos(util.Rows-10, util.Cols-10)
	passable := func(pos util.Position) bool {
		return !util.OutOfBounds(pos)
	}

	b.ReportAllocs()
	for range b.N {
		path := CalcPath(start, end, passable, nil)
		if len(path) == 0 {
			b.Fatal("expected path")
		}
	}
}

func BenchmarkCalcFlowField(b *testing.B) {
	brd := core.NewBoard()
	sources := []util.Position{
		util.NewPos(10, 10),
		util.NewPos(20, 20),
		util.NewPos(30, 30),
	}

	b.ReportAllocs()
	for range b.N {
		field := CalcFlowField(sources, brd)
		if len(field) != util.Cells {
			b.Fatalf("flow field length = %d, want %d", len(field), util.Cells)
		}
	}
}
