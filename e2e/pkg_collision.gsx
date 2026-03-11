package e2e

func init() {
	GSXFunctions["pkg_collision"] = func() Node {
		return PkgCollision()
	}
}

// Header is a local component whose name collides with html.Header.
func Header(title string, children ...Node) Node {
	return <header><h1>{title}</h1>{children}</header>
}

func PkgCollision() Node {
	return (
		<div>
			<Header title="Hello">
				<p>content</p>
			</Header>
			<header>
				<p>raw header</p>
			</header>
		</div>
	)
}
