package cartoes

import "testing"

func TestMaskCPF(t *testing.T) {
	if got, want := MaskCPF("123.456.789-01"), "***.456.789-**"; got != want {
		t.Fatalf("MaskCPF() = %q, want %q", got, want)
	}
	if got, want := MaskCPF(MaskCPF("123.456.789-01")), "***.456.789-**"; got != want {
		t.Fatalf("MaskCPF() twice = %q, want %q", got, want)
	}
	if got, want := MaskCPF("***.456.789-**"), "***.456.789-**"; got != want {
		t.Fatalf("MaskCPF(masked) = %q, want %q", got, want)
	}
	if got := MaskCPF("invalid"); got != "" {
		t.Fatalf("MaskCPF(invalid) = %q, want empty", got)
	}
}
