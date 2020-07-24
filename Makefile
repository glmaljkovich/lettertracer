build-wasm: ## Build the WebAssembly binary
	GOOS=js GOARCH=wasm go build -v -o letters.wasm