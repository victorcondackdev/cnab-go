package cnab240

import (
	"testing"

	"github.com/victorcondackdev/cnab-go/fixedwidth"
)

func TestLayouts(t *testing.T) {
	for _, v := range []any{FileHeader{}, BatchHeader{}, SegmentP{}, SegmentQ{}, SegmentR{}, SegmentT{}, SegmentU{}, BatchTrailer{}, FileTrailer{}} {
		if err := fixedwidth.Check(v, LineLength); err != nil {
			t.Errorf("%T: %v", v, err)
		}
	}
}
