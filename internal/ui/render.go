package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"durak/internal/cards"
	"durak/internal/game"
)

// Every function here takes data and returns a string. Lip Gloss works by
// styling blocks of text and gluing them together side by side
// (JoinHorizontal) or on top of each other (JoinVertical).

// cardSize is a card's outer width and height, border included.
type cardSize struct{ w, h int }

// Sizes from biggest to smallest; the View picks the biggest that fits.
var cardSizes = []cardSize{{11, 9}, {9, 7}, {7, 5}}

// pickCardSize returns the largest card size that fits the terminal: three
// card rows (opponent, table, your hand) plus some panels and text.
func pickCardSize(width, height int) cardSize {
	for _, s := range cardSizes {
		if 3*s.h+9 <= height && 10*s.w+8 <= width {
			return s
		}
	}
	return cardSizes[len(cardSizes)-1]
}

// renderer draws cards at one size for one game.
type renderer struct {
	size  cardSize
	trump cards.Suit
}

// cardFace lays out the inside of a card: rank top-left, suit in the middle,
// rank bottom-right, like a real playing card.
func (r renderer) cardFace(c cards.Card) string {
	w, h := r.size.w-2, r.size.h-2
	rank, suit := c.Rank.Label(), c.Suit.Symbol()
	pad := strings.Repeat(" ", w-lipgloss.Width(rank))
	lines := make([]string, h)
	for i := range lines {
		lines[i] = strings.Repeat(" ", w)
	}
	lines[0] = rank + pad   // rank top-left
	lines[h-1] = pad + rank // rank bottom-right

	lines[h/2] = lipgloss.PlaceHorizontal(w, lipgloss.Center, suit)
	return strings.Join(lines, "\n")
}

// card draws one face-up card, e.g. at the smallest size
//
//	┌─────┐
//	│Q    │
//	│  ♠  │
//	│    Q│
//	└─────┘
func (r renderer) card(c cards.Card, selected bool) string {
	return r.cardStyled(c, selected, false)
}

// cardStyled is card with an extra "dimmed" look for cards you can't play.
func (r renderer) cardStyled(c cards.Card, selected, dimmed bool) string {
	fg := colInk
	if c.IsRed() {
		fg = colRedInk
	}
	if dimmed {
		fg = colFaded
	}
	style := cardStyle
	if selected {
		style = selectedStyle
	} else if c.Suit == r.trump {
		style = style.BorderForeground(colTrumpEdge) // trumps get a gold edge
	}
	style = style.Width(r.size.w).Height(r.size.h).Foreground(fg).Bold(true)
	return style.Render(r.cardFace(c))
}

// back draws a face-down card: a flower with petals made of (_), framed by
// a vine border, like the floral print on real card backs. Each card size has
// its own drawing, in plain ASCII so every terminal font can show it.
func (r renderer) back() string {
	rows, ok := backDesigns[r.size]
	if !ok {
		rows = backDesigns[cardSizes[len(cardSizes)-1]]
	}
	return backStyle.Width(r.size.w).Height(r.size.h).Render(strings.Join(rows, "\n"))
}

// backDesigns maps each card size to the inside of its back (width w-2,
// height h-2).
var backDesigns = map[cardSize][]string{
	{7, 5}: {
		"~*~*~",
		"*(@)*",
		"~*~*~",
	},
	{9, 7}: {
		"~.~.~.~",
		"._(_)_.",
		"(_)@(_)",
		".:(_):.",
		"~.~.~.~",
	},
	{11, 9}: {
		".~*~*~*~.",
		":.:.:.:.:",
		":._(_)_.:",
		":(_)@(_):",
		":.:(_):.:",
		":.:.:.:.:",
		"'~*~*~*~'",
	},
}

