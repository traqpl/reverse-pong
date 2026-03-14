(() => {
    let ctx = null;

    function ac() {
        if (!ctx) {
            ctx = new (window.AudioContext || window.webkitAudioContext)();
        }
        return ctx;
    }

    // ── SFX ──────────────────────────────────────────────────────────────────

    function beep({ type = 'sine', freq = 440, freqEnd = null, gain = 0.3, duration = 0.05 }) {
        const a = ac();
        const osc = a.createOscillator();
        const env = a.createGain();
        osc.connect(env);
        env.connect(a.destination);
        osc.type = type;
        osc.frequency.setValueAtTime(freq, a.currentTime);
        if (freqEnd !== null)
            osc.frequency.linearRampToValueAtTime(freqEnd, a.currentTime + duration);
        env.gain.setValueAtTime(0, a.currentTime);
        env.gain.linearRampToValueAtTime(gain, a.currentTime + 0.005);
        env.gain.linearRampToValueAtTime(0, a.currentTime + duration);
        osc.start(a.currentTime);
        osc.stop(a.currentTime + duration + 0.01);
    }

    // ── Music (MP3) ───────────────────────────────────────────────────────────

    const music = new Audio('music.mp3');
    music.loop = true;
    music.volume = 0.35;

    let musicState = 'stopped'; // 'stopped' | 'menu' | 'game'

    function startMenuMusic() {
        if (musicState === 'menu') return;
        musicState = 'menu';
        music.volume = 0.35;
        if (music.paused) music.play().catch(() => {});
    }

    function startGameMusic() {
        if (musicState === 'game') return;
        musicState = 'game';
        music.volume = 0.22; // quieter so SFX cuts through
        if (music.paused) music.play().catch(() => {});
    }

    function stopMusic() {
        musicState = 'stopped';
        music.pause();
    }

    // Start menu music on first user interaction (browsers require a gesture).
    const startMenuOnce = () => startMenuMusic();
    ['keydown', 'mousedown', 'touchstart'].forEach(ev =>
        document.addEventListener(ev, startMenuOnce, { once: true })
    );

    // ── Public API ────────────────────────────────────────────────────────────

    window.audioPlay = {
        bounce()    { beep({ type: 'sine',     freq: 440,          gain: 0.25, duration: 0.05 }); },
        score()     { beep({ type: 'sine',     freq: 300, freqEnd: 800, gain: 0.35, duration: 0.15 }); },
        hit()       { beep({ type: 'sawtooth', freq: 80,           gain: 0.45, duration: 0.20 }); },
        countdown() { beep({ type: 'square',   freq: 600,          gain: 0.28, duration: 0.03 }); },
        go()        { [523, 659, 784].forEach(f => beep({ type: 'sine', freq: f, gain: 0.2, duration: 0.3 })); },
        gameOver()  { beep({ type: 'sine',     freq: 400, freqEnd: 80,  gain: 0.40, duration: 0.50 }); },
        menuMusic() { startMenuMusic(); },
        gameMusic() { startGameMusic(); },
        stopMusic() { stopMusic(); },
    };
})();
