package e2e

import . "github.com/kilianc/gsx/e2e/dotimport"

func init() {
	GSXFunctions["dot_import"] = func() Node {
		return DotImportTest()
	}
}

func DotImportTest() Node {
	return (
		<div>
			<Section heading="Info">
				<p>content</p>
			</Section>
			<EmptyState message="no items" />
			<Page><span>hello</span></Page>
		</div>
	)
}
