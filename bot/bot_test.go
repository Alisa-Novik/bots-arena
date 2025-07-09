package bot

import "testing"

func TestRandomDirRange(t *testing.T) {
	for i := 0; i < 100; i++ {
		d := RandomDir()
		if d != Up && d != Down && d != Left && d != Right {
			t.Fatalf("unexpected direction %#v", d)
		}
	}
}

func TestNewBot(t *testing.T) {
	b := NewBot()
	if b.Hp != 100 {
		t.Errorf("expected hp 100, got %d", b.Hp)
	}
	if b.Dir != Up && b.Dir != Down && b.Dir != Left && b.Dir != Right {
		t.Errorf("invalid direction %#v", b.Dir)
	}
}

func TestNewMutatedGenome(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		genome     Genome
		doMutation bool
		want       Genome
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewMutatedGenome(tt.genome, tt.doMutation)
			// TODO: update the condition below to compare got with tt.want.
			if true {
				t.Errorf("NewMutatedGenome() = %v, want %v", got, tt.want)
			}
		})
	}
}
