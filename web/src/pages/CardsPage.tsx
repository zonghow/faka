import { useEffect, useMemo, useState } from 'react'
import { api, downloadBlob, type Card, type Pagination } from '@/lib/api'
import {
  Badge,
  Button,
  Card as UICard,
  CardContent,
  Checkbox,
  Dialog,
  DialogContent,
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
import { useTableVirtualizer } from '@/hooks/useTableVirtualizer'

const statusLabel: Record<string, string> = {
  available: '可用',
  pending: '待使用',
  sold: '已使用',
  voided: '已作废',
}

function statusVariant(status: string): 'default' | 'success' | 'warn' | 'danger' {
  if (status === 'available') return 'success'
  if (status === 'pending') return 'warn'
  if (status === 'voided') return 'danger'
  return 'default'
}

async function copyText(text: string) {
  let textarea: HTMLTextAreaElement | null = null
  try {
    if (window.isSecureContext && navigator.clipboard) {
      await navigator.clipboard.writeText(text)
      return true
    }

    textarea = document.createElement('textarea')
    textarea.value = text
    textarea.readOnly = true
    textarea.style.position = 'fixed'
    textarea.style.opacity = '0'
    document.body.appendChild(textarea)
    textarea.select()
    textarea.setSelectionRange(0, text.length)
    return document.execCommand('copy')
  } catch {
    return false
  } finally {
    textarea?.remove()
  }
}

export function CardsPage() {
  const [cards, setCards] = useState<Card[]>([])
  const [pagination, setPagination] = useState<Pagination | null>(null)
  const [selected, setSelected] = useState<number[]>([])
  const [q, setQ] = useState('')
  const [status, setStatus] = useState('all')
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState('5000')
  const [targetStatus, setTargetStatus] = useState('pending')
  const [fileCount, setFileCount] = useState(1)
  const [quantity, setQuantity] = useState(1)
  const [usage, setUsage] = useState<{ card_code: string; first_used_at: string; redemptions: Array<Record<string, unknown>> } | null>(null)
  const [loading, setLoading] = useState(true)
  const [filtering, setFiltering] = useState(false)
  const [creating, setCreating] = useState(false)
  const [updating, setUpdating] = useState(false)
  const [downloading, setDownloading] = useState(false)
  const [detailLoadingId, setDetailLoadingId] = useState<number | null>(null)
  const { toast, show } = useToast()
  const confirm = useConfirm()

  const load = async () => {
    setLoading(true)
    try {
      const params = new URLSearchParams({ page: String(page), page_size: pageSize })
      if (q) params.set('q', q)
      if (status && status !== 'all') params.set('status', status)
      const res = await api.cards(params)
      setCards(res.cards)
      setPagination(res.pagination)
      setSelected([])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load().catch((e) => show(e.message))
  }, [page, pageSize, status])

  const allChecked = useMemo(() => cards.length > 0 && selected.length === cards.length, [cards, selected])
  const { parentRef, items: virtualItems, paddingTop, paddingBottom } = useTableVirtualizer(cards.length)

  return (
    <div className="flex h-[calc(100vh-7.5rem)] flex-col">
      {toast ? <Toast message={toast.message} type={toast.type} /> : null}
      <div className="shrink-0">
        <PageHeader title="卡密管理" desc="生成、搜索和修改卡密状态" />
      </div>

      <UICard className="mb-4 shrink-0">
        <CardContent className="flex flex-wrap items-end gap-3 p-4">
          <div className="space-y-1">
            <Label>搜索</Label>
            <Input className="w-56" placeholder="搜索卡密" value={q} onChange={(e) => setQ(e.target.value)} />
          </div>
          <div className="space-y-1">
            <Label>状态</Label>
            <Select value={status} onValueChange={(v) => { setStatus(v); setPage(1) }}>
              <SelectTrigger className="w-36"><SelectValue placeholder="全部状态" /></SelectTrigger>
              <SelectContent>
                <SelectItem value="all">全部状态</SelectItem>
                {Object.keys(statusLabel).map((s) => (
                  <SelectItem key={s} value={s}>{statusLabel[s]}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <Button
            variant="secondary"
            loading={filtering || loading}
            onClick={async () => {
              setFiltering(true)
              try {
                setPage(1)
                await load()
              } catch (e) {
                show(e instanceof Error ? e.message : '筛选失败')
              } finally {
                setFiltering(false)
              }
            }}
          >
            筛选
          </Button>
          <div className="ml-auto flex flex-wrap items-end gap-2">
            <div className="space-y-1">
              <Label>绑定文件数</Label>
              <Input className="w-28" type="number" min={1} value={fileCount} onChange={(e) => setFileCount(Number(e.target.value))} />
            </div>
            <div className="space-y-1">
              <Label>生成数量</Label>
              <Input className="w-28" type="number" min={1} value={quantity} onChange={(e) => setQuantity(Number(e.target.value))} />
            </div>
            <Button
              loading={creating}
              onClick={async () => {
                setCreating(true)
                try {
                  const res = await api.createCards(fileCount, quantity)
                  show((res as { message?: string }).message || '已生成', 'success')
                  await load()
                } catch (e) {
                  show(e instanceof Error ? e.message : '生成失败')
                } finally {
                  setCreating(false)
                }
              }}
            >
              {creating ? '生成中...' : '生成卡密'}
            </Button>
          </div>
        </CardContent>
      </UICard>

      <UICard className="mb-4">
        <CardContent className="flex flex-wrap items-center gap-3 p-4">
          <span className="text-sm text-muted-foreground">已选 {selected.length} 个</span>
          <Select value={targetStatus} onValueChange={setTargetStatus} disabled={!selected.length}>
            <SelectTrigger className="w-36"><SelectValue /></SelectTrigger>
            <SelectContent>
              {Object.keys(statusLabel).map((s) => (
                <SelectItem key={s} value={s}>{statusLabel[s]}</SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Button
            variant="secondary"
            disabled={!selected.length}
            loading={updating}
            onClick={async () => {
              setUpdating(true)
              try {
                await api.updateCardStatus(selected, targetStatus)
                show('状态已更新', 'success')
                await load()
              } catch (e) {
                show(e instanceof Error ? e.message : '更新失败')
              } finally {
                setUpdating(false)
              }
            }}
          >
            {updating ? '更新中...' : '批量修改状态'}
          </Button>
          <Button
            variant="secondary"
            disabled={!selected.length}
            loading={downloading}
            onClick={async () => {
              const selectedIds = new Set(selected)
              const selectedCodes = cards
                .filter((card) => selectedIds.has(card.id) && card.status === 'available')
                .sort((a, b) => a.id - b.id)
                .map((card) => card.code)
              const mark = await confirm({
                title: '下载卡密',
                description: '下载后是否将选中卡密标记为「待使用」？',
                confirmText: '标记并下载',
                cancelText: '仅下载',
              })
              const copied = mark && selectedCodes.length > 0 ? await copyText(selectedCodes.join('\n')) : false
              setDownloading(true)
              try {
                const { blob, filename } = await api.downloadCards(selected, mark)
                downloadBlob(blob, filename)
                show(mark ? (copied ? '下载已开始，卡密已复制' : '下载已开始，但卡密复制失败') : '下载已开始', copied || !mark ? 'success' : 'error')
                await load()
              } catch (e) {
                show(e instanceof Error ? e.message : '下载失败')
              } finally {
                setDownloading(false)
              }
            }}
          >
            {downloading ? '下载中...' : '批量下载'}
          </Button>
        </CardContent>
      </UICard>

      <UICard className="min-h-0 flex-1 overflow-hidden">
        <div ref={parentRef} className="h-full overflow-auto">
          <table className="w-full min-w-[900px] table-fixed text-sm">
            <colgroup>
              <col className="w-12" />
              <col className="w-16" />
              <col />
              <col className="w-24" />
              <col className="w-24" />
              <col className="w-40" />
              <col className="w-40" />
              <col className="w-28" />
            </colgroup>
            <thead className="sticky top-0 z-10 border-b border-border bg-card text-left text-muted-foreground">
              <tr>
                <th className="px-3 py-3">
                  <Checkbox checked={allChecked} onCheckedChange={(v) => setSelected(v ? cards.map((c) => c.id) : [])} />
                </th>
                <th className="px-3 py-3">序号</th>
                <th className="px-3 py-3">卡密</th>
                <th className="px-3 py-3">绑定文件</th>
                <th className="px-3 py-3">状态</th>
                <th className="px-3 py-3">首次使用</th>
                <th className="px-3 py-3">创建时间</th>
                <th className="px-3 py-3">操作</th>
              </tr>
            </thead>
            <tbody>
              {paddingTop > 0 ? (
                <tr aria-hidden>
                  <td colSpan={8} style={{ height: paddingTop, padding: 0, border: 0 }} />
                </tr>
              ) : null}
              {virtualItems.map((virtualRow) => {
                const card = cards[virtualRow.index]
                return (
                  <tr key={card.id} className="border-b border-border/50" style={{ height: virtualRow.size }}>
                    <td className="px-3 py-3">
                      <Checkbox
                        checked={selected.includes(card.id)}
                        onCheckedChange={(v) =>
                          setSelected((prev) => (v ? [...prev, card.id] : prev.filter((id) => id !== card.id)))
                        }
                      />
                    </td>
                    <td className="px-3 py-3">{virtualRow.index + 1}</td>
                    <td className="truncate px-3 py-3 font-mono text-xs">{card.code}</td>
                    <td className="px-3 py-3">{card.file_count}</td>
                    <td className="px-3 py-3"><Badge variant={statusVariant(card.status)}>{statusLabel[card.status] || card.status}</Badge></td>
                    <td className="px-3 py-3">{card.used_at || '-'}</td>
                    <td className="px-3 py-3">{card.created_at || '-'}</td>
                    <td className="px-3 py-3">
                      <Button
                        size="sm"
                        variant="ghost"
                        loading={detailLoadingId === card.id}
                        onClick={async () => {
                          setDetailLoadingId(card.id)
                          try {
                            setUsage(await api.cardRedemptions(card.id))
                          } catch (e) {
                            show(e instanceof Error ? e.message : '加载失败')
                          } finally {
                            setDetailLoadingId(null)
                          }
                        }}
                      >
                        使用详情
                      </Button>
                    </td>
                  </tr>
                )
              })}
              {paddingBottom > 0 ? (
                <tr aria-hidden>
                  <td colSpan={8} style={{ height: paddingBottom, padding: 0, border: 0 }} />
                </tr>
              ) : null}
              {!cards.length ? (
                <tr><td className="px-3 py-8 text-center text-muted-foreground" colSpan={8}>暂无卡密</td></tr>
              ) : null}
            </tbody>
          </table>
        </div>
      </UICard>

      {pagination ? (
        <div className="mt-4 flex shrink-0 flex-wrap items-center gap-3 border-t border-border/60 bg-background pt-4 text-sm text-muted-foreground">
          <span>共 {pagination.total} 条，第 {pagination.page}/{pagination.total_pages} 页</span>
          <Select value={pageSize} onValueChange={(v) => { setPageSize(v); setPage(1) }}>
            <SelectTrigger className="w-32"><SelectValue /></SelectTrigger>
            <SelectContent>
              {['50', '100', '200', '500', '1000', '2000', '5000', '10000'].map((n) => (
                <SelectItem key={n} value={n}>每页 {n}</SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Button size="sm" variant="ghost" disabled={!pagination.has_prev} onClick={() => setPage(pagination.prev_page)}>上一页</Button>
          <Button size="sm" variant="ghost" disabled={!pagination.has_next} onClick={() => setPage(pagination.next_page)}>下一页</Button>
        </div>
      ) : null}

      <Dialog open={!!usage} onOpenChange={(open) => !open && setUsage(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>使用详情</DialogTitle>
          </DialogHeader>
          <div className="font-mono text-xs text-muted-foreground">{usage?.card_code}</div>
          <div className="text-sm text-muted-foreground">首次使用：{usage?.first_used_at || '-'}</div>
          <div className="max-h-80 space-y-2 overflow-auto">
            {usage?.redemptions?.length ? usage.redemptions.map((r, i) => (
              <div key={String(r.id || i)} className="rounded-xl border border-border p-3 text-sm">
                <div>第 {i + 1} 次 · {String(r.file_count || 0)} 个文件</div>
                <div className="text-muted-foreground">{String(r.redeemed_at || '-')} · {String(r.output_format || '-')}</div>
              </div>
            )) : <div className="text-muted-foreground">暂无使用记录</div>}
          </div>
        </DialogContent>
      </Dialog>
    </div>
  )
}
