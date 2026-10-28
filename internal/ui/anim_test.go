package ui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"durak/internal/cards"
	"durak/internal/game"
)

// cellAt returns the character at column x of line y of the plain screen.
func cellAt(screen []string, x, y int) string {
	if y < 0 || y >= len(screen) {
		return ""
	}
	runes := []rune(screen[y])
	if x < 0 || x >= len(runes) {
		return ""
	}
	return string(runes[x])
}

// TestLayoutMatchesScreen checks that computeLayout's positions match where
// render() really draws each card, across many game situations and sizes:
// a face-up card's rank sits just inside its top-left corner, a face-down
// card's corner sits exactly on its spot.
func TestLayoutMatchesScreen(t *testing.T) {
	sizes := [][2]int{{80, 30}, {100, 40}, {140, 50}, {91, 33}}
	for _, sz := range sizes {
		m := New()
		m.game.Rules.Difficulty = game.Normal // fast: this test is about positions, not play
		m.width, m.height = sz[0], sz[1]
		for step := 0; step < 60 && !m.game.Over; step++ {
			m.anims = nil
			screen, lay := m.render()
			lines := strings.Split(ansi.Strip(screen), "\n")
			for c, s := range lay.cards {
				if s.faceUp {
					if got := cellAt(lines, s.x+1, s.y+1); got != c.Rank.Label()[:1] {
						t.Log("\n" + strings.Join(lines, "\n"))
						for _, p := range m.game.Table {
							t.Logf("pair %v / %v", p.Attack, p.Defense)
						}
						t.Fatalf("%dx%d step %d: %v (zone %d) at %d,%d: found %q",
							sz[0], sz[1], step, c, s.zone, s.x, s.y, got)
					}
				} else if got := cellAt(lines, s.x, s.y); got != "┏" {
					t.Fatalf("%dx%d step %d: face-down %v at %d,%d: found %q", sz[0], sz[1], step, c, s.x, s.y, got)
				}
			}
			// Advance the game a little: both players use the simple bot.
			m.game.AutoMove(m.game.Turn)
			m.cursor = step % max(1, len(m.game.Players[game.You].Hand))
		}
	}
}

func TestOpeningDealAnimates(t *testing.T) {
	m := New()
	next, cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m = next.(Model)
	if len(m.anims) != 13 || cmd == nil { // 12 cards + the trump
		t.Fatalf("want 13 dealing animations, got %d", len(m.anims))
	}
	if !m.hiddenCards()[m.game.Players[game.You].Hand[0]] {
		t.Error("cards in flight should be hidden at their destination")
	}
	// After everything lands, the loop stops.
	for i := range m.anims {
		m.anims[i].start = time.Now().Add(-time.Second)
	}
	next, _ = m.Update(frameMsg{})
	if m = next.(Model); len(m.anims) != 0 || m.animating {
		t.Errorf("animations should be done: %d left, animating=%v", len(m.anims), m.animating)
	}
}

func TestPlayAnimatesToTable(t *testing.T) {
	m := newYouFirst()
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m = next.(Model)
	m.anims = nil
	card := m.game.Players[game.You].Hand[m.cursor]
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(Model)
	if len(m.anims) != 1 || m.anims[0].card != card || m.anims[0].to.zone != zoneTable {
		t.Fatalf("want one flight of %v to the table, got %+v", card, m.anims)
	}
}

// Six attacks, all defended, at the medium card size used to overflow the
// table and garble the screen. Every size must fit and match the layout.
func TestSixDefendedPilesFit(t *testing.T) {
	mk := func(r cards.Rank, s cards.Suit) cards.Card { return cards.Card{Rank: r, Suit: s} }
	for _, sz := range [][2]int{{80, 30}, {100, 40}, {140, 50}} {
		m := New()
		m.width, m.height = sz[0], sz[1]
		m.game.Trump = mk(cards.Six, cards.Diamonds)
		attacks := []cards.Card{mk(cards.Eight, cards.Spades), mk(cards.Eight, cards.Clubs), mk(cards.Ten, cards.Clubs),
			mk(cards.Jack, cards.Hearts), mk(cards.King, cards.Spades), mk(cards.Ace, cards.Clubs)}
		defs := []cards.Card{mk(cards.Seven, cards.Diamonds), mk(cards.Ten, cards.Diamonds), mk(cards.Jack, cards.Diamonds),
			mk(cards.King, cards.Diamonds), mk(cards.Ace, cards.Diamonds), mk(cards.Nine, cards.Diamonds)}
		m.game.Table = nil
		for i := range attacks {
			d := defs[i]
			m.game.Table = append(m.game.Table, game.Pair{Attack: attacks[i], Defense: &d})
		}
		m.game.Players[game.You].Hand = nil
		m.game.Players[game.Opponent].Hand = []cards.Card{mk(cards.Six, cards.Spades)}
		screen, lay := m.render()
		lines := strings.Split(ansi.Strip(screen), "\n")
		for _, l := range lines {
			if w := len([]rune(l)); w > sz[0] {
				t.Fatalf("%dx%d: a line is %d wide", sz[0], sz[1], w)
			}
		}
		for c, s := range lay.cards {
			if s.zone == zoneTable && cellAt(lines, s.x+1, s.y+1) != c.Rank.Label()[:1] {
				t.Fatalf("%dx%d: %v not drawn where the layout says\n%s", sz[0], sz[1], c, strings.Join(lines, "\n"))
			}
		}
	}
}
