VERSION ?= v2.0.0-dev

build:
	go build -ldflags "-X main.version=$(VERSION)" -o store ./cmd/store
