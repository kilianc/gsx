.PHONY: gen
gen: ## Regenerate every *.gsx.go from its *.gsx source
	go run ./cmd/gsx ./...

.PHONY: fmt
fmt: ## Format every *.gsx source
	go run ./cmd/gsx fmt -w ./...

.PHONY: check
check: ## Fail if any checked-in generated file is out of date, or any source is unformatted
	go run ./cmd/gsx -check ./...
	go run ./cmd/gsx fmt -l ./...
	@out=$$(gofmt -l .); \
	if [ -n "$$out" ]; then echo "not gofmt'd:"; echo "$$out"; exit 1; fi
	go generate ./...
	git diff --exit-code -- '*_tables.go' 'internal/gsx/playground/symbols'

.PHONY: test
test: ## Run the full test suite
	go test ./...

.PHONY: golden
golden: gen ## Regenerate every golden file, then run the tests
	go test ./e2e -update
	go test ./...

.PHONY: fmt
fmt: ## Format every package
	gofmt -w .

.PHONY: fmt-check
fmt-check: ## Fail if anything is unformatted
	@out=$$(gofmt -l .); \
	if [ -n "$$out" ]; then echo "not gofmt'd:"; echo "$$out"; exit 1; fi

.PHONY: vet
vet:
	go vet ./...

.PHONY: ci
ci: check vet wasm test

.PHONY: ci-extension
ci-extension: grammar-test extension-test extension-check

.PHONY: docs
docs: ## Build the documentation site into docs/dist
	go run ./docs -out ./docs/dist

.PHONY: docs-serve
docs-serve: ## Serve the documentation site, re-rendering on every request
	go run ./docs -serve localhost:8123

.PHONY: docs-prose
docs-prose: ## Serve the docs without building the playground bundle
	go run ./docs -serve localhost:8123 -wasm=false

.PHONY: wasm
wasm: ## Type-check the playground's browser build
	GOOS=js GOARCH=wasm go build -o /dev/null ./cmd/gsx-wasm

.PHONY: localplay
localplay: ## Watch localplay/page.gsx and regenerate it with the working-tree compiler
	go run ./cmd/localplay

# The extension is the only part of this repository that needs Node, so it is
# built in a pinned container rather than requiring a JS toolchain on the host.
TOOLS_IMAGE := gsx-tools
# node_modules lives in a Docker volume, not the bind mount: writing tens of
# thousands of small files through a macOS bind mount takes minutes, and the
# host has no reason to read them.
DOCKER_RUN := docker run --rm -u "$$(id -u):$$(id -g)" -e HOME=/tmp \
	-v "$(CURDIR)/vscode/gsx-vscode:/work" -w /work $(TOOLS_IMAGE)
# The typecheck runs against the project prepared inside the image, with only
# the sources mounted in, so node_modules is already resolved.
DOCKER_CHECK := docker run --rm -u "$$(id -u):$$(id -g)" -e HOME=/tmp \
	-v "$(CURDIR)/vscode/gsx-vscode/src:/opt/ext/src:ro" -w /opt/ext $(TOOLS_IMAGE)

.PHONY: tools-image
tools-image: ## Build the pinned Node toolchain image
	docker build -q -f tools/Dockerfile -t $(TOOLS_IMAGE) .

.PHONY: grammar-test
grammar-test: tools-image ## Tokenize fixtures with the real TextMate engine
	$(DOCKER_RUN) node test/tokenize.mjs

.PHONY: extension-test
extension-test: tools-image ## Run the extension unit tests
	$(DOCKER_RUN) node --test test/tags.test.mjs

.PHONY: extension-check
extension-check: tools-image ## Typecheck the extension
	$(DOCKER_CHECK) npx tsc --noEmit -p tsconfig.json

# Packaging needs webpack and vsce, which are not in the image: they pull in
# native modules that make the image build slow and unreliable, and nothing else
# needs them. They are installed here instead.
.PHONY: vsix
vsix: tools-image ## Package the extension
	$(DOCKER_RUN) sh -c "npm install --no-audit --no-fund && npm run compile && npx vsce package --no-dependencies"
