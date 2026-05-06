.PHONY: build run test docker

build:
	go build -o bin/arbitrage ./cmd

run:
	go run ./cmd/main.go

test:
	go test -v ./...

docker-build:
	docker-compose build

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down
