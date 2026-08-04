export function formatCurrency(value?: number): string {
  if (!value) return ''
  return new Intl.NumberFormat('en-IN', { style: 'currency', currency: 'INR', maximumFractionDigits: 0 }).format(value)
}

// formatContactDetail joins the present phone/email values for list rows and
// cards, e.g. "+91 98765 43210 · alice@example.com".
export function formatContactDetail(phone?: string, email?: string): string {
  return [phone, email].filter(Boolean).join(' · ')
}