// hand draws your cards in a row. The selected card is lifted by one line:
// every other card gets a blank line on top, so they sit lower. Cards that
// playable rejects are drawn faded.
func (r renderer) hand(hand []cards.Card, cursor int, playable func(cards.Card) bool) string {
	if len(hand) == 0 {
		return dimStyle.Render("(no cards)")
	}
	cols := make([]string, len(hand))
	for i, c := range hand {
		dim := !playable(c)
		if i == cursor {
			cols[i] = r.cardStyled(c, true, dim) + "\n"
		} else {
			cols[i] = "\n" + r.cardStyled(c, false, dim)
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, cols...)
}

// opponent draws the opponent's cards face down.
func (r renderer) opponent(n int) string {
	if n == 0 {
		return dimStyle.Render("(no cards)")
	}
	backs := make([]string, n)
	for i := range backs {
		backs[i] = r.back()
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, backs...)
}

// deck draws the side pile: remaining cards, a face-down card and the
// face-up trump card that is drawn last.
func (r renderer) deck(s *game.State, height int) string {
	var pile string
	switch {
	case s.Deck.Len() > 1:
		pile = lipgloss.JoinHorizontal(lipgloss.Center, r.back(), " ", r.card(s.Trump, false))
	case s.Deck.Len() == 1:
		pile = r.card(s.Trump, false)
	default:
		pile = dimStyle.Render("empty")
	}
	body := lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render(fmt.Sprintf("DECK %d", s.Deck.Len())),
		pile,
		"trump "+lipgloss.NewStyle().Foreground(colTrump).Render(s.Trump.Suit.Symbol()),
		dimStyle.Render(fmt.Sprintf("discarded %d", s.Discards)),
	)
	return panelStyle.Height(height).Render(body)
}

// table draws each attack with its defense lying on top of it, shifted down
// and to the right like a real overlapping card. The panel is width×height
// and the cards sit in its centre.
func (r renderer) table(s *game.State, width, height int) string {
	// Inner area: minus the border (2) and padding (2), minus the title line.
	innerW, innerH := width-4, height-3

	var body string
	switch {
	case s.Over:
		body = gameOverBanner(s)
	case len(s.Table) == 0:
		body = feltText.Render("the table is empty")
	default:
		body = r.piles(s)
	}
	// Place centres the body and fills the space around it with felt.
	body = lipgloss.Place(innerW, innerH, lipgloss.Center, lipgloss.Center, body,
		lipgloss.WithWhitespaceStyle(feltStyle))
	title := titleStyle.Background(colFelt).Width(innerW).Render("TABLE")
	return tableStyle.Width(width).Height(height).
		Render(lipgloss.JoinVertical(lipgloss.Left, title, body))
}

// piles draws every attack (with its defense on top) as one row. Each pile is
// built as an exact rectangle, with felt in the gaps: any cell left as a plain
// space would show the terminal's own background instead of green.
func (r renderer) piles(s *game.State) string {
	w, h := r.size.w, r.size.h
	pileW, pileH := w, h
	for _, p := range s.Table {
		if p.Defense != nil {
			pileW, pileH = w+2, h+2 // the defense is shifted 2 right, 2 down
		}
	}
	blocks := make([]string, 0, len(s.Table)*2)
	for i, p := range s.Table {
		attack := strings.Split(r.card(p.Attack, false), "\n")
		var lines []string
		if p.Defense == nil {
			lines = attack
		} else {
			// The defense lies 2 down and 2 right of the attack card, so the
			// attack's top two lines and its left two columns stay visible.
			lines = append(lines, attack[0], attack[1])
			for j, l := range strings.Split(r.card(*p.Defense, false), "\n") {
				left := felt(2) // below the attack card: bare felt
				if j+2 < len(attack) {
					left = ansi.Truncate(attack[j+2], 2, "") // attack's left edge
				}
				lines = append(lines, left+l)
			}
		}
		for j := range lines { // pad each line to the pile width
			lines[j] += felt(pileW - lipgloss.Width(lines[j]))
		}
		for len(lines) < pileH { // and the pile to the full height
			lines = append(lines, felt(pileW))
		}
		if i > 0 {
			blocks = append(blocks, feltBlock(3, pileH))
		}
		blocks = append(blocks, strings.Join(lines, "\n"))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, blocks...)
}

// felt returns n cells of green table background.
func felt(n int) string {
	if n <= 0 {
		return ""
	}
	return feltStyle.Render(strings.Repeat(" ", n))
}

// feltBlock is a w×h rectangle of felt, used as a gap between piles.
func feltBlock(w, h int) string {
	rows := make([]string, h)
	for i := range rows {
		rows[i] = felt(w)
	}
	return strings.Join(rows, "\n")
}

// gameOverBanner is the big result box shown on the table at the end.
func gameOverBanner(s *game.State) string {
	var headline, sub string
	style := winStyle
	switch s.Loser {
	case game.Opponent:
		headline, sub = "YOU WIN!", "The opponent is the durak."
	case game.You:
		headline, sub = "YOU'RE THE DURAK", "The opponent got rid of all their cards."
		style = errorStyle.Bold(true)
	default:
		headline, sub = "DRAW", "You both ran out of cards together."
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(style.GetForeground()).
		Padding(1, 4).
		Align(lipgloss.Center)
	return box.Render(style.Render(headline) + "\n\n" + sub + "\n\n" + dimStyle.Render("n new game · q quit"))
}
