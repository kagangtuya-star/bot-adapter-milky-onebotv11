package bridge

import (
	"testing"

	"milky-onebot11-bridge/internal/types"
)

func TestParseOneBotMessageArray(t *testing.T) {
	raw := []byte(`[{"type":"text","data":{"text":"hello"}},{"type":"at","data":{"qq":"12345"}},{"type":"image","data":{"file":"http://example.com/a.png"}}]`)
	segments, err := parseOneBotMessage(raw, false)
	if err != nil {
		t.Fatalf("parseOneBotMessage returned error: %v", err)
	}
	if len(segments) != 3 {
		t.Fatalf("expected 3 segments, got %d", len(segments))
	}
	if segments[0].Type != "text" || segments[0].Data["text"] != "hello" {
		t.Fatalf("unexpected text segment: %#v", segments[0])
	}
	if segments[1].Type != "at" || segments[1].Data["qq"] != "12345" {
		t.Fatalf("unexpected at segment: %#v", segments[1])
	}
	if segments[2].Type != "image" || segments[2].Data["url"] != "http://example.com/a.png" {
		t.Fatalf("unexpected image segment: %#v", segments[2])
	}
}

func TestParseOneBotMessageCQ(t *testing.T) {
	raw := []byte(`"[CQ:at,qq=42]abc[CQ:reply,id=100]"`)
	segments, err := parseOneBotMessage(raw, false)
	if err != nil {
		t.Fatalf("parseOneBotMessage returned error: %v", err)
	}
	if len(segments) != 3 {
		t.Fatalf("expected 3 segments, got %d", len(segments))
	}
	if segments[0].Type != "at" || segments[0].Data["qq"] != "42" {
		t.Fatalf("unexpected first segment: %#v", segments[0])
	}
	if segments[1].Type != "text" || segments[1].Data["text"] != "abc" {
		t.Fatalf("unexpected second segment: %#v", segments[1])
	}
	if segments[2].Type != "reply" || segments[2].Data["id"] != "100" {
		t.Fatalf("unexpected third segment: %#v", segments[2])
	}
}

func TestBuildOneBotMessageString(t *testing.T) {
	out, raw := buildOneBotMessage("string", []types.Segment{
		{Type: types.SegmentText, Data: map[string]string{"text": "a&b"}},
		{Type: types.SegmentAt, Data: map[string]string{"qq": "7"}},
	})
	s, ok := out.(string)
	if !ok {
		t.Fatalf("expected string output")
	}
	if s != "a&amp;b[CQ:at,qq=7]" {
		t.Fatalf("unexpected string output: %s", s)
	}
	if raw != s {
		t.Fatalf("expected raw to equal string output")
	}
}
