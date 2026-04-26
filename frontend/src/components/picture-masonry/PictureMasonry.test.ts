import { describe, expect, it } from 'vitest'
import { buildMasonryColumns } from './masonry-columns'

describe('buildMasonryColumns', () => {
  it('distributes items so visual rows preserve source order', () => {
    const columns = buildMasonryColumns(['1', '2', '3', '4', '5', '6', '7'], 3)

    expect(columns).toEqual([
      ['1', '4', '7'],
      ['2', '5'],
      ['3', '6'],
    ])
    expect(columns.map((column) => column[0])).toEqual(['1', '2', '3'])
    expect(columns.map((column) => column[1]).filter(Boolean)).toEqual(['4', '5', '6'])
  })

  it('uses at least one column', () => {
    expect(buildMasonryColumns(['1', '2'], 0)).toEqual([['1', '2']])
  })
})
