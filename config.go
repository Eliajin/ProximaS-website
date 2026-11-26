package main

import (
	"bufio"
	"fmt"
	"io"
	netmail "net/mail"
	"net/url"
	"strings"
	"time"
)

// Langues du site. La premiere est la langue par defaut (URL sans prefixe), et sa version fait foi.
var languages = []string{"fr", "en"}

type fieldKind int

const (
	required fieldKind = iota // vide : « [À REMPLIR] » sur le site, listé par « champs »
	optional                  // vide : la ligne n'apparaît pas
	toggle                    // oui | non
)

type fieldSpec struct {
	key  string
	kind fieldKind
	help string
}

// fields : toutes les clés admises dans site.conf. Une clé absente de cette liste est une erreur.
var fields = []fieldSpec{
	{"url_site", required, "adresse publique du site"},
	{"date_textes", required, "date des textes légaux (AAAA-MM-JJ)"},
	{"editeur_nom", required, "nom ou dénomination de l'éditeur"},
	{"editeur_nom_commercial", required, "nom commercial"},
	{"editeur_statut_fr", required, "statut juridique (français)"},
	{"editeur_statut_en", required, "statut juridique (anglais)"},
	{"editeur_adresse", required, "adresse postale"},
	{"editeur_pays_fr", required, "pays (français)"},
	{"editeur_pays_en", required, "pays (anglais)"},
	{"editeur_bce", required, "numéro d'entreprise (BCE)"},
	{"editeur_tva", required, "numéro de TVA"},
	{"editeur_telephone", optional, "téléphone"},
	{"editeur_rpm", optional, "registre des personnes morales"},
	{"responsable_publication", required, "responsable de la publication"},
	{"contact_email", required, "adresse de contact publiée"},
	{"fournisseur_boite_contact_fr", required, "prestataire de la boîte contact@ (français)"},
	{"fournisseur_boite_contact_en", required, "prestataire de la boîte contact@ (anglais)"},
	{"dpo", required, "délégué à la protection des données, ou « aucun »"},
	{"hebergeur_nom_fr", required, "nom de l'hébergeur (français)"},
	{"hebergeur_nom_en", required, "nom de l'hébergeur (anglais)"},
	{"hebergeur_description_fr", required, "description de l'hébergement (français)"},
	{"hebergeur_description_en", required, "description de l'hébergement (anglais)"},
	{"hebergeur_responsable_technique", required, "responsable technique de l'hébergement"},
	{"zone_fr", required, "pays où le service est proposé (français)"},
	{"zone_en", required, "pays où le service est proposé (anglais)"},
	{"lien_groupe_google", optional, "lien du groupe Google des testeurs"},
	{"lien_testflight", optional, "lien public TestFlight"},
	{"lot5_serveur_deploye", toggle, "API du lot 5 déployée"},
	{"suppression_dans_app", toggle, "suppression du compte dans l'application distribuée"},
	{"export_dans_app", toggle, "export des données dans l'application distribuée"},
	{"suppression_web_url", optional, "page web de demande de suppression"},
	{"relecture_messages_prives", toggle, "TEXT_FILTER_PRIVATE=on en production"},
	{"app_en_anglais", toggle, "application disponible en anglais"},
	{"lot7_serveur_deploye", toggle, "API du lot 7 déployée"},
	{"photos_privees", toggle, "photos privées ouvertes (PRIVATE_PHOTOS=on)"},
	{"masquage_photos_age", toggle, "masquage des photos après des signalements d'âge déployé"},
}

// Config : contenu de site.conf.
type Config struct {
	values map[string]string
}

// Get renvoie la valeur d'une clé (vide si non remplie).
func (c *Config) Get(key string) string { return c.values[key] }

// On indique si un interrupteur vaut « oui ».
func (c *Config) On(key string) bool { return c.values[key] == "oui" }

