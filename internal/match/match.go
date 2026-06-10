// Package match ranks repos against a query with segment-aware scoring.
package match

import (
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/danieljmt/openrepo/internal/index"
)

// Stats is the open-frequency data used to break ranking ties.
type Stats struct {
	Count      int
	LastOpened time.Time
}

// StatsFunc reports open stats for a repo path.
type StatsFunc func(path string) Stats

// Match tiers, best first. Lower is better.
const (
	tierExact         = iota // query == full name
	tierSegmentExact         // query == one separator-delimited segment
	tierNamePrefix           // name starts with query
	tierSegmentPrefix        // a segment starts with query (mid-name match)
	tierSubstring            // query appears anywhere in the name
	tierFuzzy                // subsequence match (fallback only)
)

const separators = ".-_/"

// Rank returns the repos matching query, best first. An empty query returns
// every repo ordered by open frequency. A query containing "/" is matched
// against "org/name" and "host/org/name" instead of the bare name.
func Rank(repos []index.Repo, query string, stats StatsFunc) []index.Repo {
	return rank(repos, query, stats, false)
}

// RankWithFallback is Rank, but when nothing matches strictly it retries with
// fuzzy subsequence matching so near-misses and typos still surface.
func RankWithFallback(repos []index.Repo, query string, stats StatsFunc) []index.Repo {
	if out := rank(repos, query, stats, false); len(out) > 0 {
		return out
	}
	return rank(repos, query, stats, true)
}

type scored struct {
	repo index.Repo
	tier int
	span int // fuzzy only: length of the matched span, smaller is tighter
}

func rank(repos []index.Repo, query string, stats StatsFunc, fuzzy bool) []index.Repo {
	query = strings.ToLower(query)
	var matches []scored
	for _, r := range repos {
		s, ok := score(r, query, fuzzy)
		if ok {
			matches = append(matches, s)
		}
	}
	sort.SliceStable(matches, func(i, j int) bool {
		a, b := matches[i], matches[j]
		// An exact full-name match always wins; below that, open frequency
		// outranks match shape so the repos you actually use come first.
		if (a.tier == tierExact) != (b.tier == tierExact) {
			return a.tier == tierExact
		}
		sa, sb := stats(a.repo.Path), stats(b.repo.Path)
		if sa.Count != sb.Count {
			return sa.Count > sb.Count
		}
		if a.tier != b.tier {
			return a.tier < b.tier
		}
		if a.span != b.span {
			return a.span < b.span
		}
		if !sa.LastOpened.Equal(sb.LastOpened) {
			return sa.LastOpened.After(sb.LastOpened)
		}
		if len(a.repo.Name) != len(b.repo.Name) {
			return len(a.repo.Name) < len(b.repo.Name)
		}
		return a.repo.FullName() < b.repo.FullName()
	})
	out := make([]index.Repo, len(matches))
	for i, m := range matches {
		out[i] = m.repo
	}
	return out
}

func score(r index.Repo, query string, fuzzy bool) (scored, bool) {
	if query == "" {
		return scored{repo: r, tier: tierExact}, true
	}
	targets := []string{strings.ToLower(r.Name)}
	if strings.Contains(query, "/") {
		targets = []string{strings.ToLower(r.FullName())}
		if r.Host != "" {
			targets = append(targets, strings.ToLower(r.Slug()))
		}
	}
	best := scored{repo: r, tier: tierFuzzy + 1}
	for _, t := range targets {
		if tier, ok := tierFor(query, t); ok && tier < best.tier {
			best.tier = tier
		}
	}
	if best.tier <= tierSubstring {
		return best, true
	}
	if fuzzy {
		for _, t := range targets {
			if span, ok := subsequence(query, t); ok {
				best.tier = tierFuzzy
				if best.span == 0 || span < best.span {
					best.span = span
				}
			}
		}
		if best.tier == tierFuzzy {
			return best, true
		}
	}
	return scored{}, false
}

func tierFor(query, target string) (int, bool) {
	if query == target {
		return tierExact, true
	}
	segments := strings.FieldsFunc(target, func(c rune) bool {
		return strings.ContainsRune(separators, c)
	})
	if slices.Contains(segments, query) {
		return tierSegmentExact, true
	}
	if strings.HasPrefix(target, query) {
		return tierNamePrefix, true
	}
	for _, seg := range segments {
		if strings.HasPrefix(seg, query) {
			return tierSegmentPrefix, true
		}
	}
	if strings.Contains(target, query) {
		return tierSubstring, true
	}
	return 0, false
}

// subsequence reports whether every rune of query appears in order in target,
// and the span between the first and last matched rune (tighter is better).
func subsequence(query, target string) (span int, ok bool) {
	qr := []rune(query)
	if len(qr) == 0 {
		return 0, false
	}
	qi, first, last := 0, -1, -1
	for ti, c := range []rune(target) {
		if qi < len(qr) && c == qr[qi] {
			if first < 0 {
				first = ti
			}
			last = ti
			qi++
		}
	}
	if qi != len(qr) {
		return 0, false
	}
	return last - first + 1, true
}
