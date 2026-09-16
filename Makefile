
# TL;DR:
# make build: build locally
# make test: run all tests
# make test-unit: just unit tests
# make test-e2e: just e2e tests
# make release: after git tag, release to github and prepare krew file

.PHONY: test test-unit test-e2e build goreleaser release clean
os ?= $(shell uname -s | tr '[:upper:]' '[:lower:]')
arch ?= $(shell go env GOARCH | tr '[:upper:]' '[:lower:]')
underscore = $(word $2,$(subst _, ,$1))

test: test-unit test-e2e test-integration

test-unit:
	go test -v ./...

test-e2e: dist/kubectl-neat_$(os)_$(arch)
	bats ./test/e2e-cli.bats

test-integration: dist/checksums.txt
	bats ./test/e2e-kubectl.bats
	bats ./test/e2e-krew.bats

build: dist/kubectl-neat_$(os)_$(arch)

SRC = $(shell find . -type f -name '*.go' -not -path "./vendor/*")
dist/kubectl-neat_%: $(SRC)
	GOOS=$(call underscore,$*,1) GOARCH=$(call underscore,$*,2) go build -o dist/$(@F)

# release by default will not publish. run with `publish=1` to publish
goreleaserflags = --skip=publish --snapshot
ifdef publish
	goreleaserflags =
endif
# relase always re-builds (no dependencies on purpose)
goreleaser: $(SRC)
	goreleaser --clean $(goreleaserflags) 

dist/checksums.txt: goreleaser
	# no op recipe
	@:

# Releases are normally cut by pushing a tag: .github/workflows/release.yml builds and
# publishes them. This target is the manual equivalent, run on a checked-out tag.
release: publish = 1
release: dist/checksums.txt
	hack/krew-manifest.sh "$$(git describe --tags --exact-match)" dist > dist/neat.yaml

clean:
	rm -rf dist
