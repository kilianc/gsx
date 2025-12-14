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
