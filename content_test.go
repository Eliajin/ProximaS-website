package main

import (
	"html"
	"io/fs"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"
)

// variants : états de site.conf sous lesquels les deux langues doivent rester synchrones.
func variants(t *testing.T) map[string]*Config {
	t.Helper()
	load := func(change map[string]string) *Config {
		c, err := embeddedConfig()
		if err != nil {
			t.Fatal(err)
		}
		for k, v := range change {
			c.values[k] = v
		}
		if err := c.validate(); err != nil {
			t.Fatal(err)
		}
		return c
	}
	empty := map[string]string{}
	full := map[string]string{}
	for _, f := range fields {
		switch f.kind {
		case toggle:
			empty[f.key] = "non"
			full[f.key] = "oui"
		default:
			empty[f.key] = ""
			full[f.key] = "valeur fictive"
		}
	}
	for _, m := range []map[string]string{empty, full} {
		m["url_site"] = "https://proximas.be"
		m["date_textes"] = "2026-11-26"
		m["contact_email"] = "contact@proximas.be"
	}
	for _, k := range []string{"lien_groupe_google", "lien_testflight", "suppression_web_url"} {
		full[k] = "https://exemple.invalid/" + k
	}
	dpoNone := map[string]string{"dpo": "aucun", "lot5_serveur_deploye": "oui"}
	return map[string]*Config{
		"actuel":          load(nil),
		"tout vide":       load(empty),
		"tout rempli":     load(full),
		"lot 5, sans dpo": load(dpoNone),
	}
}

