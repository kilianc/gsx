.PHONY: gen
gen: ## Regenerate every *.gsx.go from its *.gsx source
	go run ./cmd/gsx ./...

.PHONY: check
check: ## Fail if any checked-in *.gsx.go is out of date
	go run ./cmd/gsx -check ./...

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
ci: check vet test

.PHONY: playground
playground:
	go run ./cmd/playground

.PHONY: vsix
vsix:
	cd vscode/gsx-vscode && npm install && npm run compile && npx vsce package --no-dependencies
