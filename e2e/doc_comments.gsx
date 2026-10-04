package e2e

import (
	"strings"
)

// shout is the doc comment on the first declaration after the imports. It
// must stay here rather than move into the rewritten import block.
const shout = "hi"

func init() {
	GSXFunctions["doc_comments"] = func() Node {
		return DocComments()
	}
}

// DocComments renders shout upper-cased.
func DocComments() Node {
	return <p>{strings.ToUpper(shout)}</p>
}
