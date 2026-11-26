#!/usr/bin/env bash
# Deploiement de proximas.be. A lancer par le proprietaire, sur le serveur, depuis le clone
# ~/websites/generateprod/proximas_web :
#
#   deploy/build-prod.sh              git pull + build (avec les tests) + mise en prod
#   deploy/build-prod.sh --no-pull    sans git pull (build de la copie actuelle)
#   deploy/build-prod.sh --rollback   revient a la version precedente (proximas_web.prev)
#   deploy/build-prod.sh --restart    recree le conteneur en place (apres modification du .env)
#
# Dossier de prod : ~/websites/prod/proximas_web (ou PROD_DIR=/autre/chemin). Seul Docker est requis.
# La prod n'est jamais remplacee par un build incomplet. Le .env (facultatif) n'est jamais modifie,
# sauf ses droits (600). Aucun sudo.
#
# Tout le script est dans main() : bash le lit en entier avant de l'executer, donc le git pull peut
# remplacer ce fichier sans effet sur l'execution en cours. Apres un pull, il se relance.
set -euo pipefail

PROJECT=proximas_web
CONTAINER=proximas-web
APP_FILES="proximas-web Dockerfile compose.yml"

die() { echo "ERREUR: $*" >&2; exit 1; }

# env_value : lit une variable du .env de prod (sans l'executer).
env_value() {
  { grep -s "^$1=" "$PROD/.env" || true; } | tail -n 1 | cut -d= -f2- | tr -d "\"' \r"
}

check_tools() {
  docker compose version >/dev/null 2>&1 || die "« docker compose » introuvable (Ubuntu : sudo apt install docker-compose-v2)"
  docker buildx version >/dev/null 2>&1 || die "« docker buildx » introuvable (Ubuntu : sudo apt install docker-buildx)"
  command -v curl >/dev/null 2>&1 || die "curl introuvable"
}

# Le port doit etre libre (ou deja tenu par notre conteneur), sinon « compose up » echouerait apres
# l'arret de l'ancien conteneur.
check_port() {
  command -v ss >/dev/null 2>&1 || return 0
  docker port "$CONTAINER" 2>/dev/null | grep -q ":$PORT\$" && return 0
  if ss -ltn 2>/dev/null | awk '{print $4}' | grep -q ":$PORT\$"; then
    die "le port $PORT est deja utilise par un autre programme : rien n'est remplace. Voir : sudo ss -ltnp | grep $PORT"
  fi
  return 0
}

# Sante, puis deux pages reellement servies.
verify() {
  local health=""
  for _ in $(seq 1 20); do
    if health="$(curl -fsS "http://127.0.0.1:$PORT/api/health" 2>/dev/null)" &&
      curl -fsS -o /dev/null "http://127.0.0.1:$PORT/" 2>/dev/null &&
      curl -fsS -o /dev/null "http://127.0.0.1:$PORT/confidentialite" 2>/dev/null; then
      echo "Sante OK : $health"
      return 0
    fi
    sleep 1
  done
  echo "ERREUR: le site ne repond pas correctement sur 127.0.0.1:$PORT" >&2
  (cd "$PROD" && docker compose logs --tail 20) >&2 || true
  echo "Retour a la version precedente : deploy/build-prod.sh --rollback" >&2
  return 1
}

compose_up() {
  (cd "$PROD" && docker compose up -d --build --remove-orphans "$@") ||
    die "demarrage impossible. Retour a la version precedente : deploy/build-prod.sh --rollback"
}

complete() { # $1 = dossier : les trois fichiers sont presents et non vides
  local f
  for f in $APP_FILES; do [ -s "$1/$f" ] || return 1; done
}

# Copie les fichiers de l'application de $1 vers $2 : copies sous un nom temporaire, puis renommage.
install_files() {
  local src="$1" dst="$2" f
  complete "$src" || die "version incomplete dans $src : rien n'est remplace"
  mkdir -p "$dst"
  for f in $APP_FILES; do cp -f "$src/$f" "$dst/.$f.tmp" || { rm -f "$dst"/.*.tmp; die "copie impossible vers $dst"; }; done
  for f in $APP_FILES; do mv -f "$dst/.$f.tmp" "$dst/$f"; done
}

# Le contexte de build de l'image ne contient que le binaire (jamais le .env).
write_dockerignore() { printf '%s\n' '*' '!proximas-web' > "$PROD/.dockerignore"; }

build_and_install() {
  local version tmp
  cd "$SRC"
  version="$(git describe --tags --always --dirty 2>/dev/null || echo dev)"
  echo "==> Build $version (Docker, tests compris)"
  TMP_BUILD="$(mktemp -d)"
  tmp="$TMP_BUILD"
  trap 'rm -rf "${TMP_BUILD:-}"' EXIT
  DOCKER_BUILDKIT=1 docker build -f deploy/Dockerfile.build --target artifact \
    --build-arg "VERSION=$version" --output "type=local,dest=$tmp" . ||
    die "le build ou les tests ont echoue : la prod n'est pas modifiee"

  # Garde-fou : ne jamais remplacer la prod par un build incomplet.
  [ -s "$tmp/proximas-web" ] || die "build incomplet (binaire absent) : la prod n'est pas remplacee"
  cp deploy/Dockerfile deploy/compose.yml "$tmp/" || die "fichiers de deploiement manquants : la prod n'est pas remplacee"
  complete "$tmp" || die "version preparee incomplete : la prod n'est pas remplacee"

  check_port

  echo "==> Copie vers $PROD"
  if complete "$PROD"; then
    install_files "$PROD" "$PROD.prev"
  fi
  install_files "$tmp" "$PROD"
}

main() {
  local mode=deploy pull=1 arg
  for arg in "$@"; do
    case "$arg" in
      --no-pull) pull=0 ;;
      --rollback) mode=rollback ;;
      --restart) mode=restart ;;
      *) echo "Option inconnue : $arg" >&2; exit 2 ;;
    esac
  done

  SRC="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
  PROD="${PROD_DIR:-$HOME/websites/prod/$PROJECT}"

  if [ "$mode" = deploy ] && [ "$pull" = 1 ]; then
    echo "==> git pull"
    git -C "$SRC" pull --ff-only
    exec bash "$SRC/deploy/build-prod.sh" --no-pull
  fi

  check_tools
  mkdir -p "$PROD"
  if [ -f "$PROD/.env" ]; then chmod 600 "$PROD/.env"; fi
  PORT="$(env_value PROXIMAS_WEB_PORT)"
  PORT="${PORT:-8098}"

  case "$mode" in
    restart)
      complete "$PROD" || die "rien a redemarrer dans $PROD (lance d'abord un deploiement)"
      check_port
      write_dockerignore
      compose_up --force-recreate
      verify
      ;;
    rollback)
      complete "$PROD.prev" || die "pas de version precedente complete dans $PROD.prev"
      # La version qu'on quitte est gardee dans $PROD.failed.
      if complete "$PROD"; then install_files "$PROD" "$PROD.failed"; fi
      install_files "$PROD.prev" "$PROD"
      write_dockerignore
      compose_up
      verify
      echo "Version precedente restauree."
      ;;
    deploy)
      build_and_install
      write_dockerignore
      compose_up
      verify
      echo "Deploiement termine."
      ;;
  esac
}

main "$@"
