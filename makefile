.PHONY: test play
test-verbose:
	go test ./test/... -v

test-race:
	go test ./test/... -race

test:
	go test ./test/...

play:
	# compile the zeus file to llvm IR
	@if [ "$(ir)" = "true" ]; then \
		go run zeus.go build ./playground/$(file).tsl --internal-zeus-ir --internal-llvm-ir; \
	else \
		go run zeus.go build ./playground/$(file).tsl; \
	fi
	@if [ ! -f ./playground/$(file).ll ]; then \
		echo "Error: ./playground/$(file).ll does not exist"; \
		exit 1; \
	fi
	# compile the llvm IR to object file
	llc -filetype=obj -o ./playground/$(file).o ./playground/$(file).ll
	# compile the object file to executable
	clang ./playground/$(file).o -o ./playground/$(file)
	# run the executable
	./playground/$(file)
