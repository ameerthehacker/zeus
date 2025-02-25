.PHONY: test play llvmc always

always:
	rm -rf playground/debug
	mkdir -p playground/debug

test-verbose:
	go test ./test/... -v

test-race:
	go test ./test/... -race

test:
	go test ./test/...

play: always
	@if [ "$(ir)" = "true" ]; then \
		go run zeus.go build ./playground/$(file).zs --internal-zeus-ir --internal-llvm-ir -o ./playground/debug/$(file); \
	else \
		go run zeus.go build --file-type=ll ./playground/$(file).zs -o ./playground/debug/$(file).ll; \
		llc -filetype=obj -o ./playground/debug/$(file).o ./playground/debug/$(file).ll; \
		clang ./playground/debug/$(file).o -o ./playground/debug/$(file); \
	fi

llvmc:
	llc -filetype=obj -o ./playground/debug/$(file).o ./playground/debug/$(file).ll
	clang ./playground/debug/$(file).o -o ./playground/debug/$(file)

