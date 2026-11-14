package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/abxengg/codist-git/testrepo"
)

func open(t *testing.T) (*Repo, *testrepo.Repo) {
	t.Helper()
	tr := testrepo.New(t)
	r, err := Open(filepath.Join(tr.Dir, "order-service")) // any folder inside finds the root
	if err != nil {
		t.Fatal(err)
	}
	return r, tr
}

func TestAvailableHeadBranch(t *testing.T) {
	if !Available() {
		t.Fatal("tests need git")
	}
	r, tr := open(t)
	if r.Head() != tr.Fix || r.Branch() != "main" {
		t.Errorf("head %s branch %s", r.Head(), r.Branch())
	}
	tr.Git("checkout", "-q", "--detach", tr.Bug)
	if r.Branch() != "detached" || r.Head() != tr.Bug {
		t.Errorf("detached: %s %s", r.Branch(), r.Head())
	}
	empty := &Repo{Root: t.TempDir()} // not a repository
	if empty.Head() != "" || empty.Branch() != "detached" || empty.Count() != 0 || empty.CountRef("HEAD") != 0 {
		t.Error("outside a repository everything is empty")
	}
}

func TestOpenOutside(t *testing.T) {
	if _, err := Open(t.TempDir()); err == nil || !strings.Contains(err.Error(), "not inside a git repository") {
		t.Errorf("%v", err)
	}
}

func TestFilesAndReadAll(t *testing.T) {
	r, tr := open(t)
	files, err := r.Files(tr.Fix)
	if err != nil || len(files) != len(testrepo.Files) || !slices.Contains(files, testrepo.ServicePath) {
		t.Fatalf("%d files, %v", len(files), err)
	}
	if _, err := r.Files("nope"); err == nil {
		t.Error("a missing commit has no files")
	}
	all := r.ReadAll(tr.Bug, []string{testrepo.ServicePath, "missing.java", "order-service/pom.xml"})
	if string(all[testrepo.ServicePath]) != testrepo.ServiceBug || len(all) != 2 {
		t.Errorf("read %d files", len(all))
	}
	if len(r.ReadAll(tr.Bug, nil)) != 0 {
		t.Error("nothing asked, nothing read")
	}
	if len((&Repo{Root: t.TempDir()}).ReadAll("HEAD", []string{"a"})) != 0 {
		t.Error("outside a repository nothing is read")
	}
}

