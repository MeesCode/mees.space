.PHONY: build build-frontend build-backend run test clean

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
