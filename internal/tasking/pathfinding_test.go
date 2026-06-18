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
