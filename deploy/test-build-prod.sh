#!/usr/bin/env bash
# Test de deploy/build-prod.sh sans serveur ni Docker : dossiers temporaires, docker/curl/git/ss simules.
# Verifie surtout que la prod n'est jamais remplacee par un build incomplet. Lance par « make test ».
set -uo pipefail
SRC="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
T="$(mktemp -d)"
trap 'rm -rf "$T"' EXIT
FAIL=0
ok()  { echo "  ok: $1"; }
bad() { echo "  ECHEC: $1"; FAIL=1; }
check() { local m="$1"; shift; if "$@"; then ok "$m"; else bad "$m"; fi; }
same() { [ "$(cat "$1" 2>/dev/null)" = "$2" ]; }
logged() { grep -q -- "$1" "$T/log"; }
not_logged() { ! grep -q -- "$1" "$T/log"; }

# Faux depot : le script et les fichiers de deploiement
REPO="$T/repo"; mkdir -p "$REPO/deploy" "$T/bin"
cp "$SRC/build-prod.sh" "$SRC/Dockerfile" "$SRC/compose.yml" "$REPO/deploy/"

# Simulations. STUB_BUILD=ok|incomplete|fail ; journal des appels dans STUB_LOG.
cat > "$T/bin/docker" <<'STUB'
#!/usr/bin/env bash
echo "docker $*" >> "$STUB_LOG"
case "$1" in
  build)
    [ "${STUB_BUILD:-ok}" = fail ] && exit 1
    for a in "$@"; do case "$a" in type=local,dest=*) d="${a#type=local,dest=}" ;; esac; done
    [ "${STUB_BUILD:-ok}" = ok ] && echo "${STUB_BIN:-nouveau}" > "$d/proximas-web" ;;
  port) [ "${STUB_OWN_PORT:-0}" = 1 ] && echo "8080/tcp -> 127.0.0.1:8098" ;;
  compose) case "$*" in *" up "*) exit "${STUB_UP:-0}" ;; esac ;;
esac
exit 0
STUB
printf '#!/usr/bin/env bash\necho "curl $*" >> "$STUB_LOG"\n[ "${STUB_CURL:-0}" = 0 ] && echo "{\\"status\\":\\"ok\\"}"\nexit "${STUB_CURL:-0}"\n' > "$T/bin/curl"
printf '#!/usr/bin/env bash\n[ "${STUB_BUSY:-0}" = 1 ] && echo "LISTEN 0 128 127.0.0.1:8098 0.0.0.0:*"\nexit 0\n' > "$T/bin/ss"
printf '#!/usr/bin/env bash\necho "git $*" >> "$STUB_LOG"\nexit 0\n' > "$T/bin/git"
printf '#!/usr/bin/env bash\nexit 0\n' > "$T/bin/sleep"
chmod +x "$T/bin/"*
export PATH="$T/bin:$PATH" STUB_LOG="$T/log"

run() { PROD_DIR="$T/prod" bash "$REPO/deploy/build-prod.sh" "$@" >"$T/out" 2>&1; local r=$?; [ -n "${SHOWOUT:-}" ] && cat "$T/out"; return $r; }
reset() { rm -rf "$T/prod" "$T/prod.prev" "$T/prod.failed"; : > "$T/log"
  export STUB_BUILD=ok STUB_BIN=nouveau STUB_CURL=0 STUB_BUSY=0 STUB_OWN_PORT=0 STUB_UP=0; }
installed() { reset; mkdir -p "$T/prod"; echo ancien > "$T/prod/proximas-web"; echo OLDDF > "$T/prod/Dockerfile"; echo OLDCOMPOSE > "$T/prod/compose.yml"; }

echo "1. syntaxe et option inconnue"
check "bash -n" bash -n "$SRC/build-prod.sh"
reset; run --nimporte; check "option inconnue : code 2" test $? = 2
check "option inconnue : rien lance" not_logged "docker build -f"

