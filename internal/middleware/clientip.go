package middleware

import (
	"crm/internal/ctxutil"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// ClientIP resolves the client IP for downstream consumers (rate limiting,
// logging). Forwarded headers are honored only when every preceding hop is an
// explicitly trusted proxy prefix; otherwise the socket peer is used. A
// request whose connection does not come from a trusted proxy can therefore
// never influence its own identity with spoofed headers.
func ClientIP(trustedProxies []string) func(http.Handler) http.Handler {
	prefixes := make([]netip.Prefix, 0, len(trustedProxies))
	for _, p := range trustedProxies {
		if prefix, err := netip.ParsePrefix(strings.TrimSpace(p)); err == nil {
			prefixes = append(prefixes, prefix)
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := resolveClientIP(r, prefixes)
			if ip != "" {
				r = r.WithContext(ctxutil.WithClientIP(r.Context(), ip))
			}
			next.ServeHTTP(w, r)
		})
	}
}

// resolveClientIP returns the client IP as a string. When no trusted proxy is
// configured, the socket peer is authoritative. When trusted prefixes are
// configured, the X-Forwarded-For chain is walked right-to-left (the value set
// by the hop closest to us first) skipping trusted addresses; the first
// untrusted address is the client. Any unparseable entry aborts the walk
// (fail closed) and falls back to the socket peer.
func resolveClientIP(r *http.Request, prefixes []netip.Prefix) string {
	socket := socketPeer(r.RemoteAddr)
	if len(prefixes) == 0 {
		return socket
	}
	fromTrusted := socketIPInPrefixes(r.RemoteAddr, prefixes)
	if !fromTrusted {
		return socket
	}
	entries := forwardedChain(r)
	for i := len(entries) - 1; i >= 0; i-- {
		ip, err := netip.ParseAddr(strings.TrimSpace(entries[i]))
		if err != nil {
			return socket
		}
		ip = ip.Unmap()
		if !ipInPrefixes(ip, prefixes) {
			return ip.String()
		}
	}
	return socket
}

// forwardedChain returns the merged X-Forwarded-For entries in order. Per
// RFC 2616, multiple headers are concatenated with commas, so an attacker
// cannot pick which value security logic sees by sending duplicate headers.
func forwardedChain(r *http.Request) []string {
	return strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
}

func socketPeer(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return ""
	}
	return ip.Unmap().String()
}

func socketIPInPrefixes(remoteAddr string, prefixes []netip.Prefix) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	return ipInPrefixes(ip.Unmap(), prefixes)
}

func ipInPrefixes(ip netip.Addr, prefixes []netip.Prefix) bool {
	for _, p := range prefixes {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}
