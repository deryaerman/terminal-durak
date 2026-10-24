// Package game holds the state of a Durak match and the moves that change it.
// It knows nothing about the terminal: the UI reads this state and calls these
// methods, so the same code can later drive a network game or a smarter bot.
package game

import (
	"errors"
	"fmt"
	"slices"

	"durak/internal/cards"
)

// HandSize is how many cards each player holds after drawing.
const HandSize = 6

// Player indexes. Index 0 is always the person at this terminal.
const (
	You      = 0
	Opponent = 1
	Draw     = -1 // Loser value when nobody lost
)

// Pair is one attack card on the table plus the card that beat it, if any.
// Defense is a pointer so that "not defended yet" can be nil.
type Pair struct {
	Attack  cards.Card
	Defense *cards.Card
}

// Player is a name and a hand of cards.
type Player struct {
	Name string
	Hand []cards.Card
}

// Rules are the house rules a match is played with. The UI's menu edits them.
type Rules struct {
	// PassOn lets the defender pass the attack on with a card of the same
	// rank ("perevodnoy" Durak). Off gives plain throw-in ("podkidnoy") Durak.
	PassOn bool
	// PassOnMany lets the defender pass the attack on with several cards of
	// that rank at once, not just one.
	PassOnMany bool
	// ThrowInAfterTake lets the attacker add more matching cards after the
	// defender decides to take; the defender then picks those up too.
	ThrowInAfterTake bool
	// Difficulty picks the computer opponent: Easy, Normal or Hard.
	Difficulty Difficulty
}

// Difficulty is how strong the computer opponent plays.
type Difficulty int

const (
	Easy     Difficulty = iota // the original simple bot
	Normal                     // card counting and a perfect endgame (heuristic.go, endgame.go)
	Hard                       // plus Monte Carlo search over 96 imagined deals (search.go)
	HardPlus                   // the same search, but for a fixed half second per move
	Expert                     // ISMCTS: a search tree over imagined deals, half a second (ismcts.go)

	NumDifficulties = 5
)

func (d Difficulty) String() string {
	return [...]string{"easy", "normal", "hard", "hard+", "expert"}[d]
}

// Slow reports whether this level thinks long enough that the UI should run
// it in the background.
func (d Difficulty) Slow() bool { return d == HardPlus || d == Expert }

// ThinkMove picks p's move at this difficulty without changing s. The UI
// calls it on a copy in the background for the slow levels.
func (s *State) ThinkMove(p int) Move {
	switch s.Rules.Difficulty {
	case Expert:
		return s.ISMCTSMove(p, ExpertThinkTime)
	case HardPlus:
		return s.SearchMoveFor(p, HardPlusThinkTime)
	case Hard:
		return s.SearchMove(p)
	default:
		if m, ok := s.EndgameMove(p); ok {
			return m
		}
		return s.HeuristicMove(p)
	}
}

// DefaultRules are the classic rules: pass-on Durak with throw-in after take.
var DefaultRules = Rules{PassOn: true, ThrowInAfterTake: true, Difficulty: Hard}

// State is everything about one match.
type State struct {
	Rules    Rules
	Deck     *cards.Deck
	Trump    cards.Card // the face-up card under the deck; its suit is trump
	Players  [2]*Player
	Table    []Pair
	Attacker int  // who is attacking right now (changes when the attack is passed on)
	Leader   int  // who started this round's attack; draws first when it ends
	Taking   bool // the defender has given up; the attacker may still add cards

	// What everyone at the table has seen (memory.go): cards discarded for
	// good, and cards known to be in each player's hand because they picked
	// them up from the table or drew the face-up trump.
	Gone  cards.Set
	Known [2]cards.Set

	// The endgame solver's memory of positions, per point of view, kept for
	// the whole game. A pointer, so copies of the state share it.
	endgameCache *[2]map[posKey]float64

	Turn     int // whose move it is right now
	Discards int // cards beaten and removed from play

	// DeferAdvance stops a stuck attacker's turn from ending instantly; the
	// UI sets it and calls Advance after a pause. Copies made with Clone
	// (the bots' imagined games) always advance instantly.
	DeferAdvance bool
	// Ply counts the moves made so far. The UI uses it to check that a move
	// the bot worked out in the background still fits the current game.
	Ply    int
	Status string // last thing that happened, shown to the player
	Over   bool   // the game has ended
	Loser  int    // the durak: You, Opponent, or Draw when both ran out together
}

