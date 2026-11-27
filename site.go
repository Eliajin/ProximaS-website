package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	"html/template"
	"io/fs"
	"path"
	"strings"
	"time"
)

// pageDef : une page du site, avec son adresse dans chaque langue.
type pageDef struct {
	id    string
	paths map[string]string
	legal bool   // bandeau « brouillon, à valider par un juriste » et date des textes
	date  string // version archivée : sa date, figée (vide : date_textes)
	text  bool   // page de texte (colonne étroite) qui n'est pas un texte légal
}

// pageDefs : les adresses françaises sont utilisées par l'application et les magasins : ne pas les changer.
var pageDefs = []pageDef{
	{id: "home", paths: map[string]string{"fr": "/", "en": "/en/"}},
	{id: "tester", text: true, paths: map[string]string{"fr": "/devenir-testeur", "en": "/en/become-a-tester"}},
	{id: "safety", legal: true, paths: map[string]string{"fr": "/securite", "en": "/en/safety"}},
	{id: "privacy", legal: true, paths: map[string]string{"fr": "/confidentialite", "en": "/en/privacy"}},
	{id: "terms", legal: true, paths: map[string]string{"fr": "/conditions", "en": "/en/terms"}},
	// Archives : texte figé (content/<langue>/<id>.html), jamais modifié après publication.
	{id: "terms-v1", legal: true, date: "2026-11-26", paths: map[string]string{"fr": "/conditions/version-1", "en": "/en/terms/version-1"}},
	{id: "privacy-v1", legal: true, date: "2026-11-26", paths: map[string]string{"fr": "/confidentialite/version-1", "en": "/en/privacy/version-1"}},
	{id: "privacy-v2", legal: true, date: "2026-11-27", paths: map[string]string{"fr": "/confidentialite/version-2", "en": "/en/privacy/version-2"}},
	{id: "deletion", legal: true, paths: map[string]string{"fr": "/suppression-compte", "en": "/en/account-deletion"}},
	{id: "contact", legal: true, paths: map[string]string{"fr": "/contact", "en": "/en/contact"}},
	{id: "legal", legal: true, paths: map[string]string{"fr": "/mentions-legales", "en": "/en/legal-notice"}},
}

const notFoundID = "404"

// defaultAlternate : langue proposée aux moteurs de recherche pour les autres langues (hreflang x-default).
const defaultAlternate = "en"

var (
	mainNav   = []string{"home", "tester", "safety", "privacy", "contact"}
	footerNav = []string{"legal", "privacy", "terms", "deletion", "safety", "contact", "tester"}
)

// aliases : adresses anciennes ou approchantes, redirigées (301).
var aliases = map[string]string{
	"/en":                    "/en/",
	"/fr":                    "/",
	"/fr/":                   "/",
	"/suppression-de-compte": "/suppression-compte",
	"/en/suppression-compte": "/en/account-deletion",
	"/en/confidentialite":    "/en/privacy",
	"/en/conditions":         "/en/terms",
	"/en/devenir-testeur":    "/en/become-a-tester",
}

var contentTypes = map[string]string{
	".css": "text/css; charset=utf-8",
	".png": "image/png",
	".jpg": "image/jpeg",
	".svg": "image/svg+xml",
	".ttf": "font/ttf",
	".txt": "text/plain; charset=utf-8",
}

var months = map[string][]string{
	"fr": {"janvier", "février", "mars", "avril", "mai", "juin", "juillet", "août", "septembre", "octobre", "novembre", "décembre"},
	"en": {"January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"},
}

var missingMark = map[string]string{"fr": "[À REMPLIR]", "en": "[TO BE COMPLETED]"}

// resource : un contenu prêt à servir.
type resource struct {
	body, gz []byte
	ctype    string
	cache    string
	etag     string
	version  string // fichiers statiques : empreinte ajoutée à l'adresse (?v=)
}

// Site : toutes les réponses, calculées une fois au démarrage.
type Site struct {
	cfg      *Config
	ui       map[string]map[string]string
	pages    map[string]*resource // par adresse
	notFound map[string]*resource // par langue
}

type navItem struct {
	URL, Label string
	Current    bool
}

type alternate struct{ Lang, URL string }

type view struct {
	Lang, OtherLang, OtherURL   string
	Title, Description          string
	Canonical, ShareImage, Date string
	DateISO                     string
	Alternates                  []alternate
	MainNav, FooterNav          []navItem
	Legal, NotFound, Narrow     bool
}

const (
	cacheHTML      = "public, max-age=300, no-transform"
	cacheShort     = "public, max-age=3600"
	cacheStatic    = "public, max-age=86400"
	cacheFont      = "public, max-age=2592000"
	cacheImmutable = "public, max-age=31536000, immutable"
)

