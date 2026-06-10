// Package index discovers repositories under $GOPATH/src/<host>/<org>/<repo>.
package index

import (
	"go/build"
	"os"
	"path/filepath"
	"strings"
)

// Repo is a single discovered repository checkout.
type Repo struct {
	Name string // repo directory name, e.g. "openrepo"
	Org  string // org directory name, e.g. "danieljmt"
	Host string // host directory name, e.g. "github.com"
	Path string // absolute path to the repo directory
}

// FullName returns "org/name".
func (r Repo) FullName() string { return r.Org + "/" + r.Name }

// SrcDir returns the directory scanned for repos: $GOPATH/src, where GOPATH
// comes from the environment and falls back to the go toolchain default.
func SrcDir() string {
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		gopath = build.Default.GOPATH
	}
	// GOPATH may be a list; repos live under the first element.
	gopath = filepath.SplitList(gopath)[0]
	return filepath.Join(gopath, "src")
}

// Scan walks exactly three directory levels (host/org/repo) under srcDir.
func Scan(srcDir string) ([]Repo, error) {
	hosts, err := readDirs(srcDir)
	if err != nil {
		return nil, err
	}
	var repos []Repo
	for _, host := range hosts {
		orgs, err := readDirs(filepath.Join(srcDir, host))
		if err != nil {
			continue
		}
		for _, org := range orgs {
			names, err := readDirs(filepath.Join(srcDir, host, org))
			if err != nil {
				continue
			}
			for _, name := range names {
				repos = append(repos, Repo{
					Name: name,
					Org:  org,
					Host: host,
					Path: filepath.Join(srcDir, host, org, name),
				})
			}
		}
	}
	return repos, nil
}

// readDirs lists subdirectory names, skipping hidden dirs and node_modules.
func readDirs(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") || name == "node_modules" {
			continue
		}
		names = append(names, name)
	}
	return names, nil
}
