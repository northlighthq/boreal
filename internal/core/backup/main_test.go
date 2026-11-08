package backup

import (
	"os"
	"testing"

	"github.com/northlighthq/boreal/internal/platform/keyring"
)

func TestMain(m *testing.M) {
	keyring.UseMemory()
	os.Exit(m.Run())
}
