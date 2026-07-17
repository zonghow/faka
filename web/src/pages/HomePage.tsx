import { useEffect, useMemo, useState } from 'react'
import { api, downloadBlob } from '@/lib/api'
import { Button, Textarea, Toast } from '@/components/ui'
import { cn } from '@/lib/utils'
import { useToast } from '@/hooks/useToast'
import { DotField } from '@/components/bits/DotField'
import { FuzzyText } from '@/components/bits/FuzzyText'
import { GradientText } from '@/components/bits/GradientText'
import { GlowCard } from '@/components/bits/GlowCard'
import { CountUp } from '@/components/bits/CountUp'

const fieldClass =
  'min-h-48 resize-none rounded-2xl border border-white/[0.08] bg-[#0b0b0d] px-4 py-3 text-sm text-white/90 shadow-none placeholder:text-white/35 transition-colors focus-visible:border-[#a78bfa] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#a78bfa]/45'

const softBtnClass =
  'h-12 w-full rounded-2xl border border-white/[0.08] bg-[#17171a] text-sm font-medium text-white/55 shadow-none transition-colors hover:bg-[#1d1d22] hover:text-white/75'

type SpaceInventoryItem = {
  id: number
  name: string
  card_prefix: string
  inventory: number
}

export function HomePage() {
  const [spaces, setSpaces] = useState<SpaceInventoryItem[]>([])
  const [cardCode, setCardCode] = useState('')
  const [format, setFormat] = useState<'cpa' | 'sub'>('sub')
  const [loading, setLoading] = useState(false)
  const { toast, show } = useToast()

  const refresh = () =>
    api
      .inventory()
      .then((d) => setSpaces(d.spaces || []))
      .catch(() => setSpaces([]))

  useEffect(() => {
    refresh()
    const t = window.setInterval(refresh, 10000)
    return () => window.clearInterval(t)
  }, [])

  const cardPlaceholder = useMemo(() => {
    const sample = 'XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX'
    const prefixes =
      spaces.length > 0
        ? spaces.map((s) => (s.card_prefix || 'DEFAULT').toUpperCase())
        : ['DEFAULT']
    return Array.from({ length: 3 }, (_, i) => `${prefixes[i % prefixes.length]}-${sample}`).join('\n')
  }, [spaces])

  return (
    <div className="relative min-h-screen overflow-hidden bg-[#120F17]">
      <DotField
        backgroundColor="#120F17"
        gradientFrom="rgba(168, 85, 247, 0.35)"
        gradientTo="rgba(180, 151, 207, 0.25)"
        glowColor="#120F17"
        dotRadius={1.5}
        dotSpacing={14}
        bulgeStrength={67}
        glowRadius={160}
        waveAmplitude={0}
      />
      {toast ? <Toast message={toast.message} type={toast.type} /> : null}

      <div className="relative z-10 mx-auto flex min-h-screen max-w-xl flex-col justify-center px-6 py-12">
        <header className="mb-8 text-center">
          <div className="mb-3 text-xs tracking-[0.35em] text-[#a1a1aa] uppercase">redeem portal</div>
          <div className="flex justify-center">
            <FuzzyText
              fontSize="clamp(2rem, 7vw, 3.5rem)"
              fontWeight={900}
              color="#ffffff"
              enableHover
              baseIntensity={0.18}
              hoverIntensity={0.55}
              fuzzRange={28}
              gradient={['#A855F7', '#B497CF', '#ffffff']}
            >
              whistlelads faka
            </FuzzyText>
          </div>
          <div className="mt-4 text-base md:text-lg">
            <GradientText text="输入卡密，一键提取" className="font-semibold" />
          </div>
        </header>

        <div className="mb-5 px-1">
          {spaces.length > 0 ? (
            <div className="mx-auto flex flex-wrap items-baseline justify-center gap-x-4 gap-y-1">
              {spaces.map((space) => (
                <div key={space.id} className="flex items-baseline gap-1.5 text-sm">
                  <span className="text-white/70">{space.name}</span>
                  <CountUp
                    to={space.inventory}
                    from={0}
                    duration={0.45}
                    separator=","
                    className="tabular-nums text-lg font-semibold text-white"
                  />
                </div>
              ))}
            </div>
          ) : (
            <div className="text-center text-sm text-white/40">暂无空间</div>
          )}
        </div>

        <GlowCard>
          <form
            className="space-y-6"
            onSubmit={async (e) => {
              e.preventDefault()
              setLoading(true)
              try {
                const { blob, filename } = await api.redeem(cardCode, format)
                downloadBlob(blob, filename)
                show('提取成功，下载已开始', 'success')
                refresh()
              } catch (err) {
                show(err instanceof Error ? err.message : '提取失败')
              } finally {
                setLoading(false)
              }
            }}
          >
            <div>
              <Textarea
                id="cardCode"
                value={cardCode}
                onChange={(e) => setCardCode(e.target.value.toUpperCase().replace(/[^A-Z0-9\-\s,，;；]/g, ''))}
                placeholder={cardPlaceholder}
                required
                className={fieldClass}
              />
            </div>

            <div className="grid grid-cols-2 gap-3">
              {(['cpa', 'sub'] as const).map((f) => {
                const active = format === f
                return (
                  <Button
                    key={f}
                    type="button"
                    variant="outline"
                    onClick={() => setFormat(f)}
                    className={cn(
                      softBtnClass,
                      active && 'border-[#a78bfa]/70 bg-[#1a1a1f] text-white ring-2 ring-[#a78bfa]/40 hover:bg-[#1a1a1f] hover:text-white',
                    )}
                  >
                    {f === 'cpa' ? 'CPA 格式' : 'SUB 格式'}
                  </Button>
                )
              })}
            </div>

            <Button type="submit" loading={loading} className={cn(softBtnClass, 'text-white/70 hover:text-white/85')}>
              {loading ? '处理中...' : '提取'}
            </Button>
          </form>
        </GlowCard>
      </div>
    </div>
  )
}
