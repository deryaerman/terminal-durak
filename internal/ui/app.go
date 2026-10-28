package ui

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"durak/internal/game"
	"durak/internal/netplay"
)

// The App is the whole program as started with plain `durak`: it opens
// straight into a game against the computer, and the rules menu (m) offers
// to host or join a game on the Wi-Fi. Joining shows a screen listing the
// games found on the network. The App owns the long-running network pieces,
// the hosted server and the discovery, and shuts them down when you leave.
//
// The command-line `durak host` / `durak join` run a network Model directly;
// this is the same thing reached through the menu.

type screen int

const (
	screenGame screen = iota
	screenJoin
)

// App is the top-level Bubble Tea model.
type App struct {
	screen        screen
	game          Model
	name          string
	width, height int
	cursor        int    // selected row on the join screen
	note          string // a message under the menu (errors, hints)

	// Settings carried between games (changed in the in-game rules menu).
	rules    game.Rules
	turnTime time.Duration

	// Join screen.
	hosts        []netplay.Host
	stopDiscover context.CancelFunc
	typing       bool   // typing an address by hand
	addr         string // what has been typed so far
	connecting   bool

	// A game we host: cancelling stops the server and the announcements.
	stopHost context.CancelFunc
}

// NewApp starts a game against the computer. name is shown to other players
// in network games.
func NewApp(name string) App {
	a := App{name: name, width: 100, height: 30, rules: game.DefaultRules, turnTime: TurnTime}
	a.game = newGame(a.rules, a.turnTime)
	return a
}

func (a App) Init() tea.Cmd { return a.game.Init() }

// playComputer switches to a fresh game against the computer. note, if
// given, appears in its status line (e.g. why hosting failed).
func (a App) playComputer(note string) (App, tea.Cmd) {
	a.screen = screenGame
	a.game = newGame(a.rules, a.turnTime).withSize(a.width, a.height)
	if note != "" {
		a.game.err = errors.New(note)
	}
	return a, a.game.Init()
}

// Messages of the start and join screens.
type (
	hostFoundMsg struct {
		host netplay.Host
		ok   bool // false: discovery has stopped
		ch   <-chan netplay.Host
	}
	connectedMsg struct {
		client *netplay.Client
		addr   string
		err    error
	}
)

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		a.width, a.height = ws.Width, ws.Height
	}
	if a.screen == screenJoin {
		return a.updateJoin(msg)
	}
	return a.updateGame(msg)
}

