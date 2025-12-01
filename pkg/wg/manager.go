package wg

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"runtime"
	"sync"
)

// Manager manages a simple in-memory IP allocator for WireGuard peers
// and can optionally apply configuration using the `wg` CLI on Linux.
type Manager struct {
	iface   string   // e.g. "wg0"
	network *net.IPNet
	server  net.IP   // server IP inside the WG network
	nextIP  net.IP   // next client IP to hand out (/32)

	apply bool // whether to actually call `wg set ...`
	mu    sync.Mutex
}

// NewManagerFromEnv initializes a Manager based on environment variables.
//
//   MEERKAT_NODE_WG_INTERFACE  (default "wg0")
//   MEERKAT_NODE_WG_NETWORK    (default "10.8.0.1/24")
//   MEERKAT_NODE_WG_APPLY      ("1" to actually run `wg`, otherwise log-only)
//
// The .1 address is treated as the server IP and the first client
// will be .2, then .3, etc.
func NewManagerFromEnv() (*Manager, error) {
	iface := os.Getenv("MEERKAT_NODE_WG_INTERFACE")
	if iface == "" {
		iface = "wg0"
	}

	// CIDR for server + clients
	cidr := os.Getenv("MEERKAT_NODE_WG_NETWORK")
	if cidr == "" {
		cidr = "10.8.0.1/24"
	}

	ip, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, fmt.Errorf("parse MEERKAT_NODE_WG_NETWORK: %w", err)
	}
	ip4 := ip.To4()
	if ip4 == nil {
		return nil, fmt.Errorf("MEERKAT_NODE_WG_NETWORK must be IPv4")
	}

	// server IP = ip4 (e.g. 10.8.0.1)
	serverIP := make(net.IP, len(ip4))
	copy(serverIP, ip4)

	// nextIP = serverIP + 1 (e.g. 10.8.0.2)
	next := make(net.IP, len(ip4))
	copy(next, ip4)
	incrementIP(next)

	apply := os.Getenv("MEERKAT_NODE_WG_APPLY") == "1"

	m := &Manager{
		iface:   iface,
		network: ipNet,
		server:  serverIP,
		nextIP:  next,
		apply:   apply,
	}

	mode := "log-only"
	if apply {
		if runtime.GOOS == "linux" {
			mode = "apply (wg CLI)"
		} else {
			mode = "apply requested, but OS is not linux -> log-only"
			m.apply = false
		}
	}

	log.Printf("[wg] manager initialized: iface=%s network=%s server=%s mode=%s\n",
		iface, ipNet.String(), serverIP.String(), mode)

	return m, nil
}

// AllocatePeer assigns a /32 IP to the given client WG public key and
// logs the WireGuard command you would run on your Linux server.
func (m *Manager) AllocatePeer(clientPub string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ip := make(net.IP, len(m.nextIP))
	copy(ip, m.nextIP)

	// Ensure it stays in the same network
	if !m.network.Contains(ip) {
		return "", fmt.Errorf("address pool exhausted")
	}

	// Advance nextIP
	incrementIP(m.nextIP)

	ipStr := ip.String() + "/32"

	log.Printf("[wg] allocating peer IP %s for client pubkey %s\n", ipStr, clientPub)
	log.Printf("[wg] To configure on a Linux box manually, you can run:\n"+
		"  wg set %s peer %s allowed-ips %s\n",
		m.iface, clientPub, ipStr)

	return ipStr, nil
}

// ApplyPeer, if enabled, will run `wg set <iface> peer <clientPub> allowed-ips <clientIP>`
// on Linux. If apply mode is off, this is a no-op that just logs.
func (m *Manager) ApplyPeer(clientPub, clientIP string) error {
	if !m.apply {
		// log-only mode
		log.Printf("[wg] apply disabled; not running wg for peer %s (%s)\n", clientPub, clientIP)
		return nil
	}

	// We only allow this on Linux, and we already enforce that in NewManagerFromEnv,
	// but double-check here just in case.
	if runtime.GOOS != "linux" {
		log.Printf("[wg] apply requested but OS=%s; skipping wg for peer %s\n", runtime.GOOS, clientPub)
		return nil
	}

	args := []string{"set", m.iface, "peer", clientPub, "allowed-ips", clientIP}
	log.Printf("[wg] applying peer via: wg %v\n", args)

	cmd := exec.Command("wg", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running wg set: %w", err)
	}

	log.Printf("[wg] successfully applied WireGuard peer %s (%s)\n", clientPub, clientIP)
	return nil
}

// incrementIP increments an IPv4 address in-place (last byte rolls over).
func incrementIP(ip net.IP) {
	for i := len(ip) - 1; i >= 0; i-- {
		ip[i]++
		if ip[i] != 0 {
			break
		}
	}
}
