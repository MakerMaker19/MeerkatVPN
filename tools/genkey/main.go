package main

import (
	"fmt"

	"github.com/nbd-wtf/go-nostr"
)

func main() {
	sk := nostr.GeneratePrivateKey()
	pk, _ := nostr.GetPublicKey(sk)

	fmt.Println("nsec:", sk)
	fmt.Println("npub:", pk)
}
