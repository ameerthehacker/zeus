.PHONY: test test-go test-runtime build-runtime clean compile run always build lsp

clean:
	rm -rf playground/debug
	rm -rf zeus-vscode/vsix
	cd runtime && zig build clean

always:
	rm -rf playground/debug
	mkdir -p playground/debug playground/debug/out

test-verbose:
	go test ./test/... -v

test-race:
	go test ./test/... -race

test: test-go test-runtime

test-go:
	go test ./test/...

test-e2e: build-runtime
	ZEUS_HOME=$(CURDIR)/runtime/zig-out/out go test ./test/e2e/... -v -count=1

test-runtime:
	cd runtime && zig build test

build-runtime:
	@if [ "$(release)" = "true" ]; then \
		cd runtime && zig build -Doptimize=ReleaseSmall; \
	else \
		cd runtime && zig build; \
	fi

build-runtime-debug:
	cd runtime && zig build

build-zeus-vscode:
	mkdir -p zeus-vscode/vsix
	cd zeus-vscode && npm run package &&npm run vsix 

build-runtime-release: always
	cd runtime && zig build -Doptimize=ReleaseSmall

compile: always build-runtime
	@if [ "$(debug)" = "true" ]; then \
		ZEUS_DEBUG=true ZEUS_HOME=$(CURDIR)/runtime/zig-out/out $(if $(filter true,$(nogc)),ZEUS_NO_GC=true) go run zeus.go build --target-dir ./playground/debug/out ./playground/$(file).zs -o ./playground/debug/$(file); \
	else \
		ZEUS_HOME=$(CURDIR)/runtime/zig-out/out $(if $(filter true,$(nogc)),ZEUS_NO_GC=true) go run zeus.go build ./playground/$(file).zs -o ./playground/debug/$(file); \
	fi

compile-release: always
	cd runtime && zig build -Doptimize=ReleaseSmall
	ZEUS_HOME=$(CURDIR)/runtime/zig-out/out $(if $(filter true,$(nogc)),ZEUS_NO_GC=true) go run zeus.go build ./playground/$(file).zs -o ./playground/debug/$(file)

run: always build-runtime
	@if [ "$(debug)" = "true" ]; then \
		ZEUS_DEBUG=true ZEUS_HOME=$(CURDIR)/runtime/zig-out/out $(if $(filter true,$(nogc)),ZEUS_NO_GC=true) go run zeus.go build --target-dir ./playground/debug/out ./playground/$(file).zs -o ./playground/debug/$(file); \
		ZEUS_GC_DEBUG=true ./playground/debug/$(file); \
	else \
		ZEUS_HOME=$(CURDIR)/runtime/zig-out/out $(if $(filter true,$(nogc)),ZEUS_NO_GC=true) go run zeus.go build ./playground/$(file).zs -o ./playground/debug/$(file); \
		./playground/debug/$(file); \
	fi

build:
	go build -o zeus zeus.go

lsp: build
	./zeus lsp --stdio
