package core

import (
	"golab/internal/util"
	"math/rand"
	"testing"
	"time"
)

func TestNewChildFullyInitializesPooledBotAndLinksLineage(t *testing.T) {
	rand.Seed(1)

	parent := NewBot(util.NewPos(12, 12))
	colony := NewColony(parent.Pos)
	colony.AddFamily(&parent)
	parent.ConnnectedToColony = true

	for i := 0; i < 2000; i++ {
		staleOffspring := NewBot(util.NewPos(1, 1))
		BotPool.Put(&Bot{
			Inventory:          Inventory{Food: 44, Ore: 55},
			Colony:             &Colony{},
			ConnnectedToColony: true,
			Parent:             &staleOffspring,
			Offsprings:         map[*Bot]struct{}{&staleOffspring: {}},
			Divisions:          99,
			LineageDepth:       99,
			Age:                99,
			Hp:                 -1,
			Color:              [3]float32{0.9, 0.8, 0.7},
			PrevColor:          [3]float32{0.6, 0.5, 0.4},
			IsSelected:         true,
			HasSpawner:         true,
			Pos:                util.NewPos(2, 2),
			CurrTask:           &ColonyTask{},
			CooldownUntil:      time.Now().Add(time.Hour),
		})

		childPos := util.NewPos(20+i%10, 30+i%20)
		child := parent.NewChild(childPos, false)

		if child == nil {
			t.Fatal("child is nil")
		}
		if child.Pos != childPos {
			t.Fatalf("child position = %v, want %v", child.Pos, childPos)
		}
		if child.Parent != &parent {
			t.Fatalf("child parent = %p, want %p", child.Parent, &parent)
		}
		if child.Divisions != 0 {
			t.Fatalf("child divisions = %d, want 0", child.Divisions)
		}
		if child.LineageDepth != parent.LineageDepth+1 {
			t.Fatalf("child lineage depth = %d, want %d", child.LineageDepth, parent.LineageDepth+1)
		}
		if child.Age != 0 {
			t.Fatalf("child age = %d, want 0", child.Age)
		}
		if _, ok := parent.Offsprings[child]; !ok {
			t.Fatalf("parent does not contain child in offspring map")
		}
		if child.Colony != parent.Colony {
			t.Fatalf("child colony = %p, want parent colony %p", child.Colony, parent.Colony)
		}
		if !colonyContains(parent.Colony, child) {
			t.Fatalf("parent colony does not contain child")
		}
		if child.ConnnectedToColony {
			t.Fatalf("child should start disconnected from colony")
		}
		if child.Inventory.Total() != 0 {
			t.Fatalf("child inventory = %+v, want empty", child.Inventory)
		}
		if child.Hp != botHp {
			t.Fatalf("child hp = %d, want %d", child.Hp, botHp)
		}
		if child.CurrTask != nil {
			t.Fatalf("child task = %v, want nil", child.CurrTask)
		}
		if !child.CooldownUntil.IsZero() {
			t.Fatalf("child cooldown = %v, want zero", child.CooldownUntil)
		}
		if child.IsSelected {
			t.Fatalf("child should not inherit selection")
		}
		if child.HasSpawner {
			t.Fatalf("child should not inherit spawner state")
		}
		if len(child.Offsprings) != 0 {
			t.Fatalf("child offspring count = %d, want 0", len(child.Offsprings))
		}
		if child.Color != parent.Color {
			t.Fatalf("child color = %v, want parent color %v", child.Color, parent.Color)
		}
		if child.PrevColor != child.Color {
			t.Fatalf("child previous color = %v, want %v", child.PrevColor, child.Color)
		}
	}
}

func colonyContains(colony *Colony, child *Bot) bool {
	if colony == nil {
		return false
	}
	for _, member := range colony.Members {
		if member == child {
			return true
		}
	}
	return false
}
