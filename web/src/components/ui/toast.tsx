import { cn } from '@/lib/utils'

export function Toast({ message, type = 'error' }: { message: string; type?: 'error' | 'success' }) {
  return (
    <div
      className={cn(
        'fixed right-4 top-4 z-50 rounded-md border px-4 py-3 text-sm font-medium shadow-lg',
        type === 'success'
          ? 'border-border bg-primary text-primary-foreground'
          : 'border-border bg-destructive text-white',
      )}
    >
      {message}
    </div>
  )
}
