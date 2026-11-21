package docid

import "testing"

func TestClassify(t *testing.T) {
	tests := []struct {
		in   string
		want Kind
	}{
		{"52998224725", CPF},
		{"52998224724", Invalid},
		{"11111111111", Invalid},
		{"11222333000181", CNPJ},
		{"11222333000182", Invalid},
		{"00000000000000", Invalid},
		{"1122233300018a", Invalid},
		{"123", Invalid},
		{"", Invalid},
	}
	for _, tt := range tests {
		if got := Classify(tt.in); got != tt.want {
			t.Errorf("Classify(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}
