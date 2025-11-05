// mic-worklet.js — streams mono Float32 blocks from the mic to main thread.
// app.js converts to PCM16 and sends via WS.

class MicProcessor extends AudioWorkletProcessor {
    constructor() { super(); this._buf = new Float32Array(0); this.blockSize = 480; }
    process(inputs) {
        const ch = inputs?.[0]?.[0];
        if (!ch) return true;

        // Concatenate and flush fixed-size packets
        let data = ch;
        if (this._buf.length) {
            const tmp = new Float32Array(this._buf.length + ch.length);
            tmp.set(this._buf, 0); tmp.set(ch, this._buf.length);
            data = tmp; this._buf = new Float32Array(0);
        }
        let off = 0;
        while (off + this.blockSize <= data.length) {
            const slice = data.subarray(off, off + this.blockSize).slice();
            this.port.postMessage(slice, [slice.buffer]);
            off += this.blockSize;
        }
        if (off < data.length) this._buf = data.subarray(off).slice();
        return true;
    }
}
registerProcessor('mic-worklet', MicProcessor);
