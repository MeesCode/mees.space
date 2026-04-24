.PHONY: build build-frontend build-backend build-run run test test-backend test-frontend clean

build: build-frontend build-backend

build-frontend:
	cd frontend && npm run build

build-backend:
	go build -o mees-server ./cmd/server

build-run: build run

run:
	./mees-server

test: test-backend test-frontend

test-backend:
	go test ./... -v -cover

test-frontend:
	cd frontend && npm test

clean:
	rm -rf dist/ mees-server tmp/
