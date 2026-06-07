package sec

import "testing"

func TestKeepForm(t *testing.T) {
	if !KeepForm("8-K", []string{"10-Q", "8-K"}) {
		t.Fatal("expected keep")
	}
	if KeepForm("4", []string{"8-K"}) {
		t.Fatal("expected reject")
	}
}
