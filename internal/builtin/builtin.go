package builtin

var names = map[string]struct{}{
	"Len":          {},
	"Append":       {},
	"Abs":          {},
	"Sqrt":         {},
	"Sin":          {},
	"Cos":          {},
	"Print":        {},
	"WriteOctagon": {},
	"LoadOctagon":  {},
	"PlotLine":     {},
	"PlotScatter":  {},
	"Step":         {},
	"Active":       {},
	"Result":       {},
}

func IsName(name string) bool {
	_, ok := names[name]
	return ok
}
