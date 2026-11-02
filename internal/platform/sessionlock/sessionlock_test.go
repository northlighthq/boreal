package sessionlock

import "testing"

func TestUnlockedWhileTesting(t *testing.T) {
	if Locked() {
		t.Fatal("the session running the tests should not be locked")
	}
}
