package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
)

// ConnectToNode currently talks to a Meerkat node over HTTP,
// fetches a VPN client configuration, and writes it to disk.
//
// Backend is selected via MEERKAT_TUNNEL_BACKEND:
//   - "openvpn"  -> writes meerkat.ovpn (OpenVPN profile)
//   - "wireguard" -> writes meerkat-wg.conf (WG config, not auto-applied)
//
// WireGuard + Nostr integration from the older implementation is
// intentionally "on hold" so you can focus on getting OpenVPN working.
func ConnectToNode(ctx context.Context, nodePubKey string) error {
	backend := os.Getenv("MEERKAT_TUNNEL_BACKEND")
	if backend == "" {
		backend = "openvpn" // default to OpenVPN while WG is paused
	}

	nodeURL := os.Getenv("MEERKAT_NODE_URL")
	if nodeURL == "" {
		return fmt.Errorf("MEERKAT_NODE_URL is not set (e.g. http://46.62.204.11:9090)")
	}

	// For now we assume the node exposes a simple HTTP endpoint like:
	//   GET /connect
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, nodeURL+"/connect", nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("request node: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("node returned %s: %s", resp.Status, string(body))
	}

	cfgBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read node response: %w", err)
	}

	switch backend {
	case "openvpn":
		path := "meerkat.ovpn"
		if err := os.WriteFile(path, cfgBytes, 0o600); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}

		fmt.Println("Saved OpenVPN profile to", path)
		if runtime.GOOS == "windows" {
			fmt.Println("→ Import this file into the OpenVPN GUI and click Connect.")
		} else {
			fmt.Println("→ You can start it with:")
			fmt.Println("    sudo openvpn --config", path)
		}
		return nil

	case "wireguard":
		// Parking WG for now: just save the config so you can inspect it.
		path := "meerkat-wg.conf"
		if err := os.WriteFile(path, cfgBytes, 0o600); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
		fmt.Println("Saved WireGuard config to", path)
		fmt.Println("→ WireGuard backend is currently on hold; config is not auto-applied.")
		return nil

	default:
		return fmt.Errorf("unknown MEERKAT_TUNNEL_BACKEND %q (expected \"openvpn\" or \"wireguard\")", backend)
	}
}
