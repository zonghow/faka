import { useCallback, useEffect, useMemo, useState } from 'react'
import { api, type Relay, type RelayGroup, type RelayStats } from '@/lib/api'
import {
  Button,
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Input,
  Label,
  PageHeader,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  Toast,
} from '@/components/ui'
import { useToast } from '@/hooks/useToast'
import { useConfirm } from '@/components/confirm-provider'

type StatsEntry = RelayStats & { loading?: boolean }

type StatsMap = Record<number, StatsEntry>

function dash(v: number | undefined | null) {
  if (v === undefined || v === null) return '-'
  return String(v)
}

export function RelaysPage() {
  const [relays, setRelays] = useState<Relay[]>([])
  const [stats, setStats] = useState<StatsMap>({})
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [deletingId, setDeletingId] = useState<number | null>(null)
  const [editing, setEditing] = useState<Relay | null>(null)
  const [name, setName] = useState('')
  const [type, setType] = useState<'sub2api' | 'cpa'>('sub2api')
  const [address, setAddress] = useState('')
  const [password, setPassword] = useState('')
  const [supplyRelay, setSupplyRelay] = useState<Relay | null>(null)
  const [supplyMode, setSupplyMode] = useState<'cdkey' | 'idle'>('cdkey')
  const [cardCode, setCardCode] = useState('')
  const [idleCount, setIdleCount] = useState(1)
  const [groups, setGroups] = useState<RelayGroup[]>([])
  const [groupID, setGroupID] = useState<string>('')
  const [groupsLoading, setGroupsLoading] = useState(false)
  const [supplying, setSupplying] = useState(false)
  const { toast, show } = useToast()
  const confirm = useConfirm()

  const sub2apiRelays = useMemo(() => relays.filter((r) => r.type === 'sub2api'), [relays])
  const cpaRelays = useMemo(() => relays.filter((r) => r.type === 'cpa'), [relays])

  const loadStats = useCallback(async (list: Relay[]) => {
    setStats((prev) => {
      const next = { ...prev }
      for (const r of list) next[r.id] = { loading: true }
      return next
    })
    await Promise.all(
      list.map(async (r) => {
        try {
          const res = await api.relayStats(r.id)
          setStats((prev) => ({ ...prev, [r.id]: res.stats }))
        } catch (e) {
          setStats((prev) => ({
            ...prev,
            [r.id]: { message: e instanceof Error ? e.message : '获取失败' },
          }))
        }
      }),
    )
  }, [])

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const res = await api.relays()
      setRelays(res.relays)
      void loadStats(res.relays)
    } finally {
      setLoading(false)
    }
  }, [loadStats])

  useEffect(() => {
    load().catch((e) => show(e.message))
  }, [load, show])

  const resetForm = () => {
    setEditing(null)
    setName('')
    setType('sub2api')
    setAddress('')
    setPassword('')
  }

  const startEdit = (r: Relay) => {
    setEditing(r)
    setName(r.name)
    setType(r.type)
    setAddress(r.address)
    setPassword(r.password)
  }

  const openSupply = async (r: Relay) => {
    setSupplyRelay(r)
    setSupplyMode('cdkey')
    setCardCode('')
    setIdleCount(1)
    setGroups([])
    setGroupID('')
    if (r.type !== 'sub2api') return
    setGroupsLoading(true)
    try {
      const res = await api.relayGroups(r.id)
      setGroups(res.groups || [])
      if (res.groups?.length) setGroupID(String(res.groups[0].id))
      else show('该中转暂无可用分组，请先在 sub2api 创建分组')
    } catch (e) {
      show(e instanceof Error ? e.message : '加载分组失败')
    } finally {
      setGroupsLoading(false)
    }
  }

  const renderSub2apiTable = () => (
    <Card className="overflow-x-auto">
      <CardHeader className="flex flex-row items-center justify-between gap-2">
        <CardTitle>sub2api 中转</CardTitle>
        <Button size="sm" variant="secondary" onClick={() => loadStats(sub2apiRelays)} disabled={sub2apiRelays.length === 0}>
          刷新数据
        </Button>
      </CardHeader>
      <table className="w-full text-sm">
        <thead className="border-b border-border text-left text-muted-foreground">
          <tr>
            <th className="px-4 py-3">名称</th>
            <th className="px-4 py-3">地址</th>
            <th className="px-4 py-3">可用</th>
            <th className="px-4 py-3">总计</th>
            <th className="px-4 py-3">限流</th>
            <th className="px-4 py-3">错误</th>
            <th className="px-4 py-3">队列</th>
            <th className="px-4 py-3">操作</th>
          </tr>
        </thead>
        <tbody>
          {sub2apiRelays.length === 0 ? (
            <tr>
              <td colSpan={8} className="px-4 py-6 text-center text-muted-foreground">
                暂无 sub2api 中转
              </td>
            </tr>
          ) : (
            sub2apiRelays.map((r) => {
              const s = stats[r.id]
              const loadingRow = s && 'loading' in s && s.loading
              return (
                <tr key={r.id} className="border-b border-border/50">
                  <td className="px-4 py-3 font-medium">{r.name}</td>
                  <td className="max-w-[220px] truncate px-4 py-3 font-mono text-xs" title={r.address}>
                    {r.address}
                  </td>
                  <td className="px-4 py-3">{loadingRow ? '...' : dash(s?.available)}</td>
                  <td className="px-4 py-3">{loadingRow ? '...' : dash(s?.total)}</td>
                  <td className="px-4 py-3">{loadingRow ? '...' : dash(s?.rate_limit)}</td>
                  <td className="px-4 py-3">{loadingRow ? '...' : dash(s?.error)}</td>
                  <td className="px-4 py-3">{loadingRow ? '...' : dash(s?.queue)}</td>
                  <td className="px-4 py-3">
                    <div className="flex flex-wrap gap-2">
                      <Button size="sm" onClick={() => openSupply(r)}>
                        补号
                      </Button>
                      <Button size="sm" variant="secondary" onClick={() => startEdit(r)}>
                        编辑
                      </Button>
                      <Button
                        size="sm"
                        variant="ghost"
                        loading={deletingId === r.id}
                        onClick={async () => {
                          const ok = await confirm({
                            title: '删除中转',
                            description: `确定删除中转「${r.name}」吗？`,
                            confirmText: '确认删除',
                            cancelText: '取消',
                            danger: true,
                          })
                          if (!ok) return
                          setDeletingId(r.id)
                          try {
                            await api.deleteRelay(r.id)
                            show('已删除', 'success')
                            await load()
                          } catch (e) {
                            show(e instanceof Error ? e.message : '删除失败')
                          } finally {
                            setDeletingId(null)
                          }
                        }}
                      >
                        删除
                      </Button>
                    </div>
                    {s?.message ? <div className="mt-1 text-xs text-destructive">{s.message}</div> : null}
                  </td>
                </tr>
              )
            })
          )}
        </tbody>
      </table>
    </Card>
  )

  const renderCpaTable = () => (
    <Card className="overflow-x-auto">
      <CardHeader className="flex flex-row items-center justify-between gap-2">
        <CardTitle>cpa 中转</CardTitle>
        <Button size="sm" variant="secondary" onClick={() => loadStats(cpaRelays)} disabled={cpaRelays.length === 0}>
          刷新数据
        </Button>
      </CardHeader>
      <table className="w-full text-sm">
        <thead className="border-b border-border text-left text-muted-foreground">
          <tr>
            <th className="px-4 py-3">名称</th>
            <th className="px-4 py-3">地址</th>
            <th className="px-4 py-3">总计</th>
            <th className="px-4 py-3">异常</th>
            <th className="px-4 py-3">操作</th>
          </tr>
        </thead>
        <tbody>
          {cpaRelays.length === 0 ? (
            <tr>
              <td colSpan={5} className="px-4 py-6 text-center text-muted-foreground">
                暂无 cpa 中转
              </td>
            </tr>
          ) : (
            cpaRelays.map((r) => {
              const s = stats[r.id]
              const loadingRow = s && 'loading' in s && s.loading
              return (
                <tr key={r.id} className="border-b border-border/50">
                  <td className="px-4 py-3 font-medium">{r.name}</td>
                  <td className="max-w-[280px] truncate px-4 py-3 font-mono text-xs" title={r.address}>
                    {r.address}
                  </td>
                  <td className="px-4 py-3">{loadingRow ? '...' : dash(s?.total)}</td>
                  <td className="px-4 py-3">{loadingRow ? '...' : dash(s?.abnormal)}</td>
                  <td className="px-4 py-3">
                    <div className="flex flex-wrap gap-2">
                      <Button size="sm" onClick={() => openSupply(r)}>
                        补号
                      </Button>
                      <Button size="sm" variant="secondary" onClick={() => startEdit(r)}>
                        编辑
                      </Button>
                      <Button
                        size="sm"
                        variant="ghost"
                        loading={deletingId === r.id}
                        onClick={async () => {
                          const ok = await confirm({
                            title: '删除中转',
                            description: `确定删除中转「${r.name}」吗？`,
                            confirmText: '确认删除',
                            cancelText: '取消',
                            danger: true,
                          })
                          if (!ok) return
                          setDeletingId(r.id)
                          try {
                            await api.deleteRelay(r.id)
                            show('已删除', 'success')
                            await load()
                          } catch (e) {
                            show(e instanceof Error ? e.message : '删除失败')
                          } finally {
                            setDeletingId(null)
                          }
                        }}
                      >
                        删除
                      </Button>
                    </div>
                    {s?.message ? <div className="mt-1 text-xs text-destructive">{s.message}</div> : null}
                  </td>
                </tr>
              )
            })
          )}
        </tbody>
      </table>
    </Card>
  )

  return (
    <div className="flex h-[calc(100vh-7.5rem)] flex-col gap-4 overflow-auto">
      {toast ? <Toast message={toast.message} type={toast.type} /> : null}
      <div className="shrink-0">
        <PageHeader title="中转补号" desc="管理 sub2api / CPA 中转，并向中转补入账号" />
      </div>

      <div className="grid gap-6 lg:grid-cols-[1fr_320px]">
        <div className="space-y-6">
          {renderSub2apiTable()}
          {renderCpaTable()}
          {loading ? <div className="text-xs text-muted-foreground">列表加载中...</div> : null}
        </div>

        <Card className="h-fit">
          <CardHeader>
            <CardTitle>{editing ? '编辑中转' : '新建中转'}</CardTitle>
          </CardHeader>
          <CardContent>
            <form
              className="space-y-3"
              onSubmit={async (e) => {
                e.preventDefault()
                setSaving(true)
                try {
                  if (editing) await api.updateRelay(editing.id, name, type, address, password)
                  else await api.createRelay(name, type, address, password)
                  show(editing ? '已更新' : '已创建', 'success')
                  resetForm()
                  await load()
                } catch (err) {
                  show(err instanceof Error ? err.message : '保存失败')
                } finally {
                  setSaving(false)
                }
              }}
            >
              <div className="space-y-2">
                <Label>名称</Label>
                <Input value={name} onChange={(e) => setName(e.target.value)} maxLength={80} required />
              </div>
              <div className="space-y-2">
                <Label>类型</Label>
                <Select value={type} onValueChange={(v) => setType(v as 'sub2api' | 'cpa')}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="sub2api">sub2api</SelectItem>
                    <SelectItem value="cpa">cpa</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label>地址</Label>
                <Input
                  value={address}
                  onChange={(e) => setAddress(e.target.value)}
                  placeholder="https://example.com"
                  required
                />
              </div>
              <div className="space-y-2">
                <Label>密码</Label>
                <Input
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder={type === 'sub2api' ? 'Admin API Key (x-api-key)' : 'Management Key'}
                  required
                />
              </div>
              <div className="flex gap-2">
                <Button type="submit" loading={saving}>
                  {saving ? '保存中...' : '保存'}
                </Button>
                {editing ? (
                  <Button type="button" variant="ghost" disabled={saving} onClick={resetForm}>
                    取消
                  </Button>
                ) : null}
              </div>
            </form>
          </CardContent>
        </Card>
      </div>

      <Dialog open={!!supplyRelay} onOpenChange={(open) => !open && setSupplyRelay(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>补号 · {supplyRelay?.name}</DialogTitle>
            <DialogDescription>
              类型 {supplyRelay?.type}。可用 CDKey 兑换后推送，或从当前空间空闲文件随机挑选。
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <Label>补号方式</Label>
              <Select value={supplyMode} onValueChange={(v) => setSupplyMode(v as 'cdkey' | 'idle')}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="cdkey">使用 CDKey</SelectItem>
                  <SelectItem value="idle">使用空闲文件</SelectItem>
                </SelectContent>
              </Select>
            </div>
            {supplyMode === 'cdkey' ? (
              <div className="space-y-2">
                <Label>卡密</Label>
                <Input value={cardCode} onChange={(e) => setCardCode(e.target.value)} placeholder="输入卡密" required />
              </div>
            ) : (
              <div className="space-y-2">
                <Label>数量</Label>
                <Input
                  type="number"
                  min={1}
                  max={500}
                  value={idleCount}
                  onChange={(e) => setIdleCount(Number(e.target.value) || 1)}
                />
                <p className="text-xs text-muted-foreground">将随机挑选当前空间可用文件，成功后标记为「已补中转」</p>
              </div>
            )}
            {supplyRelay?.type === 'sub2api' ? (
              <div className="space-y-2">
                <Label>绑定分组</Label>
                <Select value={groupID || undefined} onValueChange={setGroupID} disabled={groupsLoading || groups.length === 0}>
                  <SelectTrigger>
                    <SelectValue placeholder={groupsLoading ? '加载分组中...' : groups.length ? '选择分组' : '暂无分组'} />
                  </SelectTrigger>
                  <SelectContent>
                    {groups.map((g) => (
                      <SelectItem key={g.id} value={String(g.id)}>
                        {g.name}
                        {g.platform ? ` (${g.platform})` : ''}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <p className="text-xs text-muted-foreground">补入的账号会自动绑定到所选分组，默认选中第一个</p>
              </div>
            ) : null}
          </div>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setSupplyRelay(null)} disabled={supplying}>
              取消
            </Button>
            <Button
              loading={supplying}
              disabled={supplyRelay?.type === 'sub2api' && (!groupID || groupsLoading)}
              onClick={async () => {
                if (!supplyRelay) return
                if (supplyRelay.type === 'sub2api' && !groupID) {
                  show('请选择要绑定的分组')
                  return
                }
                setSupplying(true)
                try {
                  const group_id = groupID ? Number(groupID) : undefined
                  const res =
                    supplyMode === 'cdkey'
                      ? await api.supplyRelay(supplyRelay.id, { mode: 'cdkey', card_code: cardCode, group_id })
                      : await api.supplyRelay(supplyRelay.id, { mode: 'idle', count: idleCount, group_id })
                  show(res.message, 'success')
                  setSupplyRelay(null)
                  await loadStats([supplyRelay])
                } catch (e) {
                  show(e instanceof Error ? e.message : '补号失败')
                } finally {
                  setSupplying(false)
                }
              }}
            >
              {supplying ? '补号中...' : '确认补号'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
