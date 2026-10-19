package ui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func press(m Model, keys ...tea.KeyPressMsg) Model {
	for _, k := range keys {
		next, _ := m.Update(k)
		m = next.(Model)
	}
	return m
}

var (
	keyM     = tea.KeyPressMsg{Code: 'm', Text: "m"}
	keyDown  = tea.KeyPressMsg{Code: tea.KeyDown}
	keyEnter = tea.KeyPressMsg{Code: tea.KeyEnter}
	keyLeft  = tea.KeyPressMsg{Code: tea.KeyLeft}
)

func TestMenuTogglesRules(t *testing.T) {
	m := press(newYouFirst(), keyM)
	if !m.menuOpen {
		t.Fatal("m should open the menu")
	}
	m = press(m, keyEnter)               // pass-on off
	m = press(m, keyDown, keyEnter)      // several cards: disabled now, no change
	m = press(m, keyDown, keyEnter)      // throw-in after take off
	m = press(m, keyDown, keyLeft, keyM) // timer 30s → 15s, close
	r := m.game.Rules
	if r.PassOn || r.PassOnMany || r.ThrowInAfterTake {
		t.Errorf("rules after toggling: %+v", r)
	}
	if m.menuOpen || m.turnTime != 15*time.Second {
		t.Errorf("menu open=%v turnTime=%v, want closed and 15s", m.menuOpen, m.turnTime)
	}
}

func TestMenuPausesTimer(t *testing.T) {
	m := newYouFirst()
	m, _ = m.syncTimer()
	m.deadline = time.Now().Add(10 * time.Second)
	m = press(m, keyM)
	m.paused = 10 * time.Second
	next, _ := m.Update(timerTickMsg{id: m.timerID})
	m = next.(Model)
	if len(m.game.Table) != 0 {
		t.Fatal("no timeout while the menu is open")
	}
	m = press(m, keyM)
	if left := m.timeLeft(); left < 9*time.Second || left > 10*time.Second {
		t.Errorf("timer should resume with ~10s left, got %v", left)
	}
}
