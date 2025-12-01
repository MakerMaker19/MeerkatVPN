package main

import "fmt"

func main() {
	fmt.Println("MeerkatVPN TUI is not implemented yet. This is a placeholder binary.")
}




// package main

// import (
// 	"context"
// 	"fmt"
// 	"log"
// 	"os"
// 	"time"

// 	tea "github.com/charmbracelet/bubbletea"
// 	"github.com/charmbracelet/bubbles/list"

// 	"github.com/MakerMaker19/meerkatvpn/pkg/vpn"
// 	"github.com/MakerMaker19/meerkatvpn/pkg/nostrutil"
// 	"github.com/MakerMaker19/meerkatvpn/pkg/pool"
// 	"github.com/MakerMaker19/meerkatvpn/pkg/client"
// 	"github.com/MakerMaker19/meerkatvpn/pkg/wg"
// )

// // ---- token list item for BubbleTea list ----

// type tokenItem struct {
// 	token vpn.SubscriptionToken
// }

// func (i tokenItem) Title() string {
// 	return i.token.Payload.TokenID
// }
// func (i tokenItem) Description() string {
// 	exp := time.Unix(i.token.Payload.ExpiresAt, 0)
// 	return fmt.Sprintf("plan=%s expires=%s", i.token.Payload.SubscriptionType, exp.Format(time.RFC3339))
// }
// func (i tokenItem) FilterValue() string { return i.token.Payload.TokenID }

// // ---- model ----

// type model struct {
// 	ctx        context.Context
// 	cancel     context.CancelFunc
// 	client     *nostrutil.Client
// 	poolPubHex string

// 	tokens    []vpn.SubscriptionToken
// 	list      list.Model
// 	statusMsg string
// 	loading   bool
// }

// func initialModel(ctx context.Context, nostrPriv, poolPubHex string) (model, error) {
// 	relayURLs := []string{"wss://relay.damus.io", "wss://relay.primal.net"}
// 	client, err := nostrutil.NewClient(ctx, nostrPriv, relayURLs)
// 	if err != nil {
// 		return model{}, err
// 	}

// 	// load tokens from file
// 	store, err := LoadTokenStore()
// 	if err != nil {
// 		return model{}, err
// 	}

// 	items := []list.Item{}
// 	for _, t := range store.Tokens {
// 		items = append(items, tokenItem{token: t})
// 	}

// 	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
// 	l.Title = "MeerkatVPN Subscriptions"

// 	ctx2, cancel := context.WithCancel(ctx)

// 	m := model{
// 		ctx:        ctx2,
// 		cancel:     cancel,
// 		client:     client,
// 		poolPubHex: poolPubHex,
// 		tokens:     store.Tokens,
// 		list:       l,
// 		statusMsg:  "Press r: listen for tokens | c: connect | q: quit",
// 	}

// 	return m, nil
// }

// // ---- messages ----

// type statusMsg string
// type tokenAddedMsg vpn.SubscriptionToken

// // ---- BubbleTea interface ----

// func (m model) Init() tea.Cmd {
// 	// no async on start
// 	return nil
// }

// func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
// 	switch msg := msg.(type) {

// 	case tea.KeyMsg:
// 		switch msg.String() {
// 		case "ctrl+c", "q":
// 			m.cancel()
// 			return m, tea.Quit

// 		case "r":
// 			// start token listener
// 			m.statusMsg = "Listening for subscription tokens..."
// 			return m, m.listenForTokensCmd()

// 		case "c":
// 			// naive connect: use latest token & prompt via stdin for node pubkey
// 			return m, m.connectCmd()
// 		}

// 	case statusMsg:
// 		m.statusMsg = string(msg)
// 		return m, nil

// 	case tokenAddedMsg:
// 		t := vpn.SubscriptionToken(msg)
// 		m.tokens = append(m.tokens, t)
// 		m.list.InsertItem(0, tokenItem{token: t})
// 		m.statusMsg = "Received new subscription token."
// 		return m, nil
// 	}

// 	var cmd tea.Cmd
// 	m.list, cmd = m.list.Update(msg)
// 	return m, cmd
// }

// func (m model) View() string {
// 	return fmt.Sprintf(
// 		"MeerkatVPN TUI\nNostr pubkey: %s\n\n%s\n\nStatus: %s\n",
// 		m.client.PubKey,
// 		m.list.View(),
// 		m.statusMsg,
// 	)
// }

// // ---- commands ----