// Errors returned by moves. The UI shows them in the status line.
var (
	ErrNotYourTurn    = errors.New("it's not your turn")
	ErrCannotBeat     = errors.New("that card can't beat the attack")
	ErrNothingToDo    = errors.New("nothing to defend")
	ErrTableFull      = errors.New("no more attacks: the table is full or the defender is out of cards")
	ErrRankNotOnTable = errors.New("you can only add cards whose rank is already on the table")
	ErrGameOver       = errors.New("the game is over — press n for a new game")
	ErrNothingPlayed  = errors.New("play at least one card first")
	ErrCannotTransfer = errors.New("can't pass on: ranks must match, nothing defended yet, and they need enough cards")
	ErrMustBeatOrTake = errors.New("beat every card or take them (t)")
	ErrTaking         = errors.New("you're taking the cards — wait for the attacker")
)

// New starts a match with the default rules.
func New() *State { return NewWithRules(DefaultRules) }

// NewWithRules shuffles a deck, deals six cards each, turns up the trump card
// and lets the holder of the lowest trump attack first.
func NewWithRules(r Rules) *State {
	d := cards.NewDeck()
	d.Shuffle()

	s := &State{
		Rules:   r,
		Deck:    d,
		Players: [2]*Player{{Name: "You"}, {Name: "Opponent"}},
	}
	for _, p := range s.Players {
		p.Hand, _ = d.Draw(HandSize) // a fresh deck always has enough cards
	}

	// The trump card is the bottom of the deck (index 0, since we draw from
	// the end). It stays in the deck and is the last card drawn.
	s.Trump = d.Cards[0]
	s.Attacker = s.lowestTrumpHolder()
	s.Leader, s.Turn = s.Attacker, s.Attacker
	if s.Attacker == You {
		s.Status = "You hold the lowest trump, so you attack first."
	} else {
		s.Status = "The opponent holds the lowest trump and attacks first."
	}
	s.SortHand(You)
	return s
}

// lowestTrumpHolder returns the player holding the lowest trump. If nobody
// holds a trump, you start.
func (s *State) lowestTrumpHolder() int {
	best, bestRank := You, cards.Ace+1
	for p, pl := range s.Players {
		for _, c := range pl.Hand {
			if c.Suit == s.Trump.Suit && c.Rank < bestRank {
				best, bestRank = p, c.Rank
			}
		}
	}
	return best
}

// Beats reports whether defense beats attack: a higher card of the same suit,
// or any trump against a non-trump.
func (s *State) Beats(defense, attack cards.Card) bool {
	trump := s.Trump.Suit
	if defense.Suit == attack.Suit {
		return defense.Rank > attack.Rank
	}
	return defense.Suit == trump && attack.Suit != trump
}

// Defender is whoever is not attacking.
func (s *State) Defender() int { return 1 - s.Attacker }

// Undefended returns the index of the first table pair without a defense,
// or -1 if every attack has been beaten.
func (s *State) Undefended() int {
	return slices.IndexFunc(s.Table, func(p Pair) bool { return p.Defense == nil })
}

// Play puts card i from player p's hand on the table: as a new attack when p
// is attacking, or on the first undefended attack when p is defending.
func (s *State) Play(p, i int) error {
	s.Ply++
	if s.Over {
		return ErrGameOver
	}
	if s.Turn != p {
		return ErrNotYourTurn
	}
	hand := s.Players[p].Hand
	card := hand[i]

	if p == s.Attacker {
		if err := s.checkAttack(card); err != nil {
			return err
		}
		s.Table = append(s.Table, Pair{Attack: card})
	} else {
		if s.Taking {
			return ErrTaking
		}
		u := s.Undefended()
		if u < 0 {
			return ErrNothingToDo
		}
		if !s.Beats(card, s.Table[u].Attack) {
			// Can't beat it, but a card of the same rank passes the attack on.
			if s.CanTransfer(card) {
				return s.Transfer(p, i)
			}
			return ErrCannotBeat
		}
		s.Table[u].Defense = &card
	}
	s.Players[p].Hand = slices.Delete(hand, i, i+1)
	s.Known[p] = s.Known[p].Remove(card)
	s.Status = fmt.Sprintf("%s played %v", s.Players[p].Name, card)
	if p != s.Attacker && s.Undefended() < 0 {
		s.Turn = s.Attacker // everything is beaten: nothing left for the defender to do
	}
	s.advanceIfStuck()
	return nil
}

