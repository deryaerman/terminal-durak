package ui

import (
	"context"
	"slices"
	"strconv"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"durak/internal/game"
	"durak/internal/netplay"
)

// TestRemoteGameThroughTheUI connects two UIs to a real server and plays a
// whole game by pressing keys, the way people would. Every screen is drawn
// along the way, so any rendering problem with placeholders shows up here.
func TestRemoteGameThroughTheUI(t *testing.T) {
	srv, err := netplay.Listen("127.0.0.1:0", game.DefaultRules, 0)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.Run(ctx)
	addr := "127.0.0.1:" + strconv.Itoa(srv.Port())

	var players [2]Model
	for i, name := range []string{"Ann", "Bob"} {
		c, err := netplay.Dial(addr, name)
		if err != nil {
			t.Fatal(err)
		}
		defer c.Close()
		players[i] = NewRemote(c, "lobby")
		next, _ := players[i].Update(tea.WindowSizeMsg{Width: 100, Height: 40})
		players[i] = next.(Model)
	}

	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		for i := range players {
			m := players[i]
			// Deliver whatever the server sent.
			for more := true; more; {
				select {
				case msg, ok := <-m.net.Incoming:
					if !ok {
						t.Fatal("connection closed")
					}
					next, _ := m.Update(netMsg{Message: msg, from: m.net})
					m = next.(Model)
				case <-time.After(20 * time.Millisecond):
					more = false
				}
			}
			_ = m.View() // must draw without trouble, placeholders and all
			m.anims = nil
			if m.game != nil && m.game.Over {
				players[i] = m
				if players[1-i].game != nil && players[1-i].game.Over {
					return // both saw the end
				}
				continue
			}
			if m.game != nil && m.game.Turn == game.You && !m.game.Stuck() {
				move := m.game.HeuristicMove(game.You)
				var key string
				switch move.Kind {
				case game.PlayCard:
					m.cursor = slices.Index(m.game.Players[game.You].Hand, move.Card)
					key = "enter"
				case game.PassOn:
					m.cursor = slices.Index(m.game.Players[game.You].Hand, move.Card)
					key = "r"
				case game.TakeCards:
					key = "t"
				default:
					key = "space"
				}
				next, _ := m.Update(tea.KeyPressMsg{Text: key, Code: []rune(key)[0]})
				if key == "enter" {
					next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
				} else if key == "space" {
					next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
				}
				m = next.(Model)
			}
			players[i] = m
		}
	}
	t.Fatal("the network game didn't finish in time")
}
