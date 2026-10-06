package main

import "testing"

// A missing or garbled seqpacket:// argument must surface as fd < 0 so
// main() fatals — never silently run the engine on fd 0 (stdin).
func TestParseArgsFD(t *testing.T) {
	// Normal UML invocation.
	descr, fd, vnl := parseArgs([]string{"--descr", "vm1", "seqpacket://7", "slirp://,switch"})
	if descr != "vm1" || fd != 7 || vnl != "slirp://,switch" {
		t.Fatalf("got descr=%q fd=%d vnl=%q", descr, fd, vnl)
	}
	// --descr=VALUE form.
	descr, _, _ = parseArgs([]string{"--descr=vm2", "seqpacket://3"})
	if descr != "vm2" {
		t.Fatalf("inline descr: %q", descr)
	}
	// No seqpacket at all.
	if _, fd, _ := parseArgs([]string{"--descr", "vm3"}); fd >= 0 {
		t.Fatalf("missing seqpacket: fd=%d, want <0", fd)
	}
	// Garbled fd.
	if _, fd, _ := parseArgs([]string{"seqpacket://banana"}); fd >= 0 {
		t.Fatalf("garbled seqpacket: fd=%d, want <0", fd)
	}
}
