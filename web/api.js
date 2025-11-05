// api.js — OpenAPI-compliant HTTP helpers (only endpoints from the spec)

const JSON_HEADERS = { 'Content-Type': 'application/json' };

async function http(method, url) {
    const res = await fetch(url, { method, credentials: 'include', headers: JSON_HEADERS });
    if (res.status === 204) return;
    const text = await res.text();
    const data = text ? JSON.parse(text) : {};
    if (!res.ok) throw new Error(data?.error || `${method} ${url} -> ${res.status}`);
    return data;
}

// Robust normalizers (accept both shapes):
// - members: [{ user:{id,username} }]  OR  [{ id, username }]
const norm = {
    roomName(item) {
        if (item == null) return '';
        if (typeof item === 'string') return item;
        if (typeof item.name === 'string') return item.name;
        if (typeof item.id === 'string') return item.id;
        return String(item);
    },
    member(m) {
        const direct = (typeof m?.username === 'string') || (typeof m?.id === 'string');
        const user = direct ? m : (m?.user || {});
        const id = typeof user.id === 'string' ? user.id : '';
        const username = typeof user.username === 'string' ? user.username : (typeof m?.username === 'string' ? m.username : '');
        return {
            id,
            username: username || (id ? `user-${id.slice(0, 4)}` : 'user')
        };
    }
};

export const Rooms = {
    async list() {
        const j = await http('GET', '/api/rooms');
        const arr = Array.isArray(j?.rooms) ? j.rooms : [];
        return arr.map(norm.roomName);
    },
    async info(name) {
        const j = await http('GET', `/api/rooms/${encodeURIComponent(name)}`);
        const members = Array.isArray(j?.members) ? j.members.map(norm.member) : [];
        const memberCount = Number.isFinite(j?.memberCount) ? j.memberCount : members.length;
        const roomName = norm.roomName(j?.name ?? name);
        return { name: roomName, memberCount, members };
    },
};

export const Me = {
    leave() { return http('POST', '/api/me/leave'); },
    move(to) { return http('POST', `/api/me/move?to=${encodeURIComponent(norm.roomName(to))}`); },
};

// Build WebSocket join URL per spec
export function joinURL({ name, room }) {
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    const qs = new URLSearchParams();
    qs.set('name', name);
    if (room) qs.set('room', room);
    return `${proto}//${location.host}/api/ws/join?${qs.toString()}`;
}
