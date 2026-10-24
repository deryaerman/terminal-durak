package game

import (
	"math/rand/v2"
	"testing"

	"durak/internal/cards"
)

// TestDeterminizeIsConsistent plays many games and, at every step, checks
// that a determinized copy keeps every card exactly once, keeps hand and
// deck sizes, keeps what p can see, and keeps the opponent's known cards.
func TestDeterminizeIsConsistent(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	for g := 0; g < 30; g++ {
		s := New()
		s.Rules.Difficulty = Easy
		for step := 0; step < 300 && !s.Over; step++ {
			for p := range 2 {
				w := s.Determinize(p, rng)
				all := cards.SetOf(w.Deck.Cards...).Union(w.tableSet()).Union(w.Gone)
				total := w.Deck.Len() + w.tableCount() + s.Discards
				for _, pl := range w.Players {
					all = all.Union(cards.SetOf(pl.Hand...))
					total += len(pl.Hand)
				}
				if all != cards.FullDeck || total != 36 {
					t.Fatalf("game %d step %d p%d: cards lost or duplicated (%d distinct, %d total)", g, step, p, all.Len(), total)
				}
				if len(w.Players[1-p].Hand) != len(s.Players[1-p].Hand) || w.Deck.Len() != s.Deck.Len() {
					t.Fatal("sizes changed")
				}
				if cards.SetOf(w.Players[p].Hand...) != cards.SetOf(s.Players[p].Hand...) {
					t.Fatal("own hand changed")
				}
				if cards.SetOf(w.Players[1-p].Hand...).Intersect(s.Known[1-p]) != s.Known[1-p] {
					t.Fatal("opponent's known cards missing")
				}
				if s.Deck.Len() > 0 && w.Deck.Cards[0] != s.Trump {
					t.Fatal("trump not at the bottom")
				}
			}
			s.AutoMove(s.Turn)
		}
	}
}
