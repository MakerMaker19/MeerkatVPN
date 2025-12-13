package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/MakerMaker19/meerkatvpn/pkg/client"
	"github.com/MakerMaker19/meerkatvpn/pkg/discovery"
	"github.com/MakerMaker19/meerkatvpn/pkg/nostrutil"
	"github.com/MakerMaker19/meerkatvpn/pkg/vpn"
)

func init() {
	// Start background node health probing every 30s.
	// Safe to call multiple times; discovery package guards it.
	discovery.StartBackgroundHealthProbe(30 * time.Second)
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]

	switch cmd {
	case "receive-tokens":
		if err := cmdReceiveTokens(); err != nil {
			log.Fatal(err)
		}
	case "list-tokens":
		if err := cmdListTokens(); err != nil {
			log.Fatal(err)
		}
	case "list-nodes":
		if err := cmdListNodes(); err != nil {
			log.Fatal(err)
		}
	case "subscribe":
		if err := cmdSubscribe(); err != nil {
			log.Fatal(err)
		}
	case "connect":
		if err := cmdConnect(); err != nil {
			log.Fatal(err)
		}
	case "watch-nodes":
		if err := cmdWatchNodes(); err != nil {
			log.Fatal(err)
		}
	default:
		fmt.Println("unknown command:", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("MeerkatVPN client CLI")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  meerkat-client receive-tokens   # connect to Nostr relays and store subscription tokens")
	fmt.Println("  meerkat-client list-tokens      # list stored subscription tokens")
	fmt.Println("  meerkat-client subscribe        # request a subscription token (dev/demo) and wait for it")
	fmt.Println("  meerkat-client list-nodes       # list known Meerkat nodes via discovery")
	fmt.Println("  meerkat-client connect          # use latest valid token to request a session from a node")
}

func cmdReceiveTokens() error {
	ctx := context.Background()
	return client.ListenForTokens(ctx)
}

func cmdListTokens() error {
	ts, err := client.LoadTokenStore()
	if err != nil {
		return fmt.Errorf("load token store: %w", err)
	}
	if len(ts.Tokens) == 0 {
		fmt.Println("No stored subscription tokens.")
		return nil
	}

	fmt.Println("Stored subscription tokens:")
	for _, t := range ts.Tokens {
		exp := time.Unix(t.Payload.ExpiresAt, 0).Local()
		fmt.Printf("- %s | plan=%s | expires=%s | issuer=%s\n",
			t.Payload.TokenID,
			t.Payload.SubscriptionType,
			exp.Format(time.RFC3339),
			t.Payload.IssuerPubKey,
		)
	}
	return nil
}

func cmdListNodes() error {
	ctx := context.Background()

	nodes, err := discovery.ListNodes(ctx)
	if err != nil {
		return fmt.Errorf("list nodes: %w", err)
	}

	if len(nodes) == 0 {
		fmt.Println("No nodes known via discovery.")
		return nil
	}

	fmt.Println("Known Meerkat nodes (via discovery):")
	for _, n := range nodes {
		backends := strings.Join(n.Backends, ",")
		if backends == "" {
			backends = "(none)"
		}
		fmt.Printf(
			"- id=%s | api=%s | region=%s | country=%s | city=%s | backends=%s | healthy=%v\n",
			n.ID, n.APIURL, n.Region, n.Country, n.City, backends, n.Healthy,
		)
	}

	return nil
}

func promptBackend() string {
	// If env var is set, respect it and don't prompt.
	if env := os.Getenv("MEERKAT_TUNNEL_BACKEND"); env != "" {
		log.Printf("MEERKAT_TUNNEL_BACKEND=%s (no prompt)\n", env)
		return strings.ToLower(env)
	}

	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Select tunnel backend:")
	fmt.Println("  1) OpenVPN   (stable, working now)")
	fmt.Println("  2) WireGuard (experimental; server config must be correct)")
	fmt.Print("Enter choice [1]: ")

	line, _ := reader.ReadString('\n')
	choice := strings.TrimSpace(strings.ToLower(line))

	switch choice {
	case "", "1", "openvpn", "ovpn":
		return "openvpn"
	case "2", "wireguard", "wg":
		return "wireguard"
	default:
		fmt.Println("Unrecognized choice, defaulting to OpenVPN.")
		return "openvpn"
	}
}

// writeOpenVPNProfile writes the given profile bytes directly into the OpenVPN
// config directory (or explicit profile path), overwriting any existing file.
// It never writes into the current working directory.
func writeOpenVPNProfile(data []byte) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("empty OpenVPN profile")
	}

	// Highest priority: explicit full profile path (for power users)
	if profilePath := os.Getenv("MEERKAT_OPENVPN_PROFILE_PATH"); profilePath != "" {
		if err := os.MkdirAll(filepath.Dir(profilePath), 0o755); err != nil {
			return "", fmt.Errorf("ensure dir for %s: %w", profilePath, err)
		}
		if err := os.WriteFile(profilePath, data, 0o600); err != nil {
			return "", fmt.Errorf("write %s: %w", profilePath, err)
		}
		return profilePath, nil
	}

	// Detect base config directory
	destDir := os.Getenv("MEERKAT_OPENVPN_CONFIG_DIR")

	if destDir == "" && runtime.GOOS == "windows" {
		if home, err := os.UserHomeDir(); err == nil {
			candidate := filepath.Join(home, "OpenVPN", "config")
			if st, err2 := os.Stat(candidate); err2 == nil && st.IsDir() {
				destDir = candidate
			}
		}
	}

	if destDir == "" && runtime.GOOS == "windows" {
		if pf := os.Getenv("ProgramFiles"); pf != "" {
			candidate := filepath.Join(pf, "OpenVPN", "config")
			if st, err2 := os.Stat(candidate); err2 == nil && st.IsDir() {
				destDir = candidate
			}
		}
	}

	if destDir == "" {
		return "", fmt.Errorf("OpenVPN config dir not found; set MEERKAT_OPENVPN_PROFILE_PATH or MEERKAT_OPENVPN_CONFIG_DIR")
	}

	// Search recursively for any existing "meerkat.ovpn"
	var destPath string
	_ = filepath.WalkDir(destDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if strings.EqualFold(d.Name(), "meerkat.ovpn") {
			destPath = path
		}
		return nil
	})

	// If we found an existing meerkat.ovpn anywhere under config/, overwrite it.
	if destPath == "" {
		// No existing file? Create one in the root config dir.
		destPath = filepath.Join(destDir, "meerkat.ovpn")
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return "", fmt.Errorf("ensure dir for %s: %w", destPath, err)
	}

	if err := os.WriteFile(destPath, data, 0o600); err != nil {
		return "", fmt.Errorf("write %s: %w", destPath, err)
	}

	return destPath, nil
}

