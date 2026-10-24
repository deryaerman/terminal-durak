package game

import (
	"fmt"
	"slices"

	"durak/internal/cards"
)

// Moves as values. The bots (and later a neural network) think in terms of
// "which of the legal moves now?", so a move is a small value that can be
// listed, compared and applied to any copy of the game.

// MoveKind is what kind of move it is.
type MoveKind int

const (
	PlayCard MoveKind = iota // attack, throw in, or beat the first unbeaten attack
	PassOn                   // pass the attack on with a card of the same rank
	TakeCards
	EndTurnMove
)

// Move is one thing a player can do. Card is unused for TakeCards and EndTurnMove.
type Move struct {
	Kind MoveKind
	Card cards.Card
}

func (m Move) String() string {
	switch m.Kind {
	case PlayCard:
		return "play " + m.Card.String()
	case PassOn:
		return "pass on with " + m.Card.String()
	case TakeCards:
		return "take"
	default:
		return "end turn"
	}
}

// LegalMoves lists everything player p may do right now (nil if it isn't
// their turn).
func (s *State) LegalMoves(p int) []Move {
	if s.Over || s.Turn != p {
		return nil
	}
	hand := s.Players[p].Hand
	var moves []Move
	if p == s.Attacker {
		for _, c := range hand {
			if s.checkAttack(c) == nil {
				moves = append(moves, Move{PlayCard, c})
			}
		}
		if len(s.Table) > 0 {
			moves = append(moves, Move{Kind: EndTurnMove})
		}
		return moves
	}
	if u := s.Undefended(); u >= 0 && !s.Taking {
		attack := s.Table[u].Attack
		for _, c := range hand {
			if s.Beats(c, attack) {
				moves = append(moves, Move{PlayCard, c})
			}
			if s.CanTransfer(c) {
				moves = append(moves, Move{PassOn, c})
			}
		}
		moves = append(moves, Move{Kind: TakeCards})
	} else if len(s.Table) > 0 {
		moves = append(moves, Move{Kind: EndTurnMove})
	}
	return moves
}

// Apply makes move m for player p.
func (s *State) Apply(p int, m Move) error {
	switch m.Kind {
	case PlayCard, PassOn:
		i := slices.Index(s.Players[p].Hand, m.Card)
		if i < 0 {
			return fmt.Errorf("%v is not in %s's hand", m.Card, s.Players[p].Name)
		}
		if m.Kind == PassOn {
			return s.Transfer(p, i)
		}
		return s.Play(p, i)
	case TakeCards:
		if s.Turn != p || p != s.Defender() {
			return ErrNotYourTurn
		}
		s.Take()
		return nil
	default:
		return s.EndTurn()
	}
}

// Clone returns an independent deep copy, so a bot can play out imagined
// games without touching the real one.
func (s *State) Clone() *State {
	c := *s                // copies every plain field
	c.DeferAdvance = false // imagined games never wait for the UI
	c.Deck = &cards.Deck{Cards: slices.Clone(s.Deck.Cards)}
	for i, p := range s.Players {
		c.Players[i] = &Player{Name: p.Name, Hand: slices.Clone(p.Hand)}
	}
	c.Table = make([]Pair, len(s.Table))
	for i, p := range s.Table {
		c.Table[i].Attack = p.Attack
		if p.Defense != nil {
			d := *p.Defense
			c.Table[i].Defense = &d
		}
	}
	return &c
}
