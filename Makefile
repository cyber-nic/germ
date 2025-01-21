run:
	go run cmd/main.go

test:
	go test -v ./...

build:
	go build -o grep-ast cmd/main.go

demo:
	go run cmd/main.go