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
	"PlotLine":     {},
	"PlotScatter":  {},
}

func IsName(name string) bool {
	_, ok := names[name]
	return ok
}
