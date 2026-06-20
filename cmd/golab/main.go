package main

import (
	"flag"
	"fmt"
	"golab/internal/config"
	"golab/internal/game"
	"golab/internal/ui"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"strings"
	"syscall"
	"time"

	"github.com/go-gl/glfw/v3.3/glfw"
)

func init() { runtime.LockOSThread() }

func main() {
	if runCommand(os.Args[1:]) {
		return
	}

	headless := flag.Bool("h", false, "run without creating an OpenGL window")
	headlessTicks := flag.Int("ticks", 0, "headless ticks to run before exiting; 0 runs until interrupted")
	gmMode := flag.String("gm", "mock", "game master mode: mock, external, or off")
	gmCommand := flag.String("gm-command", "", "external game-master command; receives observation JSON on stdin")
	gmInterval := flag.Int("gm-interval", 120, "logic ticks between game-master observations")
	gmTimeout := flag.Duration("gm-timeout", 750*time.Millisecond, "external game-master timeout")
	cpuProfile := flag.String("cpuprofile", "", "write CPU profile to path")
	flag.Parse()

	stopCPUProfile := startCPUProfile(*cpuProfile)
	defer stopCPUProfile()

	// config := config.LoadFromJson("conf.json")
	config := config.NewConfig()
	g := game.NewGame(&config)
	configureGameMaster(g, *gmMode, *gmCommand, *gmInterval, *gmTimeout)

	ui.SetConfig(&config)
	ui.SetBoard(g.Board)
	ui.SetGameState(g.State)
	ui.SetGodActions(g)

	if *headless {
		g.RunHeadless(*headlessTicks)
	} else {
		ui.PrepareUi()
		defer glfw.Terminate()
		g.Run()
	}
}

func startCPUProfile(path string) func() {
	if path == "" {
		return func() {}
	}

	f, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		f.Close()
		panic(err)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		pprof.StopCPUProfile()
		f.Close()
		os.Exit(0)
	}()

	return func() {
		signal.Stop(sig)
		pprof.StopCPUProfile()
		f.Close()
	}
}

func configureGameMaster(g *game.Game, mode string, command string, interval int, timeout time.Duration) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "mock":
		g.EnableGameMaster(interval)
	case "external", "coolio":
		argv := game.SplitGameMasterCommand(command)
		if len(argv) == 0 {
			fmt.Fprintln(os.Stderr, "--gm-command is required when --gm external is used")
			os.Exit(2)
		}
		g.EnableExternalGameMaster(interval, argv, timeout)
	case "off", "none", "disabled":
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown --gm mode: %s\n", mode)
		os.Exit(2)
	}
}
