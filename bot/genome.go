package bot

import (
	"golab/util"
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
	BuildMine
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
	case BuildMine:
		return "BuildMine"
	case BuildBuilding:
		return "BuildBuilding"
	}
	return "unknown"
}

type Opcode int

const (
	OpMove Opcode = iota
	OpMoveAbs
	OpCheckIfBro
	OpCheckColony
	OpTurn
	OpLook
	OpCheckHp
	OpCheckInventory
	OpHpToResource
	OpGrab
	OpEatOrganics
	OpEatOrganicsAbs
	OpPhoto
	OpEatOther
	OpBuild
	OpShareHp
	OpShareInventory
	OpAttack
	OpDivide
	OpCheckSignal
	OpSendSignal
	OpExecuteInstr

	// Register Opcodes
	OpSetReg
	OpIncReg
	OpDecReg
	OpJumpIfZero
	OpCmpReg
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
	return "OpJump/Unknown"
}

const genomeLen = 256
const genomeMaxValue = genomeLen - 1
const mutationRate = 4
const botHp = 100

type Genome struct {
	Matrix    [genomeLen]int
	Family    uint32
	Pointer   int
	NextArg   int
	Signal    int
	Registers [4]int
}

func (g Genome) Mutate() {
	panic("unimplemented")
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

func (b *Bot) CmdArgDir(i int, pos util.Position) util.Position {
	dir := util.PosClock[b.CmdArg(1)%8]
	return pos.Add(dir[0], dir[1])
}

const maxDifferences = 3

func (b *Bot) IsOffspring(parent *Bot) bool {
	if b == parent {
		return true
	}
	for o := range parent.Offsprings {
		if o.IsOffspring(parent) {
			return true
		}
	}
	return false
}

func (b *Bot) FromString(other *Bot) bool {
	return b.Colony == other.Colony
}

func (b *Bot) SameColony(other *Bot) bool {
	return b.Colony == other.Colony
}

func (b *Bot) IsBro(other *Bot) bool {
	// diff := bits.OnesCount64(b.hashLo^other.hashLo) +
	// 	bits.OnesCount64(b.hashHi^other.hashHi)
	// if diff > maxDifferences*2 {
	// 	return false
	// }

	// fall back to exact check (very rare)
	differences := 0
	for i := range b.Genome.Matrix {
		if b.Genome.Matrix[i] != other.Genome.Matrix[i] {
			differences++
			if differences > maxDifferences {
				return false
			}
		}
	}
	return true
}

func (b *Bot) ptrPlus(add int) int {
	ptr := b.Genome.Pointer
	if ptr > genomeMaxValue {
		panic("ptrPlus: ptr >= 128")
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
	// g.Matrix[0] = int(OpMove)
	// g.Matrix[1] = int(OpGrab)
	// g.Matrix[2] = int(OpBuild)
	// g.Matrix[2] = int(OpGrab)
	// g.Matrix[3] = int(OpGrab)
	// g.Matrix[4] = int(OpBuild)
	// g.Matrix[5] = int(OpBuild)
	// g.Matrix[6] = int(OpBuild)
	// g.Matrix[7] = int(OpBuild)
	// g.Matrix[2] = int(OpLook)
	// g.Matrix[3] = int(OpTurn)
	// g.Matrix[4] = int(OpCmpReg)
	// g.Matrix[5] = int(OpLook)
	// g.Matrix[6] = int(OpMove)
	// g.Matrix[7] = int(OpBuild)
	// g.Matrix[8] = int(BuildFarm)
	// g.Matrix[9] = int(OpEatOther)
	// g.Matrix[10] = int(OpCmpReg)
	// g.Matrix[11] = int(OpPhoto)
	// g.Matrix[12] = int(OpBuild)
	// g.Matrix[13] = int(OpCmpReg)
	// g.Matrix[14] = int(OpCmpReg)
	// g.Matrix[15] = int(OpBuild)
	g.Pointer = 0
	return g
}

func readGenome(data string) *Genome {
	parts := strings.Split(strings.TrimSuffix(data, ","), ",")
	var genome [genomeLen]int
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
	data, _ := os.ReadFile("genome")
	return readGenome(string(data))
}