// Domain : nom d'hôte de url_site.
func (c *Config) Domain() string {
	u, err := url.Parse(c.values["url_site"])
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// Missing liste les champs vides du type demandé, dans l'ordre de la liste.
func (c *Config) Missing(kind fieldKind) []fieldSpec {
	var out []fieldSpec
	for _, f := range fields {
		if f.kind == kind && c.values[f.key] == "" {
			out = append(out, f)
		}
	}
	return out
}

func knownField(key string) (fieldSpec, bool) {
	for _, f := range fields {
		if f.key == key {
			return f, true
		}
	}
	return fieldSpec{}, false
}

// ParseConfig lit et valide site.conf.
func ParseConfig(r io.Reader) (*Config, error) {
	c := &Config{values: map[string]string{}}
	sc := bufio.NewScanner(r)
	line := 0
	for sc.Scan() {
		line++
		l := strings.TrimSpace(strings.TrimPrefix(sc.Text(), "\ufeff"))
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		key, value, ok := strings.Cut(l, "=")
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if !ok {
			return nil, fmt.Errorf("site.conf ligne %d : il manque « = »", line)
		}
		if _, known := knownField(key); !known {
			return nil, fmt.Errorf("site.conf ligne %d : clé inconnue « %s »", line, key)
		}
		if _, dup := c.values[key]; dup {
			return nil, fmt.Errorf("site.conf ligne %d : clé « %s » en double", line, key)
		}
		c.values[key] = value
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return c, c.validate()
}

func (c *Config) validate() error {
	for _, f := range fields {
		v, present := c.values[f.key]
		if !present {
			return fmt.Errorf("site.conf : la clé « %s » est absente (laisse-la vide si besoin)", f.key)
		}
		if f.kind == toggle && v != "oui" && v != "non" {
			return fmt.Errorf("site.conf : « %s » doit valoir oui ou non", f.key)
		}
	}
	if u, err := url.Parse(c.values["url_site"]); err != nil || u.Scheme != "https" || u.Host == "" || u.Host != u.Hostname() ||
		u.Path != "" || u.RawQuery != "" || u.Fragment != "" || strings.HasPrefix(u.Host, "www.") {
		return fmt.Errorf("site.conf : url_site doit être de la forme https://domaine (sans www ni barre finale)")
	}
	if _, err := time.Parse(time.DateOnly, c.values["date_textes"]); err != nil {
		return fmt.Errorf("site.conf : date_textes doit être au format AAAA-MM-JJ")
	}
	email := c.values["contact_email"]
	if a, err := netmail.ParseAddress(email); err != nil || a.Address != email || strings.ContainsAny(email, "?&#%") {
		return fmt.Errorf("site.conf : contact_email doit être une adresse e-mail simple")
	}
	for _, k := range []string{"lien_groupe_google", "lien_testflight", "suppression_web_url"} {
		v := c.values[k]
		if v == "" {
			continue
		}
		if u, err := url.Parse(v); err != nil || u.Scheme != "https" || u.Host == "" {
			return fmt.Errorf("site.conf : %s doit être une adresse https complète", k)
		}
	}
	// Cohérence : le site ne doit pas annoncer une fonction que le serveur n'offre pas.
	if c.On("photos_privees") && !c.On("lot7_serveur_deploye") {
		return fmt.Errorf("site.conf : photos_privees = oui exige lot7_serveur_deploye = oui")
	}
	if !c.On("lot5_serveur_deploye") {
		for _, k := range []string{"suppression_dans_app", "export_dans_app"} {
			if c.On(k) {
				return fmt.Errorf("site.conf : %s = oui exige lot5_serveur_deploye = oui", k)
			}
		}
		if c.values["suppression_web_url"] != "" {
			return fmt.Errorf("site.conf : suppression_web_url exige lot5_serveur_deploye = oui")
		}
	}
	return nil
}

// ReportMissing écrit la liste des champs vides. Renvoie le nombre de champs obligatoires manquants.
func ReportMissing(w io.Writer, c *Config) int {
	req, opt := c.Missing(required), c.Missing(optional)
	if len(req) == 0 {
		fmt.Fprintln(w, "Champs obligatoires de site.conf : tous remplis.")
	} else {
		fmt.Fprintf(w, "Champs obligatoires à remplir dans site.conf (%d), affichés « [À REMPLIR] » sur le site :\n", len(req))
		for _, f := range req {
			fmt.Fprintf(w, "  - %-32s %s\n", f.key, f.help)
		}
	}
	if len(opt) > 0 {
		fmt.Fprintf(w, "Champs facultatifs vides (%d), masqués sur le site :\n", len(opt))
		for _, f := range opt {
			fmt.Fprintf(w, "  - %-32s %s\n", f.key, f.help)
		}
	}
	return len(req)
}
