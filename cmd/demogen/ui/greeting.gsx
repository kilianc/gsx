package ui

func Greeting(name string) Node {
	return (
		<main class="page">
			<h1>Hello, {name}!</h1>
			<p>Welcome to GSX.</p>
		</main>
	)
}
