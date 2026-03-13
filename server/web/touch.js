(() => {
    window.touchInput = { deltaY: 0, active: false };

    let lastY = 0;
    let lastTime = 0;
    let endTimer = null;

    document.addEventListener('touchstart', e => {
        const t = e.touches[0];
        lastY = t.clientY;
        lastTime = performance.now();
        window.touchInput.active = true;
        if (endTimer) { clearTimeout(endTimer); endTimer = null; }
    }, { passive: true });

    document.addEventListener('touchmove', e => {
        const t = e.touches[0];
        const now = performance.now();
        const dt = (now - lastTime) / 1000; // seconds
        if (dt > 0) {
            const velocity = (t.clientY - lastY) / dt;
            // clamp to ±600 px/s
            window.touchInput.deltaY = Math.max(-600, Math.min(600, velocity));
        }
        lastY = t.clientY;
        lastTime = now;
    }, { passive: true });

    document.addEventListener('touchend', () => {
        endTimer = setTimeout(() => {
            window.touchInput.active = false;
            window.touchInput.deltaY = 0;
        }, 100);
    }, { passive: true });
})();
