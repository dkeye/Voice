// mic-worklet.js — стабильный AudioWorklet-захват с аккумулированием блоков
class CaptureProcessor extends AudioWorkletProcessor {
    constructor(options) {
        super();
        this.blockSize = (options?.processorOptions?.blockSize) || 128;
        this._buf = new Float32Array(0);
    }

    process(inputs) {
        const input = inputs[0];
        if (!input || input.length === 0) return true;
        const ch = input[0];
        if (!ch) return true;

        // аккумулируем до blockSize и отправляем
        let data = ch;
        if (this._buf.length) {
            const tmp = new Float32Array(this._buf.length + ch.length);
            tmp.set(this._buf, 0);
            tmp.set(ch, this._buf.length);
            data = tmp;
            this._buf = new Float32Array(0);
        }

        let offset = 0;
        while (offset + this.blockSize <= data.length) {
            const slice = data.subarray(offset, offset + this.blockSize);
            this.port.postMessage(slice, [slice.buffer]);
            offset += this.blockSize;
        }
        if (offset < data.length) {
            this._buf = data.subarray(offset).slice();
        }
        return true;
    }
}

registerProcessor("capture-processor", CaptureProcessor);
