import { useMemo, useRef, useState } from 'react'
import { api } from '@/lib/api'
import { Button, Card, CardContent, PageHeader, Toast } from '@/components/ui'
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
  const inputRef = useRef<HTMLInputElement>(null)
  const { toast, show } = useToast()

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
            message: res.message || `新增 ${res.created || 0} 个，重复 ${res.duplicated || 0} 个`,
          })
        } catch (e) {
          failed += 1
          updateItem(item.id, {
            status: 'error',
            message: e instanceof Error ? e.message : '上传失败',
          })
        }
      }

      const summary = `新增 ${createdTotal} 个，重复 ${duplicatedTotal} 个`
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
    }
  }

  return (
    <div>
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
    </div>
  )
}
