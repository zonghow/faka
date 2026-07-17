import { useRef } from 'react'
import { useVirtualizer } from '@tanstack/react-virtual'

const ROW_HEIGHT = 48
const OVERSCAN = 12

export function useTableVirtualizer(count: number) {
  const parentRef = useRef<HTMLDivElement>(null)
  const virtualizer = useVirtualizer({
    count,
    getScrollElement: () => parentRef.current,
    estimateSize: () => ROW_HEIGHT,
    overscan: OVERSCAN,
  })

  const items = virtualizer.getVirtualItems()
  const totalSize = virtualizer.getTotalSize()
  const paddingTop = items.length > 0 ? items[0].start : 0
  const paddingBottom = items.length > 0 ? totalSize - items[items.length - 1].end : 0

  return {
    parentRef,
    virtualizer,
    items,
    paddingTop,
    paddingBottom,
    rowHeight: ROW_HEIGHT,
  }
}
