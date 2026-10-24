# terminal-durak

**Durak**, the classic Russian card game, played in your terminal. It's written in Go with [Bubble Tea](https://github.com/charmbracelet/bubbletea) and [Lip Gloss](https://github.com/charmbracelet/lipgloss).

```
                     ┏━━━━━┓┏━━━━━┓┏━━━━━┓┏━━━━━┓┏━━━━━┓
                     ┃~*~*~┃┃~*~*~┃┃~*~*~┃┃~*~*~┃┃~*~*~┃
                     ┃*(@)*┃┃*(@)*┃┃*(@)*┃┃*(@)*┃┃*(@)*┃
                     ┃~*~*~┃┃~*~*~┃┃~*~*~┃┃~*~*~┃┃~*~*~┃
                     ┗━━━━━┛┗━━━━━┛┗━━━━━┛┗━━━━━┛┗━━━━━┛
┌─────────────────┐ ┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓
│ DECK 24         │ ┃ TABLE                                                  ┃
│ ┏━━━━━┓ ┏━━━━━┓ │ ┃                                                        ┃
│ ┃~*~*~┃ ┃J    ┃ │ ┃                   ┏━━━━━┓   ┏━━━━━┓                    ┃
│ ┃*(@)*┃ ┃  ♣  ┃ │ ┃                   ┃8    ┃   ┃8    ┃                    ┃
│ ┃~*~*~┃ ┃    J┃ │ ┃                   ┃  ♠  ┃   ┃  ♥  ┃                    ┃
│ ┗━━━━━┛ ┗━━━━━┛ │ ┃                   ┃    8┃   ┃    8┃                    ┃
│ trump ♣         │ ┃                   ┗━━━━━┛   ┗━━━━━┛                    ┃
│ discarded 0     │ ┃                                                        ┃
└─────────────────┘ ┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛
                     ╔═════╗
                     ║7    ║┏━━━━━┓┏━━━━━┓┏━━━━━┓┏━━━━━┓
                     ║  ♥  ║┃8    ┃┃A    ┃┃7    ┃┃9    ┃
                     ║    7║┃  ♦  ┃┃  ♦  ┃┃  ♣  ┃┃  ♣  ┃
                     ╚═════╝┃    8┃┃    A┃┃    7┃┃    9┃
                            ┗━━━━━┛┗━━━━━┛┗━━━━━┛┗━━━━━┛

Defend!  Opponent passed the attack on with 8♥
```

## Features

- A full 36-card game (6 to Ace) against a computer opponent with three levels; Hard counts cards, plays out imagined deals and plays endings perfectly
- Card art that scales with your terminal size, red and black suits, trump cards edged in gold, and cards you can't play faded out
- Animated cards: dealing, playing, drawing and clearing the table
- Mouse support: hover, click and scroll
- Defence cards drawn overlapping the attack they beat, on a green felt table
- Durak rules: attacks must match a rank already on the table, defend with a higher card of the same suit or with a trump, take the cards, throw more in, and **pass the attack on** (the "perevodnoy" variant)
- The game ends and names the durak once the deck has run out

## Install and run

You need [Go](https://go.dev/dl/) 1.27 or newer.

```bash
git clone git@github.com:deryaerman/terminal-durak.git
cd terminal-durak
go run .
```

To build a standalone binary instead:

```bash
go build -o durak .
./durak
```

## Controls

| Key | Action |
|---|---|
| `←` `→` / `h` `l` | Select a card |
| `enter` | Play the selected card: attack, throw in, or defend |
| `r` | Pass the attack on with a card of the same rank (when defending) |
| `t` | Take every card on the table (when defending) |
| `space` | End your turn. As the attacker, this ends the round once everything is beaten |
| `s` | Sort your hand (trumps last) |
| `m` | Open the rules menu (pauses the game) |
| `n` | Start a new game (after a game ends) |
| `q` / `ctrl+c` | Quit |

**Mouse:** hover over a card to select it, click to play it, and scroll to move the selection. Buttons such as **End turn**, **Take** and **New game** appear next to your turn badge. In the rules menu, hover to choose a rule, click to change it, and click outside the menu to close it.

## How to play

Each player starts with 6 cards. The bottom card of the deck is turned face up, and its suit is **trump** for the whole game. Whoever holds the lowest trump attacks first.

1. The **attacker** plays any card. After that, they may only throw in cards whose rank is already on the table.
2. The **defender** beats each attack with a higher card of the same suit, or with any trump. A trump only loses to a higher trump.
3. Before beating anything, the defender may instead **pass the attack on** by adding a card of the same rank. The roles then swap.
4. If the defender beats everything, those cards are discarded and the defender attacks next. If they can't (or won't), they **take** every card on the table. The attacker may first throw in more matching cards for them to pick up, and then attacks again.
5. After each round, both players draw back up to 6 cards. The player who started the round draws first.

Once the deck is empty, the first player to get rid of all their cards wins. The player left holding cards is the **durak**, "the fool".

### House rules

Press `m` to change these. They apply straight away and carry over to new games.

| Rule | Default | What it does |
|---|---|---|
| Pass the attack on | on | Allows passing an attack on with a card of the same rank. Turn it off for plain throw-in (podkidnoy) Durak. |
| Pass on with several cards | off | After passing on, you may add more cards of that rank before your opponent defends. |
| Throw in after take | on | Once the defender gives up, the attacker may still add matching cards for them to pick up. |
| Opponent | hard | **Easy** plays simply. **Normal** counts cards and plays the endgame perfectly. **Hard** also tries each move out across many imagined deals of the hidden cards. **Hard+** does the same for a full half second per move, on every CPU core. **Expert** grows a search tree over the imagined deals (ISMCTS) for half a second. |
| Turn timer | 30s | Off, 15s, 30s or 60s. When it runs out, the game moves on for you: you lead your cheapest card, end your attack, or take the cards. |

## Project layout

```
main.go              starts the Bubble Tea program
internal/cards/      Card, Suit, Rank and Deck: the building blocks
internal/game/       match state, rules and the computer opponent (no UI code)
internal/ui/         Bubble Tea model (Update/View) and Lip Gloss rendering
```

The `game` package knows nothing about the terminal, so the same rules can later drive a network game or a smarter bot.

## Development

```bash
go test ./...   # run the tests
go vet ./...    # static checks
go fmt ./...    # format the code

DURAK_ARENA=1 go test ./internal/game -run Arena -v   # pit the bot levels against each other
```

## Roadmap

- [ ] A neural-network opponent trained with self-play
- [ ] Online multiplayer
- [ ] More than two players

## License

[MIT](LICENSE)
