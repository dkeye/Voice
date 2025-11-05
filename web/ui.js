// ui.js — Discord-like UI with nested members; minimal DOM churn to avoid flicker.

import { Me, Rooms } from './api.js';

const $ = (id) => document.getElementById(id);

const roomsEl = $('rooms');
const roomsEmpty = $('roomsEmpty');

const roomBadge = $('roomBadge');
const userName = $('userName');
const roomInput = $('roomInput');
const connectBtn = $('connect');
const leaveBtn = $('leave');
const talkBtn = $('talk');
const toggleMode = $('toggleMode');
const wsState = $('wsState');

let currentRoom = 'main';
let wsConnected = false;

// cache to prevent flicker & preserve order
let cachedRoomNames = [];           // sorted array
const cachedMembersHTML = new Map(); // roomName -> last HTML string
const visibleRooms = new Set();      // non-empty rooms

function setWSState(s) { wsState.textContent = s; }

userName.addEventListener('input', () => { connectBtn.disabled = !userName.value.trim(); });
roomInput.addEventListener('input', () => {
    const v = (roomInput.value || '').trim() || 'main';
    roomBadge.textContent = v;
    currentRoom = v;
});

// ---------- Rendering (rooms skeleton once) ----------
function buildRooms(namesSorted) {
    roomsEl.innerHTML = '';
    roomsEmpty.style.display = namesSorted.length ? 'none' : 'block';

    for (const name of namesSorted) {
        const wrap = document.createElement('div');
        wrap.className = 'room-group';
        wrap.dataset.room = name;

        const head = document.createElement('div');
        head.className = 'room';
        head.dataset.room = name;
        head.innerHTML = `
      <div class="ic">🔊</div>
      <div class="name mono">${name}</div>
      <div class="badge mono" id="count-${CSS.escape(name)}">0</div>
    `;
        head.addEventListener('click', async () => {
            const uname = userName.value.trim();
            // Autoconnect on first click if username entered
            if (!wsConnected && uname) {
                roomInput.value = name;
                roomBadge.textContent = name;
                currentRoom = name;
                connectBtn.click();
                return;
            }
            if (!wsConnected) return;
            if (currentRoom === name) return;
            try {
                await Me.move(name);
                currentRoom = name; roomBadge.textContent = name; roomInput.value = name;
                selectActiveRoom();
            } catch (e) { console.error(e); }
        });

        const sub = document.createElement('div');
        sub.className = 'members-sub';
        sub.id = `sub-${CSS.escape(name)}`;
        sub.style.display = 'none';

        wrap.appendChild(head);
        wrap.appendChild(sub);
        roomsEl.appendChild(wrap);
    }
    selectActiveRoom();
}

function selectActiveRoom() {
    for (const n of roomsEl.querySelectorAll('.room')) {
        const active = n.dataset.room === currentRoom;
        n.classList.toggle('active', active);
        const sub = document.getElementById(`sub-${CSS.escape(n.dataset.room)}`);
        if (sub) sub.style.display = (active && visibleRooms.has(n.dataset.room)) ? 'block' : 'none';
    }
}

// ---------- Loader with minimal DOM updates ----------
async function loadRoomsAndMembers() {
    try {
        // 1) Stable sorted names
        let names = await Rooms.list();
        names = (Array.isArray(names) ? names : []).map(x => (typeof x === 'string' ? x : (x?.name || x?.id || String(x)))).sort();

        // rebuild skeleton only if set of names changed
        if (cachedRoomNames.join('|') !== names.join('|')) {
            cachedRoomNames = names;
            buildRooms(names);
            cachedMembersHTML.clear();
            visibleRooms.clear();
        }

        // 2) Fetch infos; filter non-empty; update badges+sublists without flicker
        const infos = await Promise.all(names.map(n => Rooms.info(n).catch(() => null)));
        visibleRooms.clear();
        for (const info of infos) {
            if (!info) continue;
            const { name, members, memberCount } = info;
            const badge = document.getElementById(`count-${CSS.escape(name)}`);
            if (badge) badge.textContent = String(memberCount ?? 0);

            const sub = document.getElementById(`sub-${CSS.escape(name)}`);
            if (!sub) continue;

            if ((memberCount ?? 0) > 0 || name === currentRoom) {
                visibleRooms.add(name);
                // Build stable, sorted members by username
                const sorted = [...members].sort((a, b) => (a?.username || '').localeCompare(b?.username || ''));
                const html = sorted.map(m => {
                    const nm = (m?.username || '').trim() || `user-${(m?.id || '').slice(0, 4)}`;
                    const initial = nm.slice(0, 1).toUpperCase() || '?';
                    return `<div class="member"><div class="avatar">${initial}</div><div class="mono-sm">${nm}</div></div>`;
                }).join('');

                if (cachedMembersHTML.get(name) !== html) {
                    cachedMembersHTML.set(name, html);
                    sub.innerHTML = html; // update only if changed -> no flicker
                }
            } else {
                // empty room: hide sublist and clear cached html
                cachedMembersHTML.set(name, '');
                sub.innerHTML = '';
            }
        }

        // 3) Reflect active/visible state
        selectActiveRoom();
    } catch (e) { console.error(e); }
}

// poll
setInterval(loadRoomsAndMembers, 3000);

// ---------- Controls ----------
leaveBtn.addEventListener('click', async () => {
    try {
        await Me.leave();
        visibleRooms.clear();
        cachedMembersHTML.clear();
        await loadRoomsAndMembers();
    } catch (e) { console.error(e); }
    window.UIHooks?.onDisconnected?.();
    setWSState('idle');
    talkBtn.disabled = true;
    connectBtn.disabled = false;
    window.setTalking?.(false);
});

// PTT
let toggleState = false;
talkBtn.onmousedown = () => {
    if (toggleMode.checked) { toggleState = !toggleState; window.setTalking?.(toggleState); }
    else window.setTalking?.(true);
};
talkBtn.onmouseup = () => { if (!toggleMode.checked) window.setTalking?.(false); };
talkBtn.onmouseleave = () => { if (!toggleMode.checked) window.setTalking?.(false); };

// ---------- Init ----------
await loadRoomsAndMembers();

// WS Hooks
window.UIHooks = {
    onConnected(room) {
        wsConnected = true;
        setWSState('connected');
        talkBtn.disabled = false;
        connectBtn.disabled = true;
        if (room) { currentRoom = room; roomBadge.textContent = room; roomInput.value = room; selectActiveRoom(); }
    },
    onDisconnected() {
        wsConnected = false;
        setWSState('idle');
        talkBtn.disabled = true;
        connectBtn.disabled = false;
        // visually collapse sublists
        for (const s of document.querySelectorAll('.members-sub')) s.style.display = 'none';
    },
};
