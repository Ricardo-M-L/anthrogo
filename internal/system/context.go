package system

import (
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// GitStatusSnapshot mirrors src/context.ts getGitStatus(). Returns "" when
// not in a git repo. Truncates `git status --short` to 2000 chars.
const gitStatusMaxChars = 2000

func GitStatusSnapshot(cwd string) (string, error) {
	if !isGitRepo(cwd) {
		return "", nil
	}
	run := func(args ...string) string {
		c := exec.Command("git", append([]string{"--no-optional-locks"}, args...)...)
		c.Dir = cwd
		out, _ := c.Output()
		return strings.TrimSpace(string(out))
	}
	branch := run("rev-parse", "--abbrev-ref", "HEAD")
	mainBranch := defaultBranch(cwd)
	status := run("status", "--short")
	log := run("log", "--oneline", "-n", "5")
	user := run("config", "user.name")

	if len(status) > gitStatusMaxChars {
		status = status[:gitStatusMaxChars] + "\n... (truncated)"
	}
	var b strings.Builder
	b.WriteString("This is the git status at the start of the conversation. " +
		"This snapshot will not update during the turn.\n\n")
	fmt.Fprintf(&b, "Current branch: %s\n", branch)
	if mainBranch != "" {
		fmt.Fprintf(&b, "Main branch: %s\n", mainBranch)
	}
	if user != "" {
		fmt.Fprintf(&b, "Git user: %s\n", user)
	}
	if status == "" {
		b.WriteString("Status: (clean)\n")
	} else {
		fmt.Fprintf(&b, "Status:\n%s\n", status)
	}
	if log != "" {
		fmt.Fprintf(&b, "Recent commits:\n%s\n", log)
	}
	return b.String(), nil
}

func isGitRepo(cwd string) bool {
	c := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	c.Dir = cwd
	c.Stderr = io.Discard
	out, err := c.Output()
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

func defaultBranch(cwd string) string {
	for _, attempt := range [][]string{
		{"symbolic-ref", "refs/remotes/origin/HEAD"},
		{"config", "init.defaultBranch"},
	} {
		c := exec.Command("git", attempt...)
		c.Dir = cwd
		c.Stderr = io.Discard
		out, err := c.Output()
		if err == nil {
			s := strings.TrimSpace(string(out))
			if strings.HasPrefix(s, "refs/remotes/origin/") {
				return strings.TrimPrefix(s, "refs/remotes/origin/")
			}
			return s
		}
	}
	return ""
}
