package git

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// FileStat is how much one commit changed one file.
type FileStat struct {
	Path           string
	Added, Deleted int // lines; -1 for a binary file
}

// CommitStat is a commit with the size of its change, for learning which changes tend to cause bugs.
type CommitStat struct {
	SHA, Author, Subject string
	Time                 time.Time
	Parents              int
	Files                []FileStat
}

// History lists up to limit commits reachable from HEAD with what each changed, oldest first, in one git call.
// Renames are reported under the new path.
func (r *Repo) History(limit int) ([]CommitStat, error) {
	out, err := r.run("log", "-n", strconv.Itoa(limit), "--reverse", "--numstat", "--no-renames",
		"--format="+end+"%H"+sep+"%an"+sep+"%at"+sep+"%P"+sep+"%s")
	if err != nil {
		return nil, err
	}
	var cs []CommitStat
	for _, rec := range strings.Split(out, end) {
		head, stats, _ := strings.Cut(strings.TrimLeft(rec, "\n"), "\n")
		f := strings.Split(head, sep)
		if len(f) < 5 {
			continue
		}
		at, _ := strconv.ParseInt(f[2], 10, 64)
		c := CommitStat{SHA: f[0], Author: f[1], Time: time.Unix(at, 0), Parents: len(strings.Fields(f[3])), Subject: f[4]}
		for _, l := range strings.Split(stats, "\n") {
			parts := strings.SplitN(l, "\t", 3)
			if len(parts) != 3 {
				continue
			}
			a, errA := strconv.Atoi(parts[0])
			d, errD := strconv.Atoi(parts[1])
			if errA != nil || errD != nil {
				a, d = -1, -1
			}
			c.Files = append(c.Files, FileStat{Path: parts[2], Added: a, Deleted: d})
		}
		cs = append(cs, c)
	}
	return cs, nil
}

// BlameAt names, for lines from..to of a file at ref, the commit that last changed each line.
func (r *Repo) BlameAt(ref, path string, from, to int) []string {
	if unsafe(ref) {
		return nil
	}
	out, err := r.run("blame", "--porcelain", "-L", fmt.Sprintf("%d,%d", from, to), ref, "--", path)
	if err != nil {
		return nil
	}
	var shas []string
	for _, l := range strings.Split(out, "\n") {
		// A header line: <40-hex sha> <old line> <new line> [<count>]
		if f := strings.Fields(l); len(f) >= 3 && len(f[0]) == 40 && isHex(f[0]) {
			shas = append(shas, f[0])
		}
	}
	return shas
}

func isHex(s string) bool {
	for _, c := range s {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return true
}
