APP_NAME := hotreload
HOTRELOAD_BIN := ./bin/$(APP_NAME)
SERVER_BIN := ./bin/server

.PHONY: build build-server run fmt test clean

build:
	@mkdir -p ./bin
	go build -o $(HOTRELOAD_BIN) ./cmd/hotreload

build-server:
	@mkdir -p ./bin
	go build -o $(SERVER_BIN) ./testserver

run: build
	$(HOTRELOAD_BIN) --root ./testserver --build "go build -o ./bin/server ./testserver" --exec "./bin/server"

fmt:
	gofmt -w ./cmd ./internal ./testserver

test:
	go test ./...

clean:
	rm -rf ./bin
