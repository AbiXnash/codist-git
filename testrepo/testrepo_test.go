package testrepo

import (
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	r := New(t)
	if r.Head() != r.Fix {
		t.Errorf("checked out %s, want the fix %s", r.Head(), r.Fix)
	}
	if got := r.Git("rev-parse", Branch); got != r.Ext {
		t.Errorf("%s is at %s, want %s", Branch, got, r.Ext)
	}
	if log := r.Git("log", "--format=%s"); log != "Rethrow discount errors\nAdd discount\nAdd order and payment services" {
		t.Errorf("history:\n%s", log)
	}
	bug := strings.Split(r.Git("show", r.Bug+":"+ServicePath), "\n")
	if strings.TrimSpace(bug[BugLine-1]) != "} catch (Exception e) {" {
		t.Errorf("line %d of the bug is %q", BugLine, bug[BugLine-1])
	}
	java, other := Sources()
	if len(java)+len(other) != len(Files) || string(java[ServicePath]) != ServiceFix {
		t.Errorf("sources: %d java, %d other", len(java), len(other))
	}
}
