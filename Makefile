VERSION ?= v1.1.0

build:
	go build -ldflags "-X main.version=$(VERSION)" -o store ./cmd/store
