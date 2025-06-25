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
