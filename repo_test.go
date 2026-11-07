package git

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/AbiXnash/codist-git/testrepo"
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
