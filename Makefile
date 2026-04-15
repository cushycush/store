VERSION ?= v1.2.1

build:
	go build -ldflags "-X main.version=$(VERSION)" -o store ./cmd/store
