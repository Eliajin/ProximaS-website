# proximas_web : site proximas.be

Site de présentation de ProximaS (rencontres de proximité, 18+). Lire `README.md` puis `PRESENTATION.md`. Répondre en français, tutoiement.

## Règles

- **Deux langues, toujours** : français aux adresses existantes (utilisées par l'app et les magasins, ne pas les changer), anglais sous `/en/`. Toute modification de texte se fait dans les deux, avec la même structure (`go test` l'impose). La version française fait foi. Aucune redirection selon la langue ou la géolocalisation.
- **Lancement en Europe** : Union européenne, Islande, Norvège, Liechtenstein, Suisse ; Royaume-Uni plus tard. La liste des pays n'est écrite que dans `site.conf` (`zone_fr`, `zone_en` ; le passage sur la Suisse de la confidentialité est à part). Ne pas annoncer de disponibilité déjà effective : l'app est en bêta.
- **Textes légaux** : brouillons, marqués « à valider par un juriste » ; ils décrivent le fonctionnement **réel** (vérifier dans `../ProximaS`, en lecture seule). Une fonction non déployée n'est pas annoncée : utiliser les interrupteurs de `site.conf`.
- **Rien d'inventé** : une information inconnue reste vide dans `site.conf` (affichée « [À REMPLIR] »). Aucun chiffre, témoignage ou promesse de sécurité absolue ; ne pas promettre « jamais de publicité ».
- Visuels : seulement ceux de `ProximaS/design/` ; images générées par IA signalées comme telles, jamais présentées comme des utilisateurs.
- Sans cookies, traceur, script ni ressource externe (donc pas de bandeau de consentement). Si cela change, mettre à jour la CSP et la politique de confidentialité d'abord.
- Contact publié : `contact@proximas.be`.

## Méthode

`make test` et `make docker` avant de dire « prêt ». Déploiement : `deploy/build-prod.sh` (lancé par le propriétaire), port 127.0.0.1:8098. Jamais de commit ni de push : `git add` seulement.
