package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// newRepo makes a repository with two commits: one adds A.java, the next changes line 3 and adds line 5.
func newRepo(t *testing.T) *Repo {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=T", "-c", "user.email=t@t"}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	write := func(s string) { os.WriteFile(filepath.Join(dir, "A.java"), []byte(s), 0o644) }
	run("init", "-q")
	write("class A {\n  int a() {\n    return 1;\n  }\n}\n")
	run("add", ".")
	run("commit", "-q", "-m", "Add A")
	write("class A {\n  int a() {\n    return 2;\n  }\n  int b() { return 3; }\n}\n")
	run("commit", "-q", "-am", "Change A", "-m", "With a body.")
	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestLogAndChanges(t *testing.T) {
	r := newRepo(t)
	log, err := r.Log(10)
	if err != nil || len(log) != 2 {
		t.Fatalf("log: %v %v", log, err)
	}
	if log[0].Subject != "Change A" || log[0].Body != "With a body." || len(log[0].Parents) != 1 || log[1].Subject != "Add A" {
		t.Errorf("commits: %+v", log)
	}
	ch, err := r.Changes(log[0].SHA)
	if err != nil || len(ch) != 1 {
		t.Fatalf("changes: %v %v", ch, err)
	}
	if ch[0].Path != "A.java" || ch[0].Status != "modified" || !slices.Equal(ch[0].Added, []int{3, 5}) {
		t.Errorf("change: %+v", ch[0])
	}
	first, _ := r.Changes(log[1].SHA)
	if len(first) != 1 || first[0].Status != "added" || len(first[0].Added) != 5 {
		t.Errorf("first commit: %+v", first)
	}
	if src, ok := r.FileAt(log[1].SHA, "A.java"); !ok || string(src) == "" {
		t.Error("FileAt failed")
	}
	if c, ok := r.LastChange("A.java", 5); !ok || c.SHA != log[0].SHA {
		t.Errorf("blame: %+v", c)
	}
	if all := r.ReadAll(log[1].SHA, []string{"A.java", "missing.java"}); len(all) != 1 || !strings.Contains(string(all["A.java"]), "return 1;") {
		t.Errorf("ReadAll: %q", all)
	}
	if refs := r.Refs(); len(refs) != 1 || refs[0].SHA != log[0].SHA || r.Resolve(refs[0].Name) != log[0].SHA {
		t.Errorf("refs: %+v", refs)
	}
	if r.Authors("HEAD")["T"] != 2 || r.CountRef("HEAD") != 2 {
		t.Error("authors or count wrong")
	}
	if r.Count() != 2 || r.Churn(10)["A.java"] != 2 {
		t.Error("count or churn wrong")
	}
}
