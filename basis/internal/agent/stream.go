package agent

import (
	"context"
	"strings"
	"time"
)

const (
	typingBaseMs  = 400
	typingPerChar = 25
	typingMinMs   = 400
	typingMaxMs   = 2500
	interBubbleMs = 500
)

func typingDelayMs(s string) int32 {
	d := typingBaseMs + typingPerChar*len(s)
	if d < typingMinMs {
		d = typingMinMs
	}
	if d > typingMaxMs {
		d = typingMaxMs
	}
	return int32(d)
}

func paceBubbles(ctx context.Context, deltas <-chan string, sink EventSink) error {
	return paceBubblesWith(ctx, deltas, sink, typingDelayMs, interBubbleMs*time.Millisecond)
}

func paceBubblesWith(
	ctx context.Context,
	deltas <-chan string,
	sink EventSink,
	typingDelay func(string) int32,
	interBubble time.Duration,
) error {
	var buf strings.Builder
	var seq int32

	emit := func(text string) error {
		text = strings.TrimSpace(text)
		if text == "" {
			return nil
		}
		delay := typingDelay(text)
		if err := sink.Typing(delay); err != nil {
			return err
		}
		if err := sleep(ctx, time.Duration(delay)*time.Millisecond); err != nil {
			return err
		}
		if err := sink.Bubble(seq, text); err != nil {
			return err
		}
		seq++
		return sleep(ctx, interBubble)
	}

	flushComplete := func() error {
		for {
			s := buf.String()
			idx := firstBoundary(s)
			if idx < 0 {
				return nil
			}
			bubble := s[:idx]
			buf.Reset()
			buf.WriteString(s[idx+2:])
			if err := emit(bubble); err != nil {
				return err
			}
		}
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case d, ok := <-deltas:
			if !ok {
				return emit(buf.String())
			}
			buf.WriteString(d)
			if err := flushComplete(); err != nil {
				return err
			}
		}
	}
}

func firstBoundary(s string) int {
	inFence := false
	for i := 0; i+1 < len(s); i++ {
		if strings.HasPrefix(s[i:], "```") {
			inFence = !inFence
			i += 2
			continue
		}
		if !inFence && s[i] == '\n' && s[i+1] == '\n' {
			return i
		}
	}
	return -1
}

func sleep(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}
