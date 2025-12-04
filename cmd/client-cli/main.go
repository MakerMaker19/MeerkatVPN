package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/MakerMaker19/meerkatvpn/pkg/client"
	"github.com/MakerMaker19/meerkatvpn/pkg/vpn"
)

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
	case "connect":
		if err := cmdConnect(); err != nil {
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

// cmdConnect:
// - Uses the subscription token flow exactly as before
// - Calls POST /session/create with {token, client_wg_pubkey, backend}
// - If backend == "wireguard": builds and writes a WG config (existing behavior)
// - If backend == "openvpn": expects ovpn_profile in the response and writes meerkat.ovpn
func cmdConnect() error {
	nodeURL := os.Getenv("MEERKAT_NODE_URL")
	if nodeURL == "" {
		nodeURL = "http://localhost:9090"
	}

	backend := os.Getenv("MEERKAT_TUNNEL_BACKEND")
	if backend == "" {
		backend = "wireguard" // default to current behavior
	}

	poolPub := os.Getenv("MEERKAT_CLIENT_POOL_PUBKEY")
	if poolPub == "" {
		return fmt.Errorf("MEERKAT_CLIENT_POOL_PUBKEY not set")
	}

	ts, err := client.LoadTokenStore()
	if err != nil {
		return fmt.Errorf("load token store: %w", err)
	}

	tok, err := ts.LatestValid(poolPub, time.Now())
	if err != nil {
		return fmt.Errorf("no valid tokens: %w", err)
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
		ClientWGPubKey string               `json:"client_wg_pubkey"`
		Backend        string               `json:"backend"` // "wireguard" or "openvpn"
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

		path := "meerkat.ovpn"
		if err := os.WriteFile(path, []byte(sr.OVPNProfile), 0o600); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}

		fmt.Println("Node accepted session:")
		fmt.Println("  status :", sr.Status)
		fmt.Println("  message:", sr.Message)
		fmt.Println()
		fmt.Println("OpenVPN profile written to:")
		fmt.Println(" ", path)
		fmt.Println()
		if runtime.GOOS == "windows" {
			fmt.Println("Import this file into the OpenVPN GUI and click Connect.")
		} else {
			fmt.Println("You can start it with:")
			fmt.Println("  sudo openvpn --config", path)
		}
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
