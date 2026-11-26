#!/usr/bin/env bash
# Test local de l'image de production, lance par « make docker » : demarre out/compose.yml sur un port
# a part, verifie les pages, les en-tetes et les restrictions du conteneur, puis arrete tout.
# Local uniquement : aucun serveur distant.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/../out"
export PROXIMAS_WEB_PORT="${SMOKE_PORT:-18098}"
BASE="http://127.0.0.1:$PROXIMAS_WEB_PORT"
compose() { docker compose -p proximas-web-smoke -f compose.yml "$@"; }
fail() { echo "ECHEC: $*" >&2; compose logs --tail 20 >&2 || true; exit 1; }
trap 'compose down --remove-orphans >/dev/null 2>&1 || true' EXIT

compose up -d --build >/dev/null
for _ in $(seq 1 30); do curl -fsS "$BASE/api/health" >/dev/null 2>&1 && break; sleep 1; done
health="$(curl -fsS "$BASE/api/health")" || fail "/api/health"
echo "sante : $health"

for p in / /mentions-legales /confidentialite /conditions /conditions/version-1 /confidentialite/version-1 /suppression-compte /securite /contact \
  /en/ /en/legal-notice /en/privacy /en/terms /en/account-deletion /en/safety /en/contact \
  /robots.txt /sitemap.xml /favicon.ico /static/style.css /static/fonts/Jost-Variable.ttf; do
  [ "$(curl -s -o /dev/null -w '%{http_code}' "$BASE$p")" = 200 ] || fail "$p"
done
[ "$(curl -s -o /dev/null -w '%{http_code}' "$BASE/absent")" = 404 ] || fail "404"
[ "$(curl -s -o /dev/null -w '%{http_code} %{redirect_url}' -H 'Host: www.proximas.be' "$BASE/contact")" = "301 https://proximas.be/contact" ] ||
  fail "redirection www"
headers="$(curl -sS -D - -o /dev/null "$BASE/" | tr -d '\r')"
for h in "Content-Security-Policy: default-src 'none'" "Strict-Transport-Security" "X-Content-Type-Options: nosniff" "X-Frame-Options: DENY"; do
  grep -qi -- "$h" <<<"$headers" || fail "en-tete absent : $h"
done
! grep -qi '^set-cookie' <<<"$headers" || fail "cookie envoye"

id="$(compose ps -q web)"
got="$(docker inspect -f '{{.HostConfig.ReadonlyRootfs}} {{.HostConfig.CapDrop}} {{.HostConfig.SecurityOpt}} {{.Config.User}} {{.HostConfig.LogConfig.Config}}' "$id")"
[ "$got" = "true [ALL] [no-new-privileges:true] 65532:65532 map[max-file:3 max-size:10m]" ] || fail "restrictions du conteneur : $got"
docker port "$id" | grep -q "127.0.0.1:$PROXIMAS_WEB_PORT" || fail "le port doit etre publie sur 127.0.0.1"
! docker port "$id" | grep -q '0.0.0.0' || fail "port public"
state=starting
for _ in $(seq 1 45); do
  state="$(docker inspect -f '{{.State.Health.Status}}' "$id")"
  [ "$state" = healthy ] && break
  sleep 1
done
[ "$state" = healthy ] || fail "controle de sante Docker : $state"
echo "OK: image de production verifiee (pages, en-tetes, conteneur restreint, sante)"
