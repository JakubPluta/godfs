build:
	@go build -o ./bin/fs

run: build
	@./bin/fs

test-v:
	@go test -v ./...

test:
	@go test ./...