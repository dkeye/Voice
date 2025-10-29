// app.js — минимальный клиент без лишних флажков

const SAMPLE_RATE = 48000;
const CAPTURE_BLOCK = 128;
const SEND_FRAME_SAMPLES = 4096;
const WS_BUFFERED_LIMIT = 2 * 1024 * 1024;

let ws, ctx, source, workletNode;
let isTalking = false;
let sendAccumulator = new Float32Array(0);
let playQueue = [];
let isScheduling = false;
let scheduledTime = 0;

const log = msg => {
  const el = document.getElementById("log");
  el.textContent += msg + "\n";
  el.scrollTop = el.scrollHeight;
};

document.getElementById("connect").onclick = connect;
const talkBtn = document.getElementById("talk");
const toggleMode = document.getElementById('toggleMode');
let toggleState = false;

talkBtn.onmousedown = () => {
  if (toggleMode?.checked) {
    toggleState = !toggleState;
    setTalking(toggleState);
  } else {
    setTalking(true);
  }
};

talkBtn.onmouseup = () => {
  if (!toggleMode?.checked) setTalking(false);
};

talkBtn.onmouseleave = () => {
  if (!toggleMode?.checked) setTalking(false);
};

// ----------------------------
//  Build WS URL with username
// ----------------------------
function buildWSUrl() {
  const input = document.getElementById("userName");
  let name = input?.value?.trim() || "";
  if (!name) {
    const rand = Math.random().toString(36).substring(2, 6);
    name = "user-" + rand;
    input.value = name;
  }
  const protocol = location.protocol === "https:" ? "wss://" : "ws://";
  return `${protocol}${location.host}/join?name=${encodeURIComponent(name)}`;
}

// ----------------------------
//  Connect and setup audio
// ----------------------------
async function connect() {
  const WS_URL = buildWSUrl();

  ctx = new AudioContext({ sampleRate: SAMPLE_RATE });
  const stream = await navigator.mediaDevices.getUserMedia({
    audio: { channelCount: 1, echoCancellation: false, noiseSuppression: false, autoGainControl: false }
  });
  source = ctx.createMediaStreamSource(stream);
  await ctx.audioWorklet.addModule("/static/mic-worklet.js");

  workletNode = new AudioWorkletNode(ctx, "capture-processor", {
    processorOptions: { blockSize: CAPTURE_BLOCK }
  });
  workletNode.port.onmessage = ev => onCaptureBlock(ev.data);
  source.connect(workletNode);

  ws = new WebSocket(WS_URL);
  ws.binaryType = "arraybuffer";
  ws.onopen = () => log("✅ Connected");
  ws.onclose = () => log("🔌 WS closed");
  ws.onerror = e => log("⚠️ WS error");
  ws.onmessage = async e => {
    const ab = e.data instanceof Blob ? await e.data.arrayBuffer() : e.data;
    if (!ab || ab.byteLength === 0) return;
    const pcm16 = new Int16Array(ab);
    const f32 = new Float32Array(pcm16.length);
    for (let i = 0; i < pcm16.length; i++) f32[i] = pcm16[i] / 0x8000;
    enqueuePlayback(f32);
  };

  talkBtn.disabled = false;
  log("🎧 Ready. Hold to talk.");
}

// ----------------------------
//  Push-to-talk logic
// ----------------------------
function setTalking(v) {
  isTalking = !!v;
  talkBtn.textContent = isTalking ? "🔴 Speaking..." : "🎙️ Push-to-Talk";
  log(isTalking ? "🎙️ Start talking" : "🔇 Stop talking");
}

// ----------------------------
//  Capture & send audio
// ----------------------------
function onCaptureBlock(f32) {
  if (!isTalking || ws.readyState !== WebSocket.OPEN) return;
  if (ws.bufferedAmount > WS_BUFFERED_LIMIT) return;

  const needed = SEND_FRAME_SAMPLES - sendAccumulator.length;
  if (f32.length >= needed) {
    const frame = new Float32Array(SEND_FRAME_SAMPLES);
    frame.set(sendAccumulator, 0);
    frame.set(f32.subarray(0, needed), sendAccumulator.length);
    sendAccumulator = f32.subarray(needed);
    sendPCM16(frame);
  } else {
    const tmp = new Float32Array(sendAccumulator.length + f32.length);
    tmp.set(sendAccumulator, 0);
    tmp.set(f32, sendAccumulator.length);
    sendAccumulator = tmp;
  }
}

function sendPCM16(f32) {
  const len = f32.length;
  const ab = new ArrayBuffer(len * 2);
  const view = new DataView(ab);
  for (let i = 0; i < len; i++) {
    let s = f32[i];
    if (i < 4) s *= i / 4;
    else if (i > len - 5) s *= (len - 1 - i) / 4;
    s = Math.max(-1, Math.min(1, s));
    view.setInt16(i * 2, s < 0 ? s * 0x8000 : s * 0x7fff, true);
  }
  ws.send(ab);
}

// ----------------------------
//  Playback scheduling
// ----------------------------
function enqueuePlayback(f32) {
  playQueue.push(f32);
  if (!isScheduling) {
    scheduledTime = Math.max(ctx.currentTime + 0.05, scheduledTime);
    isScheduling = true;
    scheduleLoop();
  }
}

function scheduleLoop() {
  if (!isScheduling) return;
  if (playQueue.length === 0) { isScheduling = false; return; }

  const chunk = playQueue.shift();
  const buffer = ctx.createBuffer(1, chunk.length, SAMPLE_RATE);
  buffer.copyToChannel(chunk, 0);
  const src = ctx.createBufferSource();
  src.buffer = buffer;
  src.connect(ctx.destination);

  const duration = buffer.duration;
  if (scheduledTime < ctx.currentTime) scheduledTime = ctx.currentTime + 0.01;
  src.start(scheduledTime);
  scheduledTime += duration;
  src.onended = () => {
    if (playQueue.length > 0) scheduleLoop();
    else isScheduling = false;
  };
}
