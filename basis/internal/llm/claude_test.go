package llm

import (
	"strings"
	"testing"
)

// splitBubbles is the full-text equivalent of the incremental splitting done
// during streaming. Used here to test correctness of the splitting algorithm.
func splitBubbles(text string) []string {
	var bubbles []string
	remaining := text
	for {
		idx := bubbleBoundary(remaining)
		if idx < 0 {
			break
		}
		if b := strings.TrimSpace(remaining[:idx]); b != "" {
			bubbles = append(bubbles, b)
		}
		remaining = remaining[idx+2:]
	}
	if b := strings.TrimSpace(remaining); b != "" {
		bubbles = append(bubbles, b)
	}
	return bubbles
}

func TestBubbles_BlankLineSplits(t *testing.T) {
	got := splitBubbles("first thought\n\nsecond thought\n\nthird")
	want := []string{"first thought", "second thought", "third"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestBubbles_KeepsCodeFenceWhole(t *testing.T) {
	in := "here's code:\n\n```go\nfunc main() {}\n\nprintln(1)\n```\n\ndone"
	got := splitBubbles(in)
	if len(got) != 3 {
		t.Fatalf("want 3 bubbles, got %d: %q", len(got), got)
	}
	if !strings.Contains(got[1], "```go") || !strings.Contains(got[1], "println(1)") {
		t.Fatalf("code fence was split across bubbles: %q", got[1])
	}
}

func TestBubbles_SingleBubbleNoBlankLine(t *testing.T) {
	got := splitBubbles("just one line, no blank lines here")
	if len(got) != 1 || got[0] != "just one line, no blank lines here" {
		t.Fatalf("want single bubble, got %q", got)
	}
}

func TestSplitSentences_ShortBubbleUntouched(t *testing.T) {
	in := "Two sentences. Still short though."
	got := splitSentences(in)
	if len(got) != 1 || got[0] != in {
		t.Fatalf("short bubble should pass through whole, got %q", got)
	}
}

func TestSplitSentences_LongWallBreaksPerSentence(t *testing.T) {
	in := "A dark future isn't necessarily fixed. Future sight reads probability distributions, not certainties. " +
		"If everything converges on darkness, that means the variables are heavily weighted in one direction."
	got := splitSentences(in)
	if len(got) != 3 {
		t.Fatalf("want 3 sentences, got %d: %q", len(got), got)
	}
	if !strings.HasPrefix(got[0], "A dark future") || !strings.HasPrefix(got[1], "Future sight") {
		t.Fatalf("split misaligned: %q", got)
	}
}

func TestSplitSentences_CodeFenceNeverSplit(t *testing.T) {
	in := "Here is a long explanation that runs well past the gate so the splitter would normally fire on it. " +
		"```go\nfunc main() {}\n```"
	got := splitSentences(in)
	if len(got) != 1 {
		t.Fatalf("code-fenced bubble must stay whole, got %d: %q", len(got), got)
	}
}

func TestSplitSentences_GuardsDecimalsAndAbbrev(t *testing.T) {
	in := "The model weighs probability at 3.5 against the baseline e.g. the prior run we discussed at length. " +
		"That gap is what matters here, nothing else does."
	got := splitSentences(in)
	for _, p := range got {
		if strings.HasSuffix(p, "3.") || strings.HasSuffix(p, "e.g.") {
			t.Fatalf("split on a decimal or abbreviation: %q", got)
		}
	}
}

func TestBubbleBoundary_OutsideFenceOnly(t *testing.T) {
	if got := bubbleBoundary("ab\n\ncd"); got != 2 {
		t.Fatalf("plain boundary: got %d want 2", got)
	}
	if got := bubbleBoundary("```\n\n```"); got != -1 {
		t.Fatalf("boundary inside fence should be ignored: got %d want -1", got)
	}
	if got := bubbleBoundary("no boundary here"); got != -1 {
		t.Fatalf("no boundary: got %d want -1", got)
	}
}
