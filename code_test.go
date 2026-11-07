package git

import (
	"strings"
	"testing"

	"github.com/AbiXnash/codist-git/testrepo"
)

func TestIsCode(t *testing.T) {
	for line, want := range map[string]bool{
		"  return a + b;": true, "": false, "   ": false, "// note": false, "/** Doc": false, " * more doc": false,
		" */": false, "import java.util.List;": false, "package shop;": false, "}": true,
	} {
		if IsCode(line) != want {
			t.Errorf("%q", line)
		}
	}
}

func TestCodeChurn(t *testing.T) {
	r, tr := open(t)
	path := "order-service/src/main/java/com/shop/order/Order.java"
	src := strings.Replace(testrepo.Files[path], "public class Order {", "/**\n * An order.\n * More words.\n */\npublic class Order {", 1)
	tr.Commit("Document orders", map[string]string{path: src})
	doc := r.Head()
	churn, err := r.CodeChurn(10)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := churn[doc]; ok {
		t.Errorf("a javadoc-only commit has no code lines: %v", churn[doc])
	}
	if c := churn[tr.Fix][testrepo.ServicePath]; c != [2]int{1, 0} {
		t.Errorf("the fix adds one line of code: %v", c)
	}
	if c := churn[tr.Init][testrepo.ServicePath]; c[0] < 15 || c[1] != 0 {
		t.Errorf("first commit: %v", c)
	}
	if _, err := (&Repo{Root: t.TempDir()}).CodeChurn(1); err == nil {
		t.Error("outside a repository")
	}
}

func TestCodeLines(t *testing.T) {
	src := `package shop;
import java.util.List;
/**
 Finds elements, written the jsoup way: no stars.
 <p>Call {@link Element#select(String)}.</p>
 */
class A {
    // a note
    String url = "http://x"; // a // inside a string is not a comment
    String block = """
        not a comment: /* still text */
        """;
    char slash = '/'; /* trailing */
    /* one line */ int b = 2;

    int c() { return 1; }
}`
	got := CodeLines([]byte(src))
	want := []bool{false, false, false, false, false, false, true, false, true, true, true, true, true, true, false, true, true}
	if len(got) != len(want) {
		t.Fatalf("%d lines, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d (%q): got %v", i+1, strings.Split(src, "\n")[i], got[i])
		}
	}
	if CountCode(got, []int{3, 4, 7, 99, 0}) != 1 {
		t.Error("count")
	}
}

// jsoup writes javadoc paragraphs without a leading star; those lines are not code either.
func TestCodeChurnStarlessJavadoc(t *testing.T) {
	r, tr := open(t)
	path := "order-service/src/main/java/com/shop/order/Order.java"
	src := strings.Replace(testrepo.Files[path], "public class Order {", "/**\n An order a customer placed.\n <p>Totals are in cents.</p>\n */\npublic class Order {", 1)
	tr.Commit("Document orders", map[string]string{path: src})
	doc := r.Head()
	tr.Git("rm", "-q", "order-service/src/main/java/com/shop/order/OrderApplication.java")
	tr.Commit("Remove the application class", nil)
	removed := r.Head()
	churn, err := r.CodeChurn(10)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := churn[doc]; ok {
		t.Errorf("a javadoc-only commit has no code lines: %v", churn[doc])
	}
	if c := churn[removed]["order-service/src/main/java/com/shop/order/OrderApplication.java"]; c[0] != 0 || c[1] < 4 {
		t.Errorf("a deleted file counts its code lines as deleted: %v", c)
	}
}
