// Age display helpers for contacts. The exact birth date is the truth when
// present; the stored approximate integer is the fallback.

// computeAge returns the completed age in years for a date-only YYYY-MM-DD
// value, or null when the value is missing, malformed, or not a real calendar
// date (for example 2023-02-31).
export function computeAge(dateOfBirth?: string | null): number | null {
  if (!dateOfBirth) return null
  const parts = dateOfBirth.split('-')
  if (parts.length !== 3) return null
  const [y, m, d] = parts.map(Number)
  const dob = new Date(y, m - 1, d)
  if (dob.getFullYear() !== y || dob.getMonth() !== m - 1 || dob.getDate() !== d) return null

  const today = new Date()
  let age = today.getFullYear() - y
  const beforeBirthday = today.getMonth() < m - 1 || (today.getMonth() === m - 1 && today.getDate() < d)
  if (beforeBirthday) age--
  return age >= 0 ? age : null
}

// displayAge returns the computed age from the birth date when it is usable,
// else the stored approximate age, else null.
export function displayAge(dateOfBirth?: string | null, approximateAge?: number | null): number | null {
  const computed = computeAge(dateOfBirth)
  if (computed !== null) return computed
  return approximateAge ?? null
}
