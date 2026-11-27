# proximas.be : le site de ProximaS

Page de présentation honnête de l'application (concept, Mon ciel, envie du moment, signaux, messagerie, sécurité, vie privée, bêta) et pages demandées par la loi et les magasins : mentions légales, confidentialité, conditions, suppression de compte, sécurité, contact et signalement. Français et anglais.

## État (26/11/2026)

- Première version prête en local, **pas encore en ligne** (`docs/MISE_EN_LIGNE.md`).
- Textes légaux : brouillons, non relus par un juriste.
- Réglages actuels (`site.conf`) : API du lot 5 déployée (suppression demandable par le web), suppression et export dans l'application annoncés, lot 7 et photos privées annoncés, lien du groupe Google des testeurs, application annoncée en français et en anglais.
- Les textes décrivent le lot 6 de l'API (visites de profil, prénom modifiable, 3 envies, délais des signaux, grille) : déployer le site après elle.
- Textes du lot « Europe, français et anglais » : publier le site après l'API correspondante (voir `docs/MISE_EN_LIGNE.md`).
- 27/11 : masquage des photos après des signalements d'âge et retrait de contenus (texte conservé) décrits (API `0c83f24` déployée).
- Lot 8 (favoris, « vu », saisie en cours, mise à jour, export élargi) : textes prêts, masqués par `lot8_serveur_deploye` ; à passer à « oui » après le déploiement de l'API, avant de distribuer le build 2.
- Champs à remplir : `make champs`.

## Pile

Go (bibliothèque standard), HTML et CSS, Docker, Cloudflared. Port 127.0.0.1:8098.

## Pistes d'amélioration

1. Faire relire les textes légaux (RGPD art. 9, DSA, droit de la consommation, Suisse), puis retirer ou adapter le bandeau « brouillon ».
2. Remplacer la maquette par de vraies captures de l'application (profils fictifs).
3. Visuel et icône définitifs, avec droits documentés ; police en WOFF2 pour alléger la page.
4. Ajouter les liens des magasins quand l'application y sera publiée.
5. Page d'aide (questions fréquentes) si les mêmes demandes reviennent à l'adresse de contact.
6. Autres langues : ajouter la langue dans `config.go`, `ui.json`, `content/<langue>/` et les adresses dans `site.go`.
