package game

import (
	"testing"

	"durak/internal/cards"
)

func TestLowestTrumpAttacksFirst(t *testing.T) {
	s := New()
	s.Trump = c(cards.Six, cards.Hearts)
	s.Players[You].Hand = []cards.Card{c(cards.Nine, cards.Hearts), c(cards.Ace, cards.Spades)}
	s.Players[Opponent].Hand = []cards.Card{c(cards.Seven, cards.Hearts), c(cards.Six, cards.Spades)}
	if got := s.lowestTrumpHolder(); got != Opponent {
		t.Errorf("7♥ is the lowest trump, so the opponent starts; got player %d", got)
	}
}

func TestDefenderMustBeatOrTake(t *testing.T) {
	s := fixed(cards.Hearts, []cards.Card{c(cards.Seven, cards.Spades)},
		[]cards.Card{c(cards.Six, cards.Clubs)})
	_ = s.Play(You, 0)
	_ = s.EndTurn()
	if err := s.EndTurn(); err != ErrMustBeatOrTake {
		t.Fatalf("defender ending with an unbeaten card: got %v, want ErrMustBeatOrTake", err)
	}
}

func TestThrowInAfterTake(t *testing.T) {
	you := []cards.Card{c(cards.Seven, cards.Spades), c(cards.Seven, cards.Clubs), c(cards.Ace, cards.Clubs)}
	opp := []cards.Card{c(cards.Six, cards.Diamonds), c(cards.Eight, cards.Diamonds), c(cards.Nine, cards.Diamonds)}
	s := fixed(cards.Hearts, you, opp)
	_ = s.Play(You, 0) // 7♠
	_ = s.EndTurn()
	s.Take() // opponent gives up
	if !s.Taking || s.Turn != You {
		t.Fatalf("attacker should get to add cards: taking=%v turn=%d", s.Taking, s.Turn)
	}
	if err := s.Play(You, 0); err != nil { // 7♣ thrown in
		t.Fatal(err)
	}
	if err := s.Play(Opponent, 0); err != ErrNotYourTurn && err != ErrTaking {
		t.Errorf("a taking defender can't defend, got %v", err)
	}
	_ = s.EndTurn()
	if len(s.Table) != 0 || s.Attacker != You || len(s.Players[Opponent].Hand) < 5 {
		t.Fatalf("opponent should pick up both sevens and you attack again: table %d attacker %d opp hand %d",
			len(s.Table), s.Attacker, len(s.Players[Opponent].Hand))
	}
}

func TestLeaderDrawsFirstAfterPassOn(t *testing.T) {
	you := []cards.Card{c(cards.Seven, cards.Spades), c(cards.King, cards.Diamonds), c(cards.King, cards.Clubs)}
	opp := []cards.Card{c(cards.Seven, cards.Diamonds), c(cards.Ace, cards.Spades)}
	s := fixed(cards.Hearts, you, opp)
	last := c(cards.Ten, cards.Hearts)
	s.Deck.Cards = []cards.Card{last} // one card left
	_ = s.Play(You, 0)                // you lead 7♠
	_ = s.EndTurn()
	_ = s.Play(Opponent, 0) // 7♦ passes it back: you now defend two sevens
	s.Take()                // you take them…
	_ = s.EndTurn()         // …and the opponent, now attacking, adds nothing
	// You led the round, so you draw the last card even though the
	// opponent was the attacker when it ended.
	found := false
	for _, h := range s.Players[You].Hand {
		found = found || h == last
	}
	if !found {
		t.Errorf("the round's leader (you) should draw the last card first")
	}
}

func TestPassOnToggle(t *testing.T) {
	s := fixed(cards.Hearts, []cards.Card{c(cards.Seven, cards.Spades), c(cards.Ace, cards.Clubs)},
		[]cards.Card{c(cards.Seven, cards.Diamonds), c(cards.Six, cards.Clubs)})
	s.Rules.PassOn = false
	_ = s.Play(You, 0)
	_ = s.EndTurn()
	if s.CanTransfer(c(cards.Seven, cards.Diamonds)) {
		t.Error("with PassOn off, passing on must not be allowed")
	}
}

func TestPassOnMany(t *testing.T) {
	you := []cards.Card{c(cards.Seven, cards.Spades), c(cards.Ace, cards.Clubs), c(cards.Ace, cards.Spades), c(cards.Ace, cards.Diamonds)}
	opp := []cards.Card{c(cards.Seven, cards.Diamonds), c(cards.Seven, cards.Clubs), c(cards.Six, cards.Clubs)}
	s := fixed(cards.Hearts, you, opp)
	s.Rules.PassOnMany = true
	_ = s.Play(You, 0)
	_ = s.EndTurn()
	if err := s.Transfer(Opponent, 0); err != nil { // 7♦
		t.Fatal(err)
	}
	if s.Turn != Opponent {
		t.Fatalf("with PassOnMany the passer stays on the move to add more")
	}
	if err := s.Play(Opponent, 0); err != nil { // 7♣ as well
		t.Fatalf("adding a second seven: %v", err)
	}
	_ = s.EndTurn()
	if s.Turn != You || len(s.Table) != 3 {
		t.Fatalf("you should now face three sevens: turn %d table %d", s.Turn, len(s.Table))
	}
}

func TestAutoAdvance(t *testing.T) {
	// You lead 7♠ and still hold a 7, so you stay on the move...
	you := []cards.Card{c(cards.Seven, cards.Spades), c(cards.Seven, cards.Clubs), c(cards.Ace, cards.Diamonds)}
	opp := []cards.Card{c(cards.King, cards.Spades), c(cards.King, cards.Clubs), c(cards.Six, cards.Diamonds)}
	s := fixed(cards.Hearts, you, opp)
	_ = s.Play(You, 0)
	if s.Turn != You {
		t.Fatal("you can still add 7♣, so it should stay your turn")
	}
	// ...after adding it you have nothing left that fits: the turn passes on.
	_ = s.Play(You, 0)
	if s.Turn != Opponent {
		t.Fatal("no more sevens: the turn should pass to the defender")
	}
	// The opponent beats both; you can't add anything, so the round ends
	// without anyone pressing space.
	_ = s.Play(Opponent, 0) // K♠ on 7♠
	_ = s.Play(Opponent, 0) // K♣ on 7♣
	if len(s.Table) != 0 || s.Attacker != Opponent {
		t.Fatalf("round should end by itself: table %d, attacker %d", len(s.Table), s.Attacker)
	}
}

func TestDefenderTurnEndsWhenAllBeaten(t *testing.T) {
	// You can still throw in a king after the defence, so the round goes on,
	// but the defender has nothing left to do: the move comes back to you.
	you := []cards.Card{c(cards.Seven, cards.Spades), c(cards.King, cards.Diamonds)}
	opp := []cards.Card{c(cards.King, cards.Spades), c(cards.Six, cards.Clubs), c(cards.Six, cards.Diamonds)}
	s := fixed(cards.Hearts, you, opp)
	_ = s.Play(You, 0)      // 7♠ — no other seven, passes on by itself
	_ = s.Play(Opponent, 0) // K♠
	if s.Turn != You || len(s.Table) != 1 {
		t.Fatalf("you may add K♦, so it should be your move: turn %d table %d", s.Turn, len(s.Table))
	}
}
