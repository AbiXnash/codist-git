package git

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// IsCode says whether one line of Java, seen alone, is code rather than a comment, an import or a blank line. A
// line inside a /* */ block that does not start with * cannot be told apart this way; CodeLines reads the whole
// file and can.
func IsCode(line string) bool {
	t := strings.TrimSpace(line)
	switch {
	case t == "", strings.HasPrefix(t, "//"), strings.HasPrefix(t, "/*"), strings.HasPrefix(t, "*"),
		strings.HasPrefix(t, "import "), strings.HasPrefix(t, "package "):
		return false
	}
	return true
}

// CodeLines marks, for each line of a Java file, whether it holds code: anything outside comments and javadoc,
// other than imports, the package line and blank space. It follows strings, characters and text blocks, so a "//"
// inside a string is not a comment.
func CodeLines(src []byte) []bool {
	lines := []bool{false}
	inBlock, inString, inChar, inText := false, false, false, false
	n := len(src)
	for i := 0; i < n; i++ {
		c := src[i]
		if c == '\n' {
			lines = append(lines, false)
			inString, inChar = false, false // an unclosed literal does not run past the line
			continue
		}
		cur := len(lines) - 1
		switch {
		case inBlock:
			if c == '*' && i+1 < n && src[i+1] == '/' {
				inBlock = false
				i++
			}
		case inText:
			lines[cur] = true
			if c == '"' && i+2 < n && src[i+1] == '"' && src[i+2] == '"' {
				inText = false
				i += 2
			}
		case inString || inChar:
			lines[cur] = true
			if c == '\\' {
				i++
			} else if (inString && c == '"') || (inChar && c == '\'') {
				inString, inChar = false, false
			}
		case c == '/' && i+1 < n && src[i+1] == '/':
			for i+1 < n && src[i+1] != '\n' {
				i++
			}
		case c == '/' && i+1 < n && src[i+1] == '*':
			inBlock = true
			i++
		case c == '"' && i+2 < n && src[i+1] == '"' && src[i+2] == '"':
			inText = true
			lines[cur] = true
			i += 2
		case c == '"':
			inString = true
			lines[cur] = true
		case c == '\'':
			inChar = true
			lines[cur] = true
		case c != ' ' && c != '\t' && c != '\r':
			lines[cur] = true
		}
	}
	// Imports and the package line are not logic.
	for i, l := range bytes.Split(src, []byte("\n")) {
		if i < len(lines) && lines[i] {
			t := bytes.TrimSpace(l)
			if bytes.HasPrefix(t, []byte("import ")) || bytes.HasPrefix(t, []byte("package ")) {
				lines[i] = false
			}
		}
	}
	return lines
}

// CountCode counts the given 1-based lines that hold code.
func CountCode(code []bool, lines []int) int {
	n := 0
	for _, l := range lines {
		if l >= 1 && l <= len(code) && code[l-1] {
			n++
		}
	}
	return n
}

var hunkLines = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

// fileDiff is one file's changed lines in one commit.
type fileDiff struct {
	path           string
	added, deleted []int // new-side and old-side line numbers
}

