build-wasm: ## Build the WebAssembly binary
	GOOS=js GOARCH=wasm go build -v -o letters.wasm
dist:
	mkdir -p dist && cp index.html dist/ && cp wasm_exec.js dist/ && cp letters.wasm dist/ && cp -R assets/ dist/assets/