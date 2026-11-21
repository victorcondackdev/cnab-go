package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const td = "../../file/testdata/"

func runCLI(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := run(args, &out, &errb)
	return code, out.String(), errb.String()
}

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestInspect(t *testing.T) {
	code, out, _ := runCLI("inspect", td+"retorno_341.ret")
	if code != 0 || !strings.Contains(out, "CNAB 240 retorno, banco 341") || !strings.Contains(out, "T:9 U:9") || !strings.Contains(out, "06 Liquidação") {
		t.Fatalf("%d %s", code, out)
	}
	code, out, _ = runCLI("inspect", "--json", td+"remessa_341.rem")
	var s fileSummary
	if code != 0 || json.Unmarshal([]byte(out), &s) != nil || s.Tipo != "remessa" || s.Batches[0].Segments["P"] != 5 || s.Batches[0].Nominal != 317040 {
		t.Fatalf("%d %s", code, out)
	}
	code, out, _ = runCLI("inspect", td+"remessa_237.rem")
	if code != 0 || !strings.Contains(out, "CNAB 400 remessa") || !strings.Contains(out, "2 detail records") {
		t.Fatalf("%d %s", code, out)
	}
	if code, _, _ := runCLI("inspect", "--json", td+"remessa_237.rem"); code != 0 {
		t.Fatal(code)
	}
	if code, _, _ := runCLI("inspect"); code != 2 {
		t.Error("missing arg")
	}
	if code, _, _ := runCLI("inspect", "--bogus", "x"); code != 2 {
		t.Error("bad flag")
	}
	if code, _, _ := runCLI("inspect", "/nonexistent"); code != 1 {
		t.Error("missing file")
	}
	if code, _, errs := runCLI("inspect", td+"expected.csv"); code != 1 || !strings.Contains(errs, "240") {
		t.Error("invalid 240")
	}
	bad400 := writeTemp(t, "bad.rem", strings.Repeat("x", 400)+"\n")
	if code, _, _ := runCLI("inspect", bad400); code != 1 {
		t.Error("invalid 400")
	}
	// Unknown header code and raw segments.
	data, _ := os.ReadFile(td + "remessa_341.rem")
	odd := string(data[:142]) + "3" + string(data[143:])
	if code, out, _ := runCLI("inspect", writeTemp(t, "odd.rem", odd)); code != 0 || !strings.Contains(out, "código 3") {
		t.Errorf("%d %s", code, out)
	}
}

func TestValidate(t *testing.T) {
	code, out, _ := runCLI("validate", "--totals", td+"remessa_341.rem", td+"retorno_341.ret", td+"remessa_237.rem")
	if code != 0 || strings.Count(out, "ok ") != 3 {
		t.Fatalf("%d %s", code, out)
	}
	code, out, _ = runCLI("validate", td+"remessa_341.rem", td+"expected.csv", "/nonexistent")
	if code != 1 || strings.Count(out, "FAIL") != 2 {
		t.Fatalf("%d %s", code, out)
	}
	if code, _, _ := runCLI("validate"); code != 2 {
		t.Error("no args")
	}
	if code, _, _ := runCLI("validate", "--x"); code != 2 {
		t.Error("bad flag")
	}
}

func TestReconcile(t *testing.T) {
	code, out, _ := runCLI("reconcile", "--expected", td+"expected.csv", td+"retorno_341.ret")
	if code != 0 || !strings.Contains(out, "[duplicate] 00000001") {
		t.Fatalf("%d %s", code, out)
	}
	if code, _, _ := runCLI("reconcile", "--strict", "--expected", td+"expected.csv", td+"retorno_341.ret"); code != 1 {
		t.Error("strict")
	}
	code, out, _ = runCLI("reconcile", "--json", "--grace-days", "3", "--expected", td+"expected.csv", td+"retorno_341.ret")
	var rep jsonReport
	if code != 0 || json.Unmarshal([]byte(out), &rep) != nil || rep.Counts["paid_adjusted"] != 2 || rep.PaidTotal != 281880 || !rep.NeedsAttention {
		t.Fatalf("%d %s", code, out)
	}
	semi := writeTemp(t, "exp.csv", "\xef\xbb\xbfNosso_Numero;Valor;Vencimento\n1;1.500,00;10/12/2026\n2;899,90;15122026\n4;420.50;\n")
	code, out, _ = runCLI("reconcile", "--expected", semi, td+"retorno_341.ret")
	if code != 0 || !strings.Contains(out, "3 expected boletos") {
		t.Fatalf("%d %s", code, out)
	}
	bad := map[string]string{
		"empty":   "",
		"columns": "nosso_numero,valor\n1,2\n",
		"valor":   "nosso_numero,valor,vencimento\n1,abc,2026-01-01\n",
		"date":    "nosso_numero,valor,vencimento\n1,1.00,2026-13-01\n",
		"csv":     "nosso_numero,valor,vencimento\n1,\"x,2026-01-01\n",
		"dup":     "nosso_numero,valor,vencimento\n1,1.00,\n01,1.00,\n",
	}
	for name, content := range bad {
		if code, _, _ := runCLI("reconcile", "--expected", writeTemp(t, name+".csv", content), td+"retorno_341.ret"); code != 1 {
			t.Errorf("%s: code %d", name, code)
		}
	}
	for _, args := range [][]string{
		{"reconcile", td + "retorno_341.ret"},
		{"reconcile", "--expected", td + "expected.csv"},
		{"reconcile", "--nope"},
	} {
		if code, _, _ := runCLI(args...); code != 2 {
			t.Errorf("%v: code %d", args, code)
		}
	}
	if code, _, _ := runCLI("reconcile", "--expected", "/nonexistent", td+"retorno_341.ret"); code != 1 {
		t.Error("missing expected")
	}
	if code, _, _ := runCLI("reconcile", "--expected", td+"expected.csv", "/nonexistent"); code != 1 {
		t.Error("missing retorno")
	}
	if code, _, _ := runCLI("reconcile", "--expected", td+"expected.csv", td+"remessa_237.rem"); code != 1 {
		t.Error("400 retorno")
	}
}

func TestUsage(t *testing.T) {
	if code, _, errs := runCLI(); code != 2 || !strings.Contains(errs, "Usage") {
		t.Error("no args")
	}
	if code, out, _ := runCLI("help"); code != 0 || !strings.Contains(out, "Usage") {
		t.Error("help")
	}
	if code, _, errs := runCLI("frobnicate"); code != 2 || !strings.Contains(errs, "unknown command") {
		t.Error("unknown")
	}
}
