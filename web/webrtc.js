// /static/webrtc.js
// WebRTC-аудио: медиаподключение, сигналинг через signal.js.

import { NetStats } from './net-stats.js';
import { on as onSignal, send as sendSignal } from './signal.js';

let pc = null;
let localStream = null;
let pendingCandidates = [];
let gotInitialAnswer = false;
let isRenegotiating = false;

let log = () => { };
let setStatus = () => { };
let remoteContainer = null;
let micEnabled = true;
let incomingEnabled = true;

let statsTimer = null;
let lastBytes = { sent: 0, recv: 0 };

/**
 * Инициализация: задаём хуки логирования и ID контейнера для удалённого звука.
 */
export function initWebRTC({ onLog, onStatus, remoteContainerId = 'remoteAudio' } = {}) {
    log = typeof onLog === 'function' ? onLog : () => { };
    setStatus = typeof onStatus === 'function' ? onStatus : () => { };
    remoteContainer = document.getElementById(remoteContainerId) || document.body;

    onSignal('answer', handleAnswerMessage);
    onSignal('offer', handleOfferMessage);
    onSignal('candidate', handleCandidateMessage);
}

/* ===========================
   PUBLIC API
   =========================== */

export async function startVoice() {
    if (pc) {
        stopVoice();
    }

    if (!remoteContainer) {
        remoteContainer = document.getElementById('remoteAudio') || document.body;
    }

    gotInitialAnswer = false;
    pendingCandidates = [];
    isRenegotiating = false;
    lastBytes = { sent: 0, recv: 0 };

    pc = new RTCPeerConnection({
        iceServers: [
            { urls: 'stun:stun.l.google.com:19302' },
            { urls: 'stun:stun.cloudflare.com:3478' },
        ],
    });

    setStatus('created');
    log('PC created');

    pc.onicecandidate = (e) => {
        if (e.candidate) {
            sendSignal({
                type: 'candidate',
                candidate: e.candidate.candidate,
                sdpMid: e.candidate.sdpMid,
                sdpMLineIndex: e.candidate.sdpMLineIndex,
            });
        } else {
            sendSignal({ type: 'candidate', candidate: '' });
        }
    };

    pc.oniceconnectionstatechange = () => {
        log(`ICE: ${pc.iceConnectionState}`);
        setStatus(`ice:${pc.iceConnectionState}`);
    };

    pc.onconnectionstatechange = () => {
        log(`PC state: ${pc.connectionState}`);
    };

    pc.ontrack = (evt) => {
        log(`ontrack: ${evt.track.kind}`);
        if (evt.track.kind !== 'audio') return;

        const cont = document.getElementById('remoteAudio') || remoteContainer;
        const a = document.createElement('audio');
        a.autoplay = true;
        a.controls = false;
        a.muted = !incomingEnabled;

        const s = new MediaStream();
        s.addTrack(evt.track);
        a.srcObject = s;

        cont.appendChild(a);
    };

    try {
        localStream = await navigator.mediaDevices.getUserMedia({
            audio: {
                echoCancellation: true,
                noiseSuppression: true,
                autoGainControl: true,
            },
        });
        for (const t of localStream.getTracks()) {
            t.enabled = micEnabled;
            pc.addTrack(t, localStream);
        }
        log('Local audio READY');
    } catch (err) {
        log('ERROR getUserMedia: ' + err);
        stopVoice();
        throw err;
    }

    log('Creating OFFER...');
    const offer = await pc.createOffer();
    await pc.setLocalDescription(offer);

    sendSignal({ type: 'offer', sdp: offer.sdp });
    log('OFFER sent');

    // Стартуем сбор статистики
    if (!statsTimer) {
        statsTimer = setInterval(collectRTCStats, 1000);
    }
}

