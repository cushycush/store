VERSION ?= v0.3.0

build:
	go build -ldflags "-X main.version=$(VERSION)" -o store ./cmd/store
