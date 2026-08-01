export function parseCSV(text: string): { headers: string[]; rows: string[][] } {
  const clean = text.replace(/^\uFEFF/, '').replace(/\r\n/g, '\n').replace(/\r/g, '\n')
  const records: string[][] = []
  let record: string[] = []
  let field = ''
  let inQuotes = false
  let i = 0

  while (i < clean.length) {
    const ch = clean[i]
    if (inQuotes) {
      if (ch === '"') {
        if (clean[i + 1] === '"') {
          field += '"'
          i += 2
          continue
        }
        inQuotes = false
        i++
        continue
      }
      field += ch
      i++
      continue
    }
    if (ch === '"' && field === '') {
      inQuotes = true
      i++
      continue
    }
    if (ch === ',') {
      record.push(field)
      field = ''
      i++
      continue
    }
    if (ch === '\n') {
      record.push(field)
      if (record.length > 1 || record[0] !== '') {
        records.push(record)
      }
      record = []
      field = ''
      i++
      continue
    }
    field += ch
    i++
  }
  record.push(field)
  if (record.length > 1 || record[0] !== '') {
    records.push(record)
  }

  if (records.length === 0) return { headers: [], rows: [] }
  const headers = records[0].map(h => h.trim().toLowerCase())
  const rows = records.slice(1).map(r => r.map(c => c.trim()))
  return { headers, rows }
}
