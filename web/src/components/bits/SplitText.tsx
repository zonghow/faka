import { useEffect, useRef } from 'react'
import gsap from 'gsap'
import { cn } from '@/lib/utils'

export function SplitText({
  text,
  className,
  delay = 0,
}: {
  text: string
  className?: string
  delay?: number
}) {
  const rootRef = useRef<HTMLHeadingElement>(null)

  useEffect(() => {
    const el = rootRef.current
    if (!el) return
    const chars = el.querySelectorAll<HTMLElement>('[data-char]')
    const tween = gsap.fromTo(
      chars,
      { y: 28, opacity: 0, rotateX: -50 },
      {
        y: 0,
        opacity: 1,
        rotateX: 0,
        duration: 0.75,
        ease: 'power3.out',
        stagger: 0.035,
        delay,
      },
    )
    return () => {
      tween.kill()
    }
  }, [text, delay])

  return (
    <h1 ref={rootRef} className={cn('inline-flex flex-wrap justify-center', className)} aria-label={text}>
      {text.split('').map((ch, i) => (
        <span key={`${ch}-${i}`} data-char className="inline-block will-change-transform" style={{ whiteSpace: ch === ' ' ? 'pre' : undefined }}>
          {ch === ' ' ? '\u00A0' : ch}
        </span>
      ))}
    </h1>
  )
}
