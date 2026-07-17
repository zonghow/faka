import { useEffect, useState } from 'react'
import { api, type Space } from '@/lib/api'
import { Button, Card, CardContent, PageHeader, Toast } from '@/components/ui'
import { formatNumber } from '@/lib/utils'
import { useToast } from '@/hooks/useToast'
import { useConfirm } from '@/components/confirm-provider'

export function DashboardPage() {
  const [stats, setStats] = useState<Record<string, number>>({})
  const [space, setSpace] = useState<Space | null>(null)
  const [loading, setLoading] = useState(true)
  const [clearing, setClearing] = useState(false)
  const { toast, show } = useToast()
  const confirm = useConfirm()

  const load = async () => {
    setLoading(true)
    try {
      const res = await api.dashboard()
      setStats(res.stats)
      setSpace(res.current_space)
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
      <PageHeader title="数据概览" desc={space ? `当前空间：${space.name}（前缀 ${space.card_prefix}）` : loading ? '加载中...' : ''} />
      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
        {Object.entries(stats).map(([label, value]) => (
          <Card key={label}>
            <CardContent className="p-5">
              <div className="text-sm text-muted-foreground">{label}</div>
              <div className="mt-3 text-3xl font-bold">{formatNumber(value)}</div>
            </CardContent>
          </Card>
        ))}
        {loading && Object.keys(stats).length === 0 ? (
          <Card>
            <CardContent className="p-5 text-sm text-muted-foreground">加载中...</CardContent>
          </Card>
        ) : null}
      </div>
      <Card className="mt-6 border-destructive/40">
        <CardContent className="p-5">
          <h2 className="text-lg font-semibold">危险操作</h2>
          <p className="mt-2 text-sm text-muted-foreground">清空当前空间下全部卡密、文件、兑换记录和审计日志，不可恢复。</p>
          <Button
            className="mt-4"
            variant="destructive"
            loading={clearing}
            onClick={async () => {
              const ok = await confirm({
                title: '清空当前空间数据',
                description: '将删除当前空间下全部卡密、文件、兑换记录和审计日志，此操作不可恢复。',
                confirmText: '确认清空',
                cancelText: '取消',
                danger: true,
              })
              if (!ok) return
              setClearing(true)
              try {
                const res = await api.clearSpace()
                show(res.message, 'success')
                await load()
              } catch (e) {
                show(e instanceof Error ? e.message : '清空失败')
              } finally {
                setClearing(false)
              }
            }}
          >
            {clearing ? '清空中...' : '清空当前空间数据'}
          </Button>
        </CardContent>
      </Card>
    </div>
  )
}
