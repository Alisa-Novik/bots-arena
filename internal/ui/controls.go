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
	MousePainting    bool
	ActiveTool       GodTool
	BrushRadius      int
	LastGodMessage   string
	InspectLines     []string
	ResetRequested   bool
	RenderMode       RenderMode
}

type RenderMode int

const (
	RenderModeNormal RenderMode = iota
	RenderModeGenome
	RenderModeHealth
	RenderModeInventory
	RenderModeColony
	RenderModeTask
	RenderModeBiome
	RenderModePheromone
	renderModeCount
)

type GodTool int

const (
	GodToolInspect GodTool = iota
	GodToolWater
	GodToolPoison
	GodToolFood
	GodToolColony
	GodToolFreeze
	GodToolUnfreeze
	GodToolBless
	GodToolCurse
	GodToolBuild
)

type GodReport struct {
	Message string
	Lines   []string
}

type GodActions interface {
	ApplyGodTool(tool GodTool, pos core.Position, radius int) GodReport
	SaveGenome(pos core.Position) GodReport
	SaveMap() GodReport
	SelectedColonyLabel() string
}

var godActions GodActions

func SetGodActions(actions GodActions) {
	godActions = actions
}

func (m RenderMode) Label() string {
	switch m {
	case RenderModeNormal:
		return "Normal"
	case RenderModeGenome:
		return "Genome"
	case RenderModeHealth:
		return "Health"
	case RenderModeInventory:
		return "Inventory"
	case RenderModeColony:
		return "Colony"
	case RenderModeTask:
		return "Task"
	case RenderModeBiome:
		return "Biome"
	case RenderModePheromone:
		return "Pheromone"
	default:
		return "Unknown"
	}
}

func cycleRenderMode() {
	prev := ctrlState.RenderMode
	ctrlState.RenderMode = (ctrlState.RenderMode + 1) % renderModeCount
	ctrlState.LastGodMessage = "Render mode: " + ctrlState.RenderMode.Label()
	ctrlState.InspectLines = nil
	if brd != nil && (prev.fullBoardMode() || ctrlState.RenderMode.fullBoardMode()) {
		brd.MarkAllDirty()
		return
	}
	markBotCellsDirty()
}

func (m RenderMode) fullBoardMode() bool {
	return m == RenderModeColony || m == RenderModeBiome || m == RenderModePheromone
}

func markBotCellsDirty() {
	if brd == nil {
		return
	}
	for _, id := range brd.ActiveBotIDs() {
		if cell := brd.BotCell(id); cell >= 0 {
			brd.MarkDirty(cell)
		}
	}
}

func ConsumeSimulationResetRequest() bool {
	if !ctrlState.ResetRequested {
		return false
	}
	ctrlState.ResetRequested = false
	return true
}

func MarkSimulationResetComplete() {
	ctrlState.HoveredIdx = -1
	ctrlState.LastClickIdx = -1
	ctrlState.MousePainting = false
	ctrlState.InspectLines = nil
	ctrlState.LastGodMessage = "Simulation reset"
	dragging = false
}

func (t GodTool) Label() string {
	switch t {
	case GodToolInspect:
		return "Inspect"
	case GodToolWater:
		return "Water"
	case GodToolPoison:
		return "Poison"
	case GodToolFood:
		return "Food"
	case GodToolColony:
		return "Colony"
	case GodToolFreeze:
		return "Freeze"
	case GodToolUnfreeze:
		return "Unfreeze"
	case GodToolBless:
		return "Bless"
	case GodToolCurse:
		return "Curse"
	case GodToolBuild:
		return "Build"
	default:
		return "Unknown"
	}
}

func (t GodTool) PaintsContinuously() bool {
	switch t {
	case GodToolWater, GodToolPoison, GodToolFood, GodToolFreeze, GodToolUnfreeze:
		return true
	default:
		return false
	}
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
	if idx, ok := cursorBoardIdx(w, xpos, ypos); ok && idx != ctrlState.HoveredIdx {
		if ctrlState.HoveredIdx >= 0 {
			brd.MarkDirty(ctrlState.HoveredIdx)
		}
		ctrlState.HoveredIdx = idx
		brd.MarkDirty(ctrlState.HoveredIdx)
	}
	if ctrlState.ActiveTool == GodToolInspect && !ctrlState.MousePainting {
		highlightRect(brd, ctrlState.LastClickIdx, ctrlState.HoveredIdx)
	}
	if ctrlState.MousePainting && ctrlState.ActiveTool.PaintsContinuously() {
		applyGodToolAtHover()
	}

	if dragging {
		winW, winH := w.GetSize()
		dx := xpos - dragStartX
		dy := ypos - dragStartY
		camX = camStartX - float32(dx)*float32(cols)/float32(winW)/camScale
		camY = camStartY + float32(dy)*float32(rows)/float32(winH)/camScale
	}
}

