package ui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"durak/internal/game"
)

// The turn marker: a coloured badge next to whoever is on the move, and for
// you a draining timer bar, so you can tell whose turn it is at a glance
// without reading the status line.

var (
	colGo   = lipgloss.Color("#5fd787") // timer: plenty of time
	colDark = lipgloss.Color("#1c1c1c") // text on a coloured badge or button

	badgeBase = lipgloss.NewStyle().Bold(true).Foreground(colDark).Padding(0, 1)

	attackBadge   = badgeBase.Background(colSelect)
	defendBadge   = badgeBase.Background(colRed)
	takingBadge   = badgeBase.Background(colTrump)
	opponentBadge = badgeBase.Background(colTrump)
)

// timerBarWidth is the bar's length in cells.
const timerBarWidth = 24

// partials are block characters 1/8 to 7/8 wide, so the bar drains smoothly
// in eighths of a cell instead of jumping a whole cell at a time.
var partials = []string{"", "▏", "▎", "▍", "▌", "▋", "▊", "▉"}

// youBadge says what you're doing right now, or "" when it's not your move.
func (m Model) youBadge() string {
	g := m.game
	if g.Over || g.Turn != game.You {
		return ""
	}
	switch {
	case g.Attacker != game.You:
		return defendBadge.Render("⚔ DEFEND")
	case g.Taking:
		return takingBadge.Render("➕ ADD CARDS")
	default:
		return attackBadge.Render("▶ YOUR ATTACK")
	}
}

// opponentBadge marks the opponent while it's their move.
func (m Model) opponentBadge() string {
	g := m.game
	if g.Over || g.Turn != game.Opponent {
		return ""
	}
	name := "OPPONENT"
	if m.net != nil {
		name = strings.ToUpper(g.Players[game.Opponent].Name) // a real person: use their name
	}
	if g.Attacker == game.Opponent {
		return opponentBadge.Render("▼ " + name + " ATTACKS…")
	}
	return opponentBadge.Render("▼ " + name + " DEFENDS…")
}

// timerBar draws your remaining time as a bar that drains and changes colour
// from green to yellow to red, followed by the seconds left.
func (m Model) timerBar() string {
	if m.turnTime == 0 || m.game.Over || m.game.Turn != game.You {
		return ""
	}
	left := time.Until(m.deadline)
	if m.menuOpen {
		left = m.paused
	}
	frac := max(0, min(1, float64(left)/float64(m.turnTime)))

	col := colGo
	switch {
	case frac <= 0.2:
		col = colRed
	case frac <= 0.5:
		col = colTrump
	}
	if m.menuOpen {
		col = colDim
	}

	eighths := int(frac * timerBarWidth * 8)
	full, part := eighths/8, eighths%8
	bar := strings.Repeat("█", full) + partials[part]
	rest := timerBarWidth - full
	if part > 0 {
		rest--
	}
	filled := lipgloss.NewStyle().Foreground(col).Render(bar)
	empty := dimStyle.Render(strings.Repeat("░", rest))

	secs := int(left.Round(time.Second).Seconds())
	label := fmt.Sprintf(" %2ds", max(0, secs))
	if m.menuOpen {
		label = " ⏸" + label
	}
	return filled + empty + lipgloss.NewStyle().Foreground(col).Bold(true).Render(label)
}

// badgeRow is a one-line row: the badge (and timer) centred, or a blank line
// when there's nothing to show, so the layout doesn't jump between turns.
func badgeRow(width int, parts ...string) string {
	var shown []string
	for _, p := range parts {
		if p != "" {
			shown = append(shown, p)
		}
	}
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, strings.Join(shown, "  "))
}
