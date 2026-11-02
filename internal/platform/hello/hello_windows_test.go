package hello

import "testing"

func TestParameterizedIID(t *testing.T) {
	got := parameterizedIID("pinterface({9fc2b0bb-e446-44e2-aa61-9cab8f636af2};b1)").String()

	if got != "{CDB5EFB3-5788-509D-9BE1-71CCB8A3362A}" {
		t.Fatalf("got %s", got)
	}
}

func TestAvailable(t *testing.T) {
	t.Logf("Windows Hello available: %v", Available())
}
