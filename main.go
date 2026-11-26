// Site de présentation de ProximaS (proximas.be) : pages statiques rendues au démarrage et servies
// depuis la mémoire. Bibliothèque standard seulement, aucune base de données, aucun secret.
//
//	proximas-web [serve]       sert le site (LISTEN_ADDR, défaut :8080)
//	proximas-web champs        liste les champs de site.conf encore vides (--strict : échoue s'il en manque)
//	proximas-web healthcheck   interroge /api/health (contrôle Docker)
package main

import (
	"bytes"
	"context"
	"embed"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

//go:embed site.conf content static
var embedded embed.FS

// version : renseignée à la compilation (-ldflags "-X main.version=...").
var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout))
}

func run(args []string, out io.Writer) int {
	cmd := "serve"
	if len(args) > 0 {
		cmd, args = args[0], args[1:]
	}
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	if cmd == "healthcheck" {
		return healthcheck(addr)
	}
	cfg, err := embeddedConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	switch cmd {
	case "champs":
		strict := len(args) == 1 && args[0] == "--strict"
		if len(args) > 0 && !strict {
			break
		}
		if n := ReportMissing(out, cfg); strict && n > 0 {
			return 1
		}
		return 0
	case "serve":
		if len(args) > 0 {
			break
		}
		if err := serve(addr, cfg); err != nil {
			log.Println(err)
			return 1
		}
		return 0
	}
	fmt.Fprintln(os.Stderr, "usage : proximas-web [serve | champs [--strict] | healthcheck]")
	return 2
}

func embeddedConfig() (*Config, error) {
	raw, err := embedded.ReadFile("site.conf")
	if err != nil {
		return nil, err
	}
	return ParseConfig(bytes.NewReader(raw))
}

func serve(addr string, cfg *Config) error {
	site, err := BuildSite(embedded, cfg)
	if err != nil {
		return err
	}
	srv := &http.Server{
		Addr:              addr,
		Handler:           site.Handler(version),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	done := make(chan struct{})
	go func() {
		defer close(done)
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(c)
	}()
	log.Printf("proximas-web %s : écoute sur %s, %d champ(s) obligatoire(s) à remplir dans site.conf",
		version, addr, len(cfg.Missing(required)))
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	<-done
	return nil
}

func healthcheck(addr string) int {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return 1
	}
	client := &http.Client{Timeout: 4 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + port + "/api/health")
	if err != nil {
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}
