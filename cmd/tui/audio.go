package main

import (
	"bytes"
	"math"
	"sync"
	"time"

	"github.com/ebitengine/oto/v3"
)

const (
	sampleRate   = 44100
	channelCount = 1
)

var (
	audioCtx  *oto.Context
	audioOnce sync.Once
)

func initAudio() {
	audioOnce.Do(func() {
		opts := &oto.NewContextOptions{
			SampleRate:   sampleRate,
			ChannelCount: channelCount,
			Format:       oto.FormatSignedInt16LE,
		}
		ctx, ready, err := oto.NewContext(opts)
		if err != nil {
			return
		}
		<-ready
		audioCtx = ctx
	})
}

// tone generates a PCM buffer: sine wave at freq Hz for dur, with fade-out.
// Optional freqEnd != 0 creates a linear frequency sweep (freq → freqEnd).
func tone(freq, freqEnd float64, dur time.Duration, vol float64) []byte {
	n := int(float64(sampleRate) * dur.Seconds())
	buf := make([]byte, n*2)
	for i := 0; i < n; i++ {
		t := float64(i) / float64(sampleRate)
		f := freq
		if freqEnd != 0 {
			f = freq + (freqEnd-freq)*float64(i)/float64(n)
		}
		fade := math.Pow(1.0-float64(i)/float64(n), 0.5)
		s := math.Sin(2*math.Pi*f*t) * vol * fade
		v := int16(s * 32767)
		buf[i*2] = byte(v)
		buf[i*2+1] = byte(v >> 8)
	}
	return buf
}

// chord plays two tones mixed together.
func chord(f1, f2 float64, dur time.Duration, vol float64) []byte {
	n := int(float64(sampleRate) * dur.Seconds())
	buf := make([]byte, n*2)
	for i := 0; i < n; i++ {
		t := float64(i) / float64(sampleRate)
		fade := math.Pow(1.0-float64(i)/float64(n), 0.5)
		s := (math.Sin(2*math.Pi*f1*t) + math.Sin(2*math.Pi*f2*t)) * 0.5 * vol * fade
		v := int16(s * 32767)
		buf[i*2] = byte(v)
		buf[i*2+1] = byte(v >> 8)
	}
	return buf
}

func playBuf(buf []byte) {
	if audioCtx == nil {
		return
	}
	p := audioCtx.NewPlayer(bytes.NewReader(buf))
	p.Play()
}

// ── sound events ──────────────────────────────────────────────────────────────

func soundBounce() {
	go playBuf(tone(880, 0, 60*time.Millisecond, 0.4))
}

func soundScore() {
	go func() {
		playBuf(tone(660, 0, 60*time.Millisecond, 0.5))
		time.Sleep(70 * time.Millisecond)
		playBuf(tone(880, 0, 80*time.Millisecond, 0.5))
		time.Sleep(90 * time.Millisecond)
		playBuf(tone(1100, 0, 100*time.Millisecond, 0.5))
	}()
}

func soundHit() {
	go playBuf(tone(220, 110, 200*time.Millisecond, 0.5))
}

func soundCountdown() {
	go playBuf(tone(440, 0, 100*time.Millisecond, 0.4))
}

func soundGo() {
	go playBuf(chord(660, 880, 200*time.Millisecond, 0.5))
}

func soundGameOver() {
	go func() {
		playBuf(tone(440, 0, 100*time.Millisecond, 0.4))
		time.Sleep(110 * time.Millisecond)
		playBuf(tone(330, 0, 100*time.Millisecond, 0.4))
		time.Sleep(110 * time.Millisecond)
		playBuf(tone(220, 110, 400*time.Millisecond, 0.5))
	}()
}
