package e2e

func init() {
	GSXFunctions["var_scoping"] = func() Node {
		return VarScoping()
	}
}

// VarScoping's `account` local is a Node and `items` is a []Node. Splice type
// inference must resolve both from THIS function's scope: UnrelatedAccount
// below reuses `account` as a plain string param, and before inference was
// scoped per function that name leaked file-wide and re-typed the splice here
// as a string, wrapping it in Text and breaking the build.
func VarScoping() Node {
	account := <span class="badge">acct</span>
	items := nodeItems()
	return (
		<div>
			{account}
			<ul>{items}</ul>
			{UnrelatedAccount(true, "u-1")}
		</div>
	)
}

// UnrelatedAccount reuses the name `account` for a string param that is never
// spliced. It must not affect how VarScoping's `{account}` is lowered.
func UnrelatedAccount(ok bool, account string) Node {
	msg := "subscription for " + account
	if !ok {
		msg = "no subscription"
	}
	return <p>{msg}</p>
}

// nodeItems exercises []Node return-type inference: VarScoping's `items` must
// come from this signature, not from `out` leaking across function bodies.
func nodeItems() []Node {
	var out []Node
	for _, s := range []string{"a", "b"} {
		out = append(out, <li>{s}</li>)
	}
	return out
}
