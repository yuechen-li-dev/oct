package builtin

var names = map[string]struct{}{
	"Len":          {},
	"Append":       {},
	"Abs":          {},
	"Sqrt":         {},
	"Sin":          {},
	"Cos":          {},
	"Tan":          {},
	"Asin":         {},
	"Acos":         {},
	"Atan":         {},
	"Exp":          {},
	"Ln":           {},
	"Log10":        {},
	"Pi":           {},
	"E":            {},
	"Print":        {},
	"WriteOctagon": {},
	"LoadOctagon":  {},
	"PlotLine":     {},
	"PlotScatter":  {},
	"Step":         {},
	"Active":       {},
	"Result":       {},
	"Complete":     {},
	"StateHistory": {},
	"ResumeTarget": {},
}

func IsName(name string) bool {
	_, ok := names[name]
	return ok
}
