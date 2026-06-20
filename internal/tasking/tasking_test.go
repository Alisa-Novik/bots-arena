package tasking

import (
	"golab/internal/core"
	"golab/internal/util"
	"testing"
)

func TestProcessColonyTasksSkipsFlowFieldWithoutWaterPath(t *testing.T) {
	brd := core.NewBoard()
	ctrlPos := util.NewPos(20, 20)
	colony := core.NewColony(ctrlPos)
	ctrl := core.Controller{
		Pos:    ctrlPos,
		Colony: &colony,
	}

	for range 10 {
		ProcessColonyTasks(&ctrl, brd)
	}

	if colony.WaterPathFlowField != nil {
		t.Fatalf("water flow field was initialized without a path")
	}
}

func TestProcessColonyTasksDoesNotRecomputeStableWaterFlowField(t *testing.T) {
	brd := core.NewBoard()
	ctrlPos := util.NewPos(20, 20)
	colony := core.NewColony(ctrlPos)
	colony.PathToWater = []util.Position{util.NewPos(20, 21)}
	colony.WaterPathFlowField = []int16{42}
	ctrl := core.Controller{
		Pos:    ctrlPos,
		Colony: &colony,
	}

	for range 10 {
		ProcessColonyTasks(&ctrl, brd)
	}

	if len(colony.WaterPathFlowField) != 1 {
		t.Fatalf("stable water flow field length = %d, want 1", len(colony.WaterPathFlowField))
	}
	if colony.WaterPathFlowField[0] != 42 {
		t.Fatalf("stable water flow field was recomputed: first=%d, want 42", colony.WaterPathFlowField[0])
	}
}
