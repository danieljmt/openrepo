// Command openrepo finds a repo under $GOPATH/src by name and opens it in
// your editor, with frequency-ranked matching and an interactive picker.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"github.com/danieljmt/openrepo/internal/freq"
	"github.com/danieljmt/openrepo/internal/index"
	"github.com/danieljmt/openrepo/internal/match"
	"github.com/danieljmt/openrepo/internal/opener"
	"github.com/danieljmt/openrepo/internal/picker"
)

const zshCompletion = `#compdef openrepo
if ! whence compdef >/dev/null; then
  autoload -Uz compinit
  compinit
fi
_openrepo() {
  local -a repos
  repos=(${(f)"$(command openrepo __complete "${words[CURRENT]}" 2>/dev/null)"})
  (( ${#repos} )) || return 1
  compadd -U -V repos -- "${repos[@]}"
  # Substring matches share no common prefix; without menu insertion zsh
  # would insert that (empty) prefix and wipe the typed word.
  (( ${#repos} > 1 )) && compstate[insert]=menu
  return 0
}
compdef _openrepo openrepo
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "openrepo:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "--version", "-version", "version":
			fmt.Println(version())
			return nil
		case "completion":
			if len(args) < 2 || args[1] != "zsh" {
				return fmt.Errorf("usage: openrepo completion zsh")
			}
			fmt.Print(zshCompletion)
			return nil
		case "__complete":
			word := ""
			if len(args) > 1 {
				word = args[1]
			}
			return complete(word)
		}
	}

	fs := flag.NewFlagSet("openrepo", flag.ExitOnError)
	printOnly := fs.Bool("p", false, "print the repo path instead of opening the editor")
	fs.BoolVar(printOnly, "print", *printOnly, "alias for -p")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), `usage: openrepo [-p] [query]

Finds a repo under %s by name and opens it in your editor
($OPENREPO_EDITOR, $EDITOR, or "code"). With no query, or when the query is
ambiguous, an interactive picker appears.

  openrepo completion zsh   print the zsh completion script
  openrepo --version        print the build's git revision

`, index.SrcDir())
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 1 {
		fs.Usage()
		os.Exit(2)
	}
	query := fs.Arg(0)

	repos, err := index.Scan(index.SrcDir())
	if err != nil {
		return fmt.Errorf("scanning %s: %w", index.SrcDir(), err)
	}
	if len(repos) == 0 {
		return fmt.Errorf("no repos found under %s", index.SrcDir())
	}
	store := freq.Load()
	stats := func(path string) match.Stats {
		e := store.Get(path)
		return match.Stats{Score: store.Score(path), Count: e.Count, LastOpened: e.LastOpened}
	}

	var chosen index.Repo
	ranked := match.Rank(repos, query, stats)
	exact := match.Exact(repos, query)
	if len(exact) == 1 {
		// The full name was typed: open it, even if other repos also match.
		chosen = exact[0]
	} else if query != "" && len(ranked) == 1 {
		chosen = ranked[0]
	} else {
		// No query, ambiguous, or no strict match: interactive picker
		// (falls back to fuzzy matching as you type).
		r, ok, err := picker.Pick(repos, query, stats)
		if err != nil {
			return err
		}
		if !ok {
			os.Exit(1) // cancelled
		}
		chosen = r
	}

	if err := store.Bump(chosen.Path); err != nil {
		fmt.Fprintln(os.Stderr, "openrepo: saving frequency:", err)
	}
	if *printOnly {
		fmt.Println(chosen.Path)
		return nil
	}
	return opener.Open(chosen.Path)
}

// version reports the git revision Go embedded at build time: vcs.revision
// for builds from a checkout, or the module pseudo-version (which embeds the
// sha) for proxy installs like go install ...@main.
func version() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	var rev, suffix string
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			if s.Value == "true" {
				suffix = " (modified)"
			}
		}
	}
	if len(rev) > 12 {
		rev = rev[:12]
	}
	if rev != "" {
		return rev + suffix
	}
	if v := bi.Main.Version; v != "" && v != "(devel)" {
		return v
	}
	return "unknown"
}

// complete prints frequency-ranked candidates for shell completion, one per
// line: bare repo names, or org/name forms when the word contains "/".
func complete(word string) error {
	repos, err := index.Scan(index.SrcDir())
	if err != nil {
		return err
	}
	store := freq.Load()
	stats := func(path string) match.Stats {
		e := store.Get(path)
		return match.Stats{Score: store.Score(path), Count: e.Count, LastOpened: e.LastOpened}
	}
	seen := map[string]bool{}
	for _, r := range match.RankWithFallback(repos, word, stats) {
		out := r.Name
		if strings.Contains(word, "/") {
			out = r.FullName()
		}
		if !seen[out] {
			seen[out] = true
			fmt.Println(out)
		}
	}
	return nil
}
