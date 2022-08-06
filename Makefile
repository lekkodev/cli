MAKEGO := make/go
MAKEGO_REMOTE := https://github.com/lekkodev/makego.git
PROJECT := cli
GO_MODULE := github.com/lekkodev/cli
DOCKER_ORG := lekko
DOCKER_PROJECT := cli

include make/cli/all.mk
