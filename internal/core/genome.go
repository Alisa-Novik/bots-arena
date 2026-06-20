package core

import (
	"golab/internal/util"
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
	BuildColonyFlag
	BuildDepot
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
	case BuildColonyFlag:
		return "BuildColonyFlag"
	case BuildDepot:
		return "BuildDepot"
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
	OpCheckConnection
	OpCheckSignal
	OpSendSignal
	OpExecuteInstr

	// Register Opcodes
	OpSetReg
	OpIncReg
	OpDecReg
	OpJumpIfZero
	OpCmpReg
	OpEmitPheromone
	OpSensePheromone
	OpFollowPheromone
	numOpcodes
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
	case OpEmitPheromone:
		return "OpEmitPheromone"
	case OpSensePheromone:
		return "OpSensePheromone"
	case OpFollowPheromone:
		return "OpFollowPheromone"
	}
	return "OpJump/Unknown"
}

func DecodeOpcode(value int) Opcode {
	if value < 0 {
		value = -value
	}
	return Opcode(value % int(numOpcodes))
}

func OpcodeCount() int {
	return int(numOpcodes)
}

const genomeLen = 64
const genomeMaxValue = genomeLen - 1
const defaultGenomeMutationRate = 4
const broGenomeDifferenceLimit = 4
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
	dir := util.PosClock[b.CmdArg(i)%8]
	return pos.AddDir(dir)
}

func (b *Bot) IsOffspring(parent *Bot) bool {
	return b.isOffspring(parent, map[*Bot]struct{}{})
}

func (b *Bot) isOffspring(parent *Bot, visited map[*Bot]struct{}) bool {
	if b == nil || parent == nil {
		return false
	}
	if b == parent {
		return true
	}
	if _, ok := visited[parent]; ok {
		return false
	}
	visited[parent] = struct{}{}
	for o := range parent.Offsprings {
		if b.isOffspring(o, visited) {
			return true
		}
	}
	return false
}

func (b *Bot) FromString(other *Bot) bool {
	return b.SameColony(other)
}

func (b *Bot) SameColony(other *Bot) bool {
	return b != nil && other != nil && b.Colony != nil && b.Colony == other.Colony
}

func (b *Bot) IsBro(other *Bot) bool {
	if b == nil || other == nil {
		return false
	}
	differences := 0
	for i := range b.Genome.Matrix {
		if b.Genome.Matrix[i] != other.Genome.Matrix[i] {
			differences++
			if differences > broGenomeDifferenceLimit {
				return false
			}
		}
	}
	return true
}

func (b *Bot) IsKin(other *Bot) bool {
	return b != nil && other != nil &&
		(b == other || b.IsBro(other) || b.IsOffspring(other) || other.IsOffspring(b))
}

func BotsFriendly(a, b *Bot) bool {
	return a != nil && b != nil && (a == b || a.SameColony(b) || a.IsKin(b))
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
	return NewMutatedGenomeWithRate(genome, defaultGenomeMutationRate)
}

func NewMutatedGenomeWithRate(genome Genome, mutationRate int) Genome {
	if mutationRate <= 0 {
		return genome
	}
	for range mutationRate {
		mutationIdx := rand.Intn(genomeLen)
		genome.Matrix[mutationIdx] = NewRandomGenomeValue()
	}
	return genome
}

func NewRandomGenome() Genome {
	var g Genome
	for i := range g.Matrix {
		g.Matrix[i] = NewRandomGenomeValue()
	}
	g.Pointer = 0
	return g
}

func NewRandomGenomeValue() int {
	return rand.Intn(genomeMaxValue + 1)
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
