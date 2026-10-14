package cards

import (
	"errors"
	"testing"
)

// Run with: go test ./...
// Go finds tests by name: any func TestXxx(t *testing.T) in a _test.go file.

func TestRankOrder(t *testing.T) {
	if Six != 6 || Ace != 14 {
		t.Fatalf("want Six=6 and Ace=14, got Six=%d Ace=%d", Six, Ace)
	}
	if Clubs != 3 {
		t.Fatalf("want 4 suits Spades..Clubs (Clubs=3), got Clubs=%d", Clubs)
	}
}

func TestCardString(t *testing.T) {
	// A "table-driven test": a slice of cases looped over, very common in Go.
	cases := []struct {
		card Card
		want string
	}{
		{Card{Queen, Spades}, "Q♠"},
		{Card{Ten, Hearts}, "10♥"},
		{Card{Ace, Diamonds}, "A♦"},
		{Card{Six, Clubs}, "6♣"},
	}
	for _, c := range cases {
		if got := c.card.String(); got != c.want {
			t.Errorf("String() = %q, want %q", got, c.want)
		}
	}
}

func TestNewDeck(t *testing.T) {
	d := NewDeck()
	if d.Len() != 36 {
		t.Fatalf("deck has %d cards, want 36", d.Len())
	}
	seen := map[Card]bool{}
	for _, c := range d.Cards {
		if seen[c] {
			t.Fatalf("duplicate card %v", c)
		}
		if c.Rank < Six || c.Rank > Ace || c.Suit < Spades || c.Suit > Clubs {
			t.Fatalf("invalid card %+v", c)
		}
		seen[c] = true
	}
}

func TestDraw(t *testing.T) {
	d := NewDeck()
	top := d.Cards[len(d.Cards)-1]
	hand, err := d.Draw(6)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hand) != 6 || d.Len() != 30 {
		t.Fatalf("drew %d, %d left; want 6 drawn and 30 left", len(hand), d.Len())
	}
	found := false
	for _, c := range hand {
		found = found || c == top
	}
	if !found {
		t.Errorf("Draw should take from the top (end of the slice)")
	}
	if _, err := d.Draw(31); !errors.Is(err, ErrNotEnoughCards) {
		t.Errorf("drawing too many: got err %v, want ErrNotEnoughCards", err)
	}
	if d.Len() != 30 {
		t.Errorf("a failed Draw must leave the deck untouched, %d left", d.Len())
	}
}
