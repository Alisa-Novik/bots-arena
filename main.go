package main

import (
	"flag"
	"golab/bot"
	"golab/game"
	"golab/ui"
	"golab/util"
	"runtime"
	"time"

	"github.com/go-gl/glfw/v3.3/glfw"
)

func init() { runtime.LockOSThread() }

func main() {
	headless := flag.Bool("h", false, "is headless mode?")
	useGenome := flag.Bool("i", false, "run with initial genome")
	flag.Parse()

	genConf := game.GenerationConfig{
		BotChance:       1,
		ResourceChance:  1,
		NewGenThreshold: 3,
		ChildrenByBot:   10,
		InitialGenome:   getInitialGenome(*useGenome),

		ControllerInitialAmount: 10,
		HpFromController:        10000,
		InventoryFromController: -100,

		HpFromResource:        300,
		InventoryFromResource: 1000,

		HpFromBuilding:        300,
		InventoryFromBuilding: 300,

		LogicStep: 100000 * time.Nanosecond,
	}

	g := game.NewGame(genConf)

	if *headless {
		g.RunHeadless()
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
