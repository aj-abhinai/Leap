export function parseCSV(text: string): { headers: string[]; rows: string[][] } {
  const clean = text.replace(/^\uFEFF/, '')
  const lines = clean.trim().split('\n').map(l => l.trim()).filter(l => l)
  if (lines.length === 0) return { headers: [], rows: [] }
  const h = lines[0].split(',').map(hh => hh.trim().toLowerCase())
  const r: string[][] = []
  for (let i = 1; i < lines.length; i++) {
    const line = lines[i]
    const result: string[] = []
    let current = ''
    let inQuotes = false
    for (const ch of line) {
      if (ch === '"') { inQuotes = !inQuotes }
      else if (ch === ',' && !inQuotes) { result.push(current.trim()); current = '' }
      else { current += ch }
    }
    result.push(current.trim())
    r.push(result)
  }
  return { headers: h, rows: r }
}
