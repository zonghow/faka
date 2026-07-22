import { useEffect, useMemo, useRef, useState } from 'react'
import { api, type Pagination, type UploadRecord } from '@/lib/api'
import { Button, Card, CardContent, CardHeader, CardTitle, PageHeader, Toast } from '@/components/ui'
import { useToast } from '@/hooks/useToast'
import { cn } from '@/lib/utils'

const MAX_UPLOAD_BYTES = 500 * 1024 * 1024
const ACCEPT_EXT = ['.json', '.zip']

type UploadItem = {
  id: string
  file: File
  progress: number
  status: 'idle' | 'uploading' | 'done' | 'error'
  message?: string
}

function isAcceptedFile(file: File) {
  const name = file.name.toLowerCase()
  return ACCEPT_EXT.some((ext) => name.endsWith(ext))
}

function isZipFile(file: File) {
  return file.name.toLowerCase().endsWith('.zip')
}

function formatSize(bytes: number) {
  if (bytes >= 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024 / 1024).toFixed(2)} GB`
  if (bytes >= 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`
  if (bytes >= 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${bytes} B`
}

function fileKey(file: File) {
  return `${file.name}::${file.size}::${file.lastModified}`
}

function mergeItems(current: UploadItem[], incoming: File[]) {
  const map = new Map<string, UploadItem>()
  for (const item of current) map.set(item.id, item)
  for (const file of incoming) {
    const id = fileKey(file)
    if (!map.has(id)) {
      map.set(id, {
        id,
        file,
        progress: 0,
        status: 'idle',
      })
    }
  }
  return Array.from(map.values())
}

export function UploadsPage() {
  const [items, setItems] = useState<UploadItem[]>([])
  const [loading, setLoading] = useState(false)
  const [dragging, setDragging] = useState(false)
  const [records, setRecords] = useState<UploadRecord[]>([])
  const [recordsPagination, setRecordsPagination] = useState<Pagination | null>(null)
  const [recordsPage, setRecordsPage] = useState(1)
  const [recordsLoading, setRecordsLoading] = useState(true)
  const inputRef = useRef<HTMLInputElement>(null)
  const { toast, show } = useToast()

  const loadRecords = async () => {
    setRecordsLoading(true)
    try {
      const res = await api.uploadRecords(recordsPage)
      setRecords(res.records)
      setRecordsPagination(res.pagination)
    } finally {
      setRecordsLoading(false)
    }
  }

  useEffect(() => {
    setRecordsLoading(true)
    api.uploadRecords(recordsPage)
      .then((res) => {
        setRecords(res.records)
        setRecordsPagination(res.pagination)
      })
      .catch((e) => show(e instanceof Error ? e.message : '上传记录加载失败'))
      .finally(() => setRecordsLoading(false))
  }, [recordsPage, show])

  const totalBytes = useMemo(() => items.reduce((sum, item) => sum + item.file.size, 0), [items])
  const oversized = totalBytes > MAX_UPLOAD_BYTES
  const invalid = items.filter((item) => !isAcceptedFile(item.file))
  const canUpload = items.length > 0 && !loading && !oversized && invalid.length === 0

  const updateItem = (id: string, patch: Partial<UploadItem>) => {
    setItems((prev) => prev.map((item) => (item.id === id ? { ...item, ...patch } : item)))
  }

  const addFiles = (list: FileList | File[]) => {
    const incoming = Array.from(list)
    if (!incoming.length) return
    setItems((prev) => mergeItems(prev, incoming))
  }

  const onDrop = (e: React.DragEvent<HTMLDivElement>) => {
    e.preventDefault()
    e.stopPropagation()
    setDragging(false)
    if (e.dataTransfer.files?.length) addFiles(e.dataTransfer.files)
  }

  const startUpload = async () => {
    if (!canUpload) return
    setLoading(true)
    let createdTotal = 0
    let duplicatedTotal = 0
    let failed = 0

    try {
      for (const item of items) {
        if (!isAcceptedFile(item.file)) {
          updateItem(item.id, { status: 'error', message: '不支持的格式', progress: 0 })
          failed += 1
          continue
        }

        updateItem(item.id, { status: 'uploading', progress: 0, message: undefined })
        try {
          const res = await api.uploadFileWithProgress(item.file, (percent) => {
            updateItem(item.id, { progress: percent, status: 'uploading' })
          })
          createdTotal += res.created || 0
          duplicatedTotal += res.duplicated || 0
          updateItem(item.id, {
            status: 'done',
            progress: 100,
            message: res.message || `新增 ${res.created || 0} 个，覆盖 ${res.duplicated || 0} 个`,
          })
        } catch (e) {
          failed += 1
          updateItem(item.id, {
            status: 'error',
            message: e instanceof Error ? e.message : '上传失败',
          })
        }
      }

      const summary = `新增 ${createdTotal} 个，覆盖 ${duplicatedTotal} 个`
      if (failed === 0) {
        show(`上传完成：${summary}`, 'success')
        setItems([])
      } else if (createdTotal + duplicatedTotal > 0) {
        show(`部分成功：${summary}，失败 ${failed} 个`)
      } else {
        show(`上传失败 ${failed} 个文件`)
      }
    } finally {
      setLoading(false)
      if (recordsPage === 1) {
        loadRecords().catch((e) => show(e instanceof Error ? e.message : '上传记录刷新失败'))
      } else {
        setRecordsPage(1)
      }
    }
  }

  return (
    <div className="h-full overflow-auto pb-1">
      {toast ? <Toast message={toast.message} type={toast.type} /> : null}
      <PageHeader title="上传文件" desc="支持拖拽/多选 .json 与 .zip（可多个 zip），单批不超过 500MB，文件名不能包含中文" />
      <Card>
        <CardContent className="space-y-4 p-5">
          <div
            className={cn(
              'rounded-lg border border-dashed px-4 py-10 text-center transition',
              dragging ? 'border-ring bg-accent' : 'border-border bg-muted/30 hover:bg-accent/40',
            )}
            onDragEnter={(e) => {
              e.preventDefault()
              e.stopPropagation()
              setDragging(true)
            }}
            onDragOver={(e) => {
              e.preventDefault()
              e.stopPropagation()
              setDragging(true)
            }}
            onDragLeave={(e) => {
              e.preventDefault()
              e.stopPropagation()
              setDragging(false)
            }}
            onDrop={onDrop}
            onClick={() => inputRef.current?.click()}
            role="button"
            tabIndex={0}
            onKeyDown={(e) => {
              if (e.key === 'Enter' || e.key === ' ') inputRef.current?.click()
            }}
          >
            <div className="text-sm font-medium">拖拽文件到此处，或点击选择</div>
            <div className="mt-2 text-xs text-muted-foreground">支持多个 .json / .zip，可同时拖入多个 zip</div>
            <input
              ref={inputRef}
              type="file"
              multiple
              accept=".json,.zip,application/json,application/zip"
              className="hidden"
              onChange={(e) => {
                if (e.target.files?.length) addFiles(e.target.files)
                e.currentTarget.value = ''
              }}
            />
          </div>

          <div className="text-sm text-muted-foreground">
            {items.length ? `已选择 ${items.length} 个文件，共 ${formatSize(totalBytes)}` : '尚未选择文件'}
            {oversized ? <span className="ml-2 text-destructive">已超过 500MB 限制</span> : null}
            {invalid.length ? <span className="ml-2 text-destructive">含有非 .json/.zip 文件</span> : null}
          </div>

          {items.length > 0 ? (
            <div className="max-h-80 space-y-3 overflow-auto rounded-md border p-3">
              {items.map((item) => {
                const showProgress = isZipFile(item.file) || item.status === 'uploading' || item.status === 'done' || item.status === 'error'
                return (
                  <div key={item.id} className="space-y-2 rounded-md border border-border/60 p-3">
                    <div className="flex items-start justify-between gap-3 text-sm">
                      <div className="min-w-0">
                        <div className="truncate font-medium">{item.file.name}</div>
                        <div className="text-xs text-muted-foreground">
                          {formatSize(item.file.size)}
                          {!isAcceptedFile(item.file) ? ' · 不支持的格式' : ''}
                          {item.status === 'uploading' ? ` · 上传中 ${item.progress}%` : ''}
                          {item.status === 'done' ? ' · 完成' : ''}
                          {item.status === 'error' ? ` · 失败${item.message ? `：${item.message}` : ''}` : ''}
                        </div>
                      </div>
                      <Button
                        type="button"
                        size="sm"
                        variant="ghost"
                        disabled={loading && item.status === 'uploading'}
                        onClick={() => setItems((prev) => prev.filter((f) => f.id !== item.id))}
                      >
                        移除
                      </Button>
                    </div>

                    {showProgress ? (
                      <div className="space-y-1">
                        <div className="h-2 overflow-hidden rounded-full bg-muted">
                          <div
                            className={cn(
                              'h-full rounded-full transition-all duration-200',
                              item.status === 'error' ? 'bg-destructive' : 'bg-primary',
                            )}
                            style={{ width: `${item.status === 'error' ? Math.max(item.progress, 8) : item.progress}%` }}
                          />
                        </div>
                        <div className="text-right text-xs text-muted-foreground">{item.progress}%</div>
                      </div>
                    ) : null}
                  </div>
                )
              })}
            </div>
          ) : null}

          <div className="flex flex-wrap gap-2">
            <Button disabled={!canUpload && !loading} loading={loading} onClick={startUpload}>
              {loading ? '上传中...' : '开始上传'}
            </Button>
            <Button type="button" variant="outline" disabled={!items.length || loading} onClick={() => setItems([])}>
              清空列表
            </Button>
          </div>
        </CardContent>
      </Card>

      <Card className="mt-4 overflow-hidden">
        <CardHeader className="border-b p-4">
          <CardTitle className="text-base">上传记录</CardTitle>
        </CardHeader>
        <div className="max-h-[32rem] overflow-auto">
          <table className="w-full min-w-[640px] text-sm">
            <thead className="sticky top-0 border-b border-border bg-card text-left text-muted-foreground">
              <tr>
                <th className="w-44 px-4 py-3 font-medium">上传时间</th>
                <th className="px-4 py-3 font-medium">文件名</th>
                <th className="w-28 px-4 py-3 text-right font-medium">新增数量</th>
                <th className="w-28 px-4 py-3 text-right font-medium">覆盖数量</th>
              </tr>
            </thead>
            <tbody>
              {records.map((record) => (
                <tr key={record.id} className="border-b border-border/50 last:border-b-0">
                  <td className="whitespace-nowrap px-4 py-3">{record.created_at || '-'}</td>
                  <td className="max-w-0 truncate px-4 py-3 font-mono text-xs" title={record.filename}>{record.filename}</td>
                  <td className="px-4 py-3 text-right tabular-nums">{record.created_count}</td>
                  <td className="px-4 py-3 text-right tabular-nums">{record.overwritten_count}</td>
                </tr>
              ))}
              {!records.length ? (
                <tr>
                  <td className="px-4 py-8 text-center text-muted-foreground" colSpan={4}>
                    {recordsLoading ? '加载中...' : '暂无上传记录'}
                  </td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>
        {recordsPagination ? (
          <div className="flex flex-wrap items-center justify-between gap-3 border-t border-border px-4 py-3 text-sm text-muted-foreground">
            <span>共 {recordsPagination.total} 条，第 {recordsPagination.page}/{recordsPagination.total_pages} 页</span>
            <div className="flex gap-2">
              <Button
                type="button"
                size="sm"
                variant="ghost"
                disabled={!recordsPagination.has_prev || recordsLoading}
                onClick={() => setRecordsPage(recordsPagination.prev_page)}
              >
                上一页
              </Button>
              <Button
                type="button"
                size="sm"
                variant="ghost"
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
  )
}
