// Package testrepo builds a small Spring repository for tests: order-service calls payment-service
// through a Feign client. It is a real git repository with a known history:
//
//	init   both services, with a slow call in a loop that a full check finds
//	bug    OrderService.discount swallows an exception (empty catch at BugLine)
//	fix    the catch now rethrows, which resolves the issue
//
// and a branch feature/audit one commit ahead of fix that adds AuditService.
package testrepo

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	order   = "order-service/src/main/java/com/shop/order/"
	payment = "payment-service/src/main/java/com/shop/payment/"

	// ServicePath is the file the bug and fix commits change.
	ServicePath = order + "OrderService.java"
	// BugLine is the line of the empty catch in the bug commit.
	BugLine = 30
	// Branch is the extra branch.
	Branch = "feature/audit"
)

// Files is the code as of the first commit, by path.
var Files = map[string]string{
	"order-service/pom.xml":                                     "<project><artifactId>order-service</artifactId></project>\n",
	"payment-service/pom.xml":                                   "<project><artifactId>payment-service</artifactId></project>\n",
	"order-service/src/main/resources/application.yml":          "spring:\n  application:\n    name: order-service\nserver:\n  port: 8081\n",
	"payment-service/src/main/resources/application.properties": "spring.application.name=payment-service\nserver.port=8082\n",

	order + "OrderApplication.java": `package com.shop.order;

import org.springframework.boot.autoconfigure.SpringBootApplication;

@SpringBootApplication
public class OrderApplication {
    public static void main(String[] args) {
        System.exit(0);
    }
}
`,
	order + "OrderController.java": `package com.shop.order;

import org.springframework.web.bind.annotation.*;

@RestController
@RequestMapping("/orders")
public class OrderController {
    private final OrderService service;

    public OrderController(OrderService service) {
        this.service = service;
    }

    @GetMapping("/{id}")
    public Order get(@PathVariable Long id) {
        return service.find(id);
    }
}
`,
	ServicePath: serviceV1,
	order + "Order.java": `package com.shop.order;

public class Order {
    private Long id;
    private long total;

    public Long getId() { return id; }
    public long getTotal() { return total; }
    public void applyDiscount() { total = total * 9 / 10; }
}
`,
	order + "OrderRepository.java": `package com.shop.order;

import org.springframework.data.jpa.repository.JpaRepository;

public interface OrderRepository extends JpaRepository<Order, Long> {
}
`,
	order + "PaymentClient.java": `package com.shop.order;

import org.springframework.cloud.openfeign.FeignClient;
import org.springframework.web.bind.annotation.*;

@FeignClient(name = "payment-service", path = "/payments")
public interface PaymentClient {
    @PostMapping("/{orderId}")
    void charge(@PathVariable("orderId") Long orderId);
}
`,
	payment + "PaymentController.java": `package com.shop.payment;

import org.springframework.web.bind.annotation.*;

@RestController
@RequestMapping("/payments")
public class PaymentController {
    private final PaymentService service;

    public PaymentController(PaymentService service) {
        this.service = service;
    }

    @PostMapping("/{id}")
    public void charge(@PathVariable Long id) {
        service.charge(id);
    }
}
`,
	payment + "PaymentService.java": `package com.shop.payment;

import org.springframework.stereotype.Service;

@Service
public class PaymentService {
    public void charge(Long id) {
        if (id == null) {
            throw new IllegalArgumentException("no order");
        }
    }
}
`,
}

const serviceV1 = `package com.shop.order;

import org.springframework.stereotype.Service;
import java.util.List;

@Service
public class OrderService {
    private final OrderRepository orders;
    private final PaymentClient payments;

    public OrderService(OrderRepository orders, PaymentClient payments) {
        this.orders = orders;
        this.payments = payments;
    }

    public Order find(Long id) {
        Order order = orders.findById(id).orElse(null);
        return order;
    }

    public void payAll(List<Order> list) {
        for (Order o : list) {
            payments.charge(o.getId());
        }
    }
`

// ServiceBug is OrderService in the bug commit.
const ServiceBug = serviceV1 + `
    public void discount(Order order) {
        try {
            order.applyDiscount();
        } catch (Exception e) {
        }
    }
}
`

// ServiceFix is OrderService in the fix commit.
const ServiceFix = serviceV1 + `
    public void discount(Order order) {
        try {
            order.applyDiscount();
        } catch (Exception e) {
            throw new IllegalStateException(e);
        }
    }
}
`

// Repo is a built repository.
type Repo struct {
	Dir                 string
	Init, Bug, Fix, Ext string // commit ids; Ext is the commit on Branch
	t                   testing.TB
}

// New builds the repository in a temporary directory, checked out on main at Fix.
func New(t testing.TB) *Repo {
	t.Helper()
	r := &Repo{Dir: t.TempDir(), t: t}
	r.Git("init", "-q", "-b", "main")
	files := map[string]string{}
	for p, s := range Files {
		files[p] = s
	}
	files[ServicePath] = serviceV1 + "}\n"
	r.Commit("Add order and payment services", files)
	r.Init = r.Head()
	r.Commit("Add discount", map[string]string{ServicePath: ServiceBug})
	r.Bug = r.Head()
	r.Commit("Rethrow discount errors", map[string]string{ServicePath: ServiceFix})
	r.Fix = r.Head()
	r.Git("checkout", "-q", "-b", Branch)
	r.Commit("Add audit", map[string]string{order + "AuditService.java": "package com.shop.order;\n\npublic class AuditService {\n    public void log(Order o) {\n    }\n}\n"})
	r.Ext = r.Head()
	r.Git("checkout", "-q", "main")
	return r
}

// Git runs git in the repository and returns its output.
func (r *Repo) Git(args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", append([]string{"-C", r.Dir, "-c", "user.name=Dev", "-c", "user.email=dev@shop", "-c", "commit.gpgsign=false"}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %v: %s", args, out)
	}
	return strings.TrimSpace(string(out))
}

// Commit writes the files and commits them.
func (r *Repo) Commit(msg string, files map[string]string) {
	r.t.Helper()
	for p, s := range files {
		full := filepath.Join(r.Dir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			r.t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(s), 0o644); err != nil {
			r.t.Fatal(err)
		}
	}
	r.Git("add", "-A")
	r.Git("commit", "-q", "-m", msg)
}

// Head is the checked-out commit.
func (r *Repo) Head() string { return r.Git("rev-parse", "HEAD") }

// Sources is Files with only the Java files, as bytes, the way the indexer takes them.
func Sources() (java, other map[string][]byte) {
	java, other = map[string][]byte{}, map[string][]byte{}
	for p, s := range Files {
		if strings.HasSuffix(p, ".java") {
			java[p] = []byte(s)
		} else {
			other[p] = []byte(s)
		}
	}
	java[ServicePath] = []byte(ServiceFix)
	return java, other
}
