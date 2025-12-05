package discovery

import (
	"log"
	"net"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// HealthInfo tracks runtime health/latency for a node.
type HealthInfo struct {
	LatencyMs   int
	Healthy     bool
	LastChecked time.Time
	LastError   string
}

var (
	healthMu   sync.RWMutex
	healthByID = map[string]HealthInfo{}
)

// getHealth returns the health info (if any) for a node ID.
func getHealth(id string) (HealthInfo, bool) {
	healthMu.RLock()
	defer healthMu.RUnlock()
	h, ok := healthByID[id]
	return h, ok
}

// setHealth updates health info for a node ID.
func setHealth(id string, h HealthInfo) {
	healthMu.Lock()
	defer healthMu.Unlock()
	healthByID[id] = h
}

// StartBackgroundHealthProbe launches a goroutine that periodically
// probes all staticNodes and records health/latency. Safe to call
// multiple times; the first call wins.
var healthProbeOnce sync.Once

func StartBackgroundHealthProbe(interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Second
	}

	healthProbeOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(interval)
			defer ticker.Stop()

			for {
				probeAllNodesOnce()
				<-ticker.C
			}
		}()
	})
}

// probeAllNodesOnce probes each statically-known node once.
func probeAllNodesOnce() {
	for _, n := range staticNodes {
		// Only probe nodes that are "enabled" statically.
		if !n.Healthy {
			continue
		}
		probeNode(n)
	}
}

// probeNode does a simple TCP dial to the node's APIURL host:port
// and records latency + success/failure.
func probeNode(n NodeInfo) {
	hostPort, err := hostPortFromAPIURL(n.APIURL)
	if err != nil {
		setHealth(n.ID, HealthInfo{
			Healthy:     false,
			LastChecked: time.Now(),
			LastError:   "parse api url: " + err.Error(),
		})
		return
	}

	start := time.Now()
	conn, err := net.DialTimeout("tcp", hostPort, 3*time.Second)
	latency := time.Since(start)

	h := HealthInfo{
		LatencyMs:   int(latency.Milliseconds()),
		LastChecked: time.Now(),
	}

	if err != nil {
		h.Healthy = false
		h.LastError = err.Error()
	} else {
		h.Healthy = true
		_ = conn.Close()
	}

	setHealth(n.ID, h)

	if debug := (strings.ToLower(strings.TrimSpace(
		getenvDefault("MEERKAT_DEBUG_DISCOVERY", ""))) == "1"); debug {
		log.Printf("[discovery] probe %s (%s): healthy=%v latency=%dms err=%v\n",
			n.ID, hostPort, h.Healthy, h.LatencyMs, err)
	}
}

func hostPortFromAPIURL(api string) (string, error) {
	u, err := url.Parse(api)
	if err != nil {
		return "", err
	}
	host := u.Host
	if !strings.Contains(host, ":") {
		// default to 80 or 443 based on scheme
		if u.Scheme == "https" {
			host += ":443"
		} else {
			host += ":80"
		}
	}
	return host, nil
}

// rankByLatency returns a new slice sorted by:
//   1) dynamically healthy over unhealthy
//   2) lower latency first
//   3) original order as tie-breaker
//
// If no health info is available yet, it preserves the original order.
func rankByLatency(nodes []NodeInfo) []NodeInfo {
	out := make([]NodeInfo, len(nodes))
	copy(out, nodes)

	healthMu.RLock()
	defer healthMu.RUnlock()

	type nodeWithIndex struct {
		N   NodeInfo
		Idx int
		H   HealthInfo
		OK  bool
	}

	wrapped := make([]nodeWithIndex, len(out))
	for i, n := range out {
		h, ok := healthByID[n.ID]
		wrapped[i] = nodeWithIndex{
			N:   n,
			Idx: i,
			H:   h,
			OK:  ok,
		}
	}

	sort.SliceStable(wrapped, func(i, j int) bool {
		a := wrapped[i]
		b := wrapped[j]

		// If either has no health data, keep original relative order.
		if !a.OK && !b.OK {
			return a.Idx < b.Idx
		}
		if a.OK && !b.OK {
			return true
		}
		if !a.OK && b.OK {
			return false
		}

		// Both have health: prefer healthy over unhealthy.
		if a.H.Healthy && !b.H.Healthy {
			return true
		}
		if !a.H.Healthy && b.H.Healthy {
			return false
		}

		// Both healthy or both unhealthy: lower latency first.
		if a.H.LatencyMs != b.H.LatencyMs {
			return a.H.LatencyMs < b.H.LatencyMs
		}

		// Stable fallback: original order
		return a.Idx < b.Idx
	})

	for i, w := range wrapped {
		out[i] = w.N
	}
	return out
}

// small helper so we can reuse this without importing os here/there
func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
