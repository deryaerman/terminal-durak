package ui

import (
	"math"
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"durak/internal/cards"
	"durak/internal/game"
)

// Card animations.
//
// One generic idea covers everything: after every Update we work out where
// each visible card sits on screen (its "spot"). If a card's zone changed since
// last time (hand → table, deck → hand, table → discard...), it flies from its
// old spot to its new one. While it flies, its new spot is drawn empty and a
// copy is drawn on top of the screen at an in-between position, frame by frame.
//
// The game state itself changes instantly; animations are purely visual.

// zone is the area of the screen a card is in.
type zone int

const (
	zoneDeck zone = iota + 1
	zoneTrump
	zoneYouHand
	zoneOppHand
	zoneTable
	zoneDiscard
)

// spot is a card's top-left corner on screen, its zone, and whether it's face up.
type spot struct {
	x, y   int
	zone   zone
	faceUp bool
}

// layout is where everything is on the current screen.
type layout struct {
	cards   map[cards.Card]spot
	deck    spot // the face-down deck pile
	discard spot // where discarded cards fly to (under the deck panel)
}

// anim is one card flying from one spot to another.
type anim struct {
	card     cards.Card
	from, to spot
	start    time.Time // when it starts moving (later than now while delayed)
}

const (
	animDuration = 280 * time.Millisecond
	frameEvery   = time.Second / 30 // 30 frames per second

	staggerClear = 40 * time.Millisecond // between cards leaving the table
	staggerPlay  = 60 * time.Millisecond
	staggerDraw  = 90 * time.Millisecond // between cards dealt from the deck
)

// frameMsg asks for the next animation frame.
type frameMsg struct{}

func nextFrame() tea.Cmd {
	return tea.Tick(frameEvery, func(time.Time) tea.Msg { return frameMsg{} })
}

func (a anim) done(now time.Time) bool { return !now.Before(a.start.Add(animDuration)) }

// hiddenCards are the cards still in flight; their real spot is drawn empty.
func (m Model) hiddenCards() map[cards.Card]bool {
	if len(m.anims) == 0 {
		return nil
	}
	now := time.Now()
	h := make(map[cards.Card]bool, len(m.anims))
	for _, a := range m.anims {
		if !a.done(now) {
			h[a.card] = true
		}
	}
	return h
}

// computeLayout works out every card's screen position. It mirrors exactly
// how render() stacks the blocks: opponent row, then deck + table, then your
// row. Lip Gloss centres with the extra space split evenly, the odd cell
// going to the right, hence the plain integer division for left offsets.
func (m Model) computeLayout(r renderer, oppH, midH, deckW, tableW int) layout {
	g := m.game
	w, h := r.size.w, r.size.h
	W := m.width
	lay := layout{cards: map[cards.Card]spot{}}
	leftOf := func(contentW int) int { return max(0, (W-contentW)/2) }

	// Opponent's hand, top row.
	opp := g.Players[game.Opponent].Hand
	x0 := leftOf(len(opp) * w)
	for i, c := range opp {
		lay.cards[c] = spot{x: x0 + i*w, y: 0, zone: zoneOppHand}
	}

	// Deck panel: a border and 1 cell of padding, then the title line.
	y0 := oppH
	pileY := y0 + 2
	lay.deck = spot{x: 2, y: pileY, zone: zoneDeck}
	lay.discard = spot{x: 2, y: pileY + h + 2, zone: zoneDiscard}
	switch {
	case g.Deck.Len() > 1:
		lay.cards[g.Trump] = spot{x: 2 + w + 1, y: pileY, zone: zoneTrump, faceUp: true}
	case g.Deck.Len() == 1:
		lay.cards[g.Trump] = spot{x: 2, y: pileY, zone: zoneTrump, faceUp: true}
	}

	// Table: border + padding, title line, then the piles centred in the rest.
	if !g.Over && len(g.Table) > 0 {
		innerX, innerY := deckW+1+2, y0+2
		innerW, innerH := tableW-4, midH-3
		pileW, pileH := w, h
		for _, p := range g.Table {
			if p.Defense != nil {
				pileW, pileH = w+2, h+2
			}
		}
		n := len(g.Table)
		rowW := n*pileW + (n-1)*3
		ox, oy := max(0, (innerW-rowW)/2), max(0, (innerH-pileH)/2)
		for i, p := range g.Table {
			px, py := innerX+ox+i*(pileW+3), innerY+oy
			lay.cards[p.Attack] = spot{x: px, y: py, zone: zoneTable, faceUp: true}
			if p.Defense != nil {
				lay.cards[*p.Defense] = spot{x: px + 2, y: py + 2, zone: zoneTable, faceUp: true}
			}
		}
	}

	// Your hand: under the badge row; the selected card is one line higher.
	handY := y0 + midH + 1
	you := g.Players[game.You].Hand
	x0 = leftOf(len(you) * w)
	for i, c := range you {
		y := handY + 1
		if i == m.cursor {
			y = handY
		}
		lay.cards[c] = spot{x: x0 + i*w, y: y, zone: zoneYouHand, faceUp: true}
	}
	return lay
}

