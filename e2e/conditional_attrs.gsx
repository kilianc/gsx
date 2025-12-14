package e2e

import "strconv"

func init() {
	GSXFunctions["conditional_attrs"] = func() Node {
		return ConditionalAttrs(false, true, true, 123)
	}
}

func ConditionalAttrs(disabled bool, required bool, showID bool, id int) Node {
	return (
		<form>
			<input
				disabled={disabled}
				required={required}
				{If(showID, Attr("data-id", strconv.Itoa(id)))}
			/>
		</form>
	)
}
