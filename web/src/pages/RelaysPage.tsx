import { useCallback, useEffect, useMemo, useState } from 'react'
import { api, type Pagination, type Relay, type RelayGroup, type RelayStats, type RelaySupplyRecord } from '@/lib/api'
import {
  Badge,
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
  Textarea,
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

const quickCounts = [100, 200, 300, 500, 1000, 2000, 3000]

function dash(v: number | undefined | null) {
  if (v === undefined || v === null) return '-'
  return String(v)
}

function modeLabel(mode: string) {
  if (mode === 'cdkey') return 'CDKey'
  if (mode === 'idle') return '空闲文件'
  return mode || '-'
}

function statusVariant(status: string): 'default' | 'success' | 'warn' | 'danger' {
  if (status === 'success') return 'success'
  if (status === 'partial') return 'warn'
  if (status === 'failed') return 'danger'
  return 'default'
}

function statusLabel(status: string) {
  if (status === 'success') return '成功'
  if (status === 'partial') return '部分成功'
  if (status === 'failed') return '失败'
  return status || '-'
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
  const [supplyMode, setSupplyMode] = useState<'cdkey' | 'idle'>('idle')
  const [cardCode, setCardCode] = useState('')
  const [idleCount, setIdleCount] = useState(1)
  const [groups, setGroups] = useState<RelayGroup[]>([])
  const [groupID, setGroupID] = useState<string>('')
  const [groupsLoading, setGroupsLoading] = useState(false)
  const [concurrency, setConcurrency] = useState(10)
  const [fillingFreeCount, setFillingFreeCount] = useState(false)
  const [supplying, setSupplying] = useState(false)
  const [records, setRecords] = useState<RelaySupplyRecord[]>([])
  const [recordsPage, setRecordsPage] = useState(1)
  const [recordsPagination, setRecordsPagination] = useState<Pagination | null>(null)
  const [recordsLoading, setRecordsLoading] = useState(true)
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

  const loadRecords = useCallback(async (page = recordsPage) => {
    setRecordsLoading(true)
    try {
      const res = await api.relaySupplyRecords(page, 50)
      setRecords(res.records || [])
      setRecordsPagination(res.pagination)
    } finally {
      setRecordsLoading(false)
    }
  }, [recordsPage])

  useEffect(() => {
    load().catch((e) => show(e.message))
  }, [load, show])

  useEffect(() => {
    loadRecords(recordsPage).catch((e) => show(e instanceof Error ? e.message : '补号记录加载失败'))
  }, [loadRecords, recordsPage, show])

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

  const fillFreeCount = async () => {
    setFillingFreeCount(true)
    try {
      const res = await api.dashboard()
      const freeFiles = Number(res.stats['当前空闲文件数'] || 0)
      if (freeFiles <= 0) {
        show('当前没有空闲文件')
        return
      }
      const count = Math.min(freeFiles, 500)
      setIdleCount(count)
      if (freeFiles > 500) {
        show('空闲文件数超过补号上限，已填入 500', 'success')
      }
    } catch (e) {
      show(e instanceof Error ? e.message : '空闲文件数加载失败')
    } finally {
      setFillingFreeCount(false)
    }
  }

  const openSupply = async (r: Relay) => {
    setSupplyRelay(r)
    setSupplyMode('idle')
    setCardCode('')
    setIdleCount(1)
    setConcurrency(10)
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

          <Card className="overflow-x-auto">
            <CardHeader className="flex flex-row items-center justify-between gap-2">
              <CardTitle>补号记录</CardTitle>
              <Button size="sm" variant="secondary" onClick={() => loadRecords(recordsPage)} disabled={recordsLoading}>
                刷新
              </Button>
            </CardHeader>
            <table className="w-full text-sm">
              <thead className="border-b border-border text-left text-muted-foreground">
                <tr>
                  <th className="px-4 py-3">时间</th>
                  <th className="px-4 py-3">中转</th>
                  <th className="px-4 py-3">类型</th>
                  <th className="px-4 py-3">方式</th>
                  <th className="px-4 py-3">空间</th>
                  <th className="px-4 py-3">分组</th>
                  <th className="px-4 py-3">并发</th>
                  <th className="px-4 py-3">请求</th>
                  <th className="px-4 py-3">成功</th>
                  <th className="px-4 py-3">失败</th>
                  <th className="px-4 py-3">状态</th>
                  <th className="px-4 py-3">说明</th>
                </tr>
              </thead>
              <tbody>
                {records.length === 0 ? (
                  <tr>
                    <td colSpan={12} className="px-4 py-6 text-center text-muted-foreground">
                      {recordsLoading ? '加载中...' : '暂无补号记录'}
                    </td>
                  </tr>
                ) : (
                  records.map((r) => (
                    <tr key={r.id} className="border-b border-border/50 align-top">
                      <td className="whitespace-nowrap px-4 py-3 text-xs">{r.created_at || '-'}</td>
                      <td className="px-4 py-3 font-medium">{r.relay_name}</td>
                      <td className="px-4 py-3 font-mono text-xs">{r.relay_type}</td>
                      <td className="px-4 py-3">{modeLabel(r.mode)}</td>
                      <td className="px-4 py-3">{r.space_name || '-'}</td>
                      <td className="px-4 py-3">{r.group_name || (r.group_id ? String(r.group_id) : '-')}</td>
                      <td className="px-4 py-3">{r.concurrency > 0 ? r.concurrency : '-'}</td>
                      <td className="px-4 py-3">
                        {r.mode === 'cdkey'
                          ? r.card_count > 0
                            ? `${r.card_count} 卡密`
                            : dash(r.request_count)
                          : dash(r.request_count)}
                      </td>
                      <td className="px-4 py-3">{dash(r.supplied)}</td>
                      <td className="px-4 py-3">{dash(r.failed)}</td>
                      <td className="px-4 py-3">
                        <Badge variant={statusVariant(r.status)}>{statusLabel(r.status)}</Badge>
                      </td>
                      <td className="max-w-[280px] px-4 py-3 text-xs text-muted-foreground" title={r.errors || r.message}>
                        <div className="line-clamp-2">{r.message || '-'}</div>
                        {r.errors ? <div className="mt-1 line-clamp-2 text-destructive">{r.errors}</div> : null}
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
            {recordsPagination ? (
              <div className="flex items-center justify-between gap-3 border-t border-border px-4 py-3 text-sm text-muted-foreground">
                <div>
                  共 {recordsPagination.total} 条 · 第 {recordsPagination.page}/{recordsPagination.total_pages} 页
                </div>
                <div className="flex gap-2">
                  <Button
                    size="sm"
                    variant="secondary"
                    disabled={!recordsPagination.has_prev || recordsLoading}
                    onClick={() => setRecordsPage(recordsPagination.prev_page)}
                  >
                    上一页
                  </Button>
                  <Button
                    size="sm"
                    variant="secondary"
                    disabled={!recordsPagination.has_next || recordsLoading}
                    onClick={() => setRecordsPage(recordsPagination.next_page)}
                  >
                    下一页
                  </Button>
                </div>
              </div>
            ) : null}
          </Card>
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
                  <SelectItem value="idle">使用空闲文件</SelectItem>
                  <SelectItem value="cdkey">使用 CDKey</SelectItem>
                </SelectContent>
              </Select>
            </div>
            {supplyMode === 'cdkey' ? (
              <div className="space-y-2">
                <Label>卡密</Label>
                <Textarea
                  value={cardCode}
                  onChange={(e) => setCardCode(e.target.value)}
                  placeholder={'支持多个卡密，每行一个\n也可用逗号/空格分隔'}
                  className="min-h-28"
                  required
                />
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
                <div className="flex flex-wrap gap-1 pt-1">
                  {quickCounts.map((count) => (
                    <Button
                      key={count}
                      type="button"
                      size="sm"
                      variant={idleCount === count ? 'secondary' : 'outline'}
                      className="h-6 px-2"
                      onClick={() => setIdleCount(count)}
                    >
                      {count}
                    </Button>
                  ))}
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    className="h-6 px-2"
                    loading={fillingFreeCount}
                    disabled={supplying || fillingFreeCount}
                    onClick={() => fillFreeCount()}
                  >
                    填入空闲数
                  </Button>
                </div>
                <p className="text-xs text-muted-foreground">将随机挑选当前空间可用文件，成功后标记为「已补中转」</p>
              </div>
            )}
            {supplyRelay?.type === 'sub2api' ? (
              <>
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
                <div className="space-y-2">
                  <Label>账号并发</Label>
                  <Input
                    type="number"
                    min={1}
                    max={10000}
                    value={concurrency}
                    onChange={(e) => setConcurrency(Number(e.target.value) || 10)}
                  />
                  <p className="text-xs text-muted-foreground">设置这批补入账号的并发数，默认 10</p>
                </div>
              </>
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
                  const conc = supplyRelay.type === 'sub2api' ? concurrency : undefined
                  const res =
                    supplyMode === 'cdkey'
                      ? await api.supplyRelay(supplyRelay.id, { mode: 'cdkey', card_code: cardCode, group_id, concurrency: conc })
                      : await api.supplyRelay(supplyRelay.id, { mode: 'idle', count: idleCount, group_id, concurrency: conc })
                  show(res.message, 'success')
                  setSupplyRelay(null)
                  await Promise.all([loadStats([supplyRelay]), loadRecords(1)])
                  setRecordsPage(1)
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
