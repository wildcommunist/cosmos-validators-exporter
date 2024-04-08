GIT_HASH ?= $(shell git log --format="%h" -n 1)
VERSION := $(shell echo $(shell git describe --tags) | sed 's/^v//')
CI_COMMIT_SHA := $(shell git rev-parse HEAD)
LDFLAGS = -X main.version=${VERSION}
DOCKER_USERNAME ?= wildcommunist
APPLICATION_NAME ?= cosmos-exporter

build:
	go build -ldflags '$(LDFLAGS)' cmd/cosmos-validators-exporter.go

install:
	go install -ldflags '$(LDFLAGS)' cmd/cosmos-validators-exporter.go

lint:
	golangci-lint run --fix ./...

docker_build :
	docker build \
		-t ${DOCKER_USERNAME}/${APPLICATION_NAME}:${GIT_HASH} \
		--build-arg APP_BUID_VERSION=${VERSION} \
		--build-arg APP_GIT_HASH=${GIT_HASH} \
		--build-arg CI_COMMIT_SHA=${CI_COMMIT_SHA} \
		.

push :
	docker push ${DOCKER_USERNAME}/${APPLICATION_NAME}:${GIT_HASH}


release : docker_build
	 docker tag  ${DOCKER_USERNAME}/${APPLICATION_NAME}:${GIT_HASH} ${DOCKER_USERNAME}/${APPLICATION_NAME}:latest
	 docker push ${DOCKER_USERNAME}/${APPLICATION_NAME}:${GIT_HASH}
	 docker push ${DOCKER_USERNAME}/${APPLICATION_NAME}:latest

build_run : docker_build run

go_run :
	go run cmd/cosmos-validators-exporter.go --config ./config.example.toml

run :
	 docker run -it --rm -p 9560:9560 ${DOCKER_USERNAME}/${APPLICATION_NAME}:${GIT_HASH}