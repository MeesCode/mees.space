.PHONY: dev-backend dev-frontend build build-frontend build-backend run test clean

dev-backend:
	go run ./cmd/server

dev-frontend:
	cd frontend && npm run dev

build: build-frontend build-backend

build-frontend:
	cd frontend && npm run build

build-backend:
	go build -o mees-server ./cmd/server

run:
	./mees-server

test:
	go test ./... -v -cover

clean:
	rm -rf dist/ mees-server tmp/
