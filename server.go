package main

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// En-têtes de sécurité envoyés avec chaque réponse. Aucune ressource externe, aucun script.
var securityHeaders = map[string]string{
	"Content-Security-Policy": "default-src 'none'; img-src 'self'; style-src 'self'; font-src 'self'; " +
		"base-uri 'none'; form-action 'none'; frame-ancestors 'none'",
	"Strict-Transport-Security": "max-age=31536000",
	"X-Content-Type-Options":    "nosniff",
	"X-Frame-Options":           "DENY",
	"Referrer-Policy":           "strict-origin-when-cross-origin",
	"Permissions-Policy": "accelerometer=(), browsing-topics=(), camera=(), geolocation=(), gyroscope=(), " +
		"magnetometer=(), microphone=(), payment=(), usb=()",
	"Cross-Origin-Opener-Policy": "same-origin",
}

// Handler sert le site. Aucune requête n'est journalisée (la politique de confidentialité l'annonce).
func (s *Site) Handler(version string) http.Handler {
	missing := len(s.cfg.Missing(required))
	domain := s.cfg.Domain()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		for k, v := range securityHeaders {
			h.Set(k, v)
		}

		// « www. » vers le domaine principal. La destination est fixe : pas de redirection ouverte.
		host := r.Host
		if hn, _, err := net.SplitHostPort(host); err == nil {
			host = hn
		}
		if strings.EqualFold(strings.TrimSuffix(host, "."), "www."+domain) {
			redirect(w, s.cfg.Get("url_site")+safePath(r))
			return
		}

		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			h.Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		p := r.URL.Path
		if p == "/api/health" {
			h.Set("Content-Type", "application/json")
			h.Set("Cache-Control", "no-store")
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "version": version, "champs_manquants": missing})
			return
		}

		if res, ok := s.pages[p]; ok {
			s.write(w, r, res, http.StatusOK)
			return
		}
		if to, ok := aliases[p]; ok {
			redirect(w, to)
			return
		}
		// Barre finale en trop : /confidentialite/ vers /confidentialite.
		if trimmed := strings.TrimRight(p, "/"); trimmed != p {
			if res, ok := s.pages[trimmed]; ok && strings.HasPrefix(res.ctype, "text/html") {
				redirect(w, trimmed)
				return
			}
			if to, ok := aliases[trimmed]; ok {
				redirect(w, to)
				return
			}
		}
		lang := languages[0]
		if strings.HasPrefix(p, "/en/") {
			lang = "en"
		}
		s.write(w, r, s.notFound[lang], http.StatusNotFound)
	})
}

// safePath : chemin et requête tels que reçus, toujours précédés d'une barre.
func safePath(r *http.Request) string {
	p := r.URL.EscapedPath()
	if !strings.HasPrefix(p, "/") {
		p = "/"
	}
	if r.URL.RawQuery != "" {
		p += "?" + r.URL.RawQuery
	}
	return p
}

func redirect(w http.ResponseWriter, to string) {
	w.Header().Set("Location", to)
	w.Header().Set("Cache-Control", cacheShort)
	w.WriteHeader(http.StatusMovedPermanently)
}

func (s *Site) write(w http.ResponseWriter, r *http.Request, res *resource, status int) {
	h := w.Header()
	cache := res.cache
	if strings.HasPrefix(r.URL.Path, "/static/") && r.URL.Query().Get("v") == res.version {
		cache = cacheImmutable
	}
	h.Set("Content-Type", res.ctype)
	h.Set("Cache-Control", cache)
	if r.URL.Path != "/static/img/partage.jpg" { // image de partage : affichable par d'autres sites
		h.Set("Cross-Origin-Resource-Policy", "same-origin")
	}
	body := res.body
	if res.gz != nil {
		h.Set("Vary", "Accept-Encoding")
		if acceptsGzip(r.Header.Get("Accept-Encoding")) {
			h.Set("Content-Encoding", "gzip")
			body = res.gz
		}
	}
	if status == http.StatusOK {
		h.Set("ETag", res.etag)
		// ServeContent gère If-None-Match, HEAD et les requêtes partielles.
		http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(body))
		return
	}
	h.Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		_, _ = w.Write(body)
	}
}

func acceptsGzip(header string) bool {
	for _, part := range strings.Split(header, ",") {
		name, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		if strings.EqualFold(strings.TrimSpace(name), "gzip") {
			q := strings.ReplaceAll(params, " ", "")
			return q != "q=0" && q != "q=0.0"
		}
	}
	return false
}
