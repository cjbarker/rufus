.PHONY: build build-faces build-no-faces test lint ci clean install-dlib

BINARY  := rufus
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags="-s -w -X github.com/cjbarker/rufus/cmd.Version=$(VERSION)"

build: build-faces

# install-dlib installs dlib and libjpeg system libraries needed for face detection.
install-dlib:
	@if [ "$$(uname)" = "Darwin" ]; then \
		echo "==> Installing dlib and jpeg-turbo via Homebrew..."; \
		brew install dlib jpeg-turbo; \
	elif command -v apt-get >/dev/null 2>&1; then \
		echo "==> Installing dlib and libjpeg via apt-get..."; \
		sudo apt-get install -y libdlib-dev libjpeg-dev libblas-dev liblapack-dev libopenblas-dev; \
	else \
		echo "ERROR: Unsupported platform. Install dlib manually: https://github.com/Kagami/go-face#requirements"; \
		exit 1; \
	fi

# build-faces installs required system libraries then compiles rufus with face detection enabled.
DLIB_CFLAGS := $(shell pkg-config --cflags dlib-1 2>/dev/null)
DLIB_LIBS   := $(shell pkg-config --libs   dlib-1 2>/dev/null)
JPEG_PREFIX := $(shell brew --prefix jpeg-turbo 2>/dev/null || brew --prefix jpeg 2>/dev/null)
JPEG_CFLAGS := $(if $(JPEG_PREFIX),-I$(JPEG_PREFIX)/include)
JPEG_LIBS   := $(if $(JPEG_PREFIX),-L$(JPEG_PREFIX)/lib -ljpeg)

build-faces:
	@if ! pkg-config --exists dlib-1 2>/dev/null; then \
		$(MAKE) install-dlib; \
	fi
	$(eval DLIB_CFLAGS := $(shell pkg-config --cflags dlib-1 2>/dev/null))
	$(eval DLIB_LIBS   := $(shell pkg-config --libs   dlib-1 2>/dev/null))
	$(eval JPEG_PREFIX := $(shell brew --prefix jpeg-turbo 2>/dev/null || brew --prefix jpeg 2>/dev/null))
	$(eval JPEG_CFLAGS := $(if $(JPEG_PREFIX),-I$(JPEG_PREFIX)/include))
	$(eval JPEG_LIBS   := $(if $(JPEG_PREFIX),-L$(JPEG_PREFIX)/lib -ljpeg))
	CGO_ENABLED=1 \
	CGO_CPPFLAGS="$(DLIB_CFLAGS) $(JPEG_CFLAGS)" \
	CGO_LDFLAGS="$(DLIB_LIBS) $(JPEG_LIBS)" \
	go build -a -tags dlib $(LDFLAGS) -o $(BINARY) .
	@echo "==> Built $(BINARY) with face detection enabled"

# build-no-faces compiles rufus without CGO or dlib — no face detection support.
build-no-faces:
	CGO_ENABLED=0 go build $(LDFLAGS) -o $(BINARY) .
	@echo "==> Built $(BINARY) without face detection"

test:
	go test -v -race ./...

lint:
	golangci-lint run ./...

ci: lint test build

clean:
	rm -f $(BINARY)
	go clean -cache -testcache
