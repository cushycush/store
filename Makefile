VERSION ?= v1.3.1

build:
	go build -ldflags "-X main.version=$(VERSION)" -o store ./cmd/store
