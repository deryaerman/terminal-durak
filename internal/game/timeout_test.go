package game

import "testing"

func TestTimeoutAttackerLeads(t *testing.T) {
	s := youFirst(New())
	before := len(s.Players[You].Hand)
	s.Timeout(You)
	if len(s.Table) != 1 || len(s.Players[You].Hand) != before-1 {
		t.Fatalf("timeout should play one card: table %d, hand %d", len(s.Table), len(s.Players[You].Hand))
	}
	if s.Turn != Opponent {
		t.Errorf("turn should pass to the defender")
	}
}

func TestTimeoutDefenderTakes(t *testing.T) {
	s := NewWithRules(Rules{PassOn: true}) // no throw-in after take: collect at once
	s.Attacker, s.Turn = Opponent, Opponent
	s.AutoMove(Opponent) // opponent attacks with one card
	if s.Turn != You || len(s.Table) != 1 {
		t.Fatalf("setup: want your move with 1 attack, got turn %d table %d", s.Turn, len(s.Table))
	}
	before := len(s.Players[You].Hand)
	s.Timeout(You)
	if len(s.Table) != 0 || len(s.Players[You].Hand) != before+1 {
		t.Fatalf("defender should take the card: table %d, hand %d (was %d)", len(s.Table), len(s.Players[You].Hand), before)
	}
	if s.Attacker != Opponent || s.Turn != Opponent {
		t.Errorf("after taking, the same attacker goes again")
	}
}

func TestTimeoutAttackerEndsRound(t *testing.T) {
	s := youFirst(New())
	c := s.Players[You].Hand[0]
	def := s.Players[Opponent].Hand[0]
	s.Players[You].Hand = s.Players[You].Hand[1:]
	s.Players[Opponent].Hand = s.Players[Opponent].Hand[1:]
	s.Table = []Pair{{Attack: c, Defense: &def}} // everything beaten
	s.Timeout(You)
	if len(s.Table) != 0 || s.Attacker != Opponent {
		t.Fatalf("round should end and the defender attack next: table %d, attacker %d", len(s.Table), s.Attacker)
	}
}

func TestTimeoutIgnoresWrongPlayer(t *testing.T) {
	s := youFirst(New())
	s.Timeout(Opponent)
	if len(s.Table) != 0 || s.Turn != You {
		t.Errorf("a timeout for the player not on the move must do nothing")
	}
}

// youFirst makes you the attacker, whoever held the lowest trump.
func youFirst(s *State) *State {
	s.Attacker, s.Leader, s.Turn = You, You, You
	return s
}
