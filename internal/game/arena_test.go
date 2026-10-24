package game

import (
	"os"
	"testing"
	"time"
)

// playMatch plays n games between two difficulties (alternating who is
// player 0) and returns how often a ended up as the durak, and the slowest
// single move.
func playMatch(n int, a, b Difficulty) (aLost, bLost, draws int, slowest time.Duration) {
	for g := range n {
		s := New()
		levels := [2]Difficulty{a, b}
		if g%2 == 1 {
			levels = [2]Difficulty{b, a}
		}
		for step := 0; !s.Over && step < 2000; step++ {
			p := s.Turn
			s.Rules.Difficulty = levels[p]
			t := time.Now()
			s.AutoMove(p)
			slowest = max(slowest, time.Since(t))
		}
		switch {
		case s.Loser == Draw || !s.Over:
			draws++
		case levels[s.Loser] == a && (a != b || s.Loser == g%2):
			aLost++
		default:
			bLost++
		}
	}
	return
}

// TestArena pits the bots against each other. It's slow, so it only runs on
// request:  DURAK_ARENA=1 go test ./internal/game -run Arena -v
func TestArena(t *testing.T) {
	if os.Getenv("DURAK_ARENA") == "" {
		t.Skip("set DURAK_ARENA=1 to run")
	}
	for _, m := range []struct {
		a, b Difficulty
		n    int
	}{{Normal, Easy, 400}, {Hard, Easy, 60}, {Hard, Normal, 100}} {
		aL, bL, d, slow := playMatch(m.n, m.a, m.b)
		t.Logf("%-6v vs %-6v: %v is durak %4.1f%%, %v is durak %4.1f%%, draws %d  (slowest move %v)",
			m.a, m.b, m.a, 100*float64(aL)/float64(m.n), m.b, 100*float64(bL)/float64(m.n), d, slow.Round(time.Millisecond))
	}
}

func TestHardBotIsQuickAndLegal(t *testing.T) {
	s := New()
	s.Rules.Difficulty = Hard
	for step := 0; !s.Over && step < 2000; step++ {
		start := time.Now()
		s.AutoMove(s.Turn)
		if d := time.Since(start); d > 2*time.Second {
			t.Fatalf("a Hard move took %v", d)
		}
	}
	if !s.Over {
		t.Fatal("a Hard-vs-Hard game didn't finish")
	}
}
