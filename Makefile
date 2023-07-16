PROJECT         := github.com/i-rocky/docker-win-networking

run:: build
	sudo ./docker-win-networking

build::
	GOOS="windows";GOARCH="amd64";go build ${PROJECT}

build-client::
	cd client && GOOS="linux";GOARCH="amd64";go build -o app main.go
	docker build -t wpkpda/docker-win-net-setup ./client

push-client:: build-client
	docker push wpkpda/docker-win-net-setup
