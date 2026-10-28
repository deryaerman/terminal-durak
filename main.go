// Command durak runs the Durak card game in the terminal.
//
//	durak                  play the computer (host or join from the menu, m)
//	durak host [flags]     host a game on the local network
//	durak join [address]   join one (without an address: find games automatically)
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/user"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"durak/internal/game"
	"durak/internal/netplay"
	"durak/internal/ui"
)

const defaultPort = netplay.DefaultPort

func main() {
	var err error
	switch {
	case len(os.Args) > 1 && os.Args[1] == "host":
		err = host(os.Args[2:])
	case len(os.Args) > 1 && os.Args[1] == "join":
		err = join(os.Args[2:])
	case len(os.Args) > 1:
		err = fmt.Errorf("unknown command %q; use: durak, durak host, durak join", os.Args[1])
	default:
		_, err = tea.NewProgram(ui.NewApp(defaultName())).Run()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// host starts a server, announces it on the network, and joins it ourselves.
func host(args []string) error {
	fs := flag.NewFlagSet("host", flag.ExitOnError)
	name := fs.String("name", defaultName(), "your name, shown to the other player")
	port := fs.Int("port", defaultPort, "TCP port to listen on")
	timer := fs.Int("timer", 30, "seconds per turn (0 = no timer)")
	passOn := fs.Bool("pass-on", true, "allow passing the attack on")
	passMany := fs.Bool("pass-many", false, "allow passing on with several cards")
	throwIn := fs.Bool("throw-in", true, "allow throwing in after the defender takes")
	fs.Parse(args)

	rules := game.DefaultRules
	rules.PassOn, rules.PassOnMany, rules.ThrowInAfterTake = *passOn, *passMany, *throwIn

	srv, err := netplay.Listen(":"+strconv.Itoa(*port), rules, time.Duration(*timer)*time.Second)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.Run(ctx)
	go netplay.Announce(ctx, *name, srv.Port())

	c, err := netplay.Dial("127.0.0.1:"+strconv.Itoa(srv.Port()), *name)
	if err != nil {
		return err
	}
	lobby := ui.HostLobbyText(*name, srv.Port())
	_, err = tea.NewProgram(ui.NewRemote(c, lobby)).Run()
	return err
}

// join connects to a host, found automatically if no address is given.
func join(args []string) error {
	fs := flag.NewFlagSet("join", flag.ExitOnError)
	name := fs.String("name", defaultName(), "your name, shown to the other player")
	fs.Parse(args)

	addr := fs.Arg(0)
	if addr == "" {
		var err error
		if addr, err = findGame(); err != nil {
			return err
		}
	} else if _, _, err := net.SplitHostPort(addr); err != nil {
		addr = net.JoinHostPort(addr, strconv.Itoa(defaultPort)) // no port given
	}

	c, err := netplay.Dial(addr, *name)
	if err != nil {
		return fmt.Errorf("can't connect to %s: %w", addr, err)
	}
	_, err = tea.NewProgram(ui.NewRemote(c, "Joining "+addr+"…")).Run()
	return err
}

// findGame listens for hosts for a few seconds and lets you pick one.
func findGame() (string, error) {
	fmt.Println("Looking for games on this network…")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	found, err := netplay.Discover(ctx)
	if err != nil {
		return "", err
	}
	var hosts []netplay.Host
	for h := range found {
		hosts = append(hosts, h)
		fmt.Printf("  %d) %s  (%s)\n", len(hosts), h.Name, h.Addr)
	}
	switch len(hosts) {
	case 0:
		return "", fmt.Errorf("no games found; ask the host for their address and run: durak join <address>")
	case 1:
		return hosts[0].Addr, nil
	}
	fmt.Print("Which game? ")
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	n, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil || n < 1 || n > len(hosts) {
		return "", fmt.Errorf("pick a number from 1 to %d", len(hosts))
	}
	return hosts[n-1].Addr, nil
}

// defaultName is your user name, capitalised.
func defaultName() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return strings.ToUpper(u.Username[:1]) + u.Username[1:]
	}
	return "Player"
}