// syncAnimations compares the new layout with the previous one and starts a
// flight for every card that changed zone. It runs after every Update.
func (m Model) syncAnimations() (Model, tea.Cmd) {
	_, lay := m.render()
	before := m.lastLayout
	m.lastLayout = lay

	from := func(c cards.Card) spot {
		if s, ok := before.cards[c]; ok {
			return s
		}
		// Not on screen before: it came out of the deck.
		d := before.deck
		if before.cards == nil {
			d = lay.deck // very first frame: the opening deal
		}
		return d
	}

	var leaving, played, drawn []anim
	for c, to := range lay.cards {
		f := from(c)
		if f.zone == to.zone {
			continue
		}
		a := anim{card: c, from: f, to: to}
		switch {
		case f.zone == zoneTable:
			leaving = append(leaving, a) // taken from the table
		case to.zone == zoneTable:
			played = append(played, a)
		default:
			drawn = append(drawn, a)
		}
	}
	for c, f := range before.cards {
		if _, ok := lay.cards[c]; !ok && f.zone == zoneTable {
			leaving = append(leaving, anim{card: c, from: f, to: lay.discard})
		}
	}
	if len(leaving)+len(played)+len(drawn) == 0 {
		return m, nil
	}

	// Timing: the table clears first, played cards go at once, and dealt
	// cards follow the clearing one by one, alternating between the players,
	// with the trump card last.
	now := time.Now()
	byX := func(a, b anim) int { return a.from.x - b.from.x }
	slices.SortFunc(leaving, byX)
	slices.SortFunc(played, byX)
	for i := range leaving {
		leaving[i].start = now.Add(time.Duration(i) * staggerClear)
	}
	for i := range played {
		played[i].start = now.Add(time.Duration(i) * staggerPlay)
	}
	drawStart := now
	if len(leaving) > 0 {
		drawStart = now.Add(time.Duration(len(leaving))*staggerClear + animDuration/2)
	}
	for i, a := range dealOrder(drawn) {
		a.start = drawStart.Add(time.Duration(i) * staggerDraw)
		m.anims = append(m.anims, a)
	}
	m.anims = append(m.anims, leaving...)
	m.anims = append(m.anims, played...)

	if m.animating {
		return m, nil // a frame loop is already running
	}
	m.animating = true
	return m, nextFrame()
}

// dealOrder interleaves the cards going to each hand (left to right), like a
// dealer going round the table, and puts the trump card last.
func dealOrder(drawn []anim) []anim {
	var you, opp, rest []anim
	for _, a := range drawn {
		switch a.to.zone {
		case zoneYouHand:
			you = append(you, a)
		case zoneOppHand:
			opp = append(opp, a)
		default:
			rest = append(rest, a)
		}
	}
	byX := func(a, b anim) int { return a.to.x - b.to.x }
	slices.SortFunc(you, byX)
	slices.SortFunc(opp, byX)
	out := make([]anim, 0, len(drawn))
	for i := 0; i < max(len(you), len(opp)); i++ {
		if i < len(you) {
			out = append(out, you[i])
		}
		if i < len(opp) {
			out = append(out, opp[i])
		}
	}
	return append(out, rest...)
}

// stepAnimations drops landed cards and keeps the frame loop going while any
// are still flying.
func (m Model) stepAnimations() (Model, tea.Cmd) {
	now := time.Now()
	m.anims = slices.DeleteFunc(m.anims, func(a anim) bool { return a.done(now) })
	if len(m.anims) == 0 {
		m.animating = false
		return m, nil
	}
	return m, nextFrame()
}

// withAnimations draws every card currently in flight on top of the screen.
func (m Model) withAnimations(screen string) string {
	if len(m.anims) == 0 {
		return screen
	}
	r := m.renderer()
	now := time.Now()
	layers := []*lipgloss.Layer{lipgloss.NewLayer(screen)}
	for i, a := range m.anims {
		if now.Before(a.start) || a.done(now) {
			continue // still waiting, or already landed
		}
		t := easeOut(float64(now.Sub(a.start)) / float64(animDuration))
		x := lerp(a.from.x, a.to.x, t)
		y := lerp(a.from.y, a.to.y, t)
		// Flip halfway when a card turns over (e.g. from the opponent's hand
		// to the table, or from the deck into your hand).
		faceUp := a.from.faceUp
		if t >= 0.5 {
			faceUp = a.to.faceUp
		}
		img := r.back()
		if faceUp {
			img = r.card(a.card, false)
		}
		layers = append(layers, lipgloss.NewLayer(img).X(max(0, x)).Y(max(0, y)).Z(i+1))
	}
	if len(layers) == 1 {
		return screen
	}
	return lipgloss.NewCompositor(layers...).Render()
}

// easeOut starts fast and slows down as the card lands (cubic ease-out).
func easeOut(t float64) float64 {
	t = max(0, min(1, t))
	return 1 - math.Pow(1-t, 3)
}

func lerp(a, b int, t float64) int {
	return a + int(math.Round(float64(b-a)*t))
}
