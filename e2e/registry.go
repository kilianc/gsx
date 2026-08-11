package e2e

import g "maragu.dev/gomponents"

// GSXFunctions maps a fixture name to a function that renders it.
//
// Fixtures that have a matching `<name>.html.out` golden register themselves
// from an `init()` in their `.gsx` source. TestRenderHTML renders every entry
// and compares the result against that golden.
var GSXFunctions = map[string]func() g.Node{}
