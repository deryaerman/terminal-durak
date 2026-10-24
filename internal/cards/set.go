package cards

import "math/bits"

// Set is a set of cards packed into the bits of one number: card i of the
// 36-card deck is bit i. Copying, comparing and combining sets is then a
// single machine instruction, which the bot's search relies on.
type Set uint64

// Index gives every card a number from 0 to 35.
func (c Card) Index() int { return int(c.Suit)*9 + int(c.Rank-Six) }

// FromIndex is the inverse of Index.
func FromIndex(i int) Card { return Card{Rank: Six + Rank(i%9), Suit: Suit(i / 9)} }

// FullDeck is the set of all 36 cards.
const FullDeck Set = 1<<36 - 1

// SetOf builds a set from cards.
func SetOf(cs ...Card) Set {
	var s Set
	for _, c := range cs {
		s = s.Add(c)
	}
	return s
}

func (s Set) Add(c Card) Set      { return s | 1<<c.Index() }
func (s Set) Remove(c Card) Set   { return s &^ (1 << c.Index()) }
func (s Set) Has(c Card) bool     { return s&(1<<c.Index()) != 0 }
func (s Set) Len() int            { return bits.OnesCount64(uint64(s)) }
func (s Set) Union(o Set) Set     { return s | o }
func (s Set) Minus(o Set) Set     { return s &^ o }
func (s Set) Intersect(o Set) Set { return s & o }

// Cards lists the cards in the set, in index order.
func (s Set) Cards() []Card {
	out := make([]Card, 0, s.Len())
	for s != 0 {
		i := bits.TrailingZeros64(uint64(s))
		out = append(out, FromIndex(i))
		s &= s - 1 // clear the lowest set bit
	}
	return out
}
