package ui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// The rules menu: a popup opened with m. While it's open the game is paused
// (your turn timer and the opponent both wait), and every key goes here.

// timerChoices are the turn-timer options the menu cycles through; 0 is off.
var timerChoices = []time.Duration{0, 15 * time.Second, 30 * time.Second, 60 * time.Second}

// menuItem is one row of the menu.
type menuItem struct {
	label    string
	help     string
	value    func(m *Model) string // current setting, shown on the right
	change   func(m *Model, dir int)
	disabled func(m *Model) bool
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

var menuItems = []menuItem{
	{
		label:  "Pass the attack on",
		help:   "Defend with a card of the same rank to send the attack back (perevodnoy).",
		value:  func(m *Model) string { return onOff(m.game.Rules.PassOn) },
		change: func(m *Model, _ int) { m.game.Rules.PassOn = !m.game.Rules.PassOn },
	},
	{
		label:    "Pass on with several cards",
		help:     "After passing on, you may add more cards of that rank before your opponent defends.",
		value:    func(m *Model) string { return onOff(m.game.Rules.PassOnMany) },
		change:   func(m *Model, _ int) { m.game.Rules.PassOnMany = !m.game.Rules.PassOnMany },
		disabled: func(m *Model) bool { return !m.game.Rules.PassOn },
	},
	{
		label:  "Throw in after take",
		help:   "When the defender gives up, the attacker may still add matching cards for them to pick up.",
		value:  func(m *Model) string { return onOff(m.game.Rules.ThrowInAfterTake) },
		change: func(m *Model, _ int) { m.game.Rules.ThrowInAfterTake = !m.game.Rules.ThrowInAfterTake },
	},
	{
		label: "Turn timer",
		help:  "How long you get per turn before the game moves on for you.",
		value: func(m *Model) string {
			if m.turnTime == 0 {
				return "off"
			}
			return fmt.Sprintf("%ds", int(m.turnTime.Seconds()))
		},
		change: func(m *Model, dir int) {
			i := 0
			for j, d := range timerChoices {
				if d == m.turnTime {
					i = j
				}
			}
			n := len(timerChoices)
			m.turnTime = timerChoices[((i+dir)%n+n)%n] // wrap around both ways
			m.timerChanged = true
		},
	},
}

// openMenu pauses the turn timer and shows the menu.
func (m Model) openMenu() Model {
	m.menuOpen = true
	m.menuCursor = 0
	m.paused = time.Until(m.deadline)
	m.timerChanged = false
	return m
}

// closeMenu hides the menu and resumes the timer where it left off, or
// restarts it from full time if the timer setting was changed.
func (m Model) closeMenu() Model {
	m.menuOpen = false
	if m.timerChanged {
		m.timerOwner = -2 // forces syncTimer to start a fresh timer
	} else {
		m.deadline = time.Now().Add(m.paused)
	}
	return m
}

// updateMenu handles keys while the menu is open.
func (m Model) updateMenu(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	item := menuItems[m.menuCursor]
	disabled := item.disabled != nil && item.disabled(&m)

	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "m", "esc", "q":
		return m.closeMenu(), nil
	case "up", "k":
		m.menuCursor = (m.menuCursor + len(menuItems) - 1) % len(menuItems)
	case "down", "j":
		m.menuCursor = (m.menuCursor + 1) % len(menuItems)
	case "enter", "space", " ", "right", "l":
		if !disabled {
			item.change(&m, 1)
		}
	case "left", "h":
		if !disabled {
			item.change(&m, -1)
		}
	}
	m.clampCursor()
	return m, nil
}

// menuView draws the popup box.
func (m Model) menuView() string {
	labelW := 0
	for _, it := range menuItems {
		labelW = max(labelW, lipgloss.Width(it.label))
	}

	rows := []string{titleStyle.Render("GAME RULES"), ""}
	for i, it := range menuItems {
		disabled := it.disabled != nil && it.disabled(&m)
		pointer := "  "
		if i == m.menuCursor {
			pointer = selectStyle.Render("› ")
		}
		label := it.label + strings.Repeat(" ", labelW-lipgloss.Width(it.label))
		value := "‹ " + it.value(&m) + " ›"
		row := pointer + label + "   " + value
		switch {
		case disabled:
			row = dimStyle.Render(pointer + label + "   " + "(needs pass-on)")
		case i == m.menuCursor:
			row = pointer + titleStyle.Render(label) + "   " + selectStyle.Render(value)
		}
		rows = append(rows, row)
	}
	help := lipgloss.NewStyle().Width(labelW + 16).Foreground(colDim).
		Render(menuItems[m.menuCursor].help)
	rows = append(rows, "", help, "",
		dimStyle.Render("↑/↓ or hover choose · enter/click change · m close"))

	return menuStyle.Render(strings.Join(rows, "\n"))
}

// menuBox is the popup's position and size on screen.
func (m Model) menuBox() (x, y, w, h int) {
	box := m.menuView()
	w, h = lipgloss.Width(box), lipgloss.Height(box)
	return max(0, (m.width-w)/2), max(0, (m.height-h)/2), w, h
}

// withMenu draws the menu box on top of the game screen, centred.
func (m Model) withMenu(screen string) string {
	box := m.menuView()
	x, y, _, _ := m.menuBox()
	return lipgloss.NewCompositor(
		lipgloss.NewLayer(screen),
		lipgloss.NewLayer(box).X(x).Y(y).Z(1),
	).Render()
}
