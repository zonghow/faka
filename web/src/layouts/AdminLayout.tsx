import { NavLink, Outlet, useNavigate } from 'react-router-dom'
import { useEffect, useState } from 'react'
import { api, setSpaceCookie, type Space } from '@/lib/api'
import { Button, Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui'
import { cn } from '@/lib/utils'

const nav = [
  { to: '/admin', label: '数据概览', end: true },
  { to: '/admin/spaces', label: '空间设置' },
  { to: '/admin/cards', label: '卡密管理' },
  { to: '/admin/files', label: '文件管理' },
  { to: '/admin/uploads', label: '上传文件' },
]

export function AdminLayout() {
  const navigate = useNavigate()
  const [spaces, setSpaces] = useState<Space[]>([])
  const [current, setCurrent] = useState<Space | null>(null)
  const [ready, setReady] = useState(false)
  const [loggingOut, setLoggingOut] = useState(false)

  useEffect(() => {
    api
      .me()
      .then((res) => {
        if (!res.authenticated) {
          navigate('/admin/login', { replace: true })
          return
        }
        return api.spaces()
      })
      .then((res) => {
        if (!res) return
        setSpaces(res.spaces)
        setCurrent(res.current_space)
        if (res.current_space) setSpaceCookie(res.current_space.id)
        setReady(true)
      })
      .catch(() => navigate('/admin/login', { replace: true }))
  }, [navigate])

  if (!ready) {
    return <div className="grid min-h-screen place-items-center text-muted-foreground">加载中...</div>
  }

  return (
    <div className="min-h-screen md:grid md:grid-cols-[240px_1fr]">
      <aside className="border-b border-border bg-card p-5 md:border-b-0 md:border-r">
        <div className="mb-8 text-lg font-bold tracking-tight">GPT PLUS 提卡网</div>
        <nav className="flex flex-col gap-1">
          {nav.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.end}
              className={({ isActive }) =>
                cn(
                  'rounded-xl px-3 py-2 text-sm transition',
                  isActive ? 'bg-accent text-accent-foreground' : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground',
                )
              }
            >
              {item.label}
            </NavLink>
          ))}
        </nav>
      </aside>
      <div className="flex min-h-screen flex-col">
        <header className="flex items-center justify-between gap-4 border-b border-border px-6 py-4">
          <div />
          <div className="flex shrink-0 items-center gap-4 whitespace-nowrap">
            <div className="flex shrink-0 items-center gap-2 whitespace-nowrap text-sm text-muted-foreground">
              <span className="shrink-0 whitespace-nowrap">当前空间</span>
              <Select
                value={current ? String(current.id) : undefined}
                onValueChange={(value) => {
                  const id = Number(value)
                  setSpaceCookie(id)
                  const next = spaces.find((s) => s.id === id) || null
                  setCurrent(next)
                  window.location.reload()
                }}
              >
                <SelectTrigger className="!w-[220px] shrink-0">
                  <SelectValue placeholder="选择空间" />
                </SelectTrigger>
                <SelectContent>
                  {spaces.map((s) => (
                    <SelectItem key={s.id} value={String(s.id)}>
                      {s.name} ({s.card_prefix})
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <Button
              className="shrink-0"
              variant="secondary"
              loading={loggingOut}
              onClick={async () => {
                setLoggingOut(true)
                try {
                  await api.logout()
                  navigate('/admin/login')
                } finally {
                  setLoggingOut(false)
                }
              }}
            >
              {loggingOut ? '退出中...' : '退出'}
            </Button>
          </div>
        </header>
        <main className="min-h-0 flex-1 overflow-hidden p-6">
          <Outlet
            context={{
              spaces,
              current,
              refreshSpaces: async () => {
                const res = await api.spaces()
                setSpaces(res.spaces)
                setCurrent(res.current_space)
              },
            }}
          />
        </main>
      </div>
    </div>
  )
}