// cmdSubscribe calls the pool's dev invoice endpoint and waits for a token DM.
// Env:
//
//	MEERKAT_POOL_API_URL           (e.g., http://localhost:8080)
//	MEERKAT_SUBSCRIBE_PLAN         (optional; default "monthly")
//	MEERKAT_WAIT_FOR_TOKEN_SECS    (optional; default used from waitForTokenSeconds)
func cmdSubscribe() error {
	ctx := context.Background()

	plan := os.Getenv("MEERKAT_SUBSCRIBE_PLAN")
	if plan == "" {
		plan = "monthly"
	}

	poolURL := os.Getenv("MEERKAT_POOL_API_URL")
	if poolURL == "" {
		return fmt.Errorf("MEERKAT_POOL_API_URL not set (e.g., http://localhost:8080)")
	}

	// Load client privkey (env -> config) and derive pubkey.
	privRaw := os.Getenv("MEERKAT_CLIENT_NOSTR_PRIVKEY")
	if privRaw == "" {
		if cfg := client.Config(); cfg != nil && cfg.NostrPrivKey != "" {
			privRaw = cfg.NostrPrivKey
		}
	}
	if privRaw == "" {
		return fmt.Errorf("MEERKAT_CLIENT_NOSTR_PRIVKEY not set and no nostr_privkey in config")
	}

	parsed, err := nostrutil.ParsePrivKey(privRaw)
	if err != nil {
		return fmt.Errorf("parse client privkey: %w", err)
	}

	body := struct {
		NostrPubKey string `json:"nostr_pubkey"`
		Plan        string `json:"plan"`
	}{
		NostrPubKey: parsed.PubHex,
		Plan:        plan,
	}

	b, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	url := strings.TrimRight(poolURL, "/") + "/invoice"
	log.Printf("Requesting %s plan from pool at %s\n", plan, url)

	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("POST /invoice: %w", err)
	}
	defer resp.Body.Close()

	var invoiceResp struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		TokenID string `json:"token_id"`
		Plan    string `json:"plan"`
		Invoice string `json:"invoice"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&invoiceResp)
	log.Printf("Invoice response: status=%s message=%s token_id=%s plan=%s\n", invoiceResp.Status, invoiceResp.Message, invoiceResp.TokenID, invoiceResp.Plan)

	waitSecs := waitForTokenSeconds()
	if waitSecs <= 0 {
		waitSecs = 30
	}
	log.Printf("Waiting up to %ds for subscription token DM...\n", waitSecs)

	ctxWait, cancel := context.WithTimeout(ctx, time.Duration(waitSecs)*time.Second)
	defer cancel()
	_ = client.ListenForTokens(ctxWait)

	ts, err := client.LoadTokenStore()
	if err != nil {
		return fmt.Errorf("load token store after subscribe: %w", err)
	}
	if _, err := ts.LatestValid("", time.Now()); err != nil {
		return fmt.Errorf("no valid token received (plan=%s): %w", plan, err)
	}

	fmt.Println("Subscription token received and stored.")
	return nil
}

// chooseNodeInteractively lists eligible nodes (backend support, non-expired)
// ranked by health/latency and region match, then prompts the user to pick one.
func chooseNodeInteractively(ctx context.Context, poolPubKey string, preferredRegion string, backend string) (*discovery.NodeInfo, error) {
	var eligible []discovery.NodeInfo

	// Give discovery a short window to ingest announcements before failing.
	deadline := time.Now().Add(10 * time.Second)
	for {
		all, err := discovery.ListNodes(ctx)
		if err != nil {
			return nil, fmt.Errorf("list nodes: %w", err)
		}

		eligible = filterNodesForBackend(all, backend, poolPubKey)
		if len(eligible) > 0 {
			break
		}

		if time.Now().After(deadline) {
			return nil, fmt.Errorf("no suitable node found via discovery for backend=%s", backend)
		}
		time.Sleep(500 * time.Millisecond)
	}

	ranked := rankNodes(eligible, preferredRegion)

	fmt.Println("Discovered nodes:")
	for i, n := range ranked {
		health := "unknown"
		if n.Healthy {
			health = "healthy"
		} else if !n.LastHealthCheck.IsZero() {
			health = "unhealthy"
		}

		lat := "-"
		if n.LastLatency > 0 {
			lat = n.LastLatency.String()
		}

		region := n.Region
		if region == "" {
			region = "(none)"
		}

		fmt.Printf("  [%d] id=%s | region=%s | api=%s | backends=%s | health=%s | latency=%s\n",
			i+1,
			n.ID,
			region,
			n.APIURL,
			strings.Join(n.Backends, ","),
			health,
			lat,
		)
	}

	fmt.Print("Select node [1]: ")
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	choice := strings.TrimSpace(line)
	if choice == "" {
		return &ranked[0], nil
	}

	idx, err := strconv.Atoi(choice)
	if err != nil || idx < 1 || idx > len(ranked) {
		return nil, fmt.Errorf("invalid selection")
	}

	return &ranked[idx-1], nil
}

// filterNodesForBackend keeps nodes that support the backend, are not expired,
// and are either healthy or not yet probed. If PoolPubKey is set on a node,
// it must match the requested pool (when provided).
func filterNodesForBackend(nodes []discovery.NodeInfo, backend string, poolPubKey string) []discovery.NodeInfo {
	backend = strings.ToLower(strings.TrimSpace(backend))
	poolPubKey = strings.TrimSpace(poolPubKey)

	var out []discovery.NodeInfo
	for _, n := range nodes {
		if poolPubKey != "" && n.PoolPubKey != "" && !strings.EqualFold(n.PoolPubKey, poolPubKey) {
			continue
		}

		// Keep nodes that are healthy or not yet probed; drop explicit unhealthy.
		if !n.Healthy && !n.LastHealthCheck.IsZero() {
			continue
		}

		if backend == "" {
			out = append(out, n)
			continue
		}

		for _, b := range n.Backends {
			if strings.EqualFold(b, backend) {
				out = append(out, n)
				break
			}
		}
	}
	return out
}

// rankNodes sorts nodes by (1) region match, (2) healthy over unknown,
// (3) lower latency, (4) original order.
func rankNodes(nodes []discovery.NodeInfo, preferredRegion string) []discovery.NodeInfo {
	out := make([]discovery.NodeInfo, len(nodes))
	copy(out, nodes)

	preferredRegion = strings.ToLower(strings.TrimSpace(preferredRegion))

	type withIndex struct {
		Node discovery.NodeInfo
		Idx  int
	}

	wrapped := make([]withIndex, len(out))
	for i, n := range out {
		wrapped[i] = withIndex{Node: n, Idx: i}
	}

	sort.SliceStable(wrapped, func(i, j int) bool {
		a := wrapped[i].Node
		b := wrapped[j].Node

		regionMatchA := preferredRegion != "" && preferredRegion != "auto" && strings.EqualFold(a.Region, preferredRegion)
		regionMatchB := preferredRegion != "" && preferredRegion != "auto" && strings.EqualFold(b.Region, preferredRegion)
		if regionMatchA != regionMatchB {
			return regionMatchA
		}

		healthScore := func(n discovery.NodeInfo) int {
			switch {
			case n.Healthy:
				return 2
			case n.LastHealthCheck.IsZero():
				return 1 // unknown
			default:
				return 0
			}
		}

		ha := healthScore(a)
		hb := healthScore(b)
		if ha != hb {
			return ha > hb
		}

		latVal := func(n discovery.NodeInfo) time.Duration {
			if n.LastLatency > 0 {
				return n.LastLatency
			}
			return time.Duration(math.MaxInt64)
		}

		la := latVal(a)
		lb := latVal(b)
		if la != lb {
			return la < lb
		}

		return wrapped[i].Idx < wrapped[j].Idx
	})

	for i, w := range wrapped {
		out[i] = w.Node
	}
	return out
}

// cmdConnect:
// - Uses the subscription token flow exactly as before
// - Calls POST /session/create with {token, client_wg_pubkey, backend}
// - If backend == "wireguard": builds and writes a WG config (existing behavior)
// - If backend == "openvpn": expects ovpn_profile in the response and writes meerkat.ovpn
func cmdConnect() error {
	ctx := context.Background()

	// Ensure discovery is wired (Nostr relays + static) before selecting a node.
	configureFinderFromEnv()

	// Backend selection (OpenVPN vs WireGuard)
	backend := promptBackend()
	log.Printf("Using backend=%s\n", backend)

	// Pool pubkey: env -> config -> error.
	poolRaw := os.Getenv("MEERKAT_CLIENT_POOL_PUBKEY")
	if poolRaw == "" {
		if cfg := client.Config(); cfg != nil && cfg.PoolPubKey != "" {
			poolRaw = cfg.PoolPubKey
		}
	}
	if poolRaw == "" {
		return fmt.Errorf("MEERKAT_CLIENT_POOL_PUBKEY not set and no pool_pubkey in meerkat-client.yaml")
	}

	poolPub, err := nostrutil.ParsePubKey(poolRaw)
	if err != nil {
		return fmt.Errorf("failed to parse pool pubkey (%q): %w", poolRaw, err)
	}

	// Node selection
	nodeURL := os.Getenv("MEERKAT_NODE_URL")
	if nodeURL != "" {
		log.Printf("Using node URL from MEERKAT_NODE_URL=%s\n", nodeURL)
	} else {
		// Region: env -> config -> "auto".
		region := os.Getenv("MEERKAT_PREFERRED_REGION")
		if region == "" {
			if cfg := client.Config(); cfg != nil {
				region = cfg.PreferredRegion
			}
		}
		if region == "" {
			region = "auto"
		}

		node, err := chooseNodeInteractively(ctx, poolPub, region, backend)
		if err != nil {
			return err
		}

		nodeURL = node.APIURL
		log.Printf("Selected node %s (%s) via discovery\n", node.ID, node.APIURL)
	}

	// - load token store
	ts, err := client.LoadTokenStore()
	if err != nil {
		return fmt.Errorf("load token store: %w", err)
	}

	// - pick latest valid token
	tok, err := ts.LatestValid(poolPub, time.Now())
	if err != nil {
		// Optional: wait for a token to arrive over Nostr before failing.
		if waitSecs := waitForTokenSeconds(); waitSecs > 0 {
			log.Printf("No valid tokens; waiting up to %ds for an incoming token...", waitSecs)
			ctxWait, cancel := context.WithTimeout(ctx, time.Duration(waitSecs)*time.Second)
			defer cancel()
			// Block until timeout/cancel; ignore return error and re-check store.
			_ = client.ListenForTokens(ctxWait)

			ts, err = client.LoadTokenStore()
			if err != nil {
				return fmt.Errorf("load token store after wait: %w", err)
			}
			tok, err = ts.LatestValid(poolPub, time.Now())
		}
		if err != nil {
			return fmt.Errorf("no valid tokens: %w", err)
		}
	}

	// Generate WG keypair (still required for WG backend; node can ignore for OpenVPN)
	wgKeys, err := client.GenerateWGKeypair()
	if err != nil {
		return fmt.Errorf("generate WG keypair: %w", err)
	}
	log.Printf("Generated WireGuard public key: %s\n", wgKeys.Public)

	// Build request including backend
	reqBody := struct {
		Token          vpn.SubscriptionToken `json:"token"`
		ClientWGPubKey string                `json:"client_wg_pubkey"`
		Backend        string                `json:"backend"` // "wireguard" or "openvpn"
	}{
		Token:          *tok,
		ClientWGPubKey: wgKeys.Public,
		Backend:        backend,
	}

	b, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	url := nodeURL + "/session/create"
	log.Printf("Connecting to node at %s with token %s (backend=%s)\n", url, tok.Payload.TokenID, backend)

	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("POST /session/create: %w", err)
	}
	defer resp.Body.Close()

	var sr struct {
		Status       string   `json:"status"`
		Message      string   `json:"message"`
		ServerPubKey string   `json:"server_pubkey"`
		Endpoint     string   `json:"endpoint"`
		ClientIP     string   `json:"client_ip"`
		AllowedIPs   string   `json:"allowed_ips"`
		DNS          []string `json:"dns"`

		// New for OpenVPN backend:
		OVPNProfile string `json:"ovpn_profile,omitempty"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		// try to read raw body for debugging
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("decode response: %w (raw body: %s)", err, string(body))
	}

	if resp.StatusCode != http.StatusOK || sr.Status != "ok" {
		return fmt.Errorf("node error: %s (%s)", sr.Status, sr.Message)
	}

	// === Backend-specific handling ===================================

	switch backend {
	case "openvpn":
		if sr.OVPNProfile == "" {
			return fmt.Errorf("node did not provide ovpn_profile for openvpn backend")
		}

		path, err := writeOpenVPNProfile([]byte(sr.OVPNProfile))
		if err != nil {
			return fmt.Errorf("write OpenVPN profile: %w", err)
		}

		fmt.Println("Node accepted session:")
		fmt.Println("  status :", sr.Status)
		fmt.Println("  message:", sr.Message)
		fmt.Println()
		fmt.Println("OpenVPN profile written to:")
		fmt.Println(" ", path)
		fmt.Println()
		fmt.Println("Import this profile into your OpenVPN client and connect.")
		return nil

	case "wireguard":
		fallthrough
	default:
		// Original WireGuard behavior
		if sr.ClientIP == "" {
			return fmt.Errorf("node did not provide client_ip")
		}
		if sr.ServerPubKey == "" {
			return fmt.Errorf("node did not provide server_pubkey")
		}
		if sr.Endpoint == "" {
			return fmt.Errorf("node did not provide endpoint")
		}

		cfg := client.BuildWGConfig(client.WGConfigParams{
			PrivateKey: wgKeys.Private,
			Address:    sr.ClientIP,
			DNS:        sr.DNS,
			ServerPub:  sr.ServerPubKey,
			Endpoint:   sr.Endpoint,
			AllowedIPs: sr.AllowedIPs,
			Keepalive:  25,
		})

		path, err := client.DefaultWGConfigPath()
		if err != nil {
			return fmt.Errorf("determine WG config path: %w", err)
		}

		if err := client.WriteWGConfig(path, cfg); err != nil {
			return fmt.Errorf("write WG config: %w", err)
		}

		fmt.Println("Node accepted session:")
		fmt.Println("  status :", sr.Status)
		fmt.Println("  message:", sr.Message)
		fmt.Println()
		fmt.Println("WireGuard config written to:")
		fmt.Println(" ", path)
		fmt.Println()
		fmt.Println("You can inspect it and later use it with a WireGuard client.")
		return nil
	}
}

// waitForTokenSeconds reads MEERKAT_WAIT_FOR_TOKEN_SECS (int).
func waitForTokenSeconds() int {
	if v := os.Getenv("MEERKAT_WAIT_FOR_TOKEN_SECS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 0
}
