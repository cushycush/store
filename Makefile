VERSION ?= dev

build:
	go build -ldflags "-X main.version=$(VERSION)" -o store ./cmd/store
