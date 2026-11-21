package fixedwidth

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

type rec struct {
	Tipo   string    `cnab:"1,1,const=3"`
	Codigo string    `cnab:"2-4,num"`
	Qtd    int       `cnab:"5,3,num"`
	Nome   string    `cnab:"8,10,alfa"`
	Data   time.Time `cnab:"18,8,date"`
	Data6  time.Time `cnab:"26,6,date"`
	Valor  int64     `cnab:"32,10,money"`
	Livre  string    `cnab:"42,4,raw"`
	Small  uint8     `cnab:"46,2,num"`
	Ignore string
	Skip   string `cnab:"-"`
}

const recLen = 47

func d(y, m, dd int) time.Time { return time.Date(y, time.Month(m), dd, 0, 0, 0, 0, time.UTC) }

func TestMarshal(t *testing.T) {
	v := rec{Codigo: "41", Qtd: 7, Nome: "São João", Data: d(2026, 12, 31), Data6: d(2026, 1, 2), Valor: 12345, Livre: "ab", Small: 9}
	got, err := Marshal(v, recLen)
	if err != nil {
		t.Fatal(err)
	}
	want := "3041007SAO JOAO  31122026020126000001234" + "5ab  09"
	if got != want {
		t.Fatalf("Marshal =\n%q\nwant\n%q", got, want)
	}
	// pointer works too
	if got2, err := Marshal(&v, recLen); err != nil || got2 != got {
		t.Fatalf("Marshal(&v) = %q, %v", got2, err)
	}
}

func TestRoundTrip(t *testing.T) {
	want := rec{Tipo: "3", Codigo: "041", Qtd: 7, Nome: "SAO JOAO", Data: d(2026, 12, 31), Data6: d(1999, 1, 2), Valor: 9999999999, Livre: "ab  ", Small: 99}
	line, err := Marshal(want, recLen)
	if err != nil {
		t.Fatal(err)
	}
	var got rec
	if err := Unmarshal(line, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip:\n got %+v\nwant %+v", got, want)
	}
}

func TestUnmarshalBlanksAndZeros(t *testing.T) {
	line := "3   " + "   " + strings.Repeat(" ", 10) + "00000000" + "      " + strings.Repeat(" ", 10) + "    " + "  "
	var got rec
	if err := Unmarshal(line, &got); err != nil {
		t.Fatal(err)
	}
	if got.Codigo != "" || got.Qtd != 0 || !got.Data.IsZero() || !got.Data6.IsZero() || got.Valor != 0 || got.Livre != "    " {
		t.Fatalf("unexpected %+v", got)
	}
}

func TestFieldErrors(t *testing.T) {
	base, err := Marshal(rec{}, recLen)
	if err != nil {
		t.Fatal(err)
	}
	set := func(pos int, s string) string { return base[:pos-1] + s + base[pos-1+len(s):] }
	tests := []struct {
		name  string
		line  string
		field string
		err   error
	}{
		{"const", set(1, "4"), "Tipo", ErrConst},
		{"digits", set(2, "4A1"), "Codigo", ErrDigits},
		{"int digits", set(5, "1 2"), "Qtd", ErrDigits},
		{"date", set(18, "31022026"), "Data", ErrDate},
		{"date chars", set(18, "3102202X"), "Data", ErrDate},
		{"money", set(32, "12345678-0"), "Valor", ErrDigits},
		{"overflow uint8", set(46, "99"), "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var v rec
			err := Unmarshal(tt.line, &v)
			if tt.err == nil {
				if err != nil {
					t.Fatalf("unexpected %v", err)
				}
				return
			}
			var fe *FieldError
			if !errors.As(err, &fe) || !errors.Is(err, tt.err) || fe.Field != tt.field {
				t.Fatalf("got %v, want %v on %s", err, tt.err, tt.field)
			}
			if !strings.Contains(fe.Error(), tt.field) {
				t.Errorf("message %q lacks field name", fe.Error())
			}
		})
	}
	var v rec
	if err := Unmarshal(base[:10], &v); !errors.Is(err, ErrLineLength) {
		t.Errorf("short line: %v", err)
	}
	fe := &FieldError{Line: 3, Field: "X", Start: 1, End: 2, Reason: "bad"}
	if got := fe.Error(); got != "cnab: line 3: field X (1-2): bad" {
		t.Errorf("Error() = %q", got)
	}
}

