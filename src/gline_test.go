package ircglineapi

import (
	"net"
	"testing"
)

func TestIs_valid_ipValid(t *testing.T) {
	ip := "1.2.3.4"
	want := true
	res := Is_valid_ip(ip)
	if res != want {
		t.Fatalf(`Is_valid_ip(%s) = %t. Want %t`, ip, res, want)
	}
}

func TestIs_valid_ip6Valid(t *testing.T) {
	ip := "a:bcd:123::2"
	want := true
	res := Is_valid_ip(ip)
	if res != want {
		t.Fatalf(`Is_valid_ip(%s) = %t. Want %t`, ip, res, want)
	}
}

func TestIs_valid_ipInvalid(t *testing.T) {
	ip := "1.2.3.4.5"
	want := false
	res := Is_valid_ip(ip)
	if res != want {
		t.Fatalf(`Is_valid_ip(%s) = %t. Want %t`, ip, res, want)
	}
}

func TestIs_valid_ip6Invalid(t *testing.T) {
	ip := "g:bcd:123"
	want := false
	res := Is_valid_ip(ip)
	if res != want {
		t.Fatalf(`Is_valid_ip(%s) = %t. Want %t`, ip, res, want)
	}
}

func TestIs_valid_cidrValid(t *testing.T) {
	cases := []string{
		"1.2.3.0/24",
		"2607:1:2:3::/64",
		"::/128",
	}
	want := true
	for _, c := range cases {
		res := Is_valid_cidr(c)
		if res != want {
			t.Fatalf(`Is_valid_cidr(%s) = %t. Want %t`, c, res, want)
		}
	}
}

func TestIs_valid_cidrInvalid(t *testing.T) {
	cases := []string{
		"1.2.3.0/33",
		"2607:1:2:3aaaa::/32",
		"::/129",
	}
	want := false
	for _, c := range cases {
		res := Is_valid_cidr(c)
		if res != want {
			t.Fatalf(`Is_valid_cidr(%s) = %t. Want %t`, c, res, want)
		}
	}
}

/*func TestIs_valid_cidrInvalid2(t *testing.T) {
	var ones [129]byte
	t.Fatalf("%#v", ones)
}*/

func TestParseGlineIDPresent(t *testing.T) {
	reason := "AUTO [1] You were identified as a drone. Visit https://glines.undernet.org?ip=152.231.15.130 for removal. (P327) - ID: D1785006545-2001"
	want := "D1785006545-2001"
	if res := parseGlineID(reason); res != want {
		t.Fatalf(`parseGlineID(%q) = %q. Want %q`, reason, res, want)
	}
}

func TestParseGlineIDAbsent(t *testing.T) {
	reason := "AUTO [0] test"
	if res := parseGlineID(reason); res != "" {
		t.Fatalf(`parseGlineID(%q) = %q. Want ""`, reason, res)
	}
}

func TestIsGlineIDFormat(t *testing.T) {
	cases := []struct {
		s    string
		want bool
	}{
		{"D1785006545-2001", true},
		{"1.2.3.4", false},
		{"a:bcd:123::2", false},
		{"1.2.3.0/24", false},
	}
	for _, c := range cases {
		if res := IsGlineIDFormat(c.s); res != c.want {
			t.Errorf(`IsGlineIDFormat(%q) = %t. Want %t`, c.s, res, c.want)
		}
	}
}

func TestGlineDataUpdatePreservesIDWhenReasonEmpty(t *testing.T) {
	g := newGlineData(net.IPNet{}, "user", "*@1.2.3.4", 1000, 1000, "reason - ID: D111-222", true)
	active := true
	g.Update(&active, 2000, "")
	if g.ID() != "D111-222" {
		t.Fatalf(`g.ID() = %q after empty-reason Update. Want "D111-222" (unchanged)`, g.ID())
	}
}

func TestGlineDataUpdateResetsIDWhenNewReasonHasID(t *testing.T) {
	g := newGlineData(net.IPNet{}, "user", "*@1.2.3.4", 1000, 1000, "reason1 - ID: AAA-1", true)
	active := true
	g.Update(&active, 2000, "reason2 - ID: BBB-2")
	if g.ID() != "BBB-2" {
		t.Fatalf(`g.ID() = %q after Update with new ID. Want "BBB-2"`, g.ID())
	}
}

func TestGlineDataUpdateKeepsOldIDWhenNewReasonHasNoID(t *testing.T) {
	g := newGlineData(net.IPNet{}, "user", "*@1.2.3.4", 1000, 1000, "reason1 - ID: AAA-1", true)
	active := true
	g.Update(&active, 2000, "reason2 without any id suffix")
	if g.ID() != "AAA-1" {
		t.Fatalf(`g.ID() = %q after Update whose new reason has no ID. Want "AAA-1" (preserved)`, g.ID())
	}
}

func TestGlineDataClone(t *testing.T) {
	g := newGlineData(net.IPNet{}, "user", "*@1.2.3.4", 1000, 1000, "reason - ID: D111-222", true)
	clone := g.Clone()
	active := false
	g.Update(&active, 2000, "changed reason - ID: D999-999")
	if clone.ID() != "D111-222" {
		t.Errorf(`clone.ID() = %q after mutating original. Want "D111-222" (unchanged)`, clone.ID())
	}
	if clone.active != true {
		t.Errorf(`clone.active = %t after mutating original. Want true (unchanged)`, clone.active)
	}
	if clone.reason != "reason - ID: D111-222" {
		t.Errorf(`clone.reason = %q after mutating original. Want unchanged`, clone.reason)
	}
}
