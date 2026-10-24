package game

import (
	"durak/internal/cards"
)

// The Normal bot: no search, just card counting and the habits of a decent
// human player. It only ever uses information player p could have: their own
// hand, the table, and what everyone has seen (memory.go). The Hard bot uses
// it to play out its imagined games, so it also has to be quick.

// value is how precious a card is to keep: low non-trumps are cheap, trumps
// are worth more than any non-trump, and higher ranks more than lower ones.
func (s *State) value(c cards.Card) int {
	v := int(c.Rank - cards.Six) // 0..8
	if c.Suit == s.Trump.Suit {
		v += 13
	}
	return v
}

// oppMightBeat reports whether the opponent might hold a card that beats c.
// Once the deck is empty p knows the opponent's hand exactly (every unseen
// card is in it); before that, any unseen card might be.
func (s *State) oppMightBeat(p int, c cards.Card) bool {
	possible := s.Unseen(p).Union(s.Known[1-p])
	for _, o := range possible.Cards() {
		if s.Beats(o, c) {
			return true
		}
	}
	return false
}

// HeuristicMove picks a move for player p without searching.
func (s *State) HeuristicMove(p int) Move {
	moves := s.LegalMoves(p)
	if len(moves) == 0 {
		return Move{Kind: EndTurnMove}
	}
	if p == s.Attacker {
		return s.heuristicAttack(p, moves)
	}
	return s.heuristicDefend(p, moves)
}

func (s *State) heuristicAttack(p int, moves []Move) Move {
	endgame := s.Deck.Len() == 0
	hand := s.Players[p].Hand
	sameRank := map[cards.Rank]int{}
	for _, c := range hand {
		sameRank[c.Rank]++
	}

	best, bestScore := -1, 0
	for i, m := range moves {
		if m.Kind != PlayCard {
			continue
		}
		c := m.Card
		score := -s.value(c) * 10 // cheap cards first
		if len(s.Table) == 0 {
			score += 15 * sameRank[c.Rank] // a lead I can follow up with throw-ins
		}
		if !s.oppMightBeat(p, c) {
			score += 60 // they can't cover it: costs them a pickup
			if endgame {
				score += 100
			}
		}
		// Throwing in is optional: while the deck still has cards, keep
		// trumps back and throw in only non-trumps. (Testing against the
		// Easy bot showed throwing in every non-trump beats holding high
		// ones back.)
		if len(s.Table) > 0 && !endgame && c.Suit == s.Trump.Suit {
			continue
		}
		if best < 0 || score > bestScore {
			best, bestScore = i, score
		}
	}
	if best >= 0 {
		return moves[best]
	}
	return Move{Kind: EndTurnMove} // nothing worth adding (or no card at all)
}

func (s *State) heuristicDefend(p int, moves []Move) Move {
	if s.Undefended() < 0 {
		return moves[0] // everything is beaten: end the turn
	}
	trump := s.Trump.Suit

	// Passing on with a non-trump is usually the best defence there is.
	// (Passing on with trumps, even late, tested worse.)
	for _, m := range moves {
		if m.Kind == PassOn && m.Card.Suit != trump {
			return m
		}
	}

	// Plan a full defence: the cheapest card for each unbeaten attack.
	used := cards.Set(0)
	var first cards.Card
	for i, pair := range s.Table {
		if pair.Defense != nil {
			continue
		}
		pick, found := cards.Card{}, false
		for _, c := range s.Players[p].Hand {
			if used.Has(c) || !s.Beats(c, pair.Attack) {
				continue
			}
			if !found || s.value(c) < s.value(pick) {
				pick, found = c, true
			}
		}
		if !found {
			return Move{Kind: TakeCards} // can't beat everything
		}
		used = used.Add(pick)
		if i == s.Undefended() {
			first = pick
		}
	}
	// (Taking on purpose to save high trumps sounded smart but lost more
	// games in testing, so the bot defends whenever it can.)
	return Move{PlayCard, first}
}
