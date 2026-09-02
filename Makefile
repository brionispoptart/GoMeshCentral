run-server:
	go run ./cmd/server

run-agent:
	go run ./cmd/agent

web-dev:
	cd web && npm.cmd run dev

web-build:
	cd web && npm.cmd run build

test:
	go test ./...

lint:
	go vet ./...
