package e2e

type panel struct {
	Open bool
}

func init() {
	GSXFunctions["bool_attrs"] = func() Node {
		return BoolAttrs(true, false, panel{Open: false}, 2)
	}
}

// BoolAttrs puts bool expressions on attributes gomponents has no boolean
// constructor for. Each renders bare when true and is left out when false.
func BoolAttrs(open bool, hidden bool, p panel, step int) Node {
	return (
		<div>
			<details open={open}><summary>a</summary></details>
			<details open={p.Open}><summary>b</summary></details>
			<section hidden={hidden} inert={!open}>c</section>
			<form novalidate={step > 1}>d</form>
			<iframe allowfullscreen={open && !hidden}></iframe>
		</div>
	)
}
