(() => {
    let ctx = null;

    function getCtx() {
        if (!ctx) ctx = new (window.AudioContext || window.webkitAudioContext)();
        return ctx;
    }

    function beep({ type = 'sine', freq = 440, freqEnd = null, gain = 0.3, duration = 0.05 }) {
        const ac = getCtx();
        const osc = ac.createOscillator();
        const env = ac.createGain();

        osc.connect(env);
        env.connect(ac.destination);

        osc.type = type;
        osc.frequency.setValueAtTime(freq, ac.currentTime);
        if (freqEnd !== null) {
            osc.frequency.linearRampToValueAtTime(freqEnd, ac.currentTime + duration);
        }

        env.gain.setValueAtTime(0, ac.currentTime);
        env.gain.linearRampToValueAtTime(gain, ac.currentTime + 0.005);
        env.gain.linearRampToValueAtTime(0, ac.currentTime + duration);

        osc.start(ac.currentTime);
        osc.stop(ac.currentTime + duration + 0.01);
    }

    window.audioPlay = {
        bounce() {
            beep({ type: 'sine', freq: 440, gain: 0.25, duration: 0.05 });
        },
        score() {
            beep({ type: 'sine', freq: 300, freqEnd: 800, gain: 0.35, duration: 0.15 });
        },
        hit() {
            beep({ type: 'sawtooth', freq: 80, gain: 0.45, duration: 0.2 });
        },
        countdown() {
            beep({ type: 'square', freq: 600, gain: 0.28, duration: 0.03 });
        },
        go() {
            // C major chord: C5 E5 G5
            [523, 659, 784].forEach(f =>
                beep({ type: 'sine', freq: f, gain: 0.2, duration: 0.3 })
            );
        },
        gameOver() {
            beep({ type: 'sine', freq: 400, freqEnd: 80, gain: 0.4, duration: 0.5 });
        },
    };
})();
