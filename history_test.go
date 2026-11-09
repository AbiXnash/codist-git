package git

import (
	"testing"

	"github.com/abxengg/codist-git/testrepo"
)

func TestHistory(t *testing.T) {
	r, tr := open(t)
	h, err := r.History(10)
	if err != nil || len(h) != 3 {
		t.Fatalf("%d %v", len(h), err)
	}
	if h[0].SHA != tr.Init || h[2].SHA != tr.Fix || h[0].Parents != 0 || h[1].Parents != 1 || h[1].Author != "Dev" {
		t.Errorf("order and fields: %+v", h)
	}
	if len(h[0].Files) != len(testrepo.Files) {
		t.Errorf("first commit files %d", len(h[0].Files))
	}
	if f := h[2].Files; len(f) != 1 || f[0].Path != testrepo.ServicePath || f[0].Added != 1 || f[0].Deleted != 0 {
		t.Errorf("fix stats %+v", f)
	}
	if _, err := (&Repo{Root: t.TempDir()}).History(1); err == nil {
		t.Error("outside a repository")
	}
}

func TestBlameAt(t *testing.T) {
	r, tr := open(t)
	shas := r.BlameAt(tr.Bug, testrepo.ServicePath, testrepo.BugLine, testrepo.BugLine+1)
	if len(shas) != 2 || shas[0] != tr.Bug || shas[1] != tr.Bug {
		t.Errorf("%v", shas)
	}
	if r.BlameAt(tr.Bug, "missing.java", 1, 1) != nil || isHex("xyz") {
		t.Error("missing file")
	}
}