func cursorBoardIdx(w *glfw.Window, xpos, ypos float64) (int, bool) {
	winW, winH := w.GetSize()
	cellPxX := float32(winW) / float32(cols) * camScale
	cellPxY := float32(winH) / float32(rows) * camScale
	wx := camX + float32(xpos)/cellPxX
	wy := camY + float32(float32(winH)-float32(ypos))/cellPxY

	r, c := int(wy), int(wx)
	if r < 0 || r >= core.Rows || c < 0 || c >= core.Cols {
		return -1, false
	}
	return r*core.Cols + c, true
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

		if ctrlState.ActiveTool == GodToolInspect && !ctrlState.LeftCtrlPressed {
			clearBotSelection()
		}
		if !ctrlState.LeftShiftPressed {
			ctrlState.MousePainting = ctrlState.ActiveTool.PaintsContinuously()
			applyGodToolAtHover()
		}
	case glfw.Release:
		if ctrlState.HoveredIdx != -1 {
			hoveredPos := util.PosOf(ctrlState.HoveredIdx)

			if ctrlState.ActiveTool == GodToolBuild && ctrlState.HoveredIdx == ctrlState.LastClickIdx {
				if !brd.IsEmpty(hoveredPos) {
					return
				}
				brd.Set(hoveredPos, core.Building{Pos: hoveredPos})
				return
			}

			if ctrlState.ActiveTool == GodToolInspect {
				logBot(hoveredPos)
			}
		}
		ctrlState.LastClickIdx = -1
		ctrlState.MousePainting = false
		dragging = false
	}
}

func clearBotSelection() {
	for _, id := range brd.ActiveBotIDs() {
		b := brd.BotByID(id)
		if b == nil || !b.IsSelected {
			continue
		}
		b.IsSelected = false
		if cell := brd.BotCell(id); cell >= 0 {
			brd.MarkDirty(cell)
		}
	}
}

func applyGodToolAtHover() {
	if ctrlState.HoveredIdx < 0 {
		return
	}
	if ctrlState.ActiveTool == GodToolBuild {
		pos := util.PosOf(ctrlState.HoveredIdx)
		if brd.IsEmpty(pos) {
			brd.Set(pos, core.Building{Pos: pos})
			ctrlState.LastGodMessage = "Built wall"
		}
		return
	}
	if godActions == nil {
		ctrlState.LastGodMessage = "God tools unavailable"
		return
	}
	report := godActions.ApplyGodTool(ctrlState.ActiveTool, util.PosOf(ctrlState.HoveredIdx), ctrlState.BrushRadius)
	ctrlState.LastGodMessage = report.Message
	if report.Lines != nil {
		ctrlState.InspectLines = report.Lines
	}
}

func applySaveReport(report GodReport) {
	ctrlState.LastGodMessage = report.Message
	if report.Lines != nil {
		ctrlState.InspectLines = report.Lines
	}
}

func saveGenomeAtHover() {
	if godActions == nil {
		ctrlState.LastGodMessage = "Save unavailable"
		return
	}
	pos := core.Position{R: -1}
	if ctrlState.HoveredIdx >= 0 {
		pos = util.PosOf(ctrlState.HoveredIdx)
	}
	applySaveReport(godActions.SaveGenome(pos))
}

func saveMap() {
	if godActions == nil {
		ctrlState.LastGodMessage = "Save unavailable"
		return
	}
	applySaveReport(godActions.SaveMap())
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
		ctrlState.InspectLines = []string{
			fmt.Sprintf("Bot R%d C%d", b.Pos.R, b.Pos.C),
			fmt.Sprintf("HP %d F %d O %d", b.Hp, b.Inventory.Food, b.Inventory.Ore),
			fmt.Sprintf("Task done: %s", taskIsDone),
			brd.PheromoneAt(hoveredPos).InspectString(),
		}
	} else {
		fmt.Printf("Not bot. Pos: %v; BoardEmpty: %v; Occupant: %T\n",
			util.PosOf(ctrlState.HoveredIdx), brd.IsEmpty(hoveredPos), brd.At(hoveredPos))
		ctrlState.InspectLines = []string{
			fmt.Sprintf("Cell R%d C%d", hoveredPos.R, hoveredPos.C),
			fmt.Sprintf("Occupant %T", brd.At(hoveredPos)),
			fmt.Sprintf("Empty %v", brd.IsEmpty(hoveredPos)),
			brd.PheromoneAt(hoveredPos).InspectString(),
		}
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
		case glfw.Key1:
			ctrlState.ActiveTool = GodToolInspect
		case glfw.Key2:
			ctrlState.ActiveTool = GodToolWater
		case glfw.Key3:
			ctrlState.ActiveTool = GodToolPoison
		case glfw.Key4:
			ctrlState.ActiveTool = GodToolFood
		case glfw.Key5:
			ctrlState.ActiveTool = GodToolColony
		case glfw.Key6:
			ctrlState.ActiveTool = GodToolFreeze
		case glfw.Key7:
			ctrlState.ActiveTool = GodToolUnfreeze
		case glfw.Key8:
			ctrlState.ActiveTool = GodToolBless
		case glfw.Key9:
			ctrlState.ActiveTool = GodToolCurse
		case glfw.Key0:
			ctrlState.ActiveTool = GodToolBuild
		case glfw.KeyLeftBracket:
			if ctrlState.BrushRadius > 0 {
				ctrlState.BrushRadius--
			}
		case glfw.KeyRightBracket:
			if ctrlState.BrushRadius < 20 {
				ctrlState.BrushRadius++
			}
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
		case glfw.KeyR:
			ctrlState.ResetRequested = true
			ctrlState.LastGodMessage = "Reset queued"
			ctrlState.InspectLines = nil
		case glfw.KeyG:
			saveGenomeAtHover()
		case glfw.KeyM:
			saveMap()
		case glfw.KeyV:
			cycleRenderMode()
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
