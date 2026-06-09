# gocldr — the library is pure Go (testify is test-only), so `make test` runs
# natively. Only CLDR code-generation is version-sensitive and therefore pinned
# to a hermetic Docker toolchain (see gen/Dockerfile); never run the generators
# or their Node fixture scripts on the host.

GEN_IMAGE := gocldr-cldrgen
# Keep in sync with .github/workflows/ci.yml.
STATICCHECK_VERSION := 2026.1
DOCKER_RUN := docker run --rm \
	-v "$(CURDIR)":/work \
	-u "$(shell id -u):$(shell id -g)" \
	"$(GEN_IMAGE)"

.PHONY: gen gen-image test vet fmt lint

gen-image:        ## Build the pinned CLDR generation image
	docker build --load -t "$(GEN_IMAGE)" gen

gen: gen-image    ## Regenerate all tables + Intl golden fixtures (CLDR pinned in gen/package.json)
	$(DOCKER_RUN) sh gen/generate.sh

test:             ## Run the test suite (pure Go, zero runtime deps — host is fine)
	go test ./...

vet:
	go vet ./...

lint:             ## go vet + staticcheck (pinned, matching CI) + gofmt check
	go vet ./...
	go run honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION) ./...
	@if [ -n "$$(gofmt -l .)" ]; then echo "gofmt needed in:"; gofmt -l .; exit 1; fi

fmt:
	gofmt -w .
