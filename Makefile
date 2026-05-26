APP_NAME := archwiki
BIN_DIR := bin
GO ?= go
VERSION := $(shell cat VERSION)
LDFLAGS := -X 'archwiki-tui/internal/app.Version=$(VERSION)'

.PHONY: build run test tidy fmt clean release

build:
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(APP_NAME) ./cmd/archwiki

run:
	$(GO) run -ldflags "$(LDFLAGS)" ./cmd/archwiki

test:
	$(GO) test ./...

tidy:
	$(GO) mod tidy

fmt:
	$(GO) fmt ./...

clean:
	rm -rf $(BIN_DIR)

release: clean build
	tar -czf $(BIN_DIR)/$(APP_NAME)-$(VERSION)-linux-amd64.tar.gz -C $(BIN_DIR) $(APP_NAME)
