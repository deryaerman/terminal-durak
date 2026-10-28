package ui

import (
	"errors"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"durak/internal/game"
	"durak/internal/netplay"
)

// Network play. The game lives on the host's server; this UI only ever sees
// a View of it. Each View is turned into a "mirror" State (game.FromView), so
// all the drawing, animations, mouse handling and "which cards can I play?"
// fading work exactly as against the bot. The differences:
//   - your moves are sent to the server instead of applied here
//   - the opponent is a person, so there's no bot thinking or pause logic
//   - the server keeps the turn clock and ends rounds; we only display them

// netMsg wraps a message from the server as a Bubble Tea message. from says
// which connection it came on: after leaving a game (or starting another), a
// late message from the old connection must not reach the new game.
type netMsg struct {
	netplay.Message
	from *netplay.Client
}

// disconnectedMsg arrives when the connection to the server closes.
type disconnectedMsg struct{ from *netplay.Client }

// NewRemote starts the UI for a network game over c. lobby is the text shown
// until the game starts (e.g. where others can join).
func NewRemote(c *netplay.Client, lobby string) Model {
	return Model{net: c, lobby: lobby, width: 100, height: 30, timerOwner: -1}
}

// listen waits for the next message from the server. Bubble Tea runs it in
// the background; when a message arrives it comes back to Update as a
// netMsg, which then starts the next listen. That's the usual Bubble Tea
// pattern for reading a stream: one message, one command, repeat.
func listen(c *netplay.Client) tea.Cmd {
	return func() tea.Msg {
		m, ok := <-c.Incoming
		if !ok {
			return disconnectedMsg{from: c}
		}
		return netMsg{Message: m, from: c}
	}
}

func (m Model) handleNet(msg netplay.Message) (Model, tea.Cmd) {
	next := listen(m.net)
	switch msg.Type {
	case netplay.TypeWelcome:
		m.lobby += "\n\nConnected. Waiting for the other player…"
	case netplay.TypeView:
		v := *msg.View
		m.game = game.FromView(v)
		m.err = nil
		m.turnTime = time.Duration(v.TurnTimeMs) * time.Millisecond
		if v.Turn == game.You && v.TimeLeftMs > 0 {
			m.deadline = time.Now().Add(time.Duration(v.TimeLeftMs) * time.Millisecond)
		}
		m.clampCursor()
	case netplay.TypeError:
		if m.game == nil {
			m.lobby = msg.Error + "\n\nesc back · q quit"
		} else {
			m.err = errors.New(msg.Error)
		}
	case netplay.TypeLeft:
		m.game = nil
		m.anims, m.lastLayout = nil, layout{}
		m.lobby = msg.Name + " left the game.\n\nWaiting for someone to join…\n\nesc back · q quit"
	}
	return m, next
}

// remoteAction sends the moves to the server. Before sending, the move is
// tried on a copy of the mirror, so an illegal move gets its explanation
// instantly instead of after a round trip. handled is false for keys that
// work the same as offline (selection, sorting, the menu, quitting).
func (m Model) remoteAction(key string) (Model, tea.Cmd, bool) {
	g := m.game
	hand := g.Players[game.You].Hand
	var move game.Move
	switch key {
	case "enter":
		if len(hand) == 0 {
			return m, nil, true
		}
		c := hand[m.cursor]
		move = game.Move{Kind: game.PlayCard, Card: c}
		// Like offline: a card that can't beat but has the same rank passes on.
		if u := g.Undefended(); g.Defender() == game.You && u >= 0 && !g.Beats(c, g.Table[u].Attack) && g.CanTransfer(c) {
			move.Kind = game.PassOn
		}
	case "r":
		if len(hand) == 0 {
			return m, nil, true
		}
		move = game.Move{Kind: game.PassOn, Card: hand[m.cursor]}
	case "space", " ":
		move = game.Move{Kind: game.EndTurnMove}
	case "t":
		move = game.Move{Kind: game.TakeCards}
	case "n":
		if g.Over {
			_ = m.net.Rematch()
		}
		return m, nil, true
	case "q", "ctrl+c":
		_ = m.net.Close()
		return m, tea.Quit, true
	default:
		return m, nil, false
	}

	if g.Turn != game.You {
		m.err = game.ErrNotYourTurn
		return m, nil, true
	}
	if err := g.Clone().Apply(game.You, move); err != nil {
		m.err = err
		return m, nil, true
	}
	if err := m.net.Move(move); err != nil {
		m.err = err
	}
	return m, nil, true
}

// lobbyView is the screen before the game starts.
func (m Model) lobbyView() string {
	box := lipgloss.NewStyle().
		Border(lipgloss.ThickBorder()).
		BorderForeground(colSelect).
		Padding(1, 4).
		Render(titleStyle.Render("DURAK · NETWORK GAME") + "\n\n" + strings.TrimSpace(m.lobby))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
