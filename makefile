.PHONY: test test-go test-runtime build-runtime build-runtime-release build-release package-release clean compile run check always build lsp fmt fetch-libxev

clean:
	rm -rf playground/target
	rm -rf zeus-vscode/vsix
	rm -rf build
	rm -rf third_party
	rm -rf runtime/.zig-cache
	rm -rf runtime/zig-out

always:
	rm -rf playground/target
	cmake -B build -DCMAKE_BUILD_TYPE=Release
	make -C build

GO_BUILD_TAGS := -tags llvm19

test-verbose:
	go test $(GO_BUILD_TAGS) ./test/... -v

test-race:
	go test $(GO_BUILD_TAGS) ./test/... -race

test: build-runtime test-go test-runtime

test-go:
	go test $(GO_BUILD_TAGS) ./test/...

test-e2e: build-runtime
	ZEUS_HOME=$(CURDIR) go test $(GO_BUILD_TAGS) ./test/e2e/... -v -count=1

test-runtime: fetch-libxev
	cd runtime && zig build-lib -O Debug -static --name zeus-runtime-test \
		$(ZIG_MODULE_FLAGS) $(ZIG_FLAGS) && rm -f libzeus-runtime-test.a libzeus-runtime-test.a.o

ZIG_SDK ?= $(shell xcrun --show-sdk-path 2>/dev/null)
BOEHM_GC_INCLUDE ?= $(CURDIR)/third_party/bdwgc/include
BOEHM_GC_LIB ?= $(CURDIR)/third_party/bdwgc/lib
LLVM_CONFIG ?= $(shell brew --prefix llvm@19 2>/dev/null)/bin/llvm-config
VERSION ?= dev

# libxev: last Zig 0.13-compatible commit of mitchellh/libxev
LIBXEV_HASH = 1220d220fe297775b574b0312faaf5afcc0044cbda650d0558ec2dda8fb1a6dc1228
LIBXEV_URL = https://github.com/mitchellh/libxev/archive/07bcffa0f63054152a9883baa42bce5faad297e6.tar.gz
LIBXEV_SRC = /tmp/zig-cache-15/p/$(LIBXEV_HASH)/src/main.zig

ZIG_FLAGS = --global-cache-dir /tmp/zig-cache-15 --sysroot $(ZIG_SDK) -I$(ZIG_SDK)/usr/include -I$(BOEHM_GC_INCLUDE) -lc -lunwind -L$(BOEHM_GC_LIB) -lgc
ZIG_MODULE_FLAGS = --dep xev -Mmain=main.zig -Mxev=$(LIBXEV_SRC)

fetch-libxev:
	@if [ ! -f "$(LIBXEV_SRC)" ]; then \
		zig fetch --global-cache-dir /tmp/zig-cache-15 "$(LIBXEV_URL)" > /dev/null; \
	fi

build-runtime: fetch-libxev
	@mkdir -p runtime/zig-out/out
	@if [ "$(release)" = "true" ]; then \
		cd runtime && zig build-lib -O ReleaseSmall -static --name zeus-runtime \
			$(ZIG_MODULE_FLAGS) $(ZIG_FLAGS) && \
		mv libzeus-runtime.a zig-out/out/ && mv libzeus-runtime.a.o zig-out/out/zeus-runtime.o; \
	else \
		cd runtime && zig build-lib -O ReleaseFast -static --name zeus-runtime \
			$(ZIG_MODULE_FLAGS) $(ZIG_FLAGS) && \
		mv libzeus-runtime.a zig-out/out/ && mv libzeus-runtime.a.o zig-out/out/zeus-runtime.o; \
	fi

build-runtime-debug: fetch-libxev
	@mkdir -p runtime/zig-out/out
	cd runtime && zig build-lib -O Debug -static --name zeus-runtime \
		$(ZIG_MODULE_FLAGS) $(ZIG_FLAGS) && \
		mv libzeus-runtime.a zig-out/out/ && mv libzeus-runtime.a.o zig-out/out/zeus-runtime.o

build-zeus-vscode:
	mkdir -p zeus-vscode/vsix
	cd zeus-vscode && npm run package &&npm run vsix

