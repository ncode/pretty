package jobs

import "testing"

func TestParseSentinel(t *testing.T) {
	line := "__PRETTY_EXIT__42:0"
	jobID, exitCode, ok := ParseSentinel(line)
	if !ok {
		t.Fatalf("expected sentinel parse ok")
	}
	if jobID != 42 || exitCode != 0 {
		t.Fatalf("unexpected values: jobID=%d exit=%d", jobID, exitCode)
	}
}

func TestParseSentinel_NoMatch(t *testing.T) {
	_, _, ok := ParseSentinel("whoami")
	if ok {
		t.Fatalf("expected non-sentinel line to return ok=false")
	}
}

func TestExtractSentinelInline(t *testing.T) {
	prefix, jobID, exitCode, ok := ExtractSentinel("whoami" + SentinelFor(3) + ":0")
	if !ok {
		t.Fatalf("expected inline sentinel parse")
	}
	if prefix != "whoami" || jobID != 3 || exitCode != 0 {
		t.Fatalf("unexpected values: prefix=%q jobID=%d exit=%d", prefix, jobID, exitCode)
	}
}

func TestExtractSentinelOnly(t *testing.T) {
	prefix, jobID, exitCode, ok := ExtractSentinel(SentinelFor(7) + ":1")
	if !ok {
		t.Fatalf("expected sentinel parse")
	}
	if prefix != "" || jobID != 7 || exitCode != 1 {
		t.Fatalf("unexpected values: prefix=%q jobID=%d exit=%d", prefix, jobID, exitCode)
	}
}

func TestExtractSentinelRejectTrailingText(t *testing.T) {
	_, _, _, ok := ExtractSentinel(SentinelFor(7) + ":0 extra")
	if ok {
		t.Fatalf("expected reject with trailing text")
	}
}

func BenchmarkExtractSentinel(b *testing.B) {
	line := "whoami" + SentinelFor(42) + ":0"
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ExtractSentinel(line)
	}
}
