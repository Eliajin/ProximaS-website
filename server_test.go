package main

import (
	"compress/gzip"
	"encoding/json"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testHandler(t *testing.T) (http.Handler, *Site) {
	t.Helper()
	s := build(t, variants(t)["actuel"])
	return s.Handler("test"), s
}

func do(h http.Handler, method, url string, header map[string]string) *http.Response {
	r := httptest.NewRequest(method, url, nil)
	for k, v := range header {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w.Result()
}

func body(t *testing.T, r *http.Response) string {
	t.Helper()
	b, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestPublicAddresses(t *testing.T) {
	h, _ := testHandler(t)
	// Adresses utilisées par l'application et les magasins : ne pas les changer.
	for _, p := range []string{"/", "/mentions-legales", "/confidentialite", "/conditions", "/suppression-compte",
		"/securite", "/contact", "/en/", "/en/legal-notice", "/en/privacy", "/en/terms", "/en/account-deletion",
		"/en/safety", "/en/contact"} {
		r := do(h, "GET", "https://proximas.be"+p, nil)
		if r.StatusCode != 200 || !strings.HasPrefix(r.Header.Get("Content-Type"), "text/html") {
			t.Errorf("%s : %d %s", p, r.StatusCode, r.Header.Get("Content-Type"))
		}
		b := body(t, r)
		for _, want := range []string{`<link rel="canonical" href="https://proximas.be` + p + `">`,
			`hreflang="fr"`, `hreflang="en"`, `hreflang="x-default"`, `property="og:title"`, `name="description"`} {
			if !strings.Contains(b, want) {
				t.Errorf("%s : %s absent", p, want)
			}
		}
	}
	for _, p := range []string{"/robots.txt", "/favicon.ico", "/static/style.css", "/static/fonts/Jost-Variable.ttf",
		"/static/fonts/OFL.txt", "/static/img/partage.jpg", "/static/img/icone.svg"} {
		if r := do(h, "GET", p, nil); r.StatusCode != 200 {
			t.Errorf("%s : %d", p, r.StatusCode)
		}
	}
}

func TestSecurityHeadersAndNoCookie(t *testing.T) {
	h, _ := testHandler(t)
	for _, c := range []struct{ method, url string }{
		{"GET", "/"}, {"GET", "/inconnu"}, {"GET", "/static/style.css"}, {"GET", "/api/health"}, {"POST", "/contact"},
		{"GET", "https://www.proximas.be/"}, {"GET", "/en"},
	} {
		r := do(h, c.method, c.url, nil)
		for k := range securityHeaders {
			if r.Header.Get(k) == "" {
				t.Errorf("%s %s : en-tête %s absent", c.method, c.url, k)
			}
		}
		if len(r.Header.Values("Set-Cookie")) != 0 {
			t.Errorf("%s : cookie envoyé", c.url)
		}
	}
	csp := do(h, "GET", "/", nil).Header.Get("Content-Security-Policy")
	if !strings.Contains(csp, "default-src 'none'") || strings.Contains(csp, "unsafe") || strings.Contains(csp, "http") {
		t.Errorf("CSP trop permissive : %s", csp)
	}
	if cc := do(h, "GET", "/", nil).Header.Get("Cache-Control"); !strings.Contains(cc, "no-transform") {
		t.Errorf("pages sans no-transform : %s", cc)
	}
}

func TestHealth(t *testing.T) {
	h, s := testHandler(t)
	r := do(h, "GET", "/api/health", nil)
	var got struct {
		Status  string `json:"status"`
		Version string `json:"version"`
		Missing int    `json:"champs_manquants"`
	}
	if err := json.Unmarshal([]byte(body(t, r)), &got); err != nil {
		t.Fatal(err)
	}
	if r.StatusCode != 200 || got.Status != "ok" || got.Version != "test" || got.Missing != len(s.cfg.Missing(required)) {
		t.Errorf("santé inattendue : %d %+v", r.StatusCode, got)
	}
}

func TestNotFoundInBothLanguages(t *testing.T) {
	h, _ := testHandler(t)
	for url, lang := range map[string]string{"/absent": "fr", "/en/missing": "en", "/static/absent.png": "fr",
		"/../site.conf": "fr", "/site.conf": "fr", "/content/ui.json": "fr", "/en/privacy/x": "en"} {
		r := do(h, "GET", url, nil)
		b := body(t, r)
		if r.StatusCode != 404 || !strings.Contains(b, `<html lang="`+lang+`">`) || !strings.Contains(b, `content="noindex"`) {
			t.Errorf("%s : %d, langue attendue %s", url, r.StatusCode, lang)
		}
	}
}

func TestRedirects(t *testing.T) {
	h, _ := testHandler(t)
	for from, to := range map[string]string{
		"https://www.proximas.be/":                    "https://proximas.be/",
		"https://www.proximas.be/confidentialite?a=1": "https://proximas.be/confidentialite?a=1",
		"https://WWW.PROXIMAS.BE:443/en/":             "https://proximas.be/en/",
		"https://www.proximas.be//example.invalid/x":  "https://proximas.be//example.invalid/x",
		"https://proximas.be/en":                      "/en/",
		"https://proximas.be/confidentialite/":        "/confidentialite",
		"https://proximas.be/en/privacy/":             "/en/privacy",
		"https://proximas.be/suppression-de-compte":   "/suppression-compte",
		"https://proximas.be/suppression-de-compte/":  "/suppression-compte",
		"https://proximas.be//example.invalid/":       "",
		"https://autre.invalid/confidentialite":       "",
		"https://www.autre.invalid/":                  "",
		"https://proximas.be/static/style.css/":       "",
	} {
		r := do(h, "GET", from, nil)
		if to == "" {
			if r.Header.Get("Location") != "" {
				t.Errorf("%s : redirection inattendue vers %s", from, r.Header.Get("Location"))
			}
			continue
		}
		if r.StatusCode != 301 || r.Header.Get("Location") != to {
			t.Errorf("%s : %d vers %q, attendu %q", from, r.StatusCode, r.Header.Get("Location"), to)
		}
	}
}

func TestNoForcedLanguageRedirect(t *testing.T) {
	h, _ := testHandler(t)
	r := do(h, "GET", "/", map[string]string{"Accept-Language": "en-US,en", "CF-IPCountry": "US"})
	if r.StatusCode != 200 || !strings.Contains(body(t, r), `<html lang="fr">`) {
		t.Error("la page demandée doit être servie telle quelle")
	}
}

func TestMethodsCachingAndCompression(t *testing.T) {
	h, s := testHandler(t)
	for _, m := range []string{"POST", "PUT", "DELETE", "OPTIONS"} {
		if r := do(h, m, "/contact", nil); r.StatusCode != 405 || r.Header.Get("Allow") == "" {
			t.Errorf("%s : %d", m, r.StatusCode)
		}
	}
	if r := do(h, "HEAD", "/conditions", nil); r.StatusCode != 200 || body(t, r) != "" {
		t.Error("HEAD doit répondre sans corps")
	}
	r := do(h, "GET", "/", map[string]string{"Accept-Encoding": "br, gzip"})
	if r.Header.Get("Content-Encoding") != "gzip" || r.Header.Get("Vary") != "Accept-Encoding" {
		t.Fatalf("pas de compression : %v", r.Header)
	}
	zr, err := gzip.NewReader(r.Body)
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := io.ReadAll(zr); string(b) != string(s.pages["/"].body) {
		t.Error("contenu compressé différent")
	}
	if r := do(h, "GET", "/", map[string]string{"Accept-Encoding": "gzip;q=0"}); r.Header.Get("Content-Encoding") != "" {
		t.Error("gzip refusé par le client mais envoyé")
	}
	etag := do(h, "GET", "/", nil).Header.Get("ETag")
	if r := do(h, "GET", "/", map[string]string{"If-None-Match": etag}); etag == "" || r.StatusCode != 304 {
		t.Errorf("ETag : %q %d", etag, r.StatusCode)
	}
	css := s.pages["/static/style.css"]
	if cc := do(h, "GET", "/static/style.css?v="+css.version, nil).Header.Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Errorf("cache long absent : %s", cc)
	}
	if cc := do(h, "GET", "/static/style.css?v=ancienne", nil).Header.Get("Cache-Control"); strings.Contains(cc, "immutable") {
		t.Errorf("cache long sur une mauvaise empreinte : %s", cc)
	}
}

func TestSitemapAndRobots(t *testing.T) {
	h, _ := testHandler(t)
	var set struct {
		URLs []struct {
			Loc string `xml:"loc"`
		} `xml:"url"`
	}
	raw := body(t, do(h, "GET", "/sitemap.xml", nil))
	if err := xml.Unmarshal([]byte(raw), &set); err != nil {
		t.Fatal(err)
	}
	if len(set.URLs) != len(pageDefs)*len(languages) {
		t.Errorf("%d adresses dans le plan du site", len(set.URLs))
	}
	for _, u := range set.URLs {
		if r := do(h, "GET", u.Loc, nil); r.StatusCode != 200 {
			t.Errorf("plan du site : %s répond %d", u.Loc, r.StatusCode)
		}
	}
	if robots := body(t, do(h, "GET", "/robots.txt", nil)); !strings.Contains(robots, "Sitemap: https://proximas.be/sitemap.xml") {
		t.Errorf("robots.txt : %s", robots)
	}
}
