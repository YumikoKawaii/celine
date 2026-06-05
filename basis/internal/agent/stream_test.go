package agent

import (
	"context"
	"strings"
	"testing"
)

type collectSink struct{ bubbles []string }

func (c *collectSink) Typing(int32) error                        { return nil }
func (c *collectSink) Bubble(_ int32, text string) error         { c.bubbles = append(c.bubbles, text); return nil }
func (c *collectSink) ToolCall(_, _, _ string) error             { return nil }
func (c *collectSink) ToolResult(_, _ string, _ bool) error      { return nil }

func feed(t *testing.T, s string) []string {
	t.Helper()
	deltas := make(chan string, 8)
	go func() {
		for _, r := range s {
			deltas <- string(r)
		}
		close(deltas)
	}()
	sink := &collectSink{}
	noDelay := func(string) int32 { return 0 }
	if err := paceBubblesWith(context.Background(), deltas, sink, noDelay, 0); err != nil {
		t.Fatalf("paceBubbles: %v", err)
	}
	return sink.bubbles
}

func TestSegment_BlankLineSplits(t *testing.T) {
	got := feed(t, "first thought\n\nsecond thought\n\nthird")
	want := []string{"first thought", "second thought", "third"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestSegment_KeepsCodeFenceWhole(t *testing.T) {
	in := "here's code:\n\n```go\nfunc main() {}\n\nprintln(1)\n```\n\ndone"
	got := feed(t, in)
	if len(got) != 3 {
		t.Fatalf("want 3 bubbles, got %d: %q", len(got), got)
	}
	if !strings.Contains(got[1], "```go") || !strings.Contains(got[1], "println(1)") {
		t.Fatalf("code fence was split across bubbles: %q", got[1])
	}
}

func TestSegment_SingleBubbleNoBlankLine(t *testing.T) {
	got := feed(t, "just one line, no blank lines here")
	if len(got) != 1 || got[0] != "just one line, no blank lines here" {
		t.Fatalf("want single bubble, got %q", got)
	}
}

func TestFirstBoundary_OutsideFenceOnly(t *testing.T) {
	if got := firstBoundary("ab\n\ncd"); got != 2 {
		t.Fatalf("plain boundary: got %d want 2", got)
	}
	if got := firstBoundary("```\n\n```"); got != -1 {
		t.Fatalf("boundary inside fence should be ignored: got %d want -1", got)
	}
	if got := firstBoundary("no boundary here"); got != -1 {
		t.Fatalf("no boundary: got %d want -1", got)
	}
}
