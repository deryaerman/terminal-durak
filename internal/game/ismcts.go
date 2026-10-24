package game

import (
	"math"
	"math/rand/v2"
	"runtime"
	"sync"
	"time"
)

// The Expert bot: Information Set Monte Carlo Tree Search (ISMCTS, Cowling,
// Powley & Whitehouse 2012), the "single observer" version.
//
// PIMC (search.go) plays each move out once per imagined deal and then lets
// the Normal bot finish. ISMCTS instead grows ONE search tree shared by all
// the imagined deals, exploring further moves for both players:
//
//	repeat until time is up:
//	  1. deal the hidden cards at random (a new imagined deal every time)
//	  2. SELECT: walk down the tree, at each step only considering moves that
//	     are legal in this deal, picking the one with the best balance of
//	     "has won often" and "hasn't been tried much" (the UCB formula)
//	  3. EXPAND: add one untried move as a new node
//	  4. SIMULATE: play the rest of the game out with the Normal bot
//	  5. BACK UP: every node on the path records the result for the player
//	     who made that move
//	finally play the root move that was visited most
//
// Because one tree has to work for every deal, the bot can't pick moves that
// only work if it secretly knew the hidden cards, PIMC's main weakness
// ("strategy fusion"). And because the tree explores replies, it looks
// further ahead than a single play-out.

// ExpertThinkTime is how long the Expert bot searches per decision.
const ExpertThinkTime = 500 * time.Millisecond

// uctC balances trying promising moves against exploring rarely-tried ones.
// Results are between 0 and 1, so a value around 0.7 is the usual start.
const uctC = 0.7

// treeNode is one move in the search tree.
type treeNode struct {
	move     Move
	mover    int // who made the move leading here
	parent   *treeNode
	children map[Move]*treeNode

	visits int
	wins   float64 // results for mover, summed over visits
	avail  int     // how often this move was legal when its parent was visited
}

func newNode(parent *treeNode, m Move, mover int) *treeNode {
	return &treeNode{move: m, mover: mover, parent: parent, children: map[Move]*treeNode{}}
}

// ucb scores a child for selection. avail (instead of the parent's visit
// count) makes the formula fair to moves that are only legal in some deals.
func (n *treeNode) ucb() float64 {
	return n.wins/float64(n.visits) + uctC*math.Sqrt(math.Log(float64(n.avail))/float64(n.visits))
}

// ISMCTSMove searches for d and returns p's move. Each CPU core grows its own
// tree ("root parallelisation"); their root visit counts are added up.
func (s *State) ISMCTSMove(p int, d time.Duration) Move {
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

	workers := runtime.NumCPU()
	deadline := time.Now().Add(d)
	seed := rand.Uint64()
	roots := make([]*treeNode, workers)
	var wg sync.WaitGroup
	for w := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rng := rand.New(rand.NewPCG(seed, uint64(w)))
			root := newNode(nil, Move{}, 1-p)
			for time.Now().Before(deadline) {
				s.ismctsIteration(root, p, rng)
			}
			roots[w] = root
		}()
	}
	wg.Wait()

	visits := map[Move]int{}
	for _, r := range roots {
		for m, c := range r.children {
			visits[m] += c.visits
		}
	}
	best, bestV := moves[0], -1
	for _, m := range moves {
		if visits[m] > bestV {
			best, bestV = m, visits[m]
		}
	}
	return best
}

// ismctsIteration runs one select–expand–simulate–back-up pass.
func (s *State) ismctsIteration(root *treeNode, p int, rng *rand.Rand) {
	g := s.Determinize(p, rng)
	node := root

	// SELECT and EXPAND.
	for !g.Over {
		legal := g.LegalMoves(g.Turn)
		if len(legal) == 0 {
			break
		}
		var untried []Move
		for _, m := range legal {
			c, ok := node.children[m]
			if !ok {
				untried = append(untried, m)
				continue
			}
			c.avail++
		}
		if len(untried) > 0 {
			m := untried[rng.IntN(len(untried))]
			child := newNode(node, m, g.Turn)
			child.avail = 1
			node.children[m] = child
			if g.Apply(g.Turn, m) != nil {
				return
			}
			node = child
			break
		}
		var best *treeNode
		bestScore := -1.0
		for _, m := range legal {
			c := node.children[m]
			if sc := c.ucb(); best == nil || sc > bestScore {
				best, bestScore = c, sc
			}
		}
		if g.Apply(g.Turn, best.move) != nil {
			return
		}
		node = best
	}

	// SIMULATE: finish the game with the Normal bot.
	result := rollout(g, p) // from p's point of view

	// BACK UP: each node scores the result for the player who moved into it.
	for n := node; n != nil; n = n.parent {
		n.visits++
		if n.mover == p {
			n.wins += result
		} else {
			n.wins += 1 - result
		}
	}
}
