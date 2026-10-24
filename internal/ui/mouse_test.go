package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"durak/internal/cards"
	"durak/internal/game"
)

// ready returns a sized game where you attack first, with no animations.
func ready() Model {
	m := newYouFirst()
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m = next.(Model)
	m.anims = nil
	return m
}

func send(m Model, msg tea.Msg) Model {
	next, _ := m.Update(msg)
	m = next.(Model)
	m.anims = nil
	return m
}

func centreOf(m Model, i int) (int, int) {
	s := m.lastLayout.cards[m.game.Players[game.You].Hand[i]]
	return s.x + m.lastLayout.cardW/2, m.lastLayout.handY + 2
}

func TestHoverSelectsCard(t *testing.T) {
	m := ready()
	x, y := centreOf(m, 3)
	m = send(m, tea.MouseMotionMsg{X: x, Y: y})
	if m.cursor != 3 {
		t.Fatalf("hovering card 3 should select it, cursor=%d", m.cursor)
	}
}

func TestClickPlaysCard(t *testing.T) {
	m := ready()
	card := m.game.Players[game.You].Hand[2]
	x, y := centreOf(m, 2)
	m = send(m, tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	if len(m.game.Table) != 1 || m.game.Table[0].Attack != card {
		t.Fatalf("clicking %v should play it; table = %+v", card, m.game.Table)
	}
}

func TestEndTurnButton(t *testing.T) {
	m := ready()
	// Hold a pair, so after leading one seven you could still add the other
	// and the turn doesn't pass on by itself.
	m.game.Players[game.You].Hand = []cards.Card{
		{Rank: cards.Seven, Suit: cards.Spades}, {Rank: cards.Seven, Suit: cards.Clubs}, {Rank: cards.Ace, Suit: cards.Diamonds}}
	m = send(m, tea.WindowSizeMsg{Width: 100, Height: 40}) // refresh the layout
	x, y := centreOf(m, 0)
	m = send(m, tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft}) // attack
	if len(m.lastLayout.buttons) != 1 {
		t.Fatalf("want an End turn button, got %+v", m.lastLayout.buttons)
	}
	b := m.lastLayout.buttons[0]
	// The button really is drawn where its hitbox says.
	screen, _ := m.render()
	line := []rune(strings.Split(ansi.Strip(screen), "\n")[b.y])
	if got := string(line[b.x : b.x+b.w]); !strings.Contains(got, "End turn") {
		t.Fatalf("hitbox %+v covers %q, not the button", b, got)
	}
	m = send(m, tea.MouseClickMsg{X: b.x + 1, Y: b.y, Button: tea.MouseLeft})
	if m.game.Turn != game.Opponent {
		t.Errorf("clicking End turn should hand the move to the opponent")
	}
}

func TestMenuMouse(t *testing.T) {
	m := send(ready(), tea.KeyPressMsg{Code: 'm', Text: "m"})
	bx, by, _, _ := m.menuBox()
	m = send(m, tea.MouseClickMsg{X: bx + 5, Y: by + 4, Button: tea.MouseLeft}) // first item
	if m.game.Rules.PassOn {
		t.Error("clicking 'Pass the attack on' should switch it off")
	}
	m = send(m, tea.MouseClickMsg{X: 0, Y: 0, Button: tea.MouseLeft}) // outside
	if m.menuOpen {
		t.Error("clicking outside the menu should close it")
	}
}

// When the last attack is beaten and the attacker has nothing to add, the
// table stays visible until the advance pause is over.
func TestPauseBeforeTableClears(t *testing.T) {
	m := ready()
	m.game.Players[game.You].Hand = []cards.Card{{Rank: cards.Seven, Suit: cards.Spades}, {Rank: cards.Ace, Suit: cards.Diamonds}}
	m.game.Players[game.Opponent].Hand = []cards.Card{{Rank: cards.King, Suit: cards.Spades}, {Rank: cards.Six, Suit: cards.Clubs}}
	m.game.Trump = cards.Card{Rank: cards.Six, Suit: cards.Hearts}
	m.cursor = 0
	m = send(m, tea.KeyPressMsg{Code: tea.KeyEnter}) // lead 7♠; no other seven, so it passes on
	if m.game.Turn != game.Opponent {
		t.Fatal("setup: the opponent should be defending")
	}
	m.game.Rules.Difficulty = game.Normal
	m = send(m, opponentMoveMsg{}) // K♠ beats it; you can't add anything
	if len(m.game.Table) != 1 || m.game.Table[0].Defense == nil {
		t.Fatalf("the beaten pair should stay on the table for now: %+v", m.game.Table)
	}
	if !m.advancePending {
		t.Fatal("an advance should be scheduled")
	}
	m = send(m, advanceMsg{ply: m.game.Ply})
	if len(m.game.Table) != 0 {
		t.Fatal("after the pause the round should end")
	}
}
