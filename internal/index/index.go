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
	Host string // host directory name, e.g. "github.com"; empty for depth-2 group dirs like "sandbox"
	Path string // absolute path to the repo directory
}

// FullName returns "org/name".
func (r Repo) FullName() string { return r.Org + "/" + r.Name }

// Slug returns "host/org/name", or "org/name" when there is no host.
func (r Repo) Slug() string {
	if r.Host == "" {
		return r.FullName()
	}
	return r.Host + "/" + r.FullName()
}

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

// Scan discovers repos under srcDir. Top-level dirs with a dot in the name
// are hosts (github.com) laid out as host/org/repo; dot-less dirs (sandbox)
// are plain groups whose immediate children are repos.
func Scan(srcDir string) ([]Repo, error) {
	hosts, err := readDirs(srcDir)
	if err != nil {
		return nil, err
	}
	var repos []Repo
	for _, host := range hosts {
		if !strings.Contains(host, ".") {
			names, err := readDirs(filepath.Join(srcDir, host))
			if err != nil {
				continue
			}
			for _, name := range names {
				repos = append(repos, Repo{
					Name: name,
					Org:  host,
					Path: filepath.Join(srcDir, host, name),
				})
			}
			continue
		}
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
