package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"durak/internal/game"
)

// Mouse support. Bubble Tea reports the mouse as cell coordinates (column,
// row). Using the layout from the last render we find what is under the
// pointer, then call the same action() the keyboard uses:
//
//	hover a card in your hand  → select it
//	click a card               → play it (the same as enter)
//	scroll wheel               → move the selection
//	click a button             → that button's key (End turn, Take…)
//	in the rules menu          → hover to choose, click to change, click outside to close

// button is a clickable action shown next to your turn badge.
type button struct {
	label string
	key   string // the key it stands for, passed to action()
}

// hitbox is a button's position on screen (one line tall).
type hitbox struct {
	x, y, w int
	key     string
}

// youButtons are the actions that make sense right now.
func (m Model) youButtons() []button {
	g := m.game
	switch {
	case g.Over:
		return []button{{"New game", "n"}}
	case g.Turn != game.You || len(g.Table) == 0:
		return nil
	case g.Attacker == game.You && g.Taking:
		return []button{{"Done ␣", "space"}}
	case g.Attacker == game.You:
		return []button{{"End turn ␣", "space"}}
	case g.Undefended() < 0:
		return []button{{"Done ␣", "space"}}
	default:
		return []button{{"Take t", "t"}}
	}
}

var (
	buttonStyle      = lipgloss.NewStyle().Foreground(colDark).Background(colDim).Padding(0, 1)
	buttonHoverStyle = buttonStyle.Background(colSelect).Bold(true)
)

// youRow is the line above your hand: the badge, the timer bar and the
// buttons, centred. It also returns each button's x range for hit-testing;
// the caller fills in the y once it knows where the row ends up.
func (m Model) youRow(y int) (string, []hitbox) {
	parts := []string{}
	if b := m.youBadge(); b != "" {
		parts = append(parts, b)
	}
	if t := m.timerBar(); t != "" {
		parts = append(parts, t)
	}
	firstButton := len(parts)
	btns := m.youButtons()
	for _, b := range btns {
		style := buttonStyle
		if m.hoverKey == b.key {
			style = buttonHoverStyle
		}
		parts = append(parts, style.Render(b.label))
	}

	const gap = "  "
	row := strings.Join(parts, gap)
	x := max(0, (m.width-lipgloss.Width(row))/2) // same rounding as PlaceHorizontal
	var hits []hitbox
	for i, p := range parts {
		if i >= firstButton {
			hits = append(hits, hitbox{x: x, y: y, w: lipgloss.Width(p), key: btns[i-firstButton].key})
		}
		x += lipgloss.Width(p) + len(gap)
	}
	return lipgloss.PlaceHorizontal(m.width, lipgloss.Center, row), hits
}

// cardAt returns the index of the card in your hand under the pointer, or -1.
// Each card's hit area includes the row above it, so the lifted card and its
// neighbours don't flicker as the selection moves.
func (m Model) cardAt(x, y int) int {
	lay := m.lastLayout
	for i, c := range m.game.Players[game.You].Hand {
		s, ok := lay.cards[c]
		if !ok {
			continue
		}
		top := lay.handY
		if x >= s.x && x < s.x+lay.cardW && y >= top && y <= top+lay.cardH {
			return i
		}
	}
	return -1
}

// buttonAt returns the key of the button under the pointer, or "".
func (m Model) buttonAt(x, y int) string {
	for _, b := range m.lastLayout.buttons {
		if y == b.y && x >= b.x && x < b.x+b.w {
			return b.key
		}
	}
	return ""
}

func (m Model) handleMouse(msg tea.MouseMsg) (Model, tea.Cmd) {
	mouse := msg.Mouse()
	if m.menuOpen {
		return m.menuMouse(msg, mouse.X, mouse.Y)
	}
	m.hoverKey = m.buttonAt(mouse.X, mouse.Y)

	switch msg.(type) {
	case tea.MouseMotionMsg:
		if i := m.cardAt(mouse.X, mouse.Y); i >= 0 {
			m.cursor = i
		}

	case tea.MouseWheelMsg:
		switch mouse.Button {
		case tea.MouseWheelUp, tea.MouseWheelLeft:
			return m.action("left")
		case tea.MouseWheelDown, tea.MouseWheelRight:
			return m.action("right")
		}

	case tea.MouseClickMsg:
		if mouse.Button != tea.MouseLeft {
			return m, nil
		}
		if key := m.buttonAt(mouse.X, mouse.Y); key != "" {
			return m.action(key)
		}
		if i := m.cardAt(mouse.X, mouse.Y); i >= 0 {
			m.cursor = i
			return m.action("enter")
		}
	}
	return m, nil
}

// menuMouse: hover picks a row, a click changes it, a click outside closes.
func (m Model) menuMouse(msg tea.MouseMsg, x, y int) (Model, tea.Cmd) {
	bx, by, bw, bh := m.menuBox()
	row := y - (by + 4) // border + padding + title + blank line
	onItem := x >= bx && x < bx+bw && row >= 0 && row < len(menuItems)

	switch msg.(type) {
	case tea.MouseMotionMsg:
		if onItem {
			m.menuCursor = row
		}
	case tea.MouseClickMsg:
		inside := x >= bx && x < bx+bw && y >= by && y < by+bh
		switch {
		case !inside:
			return m.closeMenu(), nil
		case onItem:
			m.menuCursor = row
			it := menuItems[row]
			if it.disabled == nil || !it.disabled(&m) {
				dir := 1
				if msg.Mouse().Button == tea.MouseRight {
					dir = -1 // right-click cycles the timer backwards
				}
				it.change(&m, dir)
			}
		}
	}
	return m, nil
}
