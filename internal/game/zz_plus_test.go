package game

import (
	"os"
	"testing"
)

func TestArenaHardPlus(t *testing.T) {
	if os.Getenv("DURAK_ARENA") == "" {
		t.Skip()
	}
	aL, bL, d, slow := playMatch(120, HardPlus, Hard)
	t.Logf("hard+ vs hard (120 games): hard+ durak %d, hard durak %d, draws %d, slowest %v", aL, bL, d, slow)
}

func TestArenaExpert(t *testing.T) {
	if os.Getenv("DURAK_ARENA") == "" {
		t.Skip()
	}
	aL, bL, d, slow := playMatch(120, Expert, HardPlus)
	t.Logf("expert vs hard+ (120 games): expert durak %d, hard+ durak %d, draws %d, slowest %v", aL, bL, d, slow)
}
