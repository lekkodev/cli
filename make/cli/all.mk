GO_BINS := $(GO_BINS) cmd/lekko
DOCKER_BINS := $(DOCKER_BINS) lekko
GO_ALL_REPO_PKGS ?= ./cmd/... ./pkg/...


LICENSE_HEADER_LICENSE_TYPE := apache
LICENSE_HEADER_COPYRIGHT_HOLDER := Lekko Technologies, Inc.
LICENSE_HEADER_YEAR_RANGE := 2022
LICENSE_HEADER_IGNORES := \/testdata

BUF_LINT_INPUT := .
BUF_FORMAT_INPUT := .

GO_PRIVATE := $(GOPRIVATE),github.com/lekkodev/*,buf.build/gen/go

include make/go/bootstrap.mk
include make/go/go.mk
include make/go/docker.mk
include make/go/buf.mk
include make/go/license_header.mk
include make/go/dep_protoc_gen_go.mk
include make/go/dep_protoc_gen_connect_go.mk

bufgeneratedeps:: $(BUF) $(PROTOC_GEN_GO) $(PROTOC_GEN_CONNECT_GO)

.PHONY: bufgeneratecleango
bufgeneratecleango:
	rm -rf internal/gen/proto

bufgenerateclean:: bufgeneratecleango

.PHONY: bufgeneratego
bufgeneratego:
	buf generate

bufgeneratesteps:: bufgeneratego
