package build

import "fmt"

// Image handles belong to the image sidecar. Transfer encoded bytes to the
// PDF sidecar through the existing typed wire operations; never reinterpret
// a handle owned by one process as a handle owned by another.
func (c *lowerCtx) lowerPdfImageHandleCall(name string, args, types []string) (string, string, bool, error) {
	encode, _ := lookupSidecarBuiltin("ImageEncodePng")
	encoded, _, _, err := c.lowerSidecarBuiltinCall(encode, args[1:2], types[1:2])
	if err != nil {
		return "", "", false, err
	}
	result := c.temp(fallibleType("Int"))
	start := c.cur
	okID, errID, mergeID := len(c.blocks), len(c.blocks)+1, len(c.blocks)+2
	for _, id := range []int{okID, errID, mergeID} {
		c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", id)})
	}
	c.blocks[start].Terminator = MIRBranch{Cond: MIRFieldAccess{Target: lowerMIRValue(encoded, ""), Field: "IsErr", Type: "Bool"}, TrueTarget: c.blocks[errID].Label, FalseTarget: c.blocks[okID].Label}
	c.cur = errID
	c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: result, Value: MIRResultValue{ResultType: "Int", Error: MIRFieldAccess{Target: lowerMIRValue(encoded, ""), Field: "Err", Type: "Error"}, IsError: true}})
	c.blocks[c.cur].Terminator = MIRJump{Target: c.blocks[mergeID].Label}
	c.cur = okID
	bytes := c.temp("Bytes")
	c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: bytes, Value: MIRFieldAccess{Target: lowerMIRValue(encoded, ""), Field: "Value", Type: "Bytes"}})
	drawName := "PdfDrawImageBytes"
	if name == "PdfDrawImageSized" {
		drawName += "Sized"
	}
	draw, _ := lookupSidecarBuiltin(drawName)
	drawArgs := append([]string{args[0], bytes, `"png"`}, args[2:]...)
	drawTypes := append([]string{types[0], "Bytes", "String"}, types[2:]...)
	drawn, _, _, err := c.lowerSidecarBuiltinCall(draw, drawArgs, drawTypes)
	if err != nil {
		return "", "", false, err
	}
	c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: result, Value: lowerMIRValue(drawn, fallibleType("Int"))})
	c.blocks[c.cur].Terminator = MIRJump{Target: c.blocks[mergeID].Label}
	c.cur = mergeID
	return result, "Int", true, nil
}

func (c *lowerCtx) lowerShortPlotCall(name string, args, types []string) (string, string, bool, error) {
	renderName := "PlotRenderLine"
	if name == "PlotScatter" {
		renderName = "PlotRenderScatter"
	}
	render, _ := lookupSidecarBuiltin(renderName)
	args = append(args, "384", "384", `""`, `"x"`, `"y"`, `""`)
	types = append(types, "Int<px>", "Int<px>", "String", "String", "String", "String")
	raw, _, _, err := c.lowerSidecarBuiltinCall(render, args, types)
	if err != nil {
		return "", "", false, err
	}
	value := c.temp("Int")
	c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: value, Value: MIRBackendValue{Backend: "go", Type: "Int", Reason: "infallible-short-plot-adapter", Expression: fmt.Sprintf(`func() int { if %s.IsErr { panic("runtime error: " + %s.Err) }; return %s.Value }()`, raw, raw, raw)}})
	return value, "Int", false, nil
}
