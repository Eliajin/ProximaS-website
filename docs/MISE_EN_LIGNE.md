# Mise en ligne de proximas.be

Rien de ce document n'a été exécuté sur le serveur ni dans Cloudflare : à lancer par le propriétaire. Les intitulés des menus GitHub et Cloudflare changent et n'ont pas été vérifiés dans les tableaux de bord.

Relevé DNS public du 26/11/2026 (lecture seule) : zone `proximas.be` chez Cloudflare ; **aucun** enregistrement sur `proximas.be` (racine) ni `www` ; MX vers Cloudflare Email Routing (`route1` à `route3.mx.cloudflare.net`) ; TXT SPF (Cloudflare et Brevo), code Brevo et `_dmarc` ; `api.proximas.be` existe, `admin.proximas.be` pas encore.

## 0. Avant

1. `make champs` : remplir dans `site.conf` ce qui manque (voir le README).
2. Vérifier que `contact@proximas.be` reçoit vraiment : Cloudflare > zone `proximas.be` > Email > Email Routing > une règle pour `contact@` vers une boîte relevée tous les jours (cette boîte est à nommer dans `fournisseur_boite_contact_fr` et `_en`). Envoyer un message d'essai. Cette adresse sert aux demandes de suppression, aux droits RGPD et aux signalements.
3. Sur le serveur, vérifier que les messages privés ne sont pas relus automatiquement (le site l'affirme) :

```bash
grep -c '^TEXT_FILTER_PRIVATE=off$' ~/websites/prod/proximas/.env
```

Attendu : `1`. Si `0`, la valeur par défaut de l'API est `on` : ajouter `TEXT_FILTER_PRIVATE=off` au `.env` puis `cd ~/websites/prod/proximas && docker compose up -d`, **ou** mettre `relecture_messages_prives = oui` dans `site.conf`.

## 1. Dépôt GitHub (privé)

Sur GitHub : créer un dépôt **privé**, vide (sans README, `.gitignore` ni licence). Noter son adresse.

Sur le PC (PowerShell), après avoir lu les fichiers :

```powershell
cd C:\Users\Eliajin\Documents\testAvecClaude\proximas_web
git status
git ls-files --stage deploy
git commit -m "<message proposé>"
git remote add origin <URL_DU_DEPOT_PRIVE>
git push -u origin main
```

Attendu : `git ls-files --stage deploy` montre `100755` devant `build-prod.sh`, `smoke.sh` et `test-build-prod.sh` ; le push crée la branche `main`.

## 2. Clone sur le serveur

```bash
mkdir -p ~/websites/generateprod
cd ~/websites/generateprod
git clone <URL_DU_DEPOT_PRIVE> proximas_web
```

Attendu : « Cloning into 'proximas_web'... » puis aucun message d'erreur. Le dépôt étant privé, le serveur doit y avoir accès, de la même façon que pour tes autres dépôts privés. S'il n'en a pas, une clé de déploiement en lecture seule suffit :

```bash
ssh-keygen -t ed25519 -f ~/.ssh/proximas_web_deploy -N ''
cat ~/.ssh/proximas_web_deploy.pub      # clé PUBLIQUE : GitHub > dépôt > Settings > Deploy keys (sans droit d'écriture)
printf '\nHost github-proximas-web\n  HostName github.com\n  User git\n  IdentityFile ~/.ssh/proximas_web_deploy\n  IdentitiesOnly yes\n' >> ~/.ssh/config
chmod 600 ~/.ssh/config
git clone git@github-proximas-web:<COMPTE>/<DEPOT>.git ~/websites/generateprod/proximas_web
```

## 3. Premier déploiement

```bash
ss -ltn | grep ':8098' ; echo "(rien au-dessus = port libre)"
cd ~/websites/generateprod/proximas_web
deploy/build-prod.sh --no-pull
```

Attendu : le build Docker (tests compris, puis la liste des champs vides), le démarrage du conteneur `proximas-web`, puis :

```
Sante OK : {"champs_manquants":N,"status":"ok","version":"..."}
Deploiement termine.
```

`N` est le nombre de champs obligatoires encore vides (le déploiement fonctionne quand même). Aucun `.env` n'est nécessaire. Les fois suivantes : `deploy/build-prod.sh` seul (il fait le `git pull`). Retour arrière : `deploy/build-prod.sh --rollback`.

```bash
curl -s http://127.0.0.1:8098/api/health
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:8098/confidentialite      # 200
curl -s -o /dev/null -w '%{http_code} %{redirect_url}\n' -H 'Host: www.proximas.be' http://127.0.0.1:8098/contact
# attendu : 301 https://proximas.be/contact
```

## 4. Cloudflare

**Ne pas toucher** : les enregistrements MX, les TXT (SPF, code Brevo, `_dmarc`), les enregistrements DKIM, `api` et `admin`. On n'ajoute que `proximas.be` (racine, notée `@`) et `www`.

D'abord savoir comment le tunnel est géré (utiliser la même méthode que pour `api.proximas.be`) :

```bash
systemctl cat cloudflared | grep ExecStart
journalctl -u cloudflared -n 200 --no-pager | grep -i "updated to new configuration" | tail -1
```

- `--token` dans `ExecStart`, ou des lignes « updated to new configuration » : tunnel géré dans le tableau de bord, variante A. La configuration distante **remplace** alors tout `config.yml`.
- Sinon (`--config`, fichier `config.yml`) : variante B.

D'après les notes d'Orbite et de TouchePaMa, les deux façons ont été rencontrées sur ce serveur : vérifier plutôt que supposer.

### Variante A : tableau de bord

Zero Trust > Networks > Tunnels > le tunnel qui sert `api.proximas.be` > Edit > Public Hostname (ou « Published application routes ») > Add :

| Sous-domaine | Domaine | Chemin | Type | URL |
|---|---|---|---|---|
| (vide) | `proximas.be` | (vide) | HTTP | `localhost:8098` |
| `www` | `proximas.be` | (vide) | HTTP | `localhost:8098` |

- Ne pas renseigner « HTTP Host Header » : le site a besoin du nom demandé pour rediriger `www`.
- Si une règle joker du même domaine existe, ces deux règles doivent être au-dessus.
- Le tableau de bord crée normalement les enregistrements DNS. Vérifier dans la zone `proximas.be` > DNS : `CNAME @` et `CNAME www` vers `<ID_DU_TUNNEL>.cfargotunnel.com`, proxy activé. Sinon les créer à la main (la racine accepte un CNAME chez Cloudflare).

### Variante B : fichier `config.yml`

```bash
sudo cp /etc/cloudflared/config.yml /etc/cloudflared/config.yml.bak-proximas-web
sudo nano /etc/cloudflared/config.yml
```

Ajouter **avant** la règle finale (`- service: http_status:404`), sans modifier les autres :

```yaml
  - hostname: proximas.be
    service: http://localhost:8098
  - hostname: www.proximas.be
    service: http://localhost:8098
```

```bash
sudo cloudflared tunnel --config /etc/cloudflared/config.yml ingress validate                                     # OK
sudo cloudflared tunnel --config /etc/cloudflared/config.yml ingress rule https://proximas.be | grep -i service       # localhost:8098
sudo cloudflared tunnel --config /etc/cloudflared/config.yml ingress rule https://www.proximas.be | grep -i service   # localhost:8098
sudo systemctl restart cloudflared && systemctl is-active cloudflared                                                 # active
```

Puis les enregistrements DNS.

> **Attention, piège connu sur `pamaelia-001`** (noté dans le README d'Orbite) : le certificat de `cloudflared` y est lié à la zone `lasdestoits.be`. `cloudflared tunnel route dns` crée alors `proximas.be.lasdestoits.be` au lieu de `proximas.be`. Dans ce cas, **ne pas utiliser la commande** : créer à la main dans Cloudflare > zone `proximas.be` > DNS les deux CNAME `@` et `www` vers `<ID_DU_TUNNEL>.cfargotunnel.com`, proxy activé.

Seulement si le certificat correspond bien à la zone `proximas.be` :

```bash
cloudflared tunnel list
cloudflared tunnel route dns <NOM_DU_TUNNEL> proximas.be
cloudflared tunnel route dns <NOM_DU_TUNNEL> www.proximas.be
```

Attendu : « Added CNAME proximas.be which will route to this tunnel » (le nom affiché doit se terminer exactement par `proximas.be`). Si un autre suffixe apparaît, supprimer l'enregistrement créé dans la mauvaise zone et passer par le tableau de bord.

### Redirection `www`

Elle est faite par le site lui-même (301 vers `https://proximas.be`, chemin conservé) : aucune règle de redirection Cloudflare à créer.

### Réglages de la zone à vérifier (le site annonce « aucun cookie, aucun script »)

- Security > Bots : « Bot Fight Mode » désactivé (il pose un cookie et injecte un script).
- Analytics & Logs > Web Analytics : pas d'injection automatique sur `proximas.be`.
- Speed : « Rocket Loader » désactivé.
- Scrape Shield : « Email Address Obfuscation » peut rester (les pages demandent à Cloudflare de ne pas les réécrire).
- SSL/TLS : ne rien changer si `api.proximas.be` fonctionne ; « Always Use HTTPS » activé.
- Aucune règle « Cache Everything » qui couvrirait aussi `api.proximas.be`.

## 5. Vérifier de l'extérieur

```bash
curl -s https://proximas.be/api/health                                                    # {"champs_manquants":N,"status":"ok",...}
curl -s -o /dev/null -w '%{http_code}\n' https://proximas.be/suppression-compte           # 200
curl -s -o /dev/null -w '%{http_code}\n' https://proximas.be/en/privacy                   # 200
curl -s -o /dev/null -w '%{http_code} %{redirect_url}\n' https://www.proximas.be/contact  # 301 https://proximas.be/contact
curl -sI https://proximas.be/ | grep -iE 'content-security|strict-transport|set-cookie'   # les deux premiers, AUCUN set-cookie
curl -s https://api.proximas.be/api/health                                                # inchangé
```

Sur le PC, les e-mails ne doivent pas avoir bougé :

```powershell
Resolve-DnsName proximas.be -Type MX
Resolve-DnsName proximas.be -Type TXT
```

Dans un navigateur (ordinateur et téléphone) : ouvrir chaque page dans les deux langues ; l'adresse `contact@proximas.be` doit s'afficher en clair (et non « [email protected] ») ; outils de développement > Application > Cookies : vide.

Puis renseigner les adresses dans les consoles des magasins : confidentialité `https://proximas.be/confidentialite`, suppression `https://proximas.be/suppression-compte`, assistance `https://proximas.be/contact`.

## Ordre de déploiement : l'API d'abord

La version du site qui décrit le lot 6 (« A vu ton profil » et « Visiter discrètement », prénom modifiable dans l'application, 3 envies, signal « passé » renvoyable après 24 h, grille) annonce des fonctions de l'API **pas encore déployées** au moment de sa rédaction (26/11/2026). Aucun interrupteur ne les masque : déployer l'API, puis le site.

Même règle pour les textes sur le mode invisible (26/11/2026) : fiche et photos encore ouvertes aux conversations et aux personnes ayant reçu un signal en attente, et aucune distance en conversation quand l'une des deux personnes est invisible. Déployer la version de l'API qui contient ces deux changements **avant** le site (`ProximaS/docs/SPEC_TECHNIQUE.md`, 4.5).

La liste des visiteurs, le réglage « Visiter discrètement », le prénom modifiable et la grille demandent aussi la nouvelle version de l'application. Avant le déploiement du site, faire valider (juriste) la base légale des visites de profil : `ProximaS/docs/CONFORMITE.md`, section 4, demande de la valider avant d'annoncer la fonction.

À vérifier avant de lancer `deploy/build-prod.sh` du site :

- `curl -s -o /dev/null -w '%{http_code}\n' https://api.proximas.be/api/v1/me/visitors` répond `401` (la route existe et demande une connexion) ; `404` : l'API du lot 6 n'est pas déployée ;
- `curl -s -o /dev/null -w '%{http_code}\n' https://api.proximas.be/confirmation-adresse` répond `400` (lien non valide : la page de confirmation d'une nouvelle adresse existe) ; `404` : l'API n'est pas à jour ;
- la rectification demande un compte de modération de rôle **admin** ;
- tant que le juriste n'a pas validé la consultation d'un justificatif d'identité, elle ne doit pas être utilisée (`ProximaS/docs/BACKOFFICE.md`, 3 bis).

## Quand le lot 5 est déployé (ProximaS)

Les textes suivent `site.conf`. Ne passer à `oui` que ce qui est réellement en service.

| Moment | Réglage |
|---|---|
| API du lot 5 déployée | `lot5_serveur_deploye = oui` et `suppression_web_url = https://api.proximas.be/suppression-compte` (l'ouvrir d'abord : le formulaire doit s'afficher et l'e-mail arriver) |
| Version de l'app avec la suppression distribuée aux testeurs | `suppression_dans_app = oui` |
| Version de l'app avec l'export distribuée | `export_dans_app = oui` |

Puis mettre à jour `date_textes`, `make test`, commit, push, et `deploy/build-prod.sh` sur le serveur. Relire `/suppression-compte` en ligne.

**Tant que l'export dans l'application n'est pas distribué**, une demande de copie des données reçue par e-mail doit être traitée à la main dans le mois : l'API n'a pas de commande pour cela. Une demande de suppression par e-mail peut être renvoyée vers le formulaire web.

## Lot « Europe, français et anglais » et signalements d'âge

| Moment | Action |
|---|---|
| API du lot Europe déployée (`ProximaS`, commit `7440c3d`) **et** application bilingue distribuée aux testeurs | publier le site (les textes la décrivent, `app_en_anglais = oui`) |
| Avant de distribuer cette application | les six pages `/en/…` qu'elle ouvre doivent être en ligne : `safety`, `privacy`, `terms`, `account-deletion`, `contact`, `legal-notice` |
| API qui masque les photos au-delà de 5 signalements d'âge déployée | vérifier `MINOR_REPORTS_TO_HIDE` (6 par défaut) dans le `.env` de l'API, puis `masquage_photos_age = oui` |

L'application bilingue a besoin des pages anglaises, et les textes ont besoin de l'API : déployer l'API, puis le site, puis distribuer l'application.

## Lot 8 (favoris, « vu », saisie en cours, mise à jour)

Textes écrits mais masqués par `lot8_serveur_deploye`. Ordre :

1. déployer l'API (`ProximaS`, commit `29f48f3`) ;
2. dans `site.conf`, `lot8_serveur_deploye = oui`, mettre à jour `date_textes`, `make test`, commit, push, `deploy/build-prod.sh` du site ;
3. relire `/confidentialite#favoris`, `/confidentialite#vu` et leurs équivalents anglais ;
4. **puis seulement** distribuer le build 2 de l'application.

À faire valider (juriste) : bases légales des favoris, de « Vu » et d'« en train d'écrire » et leur activation par défaut, envois de photos suspendus vers un compte dont l'âge est signalé, information dans l'application ou nouvelle version des conditions.

## Lot 7 (photos privées, conditions qui changent)

Les textes sont écrits mais masqués. Dans `site.conf` :

| Moment | Réglage |
|---|---|
| API du lot 7 déployée | `lot7_serveur_deploye = oui` |
| Photos privées ouvertes, après ta décision (`/api/health` montre `"private_photos":"on"`) | `photos_privees = oui` |

Attention : en bêta, les photos privées sont **ouvertes par défaut** dès le déploiement de l'API (`PRIVATE_PHOTOS`). Pour ne pas les ouvrir, mettre `PRIVATE_PHOTOS=off` dans le `.env` de l'API avant de la déployer.

Ordre : déployer l'API, passer les réglages à « oui », mettre à jour `date_textes`, `make test`, commit, push, `deploy/build-prod.sh` du site, **puis seulement** distribuer l'application et, le cas échéant, publier une information ou une nouvelle version dans l'application (`ProximaS/docs/MISE_A_JOUR_CONDITIONS.md`). Les versions remplacées sont archivées sur le site (voir « Nouvelle version des conditions » ci-dessous).

À faire valider avant d'ouvrir (juriste) : base légale des photos privées (article 9), copie gardée 30 jours, export restreint, procédure « contenu impliquant un mineur », section « Protection des enfants » (`/conditions#enfants`) et son point de contact.

## Nouvelle version des conditions (contenu explicite, orientation sur la fiche)

Ordre (`ProximaS/docs/MISE_A_JOUR_CONDITIONS.md`) : déployer le site (`deploy/build-prod.sh`), vérifier `/conditions`, `/conditions/version-1` et leurs équivalents anglais, **puis aussitôt** déployer l'API qui publie la nouvelle version à accepter et qui affiche l'orientation sur la fiche, puis distribuer l'application sans la case « mesure d'audience ». Entre les deux, le site annonce des changements pas encore en service : ne pas laisser traîner.

Archives : `content/<langue>/terms-v1.html` et `privacy-v1.html` sont le texte qui était en ligne le 26/11/2026 avant cette mise à jour (relevé sur proximas.be), à ne plus modifier. À la prochaine version à accepter : **avant** de modifier les textes, figer de la même façon les pages en ligne sous `-v2` et les déclarer dans `pageDefs` (`site.go`), `content/ui.json` et `deploy/smoke.sh`.
