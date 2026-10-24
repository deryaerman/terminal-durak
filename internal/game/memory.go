package game

import (
	"math/rand/v2"

	"durak/internal/cards"
)

// Card counting: what player p can know about the cards they can't see.

// Unseen returns the cards whose location p doesn't know: they're either in
// the opponent's hand or in the deck. It excludes p's own hand, the table,
// discarded cards, cards known to be in the opponent's hand, and the trump
// card while it still lies face up under the deck.
func (s *State) Unseen(p int) cards.Set {
	known := cards.SetOf(s.Players[p].Hand...).
		Union(s.tableSet()).
		Union(s.Gone).
		Union(s.Known[1-p])
	if s.Deck.Len() > 0 {
		known = known.Add(s.Trump)
	}
	return cards.FullDeck.Minus(known)
}

// OpponentKnown is the set of cards p knows the opponent holds.
func (s *State) OpponentKnown(p int) cards.Set { return s.Known[1-p] }

// Determinize returns a copy of the game in which the cards p can't see have
// been dealt at random, consistently with everything p knows: the opponent
// keeps their known cards and their hand size, the deck keeps its size with
// the trump at the bottom. The bots play out imagined games in such copies.
func (s *State) Determinize(p int, rng *rand.Rand) *State {
	w := s.Clone()
	opp := 1 - p
	unseen := s.Unseen(p).Cards()
	rng.Shuffle(len(unseen), func(i, j int) { unseen[i], unseen[j] = unseen[j], unseen[i] })

	known := s.Known[opp].Cards()
	need := len(s.Players[opp].Hand) - len(known)
	hand := append(known, unseen[:need]...)
	w.Players[opp].Hand = hand

	deck := unseen[need:]
	if s.Deck.Len() > 0 {
		// The trump stays at the bottom (index 0); the rest is unknown order.
		deck = append([]cards.Card{s.Trump}, deck...)
	}
	w.Deck.Cards = deck
	return w
}
