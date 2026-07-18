import { useEffect } from 'react'
import { useLocation } from 'react-router-dom'

const titles: Record<string, string> = {
  '/': '卡密提取 | Whistlelads Faka',
  '/admin/login': '登录 | 后台管理',
  '/admin': '数据概览 | 后台管理',
  '/admin/spaces': '空间设置 | 后台管理',
  '/admin/cards': '卡密管理 | 后台管理',
  '/admin/files': '文件管理 | 后台管理',
  '/admin/uploads': '上传文件 | 后台管理',
}

export function DocumentTitle() {
  const { pathname } = useLocation()

  useEffect(() => {
    if (titles[pathname]) {
      document.title = titles[pathname]
      return
    }
    if (pathname.startsWith('/admin')) {
      document.title = '后台管理 | Whistlelads Faka'
      return
    }
    document.title = 'Whistlelads Faka'
  }, [pathname])

  return null
}