func TestMarshalErrors(t *testing.T) {
	tests := []struct {
		name string
		v    rec
		err  error
	}{
		{"const", rec{Tipo: "X"}, ErrConst},
		{"digits", rec{Codigo: "4a"}, ErrDigits},
		{"num overflow", rec{Codigo: "1234"}, ErrOverflow},
		{"int overflow", rec{Qtd: 1000}, ErrOverflow},
		{"negative", rec{Qtd: -1}, ErrNegative},
		{"alfa long", rec{Nome: "ABCDEFGHIJK"}, ErrOverflow},
		{"alfa charset", rec{Nome: "日本"}, ErrCharset},
		{"alfa control", rec{Nome: "A\tB"}, ErrCharset},
		{"raw newline", rec{Livre: "a\nb"}, ErrCharset},
		{"raw long", rec{Livre: "abcde"}, ErrOverflow},
		{"date year", rec{Data: time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)}, ErrDate},
		{"date6 year", rec{Data6: d(2070, 1, 1)}, ErrDate},
		{"money negative", rec{Valor: -5}, ErrNegative},
		{"uint8 overflow", rec{Small: 100}, ErrOverflow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Marshal(tt.v, recLen)
			if !errors.Is(err, tt.err) {
				t.Fatalf("got %v, want %v", err, tt.err)
			}
		})
	}
}

func TestTargetErrors(t *testing.T) {
	var nilp *rec
	if _, err := Marshal(nilp, recLen); err != ErrTarget {
		t.Errorf("nil pointer: %v", err)
	}
	if _, err := Marshal(42, recLen); err != ErrTarget {
		t.Errorf("int: %v", err)
	}
	if err := Unmarshal("x", rec{}); err != ErrTarget {
		t.Errorf("non-pointer: %v", err)
	}
	if err := Check(nil, 1); err != ErrTarget {
		t.Errorf("Check(nil): %v", err)
	}
	if _, err := Width("x"); err != ErrTarget {
		t.Errorf("Width: %v", err)
	}
	if _, err := Fields(nil); err != ErrTarget {
		t.Errorf("Fields: %v", err)
	}
}

type gappy struct {
	A string `cnab:"1,3,num"`
	B string `cnab:"5,2,alfa"`
	C string `cnab:"6,3,alfa"`
}

type badTags struct {
	A string    `cnab:"1"`
	B string    `cnab:"x,1,num"`
	C string    `cnab:"1-x,num"`
	D string    `cnab:"0,1,num"`
	E string    `cnab:"5-3,num"`
	F string    `cnab:"1,0,num"`
	G string    `cnab:"1,1,weird"`
	H bool      `cnab:"1,1,num"`
	I int       `cnab:"1,19,num"`
	J string    `cnab:"1,1,money"`
	K int       `cnab:"1,1,alfa"`
	L string    `cnab:"1,8,date"`
	M time.Time `cnab:"1,7,date"`
	N int       `cnab:"1,1,const=1"`
	O string    `cnab:"1,2,const=1"`
	P int8      `cnab:"1,3,money"`
	q string    `cnab:"1,1,num"`
}

type empty struct{ X string }

type shared struct {
	A string `cnab:"1,2,num"`
}

type embedded struct {
	shared
	B string `cnab:"3,2,alfa"`
}

