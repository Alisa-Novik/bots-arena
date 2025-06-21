package bot

import "math/rand"

type Direction [2]int

var (
	Up    = Direction{0, 1}
	Right = Direction{1, 0}
	Down  = Direction{0, -1}
	Left  = Direction{-1, 0}
)

var dirs = []Direction{Up, Right, Down, Left}

func RandomDir() Direction {
	return dirs[rand.Intn(4)]
}

type Bot struct {
	Name string
	Dir  Direction
	Hp   int
}

func NewBot(name string) Bot {
	return Bot{
		Name: name,
		Dir:  RandomDir(),
		Hp:   100,
	}
}
