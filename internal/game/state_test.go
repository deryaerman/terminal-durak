package game

import (
	"testing"

	"durak/internal/cards"
)

// fixed builds a state with chosen hands so tests don't depend on shuffling.
func fixed(trump cards.Suit, you, opp []cards.Card) *State {
	s := New()
	s.Trump = cards.Card{Rank: cards.Six, Suit: trump}
	s.Players[You].Hand = you
	s.Players[Opponent].Hand = opp
	s.Table = nil
	s.Attacker, s.Leader, s.Turn = You, You, You
	return s
}

func c(r cards.Rank, su cards.Suit) cards.Card { return cards.Card{Rank: r, Suit: su} }

func TestBeats(t *testing.T) {
	s := fixed(cards.Hearts, nil, nil)
	cases := []struct {
		def, att cards.Card
		want     bool
	}{
		{c(cards.King, cards.Spades), c(cards.Seven, cards.Spades), true},  // higher, same suit
		{c(cards.Six, cards.Spades), c(cards.Seven, cards.Spades), false},  // lower
		{c(cards.Six, cards.Hearts), c(cards.Ace, cards.Spades), true},     // trump beats non-trump
		{c(cards.Ace, cards.Clubs), c(cards.Seven, cards.Spades), false},   // other suit
		{c(cards.Seven, cards.Hearts), c(cards.Nine, cards.Hearts), false}, // lower trump
	}
	for _, tc := range cases {
		if got := s.Beats(tc.def, tc.att); got != tc.want {
			t.Errorf("Beats(%v, %v) = %v, want %v", tc.def, tc.att, got, tc.want)
		}
	}
}

func TestTransfer(t *testing.T) {
	you := []cards.Card{c(cards.Seven, cards.Spades), c(cards.Ace, cards.Clubs), c(cards.Six, cards.Clubs)}
	opp := []cards.Card{c(cards.Seven, cards.Diamonds), c(cards.Nine, cards.Clubs), c(cards.Ten, cards.Clubs)}
	s := fixed(cards.Hearts, you, opp)

	if err := s.Play(You, 0); err != nil { // you attack with 7♠
		t.Fatal(err)
	}
	if s.Turn != Opponent { // you have no other seven to add
		t.Fatal("with nothing left to add, the attack should pass to the opponent by itself")
	}
	// Opponent can't beat 7♠ with 7♦, so playing it passes the attack back.
	if err := s.Play(Opponent, 0); err != nil {
		t.Fatalf("transfer via Play failed: %v", err)
	}
	if s.Attacker != Opponent || s.Turn != You || len(s.Table) != 2 {
		t.Fatalf("after transfer: attacker=%d turn=%d table=%d", s.Attacker, s.Turn, len(s.Table))
	}
	// Now you must beat both sevens.
	if s.Undefended() != 0 {
		t.Fatalf("first attack should be undefended")
	}
}

func TestNoTransferAfterDefence(t *testing.T) {
	s := fixed(cards.Hearts, []cards.Card{c(cards.Seven, cards.Spades), c(cards.Seven, cards.Clubs)},
		[]cards.Card{c(cards.King, cards.Spades), c(cards.Seven, cards.Diamonds)})
	_ = s.Play(You, 0)
	_ = s.EndTurn()
	_ = s.Play(Opponent, 0) // K♠ beats 7♠
	if s.CanTransfer(c(cards.Seven, cards.Diamonds)) {
		t.Errorf("transfer must not be allowed once a card has been defended")
	}
}

func TestGameOver(t *testing.T) {
	s := fixed(cards.Hearts, []cards.Card{c(cards.Seven, cards.Spades)},
		[]cards.Card{c(cards.King, cards.Spades), c(cards.Six, cards.Clubs)})
	s.Deck.Cards = nil      // deck empty: the endgame
	_ = s.Play(You, 0)      // your last card; the turn passes on by itself
	_ = s.Play(Opponent, 0) // K♠ beats it, and with nothing to add the round ends
	if !s.Over || s.Loser != Opponent {
		t.Fatalf("Over=%v Loser=%d, want game over with the opponent as durak", s.Over, s.Loser)
	}
	if err := s.Play(Opponent, 0); err != ErrGameOver {
		t.Errorf("moves after the end should fail with ErrGameOver, got %v", err)
	}
}

// TestFullGamesFinish plays whole games with a simple policy for "You" and
// checks that every game reaches an end.
func TestFullGamesFinish(t *testing.T) {
	for g := 0; g < 200; g++ {
		s := New()
		s.Rules.Difficulty = Normal // Hard is too slow for 200 games here; arena_test covers it
		for step := 0; !s.Over; step++ {
			if step > 5000 {
				t.Fatalf("game %d never ended", g)
			}
			if s.Turn == Opponent {
				s.OpponentMove()
				continue
			}
			if s.Attacker == You {
				if len(s.Table) == 0 && len(s.Players[You].Hand) > 0 {
					_ = s.Play(You, 0)
				}
				_ = s.EndTurn()
				continue
			}
			played := false
			for i := range s.Players[You].Hand {
				if s.Play(You, i) == nil {
					played = true
					break
				}
			}
			switch {
			case !played:
				s.Take()
			case s.Turn == You && s.Undefended() < 0:
				_ = s.EndTurn()
			}
		}
	}
}

func TestAttackRanksMustMatch(t *testing.T) {
	you := []cards.Card{c(cards.Seven, cards.Spades), c(cards.Seven, cards.Clubs), c(cards.King, cards.Clubs), c(cards.Nine, cards.Diamonds)}
	opp := []cards.Card{c(cards.Nine, cards.Spades), c(cards.Six, cards.Clubs), c(cards.Ace, cards.Diamonds), c(cards.Ten, cards.Diamonds)}
	s := fixed(cards.Hearts, you, opp)

	if err := s.Play(You, 0); err != nil { // 7♠: first card, anything goes
		t.Fatal(err)
	}
	if err := s.Play(You, 1); err != ErrRankNotOnTable { // K♣: no king on the table
		t.Fatalf("K♣ after 7♠: got %v, want ErrRankNotOnTable", err)
	}
	if err := s.Play(You, 0); err != nil { // 7♣: sevens are on the table
		t.Fatalf("7♣ after 7♠: %v", err)
	}
	_ = s.EndTurn()
	_ = s.Play(Opponent, 0) // 9♠ beats 7♠
	_ = s.Play(Opponent, 0) // A♦... can't beat 7♣ (not trump) → error, ignore
	// 9♦ may now be added: a nine is on the table as a defense.
	s.Turn = You
	if !s.CanPlay(You, c(cards.Nine, cards.Diamonds)) {
		t.Errorf("9♦ should be playable once a nine is on the table")
	}
}

func TestAttackLimitedByDefenderHand(t *testing.T) {
	you := []cards.Card{c(cards.Seven, cards.Spades), c(cards.Seven, cards.Clubs), c(cards.Seven, cards.Diamonds)}
	opp := []cards.Card{c(cards.Ace, cards.Hearts), c(cards.Six, cards.Clubs)}
	s := fixed(cards.Hearts, you, opp)
	_ = s.Play(You, 0)
	_ = s.Play(You, 0)
	if err := s.checkAttack(c(cards.Seven, cards.Diamonds)); err != ErrTableFull {
		t.Errorf("third attack against a 2-card hand: got %v, want ErrTableFull", err)
	}
	if s.Turn != Opponent {
		t.Error("with no legal card left, the turn should pass to the defender by itself")
	}
}
