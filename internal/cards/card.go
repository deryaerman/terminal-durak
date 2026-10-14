// Package cards holds the basic building blocks: suits, ranks, cards and the deck.
package cards

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

// String returns the card as short text, e.g. "Q♠", "10♥", "A♦".
//
// TODO(you) — Exercise 2:
// Because Card has a method `String() string`, it automatically satisfies Go's
// fmt.Stringer interface — no "implements" keyword needed. That means
// fmt.Println(card) will print "Q♠" instead of "{12 0}".
//
// Suggested approach: write two small helpers, (s Suit) Symbol() string
// returning "♠", "♥", "♦" or "♣", and (r Rank) Label() string returning
// "6".."10", "J", "Q", "K", "A". A `switch` works nicely for both, and
// strconv.Itoa(int(r)) turns a number into a string.
func (c Card) String() string {
	panic("TODO(you): implement Card.String")
}
