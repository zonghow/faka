import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '@/lib/api'
import { Button, Card, CardContent, Input, Label, Toast } from '@/components/ui'
import { useToast } from '@/hooks/useToast'

export function LoginPage() {
  const [password, setPassword] = useState('')
  const [loading, setLoading] = useState(false)
  const navigate = useNavigate()
  const { toast, show } = useToast()

  useEffect(() => {
    api.me().then((res) => {
      if (res.authenticated) navigate('/admin', { replace: true })
    }).catch(() => undefined)
  }, [navigate])

  return (
    <div className="grid min-h-screen place-items-center px-4">
      {toast ? <Toast message={toast.message} type={toast.type} /> : null}
      <Card className="w-full max-w-md">
        <CardContent className="p-6">
          <div className="mb-2 text-xs tracking-[0.2em] text-muted-foreground">ADMIN ACCESS</div>
          <h1 className="mb-6 text-2xl font-bold">管理平台登录</h1>
          <form
            className="space-y-4"
            onSubmit={async (e) => {
              e.preventDefault()
              setLoading(true)
              try {
                await api.login(password)
                navigate('/admin')
              } catch (err) {
                show(err instanceof Error ? err.message : '登录失败')
              } finally {
                setLoading(false)
              }
            }}
          >
            <div className="space-y-2">
              <Label htmlFor="password">密码</Label>
              <Input id="password" type="password" value={password} onChange={(e) => setPassword(e.target.value)} required />
            </div>
            <Button className="w-full" loading={loading}>
              {loading ? '登录中...' : '登录'}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
