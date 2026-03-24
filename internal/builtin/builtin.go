package builtin

var names = map[string]struct{}{
	"Len":         {},
	"Abs":         {},
	"Sqrt":        {},
	"Sin":         {},
	"Cos":         {},
	"Print":       {},
	"PlotLine":    {},
	"PlotScatter": {},
}

func IsName(name string) bool {
	_, ok := names[name]
	return ok
}
