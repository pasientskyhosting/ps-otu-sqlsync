VERSION ?= "v1.0.0"
run:
	go run -race src/*.go

all: prep binaries docker

prep:
	mkdir -p bin

binaries: linux64 darwin64

linux64:
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/ps-otu-sqlsync64 src/*.go

darwin64:
	GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o bin/ps-otu-sqlsyncOSX src/*.go

pack-linux64: linux64
	upx --brute bin/ps-otu-sqlsync64

pack-darwin64: darwin64
	upx --brute bin/ps-otu-sqlsyncOSX

docker: pack-linux64
	docker build --build-arg version="$(VERSION)" -t pasientskyhosting/ps-otu-sqlsync:latest . && \
	docker build --build-arg version="$(VERSION)" -t pasientskyhosting/ps-otu-sqlsync:"$(VERSION)" .

docker-run:
	docker run pasientskyhosting/ps-otu-sqlsync:"$(VERSION)"

docker-push: docker
	docker push pasientskyhosting/ps-otu-sqlsync:"$(VERSION)"