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
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"durak/internal/cards"
	"durak/internal/game"
)

// opponentMoveMsg arrives when the opponent's "thinking" delay is over.
// Any type can be a message; an empty struct is the usual choice for a signal.
type opponentMoveMsg struct{}

// TurnTime is the default time per turn; the menu can change or disable it.
const TurnTime = 30 * time.Second

// timerTickMsg arrives once a second while your turn timer runs. id says which
// timer sent it, so ticks from a timer that has since been reset are ignored.
type timerTickMsg struct{ id int }

// Model is the whole UI state.
type Model struct {
	game   *game.State
	cursor int // index of the selected card in your hand
	err    error
	width  int
	height int

	// Turn timer: when your move must be made by, and which timer is current.
	deadline   time.Time
	timerID    int
	timerOwner int           // whose turn the running timer belongs to, -1 if none
	turnTime   time.Duration // time per turn; 0 means no timer

	// Rules menu (see menu.go).
	menuOpen     bool
	menuCursor   int
	paused       time.Duration // time left on your turn when the menu opened
	timerChanged bool          // the timer setting changed while the menu was open
}

// New starts a fresh game with the default rules.
func New() Model { return newGame(game.DefaultRules, TurnTime) }

// newGame starts a fresh game with the given rules and turn time.
func newGame(r game.Rules, turnTime time.Duration) Model {
	return Model{
		game:       game.NewWithRules(r),
		width:      100,
		height:     30,
		timerOwner: -1,
		turnTime:   turnTime,
	}
}

// Init runs once at start-up: it starts your turn timer, or lets the
// opponent move if they hold the lowest trump.
func (m Model) Init() tea.Cmd {
	_, cmd := m.syncTimer()
	return tea.Batch(cmd, m.opponentIfTheirTurn())
}

// opponentIfTheirTurn schedules the opponent's move when it's their turn.
func (m Model) opponentIfTheirTurn() tea.Cmd {
	if !m.game.Over && m.game.Turn == game.Opponent {
		return opponentThinks()
	}
	return nil
}

// tick asks Bubble Tea for a timerTickMsg one second from now.
func tick(id int) tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return timerTickMsg{id} })
}

// syncTimer starts a fresh timer whenever the move passes to you, and stops it
// when it passes away from you or the game ends. It's called after every
// Update, so no individual key handler has to remember to reset the timer.
func (m Model) syncTimer() (Model, tea.Cmd) {
	if m.menuOpen {
		return m, nil // paused
	}
	owner := -1
	if m.turnTime > 0 && !m.game.Over && m.game.Turn == game.You {
		owner = game.You
	}
	if owner == m.timerOwner {
		return m, nil // same player still on the move: keep counting down
	}
	m.timerOwner = owner
	m.timerID++ // any tick still in flight for the old timer is now stale
	if owner < 0 {
		return m, nil
	}
	m.deadline = time.Now().Add(m.turnTime)
	return m, tick(m.timerID)
}

// timeLeft is how long you have left, rounded up to whole seconds.
func (m Model) timeLeft() time.Duration {
	return max(0, time.Until(m.deadline)).Round(time.Second)
}

// opponentThinks waits a moment, then delivers opponentMoveMsg to Update.
// tea.Tick is a command: Bubble Tea runs it in the background and sends its
// result back as a message, so the UI never freezes while waiting.
func opponentThinks() tea.Cmd {
	return tea.Tick(700*time.Millisecond, func(time.Time) tea.Msg {
		return opponentMoveMsg{}
	})
}

// Update handles one message, then lets syncTimer start or stop the turn timer.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := m.update(msg)
	nm, timerCmd := next.syncTimer()
	// tea.Batch runs several commands at once (here: the opponent's move
	// and the timer tick), skipping any that are nil.
	return nm, tea.Batch(cmd, timerCmd)
}

func (m Model) update(msg tea.Msg) (Model, tea.Cmd) {
	// A "type switch": run a different branch depending on the message's type.
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case timerTickMsg:
		if msg.id != m.timerID || m.timerOwner != game.You {
			return m, nil // a stale tick from a timer that was reset
		}
		if m.menuOpen {
			return m, tick(msg.id) // paused: keep the timer alive, don't count
		}
		if time.Now().Before(m.deadline) {
			return m, tick(msg.id) // keep ticking: each tick redraws the countdown
		}
		m.game.Timeout(game.You)
		m.clampCursor()
		if !m.game.Over && m.game.Turn == game.Opponent {
			return m, opponentThinks()
		}

	case opponentMoveMsg:
		if m.menuOpen {
			return m, opponentThinks() // paused: try again in a moment
		}
		m.game.OpponentMove()
		m.clampCursor()
		if !m.game.Over && m.game.Turn == game.Opponent {
			return m, opponentThinks() // the opponent still has more to do
		}

	case tea.KeyPressMsg:
		if m.menuOpen {
			return m.updateMenu(msg)
		}
		m.err = nil
		hand := m.game.Players[game.You].Hand

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "m":
			return m.openMenu(), nil

		case "n":
			if m.game.Over {
				// Keep the rules and timer setting chosen in the menu.
				nm := newGame(m.game.Rules, m.turnTime).withSize(m.width, m.height)
				nm.timerID = m.timerID // so the old game's ticks stay stale
				return nm, nm.opponentIfTheirTurn()
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

	if m.menuOpen {
		screen = m.withMenu(screen)
	}

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
		return titleStyle.Render("Your attack.") + " " + m.clock()
	default:
		return titleStyle.Render("Defend!") + " " + m.clock()
	}
}

// clock shows the seconds left on your turn, in red for the last few.
func (m Model) clock() string {
	if m.turnTime == 0 {
		return ""
	}
	left := m.timeLeft()
	if m.menuOpen {
		return dimStyle.Render(fmt.Sprintf("⏸ %ds", int(m.paused.Round(time.Second).Seconds())))
	}
	style := dimStyle
	if left <= 5*time.Second {
		style = errorStyle.Bold(true)
	}
	return style.Render(fmt.Sprintf("⏱ %ds", int(left.Seconds())))
}

func (m Model) helpLine() string {
	if m.game.Over {
		return "n new game · m rules · q quit"
	}
	help := "←/→ select · enter play · space end turn · "
	if m.game.Defender() == game.You {
		if m.game.Rules.PassOn {
			help += "r pass on · "
		}
		help += "t take cards · "
	}
	return help + "s sort · m rules · q quit"
}
