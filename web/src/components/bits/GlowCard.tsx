import { cn } from '@/lib/utils'
import type { ReactNode } from 'react'

export function GlowCard({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <div
      className={cn(
        'relative rounded-[28px] border border-white/[0.06] bg-[#121214]/90 p-6 shadow-[0_20px_60px_rgba(0,0,0,0.45)] backdrop-blur-xl',
        className,
      )}
    >
      {children}
    </div>
  )
}
