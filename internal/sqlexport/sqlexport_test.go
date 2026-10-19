package sqlexport

import "testing"

func TestPlainDecimal(t *testing.T) {
	for in, want := range map[string]string{
		"1.5E+3": "1500", "-0.00012": "-0.00012", "1.2E-5": "0.000012", "123": "123", "0": "0", "-0E+3": "0",
		"1.000": "1", "12.3400": "12.34", "9.99E+2": "999",
	} {
		got, ok := plainDecimal(in)
		if !ok || got != want {
			t.Errorf("%s → %q (%v), want %q", in, got, ok, want)
		}
	}
	if _, ok := plainDecimal("1E+6000"); ok {
		t.Error("huge exponent accepted")
	}
}

func TestIdentifierAndMerge(t *testing.T) {
	if identifier(`a.b"c`) != "a_b_c" {
		t.Fatal(identifier(`a.b"c`))
	}
	if k, mixed := merge("int", "double"); k != "double" || mixed {
		t.Fatal(k)
	}
	if k, mixed := merge("int", "string"); k != "json" || !mixed {
		t.Fatal(k)
	}
	if k, _ := merge("", "date"); k != "date" {
		t.Fatal(k)
	}
}
