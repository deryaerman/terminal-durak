package netplay

import (
	"bufio"
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"durak/internal/cards"
	"durak/internal/game"
)

// ── Your part: message.go ──────────────────────────────────────────────

func TestMessageRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	sent := []Message{
		{Type: TypeHello, Name: "Derya"},
		{Type: TypeMove, Move: &game.Move{Kind: game.PlayCard, Card: cards.Card{Rank: cards.Queen, Suit: cards.Spades}}},
		{Type: TypeError, Error: "it's not your turn"},
	}
	for _, m := range sent {
		if err := WriteMessage(&buf, m); err != nil {
			t.Fatalf("WriteMessage: %v", err)
		}
	}
	if lines := strings.Count(buf.String(), "\n"); lines != len(sent) {
		t.Fatalf("want one line per message (%d), got %d newlines:\n%s", len(sent), lines, buf.String())
	}
	r := bufio.NewReader(&buf)
	for i, want := range sent {
		got, err := ReadMessage(r)
		if err != nil {
			t.Fatalf("ReadMessage #%d: %v", i, err)
		}
		if got.Type != want.Type || got.Name != want.Name || got.Error != want.Error ||
			(want.Move != nil && (got.Move == nil || *got.Move != *want.Move)) {
			t.Fatalf("message #%d: sent %+v, got %+v", i, want, got)
		}
	}
	if _, err := ReadMessage(r); err == nil {
		t.Fatal("reading past the last message should fail (io.EOF)")
	}
}

// ── Your part: discovery.go ────────────────────────────────────────────

func TestDiscovery(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	found, err := Discover(ctx)
	if err != nil {
		t.Skipf("can't listen for UDP here: %v", err)
	}
	go Announce(ctx, "Derya", 4747)
	select {
	case h, ok := <-found:
		if !ok {
			t.Fatal("discovery stopped without finding the host")
		}
		if h.Name != "Derya" || !strings.HasSuffix(h.Addr, ":4747") {
			t.Fatalf("found %+v, want Derya on port 4747", h)
		}
	case <-ctx.Done():
		t.Skip("no announcement heard within 5 s. Check that Announce writes the packet; " +
			"if it does, this network may block broadcasts (on a normal home Wi-Fi it should pass)")
	}
}

// ── The server, end to end ─────────────────────────────────────────────

// player connects and plays whole games with the Normal bot's choices,
// using only its own View, like a real client.
func player(t *testing.T, addr, name string, done chan<- game.View) {
	c, err := Dial(addr, name)
	if err != nil {
		t.Error(err)
		return
	}
	defer c.Close()
	for m := range c.Incoming {
		if m.Type == TypeError {
			continue // e.g. "round is ending", just wait for the next view
		}
		if m.Type != TypeView {
			continue
		}
		v := *m.View
		if v.Over {
			done <- v
			return
		}
		if v.Turn != game.You {
			continue
		}
		mirror := game.FromView(v)
		moves := mirror.LegalMoves(game.You)
		if len(moves) == 0 || mirror.Stuck() {
			continue
		}
		move := mirror.HeuristicMove(game.You)
		if err := c.Move(move); err != nil {
			t.Error(err)
			return
		}
	}
}

func TestTwoPlayersOverTCP(t *testing.T) {
	srv, err := Listen("127.0.0.1:0", game.DefaultRules, 0)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.Run(ctx)

	addr := "127.0.0.1:" + itoa(srv.Port())
	done := make(chan game.View, 2)
	go player(t, addr, "Ann", done)
	go player(t, addr, "Bob", done)

	var results []game.View
	timeout := time.After(30 * time.Second) // (hangs until message.go is filled in)
	for len(results) < 2 {
		select {
		case v := <-done:
			results = append(results, v)
		case <-timeout:
			t.Fatal("the game didn't finish")
		}
	}
	// Both saw the same ending, from opposite sides.
	a, b := results[0], results[1]
	if a.Loser != game.Draw && a.Loser != 1-b.Loser {
		t.Fatalf("players disagree on the loser: %d vs %d", a.Loser, b.Loser)
	}
}

func TestThirdPlayerIsTurnedAway(t *testing.T) {
	srv, err := Listen("127.0.0.1:0", game.DefaultRules, 0)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.Run(ctx)
	addr := "127.0.0.1:" + itoa(srv.Port())
	for _, n := range []string{"Ann", "Bob"} {
		c, err := Dial(addr, n)
		if err != nil {
			t.Fatal(err)
		}
		defer c.Close()
		<-c.Incoming // welcome
	}
	c, err := Dial(addr, "Cem")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if m := <-c.Incoming; m.Type != TypeError {
		t.Fatalf("third player got %+v, want an error", m)
	}
}
