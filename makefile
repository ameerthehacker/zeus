.PHONY: test test-go test-runtime build-runtime clean compile run always

clean:
	rm -rf playground/debug e2e/out
	cd runtime && zig build clean

always:
	rm -rf playground/debug
	mkdir -p playground/debug playground/debug/out e2e/out

test-verbose:
	go test ./test/... -v

test-race:
	go test ./test/... -race

test: test-go test-runtime

test-go:
	go test ./test/...

test-runtime:
	cd runtime && zig build test

build-runtime:
	cd runtime && zig build

build-runtime-release: always
	cd runtime && zig build --release=small

compile: always build-runtime
	@if [ "$(debug)" = "true" ]; then \
		ZEUS_DEBUG=true go run zeus.go build --target-dir ./playground/debug/out ./playground/$(file).zs -o ./playground/debug/$(file); \
	else \
		go run zeus.go build ./playground/$(file).zs -o ./playground/debug/$(file); \
	fi

run: always build-runtime
	@if [ "$(debug)" = "true" ]; then \
		ZEUS_DEBUG=true go run zeus.go build --target-dir ./playground/debug/out ./playground/$(file).zs -o ./playground/debug/$(file); \
		ZEUS_GC_DEBUG=true ./playground/debug/$(file); \
	else \
		go run zeus.go build ./playground/$(file).zs -o ./playground/debug/$(file); \
		./playground/debug/$(file); \
	fi
