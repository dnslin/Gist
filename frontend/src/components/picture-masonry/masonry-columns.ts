export function buildMasonryColumns<T>(items: T[], columnCount: number): T[][] {
  const safeColumnCount = Math.max(Math.floor(columnCount), 1)
  const columns = Array.from({ length: safeColumnCount }, () => [] as T[])

  for (let index = 0; index < items.length; index += 1) {
    columns[index % safeColumnCount]!.push(items[index]!)
  }

  return columns
}
