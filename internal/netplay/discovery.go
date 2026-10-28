package netplay

import (
	"context"
	"encoding/json"
	"net"
	"strconv"
	"time"
)

// DiscoveryPort is the UDP port hosts announce themselves on.
const DiscoveryPort = 47777

// Announcement is what a host broadcasts: "a Durak game, hosted by Name,
// is waiting on Port".
type Announcement struct {
	Game string `json:"game"` // always "durak", so we ignore other programs' packets
	Name string `json:"name"`
	Port int    `json:"port"`
}

// Host is a game found on the network.
type Host struct {
	Name string
	Addr string // "192.168.1.23:4747", ready to dial
}

// ─────────────────────────────────────────────────────────────────────────
// 🧑‍💻 YOUR PART — finding games on the local network
//
// How does a joining player find a host without typing its IP address?
// The host SHOUTS into the room, and the joiner LISTENS.
//
//   - TCP (what the game itself uses) is a phone call: a connection to ONE
//     known address, reliable, everything arrives in order.
//   - UDP is shouting: single packets ("datagrams"), no connection, may get
//     lost, but they can be sent to a BROADCAST address, 255.255.255.255,
//     which every device on the local network receives.
//
// So the host sends a small JSON packet to 255.255.255.255:47777 every
// second, and the joiner listens on port 47777. When a packet arrives, the
// network stack also tells us WHO sent it, which gives us the host's IP.
// The packet itself says which TCP port the game is on.
//
// Losing a packet doesn't matter: another one comes a second later. That's
// why UDP is fine here, but not for the moves themselves.
//
// The other idea here is context.Context, Go's standard way to say "stop
// now": the caller cancels the context, ctx.Done() becomes readable, and
// loops watching it end. It's how a goroutine gets told to finish.
// ─────────────────────────────────────────────────────────────────────────

// Announce broadcasts "a game is waiting here" every second until ctx is
// cancelled.
func Announce(ctx context.Context, name string, port int) error {
	conn, err := net.Dial("udp4", "255.255.255.255:"+strconv.Itoa(DiscoveryPort))
	if err != nil {
		return err
	}
	defer conn.Close() // runs when the function returns, whichever way

	packet, err := json.Marshal(Announcement{Game: "durak", Name: name, Port: port})
	if err != nil {
		return err
	}

	ticker := time.NewTicker(time.Second) // delivers a tick on ticker.C every second
	defer ticker.Stop()
	for {
		// Shout once: write packet to conn. A UDP "connection" made with
		// net.Dial has a Write method, just like the TCP one. A failed send
		// can be ignored: we try again in a second anyway.
		// TODO(you)
		conn.Write(packet)

		// Then wait for whichever comes first: the next tick, or being told
		// to stop. `select` waits on several channels at once and runs the
		// case that's ready first.
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			// Nothing to do here: the loop goes round and shouts again.
			// 🤔 To think about: why is this case needed at all? (What would
			// happen to the loop if select only waited for ctx.Done()?)
		}
	}
}

// Discover listens for announcements until ctx is cancelled, and sends every
// host it hears about on the returned channel (once per host). The channel
// is closed when listening stops.
func Discover(ctx context.Context) (<-chan Host, error) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{Port: DiscoveryPort})
	if err != nil {
		return nil, err
	}
	found := make(chan Host)

	// Stop listening when the context is cancelled: closing the connection
	// makes the blocked ReadFromUDP below return an error, which ends the loop.
	go func() {
		<-ctx.Done()
		conn.Close()
	}()

	// The listening loop runs in its own goroutine, so Discover can return
	// the channel straight away and the caller reads hosts as they come in.
	go func() {
		defer close(found)
		seen := map[string]bool{}
		buf := make([]byte, 1024)
		for {
			// Wait for a packet. n is its length; from is the sender's address.
			n, from, err := conn.ReadFromUDP(buf)
			if err != nil {
				return // closed: we're done
			}
			var a Announcement
			if json.Unmarshal(buf[:n], &a) != nil || a.Game != "durak" {
				continue // not one of ours
			}

			// Combine the two halves of the address we'll dial:
			//   - the host's IP comes from the packet's ENVELOPE: from.IP
			//     (call .String() on it to get text like "192.168.1.23")
			//   - the TCP port comes from the packet's CONTENTS: a.Port
			//     (an int; strconv.Itoa turns it into text)
			// net.JoinHostPort(host, port) glues them into "192.168.1.23:4747".
			// TODO(you): replace "" with the address.
			addr := from.IP.String() + ":" + strconv.Itoa(a.Port)
			_ = from

			if seen[addr] {
				continue // already reported; hosts repeat themselves every second
			}
			seen[addr] = true

			// Hand it to whoever is reading the channel, unless we've been
			// told to stop in the meantime.
			select {
			case found <- Host{Name: a.Name, Addr: addr}:
			case <-ctx.Done():
				return
			}
		}
	}()
	return found, nil
}
