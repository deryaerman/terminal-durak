// Package netplay lets two people play over the local network: one runs the
// game server (server.go), both connect to it as clients (client.go), and
// they find each other through discovery.go. This file is the protocol: what
// a message looks like and how it travels over a connection.
package netplay

import (
	"bufio"
	"encoding/json"
	"io"

	"durak/internal/game"
)

// Message types.
const (
	TypeHello   = "hello"   // client → server: "I'm here", with a name
	TypeWelcome = "welcome" // server → client: "you're seated, wait for the game"
	TypeView    = "view"    // server → client: your view of the game
	TypeMove    = "move"    // client → server: a move
	TypeRematch = "rematch" // client → server: start a new game
	TypeError   = "error"   // server → client: your move was refused, or the game is full
	TypeLeft    = "left"    // server → client: the other player disconnected
)

// Message is everything that travels between server and client. Only the
// fields that matter for its Type are filled in; the rest are left out of
// the JSON thanks to "omitempty".
//
// The `json:"..."` notes after each field are struct tags: they tell the
// encoding/json package what to call the field in the JSON text.
type Message struct {
	Type  string     `json:"type"`
	Name  string     `json:"name,omitempty"`
	View  *game.View `json:"view,omitempty"`
	Move  *game.Move `json:"move,omitempty"`
	Error string     `json:"error,omitempty"`
}

// ─────────────────────────────────────────────────────────────────────────
// 🧑‍💻 YOUR PART — sending and receiving messages
//
// A TCP connection is just a stream of bytes, like a pipe: it has no idea
// where one message ends and the next begins. So we need a FRAMING rule.
// Ours is the simplest one there is, often called "JSON lines":
//
//	every message is one line of JSON text, ended by a newline '\n'.
//
//	{"type":"move","move":{"Kind":0,"Card":{"Rank":12,"Suit":0}}}\n
//	{"type":"view","view":{...}}\n
//
// JSON itself never contains a raw newline (newlines inside strings are
// written as \n escapes), so a newline can only mean "message over".
//
// Two Go ideas to notice:
//   - io.Writer is an INTERFACE: anything with a Write method. A network
//     connection is one, but so is a file or a bytes.Buffer, which is what
//     the tests use. WriteMessage works with all of them.
//   - bufio.Reader wraps a connection and adds buffering plus handy methods
//     like ReadBytes, which reads until a given byte.
//
// Run `go test ./internal/netplay -run Message -v` to check your work.
// ─────────────────────────────────────────────────────────────────────────

// Keeps the compiler happy until your code uses the json package.
var _ = json.Marshal

// WriteMessage sends m as one line of JSON.
func WriteMessage(w io.Writer, m Message) error {
	var data []byte
	var err error

	// Step 1: turn the struct into JSON bytes.
	//   json.Marshal(m) returns ([]byte, error), the usual Go "result, error"
	//   pair. Assign both to data and err (use = here, not :=, because they
	//   are already declared above), then return err if it isn't nil.
	data, err = json.Marshal(m)
	if err != nil {
		return err
	}

	// Step 2: add the newline that marks the end of the message.
	//   append(data, '\n') returns the slice with one more byte on the end.
	//   ('\n' in single quotes is one character; "\n" would be a string.)
	data = append(data, '\n')

	// Step 3: write it out in one go, so two messages can never interleave.
	_, err = w.Write(data)
	return err
}

// ReadMessage reads the next line and decodes it. It returns io.EOF when the
// other side has closed the connection.
func ReadMessage(r *bufio.Reader) (Message, error) {
	var m Message

	var line []byte
	var err error

	// Step 1: read everything up to and including the next newline.
	//   r.ReadBytes(delim) reads until it meets the byte delim and returns
	//   ([]byte, error). Which byte ends our messages? Return m, err if
	//   reading failed: at the end of the connection err is io.EOF, and the
	//   caller uses that to notice the other side hung up.
	line, err = r.ReadBytes('\n')
	if err != nil {
		return m, err
	}

	// Step 2: decode the JSON in line into m.
	//   json.Unmarshal(data, target) FILLS IN an existing value, so instead of
	//   m itself it needs to know where m lives in memory: its address, &m
	//   (a pointer). That's the same reason Deck.Draw has a *Deck receiver.
	err = json.Unmarshal(line, &m)
	_ = line
	return m, err
}
