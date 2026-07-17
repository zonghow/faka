import { useEffect, useState } from 'react'
import { api, type Space } from '@/lib/api'
import { Button, Card, CardContent, CardHeader, CardTitle, Input, Label, PageHeader, Toast } from '@/components/ui'
import { useToast } from '@/hooks/useToast'
import { useConfirm } from '@/components/confirm-provider'

export function SpacesPage() {
  const [spaces, setSpaces] = useState<Space[]>([])
  const [name, setName] = useState('')
  const [prefix, setPrefix] = useState('')
  const [editing, setEditing] = useState<Space | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [deletingId, setDeletingId] = useState<number | null>(null)
  const { toast, show } = useToast()
  const confirm = useConfirm()

  const load = async () => {
    setLoading(true)
    try {
      const r = await api.spaces()
      setSpaces(r.spaces)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load().catch((e) => show(e.message))
  }, [show])

  return (
    <div>
      {toast ? <Toast message={toast.message} type={toast.type} /> : null}
      <PageHeader title="空间设置" desc="管理空间名称和卡密前缀" />
      <div className="grid gap-6 lg:grid-cols-[1fr_320px]">
        <Card className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead className="border-b border-border text-left text-muted-foreground">
              <tr>
                <th className="px-4 py-3">空间名</th>
                <th className="px-4 py-3">卡密前缀</th>
                <th className="px-4 py-3">操作</th>
              </tr>
            </thead>
            <tbody>
              {spaces.map((s) => (
                <tr key={s.id} className="border-b border-border/50">
                  <td className="px-4 py-3 font-mono">{s.name}</td>
                  <td className="px-4 py-3 font-mono">{s.card_prefix}</td>
                  <td className="px-4 py-3">
                    <div className="flex gap-2">
                      <Button
                        size="sm"
                        variant="secondary"
                        onClick={() => {
                          setEditing(s)
                          setName(s.name)
                          setPrefix(s.card_prefix)
                        }}
                      >
                        编辑
                      </Button>
                      <Button
                        size="sm"
                        variant="ghost"
                        loading={deletingId === s.id}
                        onClick={async () => {
                          const ok = await confirm({
                            title: '删除空间',
                            description: `确定删除空间「${s.name}」吗？该空间下全部卡密、文件、兑换记录和审计日志都会被删除，且不可恢复。`,
                            confirmText: '确认删除',
                            cancelText: '取消',
                            danger: true,
                          })
                          if (!ok) return
                          setDeletingId(s.id)
                          try {
                            await api.deleteSpace(s.id)
                            show('已删除', 'success')
                            await load()
                          } catch (e) {
                            show(e instanceof Error ? e.message : '删除失败')
                          } finally {
                            setDeletingId(null)
                          }
                        }}
                      >
                        {deletingId === s.id ? '删除中...' : '删除'}
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>{editing ? '编辑空间' : '创建空间'}</CardTitle>
          </CardHeader>
          <CardContent>
            <form
              className="space-y-3"
              onSubmit={async (e) => {
                e.preventDefault()
                setSaving(true)
                try {
                  if (editing) await api.updateSpace(editing.id, name, prefix)
                  else await api.createSpace(name, prefix)
                  show(editing ? '已更新' : '已创建', 'success')
                  setEditing(null)
                  setName('')
                  setPrefix('')
                  await load()
                } catch (err) {
                  show(err instanceof Error ? err.message : '保存失败')
                } finally {
                  setSaving(false)
                }
              }}
            >
              <div className="space-y-2">
                <Label>空间名</Label>
                <Input value={name} onChange={(e) => setName(e.target.value)} maxLength={20} pattern="[A-Za-z0-9_]{1,20}" required />
              </div>
              <div className="space-y-2">
                <Label>卡密前缀</Label>
                <Input
                  value={prefix}
                  onChange={(e) => setPrefix(e.target.value.toUpperCase().replace(/[^A-Z0-9]/g, ''))}
                  maxLength={10}
                  required
                />
              </div>
              <div className="flex gap-2">
                <Button type="submit" loading={saving}>
                  {saving ? '保存中...' : '保存'}
                </Button>
                {editing ? (
                  <Button type="button" variant="ghost" disabled={saving} onClick={() => { setEditing(null); setName(''); setPrefix('') }}>
                    取消
                  </Button>
                ) : null}
              </div>
            </form>
            {loading ? <div className="mt-3 text-xs text-muted-foreground">列表刷新中...</div> : null}
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
