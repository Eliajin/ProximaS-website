# proximas_web : le site proximas.be

Site de présentation de ProximaS (application de rencontres de proximité, 18+) : une page d'accueil et les pages légales, en français et en anglais. Petit serveur Go (bibliothèque standard seulement) qui rend toutes les pages au démarrage et les sert depuis la mémoire. Ni base de données, ni secret, ni script, ni cookie, ni ressource externe.

Tous les textes légaux sont des **brouillons à valider par un juriste**.

## Pages

| Page | Français | Anglais |
|---|---|---|
| Présentation | `/` | `/en/` |
| Sécurité | `/securite` | `/en/safety` |
| Confidentialité | `/confidentialite` | `/en/privacy` |
| Conditions | `/conditions` | `/en/terms` |
| Suppression de compte | `/suppression-compte` | `/en/account-deletion` |
| Contact et signalement | `/contact` | `/en/contact` |
| Mentions légales | `/mentions-legales` | `/en/legal-notice` |

Aussi : 404 dans les deux langues, `/robots.txt`, `/sitemap.xml`, `/api/health`. Les adresses françaises sont utilisées par l'application et les magasins : **ne pas les changer**. Aucune redirection selon la langue ou le pays du visiteur.

## Commandes

| Commande | Effet |
|---|---|
| `make dev` | site local sur http://127.0.0.1:8098 (relancer après une modification) |
| `make champs` | liste les champs de `site.conf` encore vides |
| `make test` | format, `go vet`, tests Go, tests de `build-prod.sh`, champs manquants |
| `make docker` | build de production dans Docker, puis essai du conteneur sur 127.0.0.1:18098 |
| `make clean` | supprime `out/` |

Sans make : `go run . serve` (avec `LISTEN_ADDR=127.0.0.1:8098`), `go run . champs` (`--strict` : échoue s'il manque un champ obligatoire), `go test ./...`.

## Remplir et régler : `site.conf`

**Un seul fichier**, commenté ligne par ligne. Il contient l'identité de l'éditeur, l'hébergeur, la zone de lancement, les liens de la bêta et l'état réel de l'application (suppression, export, relecture des messages, langue de l'app). Un champ obligatoire vide s'affiche « [À REMPLIR] » / « [TO BE COMPLETED] » ; un champ facultatif vide est masqué. Le fichier est intégré au binaire : modifier, commit, push, déployer. Son contenu est public.

Les liens `lien_groupe_google` et `lien_testflight` font apparaître les boutons de la bêta (et le statut passe de « Bêta fermée » à « Bêta en cours »).

## Modifier un texte

```
content/
  layout.html          gabarit commun
  ui.json              textes courts, « fr » et « en » côte à côte (titres, descriptions, menus)
  fr/<page>.html       contenu d'une page en français
  en/<page>.html       la même page en anglais : mêmes sections, mêmes balises, mêmes identifiants
```

Règle : toute modification se fait dans les deux langues, avec la même structure. `go test` échoue si une page ou un texte manque dans une langue, si le nombre de paragraphes ou d'éléments de liste diffère, si les liens ne mènent pas aux mêmes pages ou si les nombres (durées, limites) ne sont pas les mêmes dans une section. Les tests rejouent ces contrôles avec plusieurs états de `site.conf`. La version française fait foi (mention affichée, à valider). Après un changement de texte légal : mettre à jour `date_textes`.

Fonctions utiles dans les pages : `{{champ "editeur_bce"}}` (valeur ou « [À REMPLIR] »), `{{val "..."}}`, `{{on "..."}}`, `{{mail}}`, `{{page "privacy"}}`, `{{asset "img/..."}}`, `{{t "..."}}`.

## Ce que disent les textes, et d'où cela vient

Les faits viennent du code et des documents de `../ProximaS` (lecture seule), état du 26/11/2026 : commit `c666058` (lot 5 : suppression, export, page web de suppression ; déployé) et changements suivants de l'API, stagés et **non déployés** au moment de la rédaction (empreinte de l'adresse après soupçon de minorité, rectification du prénom, de l'e-mail et de la date de naissance, iPhone seul) : **déployer l'API avant le site** (`docs/MISE_EN_LIGNE.md`). Voir aussi `docs/CONFORMITE.md`, `docs/BACKOFFICE.md` (3 bis) et `docs/REGISTRE_TRAME.md`. Quand l'application change (durées, prestataires, modération, publicité, vérification d'âge, langue), relire `content/*/privacy.html`, `terms.html`, `deletion.html`, `safety.html` et `home.html`. Réglages de l'API dont les textes dépendent : `TEXT_FILTER_PRIVATE=off`, durées de conservation par défaut (180, 365, 730, 90, 30 et 365 jours), `SIGNALS_PER_DAY=5`.

## Visuels et police

| Fichier (`static/`) | Origine (`ProximaS/design/`) | Droits |
|---|---|---|
| `img/visuel-800.jpg`, `visuel-1600.jpg`, `partage.jpg` | `marque/visuel-couple-hd.png` | image générée par IA, publication autorisée par le propriétaire ; mention « générée par IA » affichée ; personnes jamais présentées comme des utilisateurs |
| `img/icone.svg`, `icone-96.png`, `icone-180.png`, `favicon-32.png` | `marque/icone.svg`, `icone-1024.png` | icône redessinée à partir d'une planche générée par IA |
| `img/maquette-envie.png` | `ecrans/07-Envie.png` | maquette, sans personne ni donnée ; légendée comme maquette |
| `fonts/Jost-Variable.ttf` | police de l'application | SIL Open Font License 1.1 (`fonts/OFL.txt`, publié et crédité) |

## Technique

- En-têtes : CSP `default-src 'none'` (images, styles et polices du site seulement), HSTS, `nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy`, `Permissions-Policy`.
- Aucune requête journalisée. GET et HEAD seulement.
- `www.proximas.be` redirige (301) vers `https://proximas.be`.
- Les pages sont envoyées avec `Cache-Control: no-transform` et encadrées de `<!--email_off-->` : Cloudflare ne doit ni les réécrire ni y injecter de script.
- Fichiers statiques : adresse avec empreinte (`?v=`), cache d'un an.

## Déploiement

Sur le serveur, depuis `~/websites/generateprod/proximas_web` : `deploy/build-prod.sh` (options `--no-pull`, `--rollback`, `--restart`). Il construit et teste dans Docker, garde la version précédente (`~/websites/prod/proximas_web.prev`), copie vers `~/websites/prod/proximas_web`, lance `docker compose up -d --build` puis vérifie `/api/health`, `/` et `/confidentialite`. La prod n'est jamais remplacée par un build incomplet. Port 127.0.0.1:8098, conteneur en lecture seule, sans capacités, non root, journaux 10 Mo x 3. Le `.env` est facultatif (modèle : `deploy/env.example`) ; il n'y a pas de dossier `data/`.

Première mise en ligne et Cloudflare : `docs/MISE_EN_LIGNE.md`.
