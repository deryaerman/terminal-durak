package game

import (
	"math/rand/v2"
	"testing"
	"time"
)

// TestISMCTSPlaysLegalGames plays whole games with a short think time and
// checks every move is legal and the games end.
func TestISMCTSPlaysLegalGames(t *testing.T) {
	for g := 0; g < 3; g++ {
		s := New()
		for step := 0; !s.Over; step++ {
			if step > 2000 {
				t.Fatal("game never ended")
			}
			m := s.ISMCTSMove(s.Turn, 20*time.Millisecond)
			if err := s.Apply(s.Turn, m); err != nil {
				t.Fatalf("illegal move %v: %v", m, err)
			}
		}
	}
}

// TestISMCTSTreeGrows checks the bookkeeping of one tree: visits add up
// and every root child was tried at least once.
func TestISMCTSTreeGrows(t *testing.T) {
	s := New()
	p := s.Turn
	root := newNode(nil, Move{}, 1-p)
	rng := rand.New(rand.NewPCG(3, 4))
	for range 2000 {
		s.ismctsIteration(root, p, rng)
	}
	total := 0
	for _, c := range root.children {
		total += c.visits
		if c.visits == 0 || c.avail < c.visits {
			t.Fatalf("child %v: visits %d avail %d", c.move, c.visits, c.avail)
		}
	}
	if total != 2000 || len(root.children) != len(s.LegalMoves(p)) {
		t.Fatalf("root children visits %d (want 2000), %d children for %d moves",
			total, len(root.children), len(s.LegalMoves(p)))
	}
}
