// wasm_exec.js for Cloudflare Workers
"use strict";

(() => {
    const enosys = () => {
        const err = new Error("not implemented");
        err.code = "ENOSYS";
        return err;
    };

    if (!globalThis.fs) {
        let outputBuf = "";
        globalThis.fs = {
            constants: { O_WRONLY: -1, O_RDWR: -1, O_CREAT: -1, O_TRUNC: -1, O_APPEND: -1, O_EXCL: -1 }, // dummy
            writeSync(fd, buf) {
                outputBuf += new TextDecoder("utf-8").decode(buf);
                const nl = outputBuf.lastIndexOf("\n");
                if (nl != -1) {
                    console.log(outputBuf.substring(0, nl));
                    outputBuf = outputBuf.substring(nl + 1);
                }
                return buf.length;
            },
            write(fd, buf, offset, length, position, callback) {
                if (offset !== 0 || length !== buf.length || position !== null) {
                    callback(enosys());
                    return;
                }
                const n = this.writeSync(fd, buf);
                callback(null, n);
            },
            chmod(path, mode, callback) { callback(enosys()); },
            chown(path, uid, gid, callback) { callback(enosys()); },
            close(fd, callback) { callback(enosys()); },
            fchmod(fd, mode, callback) { callback(enosys()); },
            fchown(fd, uid, gid, callback) { callback(enosys()); },
            fstat(fd, callback) { callback(enosys()); },
            fsync(fd, callback) { callback(null); },
            ftruncate(fd, length, callback) { callback(enosys()); },
            lchown(path, uid, gid, callback) { callback(enosys()); },
            link(path, link, callback) { callback(enosys()); },
            lstat(path, callback) { callback(enosys()); },
            mkdir(path, mode, callback) { callback(enosys()); },
            open(path, flags, mode, callback) { callback(enosys()); },
            read(fd, buf, offset, length, position, callback) { callback(enosys()); },
            readdir(path, callback) { callback(enosys()); },
            readlink(path, callback) { callback(enosys()); },
            rename(from, to, callback) { callback(enosys()); },
            rmdir(path, callback) { callback(enosys()); },
            stat(path, callback) { callback(enosys()); },
            symlink(path, link, callback) { callback(enosys()); },
            truncate(path, length, callback) { callback(enosys()); },
            unlink(path, callback) { callback(enosys()); },
            utimes(path, atime, mtime, callback) { callback(enosys()); },
        };
    }

    if (!globalThis.process) {
        globalThis.process = {
            getuid() { return -1; },
            getgid() { return -1; },
            getUserid() { return -1; },
            getgroups() { return []; },
            pid: -1,
            ppid: -1,
            umask() { return 0o22; },
            cwd() { return "/"; },
            chdir() { throw enosys(); },
        };
    }

    if (!globalThis.crypto) {
        throw new Error("globalThis.crypto is not available, polyfill required");
    }

    if (!globalThis.performance) {
        globalThis.performance = {
            now() {
                return Date.now();
            },
        };
    }

    if (!globalThis.TextEncoder) {
        throw new Error("globalThis.TextEncoder is not available, polyfill required");
    }

    if (!globalThis.TextDecoder) {
        throw new Error("globalThis.TextDecoder is not available, polyfill required");
    }

    const encoder = new TextEncoder("utf-8");
    const decoder = new TextDecoder("utf-8");

    globalThis.Go = class {
        constructor() {
            this.argv = ["js"];
            this.env = {};
            this.exit = (code) => {
                if (code !== 0) {
                    console.warn("exit code:", code);
                }
            };
            this._exitPromise = new Promise((resolve) => {
                this._resolveExitPromise = resolve;
            });
            this._pendingEvent = null;
            this._scheduledTimeouts = new Map();
            this._nextCallbackTimeoutID = 1;

            const setInt64 = (addr, v) => {
                this.mem.setUint32(addr + 0, v, true);
                this.mem.setUint32(addr + 4, Math.floor(v / 4294967296), true);
            };

            const getInt64 = (addr) => {
                const low = this.mem.getUint32(addr + 0, true);
                const high = this.mem.getInt32(addr + 4, true);
                return low + high * 4294967296;
            };

            const loadValue = (addr) => {
                const f = this.mem.getFloat64(addr, true);
                if (f === 0) {
                    return undefined;
                }
                if (!isNaN(f)) {
                    return f;
                }

                const id = this.mem.getUint32(addr, true);
                return this._values[id];
            };

            const storeValue = (addr, v) => {
                const nanHead = 0x7FF80000;

                if (typeof v === "number" && v !== 0) {
                    if (isNaN(v)) {
                        this.mem.setUint32(addr + 4, nanHead, true);
                        this.mem.setUint32(addr + 0, 0, true);
                        return;
                    }
                    this.mem.setFloat64(addr, v, true);
                    return;
                }

                if (v === undefined) {
                    this.mem.setFloat64(addr, 0, true);
                    return;
                }

                let id = this._ids.get(v);
                if (id === undefined) {
                    id = this._idPool.pop();
                    if (id === undefined) {
                        id = this._values.length;
                    }
                    this._values[id] = v;
                    this._ids.set(v, id);
                }
                this._refCounts[id]++;
                let typeFlag = 0;
                switch (typeof v) {
                    case "object":
                        if (v !== null) typeFlag = 1;
                        break;
                    case "string":
                        typeFlag = 2;
                        break;
                    case "symbol":
                        typeFlag = 3;
                        break;
                    case "function":
                        typeFlag = 4;
                        break;
                }
                this.mem.setUint32(addr + 4, nanHead | typeFlag, true);
                this.mem.setUint32(addr + 0, id, true);
            };

            const loadSlice = (addr) => {
                const array = loadValue(addr);
                const len = getInt64(addr + 8);
                return new Uint8Array(array.buffer, array.byteOffset, len);
            };

            const loadString = (addr) => {
                const saddr = getInt64(addr);
                const len = getInt64(addr + 8);
                return decoder.decode(new Uint8Array(this._inst.exports.mem.buffer, saddr, len));
            };

            const timeOrigin = Date.now() - performance.now();
            this.importObject = {
                go: {
                    // Go's runtime.nanotime()
                    "runtime.nanotime1": (addr) => {
                        setInt64(addr, (timeOrigin + performance.now()) * 1000000);
                    },

                    // Go's runtime.walltime()
                    "runtime.walltime": (addr) => {
                        const msec = (new Date()).getTime();
                        setInt64(addr, Math.floor(msec / 1000));
                        this.mem.setUint32(addr + 8, (msec % 1000) * 1000000, true);
                    },

                    // Go's runtime.scheduleTimeoutEvent()
                    "runtime.scheduleTimeoutEvent": (delay) => {
                        const id = this._nextCallbackTimeoutID;
                        this._nextCallbackTimeoutID++;
                        this._scheduledTimeouts.set(id, setTimeout(
                            () => {
                                this._resume();
                                while (this._scheduledTimeouts.has(id)) {
                                    // for some reason Go expects us to actually wait for this to finish
                                    // we use a simple sleep to prevent 100% CPU usage
                                }
                            },
                            delay,
                        ));
                        return id;
                    },

                    // Go's runtime.clearTimeoutEvent()
                    "runtime.clearTimeoutEvent": (id) => {
                        const timeout = this._scheduledTimeouts.get(id);
                        clearTimeout(timeout);
                        this._scheduledTimeouts.delete(id);
                    },

                    // Go's runtime.getRandomData()
                    "runtime.getRandomData": (slice) => {
                        crypto.getRandomValues(loadSlice(slice));
                    },

                    // syscall/js.finalizeRef()
                    "syscall/js.finalizeRef": (v_ref) => {
                        const id = this.mem.getUint32(v_ref, true);
                        this._refCounts[id]--;
                        if (this._refCounts[id] === 0) {
                            const v = this._values[id];
                            this._values[id] = null;
                            this._ids.delete(v);
                            this._idPool.push(id);
                        }
                    },

                    // syscall/js.stringVal()
                    "syscall/js.stringVal": (ret, v) => {
                        storeValue(ret, loadString(v));
                    },

                    // syscall/js.valueGet()
                    "syscall/js.valueGet": (ret, v, p) => {
                        storeValue(ret, Reflect.get(loadValue(v), loadString(p)));
                    },

                    // syscall/js.valueSet()
                    "syscall/js.valueSet": (v, p, x) => {
                        Reflect.set(loadValue(v), loadString(p), loadValue(x));
                    },

                    // syscall/js.valueDelete()
                    "syscall/js.valueDelete": (v, p) => {
                        Reflect.deleteProperty(loadValue(v), loadString(p));
                    },

                    // syscall/js.valueIndex()
                    "syscall/js.valueIndex": (ret, v, i) => {
                        storeValue(ret, Reflect.get(loadValue(v), i));
                    },

                    // syscall/js.valueSetIndex()
                    "syscall/js.valueSetIndex": (v, i, x) => {
                        Reflect.set(loadValue(v), i, loadValue(x));
                    },

                    // syscall/js.valueCall()
                    "syscall/js.valueCall": (ret, v, m, args) => {
                        try {
                            const v_val = loadValue(v);
                            const m_val = loadString(m);
                            const args_val = loadSliceOfValues(args);
                            const result = Reflect.apply(v_val[m_val], v_val, args_val);
                            storeValue(ret, result);
                            this.mem.setUint8(ret + 8, 1);
                        } catch (err) {
                            storeValue(ret, err);
                            this.mem.setUint8(ret + 8, 0);
                        }
                    },

                    // syscall/js.valueInvoke()
                    "syscall/js.valueInvoke": (ret, v, args) => {
                        try {
                            const v_val = loadValue(v);
                            const args_val = loadSliceOfValues(args);
                            const result = Reflect.apply(v_val, undefined, args_val);
                            storeValue(ret, result);
                            this.mem.setUint8(ret + 8, 1);
                        } catch (err) {
                            storeValue(ret, err);
                            this.mem.setUint8(ret + 8, 0);
                        }
                    },

                    // syscall/js.valueNew()
                    "syscall/js.valueNew": (ret, v, args) => {
                        try {
                            const v_val = loadValue(v);
                            const args_val = loadSliceOfValues(args);
                            const result = Reflect.construct(v_val, args_val);
                            storeValue(ret, result);
                            this.mem.setUint8(ret + 8, 1);
                        } catch (err) {
                            storeValue(ret, err);
                            this.mem.setUint8(ret + 8, 0);
                        }
                    },

                    // syscall/js.valueLength()
                    "syscall/js.valueLength": (v) => {
                        return loadValue(v).length;
                    },

                    // syscall/js.valuePrepareString()
                    "syscall/js.valuePrepareString": (ret, v) => {
                        const s = String(loadValue(v));
                        const str = encoder.encode(s);
                        storeValue(ret, s);
                        setInt64(ret + 8, str.length);
                    },

                    // syscall/js.loadString()
                    "syscall/js.loadString": (v, slice) => {
                        const s = loadValue(v);
                        loadSlice(slice).set(encoder.encode(s));
                    },

                    // syscall/js.valueInstanceOf()
                    "syscall/js.valueInstanceOf": (v, t) => {
                        return loadValue(v) instanceof loadValue(t);
                    },

                    // syscall/js.copyBytesToGo()
                    "syscall/js.copyBytesToGo": (ret, dest, src) => {
                        const d = loadSlice(dest);
                        const s = loadValue(src);
                        if (!(s instanceof Uint8Array || s instanceof Uint8ClampedArray)) {
                            this.mem.setUint8(ret + 8, 0);
                            return;
                        }
                        const n = Math.min(d.length, s.length);
                        d.set(s.subarray(0, n));
                        setInt64(ret, n);
                        this.mem.setUint8(ret + 8, 1);
                    },

                    // syscall/js.copyBytesToJS()
                    "syscall/js.copyBytesToJS": (ret, dest, src) => {
                        const d = loadValue(dest);
                        const s = loadSlice(src);
                        if (!(d instanceof Uint8Array || d instanceof Uint8ClampedArray)) {
                            this.mem.setUint8(ret + 8, 0);
                            return;
                        }
                        const n = Math.min(d.length, s.length);
                        d.set(s.subarray(0, n));
                        setInt64(ret, n);
                        this.mem.setUint8(ret + 8, 1);
                    },

                    "debug": (value) => {
                        console.log(value);
                    },
                }
            };

            const loadSliceOfValues = (addr) => {
                const array = getInt64(addr);
                const len = getInt64(addr + 8);
                const a = new Array(len);
                for (let i = 0; i < len; i++) {
                    a[i] = loadValue(array + i * 8);
                }
                return a;
            };
        }

        async run(instance) {
            if (!(instance instanceof WebAssembly.Instance)) {
                throw new Error("Go.run: WebAssembly.Instance expected");
            }
            this._inst = instance;
            this.mem = new DataView(this._inst.exports.mem.buffer);
            this._values = [ // gapless array of items/values
                NaN,
                0,
                null,
                true,
                false,
                globalThis,
                this,
            ];
            this._ids = new Map([ // maps from value to id
                [0, 1],
                [null, 2],
                [true, 3],
                [false, 4],
                [globalThis, 5],
                [this, 6],
            ]);
            this._idPool = [];   // reuse ids that were freed
            this._refCounts = new Array(this._values.length).fill(Infinity); // we never free the pre-defined ones
            this._pendingEvent = null;

            this._inst.exports.run(this.argv.length, this.argv.length);
            return this._exitPromise;
        }

        _resume() {
            if (this._inst.exports.async_resume) {
                this._inst.exports.async_resume();
            } else {
                this._inst.exports.resume();
            }
        }

        _makeCallback(id) {
            return (...args) => {
                const event = { id, construct: false, args };
                this._pendingEvent = event;
                this._resume();
                return event.result;
            };
        }
    };
})();
