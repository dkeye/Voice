// /static/net-stats.js
// Простой счётчик трафика (WS + WebRTC) с выводом в хедер.

const $ = (id) => document.getElementById(id);

const st = {
    tx: 0,
    rx: 0,
    txPrev: 0,
    rxPrev: 0,
};

function fmt(n) {
    const u = ['B', 'KB', 'MB', 'GB'];
    let i = 0;
    let x = n;
    while (x >= 1024 && i < u.length - 1) {
        x /= 1024;
        i++;
    }
    const digits = x >= 10 ? 0 : 1;
    return `${x.toFixed(digits)} ${u[i]}`;
}

setInterval(() => {
    const dtx = st.tx - st.txPrev;
    const drx = st.rx - st.rxPrev;
    st.txPrev = st.tx;
    st.rxPrev = st.rx;

    const txR = $('txRate');
    const rxR = $('rxRate');
    const txT = $('txTotal');
    const rxT = $('rxTotal');

    if (txR) txR.textContent = `${fmt(dtx)}/s`;
    if (rxR) rxR.textContent = `${fmt(drx)}/s`;
    if (txT) txT.textContent = fmt(st.tx);
    if (rxT) rxT.textContent = fmt(st.rx);
}, 1000);

export const NetStats = {
    countOut(n) {
        if (Number.isFinite(n) && n > 0) st.tx += n;
    },
    countIn(n) {
        if (Number.isFinite(n) && n > 0) st.rx += n;
    },
    reset() {
        st.tx = st.rx = st.txPrev = st.rxPrev = 0;
    },
};
