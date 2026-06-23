.PHONY: test test-go test-runtime build-runtime clean compile run always build lsp

clean:
	rm -rf playground/debug
	rm -rf zeus-vscode/vsix
	rm -rf build
	rm -rf third_party
	rm -rf runtime/.zig-cache
	rm -rf runtime/zig-out

always:
	rm -rf playground/debug
	mkdir -p playground/debug playground/debug/out
	cmake -B build -DCMAKE_BUILD_TYPE=Release
	make -C build

GO_BUILD_TAGS := -tags llvm19

test-verbose:
	go test $(GO_BUILD_TAGS) ./test/... -v

test-race:
	go test $(GO_BUILD_TAGS) ./test/... -race

test: test-go test-runtime

test-go:
	go test $(GO_BUILD_TAGS) ./test/...

test-e2e: build-runtime
	ZEUS_HOME=$(CURDIR) go test $(GO_BUILD_TAGS) ./test/e2e/... -v -count=1

test-runtime:
	cd runtime && zig build test

ZIG_FLAGS = --global-cache-dir /tmp/zig-cache-15 --sysroot $(ZIG_SDK) -I$(ZIG_SDK)/usr/include -I$(BOEHM_GC_INCLUDE) -lc -lunwind -L$(BOEHM_GC_LIB) -lgc

build-runtime:
	@mkdir -p runtime/zig-out/out
	@if [ "$(release)" = "true" ]; then \
		cd runtime && zig build-lib main.zig -O ReleaseSmall -static --name zeus-runtime $(ZIG_FLAGS) && \
		mv libzeus-runtime.a zig-out/out/ && mv libzeus-runtime.a.o zig-out/out/zeus-runtime.o; \
	else \
		cd runtime && zig build-lib main.zig -O ReleaseFast -static --name zeus-runtime $(ZIG_FLAGS) && \
		mv libzeus-runtime.a zig-out/out/ && mv libzeus-runtime.a.o zig-out/out/zeus-runtime.o; \
	fi

build-runtime-debug:
	@mkdir -p runtime/zig-out/out
	cd runtime && zig build-lib main.zig -O Debug -static --name zeus-runtime $(ZIG_FLAGS) && \
		mv libzeus-runtime.a zig-out/out/ && mv libzeus-runtime.a.o zig-out/out/zeus-runtime.o

build-zeus-vscode:
	mkdir -p zeus-vscode/vsix
	cd zeus-vscode && npm run package &&npm run vsix 

build-runtime-release: always
	cd runtime && zig build -Doptimize=ReleaseSmall

compile: always build-runtime
	@if [ "$(debug)" = "true" ]; then \
		ZEUS_DEBUG=true ZEUS_HOME=$(CURDIR) $(if $(filter true,$(nogc)),ZEUS_NO_GC=true) go run $(GO_BUILD_TAGS) zeus.go build --target-dir ./playground/debug/out ./playground/$(file).zs -o ./playground/debug/$(file); \
	else \
		ZEUS_HOME=$(CURDIR) $(if $(filter true,$(nogc)),ZEUS_NO_GC=true) go run $(GO_BUILD_TAGS) zeus.go build ./playground/$(file).zs -o ./playground/debug/$(file); \
	fi

compile-release: always
	cd runtime && zig build -Doptimize=ReleaseSmall
	ZEUS_HOME=$(CURDIR) $(if $(filter true,$(nogc)),ZEUS_NO_GC=true) go run $(GO_BUILD_TAGS) zeus.go build ./playground/$(file).zs -o ./playground/debug/$(file)

run: always build-runtime
	@if [ "$(debug)" = "true" ]; then \
		ZEUS_DEBUG=true ZEUS_HOME=$(CURDIR) $(if $(filter true,$(nogc)),ZEUS_NO_GC=true) go run $(GO_BUILD_TAGS) zeus.go build --target-dir ./playground/debug/out ./playground/$(file).zs -o ./playground/debug/$(file); \
		ZEUS_GC_DEBUG=true ./playground/debug/$(file); \
	else \
		ZEUS_HOME=$(CURDIR) $(if $(filter true,$(nogc)),ZEUS_NO_GC=true) go run $(GO_BUILD_TAGS) zeus.go build ./playground/$(file).zs -o ./playground/debug/$(file); \
		./playground/debug/$(file); \
	fi

build:
	CGO_ENABLED=1 go build $(GO_BUILD_TAGS) -o zeus zeus.go

lsp: build
	./zeus lsp --stdio