// func (m model) listenForTokensCmd() tea.Cmd {
// 	return func() tea.Msg {
// 		if err := listenForTokensOnce(m.ctx, m.client); err != nil {
// 			return statusMsg(fmt.Sprintf("listener error: %v", err))
// 		}
// 		return statusMsg("Token listener stopped.")
// 	}
// }

// func (m model) connectCmd() tea.Cmd {
// 	return func() tea.Msg {
// 		fmt.Print("Enter node pubkey (npub or hex): ")
// 		var nodePubRaw string
// 		fmt.Scanln(&nodePubRaw)

// 		fmt.Print("Enter region (e.g. us-east, or press Enter for auto): ")
// 		var region string
// 		fmt.Scanln(&region)
// 		if region == "" {
// 			region = "auto"
// 		}

// 		// call our existing connect helper (from client CLI)
// 		err := connectOnce(m.ctx, m.client, m.poolPubHex, nodePubRaw)
// 		if err != nil {
// 			return statusMsg(fmt.Sprintf("connect error: %v", err))
// 		}
// 		return statusMsg("Connection config written. Import into WireGuard.")
// 	}
// }

// // ---- helper: token listening (simplified) ----

// func listenForTokensOnce(ctx context.Context, client *nostrutil.Client) error {
// 	store, err := LoadTokenStore()
// 	if err != nil {
// 		return err
// 	}

// 	// same logic as cmdReceiveTokens but run until ctx cancelled
// 	filters := nostr.Filters{
// 		{
// 			Kinds: []int{nostr.KindEncryptedDirectMessage},
// 			Tags:  nostr.TagMap{"p": []string{client.PubKey}},
// 		},
// 	}

// 	for _, relay := range client.Relays {
// 		go func(r *nostr.Relay) {
// 			ch, subErr := r.Subscribe(ctx, filters)
// 			if subErr != nil {
// 				log.Println("subscribe error:", subErr)
// 				return
// 			}
// 			for ev := range ch {
// 				e := ev.Event
// 				if e.Kind != nostr.KindEncryptedDirectMessage {
// 					continue
// 				}
// 				if !e.Tags.Contains("t", "vpn-subscription") {
// 					continue
// 				}
// 				plain, decErr := nostr.Decrypt(client.PrivKey, e.PubKey, e.Content)
// 				if decErr != nil {
// 					log.Println("decrypt DM error:", decErr)
// 					continue
// 				}
// 				var token vpn.SubscriptionToken
// 				if err := json.Unmarshal([]byte(plain), &token); err != nil {
// 					log.Println("token parse error:", err)
// 					continue
// 				}
// 				store.AddToken(token)
// 				if err := store.Save(); err != nil {
// 					log.Println("save token error:", err)
// 				}
// 				log.Printf("New token %s exp=%v\n", token.Payload.TokenID, time.Unix(token.Payload.ExpiresAt, 0))
// 				// NOTE: could send BubbleTea message via channel; for now we just log
// 			}
// 		}(relay)
// 	}

// 	<-ctx.Done()
// 	return nil
// }

// // Very thin wrapper around connect logic from client-cli; you’d DRY that into a shared pkg.
// func connectOnce(ctx context.Context, client *nostrutil.Client, poolPubHex, nodePubRaw string) error {
// 	// parse node pub
// 	nodePubHex, err := nostrutil.ParsePubKey(nodePubRaw)
// 	if err != nil {
// 		return err
// 	}
// 	_ = nodePubHex // you’d reuse cmdConnect logic from client-cli here.
// 	return fmt.Errorf("connectOnce not fully implemented; hook into cmdConnect from client-cli")
// }

// func main() {
// 	rawPriv := os.Getenv("MEERKAT_CLIENT_NOSTR_PRIVKEY")
// 	if rawPriv == "" {
// 		log.Fatal("MEERKAT_CLIENT_NOSTR_PRIVKEY not set (nsec or hex)")
// 	}
// 	poolPubRaw := os.Getenv("MEERKAT_CLIENT_POOL_PUBKEY")
// 	if poolPubRaw == "" {
// 		log.Fatal("MEERKAT_CLIENT_POOL_PUBKEY not set (npub or hex)")
// 	}
// 	poolPubHex, err := nostrutil.ParsePubKey(poolPubRaw)
// 	if err != nil {
// 		log.Fatal("invalid pool pubkey:", err)
// 	}

// 	ctx := context.Background()
// 	m, err := initialModel(ctx, rawPriv, poolPubHex)
// 	if err != nil {
// 		log.Fatal("init model:", err)
// 	}

// 	if err := tea.NewProgram(m, tea.WithAltScreen()).Start(); err != nil {
// 		log.Fatal(err)
// 	}
// }
