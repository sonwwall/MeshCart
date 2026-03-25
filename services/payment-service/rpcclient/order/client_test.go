package order

import "testing"

func TestShouldBuildDirectFallback(t *testing.T) {
	if !shouldBuildDirectFallback("consul", "127.0.0.1:8892") {
		t.Fatal("expected consul discovery with hostPort to build direct fallback")
	}
	if shouldBuildDirectFallback("direct", "127.0.0.1:8892") {
		t.Fatal("did not expect direct discovery to build fallback")
	}
	if shouldBuildDirectFallback("consul", "") {
		t.Fatal("did not expect empty hostPort to build fallback")
	}
}

func TestShouldFallbackToDirect(t *testing.T) {
	if !shouldFallbackToDirect(assertErr("service discovery error: no service found")) {
		t.Fatal("expected no service found to trigger direct fallback")
	}
	if !shouldFallbackToDirect(assertErr("service discovery error[retried 0]: no service found")) {
		t.Fatal("expected retried no service found to trigger direct fallback")
	}
	if shouldFallbackToDirect(assertErr("rpc timeout")) {
		t.Fatal("did not expect generic rpc timeout to trigger direct fallback")
	}
}

func assertErr(msg string) error { return testErr(msg) }

type testErr string

func (e testErr) Error() string { return string(e) }
