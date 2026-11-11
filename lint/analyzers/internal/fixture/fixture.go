// Package fixture finds the analyzers' shared testdata: one GOPATH-style tree,
// with stubs of the libraries the analyzers know and one fixture module per
// analyzer, such as example.com/layers.
package fixture

import (
	"path/filepath"
	"runtime"
)

// Dir returns the absolute path of the shared testdata directory.
func Dir() string {
	_, file, _, _ := runtime.Caller(0)

	return filepath.Join(filepath.Dir(file), "..", "..", "testdata")
}
