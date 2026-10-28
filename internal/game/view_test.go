package game

import (
	"encoding/json"
	"testing"

	"durak/internal/cards"
)

// TestViewHidesAndRebuilds plays games and, at every step, checks that:
// the view never contains the opponent's unknown cards, it survives a JSON
// round trip, and the mirror rebuilt from it agrees with the real game on
// everything the receiver may know, including which of their cards are
// playable.
func TestViewHidesAndRebuilds(t *testing.T) {
	for g := 0; g < 20; g++ {
		s := New()
		s.Rules.Difficulty = Normal
		for step := 0; step < 400 && !s.Over; step++ {
			for seat := range 2 {
				v := s.ViewFor(seat)
				data, err := json.Marshal(v)
				if err != nil {
					t.Fatal(err)
				}
				var back View
				if err := json.Unmarshal(data, &back); err != nil {
					t.Fatal(err)
				}
				// Nothing of the opponent's hand beyond the known cards.
				secret := cards.SetOf(s.Players[1-seat].Hand...).Minus(s.Known[1-seat])
				if cards.SetOf(back.OppKnown...).Intersect(secret) != 0 || cards.SetOf(back.Hand...).Intersect(secret) != 0 {
					t.Fatal("view leaks the opponent's cards")
				}
				m := FromView(back)
				if len(m.Players[Opponent].Hand) != len(s.Players[1-seat].Hand) || m.Deck.Len() != s.Deck.Len() {
					t.Fatal("mirror sizes differ")
				}
				if (m.Turn == You) != (s.Turn == seat) || (m.Attacker == You) != (s.Attacker == seat) {
					t.Fatal("mirror roles differ")
				}
				for _, c := range s.Players[seat].Hand {
					if m.CanPlay(You, c) != s.CanPlay(seat, c) {
						t.Fatalf("playable %v: mirror %v, real %v", c, m.CanPlay(You, c), s.CanPlay(seat, c))
					}
				}
			}
			s.AutoMove(s.Turn)
		}
	}
}
