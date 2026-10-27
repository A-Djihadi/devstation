#!/bin/sh
# Suite complète, identique dans le sandbox agentique (hors-ligne) et en CI Linux.
set -e
export GOCACHE="${GOCACHE:-/tmp/gocache}" GOFLAGS="-mod=readonly -buildvcs=false" GOTOOLCHAIN=local
[ -n "$CI" ] || export GOPROXY=off
# Sandbox agentique : on formate (l'agent ne peut pas lancer gofmt) ; en CI, on verifie seulement.
[ -n "$CI" ] || gofmt -w .
unformatted=$(gofmt -l .)
[ -z "$unformatted" ] || { echo "Fichiers non formatés (lancer gofmt -w) :"; echo "$unformatted"; exit 1; }
go vet ./...
go test ./...
GOOS=windows go vet ./...
GOOS=windows go build ./...
pwsh -NoProfile -Command "Invoke-Pester -CI -Path tests/ps"
