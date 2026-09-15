package build

import (
	"fmt"
	"github.com/yuechen-li-dev/oct/internal/layoutcontract"
	"strings"
)

func dumpMIR(m MIRModule) string {
	var b strings.Builder
	fmt.Fprintf(&b, "module entry=%s\n", m.EntryPackage)
	for _, template := range m.Templates {
		fmt.Fprintf(&b, "template %s.%s kind=%s origin=%s.%s args=%s\n", template.Package, template.ConcreteName, template.Kind, template.OriginPackage, template.OriginName, strings.Join(template.TypeArguments, ","))
	}
	for _, record := range m.Records {
		fmt.Fprintf(&b, "record %s.%s kind=%s\n", record.Package, record.Name, record.Kind)
	}
	for _, contract := range m.LayoutContracts {
		fmt.Fprintf(&b, "layout-contract %s\n", layoutcontract.Format(contract))
	}
	for _, selector := range m.Selectors {
		fmt.Fprintf(&b, "selector %s.%s owner=%s field=%s[%d] result=%s\n", selector.Package, selector.Name, selector.Owner, selector.Field.Name, selector.Field.Ordinal, selector.Result)
	}
	for _, flow := range m.Flows {
		fmt.Fprintf(&b, "flow %s.%s -> %s\n", flow.Package, flow.Name, flow.Return)
		if flow.Async != nil {
			fmt.Fprintf(&b, "  async-lowering lifted=[%s] continuations=[%s]\n", strings.Join(flow.Async.LiftedLocals, ","), strings.Join(flow.Async.Continuations, ","))
			for _, field := range flow.Board {
				role := "await-handle"
				for _, local := range flow.Async.LiftedLocals {
					if field.Name == "Local_"+local {
						role = "lifted-local"
						break
					}
				}
				fmt.Fprintf(&b, "  persistent %s:%s role=%s\n", field.Name, field.Type, role)
			}
		}
		for idx, state := range flow.States {
			fmt.Fprintf(&b, "  state[%d] %s\n", idx, state.Name)
			for _, stmt := range state.Statements {
				fmt.Fprintf(&b, "    %s\n", dumpFlowStmt(stmt))
			}
		}
	}
	for _, fn := range m.Functions {
		fmt.Fprintf(&b, "fn %s.%s", fn.Package, fn.Name)
		if len(fn.CaptureEnv) > 0 {
			b.WriteString(" capture-env{")
			for i, capture := range fn.CaptureEnv {
				if i > 0 {
					b.WriteString(", ")
				}
				fmt.Fprintf(&b, "%s:%s", capture.Name, capture.Type)
			}
			b.WriteString("}")
		}
		fmt.Fprintf(&b, "(")
		for i, p := range fn.Params {
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "%s:%s", p.Name, p.Type)
		}
		fmt.Fprintf(&b, ") -> %s", fn.Return)
		if fn.IsFallible {
			fmt.Fprintf(&b, " ! %s", fn.ErrorType)
		}
		b.WriteString("\n")
		for _, bb := range fn.Blocks {
			fmt.Fprintf(&b, "  %s:\n", bb.Label)
			for _, s := range bb.Statements {
				switch st := s.(type) {
				case MIRAssign:
					fmt.Fprintf(&b, "    %s = %s\n", st.Target, dumpMIRValue(st.Value))
				case MIRRowAssign:
					fmt.Fprintf(&b, "    row_assign %s[%s] = %s\n", st.Target, dumpMIRValue(st.Index), dumpMIRValue(st.Value))
				case MIRIndexAssign:
					fmt.Fprintf(&b, "    index_assign %s[%s] = %s\n", st.Target, strings.Join(dumpMIRValues(st.Indices), "]["), dumpMIRValue(st.Value))
				case MIRCall:
					fmt.Fprintf(&b, "    %s = call %s(%s)\n", st.Target, st.Callee, strings.Join(dumpMIRValues(st.Args), ", "))
				case MIRConstructArray:
					fmt.Fprintf(&b, "    %s = [%s]\n", st.Target, strings.Join(dumpMIRValues(st.Values), ", "))
				case MIRConstructRecord:
					fmt.Fprintf(&b, "    %s = %s{...}\n", st.Target, st.TypeName)
				case MIRBatchMap:
					fmt.Fprintf(&b, "    %s = batch_map %s with %s\n", st.Target, dumpMIRValue(st.Input), st.Worker)
				}
			}
			switch t := bb.Terminator.(type) {
			case MIRReturn:
				fmt.Fprintf(&b, "    return %s\n", dumpMIRValue(t.Value))
			case MIRJump:
				fmt.Fprintf(&b, "    jump %s\n", t.Target)
			case MIRBranch:
				fmt.Fprintf(&b, "    branch %s ? %s : %s\n", dumpMIRValue(t.Cond), t.TrueTarget, t.FalseTarget)
			case MIRFail:
				fmt.Fprintf(&b, "    fail %s\n", dumpMIRValue(t.Value))
			}
		}
	}
	return b.String()
}

