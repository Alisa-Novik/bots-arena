package main

import (
	"fmt"
	"golab/bot"
	"math/rand"
	"strconv"
	"time"
)

type Point struct {
	X int
	Y int
}

func NewPoint(x, y int) Point {
	return Point{X: x, Y: y}
}

var botsPos = map[Point]bot.Bot{}

func main() {
	bot1 := bot.NewBot("Bot1")

	fmt.Println("=== Arena ===")
	fmt.Printf("🤖 Bot %s | HP: %d\n", bot1.Name, bot1.Hp)
	for {
		renderMap()
		time.Sleep(3 * time.Second)
	}
}

func renderMap() {
	clearScreen()
	rows := 20
	cols := 40

	// Generate bots

	for r := range rows {
		for c := range cols { if r == 0 || r == rows || c == 0 || c == cols-1 {
				continue
			}
			if rand.Intn(100) > 3 {
				continue
			}

			botName := "Bot" + strconv.Itoa(r) + strconv.Itoa(c)
			botsPos[NewPoint(r, c)] = bot.NewBot(botName)
		}
	}

	for range cols {
		fmt.Print("#")
	}

	for r := range rows {
		if r == 0 {
			continue
		}

		fmt.Println()

		for c := range cols {
			p := NewPoint(r, c)
			_, ok := botsPos[p]
			if ok {
				fmt.Print("b")
				continue
			}

			if c == 0 || c == cols-1 {
				fmt.Print("#")
				continue
			}
			fmt.Print(" ")
		}
	}
	fmt.Println()
	for range cols {
		fmt.Print("#")
	}
}

func clearScreen() {
	fmt.Print("\033[H\033[2J")
}
