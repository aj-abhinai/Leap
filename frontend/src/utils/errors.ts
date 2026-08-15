// Coerce an unknown rejection into a displayable message. Use in catch
// blocks instead of `e.message` on `any`-typed errors, which throws a second
// error when the rejection is not an Error instance.
export function errorMessage(e: unknown, fallback = 'Something went wrong'): string {
  if (e instanceof Error && e.message) return e.message
  if (typeof e === 'string' && e) return e
  if (e && typeof e === 'object' && 'message' in e && typeof (e as { message: unknown }).message === 'string' && (e as { message: string }).message) {
    return (e as { message: string }).message
  }
  return fallback
}