func dumpMIRValues(values []MIRValue) []string {
	result := make([]string, len(values))
	for i, value := range values {
		result[i] = dumpMIRValue(value)
	}
	return result
}

func dumpFlowStmt(stmt MIRFlowStmt) string {
	switch s := stmt.(type) {
	case MIRFlowGoto:
		return "goto " + s.Target
	case MIRFlowSuspend:
		return "suspend"
	case MIRFlowRemember:
		return "remember"
	case MIRFlowResume:
		return "resume"
	case MIRFlowFieldAssign:
		return fmt.Sprintf("%s.%s = %s", s.Target, s.Field, dumpFlowExpr(s.Value))
	case MIRFlowFieldIndexAssign:
		parts := make([]string, 0, len(s.Indices))
		for _, index := range s.Indices {
			parts = append(parts, dumpFlowExpr(index))
		}
		return fmt.Sprintf("%s.%s[%s] = %s", s.Target, s.Field, strings.Join(parts, "]["), dumpFlowExpr(s.Value))
	case MIRFlowReturn:
		if s.Value == nil {
			return "return"
		}
		return "return " + dumpFlowExpr(s.Value)
	case MIRFlowExprStmt:
		return "discard " + dumpFlowExpr(s.Value)
	case MIRFlowIf:
		return fmt.Sprintf("if %s { ... }", dumpFlowExpr(s.Condition))
	case MIRFlowWhen:
		parts := make([]string, 0, len(s.Cases))
		for _, c := range s.Cases {
			parts = append(parts, fmt.Sprintf("case %s -> %s", dumpFlowExpr(c.Condition), dumpFlowWhenAction(c.Action)))
		}
		return "when { " + strings.Join(parts, "; ") + "; else -> " + dumpFlowWhenAction(s.Else) + " }"
	default:
		return fmt.Sprintf("unsupported-flow-stmt(%T)", stmt)
	}
}

func dumpFlowWhenAction(action MIRFlowWhenAction) string {
	switch a := action.(type) {
	case MIRFlowWhenGoto:
		return "goto " + a.Target
	case MIRFlowWhenSuspend:
		return "suspend"
	case MIRFlowWhenReturn:
		return "return " + dumpFlowExpr(a.Value)
	case MIRFlowWhenBlock:
		parts := make([]string, 0, len(a.Statements))
		for _, stmt := range a.Statements {
			parts = append(parts, dumpFlowStmt(stmt))
		}
		return "{ " + strings.Join(parts, "; ") + " }"
	default:
		return fmt.Sprintf("unsupported-flow-action(%T)", action)
	}
}

func dumpFlowExpr(expr MIRFlowExpr) string {
	switch e := expr.(type) {
	case MIRFlowSharedExpr:
		return fmt.Sprintf("ordinary-mir[%s,fallible=%t]:%#v", e.Type, e.Fallible, e.Blocks)
	case MIRFlowUtilityWhenExpr:
		return fmt.Sprintf("utility_when[site=%d,hysteresis=%s,min_commit=%s,cases=%d]", e.SiteID, dumpFlowExpr(e.Hysteresis), dumpFlowExpr(e.MinCommit), len(e.Cases))
	default:
		return fmt.Sprintf("unsupported-flow-expr(%T)", expr)
	}
}