func build(t *testing.T, c *Config) *Site {
	t.Helper()
	s, err := BuildSite(embedded, c)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func htmlOf(t *testing.T, s *Site, lang, id string) string {
	t.Helper()
	if id == notFoundID {
		return string(s.notFound[lang].body)
	}
	p, ok := pageByID(id)
	if !ok {
		t.Fatalf("page inconnue : %s", id)
	}
	return string(s.pages[p.paths[lang]].body)
}

func allIDs() []string {
	ids := []string{notFoundID}
	for _, p := range pageDefs {
		ids = append(ids, p.id)
	}
	return ids
}

func TestEveryPageExistsInEveryLanguage(t *testing.T) {
	var want []string
	for _, id := range allIDs() {
		want = append(want, id+".html")
	}
	sort.Strings(want)
	for _, lang := range languages {
		entries, err := fs.ReadDir(embedded, "content/"+lang)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, e := range entries {
			got = append(got, e.Name())
			b, err := fs.ReadFile(embedded, "content/"+lang+"/"+e.Name())
			if err != nil || len(strings.TrimSpace(string(b))) == 0 {
				t.Errorf("content/%s/%s est vide", lang, e.Name())
			}
		}
		if !slices.Equal(got, want) {
			t.Errorf("content/%s : %v, attendu %v", lang, got, want)
		}
		for _, p := range pageDefs {
			if p.paths[lang] == "" {
				t.Errorf("page %s sans adresse en %s", p.id, lang)
			}
		}
	}
}

func TestEveryInterfaceTextIsUsed(t *testing.T) {
	s := build(t, variants(t)["actuel"])
	layout, _ := fs.ReadFile(embedded, "content/layout.html")
	src := string(layout)
	for _, lang := range languages {
		for _, id := range allIDs() {
			b, _ := fs.ReadFile(embedded, "content/"+lang+"/"+id+".html")
			src += string(b)
		}
	}
	for key := range s.ui {
		if strings.HasPrefix(key, "nav.") || strings.HasPrefix(key, "page.") {
			continue
		}
		if !strings.Contains(src, `t "`+key+`"`) {
			t.Errorf("texte jamais utilisé dans content/ui.json : %s", key)
		}
	}
}

var (
	mainRe    = regexp.MustCompile(`(?s)<main\b[^>]*>(.*)</main>`)
	tagRe     = regexp.MustCompile(`<(/?[a-zA-Z][a-zA-Z0-9]*)([^>]*)>`)
	idRe      = regexp.MustCompile(`\sid="([^"]*)"`)
	hrefRe    = regexp.MustCompile(`\shref="([^"]*)"`)
	numberRe  = regexp.MustCompile(`\d+`)
	stripRe   = regexp.MustCompile(`<[^>]*>`)
	sectionRe = regexp.MustCompile(`(?s)<section id="([^"]*)"[^>]*>(.*?)</section>`)
	commentRe = regexp.MustCompile(`(?s)<!--.*?-->`)
)

func mainOf(t *testing.T, page string) string {
	t.Helper()
	m := mainRe.FindStringSubmatch(page)
	if m == nil {
		t.Fatal("pas de balise <main>")
	}
	return m[1]
}

// normalizeLink rend comparables les liens des deux langues.
func normalizeLink(lang, href string) string {
	target, anchor, _ := strings.Cut(href, "#")
	for _, p := range pageDefs {
		if p.paths[lang] == target {
			return "page:" + p.id + "#" + anchor
		}
	}
	switch {
	case strings.HasPrefix(href, "#"):
		return href
	case strings.HasPrefix(href, "mailto:"):
		to, _, _ := strings.Cut(href, "?")
		return to
	case strings.HasPrefix(href, "/static/"):
		return href
	case strings.HasPrefix(href, "https://"):
		return "externe"
	}
	return "INCONNU " + href
}

// skeleton : suite des balises, identifiants et destinations. Un paragraphe, un élément de liste ou une
// section ajouté dans une seule langue la fait diverger.
func skeleton(lang, fragment string) []string {
	var out []string
	for _, m := range tagRe.FindAllStringSubmatch(fragment, -1) {
		line := m[1]
		if id := idRe.FindStringSubmatch(m[2]); id != nil {
			line += " id=" + id[1]
		}
		if h := hrefRe.FindStringSubmatch(m[2]); h != nil {
			line += " " + normalizeLink(lang, h[1])
		}
		if strings.Contains(m[2], `class="manque"`) {
			line += " champ-manquant"
		}
		// L'attribut lang du slogan anglais n'a de sens que sur la page française.
		out = append(out, line)
	}
	return out
}

// numbers : les nombres (durées, limites, articles) de chaque section doivent être les mêmes.
func numbers(fragment string) map[string]string {
	out := map[string]string{}
	for _, m := range sectionRe.FindAllStringSubmatch(fragment, -1) {
		text := html.UnescapeString(stripRe.ReplaceAllString(m[2], " "))
		n := numberRe.FindAllString(text, -1)
		sort.Strings(n)
		out[m[1]] = strings.Join(n, " ")
	}
	return out
}

func TestLanguagesStaySynchronized(t *testing.T) {
	for name, c := range variants(t) {
		s := build(t, c)
		for _, id := range allIDs() {
			ref := mainOf(t, htmlOf(t, s, languages[0], id))
			for _, lang := range languages[1:] {
				got := mainOf(t, htmlOf(t, s, lang, id))
				a, b := skeleton(languages[0], ref), skeleton(lang, got)
				for i := 0; i < max(len(a), len(b)); i++ {
					x, y := "(fin)", "(fin)"
					if i < len(a) {
						x = a[i]
					}
					if i < len(b) {
						y = b[i]
					}
					if x != y {
						t.Errorf("[%s] page %s : structure différente à la balise %d : %s « %s », %s « %s »",
							name, id, i+1, languages[0], x, lang, y)
						break
					}
				}
				na, nb := numbers(ref), numbers(got)
				for sec, v := range na {
					if nb[sec] != v {
						t.Errorf("[%s] page %s, section %s : nombres différents : %s « %s », %s « %s »",
							name, id, sec, languages[0], v, lang, nb[sec])
					}
				}
			}
		}
	}
}

func TestPagesAreSoundAndSelfContained(t *testing.T) {
	external := regexp.MustCompile(`(?i)(<script|<iframe|<form|<object|<embed|\sstyle="|\son[a-z]+="|\ssrc="(https?:)?//|\ssrcset="(https?:)?//|<link rel="(stylesheet|icon|preload|preconnect|dns-prefetch)[^>]*href="(https?:)?//)`)
	imgRe := regexp.MustCompile(`<img\b[^>]*>`)
	srcRe := regexp.MustCompile(`\s(?:src|href)="(/static/[^"?]*)`)
	for name, c := range variants(t) {
		s := build(t, c)
		ids := map[string]map[string]bool{}
		for path, r := range s.pages {
			if !strings.HasPrefix(r.ctype, "text/html") {
				continue
			}
			ids[path] = map[string]bool{}
			for _, m := range idRe.FindAllStringSubmatch(string(r.body), -1) {
				if ids[path][m[1]] {
					t.Errorf("[%s] %s : identifiant « %s » en double", name, path, m[1])
				}
				ids[path][m[1]] = true
			}
		}
		for _, lang := range languages {
			for _, id := range allIDs() {
				page := htmlOf(t, s, lang, id)
				where := "[" + name + "] " + lang + "/" + id
				if !strings.Contains(page, `<html lang="`+lang+`">`) {
					t.Errorf("%s : attribut lang", where)
				}
				if n := strings.Count(page, "<h1"); n != 1 {
					t.Errorf("%s : %d titres h1", where, n)
				}
				if m := external.FindString(page); m != "" {
					t.Errorf("%s : contenu interdit par la CSP ou ressource externe : %s", where, m)
				}
				if !strings.Contains(page, "<!--email_off-->") || !strings.Contains(page, "<!--/email_off-->") {
					t.Errorf("%s : protection contre la réécriture des e-mails absente", where)
				}
				if strings.Contains(commentRe.ReplaceAllString(page, ""), "ZgotmplZ") || strings.Contains(page, "<no value>") {
					t.Errorf("%s : valeur de gabarit rejetée", where)
				}
				for _, img := range imgRe.FindAllString(page, -1) {
					if !strings.Contains(img, " alt=") || !strings.Contains(img, " width=") {
						t.Errorf("%s : image sans alt ou sans dimensions : %s", where, img)
					}
				}
				for _, m := range srcRe.FindAllStringSubmatch(page, -1) {
					if s.pages[m[1]] == nil {
						t.Errorf("%s : fichier absent : %s", where, m[1])
					}
				}
				self := ""
				if p, ok := pageByID(id); ok {
					self = p.paths[lang]
				}
				for _, m := range hrefRe.FindAllStringSubmatch(page, -1) {
					href := m[1]
					if strings.HasPrefix(href, "https://") || strings.HasPrefix(href, "mailto:") || strings.HasPrefix(href, "/static/") {
						continue
					}
					target, anchor, _ := strings.Cut(href, "#")
					if target == "" {
						target = self
					}
					if href == "#contenu" {
						continue
					}
					known, ok := ids[target]
					if !ok || (anchor != "" && !known[anchor]) {
						t.Errorf("%s : lien interne cassé : %s", where, href)
					}
				}
				draft := strings.Contains(page, `class="draft"`)
				p, _ := pageByID(id)
				if draft != p.legal {
					t.Errorf("%s : bandeau brouillon = %v", where, draft)
				}
			}
		}
		css := string(s.pages["/static/style.css"].body)
		if strings.Contains(css, "http://") || strings.Contains(css, "https://") || strings.Contains(css, "@import") {
			t.Error("style.css charge une ressource externe")
		}
	}
}

func TestMissingFieldsAreShownNeverInvented(t *testing.T) {
	v := variants(t)
	empty := build(t, v["tout vide"])
	for _, lang := range languages {
		page := htmlOf(t, empty, lang, "legal")
		if n := strings.Count(page, missingMark[lang]); n < 8 {
			t.Errorf("%s : %d champs signalés manquants sur les mentions légales", lang, n)
		}
		for other, mark := range missingMark {
			if other != lang && strings.Contains(page, mark) {
				t.Errorf("%s : marqueur de l'autre langue (%s)", lang, mark)
			}
		}
	}
	full := build(t, v["tout rempli"])
	for _, lang := range languages {
		for _, id := range allIDs() {
			if strings.Contains(htmlOf(t, full, lang, id), `class="manque"`) {
				t.Errorf("%s/%s : champ affiché manquant alors que tout est rempli", lang, id)
			}
		}
	}
}

func TestTextsFollowConfiguration(t *testing.T) {
	v := variants(t)
	off, on := build(t, v["tout vide"]), build(t, v["tout rempli"])
	check := func(s *Site, lang, id, needle string, want bool) {
		t.Helper()
		if got := strings.Contains(htmlOf(t, s, lang, id), needle); got != want {
			t.Errorf("%s/%s : présence de « %s » = %v, attendu %v", lang, id, needle, got, want)
		}
	}
	const web = "https://exemple.invalid/suppression_web_url"
	check(off, "fr", "deletion", web, false)
	check(off, "fr", "deletion", "14 jours", false)
	check(off, "fr", "deletion", "mailto:contact@proximas.be", true)
	check(off, "en", "deletion", "14 days", false)
	check(on, "fr", "deletion", web, true)
	check(on, "en", "deletion", web, true)
	check(on, "fr", "deletion", "14 jours", true)
	check(on, "fr", "deletion", "mailto:contact@proximas.be", true)
	check(off, "fr", "home", "lien_groupe_google", false)
	check(off, "fr", "home", "testflight", false)
	check(off, "fr", "home", "Bêta fermée", true)
	check(on, "fr", "home", "https://exemple.invalid/lien_groupe_google", true)
	check(on, "en", "home", "https://exemple.invalid/lien_testflight", true)
	check(off, "fr", "privacy", "ne sont pas relus automatiquement", true)
	check(on, "fr", "privacy", "ne sont pas relus automatiquement", false)
	check(off, "en", "home", "in French only", true)
	check(on, "en", "home", "in French only", false)
}

func TestNoEmptyPromises(t *testing.T) {
	s := build(t, variants(t)["actuel"])
	forbidden := []string{"jamais de publicité", "aucune publicité, jamais", "never show ads", "ad-free forever",
		"100 % sûr", "100% safe", "sans faux profils", "no fake profiles", "play.google.com", "apps.apple.com", "ec.europa.eu/consumers/odr"}
	for _, lang := range languages {
		for _, id := range allIDs() {
			page := strings.ToLower(htmlOf(t, s, lang, id))
			for _, f := range forbidden {
				if strings.Contains(page, strings.ToLower(f)) {
					t.Errorf("%s/%s : formulation à éviter : %s", lang, id, f)
				}
			}
		}
	}
}
