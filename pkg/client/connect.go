package client

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
	// Backend: env -> config -> default
	backend := os.Getenv("MEERKAT_TUNNEL_BACKEND")
	if backend == "" {
		if cfg := Config(); cfg != nil && cfg.Backend != "" {
			backend = cfg.Backend
		} else {
			backend = "openvpn"
		}
	}

	// Node URL: env -> config -> error
	nodeURL := os.Getenv("MEERKAT_NODE_URL")
	if nodeURL == "" {
		if cfg := Config(); cfg != nil && cfg.NodeURL != "" {
			nodeURL = cfg.NodeURL
		}
	}
	if nodeURL == "" {
		return fmt.Errorf("MEERKAT_NODE_URL not set and no node_url in meerkat-client.yaml")
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
		path, err := writeOpenVPNProfile(cfgBytes)
		if err != nil {
			return fmt.Errorf("write OpenVPN profile: %w", err)
		}

		fmt.Println("Saved OpenVPN profile to", path)
		fmt.Println("Import this profile into your OpenVPN client and connect.")
		return nil

	case "wireguard":
		// Parking WG for now: just save the config so you can inspect it.
		path := "meerkat-wg.conf"
		if err := os.WriteFile(path, cfgBytes, 0o600); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
		fmt.Println("Saved WireGuard config to", path)
		fmt.Println("WireGuard backend is currently on hold; config is not auto-applied.")
		return nil

	default:
		return fmt.Errorf("unknown MEERKAT_TUNNEL_BACKEND %q (expected \"openvpn\" or \"wireguard\")", backend)
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
