package e2e

import "github.com/kilianc/gsx/e2e/helpers"

func init() {
	GSXFunctions["typed_params"] = func() Node {
		return TypedParams()
	}
}

func TypedParams() Node {
	return (
		<div>
			<helpers.SectionHeading text="Debug" />
			<helpers.Badge text="Active" variant="success" />
			<helpers.SectionWithHeading heading="Info">
				<p>content</p>
			</helpers.SectionWithHeading>
			<helpers.StatusDot active={true} />
			<helpers.EmptyState message="No items found" />
		</div>
	)
}