// CodeChurn counts, for up to limit commits from HEAD, the lines of Java code each commit added and deleted per
// file, leaving out comments, javadoc, imports and blank lines. Each changed file is read in full at the commit and
// before it, so lines inside a comment block are known even when they do not start with *. A commit that only
// changed comments has no entry for that file.
func (r *Repo) CodeChurn(limit int) (map[string]map[string][2]int, error) {
	type commit struct {
		sha, parent string
		files       map[string]*fileDiff
		order       []string
	}
	cmd := exec.Command("git", "-C", r.Root, "-c", "core.quotepath=off", "log", "-n", strconv.Itoa(limit), "-p", "-U0",
		"--no-renames", "--no-color", "--format="+end+"%H"+sep+"%P", "--", "*.java")
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	var commits []*commit
	var cur *commit
	var file *fileDiff
	at := func(path string) *fileDiff {
		if f, ok := cur.files[path]; ok {
			return f
		}
		f := &fileDiff{path: path}
		cur.files[path] = f
		cur.order = append(cur.order, path)
		return f
	}
	oldPath := ""
	o, n := 0, 0
	sc := bufio.NewScanner(out)
	sc.Buffer(make([]byte, 1024*1024), 64*1024*1024)
	for sc.Scan() {
		l := sc.Text()
		switch {
		case strings.HasPrefix(l, end):
			sha, parents, _ := strings.Cut(strings.TrimPrefix(l, end), sep)
			ps := strings.Fields(parents)
			cur = &commit{sha: sha, files: map[string]*fileDiff{}}
			if len(ps) > 0 {
				cur.parent = ps[0]
			}
			commits = append(commits, cur)
			file = nil
		case cur == nil:
		case strings.HasPrefix(l, "--- "):
			oldPath = strings.TrimPrefix(strings.TrimPrefix(l, "--- "), "a/")
		case strings.HasPrefix(l, "+++ "):
			p := strings.TrimPrefix(strings.TrimPrefix(l, "+++ "), "b/")
			if p == "/dev/null" { // a deleted file: its lines are counted against the old path
				p = oldPath
			}
			file = at(p)
		case strings.HasPrefix(l, "@@"):
			if m := hunkLines.FindStringSubmatch(l); m != nil {
				o, _ = strconv.Atoi(m[1])
				n, _ = strconv.Atoi(m[3])
			}
		case file != nil && strings.HasPrefix(l, "+"):
			file.added = append(file.added, n)
			n++
		case file != nil && strings.HasPrefix(l, "-"):
			file.deleted = append(file.deleted, o)
			o++
		}
	}
	if err := cmd.Wait(); err != nil {
		return nil, err
	}
	blobs, err := r.newBatch()
	if err != nil {
		return nil, err
	}
	defer blobs.close()
	churn := map[string]map[string][2]int{}
	for _, c := range commits {
		for _, p := range c.order {
			f := c.files[p]
			added, deleted := 0, 0
			if len(f.added) > 0 {
				if src, ok := blobs.read(c.sha + ":" + f.path); ok {
					added = CountCode(CodeLines(src), f.added)
				}
			}
			if len(f.deleted) > 0 && c.parent != "" {
				if src, ok := blobs.read(c.parent + ":" + f.path); ok {
					deleted = CountCode(CodeLines(src), f.deleted)
				}
			}
			if added+deleted > 0 {
				if churn[c.sha] == nil {
					churn[c.sha] = map[string][2]int{}
				}
				churn[c.sha][f.path] = [2]int{added, deleted}
			}
		}
	}
	return churn, sc.Err()
}

// batch reads many files through one git cat-file process.
type batch struct {
	cmd *exec.Cmd
	in  io.WriteCloser
	out *bufio.Reader
}

func (r *Repo) newBatch() (*batch, error) {
	cmd := exec.Command("git", "-C", r.Root, "cat-file", "--batch")
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &batch{cmd: cmd, in: in, out: bufio.NewReaderSize(out, 1<<20)}, nil
}

// read returns the file named "rev:path", or false when there is none.
func (b *batch) read(spec string) ([]byte, bool) {
	if _, err := fmt.Fprintln(b.in, spec); err != nil {
		return nil, false
	}
	header, err := b.out.ReadString('\n')
	if err != nil {
		return nil, false
	}
	f := strings.Fields(header)
	if len(f) < 3 || f[1] != "blob" {
		return nil, false // "<spec> missing"
	}
	size, err := strconv.Atoi(f[2])
	if err != nil {
		return nil, false
	}
	buf := make([]byte, size+1) // the content and its trailing newline
	if _, err := io.ReadFull(b.out, buf); err != nil {
		return nil, false
	}
	return buf[:size], true
}

func (b *batch) close() {
	b.in.Close()
	_ = b.cmd.Wait()
}
