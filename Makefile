.PHONY: build test lint coverage stress fuzz release-check

build:
	go build -o bin/synctest-scout ./cmd/synctest-scout
test:
	python3 scripts/quality.py test
lint:
	python3 scripts/quality.py lint
coverage:
	python3 scripts/quality.py coverage --snapshot
stress:
	python3 scripts/quality.py stress
fuzz:
	python3 scripts/quality.py fuzz
release-check:
	python3 scripts/release.py --version v0.0.0-dev --output dist/snapshot
