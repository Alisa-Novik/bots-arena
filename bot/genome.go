package bot

import (
	"math/rand"
	"os"
	"strconv"
	"strings"
)

type BuildType int

const (
	BuildWall BuildType = iota
	BuildFarm
	BuildSpawner
	BuildController
	BuildBuilding
	numBuildTypes
)

func BuildTypesCount() int {
	return int(numBuildTypes)
}

func (b BuildType) String() string {
	switch b {
	case BuildWall:
		return "BuildWall"
	case BuildFarm:
		return "BuildFarm"
	case BuildSpawner:
		return "BuildSpawner"
	case BuildController:
		return "BuildController"
	case BuildBuilding:
		return "BuildBuilding"
	}
	return "unknown"
}

type Opcode int

const (
	OpMove Opcode = iota
	OpTurn
	OpLook
	OpGrab
	OpBuild
)

func (o Opcode) String() string {
	switch o {
	case OpMove:
		return "OpMove"
	case OpTurn:
		return "OpTurn"
	case OpLook:
		return "OpLook"
	case OpGrab:
		return "OpGrab"
	case OpBuild:
		return "OpBuild"
	}
	return "OpJump"
}

const genomeLen = 64
const genomeMaxValue = 63
const mutationRate = 3
const botHp = 200

type Genome struct {
	Matrix  [genomeLen]int
	Pointer int
}

func (b *Bot) PointerJump() {
	toAdd := b.Genome.Matrix[b.Genome.Pointer]
	b.Genome.Pointer = b.ptrPlus(toAdd)
}

func (b *Bot) PointerJumpBy(toAdd int) {
	b.Genome.Pointer = b.ptrPlus(toAdd)
}

func (b *Bot) CmdArg(i int) int {
	return b.Genome.Matrix[b.ptrPlus(i)]
}

func (b *Bot) ptrPlus(add int) int {
	ptr := b.Genome.Pointer
	if ptr >= 64 {
		panic("ptrPlus: ptr >= 64")
	}
	return (ptr + add) % genomeLen
}

func NewMutatedGenome(genome Genome, doMutation bool) Genome {
	if !doMutation {
		return genome
	}
	for range mutationRate {
		mutationIdx := rand.Intn(genomeLen)
		for i := range genome.Matrix {
			if i == mutationIdx {
				genome.Matrix[i] = rand.Intn(genomeMaxValue)
			}
		}
	}
	return genome
}

func NewRandomGenome() Genome {
	var g Genome
	for i := range g.Matrix {
		g.Matrix[i] = rand.Intn(genomeMaxValue)
	}
	g.Matrix[0] = int(OpLook)
	g.Matrix[1] = int(OpGrab)
	g.Matrix[2] = int(OpMove)
	g.Matrix[3] = int(OpTurn)
	g.Matrix[4] = 2
	g.Matrix[5] = int(OpLook)
	g.Matrix[6] = int(OpMove)
	g.Matrix[7] = int(OpBuild)
	g.Matrix[8] = int(BuildFarm)
	g.Matrix[9] = int(OpTurn)
	g.Matrix[10] = 4
	g.Pointer = 0
	return g
}

func ReadGenome() *Genome {
	data, _ := os.ReadFile("genome")
	parts := strings.Split(strings.TrimSuffix(string(data), ","), ",")
	var genome [64]int
	for i := range genome {
		genome[i], _ = strconv.Atoi(parts[i])
	}
	return &Genome{Matrix: genome}
}

func (b *Bot) SaveGenomeIntoFile() {
	var bld strings.Builder
	for _, v := range b.Genome.Matrix {
		bld.WriteString(strconv.Itoa(v))
		bld.WriteByte(',')
	}
	os.WriteFile("genome", []byte(bld.String()), 0644)
}

func GetInitialGenome(enabled bool) *Genome {
	if !enabled {
		return nil
	}
	return ReadGenome()
}
