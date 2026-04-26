import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, cleanup, act, fireEvent, screen } from '@testing-library/react'
import type { Entry } from '@/types/api'

const {
  mockRenderedEntryListItem,
  mockNeedsTranslation,
  mockTranslateArticlesBatch,
  mockCancelAllBatchTranslations,
  mockTranslationActionsGet,
} = vi.hoisted(() => ({
  mockRenderedEntryListItem: vi.fn(),
  mockNeedsTranslation: vi.fn(() => Promise.resolve(true)),
  mockTranslateArticlesBatch: vi.fn(() => Promise.resolve()),
  mockCancelAllBatchTranslations: vi.fn(),
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  mockTranslationActionsGet: vi.fn((): any => undefined),
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

vi.mock('@/hooks/useEntries', () => ({
  useEntriesInfinite: vi.fn(),
  useUnreadCounts: vi.fn(() => ({ data: undefined })),
}))

vi.mock('@/hooks/useFeeds', () => ({
  useFeeds: vi.fn(() => ({ data: [] })),
}))

vi.mock('@/hooks/useFolders', () => ({
  useFolders: vi.fn(() => ({ data: [] })),
}))

vi.mock('@/hooks/useAISettings', () => ({
  useAISettings: vi.fn(),
}))

vi.mock('@/hooks/useSelection', () => ({
  selectionToParams: vi.fn(() => ({})),
}))

vi.mock('@/lib/html-utils', () => ({
  stripHtml: (html: string) => html,
}))

vi.mock('@/lib/language-detect-async', () => ({
  needsTranslation: mockNeedsTranslation,
}))

vi.mock('@/services/translation-service', () => ({
  translateArticlesBatch: mockTranslateArticlesBatch,
  cancelAllBatchTranslations: mockCancelAllBatchTranslations,
}))

vi.mock('@/stores/translation-store', () => ({
  translationActions: {
    get: mockTranslationActionsGet,
    isDisabled: () => false,
  },
}))

vi.mock('./EntryListItem', async () => {
  const { forwardRef } = await import('react')

  return {
    EntryListItem: forwardRef<HTMLDivElement, { entry: Entry; 'data-index'?: number }>(
      function MockEntryListItem({ entry, 'data-index': dataIndex }, ref) {
        mockRenderedEntryListItem(entry.id)
        return <div ref={ref} data-index={dataIndex} />
      }
    ),
  }
})

vi.mock('./EntryListHeader', () => ({
  EntryListHeader: () => null,
}))

import { EntryList } from './EntryList'
import { useEntriesInfinite } from '@/hooks/useEntries'
import { useAISettings } from '@/hooks/useAISettings'

function makeEntry(id: string): Entry {
  return {
    id,
    feedId: 'feed-1',
    title: `Title ${id}`,
    content: `<p>Content for entry ${id}</p>`,
    read: false,
    starred: false,
    createdAt: '2024-01-01T00:00:00Z',
    updatedAt: '2024-01-01T00:00:00Z',
  }
}

const allEntries = ['1', '2', '3', '4', '5'].map(makeEntry)

const defaultProps = {
  selection: { type: 'all' as const },
  selectedEntryId: null as string | null,
  onSelectEntry: vi.fn(),
  onMarkAllRead: vi.fn(),
  unreadOnly: false,
  onToggleUnreadOnly: vi.fn(),
  contentType: 'article' as const,
}

async function flushTranslationScheduling() {
  await act(async () => {
    await Promise.resolve()
    await Promise.resolve()
  })
}

async function runBatchTimer() {
  await flushTranslationScheduling()
  await act(async () => {
    vi.advanceTimersByTime(500)
  })
}

describe('EntryList translation scheduling', () => {
  let originalIntersectionObserver: typeof IntersectionObserver | undefined

  beforeEach(() => {
    vi.useFakeTimers()
    vi.clearAllMocks()

    mockRenderedEntryListItem.mockReset()
    mockNeedsTranslation.mockResolvedValue(true)
    mockTranslationActionsGet.mockReturnValue(undefined)

    originalIntersectionObserver = globalThis.IntersectionObserver
    globalThis.IntersectionObserver = class {
      private callback: IntersectionObserverCallback

      constructor(callback: IntersectionObserverCallback) {
        this.callback = callback
      }

      observe = (target: Element) => {
        const index = Number((target as HTMLElement).dataset.index)
        if (index <= 1) {
          this.callback(
            [{ isIntersecting: true, target } as IntersectionObserverEntry],
            this as unknown as IntersectionObserver
          )
        }
      }

      disconnect = vi.fn()
      unobserve = vi.fn()
      takeRecords = () => []
      root = null
      rootMargin = ''
      thresholds = []
    } as unknown as typeof IntersectionObserver

    vi.mocked(useEntriesInfinite).mockReturnValue({
      data: { pages: [{ entries: allEntries, hasMore: false }] },
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isFetchingNextPage: false,
      isLoading: false,
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any)

    vi.mocked(useAISettings).mockReturnValue({
      data: { autoTranslate: true, summaryLanguage: 'zh-CN' },
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any)
  })

  afterEach(() => {
    cleanup()
    globalThis.IntersectionObserver = originalIntersectionObserver!
    vi.useRealTimers()
  })

  it('会为可视区外的选中文章安排翻译', async () => {
    render(<EntryList {...defaultProps} selectedEntryId="4" />)

    await runBatchTimer()

    expect(mockTranslateArticlesBatch).toHaveBeenCalled()
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const calledArticles = (mockTranslateArticlesBatch.mock.calls[0] as any[])[0] as Array<{ id: string }>
    const ids = calledArticles.map((article) => article.id)
    expect(ids).toContain('4')
    expect(ids).toContain('1')
    expect(ids).toContain('2')
  })

  it('选中文章已在可视区时不会重复安排', async () => {
    render(<EntryList {...defaultProps} selectedEntryId="1" />)

    await runBatchTimer()

    expect(mockTranslateArticlesBatch).toHaveBeenCalled()
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const calledArticles = (mockTranslateArticlesBatch.mock.calls[0] as any[])[0] as Array<{ id: string }>
    const ids = calledArticles.map((article) => article.id)
    expect(ids.filter((id) => id === '1')).toHaveLength(1)
  })

  it('没有选中文章时只翻译可视项', async () => {
    render(<EntryList {...defaultProps} selectedEntryId={null} />)

    await runBatchTimer()

    expect(mockTranslateArticlesBatch).toHaveBeenCalled()
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const calledArticles = (mockTranslateArticlesBatch.mock.calls[0] as any[])[0] as Array<{ id: string }>
    const ids = calledArticles.map((article) => article.id)
    expect(ids).toEqual(expect.arrayContaining(['1', '2']))
    expect(ids).not.toContain('3')
    expect(ids).not.toContain('4')
    expect(ids).not.toContain('5')
  })

  it('自动翻译关闭时不会发起批量翻译', () => {
    vi.mocked(useAISettings).mockReturnValue({
      data: { autoTranslate: false, summaryLanguage: 'zh-CN' },
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any)

    render(<EntryList {...defaultProps} selectedEntryId="4" />)
    vi.advanceTimersByTime(500)

    expect(mockTranslateArticlesBatch).not.toHaveBeenCalled()
  })

  it('批量翻译失败后会允许下次重试', async () => {
    mockTranslationActionsGet.mockReturnValue(undefined)

    const { rerender } = render(<EntryList {...defaultProps} selectedEntryId={null} />)

    await runBatchTimer()

    expect(mockTranslateArticlesBatch).toHaveBeenCalledTimes(1)
    mockTranslateArticlesBatch.mockClear()

    rerender(<EntryList {...defaultProps} selectedEntryId="3" />)

    await runBatchTimer()

    expect(mockTranslateArticlesBatch).toHaveBeenCalledTimes(1)
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const calledArticles = (mockTranslateArticlesBatch.mock.calls[0] as any[])[0] as Array<{ id: string }>
    const ids = calledArticles.map((article) => article.id)
    expect(ids).toContain('1')
    expect(ids).toContain('2')
    expect(ids).toContain('3')
  })

  it('已成功翻译的文章不会重复进入批量队列', async () => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    ;(mockTranslationActionsGet as any).mockImplementation((id: string) => {
      if (id === '1' || id === '2') {
        return { title: 'translated', summary: 'translated summary', content: null }
      }
      return undefined
    })

    const { rerender } = render(<EntryList {...defaultProps} selectedEntryId={null} />)

    await runBatchTimer()

    expect(mockTranslateArticlesBatch).toHaveBeenCalledTimes(1)
    mockTranslateArticlesBatch.mockClear()

    rerender(<EntryList {...defaultProps} selectedEntryId="3" />)

    await runBatchTimer()

    expect(mockTranslateArticlesBatch).toHaveBeenCalledTimes(1)
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const calledArticles = (mockTranslateArticlesBatch.mock.calls[0] as any[])[0] as Array<{ id: string }>
    const ids = calledArticles.map((article) => article.id)
    expect(ids).not.toContain('1')
    expect(ids).not.toContain('2')
    expect(ids).toContain('3')
  })

  it('分页有重复文章时只会安排一次翻译', async () => {
    vi.mocked(useEntriesInfinite).mockReturnValue({
      data: {
        pages: [
          { entries: [makeEntry('1'), makeEntry('2')], hasMore: true },
          { entries: [makeEntry('2'), makeEntry('3')], hasMore: false },
        ],
      },
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isFetchingNextPage: false,
      isLoading: false,
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any)

    render(<EntryList {...defaultProps} selectedEntryId={null} />)

    await runBatchTimer()

    expect(mockTranslateArticlesBatch).toHaveBeenCalledTimes(1)
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const calledArticles = (mockTranslateArticlesBatch.mock.calls[0] as any[])[0] as Array<{ id: string }>
    expect(calledArticles.map((article) => article.id)).toEqual(['1', '2'])
  })

  it('滚动接近底部时加载下一页', () => {
    const fetchNextPage = vi.fn()
    vi.mocked(useEntriesInfinite).mockReturnValue({
      data: { pages: [{ entries: allEntries, hasMore: true }] },
      fetchNextPage,
      hasNextPage: true,
      isFetchingNextPage: false,
      isLoading: false,
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any)

    render(<EntryList {...defaultProps} />)

    const viewport = screen.getByTestId('entry-list-viewport')
    Object.defineProperties(viewport, {
      scrollHeight: { configurable: true, value: 2000 },
      clientHeight: { configurable: true, value: 1000 },
      scrollTop: { configurable: true, value: 500 },
    })

    fireEvent.scroll(viewport)

    expect(fetchNextPage).toHaveBeenCalled()
  })

  it('列表滚动容器使用系统原生滚动条', () => {
    render(<EntryList {...defaultProps} />)

    const viewport = screen.getByTestId('entry-list-viewport')
    expect(viewport.className).toContain('overflow-y-auto')
    expect(viewport.className).toContain('overflow-x-hidden')
  })

  it('渲染列表时也会去重重复文章', () => {
    vi.mocked(useEntriesInfinite).mockReturnValue({
      data: {
        pages: [
          { entries: [makeEntry('1'), makeEntry('2')], hasMore: true },
          { entries: [makeEntry('2'), makeEntry('3')], hasMore: false },
        ],
      },
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isFetchingNextPage: false,
      isLoading: false,
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any)

    render(<EntryList {...defaultProps} selectedEntryId={null} />)

    expect(mockRenderedEntryListItem.mock.calls.map(([id]) => id)).toEqual(['1', '2', '3'])
  })
})
