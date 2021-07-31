ROOT = $(shell pwd)
ROOTBASENAME = $(shell basename ${ROOT})
PARENTDIR := $(shell dirname `pwd`)
export GOPATH := ${PARENTDIR}/${ROOTBASENAME}-gopath
export PATH := ${PATH}:${GOPATH}/bin

MODULE = github.com/sayotte/plannerdemo
BINARY = plannerdemo

default: test lint install

installdelve:
	if [ ! -x "${GOPATH}/bin/dlv" ]; then \
		go get github.com/go-delve/delve/cmd/dlv; \
	fi

clean:
	GO111MODULE=off go clean --modcache
	rm -rfv ${GOPATH}

lint:
	if [ ! -x "${GOPATH}/bin/golangci-lint" ]; then \
		go get github.com/golangci/golangci-lint/cmd/golangci-lint@v1.39.0; \
	fi
	${GOPATH}/bin/golangci-lint run ./...

cilint:
	golangci-lint run ./...

test:
	go test -mod vendor -cover ${MODULE}/...

install:
	gofmt -l -w .
	go install -mod vendor ${MODULE}/cmd/${BINARY}

run: install
	${GOPATH}/bin/${BINARY}

# launch a shell, handy for setting GOPATH/PATH on the command line
shell:
	bash
