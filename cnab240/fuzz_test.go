package cnab240

import (
	"reflect"
	"strings"
	"testing"
)

func FuzzDecodeSegment(f *testing.F) {
	for _, s := range []Segment{&SegmentP{}, &SegmentQ{}, &SegmentR{}, &SegmentT{}, &SegmentU{}} {
		line, err := EncodeSegment(s)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(line)
	}
	f.Add(strings.Repeat(" ", LineLength))
	f.Fuzz(func(t *testing.T, line string) {
		s, err := DecodeSegment(line)
		if err != nil {
			return
		}
		enc, err := EncodeSegment(s)
		if err != nil {
			return // lowercase or non-ASCII alfa content is rejected on encode
		}
		s2, err := DecodeSegment(enc)
		if err != nil {
			t.Fatalf("DecodeSegment(EncodeSegment(s)): %v", err)
		}
		if enc2, _ := EncodeSegment(s2); enc2 != enc || !reflect.DeepEqual(s2, mustDecode(t, enc2)) {
			t.Fatal("encoding not idempotent")
		}
	})
}

func mustDecode(t *testing.T, line string) Segment {
	s, err := DecodeSegment(line)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
