export function formatCurrency(value?: number): string {
  if (!value) return ''
  return new Intl.NumberFormat('en-IN', { style: 'currency', currency: 'INR', maximumFractionDigits: 0 }).format(value)
}
