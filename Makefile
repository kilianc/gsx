.PHONY: gen
gen: ## Regenerate every *.gsx.go from its *.gsx source
	go run ./cmd/gsx ./...

.PHONY: fmt
fmt: ## Format every *.gsx and Go source
	go run ./cmd/gsx fmt -w ./...
	gofmt -w .

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
# The editor bundle needs the whole repository in context, because it imports
# the grammar out of vscode/gsx-vscode and writes into docs/static.
DOCKER_WEB := docker run --rm -u "$$(id -u):$$(id -g)" -e HOME=/tmp \
	-e npm_config_cache=/tmp/.npm \
	-v "$(CURDIR):/work" -w /work/web $(TOOLS_IMAGE)

.PHONY: tools-image
tools-image: ## Build the pinned Node toolchain image
	docker build -q -f tools/Dockerfile -t $(TOOLS_IMAGE) .

# The bundle is checked in, so `make docs` needs only Go. Rebuild it when the
# grammar or the editor dependencies change — the grammar is imported from the
# extension, so a grammar change that is not followed by this leaves the
# playground highlighting with the old one.
.PHONY: editor
editor: tools-image ## Rebuild docs/static/vendor from web/
	$(DOCKER_WEB) npm install --no-audit --no-fund
	$(DOCKER_WEB) npm run build

.PHONY: grammar-test
grammar-test: tools-image ## Tokenize fixtures with the real TextMate engine
	$(DOCKER_RUN) node test/tokenize.mjs

.PHONY: extension-test
extension-test: tools-image ## Run the extension unit tests
	$(DOCKER_RUN) node --test test/tags.test.mjs

.PHONY: extension-check
extension-check: tools-image ## Typecheck the extension
	$(DOCKER_CHECK) npx tsc --noEmit -p tsconfig.json

# The README demo animation. Recording it is a human with a screen recorder —
# a container cannot do that part — but the conversion is pinned here so the
# result does not depend on whose ffmpeg ran it, and so nobody needs ffmpeg on
# their machine to refresh the demo after a visual change.
#
# The output is twice its display size on purpose. The README shows the GIF at
# 920px, and a 920px-wide file is upscaled on every HiDPI screen, which is most
# of them — that, more than the encoder, is what makes a demo look soft.
DEMO_IMAGE := gsx-demo
OUT ?= assets/gsx-demo.gif
WIDTH ?= 1840
FPS ?= 20
# gifski is passed --width even though ffmpeg has already scaled the frames:
# without it gifski quietly shrinks anything over "about 800x600", which is
# exactly the downscale this target exists to avoid.
#
# Frames land in the container's /tmp rather than the bind mount: there are
# hundreds of them, they are read once, and the host has no use for them.
DOCKER_DEMO := docker run --rm -u "$$(id -u):$$(id -g)" -e HOME=/tmp \
	-v "$(CURDIR):/work" -w /work $(DEMO_IMAGE)
REEL_DIR := .tmp/reel

.PHONY: demo-image
demo-image: ## Build the pinned ffmpeg + gifski image
	docker build -q -f tools/demo.Dockerfile -t $(DEMO_IMAGE) .

# The README demo is rendered, not recorded: cmd/demogen writes one HTML file
# per frame from the real compiler's output, and Chromium screenshots each at
# device scale 2. That is why it can be sharp — there is no capture to upscale —
# and why it can be regenerated when the palette or the compiler changes.
.PHONY: demo-reel
demo-reel: demo-image ## Render assets/gsx-demo.gif from cmd/demogen
	go run ./cmd/demogen -out $(REEL_DIR)
	$(DOCKER_DEMO) sh -c 'set -e; \
		rm -rf /tmp/shots; mkdir -p /tmp/shots; \
		for f in $(REEL_DIR)/*.html; do \
			n=$$(basename "$$f" .html); \
			chromium --headless=old --no-sandbox --disable-gpu --hide-scrollbars \
				--allow-file-access-from-files --force-device-scale-factor=2 \
				--window-size=920,430 --virtual-time-budget=800 \
				--screenshot=/tmp/shots/$$n.png "file:///work/$$f" >/dev/null 2>&1; \
		done; \
		gifski --fps $(FPS) --quality 100 --width $(WIDTH) -o "$(OUT)" /tmp/shots/*.png'
	@ls -lh "$(OUT)"

.PHONY: demo
demo: demo-image ## Convert a screen recording into the README demo: make demo IN=recording.mov
	@test -n "$(IN)" || { \
		echo "usage: make demo IN=recording.mov [OUT=$(OUT)] [WIDTH=$(WIDTH)] [FPS=$(FPS)]"; \
		exit 1; }
	$(DOCKER_DEMO) sh -c 'set -e; \
		rm -rf /tmp/frames; mkdir -p /tmp/frames; \
		ffmpeg -v error -y -i "$(IN)" \
			-vf "fps=$(FPS),scale=$(WIDTH):-2:flags=lanczos" /tmp/frames/%05d.png; \
		gifski --fps $(FPS) --quality 100 --width $(WIDTH) -o "$(OUT)" /tmp/frames/*.png'
	@ls -lh "$(OUT)"

# Packaging needs webpack and vsce, which are not in the image: they pull in
# native modules that make the image build slow and unreliable, and nothing else
# needs them. They are installed here instead.
.PHONY: vsix
vsix: tools-image ## Package the extension
	$(DOCKER_RUN) sh -c "npm ci --ignore-scripts --no-audit --no-fund && npm run compile && vsce package --no-dependencies"
