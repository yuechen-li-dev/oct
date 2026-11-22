package typecheck

// variantConstructorType is the type of a payload variant named without its
// payload, `Msg.SetAudio`: a function from the payload to the enum
// (DESIGN.md §7 L2). Calling it builds the variant, so it can be passed
// where a callback is expected, as in `UI.Checkbox("Audio", on, Msg.SetAudio)`.
func (c checker) variantConstructorType(enumName string, payload Type) Type {
	signature := functionSignature{parameters: []Type{payload}, returnType: Type{Name: enumName}}
	key := signature.String()
	c.functionTypes[key] = signature
	return Type{IsFunction: true, FunctionSignature: key}
}
