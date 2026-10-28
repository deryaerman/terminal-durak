package netplay

import (
	"net"
	"strconv"
)

func itoa(i int) string { return strconv.Itoa(i) }

// LocalIPs lists this machine's IPv4 addresses on the local network, the
// ones other players on the same Wi-Fi can connect to.
func LocalIPs() []string {
	var ips []string
	addrs, _ := net.InterfaceAddrs()
	for _, a := range addrs {
		if ipn, ok := a.(*net.IPNet); ok && ipn.IP.IsPrivate() && ipn.IP.To4() != nil {
			ips = append(ips, ipn.IP.String())
		}
	}
	return ips
}

// DefaultPort is the TCP port games are hosted on unless told otherwise.
const DefaultPort = 4747
