// A request guard discards stale async responses: only the newest request's
// result may land. The guard hides the sequence counter; callers ask one
// question per await point: is this response still current?
export interface RequestToken {
  isCurrent(): boolean
}

// createRequestGuard creates one guard per data surface.
export function createRequestGuard() {
  let seq = 0
  const token = (s: number): RequestToken => ({ isCurrent: () => s === seq })
  return {
    // start begins a new request and invalidates every earlier token.
    start(): RequestToken {
      seq += 1
      return token(seq)
    },
    // current adopts the latest request without invalidating it: a follow-up
    // write (post-save refetch, load-more append) is discarded only when a
    // newer request started meanwhile.
    current(): RequestToken {
      return token(seq)
    },
  }
}
