.PHONY: test play llvmc

test-verbose:
	go test ./test/... -v

test-race:
	go test ./test/... -race

test:
	go test ./test/...

play:
	@if [ "$(ir)" = "true" ]; then \
		go run zeus.go build ./playground/$(file).tsl --internal-zeus-ir --internal-llvm-ir -o ./playground/$(file).ll; \
	else \
		go run zeus.go build ./playground/$(file).tsl -o ./playground/$(file).ll; \
	fi
	@if [ ! -f ./playground/$(file).ll ]; then \
		echo "Error: ./playground/$(file).ll does not exist"; \
		exit 1; \
	fi
	llc -filetype=obj -o ./playground/$(file).o ./playground/$(file).ll
	clang ./playground/$(file).o -o ./playground/$(file)

llvmc:
	llc -filetype=obj -o ./playground/$(file).o ./playground/$(file).ll
	clang ./playground/$(file).o -o ./playground/$(file)
