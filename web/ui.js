// /static/ui.js
// Связка: UI <-> signal.js <-> webrtc.js

import { connect as connectSignal, getWSState, on as onSignal, send as sendSignal } from './signal.js';
import { initWebRTC, setIncomingEnabled, setMicEnabled, startVoice, stopVoice } from './webrtc.js';

const $ = (id) => document.getElementById(id);

// DOM
const wsStateEl = $('wsState');
const roomBadge = $('roomBadge');
const pcStateEl = $('pcState');
const pcHealthDot = $('pcHealthDot');

const roomIdInput = $('roomId');
const createRoomBtn = $('createRoom');
const joinRoomBtn = $('joinRoom');
const copyLinkBtn = $('copyLink');

const userNameInput = $('userName');
const renameBtn = $('renameBtn');

const toggleMicBtn = $('toggleMic');
const toggleSoundBtn = $('toggleSound');
const reconnectVoiceBtn = $('reconnectVoice');
const leaveRoomBtn = $('leaveRoom');

const membersEl = $('members');
const membersEmptyEl = $('membersEmpty');
const logEl = $('log');

// state
let currentRoomId = '';
let currentRoomName = '';
let inRoom = false;
let voiceActive = false;

let micOn = true;
let soundOn = true;

let currentMembers = [];

// логгер
function log(msg) {
    const line = `[${new Date().toLocaleTimeString()}] ${msg}`;
    logEl.textContent += line + '\n';
    logEl.scrollTop = logEl.scrollHeight;
    console.log(line);
}

// WebRTC status + health
function setPCState(stateStr) {
    pcStateEl.textContent = `PC: ${stateStr}`;
    let cls = 'state-warn';

    const s = (stateStr || '').toLowerCase();
    if (s.includes('connected') || s.includes('completed')) {
        cls = 'state-ok';
    } else if (s.includes('failed') || s.includes('disconnected') || s.includes('closed') || s.includes('stopped')) {
        cls = 'state-bad';
    }

    pcHealthDot.classList.remove('state-ok', 'state-warn', 'state-bad');
    pcHealthDot.classList.add(cls);
}

// нормализатор участника
function normalizeMember(m) {
    const direct = (typeof m?.username === 'string') || (typeof m?.id === 'string');
    const user = direct ? m : (m?.user || {});
    const id = typeof user.id === 'string' ? user.id : '';
    const username = typeof user.username === 'string'
        ? user.username
        : (typeof m?.username === 'string' ? m.username : '');
    return {
        id,
        username: username || (id ? `user-${id.slice(0, 4)}` : 'user'),
    };
}

function renderMembers(list) {
    membersEl.innerHTML = '';
    if (!Array.isArray(list) || list.length === 0) {
        membersEmptyEl.style.display = 'block';
        return;
    }
    membersEmptyEl.style.display = 'none';

    const sorted = [...list].map(normalizeMember)
        .sort((a, b) => a.username.localeCompare(b.username));

    for (const m of sorted) {
        const wrap = document.createElement('div');
        wrap.className = 'member';

        const avatar = document.createElement('div');
        avatar.className = 'avatar';
        const initial = (m.username || '?').slice(0, 1).toUpperCase();
        avatar.textContent = initial;

        const name = document.createElement('div');
        name.className = 'member-name';
        name.textContent = m.username;

        wrap.appendChild(avatar);
        wrap.appendChild(name);
        membersEl.appendChild(wrap);
    }
}

// join helper
function joinRoom(roomId, withName = true) {
    const room = (roomId || '').trim();
    if (!room) {
        log('Join: room id is empty');
        return;
    }
    const payload = { type: 'join', room };
    const name = userNameInput.value.trim();
    if (withName && name) {
        payload.name = name;
    }
    try {
        sendSignal(payload);
        log(`JOIN → room=${room}${payload.name ? ', name=' + payload.name : ''}`);
    } catch (err) {
        log('JOIN send error: ' + err);
    }
}

// leave helper
function leaveRoom() {
    try {
        sendSignal({ type: 'leave' });
        log('LEAVE → sent');
    } catch (err) {
        log('LEAVE send error: ' + err);
    }
}

function buildInviteLink(roomId) {
    const url = new URL(window.location.href);
    url.searchParams.set('room', roomId);
    return url.toString();
}

/* ===========================
   INIT
   =========================== */

initWebRTC({
    onLog: log,
    onStatus: setPCState,
    remoteContainerId: 'remoteAudio',
});

connectSignal();

wsStateEl.textContent = getWSState();

const urlParams = new URLSearchParams(window.location.search);
let roomFromUrl = (urlParams.get('room') || '').trim();
if (roomFromUrl) {
    roomIdInput.value = roomFromUrl;
}

// SIGNAL EVENTS

onSignal('ws_state', (s) => {
    wsStateEl.textContent = s;
});

onSignal('ws_open', () => {
    log('WS: connected');

    if (roomFromUrl && !inRoom) {
        joinRoom(roomFromUrl, false);
    }

    try {
        sendSignal({ type: 'whoami' });
    } catch (err) {
        log('WHOAMI send error: ' + err);
    }
});

onSignal('ws_close', () => {
    log('WS: closed');
    wsStateEl.textContent = 'closed';
});

// whoami
onSignal('whoami', (msg) => {
    if (msg.username && !userNameInput.value) {
        userNameInput.value = msg.username;
    }
    if (msg.room) {
        currentRoomId = msg.room;
        currentRoomName = msg.room_name || '';
        inRoom = true;

        roomIdInput.value = currentRoomId;
        roomBadge.textContent = currentRoomName || currentRoomId;

        log(`WHOAMI → username=${msg.username}, room=${currentRoomId} (${currentRoomName || 'no name'})`);
    } else {
        log(`WHOAMI → username=${msg.username}`);
    }
});

