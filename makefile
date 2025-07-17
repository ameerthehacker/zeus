.PHONY: test play llvmc always

clean:
	rm -rf playground/debug
	rm -rf runtime/out

always:
	rm -rf playground/debug
	mkdir -p playground/debug playground/debug/out

test-verbose:
	go test ./test/... -v

test-race:
	go test ./test/... -race

test:
	go test ./test/...

build-runtime: runtime/out/zeus-runtime.o

runtime/out/zeus-runtime.o: runtime/main.zig
	mkdir -p runtime/out
	zig build-obj runtime/main.zig -target native -O Debug -femit-bin=runtime/out/zeus-runtime

play: always build-runtime
	@if [ "$(debug)" = "true" ]; then \
		ZEUS_DEBUG=true go run zeus.go build --target-dir ./playground/debug/out ./playground/$(file).zs -o ./playground/debug/$(file); \
	else \
		go run zeus.go build ./playground/$(file).zs -o ./playground/debug/$(file); \
	fi

