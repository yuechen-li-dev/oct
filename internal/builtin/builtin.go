package builtin

var names = map[string]struct{}{
	"Len":          {},
	"Append":       {},
	"Complex":      {},
	"I":            {},
	"Real":         {},
	"Imag":         {},
	"Conj":         {},
	"Abs":          {},
	"Sqrt":         {},
	"Sin":          {},
	"Cos":          {},
	"Tan":          {},
	"Asin":         {},
	"Acos":         {},
	"Atan":         {},
	"Atan2":        {},
	"Exp":          {},
	"Ln":           {},
	"Log10":        {},
	"Sinh":         {},
	"Cosh":         {},
	"Tanh":         {},
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
