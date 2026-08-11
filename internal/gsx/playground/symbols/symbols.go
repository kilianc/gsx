// Package symbols is the set of packages the playground's interpreter can see.
//
// The tables beside this file are generated; see internal/gsx/playground/gen.
package symbols

import "reflect"

// Symbols maps "<import path>/<package name>" to that package's exported
// values. The generated files populate it from their init functions.
//
// It is also the sandbox boundary: interpreted code can resolve an import only
// if it appears here, so a package left out is a package a reader's browser
// cannot be made to call.
var Symbols = map[string]map[string]reflect.Value{}
