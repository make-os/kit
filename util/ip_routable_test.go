// Copyright (c) 2013-2014 The btcsuite developers
// Copyright (c) 2015-2016 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package util

import (
	"net"
	"testing"
)

// TestIPTypes ensures the various functions which determine the type of an IP
// address based on RFCs work as intended.
func TestIPTypes(t *testing.T) {
	type ipTest struct {
		in       net.IP
		rfc1918  bool
		rfc2544  bool
		rfc3849  bool
		rfc3927  bool
		rfc3964  bool
		rfc4193  bool
		rfc4380  bool
		rfc4843  bool
		rfc4862  bool
		rfc5737  bool
		rfc6052  bool
		rfc6145  bool
		rfc6598  bool
		local    bool
		valid    bool
		routable bool
		dev      bool
	}

	newIPTest := func(ip string, rfc1918, rfc2544, rfc3849, rfc3927, rfc3964,
		rfc4193, rfc4380, rfc4843, rfc4862, rfc5737, rfc6052, rfc6145, rfc6598,
		local, valid, routable, dev bool) ipTest {
		nip := net.ParseIP(ip)
		test := ipTest{nip, rfc1918, rfc2544, rfc3849, rfc3927, rfc3964, rfc4193, rfc4380,
			rfc4843, rfc4862, rfc5737, rfc6052, rfc6145, rfc6598, local, valid, routable, dev}
		return test
	}

	tests := []ipTest{
		newIPTest("10.255.255.255", true, false, false, false, false, false,
			false, false, false, false, false, false, false, false, true, false, true),
		newIPTest("192.168.0.1", true, false, false, false, false, false,
			false, false, false, false, false, false, false, false, true, false, true),
		newIPTest("172.31.255.1", true, false, false, false, false, false,
			false, false, false, false, false, false, false, false, true, false, true),
		newIPTest("172.16.238.10", true, false, false, false, false, false,
			false, false, false, false, false, false, false, false, true, false, true),
		newIPTest("172.32.1.1", false, false, false, false, false, false, false, false,
			false, false, false, false, false, false, true, true, false),
		newIPTest("169.254.250.120", false, false, false, true, false, false,
			false, false, false, false, false, false, false, false, true, false, false),
		newIPTest("0.0.0.0", false, false, false, false, false, false, false,
			false, false, false, false, false, false, true, false, false, false),
		newIPTest("255.255.255.255", false, false, false, false, false, false,
			false, false, false, false, false, false, false, false, false, false, false),
		newIPTest("127.0.0.1", false, false, false, false, false, false,
			false, false, false, false, false, false, false, true, true, false, true),
		newIPTest("fd00:dead::1", false, false, false, false, false, true,
			false, false, false, false, false, false, false, false, true, false, false),
		newIPTest("2001::1", false, false, false, false, false, false,
			true, false, false, false, false, false, false, false, true, true, false),
		newIPTest("2001:10:abcd::1:1", false, false, false, false, false, false,
			false, true, false, false, false, false, false, false, true, false, false),
		newIPTest("fe80::1", false, false, false, false, false, false,
			false, false, true, false, false, false, false, false, true, false, false),
		newIPTest("fe80:1::1", false, false, false, false, false, false,
			false, false, false, false, false, false, false, false, true, true, false),
		newIPTest("64:ff9b::1", false, false, false, false, false, false,
			false, false, false, false, true, false, false, false, true, true, false),
		newIPTest("::ffff:abcd:ef12:1", false, false, false, false, false, false,
			false, false, false, false, false, false, false, false, true, true, false),
		newIPTest("::1", false, false, false, false, false, false, false, false,
			false, false, false, false, false, true, true, false, true),
		newIPTest("198.18.0.1", false, true, false, false, false, false, false,
			false, false, false, false, false, false, false, true, false, false),
		newIPTest("100.127.255.1", false, false, false, false, false, false, false,
			false, false, false, false, false, true, false, true, false, false),
		newIPTest("203.0.113.1", false, false, false, false, false, false, false,
			false, false, false, false, false, false, false, true, false, false),
	}

	t.Logf("Running %d tests", len(tests))
	for _, test := range tests {
		if rv := isRFC1918(test.in); rv != test.rfc1918 {
			t.Errorf("isRFC1918 %s\n got: %v want: %v", test.in, rv, test.rfc1918)
		}

		if rv := isRFC3849(test.in); rv != test.rfc3849 {
			t.Errorf("isRFC3849 %s\n got: %v want: %v", test.in, rv, test.rfc3849)
		}

		if rv := isRFC3927(test.in); rv != test.rfc3927 {
			t.Errorf("isRFC3927 %s\n got: %v want: %v", test.in, rv, test.rfc3927)
		}

		if rv := isRFC3964(test.in); rv != test.rfc3964 {
			t.Errorf("isRFC3964 %s\n got: %v want: %v", test.in, rv, test.rfc3964)
		}

		if rv := isRFC4193(test.in); rv != test.rfc4193 {
			t.Errorf("isRFC4193 %s\n got: %v want: %v", test.in, rv, test.rfc4193)
		}

		if rv := isRFC4380(test.in); rv != test.rfc4380 {
			t.Errorf("isRFC4380 %s\n got: %v want: %v", test.in, rv, test.rfc4380)
		}

		if rv := isRFC4843(test.in); rv != test.rfc4843 {
			t.Errorf("isRFC4843 %s\n got: %v want: %v", test.in, rv, test.rfc4843)
		}

		if rv := isRFC4862(test.in); rv != test.rfc4862 {
			t.Errorf("isRFC4862 %s\n got: %v want: %v", test.in, rv, test.rfc4862)
		}

		if rv := isRFC6052(test.in); rv != test.rfc6052 {
			t.Errorf("isRFC6052 %s\n got: %v want: %v", test.in, rv, test.rfc6052)
		}

		if rv := isRFC6145(test.in); rv != test.rfc6145 {
			t.Errorf("isRFC1918 %s\n got: %v want: %v", test.in, rv, test.rfc6145)
		}

		if rv := isLocal(test.in); rv != test.local {
			t.Errorf("isLocal %s\n got: %v want: %v", test.in, rv, test.local)
		}

		if rv := isValid(test.in); rv != test.valid {
			t.Errorf("IsValid %s\n got: %v want: %v", test.in, rv, test.valid)
		}

		if rv := IsRoutable(test.in); rv != test.routable {
			t.Errorf("IsRoutable %s\n got: %v want: %v", test.in, rv, test.routable)
		}

		if rv := IsDevAddr(test.in); rv != test.dev {
			t.Errorf("IsDevAddr %s\n got: %v want: %v", test.in, rv, test.dev)
		}
	}
}
