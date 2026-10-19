package game

import "durak/internal/cards"

// OpponentMove plays the opponent's whole turn with a very simple strategy:
// attack with the lowest card, defend with the cheapest card that beats each
// attack, and take the cards when it can't. It's a stand-in until the real
// bot (or a second human over the network) exists.
func (s *State) OpponentMove() {
	if s.Over || s.Turn != Opponent {
		return
	}
	if Opponent == s.Attacker {
		s.botAttack()
	} else {
		s.botDefend()
	}
}

func (s *State) botAttack() {
	hand := s.Players[Opponent].Hand
	pick := -1
	if len(s.Table) == 0 {
		pick = s.cheapest(hand, func(cards.Card) bool { return true })
	} else if len(s.Table) < HandSize && len(s.Players[You].Hand) > 0 {
		// Throw in another card whose rank is already on the table.
		onTable := map[cards.Rank]bool{}
		for _, p := range s.Table {
			onTable[p.Attack.Rank] = true
			if p.Defense != nil {
				onTable[p.Defense.Rank] = true
			}
		}
		pick = s.cheapest(hand, func(c cards.Card) bool {
			return onTable[c.Rank] && c.Suit != s.Trump.Suit
		})
	}
	if pick >= 0 {
		_ = s.Play(Opponent, pick)
		s.Turn = You
		return
	}
	// Nothing to add: the round ends if everything was beaten.
	_ = s.EndTurn()
}

func (s *State) botDefend() {
	// Pass the attack back with a same-rank non-trump if possible.
	if t := s.cheapest(s.Players[Opponent].Hand, func(c cards.Card) bool {
		return c.Suit != s.Trump.Suit && s.CanTransfer(c)
	}); t >= 0 {
		_ = s.Transfer(Opponent, t)
		return
	}
	for u := s.Undefended(); u >= 0; u = s.Undefended() {
		attack := s.Table[u].Attack
		pick := s.cheapest(s.Players[Opponent].Hand, func(c cards.Card) bool {
			return s.Beats(c, attack)
		})
		if pick < 0 {
			s.Take()
			return
		}
		_ = s.Play(Opponent, pick)
	}
	s.Status = "Opponent beat everything — add more or press space to end the round"
	s.Turn = You
}

// cheapest returns the index of the lowest-value card in hand that ok accepts
// (non-trumps before trumps), or -1 if none does.
func (s *State) cheapest(hand []cards.Card, ok func(cards.Card) bool) int {
	best, bestVal := -1, 0
	for i, c := range hand {
		if !ok(c) {
			continue
		}
		val := int(c.Rank)
		if c.Suit == s.Trump.Suit {
			val += 100
		}
		if best < 0 || val < bestVal {
			best, bestVal = i, val
		}
	}
	return best
}
