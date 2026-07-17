import { cn } from '@/lib/utils'

export function Aurora({ className }: { className?: string }) {
  return (
    <div className={cn('pointer-events-none absolute inset-0 overflow-hidden', className)} aria-hidden>
      <div className="absolute inset-0 bg-[#0b0d12]" />
      <div className="aurora-blob aurora-blob-a" />
      <div className="aurora-blob aurora-blob-b" />
      <div className="aurora-blob aurora-blob-c" />
      <div className="absolute inset-0 bg-[radial-gradient(circle_at_center,transparent_0%,rgba(11,13,18,0.55)_55%,rgba(11,13,18,0.92)_100%)]" />
      <style>{`
        .aurora-blob {
          position: absolute;
          width: 55vmax;
          height: 55vmax;
          border-radius: 9999px;
          filter: blur(70px);
          opacity: 0.55;
          mix-blend-mode: screen;
          will-change: transform;
        }
        .aurora-blob-a {
          top: -18%;
          left: -10%;
          background: radial-gradient(circle, rgba(97,175,239,0.75) 0%, rgba(97,175,239,0) 70%);
          animation: aurora-float-a 14s ease-in-out infinite;
        }
        .aurora-blob-b {
          top: 20%;
          right: -18%;
          background: radial-gradient(circle, rgba(198,120,221,0.55) 0%, rgba(198,120,221,0) 70%);
          animation: aurora-float-b 18s ease-in-out infinite;
        }
        .aurora-blob-c {
          bottom: -22%;
          left: 20%;
          background: radial-gradient(circle, rgba(86,182,194,0.45) 0%, rgba(86,182,194,0) 70%);
          animation: aurora-float-c 16s ease-in-out infinite;
        }
        @keyframes aurora-float-a {
          0%, 100% { transform: translate3d(0,0,0) scale(1); }
          50% { transform: translate3d(8%, 10%, 0) scale(1.12); }
        }
        @keyframes aurora-float-b {
          0%, 100% { transform: translate3d(0,0,0) scale(1.05); }
          50% { transform: translate3d(-10%, -6%, 0) scale(0.95); }
        }
        @keyframes aurora-float-c {
          0%, 100% { transform: translate3d(0,0,0) scale(1); }
          50% { transform: translate3d(6%, -12%, 0) scale(1.15); }
        }
      `}</style>
    </div>
  )
}
