package main

import (
	"flag"
	"golab/config"
	"golab/game"
	"golab/ui"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"syscall"

	"github.com/go-gl/glfw/v3.3/glfw"
)

func init() { runtime.LockOSThread() }

func main() {
	// --- profiling setup ----------------------------------------------------
	f, err := os.Create("cpu.out")
	if err != nil {
		panic(err)
	}
	if err := pprof.StartCPUProfile(f); err != nil {
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

	headless := flag.Bool("h", false, "is headless mode?")
	useGenome := flag.Bool("i", false, "run with initial genome")
	flag.Parse()

	config := config.NewConfig(useGenome)
	g := game.NewGame(&config)

	ui.SetConfig(&config)
	ui.SetGameState(g.State)

	pprof.StartCPUProfile(f)
	if *headless {
		g.RunHeadless()
	} else {
		ui.PrepareUi()
		defer glfw.Terminate()
		g.Run()
	}
	pprof.StopCPUProfile()
	f.Close()
}
