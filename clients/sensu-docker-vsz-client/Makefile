OS_LIST= linux darwin

build: build_dist $(OS_LIST)

build_dist:
	mkdir -p dist/

$(OS_LIST):
	GOOS=$@ CGO_ENABLED=0 GOARCH=amd64 go build -o dist/docker-vsz-check-$@-amd64
