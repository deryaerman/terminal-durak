package ui

import "charm.land/lipgloss/v2"

// Colours. lipgloss.Color accepts ANSI numbers ("9") or hex ("#ff5f87").
var (
	colRed    = lipgloss.Color("#ff5f6d")
	colTrump  = lipgloss.Color("#ffd75f")
	colSelect = lipgloss.Color("#5fd7ff")
	colDim    = lipgloss.Color("#6c6c6c")
	colBack   = lipgloss.Color("#5f5fd7")
	colFelt   = lipgloss.Color("#2e7d4a") // card-table green

	// White playing cards: dark ink, red suits, and a darker gold edge for
	// trumps so it still shows up against white.
	colCard      = lipgloss.Color("#f5f5f0")
	colInk       = lipgloss.Color("#1c1c1c")
	colRedInk    = lipgloss.Color("#d0021b")
	colEdge      = lipgloss.Color("#8a8a8a")
	colTrumpEdge = lipgloss.Color("#c08a00")
	colSelEdge   = lipgloss.Color("#0a64d6")
	colFaded     = lipgloss.Color("#c2c2c2") // ink of cards you can't play
)

var (
	// A card face: thick square-cornered border (rounded corners show up as
	// empty boxes in some terminal fonts), sized per frame by the renderer.
	cardStyle = lipgloss.NewStyle().
			Border(lipgloss.ThickBorder()).
			BorderForeground(colEdge).
			Background(colCard).
			BorderBackground(colCard)

	// The selected card in your hand gets a double, coloured border.
	selectedStyle = cardStyle.
			Border(lipgloss.DoubleBorder()).
			BorderForeground(colSelEdge)

	// A face-down card.
	backStyle = cardStyle.
			Foreground(colBack).
			BorderForeground(colBack)

	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(colDim).
			Padding(0, 1)

	titleStyle  = lipgloss.NewStyle().Bold(true)
	dimStyle    = lipgloss.NewStyle().Foreground(colDim)
	statusStyle = lipgloss.NewStyle().Foreground(colTrump)
	errorStyle  = lipgloss.NewStyle().Foreground(colRed)
	// The table: green felt inside a normal panel border.
	feltStyle  = lipgloss.NewStyle().Background(colFelt)
	feltText   = lipgloss.NewStyle().Background(colFelt).Foreground(lipgloss.Color("#a8c8a8"))
	tableStyle = panelStyle.Background(colFelt).
			Border(lipgloss.ThickBorder()).
			BorderForeground(colEdge)

	winStyle = lipgloss.NewStyle().Bold(true).Foreground(colSelect)
)
