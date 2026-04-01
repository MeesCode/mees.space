.PHONY: build build-frontend build-backend build-run run test clean

build: build-frontend build-backend

build-frontend:
	cd frontend && npm run build

build-backend:
	go build -o mees-server ./cmd/server

build-run: build run

run:
	./mees-server

test:
	go test ./... -v -cover

clean:
	rm -rf dist/ mees-server tmp/
