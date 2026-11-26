package main

import (
	"bytes"
	"strings"
	"testing"
)

func confText(t *testing.T) string {
	t.Helper()
	raw, err := embedded.ReadFile("site.conf")
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(string(raw), "\r\n", "\n")
}

// setLine remplace la ligne « clé = ... » de site.conf.
func setLine(t *testing.T, text, key, value string) string {
	t.Helper()
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		if k, _, ok := strings.Cut(l, "="); ok && strings.TrimSpace(k) == key {
			lines[i] = key + " = " + value
			return strings.Join(lines, "\n")
		}
	}
	t.Fatalf("clé %s absente de site.conf", key)
	return ""
}

func TestSiteConfIsValidAndComplete(t *testing.T) {
	c, err := ParseConfig(strings.NewReader(confText(t)))
	if err != nil {
		t.Fatal(err)
	}
	if c.Domain() != "proximas.be" {
		t.Errorf("domaine inattendu : %s", c.Domain())
	}
	if len(c.values) != len(fields) {
		t.Errorf("%d clés dans site.conf, %d déclarées", len(c.values), len(fields))
	}
}

func TestParseConfigRejects(t *testing.T) {
	base := confText(t)
	// Base « lot 5 non déployé », quel que soit l'état réel écrit dans site.conf.
	noLot := setLine(t, setLine(t, base, "lot5_serveur_deploye", "non"), "suppression_web_url", "")
	noLot = setLine(t, setLine(t, noLot, "suppression_dans_app", "non"), "export_dans_app", "non")
	if _, err := ParseConfig(strings.NewReader(noLot)); err != nil {
		t.Fatalf("base sans lot refusée : %v", err)
	}
	cases := map[string]string{
		"photos sans lot 7":        setLine(t, setLine(t, base, "lot7_serveur_deploye", "non"), "photos_privees", "oui"),
		"clé inconnue":             base + "\ninconnue = 1\n",
		"clé en double":            base + "\ndpo = aucun\n",
		"ligne sans égal":          base + "\nabc\n",
		"clé absente":              strings.Replace(base, "\ndpo =", "\n#dpo =", 1),
		"interrupteur invalide":    setLine(t, base, "app_en_anglais", "peut-être"),
		"url_site en http":         setLine(t, base, "url_site", "http://proximas.be"),
		"url_site avec www":        setLine(t, base, "url_site", "https://www.proximas.be"),
		"url_site avec chemin":     setLine(t, base, "url_site", "https://proximas.be/"),
		"date invalide":            setLine(t, base, "date_textes", "26/11/2026"),
		"e-mail invalide":          setLine(t, base, "contact_email", "contact"),
		"e-mail avec paramètres":   setLine(t, base, "contact_email", "a?cc=b@example.invalid"),
		"lien non https":           setLine(t, base, "lien_groupe_google", "javascript:alert(1)"),
		"suppression web sans lot": setLine(t, noLot, "suppression_web_url", "https://api.proximas.be/suppression-compte"),
		"app sans lot":             setLine(t, noLot, "suppression_dans_app", "oui"),
		"export sans lot":          setLine(t, noLot, "export_dans_app", "oui"),
	}
	for name, text := range cases {
		if _, err := ParseConfig(strings.NewReader(text)); err == nil {
			t.Errorf("%s : accepté à tort", name)
		}
	}
}

func TestReportMissing(t *testing.T) {
	text := setLine(t, confText(t), "editeur_bce", "")
	c, err := ParseConfig(strings.NewReader(text))
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if n := ReportMissing(&out, c); n < 1 || !strings.Contains(out.String(), "editeur_bce") {
		t.Fatalf("champ manquant non signalé : %d\n%s", n, out.String())
	}
	for _, f := range fields {
		if f.kind != toggle {
			c.values[f.key] = "x"
		}
	}
	out.Reset()
	if n := ReportMissing(&out, c); n != 0 || !strings.Contains(out.String(), "tous remplis") {
		t.Fatalf("aucun champ ne devrait manquer : %d\n%s", n, out.String())
	}
}

func TestCommands(t *testing.T) {
	var out bytes.Buffer
	if code := run([]string{"champs"}, &out); code != 0 {
		t.Errorf("champs : code %d (il ne doit jamais bloquer le déploiement)", code)
	}
	c, err := embeddedConfig()
	if err != nil {
		t.Fatal(err)
	}
	want := 0
	if len(c.Missing(required)) > 0 {
		want = 1
	}
	if code := run([]string{"champs", "--strict"}, &out); code != want {
		t.Errorf("champs --strict : code %d, attendu %d", code, want)
	}
	if code := run([]string{"autre"}, &out); code != 2 {
		t.Errorf("commande inconnue : code %d", code)
	}
}
