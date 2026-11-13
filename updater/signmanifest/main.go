package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
)

func main() {
	in := flag.String("in", "", "")
	pubOut := flag.String("pub-out", "", "")
	seedEnv := flag.String("seed-env", "UPDATER_ED25519_SEED", "")
	flag.Parse()
	seed, err := base64.StdEncoding.DecodeString(os.Getenv(*seedEnv))
	if err != nil || len(seed) != ed25519.SeedSize {
		fmt.Fprintf(os.Stderr, "invalid %s: expected base64 of %d bytes\n", *seedEnv, ed25519.SeedSize)
		os.Exit(2)
	}
	key := ed25519.NewKeyFromSeed(seed)
	if *in != "" {
		body, err := os.ReadFile(*in)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		sig := ed25519.Sign(key, body)
		if err := os.WriteFile(*in+".sig", []byte(base64.StdEncoding.EncodeToString(sig)), 0644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
	if *pubOut != "" {
		pub := key.Public().(ed25519.PublicKey)
		if err := os.WriteFile(*pubOut, []byte(base64.StdEncoding.EncodeToString(pub)), 0644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
}
