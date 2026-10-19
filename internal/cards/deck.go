package cards

import (
	"errors"
	"math/rand/v2"
	"slices"
)

// ErrNotEnoughCards is returned by Draw when the deck runs out.
// Go returns errors as ordinary values instead of throwing exceptions.
var ErrNotEnoughCards = errors.New("not enough cards in the deck")

// Deck is an ordered pile of cards; the end of the slice is the top.
type Deck struct {
	Cards []Card
}

// NewDeck returns a full, unshuffled 36-card Durak deck.
//
// TODO(you) — Exercise 3a:
// Build a slice holding every combination of the 4 suits and the 9 ranks.
// Useful pieces:
//   - make([]Card, 0, 36) creates an empty slice with room for 36 cards
//   - append(slice, item) returns the slice with item added
//   - for s := Spades; s <= Clubs; s++ { ... } loops over the suits
func NewDeck() *Deck {
	cards := make([]Card, 0, 36)
	for s := Spades; s <= Clubs; s++ {
		for n := Six; n <= Ace; n++ {
			cards = append(cards, Card{n, s})
		}
	}
	return &Deck{Cards: cards}
}

// Shuffle puts the cards in random order.
func (d *Deck) Shuffle() {
	rand.Shuffle(len(d.Cards), func(i, j int) {
		d.Cards[i], d.Cards[j] = d.Cards[j], d.Cards[i]
	})
}

// Len returns how many cards are left.
func (d *Deck) Len() int { return len(d.Cards) }

// Draw removes n cards from the top (the end of the slice) and returns them.
// If the deck holds fewer than n cards it returns nil and ErrNotEnoughCards,
// and leaves the deck untouched.
//
// TODO(you) — Exercise 3b:
// Notice the two return values: Go functions often return (result, error).
// The receiver is *Deck (a pointer), so changes you make to d.Cards stick.
// Slicing tip: s[a:b] gives the elements from index a up to (not including) b.
func (d *Deck) Draw(n int) ([]Card, error) {
	if d.Len() < n {
		return nil, ErrNotEnoughCards
	}
	// Clone copies the cards into a new slice. Without it, the drawn cards
	// would still share memory with the deck, and two hands drawn one after
	// the other would sit side by side in the same array: appending to one
	// hand would overwrite the other.
	drawn := slices.Clone(d.Cards[d.Len()-n:])
	d.Cards = d.Cards[:d.Len()-n]
	return drawn, nil

}