build-runtime-release: always fetch-libxev
	@mkdir -p runtime/zig-out/out
	cd runtime && zig build-lib -O ReleaseSmall -static --name zeus-runtime \
		$(ZIG_MODULE_FLAGS) $(ZIG_FLAGS) && \
		mv libzeus-runtime.a zig-out/out/ && mv libzeus-runtime.a.o zig-out/out/zeus-runtime.o

compile: always build-runtime
	@if [ "$(debug)" = "true" ]; then \
		ZEUS_DEBUG=true ZEUS_HOME=$(CURDIR) $(if $(filter true,$(nogc)),ZEUS_NO_GC=true) go run $(GO_BUILD_TAGS) zeus.go build --target-dir ./playground/debug/out ./playground/$(file).zs -o ./playground/debug/$(file); \
	else \
		ZEUS_HOME=$(CURDIR) $(if $(filter true,$(nogc)),ZEUS_NO_GC=true) go run $(GO_BUILD_TAGS) zeus.go build ./playground/$(file).zs -o ./playground/debug/$(file); \
	fi

compile-release: always build-runtime-release
	ZEUS_HOME=$(CURDIR) $(if $(filter true,$(nogc)),ZEUS_NO_GC=true) go run $(GO_BUILD_TAGS) zeus.go build ./playground/$(file).zs -o ./playground/debug/$(file)

check:
	ZEUS_DEBUG=true ZEUS_HOME=$(CURDIR) go run $(GO_BUILD_TAGS) zeus.go check ./playground/$(file).zs --emit-ir ./playground

run: always build-runtime
	@if [ "$(debug)" = "true" ]; then \
		ZEUS_DEBUG=true ZEUS_HOME=$(CURDIR) $(if $(filter true,$(nogc)),ZEUS_NO_GC=true) go run $(GO_BUILD_TAGS) zeus.go build ./playground/$(file).zs --emit-ir --target-dir ./playground && \
		ZEUS_GC_DEBUG=true ./playground/target/debug/bin/$(file); \
	else \
		ZEUS_HOME=$(CURDIR) $(if $(filter true,$(nogc)),ZEUS_NO_GC=true) go run $(GO_BUILD_TAGS) zeus.go build ./playground/$(file).zs --target-dir ./playground && \
		./playground/target/debug/bin/$(file); \
	fi

ARCH ?= arm64

package-release:
	$(eval PACKAGE_NAME := zeus-$(VERSION)-darwin-$(ARCH))
	mkdir -p $(PACKAGE_NAME)/bin
	mkdir -p $(PACKAGE_NAME)/runtime
	mkdir -p $(PACKAGE_NAME)/lib
	mkdir -p $(PACKAGE_NAME)/third_party/bdwgc
	cp bin/zeus $(PACKAGE_NAME)/bin/
	cp -r runtime/zig-out $(PACKAGE_NAME)/runtime/
	cp -r lib/std $(PACKAGE_NAME)/lib/
	cp -r third_party/bdwgc/lib $(PACKAGE_NAME)/third_party/bdwgc/
	tar -czvf $(PACKAGE_NAME).tar.gz $(PACKAGE_NAME)
	shasum -a 256 $(PACKAGE_NAME).tar.gz | awk '{print $$1}' > $(PACKAGE_NAME).sha256
	rm -rf $(PACKAGE_NAME)

build-release:
	mkdir -p bin
	CGO_ENABLED=1 \
	CGO_CFLAGS="$(shell $(LLVM_CONFIG) --cflags)" \
	CGO_CXXFLAGS="$(shell $(LLVM_CONFIG) --cxxflags)" \
	CGO_LDFLAGS="$(shell $(LLVM_CONFIG) --ldflags --libs all --system-libs)" \
	go build $(GO_BUILD_TAGS) \
		-ldflags "-X 'github.com/ameerthehacker/zeus/cmd.Version=$(VERSION)'" \
		-o bin/zeus zeus.go

build:
	CGO_ENABLED=1 go build $(GO_BUILD_TAGS) -o zeus zeus.go

lsp: build
	./zeus lsp --stdio

fmt:
	gofmt -w $(shell find . -name '*.go' -not -path './vendor/*')
