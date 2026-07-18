import { useEffect, useState } from 'react'
import { api, downloadBlob } from '@/lib/api'
import { Button, Textarea, Toast } from '@/components/ui'
import { cn } from '@/lib/utils'
import { useToast } from '@/hooks/useToast'
import { DarkVeil } from '@/components/bits/DarkVeil'
import { ShinyText } from '@/components/bits/ShinyText'
import { ElectricBorder } from '@/components/bits/ElectricBorder'
import { ClickSpark } from '@/components/bits/ClickSpark'
import { BorderGlow } from '@/components/bits/BorderGlow'
import { CountUp } from '@/components/bits/CountUp'

const fieldClass =
  'min-h-48 resize-none rounded-2xl border border-white/[0.08] bg-[#0b0b0d] px-4 py-3 text-sm text-white/90 shadow-none placeholder:text-white/35 transition-colors focus-visible:border-white/[0.08] focus-visible:outline-none focus-visible:ring-0'

const softBtnClass =
  'h-12 w-full cursor-pointer rounded-2xl border border-white/[0.08] bg-[#17171a] text-sm font-medium text-white/55 shadow-none transition-colors hover:bg-[#1d1d22] hover:text-white/75'

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

  return (
    <ClickSpark sparkColor="#FFFFFF" sparkSize={12} sparkRadius={18} sparkCount={10} duration={450} className="min-h-screen">
      <div className="relative min-h-screen overflow-hidden bg-[#120F17]">
      <div className="pointer-events-none absolute inset-0">
        <DarkVeil hueShift={35} noiseIntensity={0.035} scanlineIntensity={0.06} scanlineFrequency={2.2} speed={2} warpAmount={0.18} />
      </div>
      {toast ? <Toast message={toast.message} type={toast.type} /> : null}

      <div className="relative z-10 mx-auto flex min-h-screen max-w-xl flex-col justify-center px-6 py-12">
        <header className="mb-8 text-center">
          <div className="mb-3 text-xs tracking-[0.35em] text-[#a1a1aa] uppercase">redeem portal</div>
          <div className="flex justify-center">
             <ShinyText
               text="whistlelads"
               speed={0.75}
               color="#6B7280"
               shineColor="#FFFFFF"
               spread={90}
               direction="left"
               yoyo
               delay={0.1}
               className="text-[clamp(2.5rem,10vw,5rem)] font-black tracking-tight"
             />
          </div>
            <div className="mt-4 flex flex-wrap items-baseline justify-center gap-x-3 gap-y-2 px-1 text-base md:text-lg">
              <span className="text-sm text-white/55">库存</span>
              {spaces.length > 0 ? (
               <div className="flex flex-wrap items-baseline justify-center gap-x-3 gap-y-1 text-sm">
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
               <span className="text-sm text-white/40">暂无空间</span>
             )}
           </div>
         </header>

        <BorderGlow
          backgroundColor="#121214"
          borderRadius={28}
          edgeSensitivity={20}
          glowColor="0 0 96"
          glowRadius={40}
          glowIntensity={1.1}
          coneSpread={28}
          colors={['#FFFFFF', '#E4E4E7', '#A1A1AA']}
          fillOpacity={0.35}
        >
          <form
            className="space-y-6 p-6"
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
                 placeholder="输入卡密，一键提取"
                required
                className={fieldClass}
              />
            </div>

            <div className="grid grid-cols-2 gap-3">
              {(['cpa', 'sub'] as const).map((f) => {
                const active = format === f
                const btn = (
                  <Button
                    type="button"
                    variant="outline"
                    onClick={() => setFormat(f)}
                    className={cn(
                      softBtnClass,
                      active
                        ? 'border-transparent bg-[#1a1a1f] text-white hover:bg-[#1a1a1f] hover:text-white'
                        : 'text-white/55',
                    )}
                  >
                    {f === 'cpa' ? 'CPA 格式' : 'SUB 格式'}
                  </Button>
                )
                return active ? (
                   <ElectricBorder key={f} color="#E9D5FF" speed={1} chaos={0.02} borderRadius={16} className="w-full">
                    {btn}
                  </ElectricBorder>
                ) : (
                  <div key={f}>{btn}</div>
                )
              })}
            </div>

             <ElectricBorder color="#E9D5FF" speed={1} chaos={0.02} borderRadius={16} className="w-full">
              <Button
                type="submit"
                loading={loading}
                className={cn(softBtnClass, 'border-transparent text-white/70 hover:text-white/85')}
              >
                {loading ? '处理中...' : '提取'}
              </Button>
            </ElectricBorder>
          </form>
        </BorderGlow>
      </div>
      </div>
    </ClickSpark>
  )
}
