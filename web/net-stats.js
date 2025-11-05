// net-stats.js — simple ↑/↓ counters bound to header badges
const $ = (id) => document.getElementById(id);
const st = { tx: 0, rx: 0, txPrev: 0, rxPrev: 0 };

function fmt(n) { const u = ['B', 'KB', 'MB', 'GB']; let i = 0, x = n; while (x >= 1024 && i < u.length - 1) { x /= 1024; i++; } return `${x.toFixed(x >= 10 ? 0 : 1)} ${u[i]}`; }

setInterval(() => {
    const dtx = st.tx - st.txPrev, drx = st.rx - st.rxPrev;
    st.txPrev = st.tx; st.rxPrev = st.rx;
    const txR = $('txRate'), rxR = $('rxRate'), txT = $('txTotal'), rxT = $('rxTotal');
    if (txR) txR.textContent = `${fmt(dtx)}/s`;
    if (rxR) rxR.textContent = `${fmt(drx)}/s`;
    if (txT) txT.textContent = fmt(st.tx);
    if (rxT) rxT.textContent = fmt(st.rx);
}, 1000);

window.NetStats = {
    countOut(n) { if (Number.isFinite(n)) st.tx += n; },
    countIn(n) { if (Number.isFinite(n)) st.rx += n; },
    reset() { st.tx = st.rx = st.txPrev = st.rxPrev = 0; },
};
