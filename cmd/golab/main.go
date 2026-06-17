package main

import (
	"flag"
	"golab/internal/config"
	"golab/internal/game"
	"golab/internal/ui"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"syscall"

	"github.com/go-gl/glfw/v3.3/glfw"
)

func init() { runtime.LockOSThread() }

func main() {
	if runCommand(os.Args[1:]) {
		return
	}

	f, err := os.Create("cpu.out")
	if err != nil {
		panic(err)
	}
	// ensure we flush even on Ctrl-C
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		pprof.StopCPUProfile()
		f.Close()
		os.Exit(0)
	}()

	headless := flag.Bool("h", false, "run without creating an OpenGL window")
	headlessTicks := flag.Int("ticks", 0, "headless ticks to run before exiting; 0 runs until interrupted")
	flag.Parse()

	// config := config.LoadFromJson("conf.json")
	config := config.NewConfig()
	g := game.NewGame(&config)

	ui.SetConfig(&config)
	ui.SetBoard(g.Board)
	ui.SetGameState(g.State)

	pprof.StartCPUProfile(f)
	if *headless {
		g.RunHeadless(*headlessTicks)
	} else {
		ui.PrepareUi()
		defer glfw.Terminate()
		g.Run()
	}
	pprof.StopCPUProfile()
	f.Close()
}
