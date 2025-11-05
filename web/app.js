// app.js — WS + Audio. UI is wired in ui.js through window.UIHooks.

import { joinURL } from './api.js';

const $ = (id) => document.getElementById(id);
const userName = $('userName');
const connectBtn = $('connect');
const talkBtn = $('talk');
const logEl = $('log');
const roomBadge = $('roomBadge');

function log(msg) {
  const ts = new Date().toLocaleTimeString();
  logEl.textContent += `[${ts}] ${msg}\n`;
  logEl.scrollTop = logEl.scrollHeight;
}

let ws = null;
let isTalking = false;

// Audio state (isolated)
let audioCtx = null, mediaStream = null, micNode = null;
let playHeadTime = 0, playRate = 48000;

function floatToPCM16LE(f32) {
  const out = new Int16Array(f32.length);
  for (let i = 0; i < f32.length; i++) { const s = Math.max(-1, Math.min(1, f32[i])); out[i] = s < 0 ? s * 0x8000 : s * 0x7fff; }
  return out;
}
function pcm16ToFloat32(buf) {
  const v = buf instanceof Int16Array ? buf : new Int16Array(buf);
  const f = new Float32Array(v.length);
  for (let i = 0; i < v.length; i++) f[i] = v[i] / 0x8000;
  return f;
}
function wsOpen() { return ws && ws.readyState === WebSocket.OPEN; }

async function initAudio() {
  if (audioCtx) return;
  audioCtx = new (window.AudioContext || window.webkitAudioContext)({ latencyHint: 'interactive' });
  mediaStream = await navigator.mediaDevices.getUserMedia({ audio: true });
  await audioCtx.audioWorklet.addModule('/static/mic-worklet.js');
  const src = audioCtx.createMediaStreamSource(mediaStream);
  micNode = new AudioWorkletNode(audioCtx, 'mic-worklet');
  src.connect(micNode);

  // Mic → WS
  micNode.port.onmessage = (ev) => {
    const chunk = ev.data;
    if (!(chunk instanceof Float32Array)) return;
    if (isTalking && wsOpen()) {
      const pcm = floatToPCM16LE(chunk);
      const ab = pcm.buffer;
      window.NetStats?.countOut(ab.byteLength);
      ws.send(ab);
    }
  };
  log('Audio ready');
}

function teardownAudio() {
  try { micNode?.disconnect(); micNode?.port.close(); } catch { }
  mediaStream?.getTracks?.().forEach(t => t.stop());
  mediaStream = null; micNode = null;
  if (audioCtx) { audioCtx.close().catch(() => { }); audioCtx = null; }
}

function schedulePlaybackFloat32(f32) {
  if (!audioCtx) return;
  const ctx = audioCtx;
  const rate = playRate || ctx.sampleRate;
  const buf = ctx.createBuffer(1, f32.length, rate);
  buf.copyToChannel(f32, 0, 0);
  const src = ctx.createBufferSource();
  src.buffer = buf; src.connect(ctx.destination);
  const now = ctx.currentTime;
  if (playHeadTime < now) playHeadTime = now + 0.02;
  src.start(playHeadTime);
  playHeadTime += buf.duration;
}

function handleIncomingBinary(data) {
  try {
    let n = 0;
    if (data instanceof ArrayBuffer) n = data.byteLength;
    else if (ArrayBuffer.isView(data)) n = data.byteLength;
    else if (data instanceof Blob) n = data.size;
    window.NetStats?.countIn(n);
  } catch { }
  if (data instanceof ArrayBuffer) schedulePlaybackFloat32(pcm16ToFloat32(data));
  else if (ArrayBuffer.isView(data)) schedulePlaybackFloat32(pcm16ToFloat32(data.buffer));
  else if (data instanceof Blob) data.arrayBuffer().then(ab => schedulePlaybackFloat32(pcm16ToFloat32(ab))).catch(() => { });
}

function closeWS(code = 1000, reason = 'client closing') {
  if (!ws) return;
  try { ws.close(code, reason); } catch { }
  ws = null;
}

async function connectWS() {
  const name = (userName.value || '').trim();
  const room = (roomBadge.textContent || 'main').trim() || 'main';
  if (!name) { log('Enter Username first'); return; }

  if (!audioCtx) {
    try { await initAudio(); } catch (e) { log('Audio init failed'); return; }
  }

  closeWS();
  const url = joinURL({ name, room });
  log(`WS connecting → ${url}`);
  ws = new WebSocket(url);
  ws.binaryType = 'arraybuffer';
  ws.addEventListener('open', () => {
    window.NetStats?.reset();
    window.UIHooks?.onConnected?.(room);
    if (talkBtn) talkBtn.disabled = false;
    log('WS connected');
  });
  ws.addEventListener('message', (ev) => handleIncomingBinary(ev.data));
  ws.addEventListener('close', () => {
    log('WS closed');
    window.UIHooks?.onDisconnected?.();
    if (talkBtn) talkBtn.disabled = true;
    if (isTalking) setTalking(false);
    connectBtn.disabled = false;
  });
  ws.addEventListener('error', () => {
    window.UIHooks?.onDisconnected?.();
    log('WS error');
  });
}

function setTalking(flag) {
  isTalking = !!flag;
  if (flag && audioCtx && audioCtx.state === 'suspended') audioCtx.resume().catch(() => { });
  talkBtn.textContent = isTalking ? '🔴 Talking…' : '🎙️ Push-to-Talk';
}
window.setTalking = setTalking;

connectBtn?.addEventListener('click', () => connectWS());
window.addEventListener('beforeunload', () => { closeWS(); teardownAudio(); });
