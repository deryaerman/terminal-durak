package game

import (
	"slices"

	"durak/internal/cards"
)

// The perfect endgame. In a two-player game, once the deck is empty nothing
// is hidden any more: every card p can't see must be in the opponent's hand.
// The game becomes a puzzle like chess, so we can try every line of play to
// the end (minimax): p picks the best move assuming the opponent always picks
// the reply that's best for them.
//
// Results are 1 (p wins), 0.5 (draw) or 0 (p is the durak). Two tricks keep
// the search fast:
//   - a position seen before is looked up instead of searched again (the
//     "transposition table"), since many move orders reach the same position
//   - as soon as a move reaches the best possible result for the player
//     choosing, the remaining moves are skipped
//
// The table is kept for the whole game (State.endgameCache), so after the
// first endgame search, later endgame moves are mostly instant lookups.

// endgameNodeLimit caps the search; beyond it the caller falls back to Monte
// Carlo search. Endgames with up to a dozen or so cards finish well within it.
const endgameNodeLimit = 200_000

// posKey identifies a position for the transposition table. It's a plain
// struct of fixed-size fields, so Go can use it directly as a map key.
type posKey struct {
	hands    [2]cards.Set
	table    [HandSize][2]int8 // card indexes of attack and defence, -1 if none
	attacker int8
	turn     int8
	taking   bool
}

func (s *State) key() posKey {
	k := posKey{
		hands:    [2]cards.Set{cards.SetOf(s.Players[0].Hand...), cards.SetOf(s.Players[1].Hand...)},
		attacker: int8(s.Attacker),
		turn:     int8(s.Turn),
		taking:   s.Taking,
	}
	for i := range k.table {
		k.table[i] = [2]int8{-1, -1}
	}
	for i, p := range s.Table {
		k.table[i][0] = int8(p.Attack.Index())
		if p.Defense != nil {
			k.table[i][1] = int8(p.Defense.Index())
		}
	}
	return k
}

type solver struct {
	p     int // whose point of view
	tt    map[posKey]float64
	nodes int
}

func (sv *solver) result(s *State) float64 {
	switch s.Loser {
	case 1 - sv.p:
		return 1
	case sv.p:
		return 0
	default:
		return 0.5
	}
}

// value returns the result of s with perfect play, and false if the search
// grew too big.
func (sv *solver) value(s *State) (float64, bool) {
	if s.Over {
		return sv.result(s), true
	}
	k := s.key()
	if v, ok := sv.tt[k]; ok {
		return v, true
	}
	if sv.nodes++; sv.nodes > endgameNodeLimit {
		return 0, false
	}

	maximize := s.Turn == sv.p
	best := 0.0
	if !maximize {
		best = 1
	}
	for _, m := range sv.ordered(s) {
		next := s.Clone()
		if err := next.Apply(next.Turn, m); err != nil {
			continue
		}
		v, ok := sv.value(next)
		if !ok {
			return 0, false
		}
		if maximize && v > best || !maximize && v < best {
			best = v
		}
		if maximize && best == 1 || !maximize && best == 0 {
			break // can't do better than this
		}
	}
	sv.tt[k] = best
	return best, true
}

// ordered tries the likely-good moves first (cheap cards before precious
// ones; taking and ending the turn last), so good lines are found early and
// the "can't do better" shortcut kicks in sooner.
func (sv *solver) ordered(s *State) []Move {
	moves := s.LegalMoves(s.Turn)
	rank := func(m Move) int {
		switch m.Kind {
		case PlayCard:
			return s.value(m.Card)
		case PassOn:
			return 50 + s.value(m.Card)
		default:
			return 100
		}
	}
	slices.SortFunc(moves, func(a, b Move) int { return rank(a) - rank(b) })
	return moves
}

// EndgameMove returns the best move for p once the deck is empty, using
// only what p can know (the opponent's hand is worked out from the unseen
// cards). ok is false if the deck isn't empty or the search grew too big.
func (s *State) EndgameMove(p int) (Move, bool) {
	if s.Deck.Len() > 0 || s.Over || s.Turn != p {
		return Move{}, false
	}
	// With the deck empty there's only one possible deal of the hidden cards,
	// so any "imagined" deal is the real one from p's point of view.
	w := s.Clone()
	w.Players[1-p].Hand = s.Unseen(p).Union(s.Known[1-p]).Cards()

	if s.endgameCache == nil {
		s.endgameCache = &[2]map[posKey]float64{{}, {}}
	}
	sv := &solver{p: p, tt: s.endgameCache[p]}
	best, bestV := Move{}, -1.0
	for _, m := range sv.ordered(w) {
		next := w.Clone()
		if err := next.Apply(p, m); err != nil {
			continue
		}
		v, ok := sv.value(next)
		if !ok {
			return Move{}, false
		}
		if v > bestV {
			best, bestV = m, v
		}
		if bestV == 1 {
			break
		}
	}
	return best, bestV >= 0
}
