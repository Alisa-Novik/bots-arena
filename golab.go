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
var rows = 20
var cols = 40

func main() {
	generateBots(rows, cols)

	for {
		botsActions()
		renderMap()
		time.Sleep(1 * time.Millisecond)
	}
}

func botsActions() {
	moves := make(map[Point]bot.Bot)

	for pos, bot := range botsPos {
		newPos := botNewPos(pos)

		if newPos.X == cols-1 || newPos.Y == rows {
			moves[pos] = bot
			continue
		}

		if _, occupied := botsPos[newPos]; occupied {
			moves[pos] = bot
			continue
		}

		if _, planned := moves[newPos]; planned {
			moves[pos] = bot
			continue
		}

		moves[newPos] = bot
	}

	botsPos = moves
}

func right(pos Point) Point {
	dirs := []Point{{1, 0}, {0, 1}, {-1, 0}, {0, -1}}
	dir := dirs[rand.Intn(len(dirs))]
	newPos := NewPoint(pos.X+1, pos.Y+dir.Y)
	return newPos
}

func botNewPos(pos Point) Point {
	dirs := []Point{{1, 0}, {0, 1}, {-1, 0}, {0, -1}}
	dir := dirs[rand.Intn(len(dirs))]
	newPos := NewPoint(pos.X+dir.X, pos.Y+dir.Y)
	return newPos
}

func hasBot(newPos Point) bool {
	_, ok := botsPos[newPos]
	return ok
}

func renderMap() {
	clearScreen()
	fmt.Println("             === Arena ===")

	for range cols {
		fmt.Print("#")
	}

	for r := range rows {
		if r == 0 {
			continue
		}

		fmt.Println()

		for c := range cols {
			_, ok := botsPos[NewPoint(c, r)]
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

func generateBots(rows int, cols int) {
	for r := range rows {
		for c := range cols {
			if r == 0 || r == rows || c == 0 || c == cols-1 {
				continue
			}
			if rand.Intn(100) > 3 {
				continue
			}

			botName := "Bot" + strconv.Itoa(r) + strconv.Itoa(c)
			botsPos[NewPoint(c, r)] = bot.NewBot(botName)
		}
	}
}

func clearScreen() {
	fmt.Print("\033[H\033[2J")
}
