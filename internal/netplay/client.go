package netplay

import (
	"bufio"
	"net"
	"time"

	"durak/internal/game"
)

// Client is one player's connection to a server.
type Client struct {
	conn net.Conn
	// Incoming delivers everything the server sends, in order. It's closed
	// when the connection ends.
	Incoming <-chan Message
}

// Dial connects to the server at addr ("192.168.1.23:4747") and says hello.
func Dial(addr, name string) (*Client, error) {
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, err
	}
	if err := WriteMessage(conn, Message{Type: TypeHello, Name: name}); err != nil {
		conn.Close()
		return nil, err
	}
	in := make(chan Message, 16)
	go func() {
		defer close(in)
		r := bufio.NewReader(conn)
		for {
			m, err := ReadMessage(r)
			if err != nil {
				return
			}
			in <- m
		}
	}()
	return &Client{conn: conn, Incoming: in}, nil
}

// Move sends a move to the server.
func (c *Client) Move(m game.Move) error {
	return WriteMessage(c.conn, Message{Type: TypeMove, Move: &m})
}

// Rematch asks the server for a new game once this one is over.
func (c *Client) Rematch() error { return WriteMessage(c.conn, Message{Type: TypeRematch}) }

// Close hangs up.
func (c *Client) Close() error { return c.conn.Close() }
