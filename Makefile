VERSION ?= v0.7.0

build:
	go build -ldflags "-X main.version=$(VERSION)" -o store ./cmd/store
