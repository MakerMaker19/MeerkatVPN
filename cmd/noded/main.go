package main

import (
    "encoding/json"
    "log"
    "net"
    "net/http"
    "os"
    "time"

    "github.com/google/uuid"

    "github.com/MakerMaker19/meerkatvpn/pkg/vpn"
    "github.com/MakerMaker19/meerkatvpn/pkg/wg"
)

type sessionCreateRequest struct {
    Token          vpn.SubscriptionToken `json:"token"`
    ClientWGPubKey string               `json:"client_wg_pubkey,omitempty"`
    Backend        string               `json:"backend,omitempty"` // "wireguard" or "openvpn"
}

type sessionCreateResponse struct {
    Status  string `json:"status"`
    Message string `json:"message,omitempty"`

    // WireGuard parameters for the client to build a config.
    ServerPubKey string   `json:"server_pubkey,omitempty"`
    Endpoint     string   `json:"endpoint,omitempty"`
    ClientIP     string   `json:"client_ip,omitempty"`
    AllowedIPs   string   `json:"allowed_ips,omitempty"`
    DNS          []string `json:"dns,omitempty"`

    // OpenVPN profile text (full .ovpn file) for OpenVPN backend.
    OVPNProfile string `json:"ovpn_profile,omitempty"`
}

func main() {
    addr := os.Getenv("MEERKAT_NODE_LISTEN_ADDR")
    if addr == "" {
        addr = ":9090"
    }

    // Optional: restrict which pool/issuer pubkey is allowed.
    allowedPool := os.Getenv("MEERKAT_NODE_ALLOWED_POOL_PUBKEY")

    wgMgr, err := wg.NewManagerFromEnv()
    if err != nil {
        log.Printf("warning: failed to init WireGuard manager: %v (will still accept sessions with static IP)\n", err)
        wgMgr = nil
    }

    http.HandleFunc("/session/create", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
            http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
            return
        }

        var req sessionCreateRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            log.Println("session create decode error:", err)
            http.Error(w, "bad request", http.StatusBadRequest)
            return
        }

        tok := req.Token

        // Optional issuer/pubkey filter.
        if allowedPool != "" && tok.Payload.IssuerPubKey != allowedPool {
            log.Printf("session create: issuer mismatch (got %s, expected %s)\n",
                tok.Payload.IssuerPubKey, allowedPool)
            writeJSON(w, http.StatusForbidden, sessionCreateResponse{
                Status:  "error",
                Message: "token issuer not allowed",
            })
            return
        }

        // Verify signature + expiry.
        if err := vpn.VerifySubscription(tok, time.Now()); err != nil {
            log.Println("session create: token invalid:", err)
            writeJSON(w, http.StatusForbidden, sessionCreateResponse{
                Status:  "error",
                Message: "invalid token: " + err.Error(),
            })
            return
        }

        // Decide which backend to use (default: openvpn).
        backend := req.Backend
        if backend == "" {
            backend = "openvpn"
        }

        // Session ID for logging/metadata.
        sessionID := uuid.NewString()

        // Parse remote IP (strip port).
        remoteIP, _, _ := net.SplitHostPort(r.RemoteAddr)

        // Structured logging for audit trail.
        log.Printf("session create: accepted token=%s user=%s backend=%s session_id=%s remote_ip=%s",
            tok.Payload.TokenID,
            tok.Payload.UserPubKey,
            backend,
            sessionID,
            remoteIP,
        )

        // === Backend: OpenVPN ==========================================
        if backend == "openvpn" {
            ovpnPath := os.Getenv("MEERKAT_NODE_OVPN_PROFILE_PATH")
            if ovpnPath == "" {
                ovpnPath = "/etc/openvpn/meerkat-client.ovpn"
            }

            profileBytes, err := os.ReadFile(ovpnPath)
            if err != nil {
                log.Printf("session create: failed to read OpenVPN profile from %s: %v\n", ovpnPath, err)
                writeJSON(w, http.StatusInternalServerError, sessionCreateResponse{
                    Status:  "error",
                    Message: "node: could not load OpenVPN profile (check MEERKAT_NODE_OVPN_PROFILE_PATH and file perms)",
                })
                return
            }

            // Build per-session header.
            header := "# MeerkatVPN session\n" +
                "# session_id: " + sessionID + "\n" +
                "# token_id: " + tok.Payload.TokenID + "\n" +
                "# user_pubkey: " + tok.Payload.UserPubKey + "\n" +
                "# backend: " + backend + "\n" +
                "# remote_ip: " + remoteIP + "\n" +
                "# created_at: " + time.Now().UTC().Format(time.RFC3339) + "\n\n"

            fullProfile := header + string(profileBytes)

            writeJSON(w, http.StatusOK, sessionCreateResponse{
                Status:      "ok",
                Message:     "session accepted (OpenVPN profile)",
                OVPNProfile: fullProfile,
            })
            return
        }

        // === Backend: WireGuard (original behavior) ====================

        // Decide client IP:
        clientIP := "10.8.0.2/32" // default fallback
        if wgMgr != nil && req.ClientWGPubKey != "" {
            if ip, err := wgMgr.AllocatePeer(req.ClientWGPubKey); err != nil {
                log.Println("wg allocate peer error:", err)
            } else {
                clientIP = ip
                if err := wgMgr.ApplyPeer(req.ClientWGPubKey, clientIP); err != nil {
                    log.Println("wg apply peer error:", err)
                }
            }
        }

        // Read WG-related env vars or use some simple defaults.
        serverPub := os.Getenv("MEERKAT_NODE_WG_PUBKEY")
        if serverPub == "" {
            serverPub = "SERVER_WG_PUBKEY_PLACEHOLDER"
        }
        endpoint := os.Getenv("MEERKAT_NODE_WG_ENDPOINT")
        if endpoint == "" {
            endpoint = "127.0.0.1:51820"
        }
        allowed := "0.0.0.0/0, ::/0"
        dns := []string{"1.1.1.1"}

        writeJSON(w, http.StatusOK, sessionCreateResponse{
            Status:       "ok",
            Message:      "session accepted (WireGuard config)",
            ServerPubKey: serverPub,
            Endpoint:     endpoint,
            ClientIP:     clientIP,
            AllowedIPs:   allowed,
            DNS:          dns,
        })
    })

    log.Printf("noded: listening on %s\n", addr)
    if err := http.ListenAndServe(addr, nil); err != nil {
        log.Fatal(err)
    }
}

func writeJSON(w http.ResponseWriter, code int, v any) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(code)
    _ = json.NewEncoder(w).Encode(v)
}
