CURRENT_REVISION = $(shell git rev-parse --short HEAD)
DATE := $(shell date +%Y-%m-%dT%H:%M:%S%z)
BUILD_LDFLAGS = "-X main.revision=${CURRENT_REVISION} -X main.buildDate=${DATE}"
ifdef update
  u=-u
endif

deps:
	go get ${u} github.com/golang/dep/cmd/dep
	dep ensure

devel-deps: deps
	go get ${u} github.com/golang/lint/golint
	go get ${u} github.com/mattn/goveralls
	go get ${u} github.com/motemen/gobump/cmd/gobump
	go get ${u} github.com/Songmu/goxz/cmd/goxz
	go get ${u} github.com/Songmu/ghch/cmd/ghch
	go get ${u} github.com/tcnksm/ghr

test: deps
	go test

lint: devel-deps
	go vet
	golint -set_exit_status

cover: devel-deps
	goveralls

build: deps
	go build -ldflags=$(BUILD_LDFLAGS) ./cmd/fluent-agent-chimera

crossbuild: devel-deps
	$(eval ver = $(shell gobump show -r ./cmd/fluent-agent-chimera))
	goxz -pv=v$(ver) -os="linux darwin" -build-ldflags=$(BUILD_LDFLAGS) \
	  -d=./dist/v$(ver) ./cmd/fluent-agent-chimera

release:
	_tools/releng
	_tools/upload_artifacts

update-deps:
	dep ensure -update

.PHONY: test deps devel-deps lint cover crossbuild release