func TestLogFileAuthorsRefs(t *testing.T) {
	r, tr := open(t)
	log := r.LogFile(testrepo.ServicePath, 10)
	if len(log) != 3 || log[0].SHA != tr.Fix || log[2].SHA != tr.Init || log[0].Author != "Dev" {
		t.Fatalf("%+v", log)
	}
	if len(r.LogFile(testrepo.ServicePath, 1)) != 1 || r.LogFile("x", 5) != nil {
		t.Error("limit, or an unknown file")
	}
	if (&Repo{Root: t.TempDir()}).LogFile("x", 1) != nil {
		t.Error("outside a repository")
	}
	if a := r.Authors(testrepo.Branch); a["Dev"] != 4 {
		t.Errorf("authors %v", a)
	}
	if len(r.Authors("nope")) != 0 {
		t.Error("unknown ref has no authors")
	}
	if r.CountRef(testrepo.Branch) != 4 || r.Count() != 3 {
		t.Errorf("counts %d %d", r.CountRef(testrepo.Branch), r.Count())
	}
	refs := r.Refs()
	names := []string{}
	for _, ref := range refs {
		names = append(names, ref.Name)
	}
	if !slices.Contains(names, "main") || !slices.Contains(names, testrepo.Branch) {
		t.Errorf("refs %v", names)
	}
	if (&Repo{Root: t.TempDir()}).Refs() != nil {
		t.Error("outside a repository there are no refs")
	}
	// A remote branch is listed as remote, and its HEAD pointer is left out.
	tr.Git("update-ref", "refs/remotes/origin/main", tr.Fix)
	tr.Git("symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
	remote := 0
	for _, ref := range r.Refs() {
		if ref.Remote {
			remote++
			if ref.Name != "origin/main" {
				t.Errorf("remote %s", ref.Name)
			}
		}
	}
	if remote != 1 {
		t.Errorf("%d remotes", remote)
	}
}

func TestLastChangeAndChurn(t *testing.T) {
	r, tr := open(t)
	c, ok := r.LastChange(testrepo.ServicePath, testrepo.BugLine)
	if !ok || c.SHA != tr.Bug {
		t.Errorf("the catch line was written by the bug commit, got %s", c.SHA)
	}
	if _, ok := r.LastChange("missing.java", 1); ok {
		t.Error("a missing file has no history")
	}
	if ch := r.Churn(10); ch[testrepo.ServicePath] != 3 {
		t.Errorf("churn %v", ch)
	}
	if len((&Repo{Root: t.TempDir()}).Churn(5)) != 0 {
		t.Error("outside a repository")
	}
}

func TestChangesDeletedAndRenamed(t *testing.T) {
	r, tr := open(t)
	os.Remove(filepath.Join(tr.Dir, "order-service/pom.xml"))
	tr.Git("mv", "payment-service/pom.xml", "payment-service/build.xml")
	tr.Commit("Move build files", nil)
	ch, err := r.Changes(r.Head())
	if err != nil {
		t.Fatal(err)
	}
	status := map[string]string{}
	for _, c := range ch {
		status[c.Path] = c.Status
	}
	if status["order-service/pom.xml"] != "deleted" || status["payment-service/build.xml"] != "renamed" {
		t.Errorf("%v", status)
	}
	if _, err := r.Changes("nope"); err == nil {
		t.Error("unknown commit")
	}
	if _, err := r.Get("nope"); err == nil {
		t.Error("unknown commit")
	}
	if _, err := (&Repo{Root: t.TempDir()}).LogRef("HEAD", 1); err == nil {
		t.Error("outside a repository")
	}
}

func TestMissing(t *testing.T) {
	r, tr := open(t)
	if _, ok := r.FileAt(tr.Init, "missing.java"); ok {
		t.Error("a missing file is not read")
	}
	if r.Resolve("nope") != "" || r.Resolve(testrepo.Branch) != tr.Ext || r.Resolve(tr.Bug[:8]) != tr.Bug {
		t.Error("resolve")
	}
}

// A revision that starts with a dash would be read by git as an option (--output=FILE writes a file), so every
// function that takes one refuses it.
func TestRevisionsThatLookLikeOptionsAreRefused(t *testing.T) {
	r := newRepo(t)
	target := filepath.Join(t.TempDir(), "written.txt")
	bad := "--output=" + target
	if _, err := r.Get(bad); err == nil {
		t.Error("Get took an option")
	}
	if _, err := r.LogRef(bad, 5); err == nil {
		t.Error("LogRef took an option")
	}
	if _, err := r.Changes(bad); err == nil {
		t.Error("Changes took an option")
	}
	if _, err := r.Files(bad); err == nil {
		t.Error("Files took an option")
	}
	if _, ok := r.FileAt(bad, "A.java"); ok {
		t.Error("FileAt took an option")
	}
	if r.Resolve(bad) != "" || r.CountRef(bad) != 0 || len(r.Authors(bad)) != 0 || r.IsAncestor(bad, "HEAD") || r.Later(bad, "A.java") != nil ||
		r.BlameAt(bad, "A.java", 1, 1) != nil || len(r.ReadAll(bad, []string{"A.java"})) != 0 || len(r.ReadAll("HEAD\nHEAD", []string{"A.java"})) != 0 {
		t.Error("a function took an option as a revision")
	}
	if _, err := os.Stat(target); err == nil {
		t.Error("git wrote a file named by a revision")
	}
}

// File names with spaces stay whole, and a changed line that looks like a diff header stays a changed line.
func TestSpacedNamesAndHeaderLikeLines(t *testing.T) {
	r := newRepo(t)
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", r.Root, "-c", "user.name=T", "-c", "user.email=t@t"}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	os.MkdirAll(filepath.Join(r.Root, "my docs"), 0o755)
	os.WriteFile(filepath.Join(r.Root, "my docs", "read me.txt"), []byte("one\n-- old note\nthree\nfour\n"), 0o644)
	run("add", ".")
	run("commit", "-q", "-m", "Add notes")
	files, err := r.Files(r.Head())
	if err != nil || !slices.Contains(files, "my docs/read me.txt") || len(files) != 2 {
		t.Fatalf("files: %q %v", files, err)
	}
	os.WriteFile(filepath.Join(r.Root, "my docs", "read me.txt"), []byte("one\n++ new note\nthree\nfour\nfive\n"), 0o644)
	run("commit", "-q", "-am", "Change notes")
	ch, err := r.Changes(r.Head())
	if err != nil || len(ch) != 1 {
		t.Fatalf("changes: %+v %v", ch, err)
	}
	if ch[0].Path != "my docs/read me.txt" || !slices.Equal(ch[0].Added, []int{2, 5}) ||
		!strings.Contains(ch[0].Patch, "--- old note") || !strings.Contains(ch[0].Patch, "+++ new note") {
		t.Errorf("change: %+v", ch[0])
	}
}

// FileAt reads through one git process that stays open: folders, missing files and names with spaces do not
// upset the files read after them, new commits are seen, and it starts again after Close.
func TestFileAtKeepsOneProcess(t *testing.T) {
	r := newRepo(t)
	t.Cleanup(r.Close)
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", r.Root, "-c", "user.name=T", "-c", "user.email=t@t"}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	os.MkdirAll(filepath.Join(r.Root, "my docs"), 0o755)
	os.WriteFile(filepath.Join(r.Root, "my docs", "read me.txt"), []byte("hello\n"), 0o644)
	run("add", ".")
	run("commit", "-q", "-m", "Add notes")
	head := r.Head()
	for i := 0; i < 3; i++ {
		if _, ok := r.FileAt(head, "my docs"); ok {
			t.Fatal("a folder is not a file")
		}
		if _, ok := r.FileAt(head, "no such file.txt"); ok {
			t.Fatal("a missing file was read")
		}
		if _, ok := r.FileAt("nosuchbranch", "A.java"); ok {
			t.Fatal("a missing revision was read")
		}
		if src, ok := r.FileAt(head, "my docs/read me.txt"); !ok || string(src) != "hello\n" {
			t.Fatalf("spaced name: %q %v", src, ok)
		}
		if src, ok := r.FileAt(head+"~1", "A.java"); !ok || !strings.Contains(string(src), "return 2;") {
			t.Fatalf("after the others: %q %v", src, ok)
		}
		if i == 1 {
			r.Close()
		}
	}
	if r.cat == nil {
		t.Error("the process is kept between reads")
	}
}
