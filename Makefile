# proximas.be : site de presentation de ProximaS (Go, bibliotheque standard).
#
#   make dev      -> site local sur http://127.0.0.1:8098
#   make test     -> format, vet, tests Go, tests de build-prod.sh, champs manquants
#   make champs   -> liste les champs de site.conf encore vides
#   make docker   -> build de production dans Docker (comme sur le serveur) + essai du conteneur
#   make clean
#
# En production : deploy/build-prod.sh (voir README.md).
# Sous Windows : GNU make (winget install ezwinports.make) + sh de Git for Windows.

SHELL := sh
PORT ?= 8098

.PHONY: dev test champs docker clean

dev:
	LISTEN_ADDR=127.0.0.1:$(PORT) go run . serve

test:
	test -z "$$(gofmt -l .)"
	go vet ./...
	go test ./...
	bash deploy/test-build-prod.sh
	go run . champs

champs:
	go run . champs

docker:
	rm -rf out
	docker build -f deploy/Dockerfile.build --target artifact --output type=local,dest=out .
	test -s out/proximas-web
	cp deploy/Dockerfile deploy/compose.yml out/
	cd out && docker compose config -q
	bash deploy/smoke.sh
	@echo "OK: out/ produit et image verifiee"

clean:
	rm -rf out
