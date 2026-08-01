import { describe, it, expect } from 'vitest'
import { parseCSV } from '@/utils/csv'

describe('parseCSV', () => {
  it('parses simple rows and normalizes headers', () => {
    const { headers, rows } = parseCSV('Name,Email\nAlice,alice@example.com\nBob,bob@example.com')
    expect(headers).toEqual(['name', 'email'])
    expect(rows).toEqual([
      ['Alice', 'alice@example.com'],
      ['Bob', 'bob@example.com'],
    ])
  })

  it('keeps commas inside quoted fields', () => {
    const { rows } = parseCSV('name,location\n"Acme, Inc.","Pune, India"')
    expect(rows).toEqual([['Acme, Inc.', 'Pune, India']])
  })

  it('keeps newlines inside quoted fields', () => {
    const { rows } = parseCSV('name,note\nAlice,"line one\nline two"')
    expect(rows).toEqual([['Alice', 'line one\nline two']])
  })

  it('unpacks doubled quotes into one quote', () => {
    const { rows } = parseCSV('name,note\nAlice,"said ""hi"""')
    expect(rows).toEqual([['Alice', 'said "hi"']])
  })

  it('normalizes CRLF and CR line endings', () => {
    const crlf = parseCSV('name,email\r\nAlice,a@b.c\r\nBob,b@b.c\r\n')
    expect(crlf.rows).toEqual([
      ['Alice', 'a@b.c'],
      ['Bob', 'b@b.c'],
    ])
    const cr = parseCSV('name,email\rAlice,a@b.c\rBob,b@b.c')
    expect(cr.rows).toEqual([
      ['Alice', 'a@b.c'],
      ['Bob', 'b@b.c'],
    ])
  })

  it('strips a leading BOM', () => {
    const { headers } = parseCSV('\uFEFFName,Email\nAlice,a@b.c')
    expect(headers).toEqual(['name', 'email'])
  })

  it('ignores trailing blank records', () => {
    const { rows } = parseCSV('name,email\nAlice,a@b.c\n\n')
    expect(rows).toEqual([['Alice', 'a@b.c']])
  })

  it('returns empty output for blank input', () => {
    expect(parseCSV('')).toEqual({ headers: [], rows: [] })
    expect(parseCSV('\n\n')).toEqual({ headers: [], rows: [] })
  })

  it('trims header names', () => {
    const { headers } = parseCSV('  Name ,  Email \nAlice,a@b.c')
    expect(headers).toEqual(['name', 'email'])
  })
})
