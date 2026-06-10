// Package opener launches the user's editor on a repo directory.
package opener

import (
	"os"
	"os/exec"
	"strings"
)

// Editor returns the editor command line: $OPENREPO_EDITOR, else $EDITOR,
// else "code".
func Editor() []string {
	for _, env := range []string{"OPENREPO_EDITOR", "EDITOR"} {
		if v := strings.TrimSpace(os.Getenv(env)); v != "" {
			return strings.Fields(v)
		}
	}
	return []string{"code"}
}

// Open runs the editor with the repo path appended, attached to the terminal
// so terminal editors like vim work.
func Open(path string) error {
	editor := Editor()
	cmd := exec.Command(editor[0], append(editor[1:], path)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