export function stopVoice() {
    log('Stopping WebRTC');

    if (pc) {
        try { pc.close(); } catch (_e) { }
        pc = null;
    }
    if (localStream) {
        for (const t of localStream.getTracks()) {
            try { t.stop(); } catch (_e) { }
        }
        localStream = null;
    }
    pendingCandidates = [];
    gotInitialAnswer = false;
    isRenegotiating = false;

    if (statsTimer) {
        clearInterval(statsTimer);
        statsTimer = null;
    }

    setStatus('stopped');
}

export function setMicEnabled(enabled) {
    micEnabled = !!enabled;
    if (localStream) {
        for (const t of localStream.getAudioTracks()) {
            t.enabled = micEnabled;
        }
    }
    log(`Mic: ${micEnabled ? 'On' : 'Off'}`);
}

export function setIncomingEnabled(enabled) {
    incomingEnabled = !!enabled;
    const cont = document.getElementById('remoteAudio') || remoteContainer;
    if (cont) {
        cont.querySelectorAll('audio').forEach((a) => {
            a.muted = !incomingEnabled;
        });
    }
    log(`Incoming audio: ${incomingEnabled ? 'On' : 'Off'}`);
}

/* ===========================
   RTC STATS → NetStats
   =========================== */

async function collectRTCStats() {
    if (!pc) return;

    try {
        const stats = await pc.getStats();
        let sent = 0;
        let recv = 0;

        stats.forEach((report) => {
            if (report.type === 'outbound-rtp' && report.kind === 'audio' && typeof report.bytesSent === 'number') {
                sent += report.bytesSent;
            }
            if (report.type === 'inbound-rtp' && report.kind === 'audio' && typeof report.bytesReceived === 'number') {
                recv += report.bytesReceived;
            }
        });

        if (sent > lastBytes.sent) {
            NetStats.countOut(sent - lastBytes.sent);
        }
        if (recv > lastBytes.recv) {
            NetStats.countIn(recv - lastBytes.recv);
        }

        lastBytes = { sent, recv };
    } catch (err) {
        // статистика не критична
        console.debug('collectRTCStats error', err);
    }
}

/* ===========================
   SIGNAL HANDLERS
   =========================== */

async function handleAnswerMessage(msg) {
    if (!pc) return;

    if (gotInitialAnswer) {
        log('WARN: duplicate initial answer');
        return;
    }
    gotInitialAnswer = true;

    try {
        await pc.setRemoteDescription({ type: 'answer', sdp: msg.sdp });
        log('SET REMOTE ANSWER');

        for (const c of pendingCandidates) {
            try {
                await pc.addIceCandidate(c);
                log('APPLIED queued ICE');
            } catch (err) {
                log('ERROR applying queued ICE: ' + err);
            }
        }
        pendingCandidates = [];
    } catch (err) {
        log('ERROR setRemoteDescription(answer): ' + err);
    }
}

async function handleOfferMessage(msg) {
    if (!pc) return;
    if (isRenegotiating) {
        log('WARN: renegotiation already in progress');
        return;
    }
    isRenegotiating = true;
    log('SERVER OFFER received → renegotiating');

    try {
        await pc.setRemoteDescription({ type: 'offer', sdp: msg.sdp });
        log('setRemoteDescription(server-offer)');

        const answer = await pc.createAnswer();
        await pc.setLocalDescription(answer);
        log('created + setLocalDescription(answer)');

        sendSignal({ type: 'answer', sdp: answer.sdp });
    } catch (err) {
        log('ERROR renegotiation: ' + err);
    }

    isRenegotiating = false;
}

async function handleCandidateMessage(msg) {
    if (!pc) return;

    if (!msg.candidate) {
        log('REMOTE end-of-candidates');
        return;
    }

    const ice = {
        candidate: msg.candidate,
        sdpMid: msg.sdpMid,
        sdpMLineIndex: msg.sdpMLineIndex,
    };

    if (!pc.remoteDescription) {
        log('QUEUE REMOTE ICE (remoteDescription missing)');
        pendingCandidates.push(ice);
        return;
    }

    try {
        await pc.addIceCandidate(ice);
        log('ADD REMOTE ICE OK');
    } catch (err) {
        log('ERROR addIceCandidate: ' + err);
    }
}
