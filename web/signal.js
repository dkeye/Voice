// /static/signal.js
// Один постоянный WebSocket на /api/ws/signal.

import { NetStats } from './net-stats.js';

let ws = null;
let wsState = 'idle'; // idle | connecting | open | closed
const listeners = new Map(); // type -> Set<fn>

function emit(type, payload) {
    const set = listeners.get(type);
    if (set) {
        for (const fn of set) {
            try { fn(payload); } catch (e) { console.error(e); }
        }
    }
    const any = listeners.get('*');
    if (any) {
        for (const fn of any) {
            try { fn(type, payload); } catch (e) { console.error(e); }
        }
    }
}

export function on(type, handler) {
    if (!listeners.has(type)) listeners.set(type, new Set());
    listeners.get(type).add(handler);
    return () => {
        listeners.get(type)?.delete(handler);
    };
}

export function getWSState() {
    return wsState;
}

export function connect() {
    if (ws && (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING)) {
        return;
    }

    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    const url = `${proto}//${location.host}/api/ws/signal`;

    wsState = 'connecting';
    emit('ws_state', wsState);

    ws = new WebSocket(url);

    ws.onopen = () => {
        wsState = 'open';
        NetStats.reset();
        emit('ws_open');
        emit('ws_state', wsState);
    };

    ws.onclose = () => {
        wsState = 'closed';
        emit('ws_close');
        emit('ws_state', wsState);
    };

    ws.onerror = (e) => {
        console.error('WS error', e);
        emit('ws_error', e);
    };

    ws.onmessage = (ev) => {
        if (typeof ev.data === 'string') {
            NetStats.countIn(ev.data.length);
        }

        let msg;
        try {
            msg = JSON.parse(ev.data);
        } catch (err) {
            console.error('WS parse error', err, ev.data);
            emit('ws_error', { kind: 'parse', raw: ev.data });
            return;
        }
        const t = msg?.type || 'unknown';
        emit(t, msg);
    };
}

export function send(msg) {
    if (!ws || ws.readyState !== WebSocket.OPEN) {
        console.warn('WS not open, cannot send', msg);
        throw new Error('WebSocket is not open');
    }
    const txt = JSON.stringify(msg);
    NetStats.countOut(txt.length);
    ws.send(txt);
}
