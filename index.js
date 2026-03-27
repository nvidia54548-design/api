import './wasm_exec.js';
import wasm from './main.wasm';

const go = new Go();

async function init() {
    const instance = await WebAssembly.instantiate(wasm, go.importObject);
    go.run(instance);
}

const initPromise = init();

export default {
    async fetch(request, env, ctx) {
        await initPromise;
        // self.fetchHandler is set in Go's main function via syumai/workers
        return self.fetchHandler(request, env, ctx);
    }
};
