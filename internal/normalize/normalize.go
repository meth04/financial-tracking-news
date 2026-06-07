package normalize

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

var (
	spaceRE      = regexp.MustCompile(`\s+`)
	punctRE      = regexp.MustCompile(`[!?.]{2,}`)
	suffixRE     = regexp.MustCompile(`(?i)\s+[|\-—]\s+(reuters|cnbc|bloomberg|ap news|marketwatch|wsj)$`)
	trackingKeys = map[string]bool{"fbclid": true, "gclid": true, "mc_cid": true, "mc_eid": true, "cmpid": true, "source": true, "ref": true}
)

func CanonicalURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	u.Fragment = ""
	q := u.Query()
	for k := range q {
		lk := strings.ToLower(k)
		if strings.HasPrefix(lk, "utm_") || trackingKeys[lk] {
			q.Del(k)
		}
	}
	u.RawQuery = q.Encode()
	u.Path = strings.TrimRight(u.EscapedPath(), "/")
	if u.Path == "" {
		u.Path = "/"
	}
	return u.String()
}

func NormalizeTitle(title string) string {
	t := strings.ToLower(strings.TrimSpace(title))
	t = suffixRE.ReplaceAllString(t, "")
	t = punctRE.ReplaceAllString(t, ".")
	t = spaceRE.ReplaceAllString(t, " ")
	return strings.Trim(t, " \t\n\r-—|")
}

func CleanText(s string) string {
	s = strings.ReplaceAll(s, " ", " ")
	lines := strings.Split(s, "\n")
	keep := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		low := strings.ToLower(line)
		if line == "" || strings.Contains(low, "cookie") || strings.Contains(low, "subscribe") || strings.Contains(low, "share this") {
			continue
		}
		keep = append(keep, line)
	}
	return spaceRE.ReplaceAllString(strings.Join(keep, " "), " ")
}

func SHA256Hex(s string) string     { sum := sha256.Sum256([]byte(s)); return hex.EncodeToString(sum[:]) }
func TitleHash(title string) string { return SHA256Hex(NormalizeTitle(title)) }
func ContentHash(text string) string {
	text = CleanText(text)
	if WordCount(text) < 20 {
		return ""
	}
	return SHA256Hex(text)
}
func RawHash(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }

func WordCount(s string) int { return len(strings.Fields(s)) }

func Tokens(s string) []string {
	s = strings.ToLower(s)
	buf := strings.Builder{}
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			buf.WriteRune(r)
		} else {
			buf.WriteByte(' ')
		}
	}
	parts := strings.Fields(buf.String())
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if len(p) > 1 {
			out = append(out, p)
		}
	}
	return out
}

func SimHash(text string) uint64 {
	toks := Tokens(text)
	if len(toks) == 0 {
		return 0
	}
	weights := make([]int, 64)
	for _, tok := range toks {
		h := sha256.Sum256([]byte(tok))
		var v uint64
		for i := 0; i < 8; i++ {
			v = (v << 8) | uint64(h[i])
		}
		for i := 0; i < 64; i++ {
			if (v>>uint(i))&1 == 1 {
				weights[i]++
			} else {
				weights[i]--
			}
		}
	}
	var out uint64
	for i := 0; i < 64; i++ {
		if weights[i] > 0 {
			out |= 1 << uint(i)
		}
	}
	return out
}

func Hamming(a, b uint64) int {
	x := a ^ b
	n := 0
	for x > 0 {
		n++
		x &= x - 1
	}
	return n
}

func JaccardShingles(a, b string, n int) float64 {
	as, bs := shingles(a, n), shingles(b, n)
	if len(as) == 0 || len(bs) == 0 {
		return 0
	}
	inter := 0
	for k := range as {
		if bs[k] {
			inter++
		}
	}
	union := len(as) + len(bs) - inter
	return float64(inter) / float64(union)
}

func shingles(s string, n int) map[string]bool {
	toks := Tokens(s)
	out := map[string]bool{}
	if len(toks) < n {
		sort.Strings(toks)
		if len(toks) > 0 {
			out[strings.Join(toks, " ")] = true
		}
		return out
	}
	for i := 0; i <= len(toks)-n; i++ {
		out[strings.Join(toks[i:i+n], " ")] = true
	}
	return out
}

func Similarity(a, b string) float64 { return JaccardShingles(a, b, 3) }
