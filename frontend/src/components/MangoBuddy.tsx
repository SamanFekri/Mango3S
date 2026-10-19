import { useCallback, useEffect, useId, useRef, useState } from 'react'

export type BuddyMood = 'idle' | 'happy' | 'worried' | 'sleepy'

const SLEEP_AFTER_MS = 60_000
const BUBBLE_MS = 4200

const reducedMotion = () => typeof window !== 'undefined' && !!window.matchMedia?.('(prefers-reduced-motion: reduce)').matches

/** Element.animate, when the platform has it (not in tests) and motion is welcome. */
function play(el: Element | null, frames: Keyframe[], options: KeyframeAnimationOptions) {
  if (!el || reducedMotion() || typeof el.animate !== 'function') return
  el.animate(frames, options)
}

/**
 * Mango, the app's mascot: a little animated mango that floats, blinks,
 * follows the pointer with its eyes, waves, and reacts (happy / worried) to
 * what happens in the app. Purely decorative — it never takes focus from
 * work, sits out of the way, and can be hidden (Settings → General).
 * All motion stops under "reduce motion".
 */
export function MangoBuddy({
  lines,
  mood: moodProp,
  moodKey,
  onHide,
  size = 72,
  className = '',
}: {
  /** What Mango says when clicked, in turn. */
  lines: string[]
  /** A mood to show for a moment (e.g. after a success or an error). */
  mood?: BuddyMood
  /** Changes whenever the mood above should be shown again. */
  moodKey?: number | string
  /** Hide Mango (shows a small × on hover). */
  onHide?: () => void
  size?: number
  className?: string
}) {
  const uid = useId().replace(/:/g, '')
  const root = useRef<HTMLDivElement>(null)
  const body = useRef<SVGGElement>(null)
  const arm = useRef<SVGGElement>(null)
  const [mood, setMood] = useState<BuddyMood>('idle')
  const [bubble, setBubble] = useState<string | null>(null)
  const lineIndex = useRef(0)
  const moodTimer = useRef<number>(0)
  const bubbleTimer = useRef<number>(0)

  const showMood = useCallback((m: BuddyMood, ms = 2600) => {
    window.clearTimeout(moodTimer.current)
    setMood(m)
    if (m !== 'idle') moodTimer.current = window.setTimeout(() => setMood('idle'), ms)
  }, [])

  const jump = useCallback(() => {
    play(
      body.current,
      [
        { transform: 'translateY(0) scale(1, 1)' },
        { transform: 'translateY(2px) scale(1.06, 0.92)', offset: 0.15 },
        { transform: 'translateY(-16px) scale(0.96, 1.05)', offset: 0.5 },
        { transform: 'translateY(0) scale(1.04, 0.96)', offset: 0.85 },
        { transform: 'translateY(0) scale(1, 1)' },
      ],
      { duration: 620, easing: 'cubic-bezier(0.34, 1.56, 0.64, 1)' },
    )
  }, [])

  const wave = useCallback(() => {
    play(
      arm.current,
      [{ transform: 'rotate(0deg)' }, { transform: 'rotate(-28deg)' }, { transform: 'rotate(10deg)' }, { transform: 'rotate(-24deg)' }, { transform: 'rotate(0deg)' }],
      { duration: 900, easing: 'ease-in-out' },
    )
  }, [])

  // Reactions from the app.
  useEffect(() => {
    if (!moodProp || moodProp === 'idle') return
    showMood(moodProp)
    if (moodProp === 'happy') jump()
  }, [moodProp, moodKey, showMood, jump])

  // Eyes follow the pointer; Mango dozes off when nothing happens for a minute.
  useEffect(() => {
    let frame = 0
    let sleepTimer = window.setTimeout(() => setMood((m) => (m === 'idle' ? 'sleepy' : m)), SLEEP_AFTER_MS)
    const onMove = (e: PointerEvent) => {
      window.clearTimeout(sleepTimer)
      sleepTimer = window.setTimeout(() => setMood((m) => (m === 'idle' ? 'sleepy' : m)), SLEEP_AFTER_MS)
      setMood((m) => (m === 'sleepy' ? 'idle' : m))
      if (frame) return
      frame = requestAnimationFrame(() => {
        frame = 0
        const el = root.current
        if (!el) return
        const r = el.getBoundingClientRect()
        const dx = e.clientX - (r.left + r.width / 2)
        const dy = e.clientY - (r.top + r.height * 0.5)
        const len = Math.hypot(dx, dy) || 1
        const k = Math.min(1, len / 240) * 2.4
        el.style.setProperty('--mb-px', `${((dx / len) * k).toFixed(2)}px`)
        el.style.setProperty('--mb-py', `${((dy / len) * k).toFixed(2)}px`)
      })
    }
    window.addEventListener('pointermove', onMove, { passive: true })
    window.addEventListener('keydown', onMove as unknown as EventListener, { passive: true })
    return () => {
      window.removeEventListener('pointermove', onMove)
      window.removeEventListener('keydown', onMove as unknown as EventListener)
      window.clearTimeout(sleepTimer)
      cancelAnimationFrame(frame)
    }
  }, [])

  useEffect(
    () => () => {
      window.clearTimeout(moodTimer.current)
      window.clearTimeout(bubbleTimer.current)
    },
    [],
  )

  const poke = () => {
    showMood('happy')
    jump()
    wave()
    if (lines.length) {
      setBubble(lines[lineIndex.current % lines.length]!)
      lineIndex.current++
      window.clearTimeout(bubbleTimer.current)
      bubbleTimer.current = window.setTimeout(() => setBubble(null), BUBBLE_MS)
    }
  }

  const ink = '#3a2312'
  return (
    <div ref={root} className={`mango-buddy mood-${mood} ${className}`} style={{ width: size }}>
      {bubble && (
        <div className="mango-buddy-bubble" role="status">
          {bubble}
        </div>
      )}
      <button type="button" className="mango-buddy-hit" onClick={poke} onMouseEnter={wave} aria-label="Mango, the Mango3S mascot. Click for a tip" title="Hi! Click me">
        <svg viewBox="0 0 120 140" width={size} height={(size * 140) / 120} aria-hidden="true" focusable="false">
          <defs>
            <radialGradient id={`${uid}-body`} cx="34%" cy="30%" r="78%">
              <stop offset="0" stopColor="#ffe36e" />
              <stop offset="0.42" stopColor="#ffb321" />
              <stop offset="0.78" stopColor="#ff8a1f" />
              <stop offset="1" stopColor="#ef5a2b" />
            </radialGradient>
            <linearGradient id={`${uid}-leaf`} x1="0" y1="0" x2="1" y2="1">
              <stop offset="0" stopColor="#8fdc8f" />
              <stop offset="1" stopColor="#2a9a57" />
            </linearGradient>
          </defs>
          <ellipse className="mb-shadow" cx="60" cy="134" rx="28" ry="4" />
          <g className="mb-float">
            <g ref={body} className="mb-body">
              <path d="M26 92 q-11 3 -13 13" className="mb-limb" />
              <g ref={arm} className="mb-arm">
                <path d="M96 90 q12 -3 15 -15" className="mb-limb" />
              </g>
              <ellipse cx="48" cy="127" rx="8" ry="4.5" fill="#d9701a" />
              <ellipse cx="72" cy="127" rx="8" ry="4.5" fill="#d9701a" />
              <path d="M62 24 C90 22 106 50 103 80 C100 110 82 128 60 128 C36 128 18 112 18 86 C18 56 34 26 62 24Z" fill={`url(#${uid}-body)`} />
              <ellipse cx="41" cy="47" rx="10" ry="6" transform="rotate(-32 41 47)" fill="#fff" opacity="0.45" />
              <path d="M61 26 q-1 -8 3 -13" stroke="#7a4a1f" strokeWidth="3.5" strokeLinecap="round" fill="none" />
              <g className="mb-leaf">
                <path d="M64 13 C72 1 91 1 97 9 C89 19 74 20 64 13Z" fill={`url(#${uid}-leaf)`} />
                <path d="M67 13 C77 9 87 8 95 9" stroke="#1f7a43" strokeWidth="1.2" fill="none" opacity="0.6" />
              </g>

              {/* Face */}
              <g className="mb-face">
                {mood === 'happy' ? (
                  <g stroke={ink} strokeWidth="2.8" strokeLinecap="round" fill="none">
                    <path d="M41 77 q6 -8 12 0" />
                    <path d="M69 77 q6 -8 12 0" />
                  </g>
                ) : mood === 'sleepy' ? (
                  <g stroke={ink} strokeWidth="2.6" strokeLinecap="round" fill="none">
                    <path d="M41 75 q6 5 12 0" />
                    <path d="M69 75 q6 5 12 0" />
                  </g>
                ) : (
                  <>
                    {[47, 75].map((cx) => (
                      <g key={cx} className="mb-eye" style={{ transformOrigin: `${cx}px 74px` }}>
                        <ellipse cx={cx} cy="74" rx="7.5" ry="8.5" fill="#fff" />
                        <g className="mb-pupil">
                          <circle cx={cx} cy="75" r="4.4" fill={ink} />
                          <circle cx={cx + 1.6} cy="73.2" r="1.4" fill="#fff" />
                        </g>
                      </g>
                    ))}
                    {mood === 'worried' && (
                      <g stroke={ink} strokeWidth="2.2" strokeLinecap="round">
                        <path d="M40 63 l11 -3" />
                        <path d="M82 63 l-11 -3" />
                      </g>
                    )}
                  </>
                )}
                <ellipse cx="37" cy="88" rx="5.5" ry="3.2" fill="#ff5d73" opacity="0.45" />
                <ellipse cx="85" cy="88" rx="5.5" ry="3.2" fill="#ff5d73" opacity="0.45" />
                {mood === 'happy' ? (
                  <g>
                    <path d="M52 87 q9 13 18 0 z" fill="#7a1f1f" />
                    <ellipse cx="61" cy="92.5" rx="4.2" ry="2.2" fill="#ff7a8a" />
                  </g>
                ) : mood === 'worried' ? (
                  <ellipse cx="61" cy="92" rx="3.2" ry="2.6" fill={ink} />
                ) : mood === 'sleepy' ? (
                  <path d="M57 91 q4 2 8 0" stroke={ink} strokeWidth="2.4" strokeLinecap="round" fill="none" />
                ) : (
                  <path d="M54 88 q7 7 14 0" stroke={ink} strokeWidth="2.6" strokeLinecap="round" fill="none" />
                )}
              </g>
              {mood === 'sleepy' && (
                <g className="mb-zzz" fill="#8a6bd1" fontFamily="var(--font-ui, sans-serif)" fontWeight="700">
                  <text x="98" y="40" fontSize="12">z</text>
                  <text x="106" y="28" fontSize="9">z</text>
                </g>
              )}
            </g>
          </g>
        </svg>
      </button>
      {onHide && (
        <button type="button" className="mango-buddy-hide" onClick={onHide} aria-label="Hide Mango" title="Hide Mango (bring it back in Settings)">
          ×
        </button>
      )}
    </div>
  )
}
