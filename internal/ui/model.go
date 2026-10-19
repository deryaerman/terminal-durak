// Package ui is the Bubble Tea front end.
//
// Bubble Tea follows the Elm architecture, a loop of three parts:
//
//	Model  — a struct holding everything the screen needs
//	Update — receives a message (key press, timer, resize…) and returns the
//	         new model plus an optional command (work to run in the background)
//	View   — turns the model into a string to draw
//
// Bubble Tea calls View after every Update, so you never redraw by hand: you
// only change the model.
package ui

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"durak/internal/cards"
	"durak/internal/game"
)

// opponentMoveMsg arrives when the opponent's "thinking" delay is over.
// Any type can be a message; an empty struct is the usual choice for a signal.
type opponentMoveMsg struct{}

// Model is the whole UI state.
type Model struct {
	game   *game.State
	cursor int // index of the selected card in your hand
	err    error
	width  int
	height int
}

// New starts a fresh game.
func New() Model {
	return Model{game: game.New(), width: 100, height: 30}
}

// Init runs once at start-up. We have no background work yet, so it returns nil.
func (m Model) Init() tea.Cmd { return nil }

// opponentThinks waits a moment, then delivers opponentMoveMsg to Update.
// tea.Tick is a command: Bubble Tea runs it in the background and sends its
// result back as a message, so the UI never freezes while waiting.
func opponentThinks() tea.Cmd {
	return tea.Tick(700*time.Millisecond, func(time.Time) tea.Msg {
		return opponentMoveMsg{}
	})
}

// Update handles one message and returns the updated model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// A "type switch": run a different branch depending on the message's type.
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case opponentMoveMsg:
		m.game.OpponentMove()
		m.clampCursor()
		if !m.game.Over && m.game.Turn == game.Opponent {
			return m, opponentThinks() // the opponent still has more to do
		}

	case tea.KeyPressMsg:
		m.err = nil
		hand := m.game.Players[game.You].Hand

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "n":
			if m.game.Over {
				return New().withSize(m.width, m.height), nil
			}

		case "left", "h":
			if m.cursor > 0 {
				m.cursor--
			}

		case "right", "l":
			if m.cursor < len(hand)-1 {
				m.cursor++
			}

		case "enter":
			if len(hand) > 0 {
				m.err = m.game.Play(game.You, m.cursor)
				m.clampCursor()
				if m.game.Turn == game.Opponent { // you passed the attack on
					return m, opponentThinks()
				}
			}

		case "r":
			if len(hand) > 0 {
				m.err = m.game.Transfer(game.You, m.cursor)
				m.clampCursor()
				if m.err == nil {
					return m, opponentThinks()
				}
			}

		case "space", " ":
			if m.game.Turn != game.You {
				m.err = game.ErrNotYourTurn
				break
			}
			m.err = m.game.EndTurn()
			m.clampCursor()
			if m.err == nil && m.game.Turn == game.Opponent {
				return m, opponentThinks()
			}

		case "t":
			if m.game.Turn == game.You && m.game.Defender() == game.You && len(m.game.Table) > 0 {
				m.game.Take()
				m.clampCursor()
				return m, opponentThinks() // opponent attacks again
			}

		case "s":
			m.game.SortHand(game.You)
		}
	}
	return m, nil
}

// withSize keeps the current window size when starting a new game.
func (m Model) withSize(w, h int) Model {
	m.width, m.height = w, h
	return m
}

// clampCursor keeps the selection inside the hand after cards come and go.
func (m *Model) clampCursor() {
	n := len(m.game.Players[game.You].Hand)
	m.cursor = max(0, min(m.cursor, n-1))
}

// View draws the whole screen.
func (m Model) View() tea.View {
	g := m.game
	// Card size grows with the terminal, and everything is laid out to fill
	// m.width × m.height exactly.
	r := renderer{size: pickCardSize(m.width, m.height), trump: g.Trump.Suit}
	center := func(s string) string {
		return lipgloss.PlaceHorizontal(m.width, lipgloss.Center, s)
	}

	opponent := center(r.opponent(len(g.Players[game.Opponent].Hand)))
	// While it's your move, fade the cards the rules don't allow.
	playable := func(c cards.Card) bool {
		return g.Turn != game.You || g.CanPlay(game.You, c)
	}
	you := center(r.hand(g.Players[game.You].Hand, m.cursor, playable))

	status := statusStyle.Render(g.Status)
	if m.err != nil {
		status = errorStyle.Render(m.err.Error())
	}
	footer := lipgloss.JoinVertical(lipgloss.Left,
		"",
		m.turnLine()+"  "+status,
		dimStyle.Render(m.helpLine()),
	)

	// The middle row (deck + table) takes whatever height is left over.
	used := lipgloss.Height(opponent) + lipgloss.Height(you) + lipgloss.Height(footer)
	midH := max(r.size.h+5, m.height-used)
	deck := r.deck(g, midH)
	tableW := max(30, m.width-lipgloss.Width(deck)-1)
	middle := lipgloss.JoinHorizontal(lipgloss.Top, deck, " ", r.table(g, tableW, midH))

	screen := lipgloss.JoinVertical(lipgloss.Left, opponent, middle, you, footer)

	v := tea.NewView(screen)
	v.AltScreen = true // draw on a separate full screen, restored on quit
	v.WindowTitle = "Durak"
	return v
}

func (m Model) turnLine() string {
	g := m.game
	switch {
	case g.Over && g.Loser == game.Opponent:
		return winStyle.Render("YOU WIN!")
	case g.Over && g.Loser == game.You:
		return errorStyle.Bold(true).Render("YOU LOSE.")
	case g.Over:
		return titleStyle.Render("DRAW.")
	case g.Turn == game.Opponent:
		return titleStyle.Render("Opponent is thinking…")
	case g.Attacker == game.You:
		return titleStyle.Render("Your attack.")
	default:
		return titleStyle.Render("Defend!")
	}
}

func (m Model) helpLine() string {
	if m.game.Over {
		return "n new game · q quit"
	}
	help := "←/→ select · enter play · space end turn · "
	if m.game.Defender() == game.You {
		help += "r pass on · t take cards · "
	}
	return help + "d draw · s sort · q quit"
}
