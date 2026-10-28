package game

import (
	"slices"

	"durak/internal/cards"
)

// Views: what one player is allowed to know.
//
// In a network game only the host holds the real State. Each player is sent
// a View: their own hand, the table and everything public, but of the
// opponent's hand only how many cards it holds and the ones everyone saw
// them pick up. The opponent's other cards never leave the host, so nobody
// can cheat by reading the network traffic.
//
// Views are always "from the receiver's side": the receiver is player 0
// (You) and the other player is 1 (Opponent), whichever seat they have on
// the host. That way the UI works the same in every game.

// View is one player's picture of the game. It is sent over the network as
// JSON, so every field is exported.
type View struct {
	Names    [2]string // [0] is the receiver
	Hand     []cards.Card
	OppCount int          // how many cards the opponent holds
	OppKnown []cards.Card // of those, the ones everyone saw them pick up

	Trump    cards.Card
	DeckLen  int
	Discards int
	Table    []Pair

	Attacker int // 0 = you, 1 = the opponent
	Leader   int
	Turn     int
	Taking   bool
	Over     bool
	Loser    int // You, Opponent, or Draw
	Status   string

	Rules Rules
	Ply   int

	// TurnTimeMs is the length of a turn, TimeLeftMs how much the player on
	// the move has left (0 when there's no timer).
	TurnTimeMs int
	TimeLeftMs int
}

// relative turns a seat on the host into the receiver's point of view: the
// receiver becomes 0, the other player 1. (Draw stays -1.)
func relative(who, seat int) int {
	if who < 0 {
		return who
	}
	return who ^ seat // 0^0=0, 1^0=1, 0^1=1, 1^1=0
}

// ViewFor builds the View for the player in seat.
func (s *State) ViewFor(seat int) View {
	opp := 1 - seat
	v := View{
		Names:    [2]string{s.Players[seat].Name, s.Players[opp].Name},
		Hand:     slices.Clone(s.Players[seat].Hand),
		OppCount: len(s.Players[opp].Hand),
		OppKnown: s.Known[opp].Cards(),
		Trump:    s.Trump,
		DeckLen:  s.Deck.Len(),
		Discards: s.Discards,
		Attacker: relative(s.Attacker, seat),
		Leader:   relative(s.Leader, seat),
		Turn:     relative(s.Turn, seat),
		Taking:   s.Taking,
		Over:     s.Over,
		Loser:    relative(s.Loser, seat),
		Status:   s.Status,
		Rules:    s.Rules,
		Ply:      s.Ply,
	}
	v.Table = s.Clone().Table // a deep copy of the pairs
	return v
}

// placeholder returns a stand-in for a card the receiver can't see. It is
// drawn face down and never compared with real cards; its rank is far
// outside 6..A so it can't be mistaken for one.
func placeholder(i int) cards.Card { return cards.Card{Rank: cards.Rank(100 + i)} }

// IsPlaceholder reports whether c stands in for an unseen card.
func IsPlaceholder(c cards.Card) bool { return c.Rank >= 100 }

// FromView rebuilds a State from a View, as the receiver sees it: their own
// hand is real; the opponent's hand holds the known cards plus face-down
// placeholders; the deck holds placeholders with the real trump at the
// bottom. The UI draws this "mirror" exactly like a local game, and can ask
// it rule questions (which cards are playable?) without asking the host.
func FromView(v View) *State {
	// Placeholders are numbered by how many unknown cards there are, not by
	// position, so picking up known cards doesn't renumber them (which would
	// look like new cards arriving to the animations).
	opp := slices.Clone(v.OppKnown)
	for i := range v.OppCount - len(v.OppKnown) {
		opp = append(opp, placeholder(i))
	}
	deck := make([]cards.Card, 0, v.DeckLen)
	if v.DeckLen > 0 {
		deck = append(deck, v.Trump)
		for i := 1; i < v.DeckLen; i++ {
			deck = append(deck, placeholder(50+i))
		}
	}
	s := &State{
		Rules:    v.Rules,
		Deck:     &cards.Deck{Cards: deck},
		Trump:    v.Trump,
		Players:  [2]*Player{{Name: v.Names[0], Hand: slices.Clone(v.Hand)}, {Name: v.Names[1], Hand: opp}},
		Table:    v.Table,
		Attacker: v.Attacker,
		Leader:   v.Leader,
		Turn:     v.Turn,
		Taking:   v.Taking,
		Discards: v.Discards,
		Ply:      v.Ply,
		Status:   v.Status,
		Over:     v.Over,
		Loser:    v.Loser,
	}
	s.Known[Opponent] = cards.SetOf(v.OppKnown...)
	s.SortHand(You)
	return s
}
