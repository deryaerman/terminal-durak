package ui

import (
	"testing"
	"time"

	"durak/internal/game"
)

func TestTimerStartsOnYourTurn(t *testing.T) {
	m := New()
	if cmd := m.Init(); cmd == nil {
		t.Fatal("Init should start the timer: you move first")
	}
	m, _ = m.syncTimer()
	if m.timerOwner != game.You || m.timeLeft() <= TurnTime-2*time.Second {
		t.Fatalf("timer not running: owner %d, left %v", m.timerOwner, m.timeLeft())
	}
}

func TestTimerExpiryAdvancesGame(t *testing.T) {
	m := New()
	m, _ = m.syncTimer()
	m.deadline = time.Now().Add(-time.Second) // time is up

	next, _ := m.Update(timerTickMsg{id: m.timerID})
	got := next.(Model)
	if len(got.game.Table) != 1 || got.game.Turn != game.Opponent {
		t.Fatalf("expiry should auto-play and pass the turn: table %d, turn %d", len(got.game.Table), got.game.Turn)
	}
	if got.timerOwner != -1 {
		t.Errorf("timer should stop while the opponent moves")
	}
}

func TestStaleTickIgnored(t *testing.T) {
	m := New()
	m, _ = m.syncTimer()
	m.deadline = time.Now().Add(-time.Second)

	next, _ := m.Update(timerTickMsg{id: m.timerID - 1}) // from an old timer
	if len(next.(Model).game.Table) != 0 {
		t.Fatal("a stale tick must not trigger a timeout")
	}
}
