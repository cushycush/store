VERSION ?= v1.2.2

build:
	go build -ldflags "-X main.version=$(VERSION)" -o store ./cmd/store
