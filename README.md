# openrepo

Find a repo under `$GOPATH/src/<host>/<org>/<repo>` by name and open it in
your editor. Top-level dirs without a dot in the name (e.g. `src/sandbox/`)
are treated as plain groups whose immediate children are repos — no git
required. Matching is segment-aware (typing `payments` finds
`123345.teamname.payments-service`), suggestions are ranked by how often you
open each repo, and ambiguous queries drop into an interactive picker.

## Install

```sh
go install github.com/danieljmt/openrepo@latest
```

Add to `.zshrc` for tab-completion:

```sh
source <(openrepo completion zsh)
```

## Usage

```sh
openrepo                # interactive picker over every repo
openrepo gh-chat        # unique match → opens immediately
openrepo metrics        # multiple matches → picker, pre-filtered
openrepo danieljmt/tool # disambiguate by org (or host/org)
openrepo -p foo         # print the path instead of opening
cd "$(openrepo -p foo)" # the picker draws on stderr, so this works
```

## Editor

Opens with `$OPENREPO_EDITOR`, falling back to `$EDITOR`, then `code`.

## How matching ranks

An exact full-name match always wins. Below that, open count ranks first, so
the repos you actually use surface ahead of better-shaped matches; remaining
ties go by match shape (exact segment (`.`/`-`/`_` separated) → name prefix →
segment prefix → substring), then recency. If nothing matches strictly, the
picker falls back to fuzzy (subsequence) matching.

Open counts live in `~/Library/Application Support/openrepo/frequency.json`
(`os.UserConfigDir()`).
