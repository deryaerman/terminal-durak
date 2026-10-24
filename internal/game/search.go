package game

import (
	"math/rand/v2"
	"runtime"
	"sync"
	"time"
)

// The Hard bot: Monte Carlo search over imagined deals (PIMC, "Perfect
// Information Monte Carlo").
//
//  1. Deal the cards p can't see at random, consistently with what p knows,
//     many times over ("worlds", see Determinize).
//  2. In every world, try each legal move and play the rest of the game out
//     with the Normal bot on both sides.
//  3. Pick the move that did best across all worlds.
//
// The same worlds are used for every move, so moves are compared on equal
// terms. Once the deck is empty, the perfect endgame solver takes over.

// searchWorlds is how many imagined deals are played out per decision.
const searchWorlds = 96

// rolloutLimit stops an imagined game that somehow runs too long.
const rolloutLimit = 1000

// HardPlusThinkTime is how long the hard+ bot searches per decision.
const HardPlusThinkTime = 500 * time.Millisecond

// SearchMove picks a move for player p with the Hard strategy: a fixed
// number of imagined deals.
func (s *State) SearchMove(p int) Move { return s.search(p, 0) }

// SearchMoveFor searches for a fixed time instead (hard+), playing out as
// many imagined deals as fit, on every CPU core.
func (s *State) SearchMoveFor(p int, d time.Duration) Move { return s.search(p, d) }

// search plays out searchWorlds deals, or, when budget > 0, keeps going
// until the time is up.
func (s *State) search(p int, budget time.Duration) Move {
	moves := s.LegalMoves(p)
	switch len(moves) {
	case 0:
		return Move{Kind: EndTurnMove}
	case 1:
		return moves[0]
	}
	if m, ok := s.EndgameMove(p); ok {
		return m
	}

	// Play out the worlds in parallel, one goroutine per CPU core. Each
	// goroutine has its own random generator and its own slice of scores,
	// so they never write to the same memory.
	workers := runtime.NumCPU()
	deadline := time.Now().Add(budget)
	scores := make([][]float64, workers)
	seed := rand.Uint64()
	var wg sync.WaitGroup
	for w := range workers {
		scores[w] = make([]float64, len(moves))
		wg.Add(1)
		go func() {
			defer wg.Done()
			rng := rand.New(rand.NewPCG(seed, uint64(w)))
			for i := w; ; i += workers {
				if budget > 0 && time.Now().After(deadline) || budget == 0 && i >= searchWorlds {
					break
				}
				world := s.Determinize(p, rng)
				for mi, m := range moves {
					g := world.Clone()
					if g.Apply(p, m) != nil {
						continue
					}
					scores[w][mi] += rollout(g, p)
				}
			}
		}()
	}
	wg.Wait()

	// Add up the workers' scores; ties go to the move the heuristic prefers.
	h := s.HeuristicMove(p)
	best, bestScore := -1, -1.0
	for mi, m := range moves {
		total := 0.0
		for w := range workers {
			total += scores[w][mi]
		}
		if total > bestScore || total == bestScore && m == h {
			best, bestScore = mi, total
		}
	}
	return moves[best]
}

// rollout plays g to the end with the Normal bot on both sides and returns
// the result for p: 1 win, 0.5 draw, 0 loss.
func rollout(g *State, p int) float64 {
	for step := 0; !g.Over && step < rolloutLimit; step++ {
		if g.Apply(g.Turn, g.HeuristicMove(g.Turn)) != nil {
			return 0.5 // shouldn't happen; count it as nothing learned
		}
	}
	switch {
	case !g.Over:
		return 0.5
	case g.Loser == 1-p:
		return 1
	case g.Loser == p:
		return 0
	default:
		return 0.5
	}
}
