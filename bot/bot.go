package bot

import "math/rand"

type Direction [2]int

type Genome [8]int

var (
	Up    = Direction{0, 1}
	Right = Direction{1, 0}
	Down  = Direction{0, -1}
	Left  = Direction{-1, 0}
)

var dirs = []Direction{Up, Right, Down, Left}

type Bot struct {
	Name   string
	Dir    Direction
	Genome Genome
	Hp     int
}

func NewBot(name string) Bot {
	return Bot{
		Name:   name,
		Dir:    RandomDir(),
		Genome: NewRandomGenome(),
		Hp:     100,
	}
}

func RandomDir() Direction {
	return dirs[rand.Intn(4)]
}

func NewRandomGenome() Genome {
	var g Genome
	for i := range g {
		g[i] = rand.Intn(64)
	}
	return g
}
