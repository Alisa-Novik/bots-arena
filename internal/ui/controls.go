package ui

import (
	"fmt"
	"golab/internal/core"
	"golab/internal/util"
	"time"

	"github.com/go-gl/glfw/v3.3/glfw"
)

type ControlState struct {
	Dragging         bool
	HoveredIdx       int
	LastClickIdx     int
	LeftCtrlPressed  bool
	LeftShiftPressed bool
}

func highlightRect(brd *core.Board, fromIdx, toIdx int) {
	if fromIdx < 0 || toIdx < 0 {
		return
	}
	r1, c1 := fromIdx/core.Cols, fromIdx%core.Cols
	r2, c2 := toIdx/core.Cols, toIdx%core.Cols
	if r1 > r2 {
		r1, r2 = r2, r1
	}
	if c1 > c2 {
		c1, c2 = c2, c1
	}

	for r := r1; r <= r2; r++ {
		for c := c1; c <= c2; c++ {
			pos := util.NewPos(r, c)
			if b := brd.GetBot(pos); b != nil {
				b.IsSelected = true
				brd.MarkDirty(idx(pos))
			}
		}
	}
}

func cursorPosCallback(w *glfw.Window, xpos, ypos float64) {
	winW, winH := w.GetSize()

	// hover calculation
	cellPxX := float32(winW) / float32(cols) * camScale
	cellPxY := float32(winH) / float32(rows) * camScale
	wx := camX + float32(xpos)/cellPxX
	wy := camY + float32(float32(winH)-float32(ypos))/cellPxY

	r, c := int(wy), int(wx)

	idx := r*core.Cols + c
	if idx != ctrlState.HoveredIdx && idx >= 0 && idx < maxCells {
		if ctrlState.HoveredIdx >= 0 {
			brd.MarkDirty(ctrlState.HoveredIdx)
		}
		ctrlState.HoveredIdx = idx
		brd.MarkDirty(ctrlState.HoveredIdx)
	}
	highlightRect(brd, ctrlState.LastClickIdx, ctrlState.HoveredIdx)
	if !dragging {
		return
	}
	dx := xpos - dragStartX
	dy := ypos - dragStartY
	camX = camStartX - float32(dx)*float32(cols)/float32(winW)/camScale
	camY = camStartY + float32(dy)*float32(rows)/float32(winH)/camScale
}

func mouseButtonCallback(w *glfw.Window, button glfw.MouseButton, action glfw.Action, mods glfw.ModifierKey) {
	if button != glfw.MouseButtonLeft {
		return
	}
	switch action {
	case glfw.Press:
		if ctrlState.LeftShiftPressed {
			dragging = true
		}
		dragStartX, dragStartY = w.GetCursorPos()
		camStartX, camStartY = camX, camY

		if ctrlState.HoveredIdx != -1 {
			ctrlState.LastClickIdx = ctrlState.HoveredIdx
		}

		for _, b := range brd.Bots {
			if b == nil {
				continue
			}
			if !ctrlState.LeftCtrlPressed {
				b.IsSelected = false
			}
		}
	case glfw.Release:
		if ctrlState.HoveredIdx != -1 {
			hoveredPos := util.PosOf(ctrlState.HoveredIdx)

			if ctrlState.HoveredIdx == ctrlState.LastClickIdx {
				if !brd.IsEmpty(hoveredPos) {
					return
				}
				brd.Set(hoveredPos, core.Building{Pos: hoveredPos})
				return
			}

			logBot(hoveredPos)
		}
		ctrlState.LastClickIdx = -1
		dragging = false
	}
}

func logBot(hoveredPos util.Position) {
	if b := brd.GetBot(hoveredPos); b != nil {
		taskIsDone := "no"
		if b.CurrTask != nil && b.CurrTask.IsDone {
			taskIsDone = "yes"
		}
		targetPos := util.NewPos(0, 0)
		if b.CurrTask != nil {
			targetPos = b.CurrTask.Pos
		}
		fmt.Printf("Bot Pos: %v; CurrTaskIsNull: %v; TaskIsDone: %v; TargetPos: %v\n",
			b.Pos, b.CurrTask == nil, taskIsDone, targetPos)
	} else {
		fmt.Printf("Not bot. Pos: %v; BoardEmpty: %v; Occupant: %T\n",
			util.PosOf(ctrlState.HoveredIdx), brd.IsEmpty(hoveredPos), brd.At(hoveredPos))
	}
}

func scrollCallback(_ *glfw.Window, _ float64, yoff float64) {
	f := 1 + float32(yoff)*0.1
	camScale *= f
	if camScale < 0.5 {
		camScale = 0.5
	}
	if camScale > 4.0 {
		camScale = 4.0
	}
}

func keyCallback(w *glfw.Window, key glfw.Key, scancode int, action glfw.Action, mods glfw.ModifierKey) {
	//Press
	switch action {
	case glfw.Press:
		switch key {
		case glfw.KeyLeftShift:
			fmt.Println("left shift pressed")
			ctrlState.LeftShiftPressed = true
		case glfw.KeyLeftControl:
			fmt.Println("left ctrl pressed")
			ctrlState.LeftCtrlPressed = true
		case glfw.KeyE:
			conf.ToggleTaskTargets()
		case glfw.KeyW:
			conf.ToggleUnreachables()
		case glfw.KeyQ:
			conf.TogglePaths()
		case glfw.KeyK:
			conf.SpeedUp()
		case glfw.KeyJ:
			conf.SlowDown()
		case glfw.KeyP:
			conf.Pause = !conf.Pause
			if !conf.Pause {
				gameState.LastLogic = time.Now()
			}
		case glfw.KeyEscape:
			w.SetShouldClose(true)
		}
	case glfw.Release:
		switch key {
		case glfw.KeyLeftShift:
			fmt.Println("left shift unpressed")
			ctrlState.LeftShiftPressed = false
		case glfw.KeyLeftControl:
			fmt.Println("left ctrl unpressed")
			ctrlState.LeftCtrlPressed = false
		}
	}
}
