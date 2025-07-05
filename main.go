package main

import (
	"flag"
	"golab/config"
	"golab/game"
	"golab/ui"
	"runtime"

	"github.com/go-gl/glfw/v3.3/glfw"
)

func init() { runtime.LockOSThread() }

func main() {
	headless := flag.Bool("h", false, "is headless mode?")
	useGenome := flag.Bool("i", false, "run with initial genome")
	flag.Parse()

	config := config.NewConfig(useGenome)
	g := game.NewGame(&config)

	ui.SetConfig(&config)

	if *headless {
		g.RunHeadless()
	} else {
		ui.PrepareUi()
		defer glfw.Terminate()
		g.Run()
	}
}
