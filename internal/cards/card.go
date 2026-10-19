// Package cards holds the basic building blocks: suits, ranks, cards and the deck.
package cards

import "strconv"

// Suit is one of the four card suits.
//
// In Go there is no `enum` keyword. Instead you declare a named type on top
// of int and list its values in a `const` block using `iota`, which starts at
// 0 and counts up by one on every line.
type Suit int

const (
	Spades Suit = iota
	Hearts
	Diamonds
	Clubs
)

// Rank is a card's value. Durak uses a 36-card deck: 6 through Ace.
type Rank int

const (
	Six Rank = iota + 6
	Seven
	Eight
	Nine
	Ten
	Jack
	Queen
	King
	Ace
)

// Card is a single playing card. Structs are Go's way of grouping fields.
type Card struct {
	Rank Rank
	Suit Suit
}

// IsRed reports whether the card is a heart or a diamond (used for colouring).
func (c Card) IsRed() bool {
	return c.Suit == Hearts || c.Suit == Diamonds
}

// Symbol returns the suit's character: "♠", "♥", "♦" or "♣".
func (s Suit) Symbol() string {
	switch s {
	case Spades:
		return "♠"
	case Hearts:
		return "♥"
	case Diamonds:
		return "♦"
	case Clubs:
		return "♣"
	default:
		return "?"
	}
}

// Label returns the rank as text: "6".."10", "J", "Q", "K" or "A".
func (r Rank) Label() string {
	switch r {
	case Jack:
		return "J"
	case Queen:
		return "Q"
	case King:
		return "K"
	case Ace:
		return "A"
	default:
		return strconv.Itoa(int(r))
	}
}

// String returns the card as short text, e.g. "Q♠", "10♥", "A♦".
//
// Because Card has a method `String() string`, it automatically satisfies Go's
// fmt.Stringer interface — no "implements" keyword needed. That means
// fmt.Println(card) prints "Q♠" instead of "{12 0}".
func (c Card) String() string {
	return c.Rank.Label() + c.Suit.Symbol()
}