func TestLayoutErrors(t *testing.T) {
	err := Check(gappy{}, 8)
	var le *LayoutError
	if !errors.As(err, &le) || !errors.Is(err, ErrLayout) {
		t.Fatalf("got %v", err)
	}
	msg := err.Error()
	for _, want := range []string{"gap at 4-4 between A and B", "overlap: B (5-6) and C (6-8)"} {
		if !strings.Contains(msg, want) {
			t.Errorf("%q lacks %q", msg, want)
		}
	}
	err = Check(badTags{}, 1)
	if !errors.As(err, &le) {
		t.Fatalf("got %v", err)
	}
	if len(le.Problems) < 17 {
		t.Errorf("want one problem per bad field, got %d: %v", len(le.Problems), le.Problems)
	}
	for _, name := range []string{"A:", "B:", "H:", "I:", "q:"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("missing problem for %s in %v", name, err)
		}
	}
	if err := Check(empty{}, 1); !errors.Is(err, ErrLayout) {
		t.Errorf("empty: %v", err)
	}
	if err := Check(rec{}, 50); err == nil || !strings.Contains(err.Error(), "48-50 not covered") {
		t.Errorf("short layout: %v", err)
	}
	if err := Check(rec{}, 40); err == nil || !strings.Contains(err.Error(), "Valor (32-41) exceeds line length 40") {
		t.Errorf("long layout: %v", err)
	}
	if _, err := Marshal(rec{}, 40); !errors.Is(err, ErrLayout) {
		t.Errorf("Marshal width mismatch: %v", err)
	}
	if _, err := Marshal(gappy{}, 8); !errors.Is(err, ErrLayout) {
		t.Errorf("Marshal invalid: %v", err)
	}
	if err := Unmarshal("12345678", &gappy{}); !errors.Is(err, ErrLayout) {
		t.Errorf("Unmarshal invalid: %v", err)
	}
	if _, err := Width(gappy{}); !errors.Is(err, ErrLayout) {
		t.Errorf("Width invalid: %v", err)
	}
	if _, err := Fields(gappy{}); !errors.Is(err, ErrLayout) {
		t.Errorf("Fields invalid: %v", err)
	}
	type leading struct {
		A string `cnab:"2,2,num"`
	}
	if err := Check(leading{}, 3); err == nil || !strings.Contains(err.Error(), "gap at 1-1 before A") {
		t.Errorf("leading gap: %v", err)
	}
}

func TestEmbeddedAndIntrospection(t *testing.T) {
	if err := Check(embedded{}, 4); err != nil {
		t.Fatal(err)
	}
	line, err := Marshal(embedded{shared: shared{A: "7"}, B: "x"}, 4)
	if err != nil || line != "07X " {
		t.Fatalf("Marshal = %q, %v", line, err)
	}
	if w, err := Width(&rec{}); err != nil || w != recLen {
		t.Errorf("Width = %d, %v", w, err)
	}
	fs, err := Fields(rec{})
	if err != nil || len(fs) != 9 || fs[0] != (Field{Name: "Tipo", Start: 1, End: 1, Format: "const", Const: "3"}) {
		t.Errorf("Fields = %+v, %v", fs, err)
	}
}

func TestFold(t *testing.T) {
	tests := map[string]string{
		"":                 "",
		"ABC 123":          "ABC 123",
		"ação":             "ACAO",
		"Avenida São João": "AVENIDA SAO JOAO",
		"Nº 10, 2ª via":    "NO 10, 2A VIA",
		"Ñandú Ölçü":       "NANDU OLCU",
		"áàâãäéèêëíìîïóòôõöúùûüçñ": "AAAAAEEEEIIIIOOOOOUUUUCN",
	}
	for in, want := range tests {
		if got, err := Fold(in); err != nil || got != want {
			t.Errorf("Fold(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"\xff", "a\x00", "€", "x\x7f"} {
		if _, err := Fold(in); err == nil {
			t.Errorf("Fold(%q) succeeded", in)
		}
	}
}

func FuzzUnmarshal(f *testing.F) {
	ok, _ := Marshal(rec{Codigo: "1", Nome: "X", Data: d(2026, 1, 1), Valor: 1}, recLen)
	f.Add(ok)
	f.Add(strings.Repeat(" ", recLen))
	f.Add(strings.Repeat("9", recLen))
	f.Add("3" + strings.Repeat("0", recLen-1))
	f.Fuzz(func(t *testing.T, line string) {
		var v rec
		if err := Unmarshal(line, &v); err != nil {
			return
		}
		m1, err := Marshal(v, recLen)
		if err != nil {
			return // e.g. lowercase or non-ASCII alfa input the encoder rejects
		}
		var w rec
		if err := Unmarshal(m1, &w); err != nil {
			t.Fatalf("Unmarshal(Marshal(v)) failed: %v\n%q", err, m1)
		}
		m2, err := Marshal(w, recLen)
		if err != nil || m2 != m1 {
			t.Fatalf("encoding not idempotent:\n%q\n%q (%v)", m1, m2, err)
		}
	})
}
