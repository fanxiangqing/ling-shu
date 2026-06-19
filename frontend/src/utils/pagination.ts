import type { PageResult } from '@/types/domain'

export const OPTION_PAGE_SIZE = 100

type PageLoader<T> = (params: { page: number; page_size: number }) => Promise<PageResult<T>>

export async function fetchAllPages<T>(loader: PageLoader<T>, pageSize = OPTION_PAGE_SIZE) {
  const items: T[] = []

  for (let page = 1; page <= 100; page += 1) {
    const result = await loader({ page, page_size: pageSize })
    const pageItems = Array.isArray(result.items) ? result.items : []
    items.push(...pageItems)

    const total = result.total || items.length
    const effectivePage = result.page || page
    const effectivePageSize = result.page_size || pageSize
    if (!pageItems.length || items.length >= total || effectivePage * effectivePageSize >= total) {
      break
    }
  }

  return items
}
