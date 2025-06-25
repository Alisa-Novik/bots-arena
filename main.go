package main

import (
	"golab/game"
	"golab/ui"
	"runtime"

	"github.com/go-gl/glfw/v3.3/glfw"
)

func init() { runtime.LockOSThread() }

func main() {
	ui.PrepareUi()
	defer glfw.Terminate()

	conf := game.GenerationConfig{
		BotChance:       5,
		ResourceChance:  5,
		NewGenThreshold: 5,
		ChildrenByBot:   3,
	}
	g := game.NewGame(conf)
	g.HeadlessRun()
}
