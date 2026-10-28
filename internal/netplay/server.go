package netplay

import (
	"bufio"
	"context"
	"errors"
	"net"
	"time"

	"durak/internal/game"
)

// The game server. It holds the only real game.State; players send moves,
// the server checks them with the rules engine and sends each player their
// own View back. It also owns everything time-based (the turn timer, the
// pause before the table clears) so both players see the same clock.
//
// Concurrency design: every connection has a goroutine that reads messages,
// and timers fire on their own goroutines. None of them touch the game.
// They all send "events" into one channel, and a single loop (run) handles
// them one at a time. One goroutine owning all the state means no locks.

// advanceDelay is how long the table stays visible before it clears.
const advanceDelay = 1200 * time.Millisecond

type eventKind int

const (
	evJoin    eventKind = iota // a new connection arrived
	evLeave                    // a connection closed
	evMessage                  // a player sent something
	evTimeout                  // a turn timer ran out
	evAdvance                  // the pause before clearing the table is over
)

type event struct {
	kind eventKind
	conn *conn
	msg  Message
	id   int // for timers: which timer fired (old ones are ignored)
}

// conn is one connected player.
type conn struct {
	net.Conn
	name string
	seat int
}

// Server runs one two-player table.
type Server struct {
	ln       net.Listener
	rules    game.Rules
	turnTime time.Duration
	events   chan event

	// Owned by run(); never touched by other goroutines.
	seats    [2]*conn
	state    *game.State
	timerID  int // bumped whenever the turn timer restarts
	timerFor int // whose turn the running timer is for (-1: none)
	deadline time.Time
	waitPly  int // the Ply an advance is scheduled for (-1: none)
}

// Listen opens the server on addr (":4747", or ":0" for any free port).
func Listen(addr string, rules game.Rules, turnTime time.Duration) (*Server, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	return &Server{ln: ln, rules: rules, turnTime: turnTime, events: make(chan event, 16),
		timerFor: -1, waitPly: -1}, nil
}

// Port is the TCP port the server listens on.
func (s *Server) Port() int { return s.ln.Addr().(*net.TCPAddr).Port }

// Run serves until ctx is cancelled.
func (s *Server) Run(ctx context.Context) error {
	go s.accept()
	defer s.ln.Close()
	for {
		select {
		case <-ctx.Done():
			for _, c := range s.seats {
				if c != nil {
					c.Close()
				}
			}
			return nil
		case ev := <-s.events:
			s.handle(ev)
		}
	}
}

// accept waits for players and starts a reader goroutine for each.
func (s *Server) accept() {
	for {
		nc, err := s.ln.Accept()
		if err != nil {
			return // listener closed
		}
		c := &conn{Conn: nc, seat: -1}
		go s.read(c)
	}
}

// read turns everything a player sends into events. The first message must
// be a hello.
func (s *Server) read(c *conn) {
	r := bufio.NewReader(c)
	hello, err := ReadMessage(r)
	if err != nil || hello.Type != TypeHello {
		c.Close()
		return
	}
	c.name = hello.Name
	s.events <- event{kind: evJoin, conn: c}
	for {
		m, err := ReadMessage(r)
		if err != nil {
			s.events <- event{kind: evLeave, conn: c}
			return
		}
		s.events <- event{kind: evMessage, conn: c, msg: m}
	}
}

func (s *Server) send(c *conn, m Message) {
	if c != nil {
		_ = WriteMessage(c, m) // a broken connection shows up as evLeave
	}
}

func (s *Server) handle(ev event) {
	switch ev.kind {
	case evJoin:
		seat := -1
		for i, c := range s.seats {
			if c == nil {
				seat = i
				break
			}
		}
		if seat < 0 {
			s.send(ev.conn, Message{Type: TypeError, Error: "this game is full"})
			ev.conn.Close()
			return
		}
		ev.conn.seat = seat
		s.seats[seat] = ev.conn
		s.send(ev.conn, Message{Type: TypeWelcome})
		if s.seats[0] != nil && s.seats[1] != nil {
			s.newGame()
		}

	case evLeave:
		c := ev.conn
		if c.seat < 0 || s.seats[c.seat] != c {
			return
		}
		s.seats[c.seat] = nil
		s.state = nil // the game is abandoned; a new one starts when someone joins
		s.send(s.seats[1-c.seat], Message{Type: TypeLeft, Name: c.name})

	case evMessage:
		s.handleMessage(ev.conn, ev.msg)

	case evTimeout:
		if s.state != nil && ev.id == s.timerID && !s.state.Over {
			s.state.Timeout(s.state.Turn)
			s.changed()
		}

	case evAdvance:
		if s.state != nil && s.state.Ply == ev.id {
			s.waitPly = -1
			s.state.Advance()
			s.changed()
		}
	}
}

func (s *Server) handleMessage(c *conn, m Message) {
	switch m.Type {
	case TypeMove:
		if s.state == nil || m.Move == nil {
			return
		}
		if err := s.apply(c.seat, *m.Move); err != nil {
			s.send(c, Message{Type: TypeError, Error: err.Error()})
			return
		}
		s.changed()
	case TypeRematch:
		if s.state != nil && s.state.Over {
			s.newGame()
		}
	}
}

// apply checks and makes a move for the player in seat.
func (s *Server) apply(seat int, m game.Move) error {
	if s.state.Over {
		return game.ErrGameOver
	}
	if s.state.Turn != seat {
		return game.ErrNotYourTurn
	}
	if s.state.Stuck() {
		return errors.New("wait a moment, the round is ending")
	}
	return s.state.Apply(seat, m)
}

func (s *Server) newGame() {
	g := game.NewWithRules(s.rules)
	g.Players[0].Name, g.Players[1].Name = s.seats[0].name, s.seats[1].name
	g.DeferAdvance = true
	if g.Attacker == 0 {
		g.Status = s.seats[0].name + " holds the lowest trump and attacks first."
	} else {
		g.Status = s.seats[1].name + " holds the lowest trump and attacks first."
	}
	s.state = g
	s.timerFor, s.waitPly = -1, -1
	s.changed()
}

// changed runs after every change to the game: it restarts the turn timer
// when the move passes to the other player, schedules the pause before the
// table clears, and sends both players their new view.
func (s *Server) changed() {
	g := s.state
	if s.turnTime > 0 && !g.Over && g.Turn != s.timerFor {
		s.timerFor = g.Turn
		s.timerID++
		s.deadline = time.Now().Add(s.turnTime)
		id := s.timerID
		time.AfterFunc(s.turnTime, func() { s.events <- event{kind: evTimeout, id: id} })
	}
	if g.Over {
		s.timerFor = -1
	}
	if g.Stuck() && s.waitPly != g.Ply {
		s.waitPly = g.Ply
		ply := g.Ply
		time.AfterFunc(advanceDelay, func() { s.events <- event{kind: evAdvance, id: ply} })
	}
	for seat, c := range s.seats {
		v := g.ViewFor(seat)
		if s.turnTime > 0 && !g.Over {
			v.TurnTimeMs = int(s.turnTime.Milliseconds())
			v.TimeLeftMs = int(max(0, time.Until(s.deadline)).Milliseconds())
		}
		s.send(c, Message{Type: TypeView, View: &v})
	}
}