// advanceIfStuck ends the attacker's turn automatically when they have no
// card they're allowed to add, so nobody has to press space for nothing:
// the attack passes to the defender, the round ends if everything is beaten,
// or the cards are handed over if the defender is taking.
func (s *State) advanceIfStuck() {
	// When the table is about to clear (everything beaten, or the defender
	// taking), the UI may want to pause first so the last card stays
	// visible; it then calls Advance itself. Passing the attack on to the
	// defender never waits.
	if s.DeferAdvance && (s.Undefended() < 0 || s.Taking) {
		return
	}
	s.Advance()
}

// Stuck reports whether the attacker is on the move with nothing they're
// allowed to add, so their turn should end by itself.
func (s *State) Stuck() bool {
	return !s.Over && s.Turn == s.Attacker && len(s.Table) > 0 && !s.canAdd(s.Attacker)
}

// Advance ends a stuck attacker's turn. With DeferAdvance set, the UI calls
// this itself after a short pause, so you can see the last card that was
// played before the table clears.
func (s *State) Advance() {
	if s.Stuck() {
		_ = s.EndTurn()
	}
}

// checkAttack says whether the attacker may put card on the table.
//   - The first card of a round can be anything.
//   - After that, only ranks already on the table (attacks or defenses).
//   - At most 6 attacks per round, and never more unbeaten cards than the
//     defender can answer with the cards in their hand.
func (s *State) checkAttack(card cards.Card) error {
	if len(s.Table) == 0 {
		return nil
	}
	if len(s.Table) >= HandSize || s.unbeaten() >= len(s.Players[s.Defender()].Hand) {
		return ErrTableFull
	}
	for _, p := range s.Table {
		if p.Attack.Rank == card.Rank || (p.Defense != nil && p.Defense.Rank == card.Rank) {
			return nil
		}
	}
	return ErrRankNotOnTable
}

// unbeaten counts attack cards that have no defense yet.
func (s *State) unbeaten() int {
	n := 0
	for _, p := range s.Table {
		if p.Defense == nil {
			n++
		}
	}
	return n
}

// CanPlay reports whether player p could play card right now. The UI uses it
// to dim the cards you can't play.
func (s *State) CanPlay(p int, card cards.Card) bool {
	if s.Over || s.Turn != p {
		return false
	}
	if p == s.Attacker {
		return s.checkAttack(card) == nil
	}
	u := s.Undefended()
	if u < 0 {
		return false
	}
	return s.Beats(card, s.Table[u].Attack) || s.CanTransfer(card)
}

// CanTransfer reports whether the defender may pass the attack on with card:
// nothing has been defended yet, every attack has the same rank as card, and
// the other player holds enough cards to face all of them.
func (s *State) CanTransfer(card cards.Card) bool {
	if !s.Rules.PassOn || s.Taking || len(s.Table) == 0 || len(s.Table) >= HandSize {
		return false
	}
	for _, p := range s.Table {
		if p.Defense != nil || p.Attack.Rank != card.Rank {
			return false
		}
	}
	return len(s.Players[s.Attacker].Hand) >= len(s.Table)+1
}

// Transfer ("return defence") adds card i as one more attack and swaps the
// roles: the old attacker now has to beat every card on the table.
func (s *State) Transfer(p, i int) error {
	s.Ply++
	if s.Over {
		return ErrGameOver
	}
	if s.Turn != p || p != s.Defender() {
		return ErrNotYourTurn
	}
	card := s.Players[p].Hand[i]
	if !s.CanTransfer(card) {
		return ErrCannotTransfer
	}
	s.Table = append(s.Table, Pair{Attack: card})
	s.Players[p].Hand = slices.Delete(s.Players[p].Hand, i, i+1)
	s.Known[p] = s.Known[p].Remove(card)
	s.Attacker = p
	s.Status = fmt.Sprintf("%s passed the attack on with %v", s.Players[p].Name, card)
	if s.Rules.PassOnMany && s.canAdd(p) {
		// Stay on the move to add more cards of the same rank.
		s.Status += fmt.Sprintf(" — add more %ss or press space", card.Rank.Label())
		return nil
	}
	s.Turn = s.Defender()
	return nil
}

// EndTurn hands the move to the other player. When the attacker ends their
// turn and every attack on the table is beaten, the round is over.
func (s *State) EndTurn() error {
	s.Ply++
	if s.Over {
		return ErrGameOver
	}
	if s.Turn == s.Attacker {
		if len(s.Table) == 0 {
			return ErrNothingPlayed
		}
		if s.Taking { // done adding: the defender picks everything up
			s.collect()
			return nil
		}
		if s.Undefended() < 0 {
			s.finishRound(false)
			return nil
		}
	} else if s.Undefended() >= 0 {
		return ErrMustBeatOrTake // a defender can't just walk away
	}
	s.Turn = 1 - s.Turn
	s.advanceIfStuck()
	return nil
}

