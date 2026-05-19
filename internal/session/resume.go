package session

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

func ResolveContinue(cwd string) (string, error) {
	ids, err := listSessions(cwd)
	if err != nil {
		return "", err
	}
	if len(ids) == 0 {
		return "", fmt.Errorf("no prior sessions for %s", cwd)
	}
	return ids[0], nil
}

func ResolveResume(cwd, idPrefix string) (string, error) {
	ids, err := listSessions(cwd)
	if err != nil {
		return "", err
	}
	if len(ids) == 0 {
		return "", fmt.Errorf("no sessions for %s", cwd)
	}
	var matches []string
	for _, id := range ids {
		if strings.HasPrefix(id, idPrefix) {
			matches = append(matches, id)
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no session matches prefix %q", idPrefix)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("ambiguous prefix %q matches %d sessions", idPrefix, len(matches))
	}
}

func listSessions(cwd string) ([]string, error) {
	dir, err := ProjectDir(cwd)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	type entry struct {
		id    string
		mtime int64
	}
	var infos []entry
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		infos = append(infos, entry{
			id:    strings.TrimSuffix(e.Name(), ".jsonl"),
			mtime: info.ModTime().UnixNano(),
		})
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].mtime > infos[j].mtime })
	ids := make([]string, len(infos))
	for i, e := range infos {
		ids[i] = e.id
	}
	return ids, nil
}