echo "2. premier deploiement, sans .env ni dossier"
reset; run --no-pull; RC=$?
check "code 0" test $RC = 0
check "binaire en place" same "$T/prod/proximas-web" nouveau
check "Dockerfile et compose.yml copies" test -s "$T/prod/Dockerfile" -a -s "$T/prod/compose.yml"
check "aucun .env cree" test ! -e "$T/prod/.env"
check "aucune version precedente" test ! -e "$T/prod.prev"
check "compose up lance" logged "compose up"
check "sante verifiee sur 8098" logged "127.0.0.1:8098/api/health"
check "pas de git pull avec --no-pull" not_logged "git .*pull"

echo "3. git pull --ff-only par defaut"
reset; run; RC=$?
check "code 0" test $RC = 0
check "pull ff-only" logged "git -C .* pull --ff-only"
check "deploiement apres le pull" same "$T/prod/proximas-web" nouveau

echo "4. mise a jour : version precedente gardee, puis rollback"
installed; echo "PROXIMAS_WEB_PORT=8198" > "$T/prod/.env"
run --no-pull; RC=$?
check "code 0" test $RC = 0
check "nouveau binaire" same "$T/prod/proximas-web" nouveau
check "ancien binaire dans .prev" same "$T/prod.prev/proximas-web" ancien
check ".env conserve" grep -q PROXIMAS_WEB_PORT=8198 "$T/prod/.env"
check "port lu dans le .env" logged "127.0.0.1:8198/api/health"
check ".env hors de .prev" test ! -e "$T/prod.prev/.env"
run --rollback; RC=$?
check "rollback : code 0" test $RC = 0
check "rollback : ancien binaire revenu" same "$T/prod/proximas-web" ancien
check "rollback : ancien compose.yml revenu" same "$T/prod/compose.yml" OLDCOMPOSE
check "rollback : version quittee dans .failed" same "$T/prod.failed/proximas-web" nouveau

echo "5. build incomplet ou en echec : prod inchangee"
for kind in incomplete fail; do
  installed; export STUB_BUILD=$kind; run --no-pull; RC=$?
  check "$kind : code non nul" test $RC != 0
  check "$kind : binaire inchange" same "$T/prod/proximas-web" ancien
  check "$kind : aucun redemarrage" not_logged "compose up"
done

echo "6. fichier de deploiement manquant : prod inchangee"
installed; mv "$REPO/deploy/compose.yml" "$T/compose.garde"
run --no-pull; RC=$?
mv "$T/compose.garde" "$REPO/deploy/compose.yml"
check "code non nul" test $RC != 0
check "binaire inchange" same "$T/prod/proximas-web" ancien

echo "7. port pris par un autre programme"
installed; export STUB_BUSY=1; run --no-pull; RC=$?
check "code non nul" test $RC != 0
check "binaire inchange" same "$T/prod/proximas-web" ancien
installed; export STUB_BUSY=1 STUB_OWN_PORT=1; run --no-pull
check "port tenu par notre conteneur : accepte" test $? = 0

echo "8. site qui ne repond pas"
installed; export STUB_CURL=22; run --no-pull; RC=$?
check "code non nul" test $RC != 0
check "indique le rollback" grep -q -- "--rollback" "$T/out"
check "version precedente disponible" same "$T/prod.prev/proximas-web" ancien

echo "9. rollback et restart sans rien en place"
reset; run --rollback; check "rollback sans .prev : echec" test $? != 0
reset; run --restart; check "restart sans prod : echec" test $? != 0
installed; run --restart
check "restart : code 0" test $? = 0
check "restart : conteneur recree, sans build du code" logged "compose up .*--force-recreate"
check "restart : pas de docker build" not_logged "docker build -f"

if [ "$FAIL" = 0 ]; then echo "build-prod.sh : tous les tests passent"; else echo "build-prod.sh : ECHEC"; cat "$T/out"; fi
exit $FAIL
