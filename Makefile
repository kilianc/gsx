.PHONY: playground
playground:
	go run ./cmd/playground

.PHONY: test
test: clean
	go test ./...
	go test ./... # run again to test HTML output

.PHONY: clean
clean:
	rm -rf ./**/*.gsx.go
	rm -rf ./**/*.html

.PHONY: vsix
vsix:
	cd vscode/gsx-vscode && npm install && npm run compile && npx vsce package --no-dependencies
