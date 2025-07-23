package tasking

import (
	"fmt"
	"golab/internal/core"
	"golab/internal/util"
	"sort"
	"time"
)

func ProcessColonyTasks(ctrl *core.Controller, brd *core.Board) {
	c := ctrl.Colony
	now := time.Now()

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

		c.PathToWater = CalcPath(ctrl.Pos, task.Pos, brd.IsEmptyOrBot, nil)
		pathLen := len(c.PathToWater)
		if pathLen == 0 {
			return
		}
		// remove water tile itself
		c.PathToWater = c.PathToWater[:pathLen-1]

		for _, pathPos := range c.PathToWater {
			c.AddTask(c.NewMaintainConnectionTask(pathPos))
			brd.PathsToRenderR = append(brd.PathsToRenderR, pathPos)
			brd.MarkDirty(util.Idx(pathPos))
		}

		task.MarkDone()
		continue
	}

	for _, task := range SortedByDist(c.Tasks, ctrl.Pos) {
		if task.Type != core.MaintainConnectionTask || task.IsDone {
			continue
		}
		if b := brd.GetBot(task.Pos); b != nil {
			if task.HasOwner() || b.Colony != c {
				continue
			}
			if old := b.CurrTask; old != nil {
				b.UnassignTask()
			}
			b.AssignTask(task)
			b.Path = nil
			b.CurrTask.MarkDone()
			continue
		}
		if task.HasOwner() && task.IsExpired(now) {
			task.Owner.StartCooldown(now)
			task.Owner.UnassignTask()
			continue
		}
		if task.HasOwner() || c.HasNoFreeMembers() {
			continue
		}
		if ctrl.Colony.AssignedUndoneTasksCount() > 10 {
			continue
		}
		for _, b := range SortedFreeBots(c.Members, task.Pos, now) {
			if b.HasTask() || b.HasCooldown(now) {
				continue
			}
			path := CalcPath(b.Pos, task.Pos, brd.IsEmpty, brd.IsSurrounded)
			if len(path) == 0 {
				b.StartCooldown(now)
				continue
			}
			b.AssignTask(task)
			b.Path = path
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
		assert(b != nil, "nil in task list")
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

func assert(cond bool, msg string) {
	if !cond {
		panic(msg)
	}
}
