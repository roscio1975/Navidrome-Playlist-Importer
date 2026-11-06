package main

import (
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// ---------------------------------------------------------------------------
// Normalization and variants (1:1 port of the original Python pipeline)
// ---------------------------------------------------------------------------

var (
	reBrackets  = regexp.MustCompile(`\(.*?\)|\[.*?\]`)
	reFeat      = regexp.MustCompile(`(?i)\b(feat|ft|featuring)\b.*$`)
	reTribute   = regexp.MustCompile(`(?i)\s*-\s*a tribute to .*$`)
	rePt        = regexp.MustCompile(`(?i)\bpt\.?\s*`)
	reApostr    = strings.NewReplacer("'", "", "\u2019", "", "`", "", "\u00b4", "")
	reNonAlnum  = regexp.MustCompile(`[^a-z0-9]+`)
	reSpaces    = regexp.MustCompile(`\s+`)
	rePrefixSep = regexp.MustCompile(`^(.*?)\s+-\s+(.+)$`)
	rePartRoman = regexp.MustCompile(`(?i)\bpart\s+(iv|iii|ii|i|v)\b`)
)

var romanToNumber = map[string]string{
	"i": "1", "ii": "2", "iii": "3", "iv": "4", "v": "5",
}

var tagNames = []string{"live", "reprise", "dub", "remix", "acoustic", "instrumental", "demo", "edit"}

var tagRegexps = func() map[string]*regexp.Regexp {
	m := make(map[string]*regexp.Regexp, len(tagNames))
	for _, t := range tagNames {
		m[t] = regexp.MustCompile(`\b` + t + `\b`)
	}
	return m
}()

// normalize reproduces:
//
//	NFKD -> strip diacritics -> lowercase -> & = " and "
//	-> remove apostrophes -> everything outside [a-z0-9] becomes a space
//	-> collapse spaces.
func normalize(s string) string {
	if s == "" {
		return ""
	}
	s = norm.NFKD.String(s)
	var b strings.Builder
	for _, r := range s {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		b.WriteRune(r)
	}
	s = strings.ToLower(b.String())
	s = strings.ReplaceAll(s, "&", " and ")
	s = reApostr.Replace(s)
	s = reNonAlnum.ReplaceAllString(s, " ")
	s = reSpaces.ReplaceAllString(strings.TrimSpace(s), " ")
	return strings.TrimSpace(s)
}

func stripFeat(x string) string { return reFeat.ReplaceAllString(x, "") }
func romanNumbers(x string) string {
	return rePartRoman.ReplaceAllStringFunc(x, func(m string) string {
		sub := rePartRoman.FindStringSubmatch(m)
		if len(sub) < 2 {
			return m
		}
		n, ok := romanToNumber[strings.ToLower(sub[1])]
		if !ok {
			return m
		}
		return "part " + n
	})
}

// variants builds the set of alternative title forms.
func variants(raw, album, artist string) []string {
	clean := func(x string) string { return strings.TrimSpace(reBrackets.ReplaceAllString(x, "")) }

	cands := map[string]bool{}
	add := func(x string) {
		if x != "" {
			cands[x] = true
		}
	}

	add(raw)
	add(clean(raw))
	add(strings.TrimSpace(stripFeat(raw)))
	add(strings.TrimSpace(reTribute.ReplaceAllString(raw, "")))
	add(rePt.ReplaceAllString(raw, "part "))

	// "<album> - " or "<artist> - " prefix (e.g. "Album - Title").
	if m := rePrefixSep.FindStringSubmatch(raw); m != nil {
		pre := normalize(m[1])
		if pre != "" {
			albumN := normalize(album)
			artistN := normalize(artist)
			if strings.Contains(albumN, pre) || strings.Contains(pre, albumN) || strings.Contains(artistN, pre) {
				add(m[2])
				add(clean(m[2]))
			}
		}
	}

	out := map[string]bool{}
	for x := range cands {
		for _, v := range []string{x, romanNumbers(x), stripFeat(x)} {
			if n := normalize(v); n != "" {
				out[n] = true
			}
		}
	}

	res := make([]string, 0, len(out))
	for v := range out {
		res = append(res, v)
	}
	sort.Strings(res)
	return res
}

func tagsOf(s string) map[string]bool {
	n := normalize(s)
	out := map[string]bool{}
	for _, t := range tagNames {
		if tagRegexps[t].MatchString(n) {
			out[t] = true
		}
	}
	return out
}

func tokensOf(s string) map[string]bool {
	out := map[string]bool{}
	for _, f := range strings.Fields(s) {
		out[f] = true
	}
	return out
}

func sameSet(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// Similarity: SequenceMatcher.ratio, equivalent to Python difflib
// ---------------------------------------------------------------------------

// ratio computes 2*M/T, where M is the number of characters in matching blocks.
func ratio(a, b string) float64 {
	ar := []rune(a)
	br := []rune(b)
	total := len(ar) + len(br)
	if total == 0 {
		return 1.0
	}
	return 2.0 * float64(matchingChars(ar, br)) / float64(total)
}

func matchingChars(a, b []rune) int {
	type block struct{ alo, ahi, blo, bhi int }
	queue := []block{{0, len(a), 0, len(b)}}
	total := 0
	for len(queue) > 0 {
		q := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		i, j, k := longestMatch(a, b, q.alo, q.ahi, q.blo, q.bhi)
		if k == 0 {
			continue
		}
		total += k
		if q.alo < i && q.blo < j {
			queue = append(queue, block{q.alo, i, q.blo, j})
		}
		if i+k < q.ahi && j+k < q.bhi {
			queue = append(queue, block{i + k, q.ahi, j + k, q.bhi})
		}
	}
	return total
}

func longestMatch(a, b []rune, alo, ahi, blo, bhi int) (besti, bestj, bestsize int) {
	besti, bestj = alo, blo
	n := bhi - blo
	if n <= 0 {
		return
	}
	prev := make([]int, n+1)
	curr := make([]int, n+1)
	for i := alo; i < ahi; i++ {
		for k := range curr {
			curr[k] = 0
		}
		ai := a[i]
		for j := blo; j < bhi; j++ {
			if ai != b[j] {
				continue
			}
			k := prev[j-blo] + 1
			curr[j-blo+1] = k
			if k > bestsize {
				besti = i - k + 1
				bestj = j - k + 1
				bestsize = k
			}
		}
		prev, curr = curr, prev
	}
	for besti > alo && bestj > blo && a[besti-1] == b[bestj-1] {
		besti, bestj, bestsize = besti-1, bestj-1, bestsize+1
	}
	for besti+bestsize < ahi && bestj+bestsize < bhi && a[besti+bestsize] == b[bestj+bestsize] {
		bestsize++
	}
	return
}

// tokenSet returns the unique tokens of s (like Python set.split()).
func tokenSet(s string) []string {
	f := strings.Fields(s)
	out := f[:0]
	for _, x := range f {
		dup := false
		for _, y := range out {
			if x == y {
				dup = true
				break
			}
		}
		if !dup {
			out = append(out, x)
		}
	}
	return out
}

func sliceSubset(a, b []string) bool {
	for _, x := range a {
		found := false
		for _, y := range b {
			if x == y {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func sliceIntersection(a, b []string) int {
	n := 0
	for _, x := range a {
		for _, y := range b {
			if x == y {
				n++
				break
			}
		}
	}
	return n
}

// sim is the similarity function from the original Python pipeline.
func sim(a, b string) float64 {
	if a == "" || b == "" {
		return 0
	}
	if a == b {
		return 1
	}
	ta := tokenSet(a)
	tb := tokenSet(b)
	if len(ta) > 0 && len(tb) > 0 && sliceSubset(ta, tb) {
		return math.Max(0.9, ratio(a, b))
	}
	r := ratio(a, b)
	if len(ta) > 0 && len(tb) > 0 {
		inter := sliceIntersection(ta, tb)
		union := len(ta) + len(tb) - inter
		if union > 0 {
			r = math.Max(r, float64(inter)/float64(union))
		}
		if strings.Contains(a, b) || strings.Contains(b, a) {
			minl, maxl := len(ta), len(tb)
			if minl > maxl {
				minl, maxl = maxl, minl
			}
			if maxl > 0 && float64(minl)/float64(maxl) >= 0.75 {
				r = math.Max(r, 0.93)
			}
		}
	}
	return r
}

func tvsim(a, b []string) float64 {
	best := 0.0
	for _, x := range a {
		for _, y := range b {
			if v := sim(x, y); v > best {
				best = v
			}
		}
	}
	return best
}

// ---------------------------------------------------------------------------
// Matcher
// ---------------------------------------------------------------------------

// Candidate is a possible match for a CSV row.
type Candidate struct {
	Index int
	Score float64
	TS    float64
	AS    float64
	ALS   float64
	FS    float64
}

// MatchResult is the matching outcome for one row.
type MatchResult struct {
	Row   Row
	Class string // exact | auto | ambiguous | none
	Pick  int    // index in Songs, -1 if none
	Cands []Candidate
}

// Matcher indexes the library for fuzzy matching.
type Matcher struct {
	Songs []Song
	index map[string]map[int]struct{}
}

func NewMatcher(songs []Song) *Matcher {
	for i := range songs {
		s := &songs[i]
		s.T = normalize(s.Title)
		s.A = normalize(s.Artist)
		s.AL = normalize(s.Album)
		s.TV = variants(s.Title, "", "")
		s.TG = tagsOf(s.Title)
		s.TOK = unionStrings(s.T, s.A)
	}
	m := &Matcher{Songs: songs, index: map[string]map[int]struct{}{}}
	for i := range songs {
		for _, tok := range songs[i].TOK {
			if m.index[tok] == nil {
				m.index[tok] = map[int]struct{}{}
			}
			m.index[tok][i] = struct{}{}
		}
	}
	return m
}

func unionStrings(parts ...string) []string {
	set := map[string]struct{}{}
	for _, p := range parts {
		for _, f := range strings.Fields(p) {
			set[f] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func (m *Matcher) song(i int) *Song { return &m.Songs[i] }

func (m *Matcher) isVTitle(A []string, s *Song) bool {
	for _, v := range A {
		if v == s.T {
			return true
		}
		for _, t := range s.TV {
			if v == t {
				return true
			}
		}
	}
	return false
}

func (m *Matcher) isUnknown(s *Song) bool {
	return s.A == "" || strings.Contains(s.A, "unknown artist")
}

func (m *Matcher) clash(x Candidate, tt map[string]bool) bool {
	tg := m.song(x.Index).TG
	if !tt["reprise"] && !tt["dub"] && !tg["reprise"] && !tg["dub"] {
		return false
	}
	return !sameSet(tt, tg)
}

func (m *Matcher) cands(t, a, al string) []Candidate {
	A := variants(t, al, a)
	an := normalize(a)
	aln := normalize(al)

	pool := map[int]struct{}{}
	for tok := range tokensOf(normalize(t)) {
		if len(tok) >= 3 {
			for i := range m.index[tok] {
				pool[i] = struct{}{}
			}
		}
	}
	for tok := range tokensOf(an) {
		if len(tok) >= 3 {
			for i := range m.index[tok] {
				pool[i] = struct{}{}
			}
		}
	}

	nt := normalize(t)
	out := make([]Candidate, 0, len(pool))
	for i := range pool {
		s := m.song(i)
		ts := tvsim(A, s.TV)
		as := sim(an, s.A)
		als := sim(aln, s.AL)
		if ts < 0.45 && as < 0.75 {
			continue
		}
		fs := sim(nt, s.T)
		out = append(out, Candidate{
			Index: i,
			Score: 0.55*ts + 0.30*as + 0.15*als,
			TS:    ts, AS: as, ALS: als, FS: fs,
		})
	}

	rounded := func(x float64) float64 { return math.Round(x*10000) / 10000 }
	sort.Slice(out, func(i, j int) bool {
		ri, rj := rounded(out[i].Score), rounded(out[j].Score)
		if ri != rj {
			return ri > rj
		}
		if out[i].FS != out[j].FS {
			return out[i].FS > out[j].FS
		}
		// Deterministic tie-break: on equal scores the lowest index wins.
		return out[i].Index < out[j].Index
	})
	if len(out) > 12 {
		out = out[:12]
	}
	return out
}

// Match runs the matching for every row.
func (m *Matcher) Match(rows []Row) []MatchResult {
	res := make([]MatchResult, 0, len(rows))
	for _, row := range rows {
		t, a, al := row.Track, row.Artist, row.Album
		tt := tagsOf(t)
		A := variants(t, al, a)
		sc := m.cands(t, a, al)

		pick := -1
		class := "none"
		var plausible []Candidate

		if len(sc) > 0 {
			albumok := func(x Candidate) bool { return x.ALS > 0.99 }

			exact := make([]Candidate, 0, 2)
			var vt, vtNoClash []Candidate
			for _, x := range sc {
				s := m.song(x.Index)
				if normalize(t) == s.T && x.AS > 0.999 {
					exact = append(exact, x)
				}
				if m.isVTitle(A, s) {
					vt = append(vt, x)
					if !m.clash(x, tt) {
						vtNoClash = append(vtNoClash, x)
					}
				}
			}

			switch {
			case len(exact) > 0:
				var ea []Candidate
				for _, x := range exact {
					if albumok(x) {
						ea = append(ea, x)
					}
				}
				if len(ea) > 0 {
					pick = ea[0].Index
				} else {
					pick = exact[0].Index
				}
				class = "exact"

			case len(vtNoClash) > 0:
				var bAlb []Candidate
				for _, x := range vtNoClash {
					if albumok(x) && x.AS >= 0.58 {
						bAlb = append(bAlb, x)
					}
				}
				var bTag []Candidate
				for _, x := range vtNoClash {
					if x.AS >= 0.85 && sameSet(m.song(x.Index).TG, tt) {
						bTag = append(bTag, x)
					}
				}
				sort.Slice(bTag, func(i, j int) bool {
					if bTag[i].ALS != bTag[j].ALS {
						return bTag[i].ALS > bTag[j].ALS
					}
					return bTag[i].Score > bTag[j].Score
				})

				oneAlbum := func(list []Candidate) bool {
					seen := map[string]struct{}{}
					for _, x := range list {
						seen[m.song(x.Index).AL] = struct{}{}
					}
					return len(seen) == 1
				}

				switch {
				case len(bAlb) > 0 && oneAlbum(bAlb):
					pick = bAlb[0].Index
					class = "auto"
				case len(bTag) > 0 && bTag[0].ALS >= 0.55 &&
					(len(bTag) == 1 || bTag[0].ALS-bTag[1].ALS >= 0.10):
					pick = bTag[0].Index
					class = "auto"
				case len(bTag) > 0 && oneAlbum(bTag):
					pick = bTag[0].Index
					class = "auto"
				default:
					switch {
					case len(bAlb) > 0:
						pick = bAlb[0].Index
					case len(bTag) > 0:
						pick = bTag[0].Index
					default:
						pick = vtNoClash[0].Index
					}
					class = "ambiguous"
				}

			default:
				b := sc[0]
				gap := b.Score
				if len(sc) > 1 {
					gap -= sc[1].Score
				}
				if b.Score >= 0.90 && gap >= 0.05 && b.AS >= 0.75 &&
					(b.ALS > 0.99 || sameSet(m.song(b.Index).TG, tt)) && !m.clash(b, tt) {
					pick = b.Index
					class = "auto"
				}
			}

			sig := map[string]bool{}
			for tok := range tokensOf(normalize(t)) {
				if len(tok) >= 3 {
					sig[tok] = true
				}
			}
			overlap := false
			for _, x := range sc {
				s := m.song(x.Index)
				overlap = false
				if len(sig) > 0 {
					for tok := range sig {
						if tokensOf(s.T)[tok] {
							overlap = true
							break
						}
					}
				}
				good := (m.isVTitle(A, s) && (x.AS >= 0.45 || m.isUnknown(s))) ||
					(x.TS >= 0.85 && x.AS >= 0.55) ||
					(x.AS >= 0.90 && x.TS >= 0.55) ||
					(albumok(x) && x.TS >= 0.6) ||
					(x.TS >= 0.95 && m.isUnknown(s))
				if good && (overlap || x.AS >= 0.9) {
					plausible = append(plausible, Candidate{Index: x.Index, Score: x.Score})
				}
				if len(plausible) == 5 {
					break
				}
			}
		}

		if class == "none" && len(plausible) > 0 {
			class = "ambiguous"
			pick = plausible[0].Index
		}
		if class == "ambiguous" && len(plausible) == 0 {
			class = "none"
			pick = -1
		}

		res = append(res, MatchResult{Row: row, Class: class, Pick: pick, Cands: plausible})
	}
	return res
}
