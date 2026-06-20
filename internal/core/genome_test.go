package core

import (
	"golab/internal/util"
	"math/rand"
	"testing"
)

func TestNewRandomGenomeUsesDecodableRandomValuesWithoutBootstrap(t *testing.T) {
	rand.Seed(1)

	genome := NewRandomGenome()
	for idx, value := range genome.Matrix {
		if value < 0 || value > genomeMaxValue {
			t.Fatalf("matrix[%d] = %d, want value <= %d", idx, value, genomeMaxValue)
		}
		if decoded := DecodeOpcode(value); decoded < 0 || decoded >= numOpcodes {
			t.Fatalf("matrix[%d] decoded to %d, want valid opcode < %d", idx, decoded, numOpcodes)
		}
	}

	if DecodeOpcode(genome.Matrix[0]) == OpMove &&
		DecodeOpcode(genome.Matrix[1]) == OpGrab &&
		DecodeOpcode(genome.Matrix[7]) == OpDivide &&
		DecodeOpcode(genome.Matrix[8]) == OpDivide &&
		DecodeOpcode(genome.Matrix[9]) == OpDivide {
		t.Fatalf("random genome still matches old move/grab/divide bootstrap")
	}
}

func TestNewMutatedGenomeUsesDecodableValues(t *testing.T) {
	rand.Seed(2)

	genome := NewRandomGenome()
	mutated := NewMutatedGenome(genome, true)

	for idx, value := range mutated.Matrix {
		if value < 0 || value > genomeMaxValue {
			t.Fatalf("mutated matrix[%d] = %d, want value <= %d", idx, value, genomeMaxValue)
		}
		if decoded := DecodeOpcode(value); decoded < 0 || decoded >= numOpcodes {
			t.Fatalf("mutated matrix[%d] decoded to %d, want valid opcode < %d", idx, decoded, numOpcodes)
		}
	}
}

func TestDecodeOpcodeMapsFullGeneRangeToInstructions(t *testing.T) {
	for value := 0; value <= genomeMaxValue; value++ {
		if got := DecodeOpcode(value); got < 0 || got >= numOpcodes {
			t.Fatalf("DecodeOpcode(%d) = %d, want valid opcode", value, got)
		}
	}
}

func TestSameColonyRequiresNonNilColony(t *testing.T) {
	first := NewBot(util.NewPos(10, 10))
	second := NewBot(util.NewPos(10, 11))

	if first.SameColony(&second) {
		t.Fatalf("colonyless bots should not compare as same colony")
	}

	colony := NewColony(util.NewPos(10, 9))
	colony.AddFamily(&first)
	colony.AddFamily(&second)

	if !first.SameColony(&second) {
		t.Fatalf("bots in the same non-nil colony should compare as same colony")
	}
}

func TestIsOffspringChecksDescendantsWithoutLoopingOnParent(t *testing.T) {
	parent := NewBot(util.NewPos(10, 10))
	child := NewBot(util.NewPos(10, 11))
	grandchild := NewBot(util.NewPos(10, 12))
	parent.AddOffspring(&child)
	child.AddOffspring(&grandchild)

	if !child.IsOffspring(&parent) {
		t.Fatalf("direct child was not recognized as offspring")
	}
	if !grandchild.IsOffspring(&parent) {
		t.Fatalf("grandchild was not recognized as offspring")
	}
	if parent.IsOffspring(&child) {
		t.Fatalf("parent should not be offspring of child")
	}
}

func TestBotsFriendlyUsesKinOrNonNilColony(t *testing.T) {
	parent := NewBot(util.NewPos(10, 10))
	child := NewBot(util.NewPos(10, 11))
	stranger := NewBot(util.NewPos(10, 12))
	for i := range parent.Genome.Matrix {
		parent.Genome.Matrix[i] = 0
		child.Genome.Matrix[i] = 0
	}
	for i := range stranger.Genome.Matrix {
		stranger.Genome.Matrix[i] = genomeMaxValue
	}
	parent.AddOffspring(&child)

	if !BotsFriendly(&parent, &child) {
		t.Fatalf("parent and child should be friendly kin")
	}
	if BotsFriendly(&parent, &stranger) {
		t.Fatalf("unrelated colonyless bots should not be friendly")
	}
}

func TestCmdArgDirUsesRequestedArgument(t *testing.T) {
	bot := NewBot(util.NewPos(20, 20))
	bot.Genome.Matrix[1] = 0
	bot.Genome.Matrix[2] = 2

	if got, want := bot.CmdArgDir(1, bot.Pos), bot.Pos.AddDir(util.PosClock[0]); got != want {
		t.Fatalf("arg1 direction = %v, want %v", got, want)
	}
	if got, want := bot.CmdArgDir(2, bot.Pos), bot.Pos.AddDir(util.PosClock[2]); got != want {
		t.Fatalf("arg2 direction = %v, want %v", got, want)
	}
}
