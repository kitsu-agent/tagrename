BIN := tagrename

.PHONY: build test fmt vet install clean

build:
	go build -o $(BIN) .

test:
	go test ./...

fmt:
	gofmt -w .

vet:
	go vet ./...

install: build
	install -d $(HOME)/.local/bin
	install -m 0755 $(BIN) $(HOME)/.local/bin/$(BIN)

clean:
	rm -f $(BIN)
