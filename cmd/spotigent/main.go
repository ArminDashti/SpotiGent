// SpotiGent — manage your Spotify account with AI.
//
// Run:
//
//	go run ./cmd/spotigent
//
// Environment (all optional):
//
//	SPOTIGENT_HOST=127.0.0.1
//	SPOTIGENT_PORT=8080
//	OPENROUTER_API_KEY=...   (fallback; UI keys take precedence)
//	OPENCODE_API_KEY=...     (fallback; UI keys take precedence)
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"spotigent/internal/config"
	"spotigent/internal/server"
	"spotigent/internal/store"
)

// version is stamped at release time, e.g.:
//	go build -ldflags "-X main.version=1.1.0" ./cmd/spotigent
var version = "dev"

func main() {
	port := flag.String("port", "", "listen port (overrides SPOTIGENT_PORT)")
	flag.Parse()

	cfg := config.Load()
	if *port != "" {
		cfg.Port = *port
	}

	dataDir := os.Getenv("SPOTIGENT_DATA_DIR")
	if dataDir == "" {
		dataDir = "data"
	}

	st, err := store.New(dataDir + "/settings.json")
	if err != nil {
		log.Fatalf("settings store: %v", err)
	}

	srv := server.New(cfg, st)
	addr := cfg.Addr()
	fmt.Printf("SpotiGent %s listening on http://%s\n", version, addr)
	if err := srv.Router().Run(addr); err != nil {
		log.Fatal(err)
	}
}
