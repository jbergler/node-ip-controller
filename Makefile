.DEFAULT_GOAL := build

.PHONY: build build.docker

BINARY        ?= node-ip-controller
SOURCES        = $(shell find . -name '*.go')
IMAGE         ?= gcr.io/jonasbergler-com/$(BINARY)
VERSION       ?= $(shell git describe --tags --always --dirty)
BUILD_FLAGS   ?= -v
LDFLAGS       ?= -w -s

build: build/$(BINARY)

build/$(BINARY): $(SOURCES)
	CGO_ENABLED=0 go build -o build/$(BINARY) $(BUILD_FLAGS) -ldflags "$(LDFLAGS)" .

build.push: build.docker
	docker push "$(IMAGE):$(VERSION)"

build.docker:
	docker build --rm --tag "$(IMAGE):$(VERSION)" .

clean:
	@rm -rf build
