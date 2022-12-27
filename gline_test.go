package main

import "testing"

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
