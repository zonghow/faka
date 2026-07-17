import { createContext, useCallback, useContext, useMemo, useRef, useState, type ReactNode } from 'react'
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'

export type ConfirmOptions = {
  title?: string
  description?: string
  confirmText?: string
  cancelText?: string
  danger?: boolean
}

type ConfirmContextValue = {
  confirm: (options: ConfirmOptions) => Promise<boolean>
}

const ConfirmContext = createContext<ConfirmContextValue | null>(null)

export function ConfirmProvider({ children }: { children: ReactNode }) {
  const [open, setOpen] = useState(false)
  const [options, setOptions] = useState<ConfirmOptions>({})
  const resolver = useRef<((value: boolean) => void) | null>(null)
  const settled = useRef(false)

  const finish = useCallback((value: boolean) => {
    if (settled.current) return
    settled.current = true
    setOpen(false)
    resolver.current?.(value)
    resolver.current = null
  }, [])

  const confirm = useCallback((opts: ConfirmOptions) => {
    settled.current = false
    setOptions(opts)
    setOpen(true)
    return new Promise<boolean>((resolve) => {
      resolver.current = resolve
    })
  }, [])

  const value = useMemo(() => ({ confirm }), [confirm])

  return (
    <ConfirmContext.Provider value={value}>
      {children}
      <AlertDialog
        open={open}
        onOpenChange={(next) => {
          if (!next) finish(false)
        }}
      >
        <AlertDialogContent
          onEscapeKeyDown={(e) => {
            e.preventDefault()
            finish(false)
          }}
          onPointerDownOutside={(e) => {
            e.preventDefault()
            finish(false)
          }}
        >
          <AlertDialogHeader>
            <AlertDialogTitle>{options.title || '确认操作'}</AlertDialogTitle>
            {options.description ? <AlertDialogDescription>{options.description}</AlertDialogDescription> : null}
          </AlertDialogHeader>
          <AlertDialogFooter>
            <Button type="button" variant="outline" onClick={() => finish(false)}>
              {options.cancelText || '取消'}
            </Button>
            <Button type="button" variant={options.danger ? 'destructive' : 'default'} onClick={() => finish(true)}>
              {options.confirmText || '确认'}
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </ConfirmContext.Provider>
  )
}

export function useConfirm() {
  const ctx = useContext(ConfirmContext)
  if (!ctx) {
    throw new Error('useConfirm must be used within ConfirmProvider')
  }
  return ctx.confirm
}
