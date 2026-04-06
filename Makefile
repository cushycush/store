VERSION ?= v0.4.1

build:
	go build -ldflags "-X main.version=$(VERSION)" -o store ./cmd/store
