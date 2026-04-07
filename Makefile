VERSION ?= v0.5.0

build:
	go build -ldflags "-X main.version=$(VERSION)" -o store ./cmd/store
