VERSION ?= v0.2.0

build:
	go build -ldflags "-X main.version=$(VERSION)" -o store ./cmd/store