// Take means the defender gives up and will pick up every card on the table.
// If the rules allow it and the attacker has matching cards, the attacker may
// first throw in more (see EndTurn); otherwise the cards are collected now.
// Either way the same attacker attacks again next round.
func (s *State) Take() {
	s.Ply++
	if s.Over || s.Taking {
		return
	}
	s.Taking = true
	if s.Rules.ThrowInAfterTake && s.canAdd(s.Attacker) {
		s.Turn = s.Attacker
		s.Status = fmt.Sprintf("%s will take — %s may add more cards, then space",
			s.Players[s.Defender()].Name, s.Players[s.Attacker].Name)
		return
	}
	s.collect()
}

// canAdd reports whether player p (attacking) could legally add any card.
func (s *State) canAdd(p int) bool {
	for _, c := range s.Players[p].Hand {
		if s.checkAttack(c) == nil {
			return true
		}
	}
	return false
}

// collect moves every card on the table into the defender's hand.
func (s *State) collect() {
	d := s.Players[s.Defender()]
	for _, pair := range s.Table {
		d.Hand = append(d.Hand, pair.Attack)
		if pair.Defense != nil {
			d.Hand = append(d.Hand, *pair.Defense)
		}
	}
	s.Known[s.Defender()] = s.Known[s.Defender()].Union(s.tableSet()) // everyone saw them
	s.Status = fmt.Sprintf("%s took %d cards", d.Name, s.tableCount())
	s.finishRound(true)
}

// finishRound clears the table, refills hands (attacker first) and decides who
// attacks next: the same attacker if the defender took the cards, otherwise
// the defender becomes the attacker.
func (s *State) finishRound(taken bool) {
	if !taken {
		s.Gone = s.Gone.Union(s.tableSet())
		s.Discards += s.tableCount()
		s.Status = "All attacks beaten — the cards are discarded"
	}
	s.Table = nil
	s.Taking = false
	// The player who started the round draws first, even if the attack was
	// passed on since; then the other player.
	s.DrawUp(s.Leader)
	s.DrawUp(1 - s.Leader)
	if !taken {
		s.Attacker = s.Defender()
	}
	s.Leader, s.Turn = s.Attacker, s.Attacker
	s.SortHand(You)
	s.checkGameOver()
}

// checkGameOver ends the game once the deck is empty and someone has no cards
// left. Players who run out are safe; the one still holding cards is the durak.
func (s *State) checkGameOver() {
	if s.Deck.Len() > 0 {
		return
	}
	youOut := len(s.Players[You].Hand) == 0
	oppOut := len(s.Players[Opponent].Hand) == 0
	switch {
	case youOut && oppOut:
		s.Over, s.Loser = true, Draw
		s.Status = "Both players ran out of cards at once — it's a draw"
	case youOut:
		s.Over, s.Loser = true, Opponent
		s.Status = "You got rid of all your cards — you win!"
	case oppOut:
		s.Over, s.Loser = true, You
		s.Status = "The opponent got rid of all their cards — you're the durak!"
	}
}

// DrawUp draws for player p until they hold HandSize cards or the deck is empty.
func (s *State) DrawUp(p int) {
	need := min(HandSize-len(s.Players[p].Hand), s.Deck.Len())
	if need <= 0 {
		return
	}
	drawn, _ := s.Deck.Draw(need)
	s.Players[p].Hand = append(s.Players[p].Hand, drawn...)
	if s.Deck.Len() == 0 && slices.Contains(drawn, s.Trump) {
		s.Known[p] = s.Known[p].Add(s.Trump) // the face-up trump went to p
	}
}

// SortHand orders player p's hand by suit, then rank, with trumps last.
func (s *State) SortHand(p int) {
	trump := s.Trump.Suit
	key := func(c cards.Card) int {
		suit := int(c.Suit)
		if c.Suit == trump {
			suit = 10 // push trumps to the end
		}
		return suit*100 + int(c.Rank)
	}
	slices.SortFunc(s.Players[p].Hand, func(a, b cards.Card) int {
		return key(a) - key(b)
	})
}

func (s *State) tableCount() int {
	n := 0
	for _, p := range s.Table {
		n++
		if p.Defense != nil {
			n++
		}
	}
	return n
}

// tableSet is every card on the table, attacks and defences.
func (s *State) tableSet() cards.Set {
	var t cards.Set
	for _, p := range s.Table {
		t = t.Add(p.Attack)
		if p.Defense != nil {
			t = t.Add(*p.Defense)
		}
	}
	return t
}
