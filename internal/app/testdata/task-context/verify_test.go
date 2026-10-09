package fixture

import "testing"

func TestTwice(t *testing.T) {
	if Twice(4) != 8 {
		t.Fatal("verification fixture arithmetic")
	}
}
