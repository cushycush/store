// Package platform re-exports store-core's platform detection so store's
// existing import paths keep working. New code should prefer importing
// github.com/cushycush/store-core/platform directly.
package platform

import core "github.com/cushycush/store-core/platform"

// Info mirrors store-core's Info type.
type Info = core.Info

// Detect returns the detected platform info.
func Detect() Info { return core.Detect() }