func newResource(body []byte, ctype, cache string) *resource {
	sum := sha256.Sum256(body)
	r := &resource{body: body, ctype: ctype, cache: cache, version: hex.EncodeToString(sum[:6])}
	r.etag = `W/"` + hex.EncodeToString(sum[:12]) + `"`
	if !strings.HasPrefix(ctype, "image/png") && !strings.HasPrefix(ctype, "image/jpeg") {
		var b bytes.Buffer
		zw, _ := gzip.NewWriterLevel(&b, gzip.BestCompression)
		_, _ = zw.Write(body)
		_ = zw.Close()
		if b.Len() < len(body) {
			r.gz = b.Bytes()
		}
	}
	return r
}

// BuildSite lit les textes et les fichiers statiques de fsys et rend toutes les pages.
func BuildSite(fsys fs.FS, cfg *Config) (*Site, error) {
	s := &Site{cfg: cfg, pages: map[string]*resource{}, notFound: map[string]*resource{}}

	raw, err := fs.ReadFile(fsys, "content/ui.json")
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &s.ui); err != nil {
		return nil, fmt.Errorf("content/ui.json : %w", err)
	}
	for key, texts := range s.ui {
		for _, lang := range languages {
			if strings.TrimSpace(texts[lang]) == "" {
				return nil, fmt.Errorf("content/ui.json : « %s » n'a pas de texte « %s »", key, lang)
			}
		}
		if len(texts) != len(languages) {
			return nil, fmt.Errorf("content/ui.json : « %s » contient une langue inconnue", key)
		}
	}

	err = fs.WalkDir(fsys, "static", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		ctype, ok := contentTypes[path.Ext(p)]
		if !ok {
			return fmt.Errorf("%s : type de fichier non prévu", p)
		}
		body, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		if !strings.HasPrefix(ctype, "image/") && !strings.HasPrefix(ctype, "font/") {
			body = bytes.ReplaceAll(body, []byte("\r\n"), []byte("\n"))
		}
		cache := cacheStatic
		if strings.HasPrefix(ctype, "font/") {
			cache = cacheFont
		}
		s.pages["/"+p] = newResource(body, ctype, cache)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if icon, ok := s.pages["/static/img/favicon-32.png"]; ok {
		s.pages["/favicon.ico"] = icon
	}

	layout, err := fs.ReadFile(fsys, "content/layout.html")
	if err != nil {
		return nil, err
	}
	for _, lang := range languages {
		for _, p := range pageDefs {
			body, err := s.render(fsys, string(layout), lang, p)
			if err != nil {
				return nil, err
			}
			s.pages[p.paths[lang]] = newResource(body, "text/html; charset=utf-8", cacheHTML)
		}
		body, err := s.render(fsys, string(layout), lang, pageDef{id: notFoundID})
		if err != nil {
			return nil, err
		}
		s.notFound[lang] = newResource(body, "text/html; charset=utf-8", "no-cache, no-transform")
	}

	s.pages["/robots.txt"] = newResource([]byte("User-agent: *\nDisallow: /api/\n\nSitemap: "+cfg.Get("url_site")+"/sitemap.xml\n"),
		"text/plain; charset=utf-8", cacheShort)
	s.pages["/sitemap.xml"] = newResource(s.sitemap(), "application/xml; charset=utf-8", cacheShort)
	return s, nil
}

func pageByID(id string) (pageDef, bool) {
	for _, p := range pageDefs {
		if p.id == id {
			return p, true
		}
	}
	return pageDef{}, false
}

// pageDate : date des textes d'une page (AAAA-MM-JJ).
func (s *Site) pageDate(p pageDef) string {
	if p.date != "" {
		return p.date
	}
	return s.cfg.Get("date_textes")
}

func otherLanguage(lang string) string {
	if lang == languages[0] {
		return languages[1]
	}
	return languages[0]
}

func (s *Site) text(lang, key string) (string, error) {
	t, ok := s.ui[key]
	if !ok {
		return "", fmt.Errorf("texte inconnu dans content/ui.json : %s", key)
	}
	return t[lang], nil
}

// funcs : fonctions disponibles dans les gabarits. Une clé inconnue arrête le démarrage.
func (s *Site) funcs(lang string) template.FuncMap {
	val := func(key string) (string, error) {
		if _, ok := knownField(key); !ok {
			return "", fmt.Errorf("clé de site.conf inconnue : %s", key)
		}
		return s.cfg.Get(key), nil
	}
	return template.FuncMap{
		"t":   func(key string) (string, error) { return s.text(lang, key) },
		"val": val,
		"on": func(key string) (bool, error) {
			f, ok := knownField(key)
			if !ok || f.kind != toggle {
				return false, fmt.Errorf("interrupteur inconnu : %s", key)
			}
			return s.cfg.On(key), nil
		},
		// champ : valeur affichée (dans la langue de la page si la clé a des variantes), ou « [À REMPLIR] ».
		"champ": func(key string) (template.HTML, error) {
			if _, ok := knownField(key + "_" + lang); ok {
				key += "_" + lang
			}
			v, err := val(key)
			if err != nil {
				return "", err
			}
			if v == "" {
				return template.HTML(`<mark class="manque">` + missingMark[lang] + `</mark>`), nil
			}
			return template.HTML(html.EscapeString(v)), nil
		},
		"mail": func() template.HTML {
			e := html.EscapeString(s.cfg.Get("contact_email"))
			return template.HTML(`<a href="mailto:` + e + `">` + e + `</a>`)
		},
		"page": func(id string) (string, error) {
			p, ok := pageByID(id)
			if !ok {
				return "", fmt.Errorf("page inconnue : %s", id)
			}
			return p.paths[lang], nil
		},
		"asset": func(name string) (string, error) {
			r, ok := s.pages["/static/"+name]
			if !ok {
				return "", fmt.Errorf("fichier statique inconnu : %s", name)
			}
			return "/static/" + name + "?v=" + r.version, nil
		},
		// Cloudflare ne doit pas réécrire les adresses e-mail (il injecterait un script, bloqué par la CSP).
		"emailOff": func() template.HTML { return "<!--email_off-->" },
		"emailOn":  func() template.HTML { return "<!--/email_off-->" },
	}
}

