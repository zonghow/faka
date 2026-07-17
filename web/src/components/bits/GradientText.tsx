import { cn } from '@/lib/utils'

export function GradientText({
  text,
  className,
}: {
  text: string
  className?: string
}) {
  return (
    <span
      className={cn(
        'bg-[linear-gradient(90deg,#A855F7_0%,#B497CF_40%,#ffffff_70%,#A855F7_100%)] bg-[length:200%_100%] bg-clip-text text-transparent animate-gradient-x',
        className,
      )}
    >
      {text}
      <style>{`
        @keyframes gradient-x {
          0% { background-position: 0% 50%; }
          50% { background-position: 100% 50%; }
          100% { background-position: 0% 50%; }
        }
        .animate-gradient-x {
          animation: gradient-x 6s ease infinite;
        }
      `}</style>
    </span>
  )
}
