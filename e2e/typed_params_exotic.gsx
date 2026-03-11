package e2e

import (
	"database/sql"
	"time"

	"github.com/kilianc/gsx/e2e/helpers"
)

func init() {
	GSXFunctions["typed_params_exotic"] = func() Node {
		return TypedParamsExotic()
	}
}

func TypedParamsExotic() Node {
	now := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	link := helpers.LinkData{URL: "/details", Text: "View"}
	name := sql.NullString{String: "Alice", Valid: true}

	return (
		<div>
			<helpers.CellTime t={now} />
			<helpers.CellLink link={link} />
			<helpers.CellNullText value={name} />
			<helpers.TimestampSection t={now}>
				<p>event details</p>
			</helpers.TimestampSection>
		</div>
	)
}