func (s *Site) render(fsys fs.FS, layout, lang string, p pageDef) ([]byte, error) {
	name := "content/" + lang + "/" + p.id + ".html"
	content, err := fs.ReadFile(fsys, name)
	if err != nil {
		return nil, fmt.Errorf("page « %s » absente en « %s » : %w", p.id, lang, err)
	}
	crlf := strings.NewReplacer("\r\n", "\n")
	t, err := template.New("layout").Funcs(s.funcs(lang)).Parse(crlf.Replace(layout))
	if err != nil {
		return nil, fmt.Errorf("content/layout.html : %w", err)
	}
	if _, err := t.New("content").Parse(crlf.Replace(string(content))); err != nil {
		return nil, fmt.Errorf("%s : %w", name, err)
	}

	base := s.cfg.Get("url_site")
	other := otherLanguage(lang)
	iso := s.pageDate(p)
	date, err := time.Parse(time.DateOnly, iso)
	if err != nil {
		return nil, err
	}
	v := view{
		Lang: lang, OtherLang: other, Legal: p.legal, NotFound: p.id == notFoundID, Narrow: p.legal || p.text,
		DateISO:    iso,
		Date:       fmt.Sprintf("%d %s %d", date.Day(), months[lang][date.Month()-1], date.Year()),
		ShareImage: base + "/static/img/partage.jpg",
	}
	if v.Title, err = s.text(lang, "page."+p.id+".title"); err != nil {
		return nil, err
	}
	if v.Description, err = s.text(lang, "page."+p.id+".desc"); err != nil {
		return nil, err
	}
	if v.NotFound {
		home, _ := pageByID("home")
		v.OtherURL = home.paths[other]
	} else {
		v.OtherURL = p.paths[other]
		v.Canonical = base + p.paths[lang]
		for _, l := range languages {
			v.Alternates = append(v.Alternates, alternate{l, base + p.paths[l]})
		}
		v.Alternates = append(v.Alternates, alternate{"x-default", base + p.paths[defaultAlternate]})
	}
	nav := func(ids []string) ([]navItem, error) {
		var out []navItem
		for _, id := range ids {
			d, ok := pageByID(id)
			if !ok {
				return nil, fmt.Errorf("page inconnue : %s", id)
			}
			label, err := s.text(lang, "nav."+id)
			if err != nil {
				return nil, err
			}
			out = append(out, navItem{URL: d.paths[lang], Label: label, Current: id == p.id})
		}
		return out, nil
	}
	if v.MainNav, err = nav(mainNav); err != nil {
		return nil, err
	}
	if v.FooterNav, err = nav(footerNav); err != nil {
		return nil, err
	}

	var out bytes.Buffer
	if err := t.ExecuteTemplate(&out, "layout", v); err != nil {
		return nil, fmt.Errorf("%s : %w", name, err)
	}
	return out.Bytes(), nil
}

func (s *Site) sitemap() []byte {
	base := s.cfg.Get("url_site")
	esc := func(v string) string {
		var b bytes.Buffer
		_ = xml.EscapeText(&b, []byte(v))
		return b.String()
	}
	var b strings.Builder
	b.WriteString(xml.Header)
	b.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:xhtml="http://www.w3.org/1999/xhtml">` + "\n")
	for _, p := range pageDefs {
		for _, lang := range languages {
			fmt.Fprintf(&b, "  <url>\n    <loc>%s</loc>\n    <lastmod>%s</lastmod>\n", esc(base+p.paths[lang]), esc(s.pageDate(p)))
			for _, l := range languages {
				fmt.Fprintf(&b, `    <xhtml:link rel="alternate" hreflang="%s" href="%s"/>`+"\n", l, esc(base+p.paths[l]))
			}
			fmt.Fprintf(&b, `    <xhtml:link rel="alternate" hreflang="x-default" href="%s"/>`+"\n", esc(base+p.paths[defaultAlternate]))
			b.WriteString("  </url>\n")
		}
	}
	b.WriteString("</urlset>\n")
	return []byte(b.String())
}
