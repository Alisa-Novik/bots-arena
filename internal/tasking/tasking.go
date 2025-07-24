package tasking

import (
	"fmt"
	"golab/internal/assert"
	"golab/internal/core"
	"golab/internal/util"
	"sort"
	"time"
)

func ProcessColonyTasks(ctrl *core.Controller, brd *core.Board) {
	c := ctrl.Colony
	now := time.Now()

	c.Counter++
	if c.Counter%5 == 0 {
		c.WaterPathFlowField = CalcFlowField(c.PathToWater, brd)
	}

	for _, task := range c.Tasks {
		switch task.Status {
		case core.NotStarted:
			fmt.Println("Task is done.")
			continue
		case core.InProgress:
			fmt.Println("Task is done.")
			continue
		case core.Done:
			fmt.Println("Task is done.")
			continue
	}

	for _, task := range c.Tasks {
		if task.Type != core.ConnectToPosTask || task.IsDone {
			continue
		}
		task.Attempts++
		if task.Attempts > 1 {
			fmt.Printf("Unable to find pas to %v\n", task.Pos)
			task.MarkDone()
			return
		}

		// c.PathToWater = CalcPath(ctrl.Pos, task.Pos, brd.IsEmptyOrBot, nil)
		c.PathToWater = CalcPath(ctrl.Pos, util.NewPos(1, 1), brd.IsEmptyOrBot, nil)
		pathLen := len(c.PathToWater)
		if pathLen == 0 {
			return
		}
		// remove water tile itself
		c.PathToWater = c.PathToWater[:pathLen-1]
		brd.PathsToRenderR = append(brd.PathsToRenderR, c.PathToWater...)
		if len(c.PathToWater) == 0 {
			continue
		}
		c.WaterPathFlowField = CalcFlowField(c.PathToWater, brd)
		for _, pos := range c.PathToWater {
			c.Tasks = append(c.Tasks, c.NewMaintainConnectionTask(pos, &c.WaterPathFlowField))
		}
		task.MarkDone()
		continue
	}

	for _, task := range c.Tasks {
		if task.Type != core.MaintainConnectionTask || task.IsDone {
			continue
		}
		if b := brd.GetBot(task.Pos); b != nil {
			if task.HasOwner() || b.Colony != c {
				continue
			}
			if old := b.CurrTask; old != nil {
				b.UnassignTask(now)
			}
			b.AssignTask(task)
			b.CurrTask.MarkDone()
			continue
		}
		if task.HasOwner() && task.IsExpired(now) {
			task.Owner.UnassignTask(now)
			continue
		}
		if task.HasOwner() {
			continue
		}
		farPos := farthestPos(c.WaterPositions, ctrl.Pos)
		for _, b := range SortedFreeBots(c.Members, farPos, now) {
			if b.HasTask() || b.HasCooldown(now) {
				continue
			}
			b.AssignTask(task)
			break
		}
	}
}

func SortedByDist(tasks []*core.ColonyTask, target util.Position) []*core.ColonyTask {
	type pair struct {
		t    *core.ColonyTask
		dist int
	}

	pairs := make([]pair, 0, len(tasks))
	for _, b := range tasks {
		assert.Assert(b != nil, "nil in task list")
		dr := b.Pos.R - target.R
		dc := b.Pos.C - target.C
		pairs = append(pairs, pair{t: b, dist: dr*dr + dc*dc})
	}

	sort.Slice(pairs, func(i, j int) bool { return pairs[i].dist < pairs[j].dist })

	out := make([]*core.ColonyTask, len(pairs))
	for i := range pairs {
		out[i] = pairs[i].t
	}
	return out
}

func SortedFreeBots(members []*core.Bot, target util.Position, now time.Time) []*core.Bot {
	type pair struct {
		b    *core.Bot
		dist int
	}

	pairs := make([]pair, 0, len(members))
	for _, b := range members {
		if b == nil || b.HasTask() || b.HasCooldown(now) {
			continue
		}
		dr := b.Pos.R - target.R
		dc := b.Pos.C - target.C
		pairs = append(pairs, pair{b: b, dist: dr*dr + dc*dc})
	}

	sort.Slice(pairs, func(i, j int) bool { return pairs[i].dist < pairs[j].dist })

	out := make([]*core.Bot, len(pairs))
	for i := range pairs {
		out[i] = pairs[i].b
	}
	return out
}

func farthestPos(path []util.Position, origin util.Position) util.Position {
	var res util.Position
	max := -1
	for _, p := range path {
		dr := p.R - origin.R
		dc := p.C - origin.C
		d := dr*dr + dc*dc
		if d > max {
			max, res = d, p
		}
	}
	return res
}
