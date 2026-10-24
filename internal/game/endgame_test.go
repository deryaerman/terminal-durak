package game

import (
	"testing"

	"durak/internal/cards"
)

// With the deck empty, you hold K♥ (trump) and 6♠; the opponent holds only
// 7♠. Leading the cheap 6♠ loses: they beat it with 7♠ and are out first.
// Leading K♥ wins: they can't beat it and must take, and you then go out
// with the 6♠. The heuristic leads the cheap card; the solver must not.
func TestEndgameFindsTheWin(t *testing.T) {
	s := fixed(cards.Hearts,
		[]cards.Card{c(cards.King, cards.Hearts), c(cards.Six, cards.Spades)},
		[]cards.Card{c(cards.Seven, cards.Spades)})
	s.Deck.Cards = nil
	s.Gone = cards.FullDeck.Minus(cards.SetOf(
		c(cards.King, cards.Hearts), c(cards.Six, cards.Spades), c(cards.Seven, cards.Spades)))

	if h := s.HeuristicMove(You); h.Card != c(cards.Six, cards.Spades) {
		t.Logf("note: heuristic now picks %v", h)
	}
	m, ok := s.EndgameMove(You)
	if !ok || m != (Move{PlayCard, c(cards.King, cards.Hearts)}) {
		t.Fatalf("solver chose %v (ok=%v), want play K♥", m, ok)
	}
	// And playing it out with the solver on both sides, you win.
	for !s.Over {
		mv, _ := s.EndgameMove(s.Turn)
		if err := s.Apply(s.Turn, mv); err != nil {
			t.Fatal(err)
		}
	}
	if s.Loser != Opponent {
		t.Fatalf("loser = %d, want the opponent", s.Loser)
	}
}
