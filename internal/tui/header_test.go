package tui

import (
	"strings"
	"testing"

	"github.com/cushycush/store/v2/internal/config"
	"github.com/cushycush/store/v2/internal/platform"
)

// makeApp builds a minimal App directly rather than going through New(),
// because New() consults the real $PATH — which would make this test flaky
// depending on whether stock happens to be installed on the host.
func makeApp(hasStock bool) *App {
	return &App{
		root:     "/tmp/fake",
		cfg:      &config.Config{Stores: map[string]config.StoreEntry{}},
		plat:     platform.Info{OS: "linux", Arch: "amd64"},
		hasStock: hasStock,
	}
}

func TestHeaderIncludesStockWhenOnPath(t *testing.T) {
	got := makeApp(true).renderHeader(120)
	if !strings.Contains(got, "stock") {
		t.Fatalf("header = %q, expected it to include 'stock' signpost when hasStock is true", got)
	}
}

func TestHeaderOmitsStockWhenMissing(t *testing.T) {
	got := makeApp(false).renderHeader(120)
	// The word "store" appears as the brand — strip that before probing.
	stripped := strings.ReplaceAll(got, "store", "")
	if strings.Contains(stripped, "stock") {
		t.Fatalf("header = %q, did not expect 'stock' signpost when hasStock is false", got)
	}
}
