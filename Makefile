PROJECT         := github.com/i-rocky/docker-win-networking
SETUP_IMAGE     := wpkpda/docker-win-net-setup
VERSION         := $(shell git describe --tags)
LD_FLAGS        := -X ${PROJECT}/version.Version=${VERSION} -X ${PROJECT}/version.SetupImage=${SETUP_IMAGE}

run:: build-docker run-win
build:: build-docker build-win

run::
	go run -ldflags "${LD_FLAGS}" ${PROJECT}

run-win:: build-win
	sudo ./docker-win-networking

build::
	go build -ldflags "-s -w ${LD_FLAGS}" ${PROJECT}

build-win::
	GOOS="windows";GOARCH="amd64";go build -ldflags "-s -w ${LD_FLAGS}" ${PROJECT}

build-docker::
	docker build -t ${SETUP_IMAGE}:${VERSION} ./client

build-push-docker::
	cd client && GOOS="linux";GOARCH="amd64";go build -o app main.go
	docker build -t ${SETUP_IMAGE} ./client
	docker push ${SETUP_IMAGE}
