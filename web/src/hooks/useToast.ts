import { useCallback, useState } from 'react'

export function useToast() {
  const [toast, setToast] = useState<{ message: string; type: 'error' | 'success' } | null>(null)
  const show = useCallback((message: string, type: 'error' | 'success' = 'error') => {
    setToast({ message, type })
    window.setTimeout(() => setToast(null), 2600)
  }, [])
  return { toast, show }
}