// создание комнаты
onSignal('room_created', (msg) => {
    const room = msg.room;
    if (!room) return;
    log(`ROOM CREATED: ${room}`);
    currentRoomId = room;
    currentRoomName = msg.room_name || '';

    roomIdInput.value = room;
    roomBadge.textContent = currentRoomName || room;

    const url = new URL(window.location.href);
    url.searchParams.set('room', room);
    window.history.replaceState(null, '', url.toString());

    joinRoom(room, true);
});

// состояние комнаты
onSignal('room_state', async (msg) => {
    const room = msg.room;
    const roomName = msg.room_name || '';

    inRoom = true;
    currentRoomId = room;
    currentRoomName = roomName;

    roomBadge.textContent = roomName || room;
    roomIdInput.value = room;

    currentMembers = Array.isArray(msg.members) ? msg.members.slice() : [];
    renderMembers(currentMembers);

    log(`ROOM STATE: room=${room}, name=${roomName}, members=${msg.count}`);

    if (!voiceActive) {
        try {
            await startVoice();
            voiceActive = true;
            setMicEnabled(micOn);
            setIncomingEnabled(soundOn);
        } catch (err) {
            log('Failed to start voice: ' + err);
            voiceActive = false;
        }
    }
});

// member join/leave/update
onSignal('member_joined', (msg) => {
    if (!inRoom) return;
    if (!msg.user) return;
    const u = normalizeMember(msg.user);
    currentMembers = [
        ...currentMembers.filter((m) => normalizeMember(m).id !== u.id),
        msg.user,
    ];
    renderMembers(currentMembers);
    log(`MEMBER JOINED: ${u.username}`);
});

onSignal('member_left', (msg) => {
    if (!inRoom) return;
    if (!msg.user) return;
    const u = normalizeMember(msg.user);
    currentMembers = currentMembers.filter(
        (m) => normalizeMember(m).id !== u.id,
    );
    renderMembers(currentMembers);
    log(`MEMBER LEFT: ${u.username}`);
});

onSignal('member_updated', (msg) => {
    if (!inRoom) return;
    if (!msg.user) return;
    const u = normalizeMember(msg.user);
    currentMembers = currentMembers.map((m) => {
        const mm = normalizeMember(m);
        return mm.id === u.id ? msg.user : m;
    });
    renderMembers(currentMembers);
    log(`MEMBER UPDATED: ${u.username}`);
});

// выход
onSignal('left', () => {
    log('LEFT room');
    inRoom = false;
    currentRoomId = '';
    currentRoomName = '';
    roomBadge.textContent = '—';
    membersEl.innerHTML = '';
    membersEmptyEl.style.display = 'block';
    currentMembers = [];

    if (voiceActive) {
        stopVoice();
        voiceActive = false;
    }
});

// ошибки
onSignal('error', (msg) => {
    const err = msg.error || 'unknown error';
    log('ERROR: ' + err);
});

// pong
onSignal('pong', () => {
    log('PONG');
});

/* ===========================
   UI HANDLERS
   =========================== */

createRoomBtn.addEventListener('click', () => {
    try {
        // сервер сейчас поддерживает имя комнаты через Name, но
        // мы пока не задаём его явно — пусть будет без имени или по умолчанию
        sendSignal({ type: 'create_room' });
        log('CREATE_ROOM → sent');
    } catch (err) {
        log('CREATE_ROOM send error: ' + err);
    }
});

joinRoomBtn.addEventListener('click', () => {
    joinRoom(roomIdInput.value, true);
});

copyLinkBtn.addEventListener('click', async () => {
    const room = (currentRoomId || roomIdInput.value || '').trim();
    if (!room) {
        log('Nothing to copy: no room id');
        return;
    }
    const link = buildInviteLink(room);
    try {
        await navigator.clipboard.writeText(link);
        log('Invite link copied to clipboard');
    } catch (_e) {
        log('Failed to copy link: ' + link);
    }
});

renameBtn.addEventListener('click', () => {
    const name = userNameInput.value.trim();
    if (!name) {
        log('Rename: empty name');
        return;
    }
    try {
        sendSignal({ type: 'rename', name });
        log('RENAME → ' + name);
    } catch (err) {
        log('RENAME send error: ' + err);
    }
});

toggleMicBtn.addEventListener('click', () => {
    micOn = !micOn;
    setMicEnabled(micOn);
    toggleMicBtn.textContent = micOn ? 'Mic: On' : 'Mic: Off';
    toggleMicBtn.classList.toggle('toggled', !micOn);
});

toggleSoundBtn.addEventListener('click', () => {
    soundOn = !soundOn;
    setIncomingEnabled(soundOn);
    toggleSoundBtn.textContent = soundOn ? 'Sound: On' : 'Sound: Off';
    toggleSoundBtn.classList.toggle('toggled', !soundOn);
});

reconnectVoiceBtn.addEventListener('click', async () => {
    if (!inRoom || !currentRoomId) {
        log('Reconnect: not in room');
        return;
    }
    try {
        stopVoice();
        voiceActive = false;
        await startVoice();
        voiceActive = true;
        setMicEnabled(micOn);
        setIncomingEnabled(soundOn);
        log('Voice reconnected');
    } catch (err) {
        log('Reconnect error: ' + err);
    }
});

leaveRoomBtn.addEventListener('click', () => {
    if (!inRoom) {
        log('Leave: not in room');
        return;
    }
    leaveRoom();
});

roomIdInput.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') {
        joinRoom(roomIdInput.value, true);
    }
});

// начальное состояние PC health
setPCState('idle');
