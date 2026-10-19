package game

import "durak/internal/cards"

// OpponentMove plays the opponent's whole turn with a very simple strategy:
// attack with the lowest card, defend with the cheapest card that beats each
// attack, and take the cards when it can't. It's a stand-in until the real
// bot (or a second human over the network) exists.
func (s *State) OpponentMove() { s.AutoMove(Opponent) }

// AutoMove plays player p's turn with the simple strategy above.
func (s *State) AutoMove(p int) {
	if s.Over || s.Turn != p {
		return
	}
	if p == s.Attacker {
		s.botAttack(p)
	} else {
		s.botDefend(p)
	}
}

func (s *State) botAttack(p int) {
	if s.Taking {
		// The defender is taking: hand over every cheap matching card.
		for {
			i := s.cheapest(s.Players[p].Hand, func(c cards.Card) bool {
				return c.Suit != s.Trump.Suit && s.checkAttack(c) == nil
			})
			if i < 0 {
				break
			}
			_ = s.Play(p, i)
		}
		_ = s.EndTurn()
		return
	}
	hand := s.Players[p].Hand
	pick := -1
	if len(s.Table) == 0 {
		pick = s.cheapest(hand, func(cards.Card) bool { return true })
	} else if len(s.Table) < HandSize && len(s.Players[1-p].Hand) > 0 {
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
		_ = s.Play(p, pick)
		s.Turn = 1 - p
		return
	}
	// Nothing to add: the round ends if everything was beaten.
	_ = s.EndTurn()
}

func (s *State) botDefend(p int) {
	// Pass the attack back with a same-rank non-trump if possible.
	if t := s.cheapest(s.Players[p].Hand, func(c cards.Card) bool {
		return c.Suit != s.Trump.Suit && s.CanTransfer(c)
	}); t >= 0 {
		_ = s.Transfer(p, t)
		return
	}
	for u := s.Undefended(); u >= 0; u = s.Undefended() {
		attack := s.Table[u].Attack
		pick := s.cheapest(s.Players[p].Hand, func(c cards.Card) bool {
			return s.Beats(c, attack)
		})
		if pick < 0 {
			s.Take()
			return
		}
		_ = s.Play(p, pick)
	}
	s.Status = s.Players[p].Name + " beat everything — add more or press space to end the round"
	s.Turn = s.Attacker
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

// Timeout advances the game when player p's turn timer runs out:
//   - an attacker facing an empty table must lead, so their cheapest card is played
//   - an attacker with cards on the table stops attacking (like ending the turn)
//   - a defender takes the cards
func (s *State) Timeout(p int) {
	if s.Over || s.Turn != p {
		return
	}
	name := s.Players[p].Name
	switch {
	case p == s.Attacker && len(s.Table) == 0:
		i := s.cheapest(s.Players[p].Hand, func(cards.Card) bool { return true })
		if i < 0 {
			return
		}
		_ = s.Play(p, i)
		s.Turn = 1 - p
		s.Status = name + " ran out of time — the cheapest card was played"
	case p == s.Attacker:
		_ = s.EndTurn()
		s.Status = name + " ran out of time — attack over"
	default:
		s.Take()
		s.Status = name + " ran out of time and took the cards"
	}
}