// host starts a server with the current rules, announces it, and joins it.
func (a App) host() (tea.Model, tea.Cmd) {
	srv, err := netplay.Listen(":"+strconv.Itoa(netplay.DefaultPort), a.rules, a.turnTime)
	if err != nil {
		// The usual port is taken (another game running?): use any free one.
		if srv, err = netplay.Listen(":0", a.rules, a.turnTime); err != nil {
			return a.playComputer("Couldn't start hosting: " + err.Error())
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	go srv.Run(ctx)
	go netplay.Announce(ctx, a.name, srv.Port())

	c, err := netplay.Dial("127.0.0.1:"+strconv.Itoa(srv.Port()), a.name)
	if err != nil {
		cancel()
		return a.playComputer("Couldn't start hosting: " + err.Error())
	}
	a.stopHost = cancel
	a.game = NewRemote(c, HostLobbyText(a.name, srv.Port())).withSize(a.width, a.height)
	a.screen = screenGame
	return a, a.game.Init()
}

// HostLobbyText is what the host sees while waiting for someone to join.
func HostLobbyText(name string, port int) string {
	s := fmt.Sprintf("Hosting as %s.\n\nOthers on this Wi-Fi can open the menu (m),\nchoose \"Join a game on this Wi-Fi\" and pick yours,\nor type one of these addresses:\n", name)
	for _, ip := range netplay.LocalIPs() {
		s += "\n    " + net.JoinHostPort(ip, strconv.Itoa(port))
	}
	return s + "\n\nesc back to the computer · q quit"
}

// ── Join screen ────────────────────────────────────────────────────────

// joinRows is what the join screen lists: the hosts found, then two actions.
func (a App) joinRows() []string {
	rows := make([]string, 0, len(a.hosts)+2)
	for _, h := range a.hosts {
		rows = append(rows, fmt.Sprintf("%s's game   (%s)", h.Name, h.Addr))
	}
	return append(rows, "Type an address…", "Back to the computer")
}

func (a App) startJoin() (tea.Model, tea.Cmd) {
	ctx, cancel := context.WithCancel(context.Background())
	found, err := netplay.Discover(ctx)
	a.screen, a.cursor, a.hosts, a.typing, a.addr = screenJoin, 0, nil, false, ""
	if err != nil {
		cancel()
		a.note = "Can't search this network (" + err.Error() + "). You can still type an address."
		return a, nil
	}
	a.stopDiscover = cancel
	return a, waitForHost(found)
}

// waitForHost delivers the next host discovery finds, one per command, the
// same pattern listen (remote.go) uses for server messages.
func waitForHost(ch <-chan netplay.Host) tea.Cmd {
	return func() tea.Msg {
		h, ok := <-ch
		return hostFoundMsg{host: h, ok: ok, ch: ch}
	}
}

// stopJoin stops looking for games.
func (a App) stopJoin() App {
	if a.stopDiscover != nil {
		a.stopDiscover()
		a.stopDiscover = nil
	}
	a.typing = false
	return a
}

func (a App) updateJoin(msg tea.Msg) (tea.Model, tea.Cmd) {
	rows := a.joinRows()
	switch msg := msg.(type) {
	case hostFoundMsg:
		if !msg.ok || a.stopDiscover == nil {
			return a, nil // discovery stopped
		}
		a.hosts = append(a.hosts, msg.host)
		return a, waitForHost(msg.ch)

	case connectedMsg:
		a.connecting = false
		if msg.err != nil {
			a.note = "Couldn't connect to " + msg.addr + ": " + msg.err.Error()
			return a, nil
		}
		a = a.stopJoin()
		a.screen = screenGame
		a.game = NewRemote(msg.client, "Joining "+msg.addr+"…\n\nesc back · q quit").withSize(a.width, a.height)
		a.screen = screenGame
		return a, a.game.Init()

	case tea.KeyPressMsg:
		if a.typing {
			return a.typeAddress(msg)
		}
		switch msg.String() {
		case "ctrl+c":
			return a, tea.Quit
		case "q", "esc":
			return a.stopJoin().playComputer("")
		case "up", "k":
			a.cursor = (a.cursor + len(rows) - 1) % len(rows)
		case "down", "j":
			a.cursor = (a.cursor + 1) % len(rows)
		case "enter", "space", " ":
			return a.chooseJoin(a.cursor)
		}

	case tea.MouseMotionMsg:
		if i := a.rowAt(msg.Mouse().Y, len(rows)); i >= 0 {
			a.cursor = i
		}
	case tea.MouseClickMsg:
		if i := a.rowAt(msg.Mouse().Y, len(rows)); i >= 0 {
			return a.chooseJoin(i)
		}
	}
	return a, nil
}

func (a App) chooseJoin(i int) (tea.Model, tea.Cmd) {
	switch {
	case i < len(a.hosts):
		return a.connect(a.hosts[i].Addr)
	case i == len(a.hosts): // type an address
		a.typing, a.addr, a.note = true, "", ""
		return a, nil
	default: // back
		return a.stopJoin().playComputer("")
	}
}

// typeAddress handles keys while typing an address by hand.
func (a App) typeAddress(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch key := msg.String(); key {
	case "ctrl+c":
		return a, tea.Quit
	case "esc":
		a.typing = false
	case "enter":
		addr := strings.TrimSpace(a.addr)
		if addr == "" {
			return a, nil
		}
		if _, _, err := net.SplitHostPort(addr); err != nil {
			addr = net.JoinHostPort(addr, strconv.Itoa(netplay.DefaultPort)) // no port: use the usual one
		}
		return a.connect(addr)
	case "backspace":
		if a.addr != "" {
			a.addr = a.addr[:len(a.addr)-1]
		}
	default:
		if len(msg.Text) == 1 && strings.ContainsAny(msg.Text, "0123456789.:abcdefABCDEF[]") {
			a.addr += msg.Text
		}
	}
	return a, nil
}

// connect dials in the background (it can take a few seconds) and reports
// back with a connectedMsg.
func (a App) connect(addr string) (tea.Model, tea.Cmd) {
	if a.connecting {
		return a, nil
	}
	a.connecting, a.note = true, "Connecting to "+addr+"…"
	name := a.name
	return a, func() tea.Msg {
		c, err := netplay.Dial(addr, name)
		return connectedMsg{client: c, addr: addr, err: err}
	}
}

// ── In a game ──────────────────────────────────────────────────────────

func (a App) updateGame(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := a.game.Update(msg)
	a.game = next.(Model)
	req := a.game.request
	if req == "" {
		return a, cmd
	}
	// Switching games: keep the offline settings for next time, then hang
	// up and stop anything we were hosting.
	if a.game.net == nil && a.game.game != nil {
		a.rules, a.turnTime = a.game.game.Rules, a.game.turnTime
	}
	if a.game.net != nil {
		_ = a.game.net.Close()
	}
	if a.stopHost != nil {
		a.stopHost()
		a.stopHost = nil
	}
	switch req {
	case "host":
		return a.host()
	case "join":
		return a.startJoin()
	default: // "leave"
		return a.playComputer("")
	}
}

// ── Drawing ────────────────────────────────────────────────────────────

const rowsTop = 4 // rows start this many lines into the box: border, padding, title, blank

// menuBox draws a centred box with a title and selectable rows; it also
// returns the screen line of the first row, for mouse hit-testing.
func (a App) menuBox(title string, rows []string, cursor int, footer string) (string, int) {
	lines := []string{titleStyle.Render(title), ""}
	for i, r := range rows {
		if i == cursor {
			lines = append(lines, selectStyle.Render("▶ "+r))
		} else {
			lines = append(lines, "  "+r)
		}
	}
	if footer != "" {
		lines = append(lines, "", footer)
	}
	box := lipgloss.NewStyle().Border(lipgloss.ThickBorder()).BorderForeground(colSelect).
		Padding(1, 4).Render(strings.Join(lines, "\n"))
	top := max(0, (a.height-lipgloss.Height(box))/2)
	return box, top + rowsTop
}

// rowAt returns which of n rows is on screen line y, or -1.
func (a App) rowAt(y, n int) int {
	var first int
	_, first = a.joinBox()
	if i := y - first; i >= 0 && i < n {
		return i
	}
	return -1
}

func (a App) joinBox() (string, int) {
	footer := dimStyle.Render("Looking for games on this Wi-Fi…")
	if len(a.hosts) > 0 {
		footer = dimStyle.Render("Pick a game, or keep waiting for more.")
	}
	if a.typing {
		footer = "Address: " + selectStyle.Render(a.addr+"▏") + "\n" +
			dimStyle.Render("like 192.168.1.23 · enter connect · esc cancel")
	}
	if a.note != "" {
		footer += "\n" + statusStyle.Render(a.note)
	}
	return a.menuBox("JOIN A GAME", a.joinRows(), a.cursor, footer)
}

func (a App) View() tea.View {
	if a.screen == screenGame {
		return a.game.View()
	}
	box, _ := a.joinBox()
	v := tea.NewView(lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, box))
	v.AltScreen = true
	v.MouseMode = tea.MouseModeAllMotion
	v.WindowTitle = "Durak"
	return v
}
