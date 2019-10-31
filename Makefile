.PHONY: all

VERSION := $(shell git describe --tags --always --dirty)
VENDOR := 
RPMVERSIONRAW := $(shell git describe --tags --always --dirty | sed -rn  's/^[v]*([0-9.]+)*[-]*(.*)/\1/p')
RPMVERSION := $(if $(RPMVERSIONRAW),$(RPMVERSIONRAW),0.0.1)
RPMRELEASERAW := $(shell git describe --tags --always --dirty | sed -rn  's/^[v]*([0-9.]+)*[-]*(.*)/\2/p' | sed 's/-/./g')
RPMRELEASE := $(if $(RPMRELEASERAW),$(RPMRELEASERAW),1)
# VENDOR := "-mod=vendor"
PKGBASE := github.com/zgiles/ptar/cmd

all: cmd-ptar

cmd-ptar:
	# VENDOR: ${VENDOR}
	# VERSION: ${VERSION}
	CGO_ENABLED=0 go build ${VENDOR} -ldflags "-w -extldflags -static -X main.version=${VERSION}" ${PKGBASE}/ptar

