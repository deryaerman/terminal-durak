package ui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func key(k string) tea.KeyPressMsg {
	switch k {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	}
	return tea.KeyPressMsg{Code: []rune(k)[0], Text: k}
}

// driver plays the part of Bubble Tea's runtime in tests: it sends
// messages to the App and runs the commands that come back, each in its own
// goroutine. Results are fed back in as they arrive (pump), so commands that
// wait for the network or a timer work just like in the real program.
type driver struct {
	app     App
	results chan tea.Msg
}

func newDriver(a App) *driver { return &driver{app: a, results: make(chan tea.Msg, 256)} }

func (d *driver) send(msg tea.Msg) {
	next, cmd := d.app.Update(msg)
	d.app = next.(App)
	d.start(cmd)
}

func (d *driver) start(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	go func() { d.results <- cmd() }()
}

// pump delivers command results until cond holds or the time is up.
func (d *driver) pump(cond func(App) bool, wait time.Duration) bool {
	deadline := time.After(wait)
	for !cond(d.app) {
		select {
		case out := <-d.results:
			switch out := out.(type) {
			case nil:
			case tea.BatchMsg:
				for _, c := range out {
					d.start(c)
				}
			default:
				d.send(out)
			}
		case <-deadline:
			return false
		}
	}
	return true
}

// settle lets quick commands (dialling, first messages) finish.
func (d *driver) settle() { d.pump(func(App) bool { return false }, 150*time.Millisecond) }

func screenText(a App) string { return ansi.Strip(a.View().Content) }

func (d *driver) press(k string) { d.send(key(k)); d.settle() }

// choose opens the rules menu and picks the row with the given label.
func (d *driver) choose(t *testing.T, label string) {
	t.Helper()
	d.press("m")
	for i, it := range d.app.game.items() {
		if it.label == label {
			d.app.game.menuCursor = i
			d.press("enter")
			return
		}
	}
	t.Fatalf("no menu row %q", label)
}

func TestStartsAgainstTheComputer(t *testing.T) {
	d := newDriver(NewApp("Ann"))
	d.send(tea.WindowSizeMsg{Width: 100, Height: 40})
	if d.app.screen != screenGame || d.app.game.game == nil || d.app.game.net != nil {
		t.Fatal("the app should open straight into a game against the computer")
	}
	if !strings.Contains(screenText(d.app), "DECK") {
		t.Fatal("the table should be showing")
	}
}

func TestHostAndJoinFromTheMenu(t *testing.T) {
	host := newDriver(NewApp("Ann"))
	host.send(tea.WindowSizeMsg{Width: 100, Height: 40})
	host.choose(t, "Host a game on this Wi-Fi")
	if host.app.stopHost == nil || host.app.game.net == nil {
		t.Fatalf("hosting failed: %v", host.app.game.err)
	}
	defer func() {
		if host.app.stopHost != nil {
			host.app.stopHost()
		}
	}()
	if !strings.Contains(screenText(host.app), "Hosting as Ann") {
		t.Fatalf("host should see the waiting screen, got:\n%s", screenText(host.app))
	}

	// The guest opens Join, picks "Type an address…" and types it.
	guest := newDriver(NewApp("Bob"))
	guest.send(tea.WindowSizeMsg{Width: 100, Height: 40})
	guest.choose(t, "Join a game on this Wi-Fi")
	if guest.app.screen != screenJoin {
		t.Fatal("should be on the join screen")
	}
	guest.app.cursor = len(guest.app.hosts) // "Type an address…"
	guest.press("enter")
	lobby := host.app.game.lobby
	port := lobby[strings.LastIndex(lobby, ":")+1:]
	port = port[:strings.IndexFunc(port, func(r rune) bool { return r < '0' || r > '9' })]
	for _, ch := range "127.0.0.1:" + port {
		guest.send(key(string(ch)))
	}
	guest.send(key("enter"))
	if !guest.pump(func(a App) bool { return a.screen == screenGame && a.game.net != nil }, 5*time.Second) {
		t.Fatalf("guest didn't get into the game: %q", guest.app.note)
	}

	// Both receive the game from the server.
	started := func(a App) bool { return a.game.game != nil }
	if !host.pump(started, 5*time.Second) || !guest.pump(started, 5*time.Second) {
		t.Fatal("game didn't start")
	}

	// The host leaves through the menu: back to the computer, no more hosting.
	host.choose(t, "Leave network game")
	if host.app.game.net != nil || host.app.game.game == nil || host.app.stopHost != nil {
		t.Fatal("leaving should go back to a game against the computer and stop hosting")
	}
	if !guest.pump(func(a App) bool { return a.game.game == nil }, 5*time.Second) {
		t.Fatal("guest should notice the host has gone")
	}
}

// Escape on the hosting screen goes back to playing the computer and stops
// the server.
func TestEscapeFromHostLobby(t *testing.T) {
	d := newDriver(NewApp("Ann"))
	d.send(tea.WindowSizeMsg{Width: 100, Height: 40})
	d.choose(t, "Host a game on this Wi-Fi")
	if d.app.game.net == nil || d.app.game.game != nil {
		t.Fatal("should be on the hosting screen")
	}
	d.press("esc")
	if d.app.game.net != nil || d.app.game.game == nil || d.app.stopHost != nil {
		t.Fatal("esc should go back to a game against the computer and stop hosting")
	}
}
