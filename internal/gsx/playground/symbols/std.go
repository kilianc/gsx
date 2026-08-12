package symbols

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// The standard library the playground exposes, written out rather than
// extracted.
//
// `yaegi extract` reflects whatever the building toolchain happens to have, so
// a generated table records one Go version's standard library and fails to
// compile on an older one — an extract on Go 1.24+ picks up strings.SplitSeq
// and friends, which do not exist in the version go.mod declares. Listing the
// names keeps the file buildable by any supported toolchain and regenerable to
// the same bytes.
//
// It also states the sandbox plainly. This list is the whole of what a reader's
// snippet can reach beyond gomponents, so widening it is an edit here rather
// than a side effect of upgrading Go. Everything chosen is long-stable and
// pure: formatting a value, casing a label, ordering rows.
func init() {
	Symbols["fmt/fmt"] = map[string]reflect.Value{
		"Errorf":   reflect.ValueOf(fmt.Errorf),
		"Sprint":   reflect.ValueOf(fmt.Sprint),
		"Sprintf":  reflect.ValueOf(fmt.Sprintf),
		"Sprintln": reflect.ValueOf(fmt.Sprintln),
	}

	Symbols["strings/strings"] = map[string]reflect.Value{
		"Contains":    reflect.ValueOf(strings.Contains),
		"ContainsAny": reflect.ValueOf(strings.ContainsAny),
		"Count":       reflect.ValueOf(strings.Count),
		"EqualFold":   reflect.ValueOf(strings.EqualFold),
		"Fields":      reflect.ValueOf(strings.Fields),
		"HasPrefix":   reflect.ValueOf(strings.HasPrefix),
		"HasSuffix":   reflect.ValueOf(strings.HasSuffix),
		"Index":       reflect.ValueOf(strings.Index),
		"Join":        reflect.ValueOf(strings.Join),
		"LastIndex":   reflect.ValueOf(strings.LastIndex),
		"Repeat":      reflect.ValueOf(strings.Repeat),
		"Replace":     reflect.ValueOf(strings.Replace),
		"ReplaceAll":  reflect.ValueOf(strings.ReplaceAll),
		"Split":       reflect.ValueOf(strings.Split),
		"SplitN":      reflect.ValueOf(strings.SplitN),
		"Title":       reflect.ValueOf(strings.Title),
		"ToLower":     reflect.ValueOf(strings.ToLower),
		"ToTitle":     reflect.ValueOf(strings.ToTitle),
		"ToUpper":     reflect.ValueOf(strings.ToUpper),
		"Trim":        reflect.ValueOf(strings.Trim),
		"TrimLeft":    reflect.ValueOf(strings.TrimLeft),
		"TrimPrefix":  reflect.ValueOf(strings.TrimPrefix),
		"TrimRight":   reflect.ValueOf(strings.TrimRight),
		"TrimSpace":   reflect.ValueOf(strings.TrimSpace),
		"TrimSuffix":  reflect.ValueOf(strings.TrimSuffix),
	}

	Symbols["strconv/strconv"] = map[string]reflect.Value{
		"Atoi":        reflect.ValueOf(strconv.Atoi),
		"FormatBool":  reflect.ValueOf(strconv.FormatBool),
		"FormatFloat": reflect.ValueOf(strconv.FormatFloat),
		"FormatInt":   reflect.ValueOf(strconv.FormatInt),
		"Itoa":        reflect.ValueOf(strconv.Itoa),
		"ParseBool":   reflect.ValueOf(strconv.ParseBool),
		"ParseFloat":  reflect.ValueOf(strconv.ParseFloat),
		"ParseInt":    reflect.ValueOf(strconv.ParseInt),
		"Quote":       reflect.ValueOf(strconv.Quote),
	}

	Symbols["sort/sort"] = map[string]reflect.Value{
		"Float64s":    reflect.ValueOf(sort.Float64s),
		"Ints":        reflect.ValueOf(sort.Ints),
		"Slice":       reflect.ValueOf(sort.Slice),
		"SliceStable": reflect.ValueOf(sort.SliceStable),
		"Strings":     reflect.ValueOf(sort.Strings),
	}
}
