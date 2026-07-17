import { useEffect, useMemo, useState } from 'react'
import { api, downloadBlob, type ManagedFile, type Pagination } from '@/lib/api'
import {
  Badge,
  Button,
  Card,
  CardContent,
  Checkbox,
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
  locked: '锁定中',
  sold: '已提取',
  voided: '已作废',
}

function statusVariant(status: string): 'default' | 'success' | 'warn' | 'danger' {
  if (status === 'available') return 'success'
  if (status === 'locked') return 'warn'
  if (status === 'voided') return 'danger'
  return 'default'
}

export function FilesPage() {
  const [files, setFiles] = useState<ManagedFile[]>([])
  const [pagination, setPagination] = useState<Pagination | null>(null)
  const [selected, setSelected] = useState<number[]>([])
  const [q, setQ] = useState('')
  const [cardCode, setCardCode] = useState('')
  const [status, setStatus] = useState('all')
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState('200')
  const [targetStatus, setTargetStatus] = useState('available')
  const [loading, setLoading] = useState(true)
  const [filtering, setFiltering] = useState(false)
  const [updating, setUpdating] = useState(false)
  const [downloading, setDownloading] = useState(false)
  const { toast, show } = useToast()
  const confirm = useConfirm()

  const load = async () => {
    setLoading(true)
    try {
      const params = new URLSearchParams({ page: String(page), page_size: pageSize })
      if (q) params.set('q', q)
      if (cardCode) params.set('card_code', cardCode)
      if (status && status !== 'all') params.set('status', status)
      const res = await api.files(params)
      setFiles(res.files)
      setPagination(res.pagination)
      setSelected([])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load().catch((e) => show(e.message))
  }, [page, pageSize, status])

  const allChecked = useMemo(() => files.length > 0 && selected.length === files.length, [files, selected])
  const { parentRef, items: virtualItems, paddingTop, paddingBottom } = useTableVirtualizer(files.length)

  return (
    <div className="flex h-[calc(100vh-7.5rem)] flex-col">
      {toast ? <Toast message={toast.message} type={toast.type} /> : null}
      <div className="shrink-0">
        <PageHeader title="文件管理" desc="查看库存、售出、锁定和作废状态" />
      </div>

      <Card className="mb-4 shrink-0">
        <CardContent className="flex flex-wrap items-end gap-3 p-4">
          <div className="space-y-1">
            <Label>文件名</Label>
            <Input className="w-48" placeholder="搜索文件名" value={q} onChange={(e) => setQ(e.target.value)} />
          </div>
          <div className="space-y-1">
            <Label>卡密</Label>
            <Input className="w-56" placeholder="卡密精确搜索" value={cardCode} onChange={(e) => setCardCode(e.target.value)} />
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
        </CardContent>
      </Card>

      <Card className="mb-4">
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
                await api.updateFileStatus(selected, targetStatus)
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
              const mark = await confirm({
                title: '下载文件',
                description: '下载后是否将选中文件标记为「已使用」？',
                confirmText: '标记并下载',
                cancelText: '仅下载',
              })
              setDownloading(true)
              try {
                const { blob, filename } = await api.downloadFiles(selected, mark)
                downloadBlob(blob, filename)
                show('下载已开始', 'success')
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
      </Card>

      <Card className="min-h-0 flex-1 overflow-hidden">
        <div ref={parentRef} className="h-full overflow-auto">
          <table className="w-full min-w-[900px] table-fixed text-sm">
            <colgroup>
              <col className="w-12" />
              <col className="w-16" />
              <col />
              <col className="w-24" />
              <col className="w-40" />
              <col className="w-56" />
              <col className="w-40" />
            </colgroup>
            <thead className="sticky top-0 z-10 border-b border-border bg-card text-left text-muted-foreground">
              <tr>
                <th className="px-3 py-3">
                  <Checkbox checked={allChecked} onCheckedChange={(v) => setSelected(v ? files.map((f) => f.id) : [])} />
                </th>
                <th className="px-3 py-3">序号</th>
                <th className="px-3 py-3">文件名</th>
                <th className="px-3 py-3">状态</th>
                <th className="px-3 py-3">首次提取</th>
                <th className="px-3 py-3">关联卡密</th>
                <th className="px-3 py-3">上传时间</th>
              </tr>
            </thead>
            <tbody>
              {paddingTop > 0 ? (
                <tr aria-hidden>
                  <td colSpan={7} style={{ height: paddingTop, padding: 0, border: 0 }} />
                </tr>
              ) : null}
              {virtualItems.map((virtualRow) => {
                const file = files[virtualRow.index]
                return (
                  <tr key={file.id} className="border-b border-border/50" style={{ height: virtualRow.size }}>
                    <td className="px-3 py-3">
                      <Checkbox
                        checked={selected.includes(file.id)}
                        onCheckedChange={(v) => setSelected((prev) => (v ? [...prev, file.id] : prev.filter((id) => id !== file.id)))}
                      />
                    </td>
                    <td className="px-3 py-3">{virtualRow.index + 1}</td>
                    <td className="truncate px-3 py-3 font-mono text-xs">{file.original_name}</td>
                    <td className="px-3 py-3"><Badge variant={statusVariant(file.status)}>{statusLabel[file.status] || file.status}</Badge></td>
                    <td className="px-3 py-3">{file.sold_at || '-'}</td>
                    <td className="truncate px-3 py-3 font-mono text-xs">{file.sold_card || '-'}</td>
                    <td className="px-3 py-3">{file.uploaded_at || '-'}</td>
                  </tr>
                )
              })}
              {paddingBottom > 0 ? (
                <tr aria-hidden>
                  <td colSpan={7} style={{ height: paddingBottom, padding: 0, border: 0 }} />
                </tr>
              ) : null}
              {!files.length ? <tr><td className="px-3 py-8 text-center text-muted-foreground" colSpan={7}>暂无文件</td></tr> : null}
            </tbody>
          </table>
        </div>
      </Card>

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
    </div>
  )
}
