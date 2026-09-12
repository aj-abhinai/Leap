// debounce returns a function that defers fn until ms have passed since the
// last call; each call cancels the previous pending invocation. The returned
// function exposes cancel() so callers can drop a pending call on unmount.
export function debounce<A extends unknown[]>(
  fn: (...args: A) => void,
  ms: number,
): ((...args: A) => void) & { cancel: () => void } {
  let timer: ReturnType<typeof setTimeout> | undefined
  const wrapped = (...args: A) => {
    clearTimeout(timer)
    timer = setTimeout(() => fn(...args), ms)
  }
  wrapped.cancel = () => clearTimeout(timer)
  return wrapped
}