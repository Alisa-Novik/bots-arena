package main

import (
	"flag"
	"github.com/go-gl/glfw/v3.3/glfw"
	"golab/bot"
	"golab/game"
	"golab/ui"
	"golab/util"
	"runtime"
)

func init() { runtime.LockOSThread() }

func main() {
	headless := flag.Bool("h", false, "is headless mode?")
	useGenome := flag.Bool("i", false, "run with initial genome")
	flag.Parse()

	genConf := game.GenerationConfig{
		BotChance:        10,
		ResourceChance:   1,
		NewGenThreshold:  3,
		ControllerAmount: 300,
		ChildrenByBot:    30,
		InitialGenome:    getInitialGenome(*useGenome),
	}

	g := game.NewGame(genConf)

	if *headless {
		g.HeadlessRun()
	} else {
		ui.PrepareUi()
		defer glfw.Terminate()
		g.Run()
	}
}

func getInitialGenome(enabled bool) *bot.Genome {
	if !enabled {
		return nil
	}
	return util.ReadGenome()
}
