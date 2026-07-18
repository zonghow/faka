export type Space = {
  id: number
  name: string
  card_prefix: string
  created_at?: string
  updated_at?: string
}

export type Card = {
  id: number
  code: string
  file_count: number
  status: string
  created_at: string
  used_at: string
  voided_at: string
}

export type ManagedFile = {
  id: number
  original_name: string
  status: string
  sold_at: string
  sold_card: string
  voided_at: string
  uploaded_at: string
}

export type Pagination = {
  page: number
  page_size: number
  total: number
  total_pages: number
  has_prev: boolean
  has_next: boolean
  prev_page: number
  next_page: number
}

async function request<T>(url: string, init?: RequestInit): Promise<T> {
  const res = await fetch(url, {
    credentials: 'include',
    headers: {
      Accept: 'application/json',
      ...(init?.body instanceof FormData ? {} : { 'Content-Type': 'application/json' }),
      ...(init?.headers || {}),
    },
    ...init,
  })
  const contentType = res.headers.get('content-type') || ''
  if (contentType.includes('application/json')) {
    const data = await res.json()
    if (!res.ok || data.ok === false) {
      throw new Error(data.error || data.message || '请求失败')
    }
    return data as T
  }
  if (!res.ok) {
    throw new Error('请求失败')
  }
  return undefined as T
}

export const api = {
  me: () => request<{ ok: boolean; authenticated: boolean }>('/api/auth/me'),
  login: (password: string) =>
    request('/api/auth/login', { method: 'POST', body: JSON.stringify({ password }) }),
  logout: () => request('/api/auth/logout', { method: 'POST', body: '{}' }),
  inventory: () =>
    request<{
      inventory: number
      spaces: Array<{ id: number; name: string; card_prefix: string; inventory: number }>
    }>('/api/inventory'),
  redeem: async (cardCode: string, outputFormat: 'cpa' | 'sub') => {
    const form = new FormData()
    form.append('card_code', cardCode)
    form.append('output_format', outputFormat)
    const res = await fetch('/api/redeem', { method: 'POST', body: form, credentials: 'include' })
    if (!res.ok) {
      const data = await res.json().catch(() => ({}))
      throw new Error(data.error || '提取失败')
    }
    const blob = await res.blob()
    const disposition = res.headers.get('content-disposition') || ''
    const match = disposition.match(/filename="?([^"]+)"?/i)
    const filename = match ? decodeURIComponent(match[1]) : outputFormat === 'sub' ? 'export.json' : 'export.zip'
    return { blob, filename }
  },
  dashboard: () =>
    request<{ ok: boolean; current_space: Space; stats: Record<string, number> }>('/api/admin/dashboard'),
  clearSpace: () => request<{ ok: boolean; message: string; counts: Record<string, number> }>('/api/admin/clear', { method: 'POST', body: '{}' }),
  spaces: () => request<{ ok: boolean; spaces: Space[]; current_space: Space }>('/api/admin/spaces'),
  createSpace: (name: string, card_prefix: string) =>
    request('/api/admin/spaces', { method: 'POST', body: JSON.stringify({ name, card_prefix }) }),
  updateSpace: (id: number, name: string, card_prefix: string) =>
    request(`/api/admin/spaces/${id}`, { method: 'POST', body: JSON.stringify({ name, card_prefix }) }),
  deleteSpace: (id: number) => request(`/api/admin/spaces/${id}`, { method: 'DELETE' }),
  cards: (params: URLSearchParams) =>
    request<{ ok: boolean; cards: Card[]; pagination: Pagination; current_space: Space }>(`/api/admin/cards?${params}`),
  createCards: (file_count: number, quantity: number) =>
    request<{ ok: boolean; message: string; count: number; ids: number[]; codes: string[] }>(
      '/api/admin/cards',
      { method: 'POST', body: JSON.stringify({ file_count, quantity }) },
    ),
  updateCardStatus: (ids: number[], target_status: string) =>
    request('/api/admin/cards/status', { method: 'POST', body: JSON.stringify({ ids, target_status }) }),
  downloadCards: async (ids: number[], mark_pending: boolean) => {
    const res = await fetch('/api/admin/cards/download', {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json', Accept: '*/*' },
      body: JSON.stringify({ ids, mark_pending }),
    })
    if (!res.ok) {
      const data = await res.json().catch(() => ({}))
      throw new Error(data.error || '下载失败')
    }
    return { blob: await res.blob(), filename: 'cards.txt' }
  },
  cardRedemptions: (id: number) =>
    request<{ card_code: string; first_used_at: string; redemptions: Array<Record<string, unknown>> }>(
      `/api/admin/cards/${id}/redemptions`,
    ),
  files: (params: URLSearchParams) =>
    request<{ ok: boolean; files: ManagedFile[]; pagination: Pagination; current_space: Space }>(`/api/admin/files?${params}`),
  uploadFiles: async (files: FileList | File[]) => {
    const form = new FormData()
    Array.from(files).forEach((f) => form.append('file', f))
    return request<{ ok: boolean; message: string; imported: number; created: number; duplicated: number }>('/api/admin/files/upload', {
      method: 'POST',
      body: form,
    })
  },
  uploadFileWithProgress: (file: File, onProgress?: (percent: number) => void) =>
    new Promise<{ ok: boolean; message: string; imported: number; created: number; duplicated: number }>((resolve, reject) => {
      const xhr = new XMLHttpRequest()
      const form = new FormData()
      form.append('file', file)

      xhr.open('POST', '/api/admin/files/upload')
      xhr.withCredentials = true
      xhr.responseType = 'json'
      xhr.setRequestHeader('Accept', 'application/json')

      xhr.upload.onprogress = (event) => {
        if (!event.lengthComputable) return
        const percent = Math.max(0, Math.min(100, Math.round((event.loaded / event.total) * 100)))
        onProgress?.(percent)
      }

      xhr.onload = () => {
        const data = xhr.response || {}
        if (xhr.status >= 200 && xhr.status < 300 && data.ok !== false) {
          onProgress?.(100)
          const created = Number(data.created ?? data.imported ?? 0)
          const duplicated = Number(data.duplicated ?? 0)
          resolve({
            ok: true,
            message: data.message || `新增 ${created} 个，重复 ${duplicated} 个`,
            imported: created,
            created,
            duplicated,
          })
          return
        }
        reject(new Error(data.error || data.message || '上传失败'))
      }

      xhr.onerror = () => reject(new Error('网络异常，上传失败'))
      xhr.onabort = () => reject(new Error('上传已取消'))
      xhr.send(form)
    }),
  updateFileStatus: (ids: number[], target_status: string) =>
    request('/api/admin/files/status', { method: 'POST', body: JSON.stringify({ ids, target_status }) }),
  downloadFiles: async (ids: number[], mark_sold: boolean) => {
    const res = await fetch('/api/admin/files/download', {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json', Accept: '*/*' },
      body: JSON.stringify({ ids, mark_sold }),
    })
    if (!res.ok) {
      const data = await res.json().catch(() => ({}))
      throw new Error(data.error || '下载失败')
    }
    const disposition = res.headers.get('content-disposition') || ''
    const match = disposition.match(/filename="?([^"]+)"?/i)
    return { blob: await res.blob(), filename: match ? decodeURIComponent(match[1]) : 'files.zip' }
  },
}

export function setSpaceCookie(id: number) {
  document.cookie = `tikawang_space_id=${id}; path=/; max-age=31536000; samesite=lax`
}

export function downloadBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.click()
  URL.revokeObjectURL(url)
}
