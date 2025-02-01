run:
	go run cmd/main.go

test:
	go test -v ./...
	# go test -v -run TestGetRelFname 

build:
	go build -o germ cmd/main.go

demo:
	go run cmd/main.go > germ.map

aider:
	rm -rf ./.aider.tags.cache.v3
	export PIP_DISABLE_PIP_VERSION_CHECK=1
	PYTHONPATH=/code/aider python3 -m aider.main --show-repo-map > aider.map
