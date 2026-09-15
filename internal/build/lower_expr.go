package build

import (
	"fmt"
	"github.com/yuechen-li-dev/oct/internal/ast"
	"github.com/yuechen-li-dev/oct/internal/batchcapture"
	"github.com/yuechen-li-dev/oct/internal/builtin"
	"github.com/yuechen-li-dev/oct/internal/project"
	"sort"
	"strings"
)

func (c *lowerCtx) lowerExpr(expr ast.Expr) (string, string, bool, error) {
	switch e := expr.(type) {
	case ast.IntegerLiteral:
		if expected, ok := c.currentExpectedType(); ok && isFloatScalarTypeString(expected) {
			return fmt.Sprintf("float64(%s)", e.Value), expected, false, nil
		}
		if e.HasUnit {
			return e.Value, fmt.Sprintf("Int<%s>", e.Dimension.String()), false, nil
		}
		return e.Value, "Int", false, nil
	case ast.FloatLiteral:
		if e.HasUnit {
			return e.Value, fmt.Sprintf("Float<%s>", e.Dimension.String()), false, nil
		}
		return e.Value, "Float", false, nil
	case ast.BoolLiteral:
		if e.Value {
			return "true", "Bool", false, nil
		}
		return "false", "Bool", false, nil
	case ast.StringLiteralExpr:
		return fmt.Sprintf("%q", e.Value), "String", false, nil
	case ast.IdentifierExpr:
		t, ok := c.locals[e.Name]
		if ok {
			return c.goLocalName(e.Name), t, false, nil
		}
		if symbol, signature, ok := c.resolveNamedFunctionValue(e); ok {
			return symbol, signature, false, nil
		}
		return "", "", false, fmt.Errorf("unknown identifier '%s'", e.Name)
	case ast.FunctionExpr:
		return c.lowerFunctionExpr(e)
	case ast.BinaryExpr:
		if e.Operator == "and" || e.Operator == "or" {
			return c.lowerLogicalBinaryExpr(e)
		}
		l, lt, _, err := c.withExpectedType("", func() (string, string, bool, error) { return c.lowerExpr(e.Left) })
		if err != nil {
			return "", "", false, err
		}
		r, rt, _, err := c.withExpectedType("", func() (string, string, bool, error) { return c.lowerExpr(e.Right) })
		if err != nil {
			return "", "", false, err
		}
		if leftTerm, leftIndexed := c.einTerm(l); leftIndexed {
			rightTerm, rightIndexed := c.einTerm(r)
			if !rightIndexed {
				return "", "", false, fmt.Errorf("compiled indexed tensor expressions must appear on both sides of '%s'", e.Operator)
			}
			leftElem, leftVector := parseVectorElemType(lt)
			if !leftVector {
				leftElem, _ = parseMatrixElemType(lt)
			}
			rightElem, rightVector := parseVectorElemType(rt)
			if !rightVector {
				rightElem, _ = parseMatrixElemType(rt)
			}
			retElem := unifyLinearElemType(leftElem, rightElem)
			switch e.Operator {
			case "+", "-":
				if leftTerm.Rank != rightTerm.Rank {
					return "", "", false, fmt.Errorf("compiled Einstein addition/subtraction requires matching indexed term ranks (left rank=%d, right rank=%d)", leftTerm.Rank, rightTerm.Rank)
				}
				if leftTerm.Rank != 1 && leftTerm.Rank != 2 {
					return "", "", false, fmt.Errorf("compiled Einstein addition/subtraction supports only rank-1 vectors and rank-2 matrices, got rank %d", leftTerm.Rank)
				}
				if !einsteinLabelsMatch(leftTerm.Labels, rightTerm.Labels) {
					return "", "", false, fmt.Errorf("compiled Einstein '%s' requires matching free-index order", e.Operator)
				}
				callee := "EinAdd"
				if e.Operator == "-" {
					callee = "EinSub"
				}
				if leftTerm.Rank == 1 {
					if !leftVector || !rightVector {
						return "", "", false, fmt.Errorf("compiled rank-1 Einstein addition/subtraction requires vector operands")
					}
					ret := "Vector<" + retElem + ">"
					tmp := c.temp(ret)
					c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: tmp, Callee: callee + "VV", Args: lowerMIRValues([]string{l, leftTerm.Labels[0], r, rightTerm.Labels[0]}, nil), Builtin: true, RetType: ret})
					c.setEinTermMeta(tmp, leftTerm.Labels, 1, ret)
					return tmp, ret, false, nil
				}
				ret := "Matrix<" + retElem + ">"
				tmp := c.temp(ret)
				c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: tmp, Callee: callee, Args: lowerMIRValues([]string{l, leftTerm.Labels[0], leftTerm.Labels[1], r, rightTerm.Labels[0], rightTerm.Labels[1]}, nil), Builtin: true, RetType: ret})
				c.setEinTermMeta(tmp, leftTerm.Labels, 2, ret)
				return tmp, ret, false, nil
			case "*":
				free, err := einsteinMulFreeLabels(leftTerm.Labels, rightTerm.Labels)
				if err != nil {
					return "", "", false, err
				}
				if leftTerm.Rank == 2 && rightTerm.Rank == 2 {
					if len(free) == 0 {
						tmp := c.temp(retElem)
						c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: tmp, Callee: "EinDoubleMM", Args: lowerMIRValues([]string{l, leftTerm.Labels[0], leftTerm.Labels[1], r, rightTerm.Labels[0], rightTerm.Labels[1]}, nil), Builtin: true, RetType: retElem})
						return tmp, retElem, false, nil
					}
					if len(free) != 2 {
						return "", "", false, fmt.Errorf("compiled matrix/matrix indexed contraction requires either 0 or 2 free indices, got %d", len(free))
					}
					ret := "Matrix<" + retElem + ">"
					tmp := c.temp(ret)
					c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: tmp, Callee: "EinMul", Args: lowerMIRValues([]string{l, leftTerm.Labels[0], leftTerm.Labels[1], r, rightTerm.Labels[0], rightTerm.Labels[1]}, nil), Builtin: true, RetType: ret})
					c.setEinTermMeta(tmp, free, 2, ret)
					return tmp, ret, false, nil
				}
				if leftTerm.Rank == 1 && rightTerm.Rank == 1 {
					if !leftVector || !rightVector {
						return "", "", false, fmt.Errorf("compiled rank-1 Einstein multiplication requires vector operands")
					}
					if len(free) == 0 {
						tmp := c.temp(retElem)
						c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: tmp, Callee: "EinDotVV", Args: lowerMIRValues([]string{l, leftTerm.Labels[0], r, rightTerm.Labels[0]}, nil), Builtin: true, RetType: retElem})
						return tmp, retElem, false, nil
					}
					if len(free) == 2 {
						ret := "Matrix<" + retElem + ">"
						tmp := c.temp(ret)
						c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: tmp, Callee: "EinOuterVV", Args: lowerMIRValues([]string{l, leftTerm.Labels[0], r, rightTerm.Labels[0]}, nil), Builtin: true, RetType: ret})
						c.setEinTermMeta(tmp, free, 2, ret)
						return tmp, ret, false, nil
					}
				}
				if leftTerm.Rank == 2 && rightTerm.Rank == 1 {
					if len(free) != 1 {
						return "", "", false, fmt.Errorf("compiled matrix-vector indexed contraction requires exactly 1 free index, got %d", len(free))
					}
					ret := "Vector<" + retElem + ">"
					tmp := c.temp(ret)
					c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: tmp, Callee: "EinMulMV", Args: lowerMIRValues([]string{l, leftTerm.Labels[0], leftTerm.Labels[1], r, rightTerm.Labels[0], free[0]}, nil), Builtin: true, RetType: ret})
					c.setEinTermMeta(tmp, free, 1, ret)
					return tmp, ret, false, nil
				}
				if leftTerm.Rank == 1 && rightTerm.Rank == 2 {
					if len(free) != 1 {
						return "", "", false, fmt.Errorf("compiled vector-matrix indexed contraction requires exactly 1 free index, got %d", len(free))
					}
					ret := "Vector<" + retElem + ">"
					tmp := c.temp(ret)
					c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: tmp, Callee: "EinMulVM", Args: lowerMIRValues([]string{l, leftTerm.Labels[0], r, rightTerm.Labels[0], rightTerm.Labels[1], free[0]}, nil), Builtin: true, RetType: ret})
					c.setEinTermMeta(tmp, free, 1, ret)
					return tmp, ret, false, nil
				}
				return "", "", false, fmt.Errorf("compiled indexed multiplication supports rank-1 vectors and rank-2 matrices only (left rank=%d, right rank=%d)", leftTerm.Rank, rightTerm.Rank)
			default:
				return "", "", false, fmt.Errorf("compiled indexed tensor expressions only support '+', '-', and '*'")
			}
		} else if _, rightIndexed := c.einTerm(r); rightIndexed {
			return "", "", false, fmt.Errorf("compiled indexed tensor expressions must appear on both sides of '%s'", e.Operator)
		}
		lt = c.eraseRefinementType(lt)
		rt = c.eraseRefinementType(rt)
		if e.Operator == "@" {
			if leftElem, ok := parseMatrixElemType(lt); ok {
				if rightElem, ok := parseVectorElemType(rt); ok {
					elemType := unifyLinearElemType(leftElem, rightElem)
					ret := "Vector<" + elemType + ">"
					tmp := c.temp(ret)
					c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{
						Target:  tmp,
						Callee:  "MatMulMV",
						Args:    lowerMIRValues([]string{l, r}, nil),
						Builtin: true,
						RetType: ret,
					})
					return tmp, ret, false, nil
				}
				if rightElem, ok := parseMatrixElemType(rt); ok {
					elemType := unifyLinearElemType(leftElem, rightElem)
					ret := "Matrix<" + elemType + ">"
					tmp := c.temp(ret)
					callee := "MatMulMM"
					if c.inPrometheus && leftElem == "Float" && rightElem == "Float" {
						callee = "PrometheusMatMulMM"
					}
					c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{
						Target:  tmp,
						Callee:  callee,
						Args:    lowerMIRValues([]string{l, r}, nil),
						Builtin: true,
						RetType: ret,
					})
					return tmp, ret, false, nil
				}
			}
			if leftElem, ok := parseVectorElemType(lt); ok {
				if rightElem, ok := parseMatrixElemType(rt); ok {
					elemType := unifyLinearElemType(leftElem, rightElem)
					ret := "Vector<" + elemType + ">"
					tmp := c.temp(ret)
					c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{
						Target:  tmp,
						Callee:  "MatMulVM",
						Args:    lowerMIRValues([]string{l, r}, nil),
						Builtin: true,
						RetType: ret,
					})
					return tmp, ret, false, nil
				}
				if rightElem, ok := parseVectorElemType(rt); ok {
					ret := unifyLinearElemType(leftElem, rightElem)
					tmp := c.temp(ret)
					c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{
						Target:  tmp,
						Callee:  "VecDot",
						Args:    lowerMIRValues([]string{l, r}, nil),
						Builtin: true,
						RetType: ret,
					})
					return tmp, ret, false, nil
				}
			}
		}
		if leftElem, ok := parseVectorElemType(lt); ok {
			if rightElem, ok := parseVectorElemType(rt); ok {
				ret := "Vector<" + unifyLinearElemType(leftElem, rightElem) + ">"
				tmp := c.temp(ret)
				c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{
					Target:  tmp,
					Callee:  "VecBinaryVV:" + e.Operator,
					Args:    lowerMIRValues([]string{l, r}, nil),
					Builtin: true,
					RetType: ret,
				})
				return tmp, ret, false, nil
			}
			if isNumericTypeString(rt) {
				tmp := c.temp(lt)
				c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{
					Target:  tmp,
					Callee:  "VecBinaryVS:" + e.Operator,
					Args:    lowerMIRValues([]string{l, r}, nil),
					Builtin: true,
					RetType: lt,
				})
				return tmp, lt, false, nil
			}
		}
		if rightElem, ok := parseVectorElemType(rt); ok && isNumericTypeString(lt) {
			tmp := c.temp(rt)
			c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{
				Target:  tmp,
				Callee:  "VecBinarySV:" + e.Operator,
				Args:    lowerMIRValues([]string{l, r}, nil),
				Builtin: true,
				RetType: "Vector<" + rightElem + ">",
			})
			return tmp, rt, false, nil
		}
		if leftElem, ok := parseMatrixElemType(lt); ok && isLinearElementwiseOperatorString(e.Operator) {
			if rightElem, ok := parseMatrixElemType(rt); ok {
				retElem := scalarBinaryResultTypeString(e.Operator, leftElem, rightElem)
				ret := "Matrix<" + retElem + ">"
				tmp := c.temp(ret)
				c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{
					Target:   tmp,
					Callee:   "MatBinaryMM:" + e.Operator,
					Args:     lowerMIRValues([]string{l, r}, nil),
					ArgTypes: []string{lt, rt},
					Builtin:  true,
					RetType:  ret,
				})
				return tmp, ret, false, nil
			}
			if isNumericTypeString(rt) {
				retElem := scalarBinaryResultTypeString(e.Operator, leftElem, rt)
				ret := "Matrix<" + retElem + ">"
				tmp := c.temp(ret)
				c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{
					Target:   tmp,
					Callee:   "MatBinaryMS:" + e.Operator,
					Args:     lowerMIRValues([]string{l, r}, nil),
					ArgTypes: []string{lt, rt},
					Builtin:  true,
					RetType:  ret,
				})
				return tmp, ret, false, nil
			}
		}
		if rightElem, ok := parseMatrixElemType(rt); ok && isNumericTypeString(lt) && isLinearElementwiseOperatorString(e.Operator) {
			retElem := scalarBinaryResultTypeString(e.Operator, lt, rightElem)
			ret := "Matrix<" + retElem + ">"
			tmp := c.temp(ret)
			c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{
				Target:   tmp,
				Callee:   "MatBinarySM:" + e.Operator,
				Args:     lowerMIRValues([]string{l, r}, nil),
				ArgTypes: []string{lt, rt},
				Builtin:  true,
				RetType:  ret,
			})
			return tmp, ret, false, nil
		}
		if leftElem, ok := parseArrayElemType(lt); ok && !strings.HasSuffix(leftElem, "[]") && isLinearElementwiseOperatorString(e.Operator) && isNumericTypeString(rt) {
			retElem := scalarBinaryResultTypeString(e.Operator, leftElem, rt)
			ret := retElem + "[]"
			tmp := c.temp(ret)
			c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: tmp, Callee: "ArrayBinaryAS:" + e.Operator, Args: lowerMIRValues([]string{l, r}, nil), ArgTypes: []string{lt, rt}, Builtin: true, RetType: ret})
			return tmp, ret, false, nil
		}
		if rightElem, ok := parseArrayElemType(rt); ok && !strings.HasSuffix(rightElem, "[]") && isLinearElementwiseOperatorString(e.Operator) && isNumericTypeString(lt) {
			retElem := scalarBinaryResultTypeString(e.Operator, lt, rightElem)
			ret := retElem + "[]"
			tmp := c.temp(ret)
			c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: tmp, Callee: "ArrayBinarySA:" + e.Operator, Args: lowerMIRValues([]string{l, r}, nil), ArgTypes: []string{lt, rt}, Builtin: true, RetType: ret})
			return tmp, ret, false, nil
		}
		if leftElem, ok := parseArrayElemType(lt); ok && !strings.HasSuffix(leftElem, "[]") && isComparisonOperatorString(e.Operator) && isNumericTypeString(rt) {
			tmp := c.temp("Bool[]")
			c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: tmp, Callee: "ArrayBinaryAS:" + e.Operator, Args: lowerMIRValues([]string{l, r}, nil), ArgTypes: []string{lt, rt}, Builtin: true, RetType: "Bool[]"})
			return tmp, "Bool[]", false, nil
		}
		if rightElem, ok := parseArrayElemType(rt); ok && !strings.HasSuffix(rightElem, "[]") && isComparisonOperatorString(e.Operator) && isNumericTypeString(lt) {
			tmp := c.temp("Bool[]")
			c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: tmp, Callee: "ArrayBinarySA:" + e.Operator, Args: lowerMIRValues([]string{l, r}, nil), ArgTypes: []string{lt, rt}, Builtin: true, RetType: "Bool[]"})
			return tmp, "Bool[]", false, nil
		}
		ret := lt
		switch e.Operator {
		case "==", "!=", "<", "<=", ">", ">=", "and", "or":
			ret = "Bool"
		case "+", "-", "*", "/":
			if isComplexScalarTypeString(lt) || isComplexScalarTypeString(rt) {
				ret = "Complex"
			} else if isFloatScalarTypeString(lt) || isFloatScalarTypeString(rt) {
				if strings.HasPrefix(lt, "Float<") && strings.HasSuffix(lt, ">") {
					ret = lt
				} else if strings.HasPrefix(rt, "Float<") && strings.HasSuffix(rt, ">") {
					ret = rt
				} else {
					ret = "Float"
				}
			}
		case "%":
			ret = "Int"
		}
		tmp := c.temp(ret)
		op := e.Operator
		if op == "and" {
			op = "&&"
		}
		if op == "or" {
			op = "||"
		}
		if ret == "Complex" && isComplexCompatibleScalarTypeString(lt) && isComplexCompatibleScalarTypeString(rt) {
			l = coerceExprToType(l, lt, "Complex")
			r = coerceExprToType(r, rt, "Complex")
		} else if isNumericTypeString(lt) && isNumericTypeString(rt) {
			l, r = coerceNumericBinaryOperands(l, lt, r, rt, ret)
			if ret == "Bool" && (isFloatScalarTypeString(lt) || isFloatScalarTypeString(rt)) {
				l, r = coerceNumericBinaryOperands(l, lt, r, rt, "Float")
			}
		}
		if op == "%" {
			c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{
				Target: tmp,
				Value:  MIRIntrinsicValue{Kind: "euclidean-modulo", Type: ret, Args: []MIRValue{lowerMIRValue(l, lt), lowerMIRValue(r, rt)}},
			})
			return tmp, ret, false, nil
		}
		if op == "/" {
			// Keep division operands as function parameters. Go rejects a literal
			// zero divisor while compiling a non-selected switch arm, whereas Oct
			// evaluates that arm lazily and reports division failure only if it is
			// selected at runtime.
			c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{
				Target: tmp,
				Value:  MIRIntrinsicValue{Kind: "safe-divide", Type: ret, Args: []MIRValue{lowerMIRValue(l, ret), lowerMIRValue(r, ret)}},
			})
			return tmp, ret, false, nil
		}
		c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: tmp, Value: MIRBinary{Op: op, Left: lowerMIRValue(l, lt), Right: lowerMIRValue(r, rt), Type: ret}})
		return tmp, ret, false, nil
	case ast.UnaryExpr:
		v, t, _, err := c.lowerExpr(e.Operand)
		if err != nil {
			return "", "", false, err
		}
		t = c.eraseRefinementType(t)
		tmp := c.temp(t)
		op := e.Operator
		if op == "not" {
			op = "!"
		}
		c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: tmp, Value: MIRUnary{Op: op, Value: lowerMIRValue(v, t), Type: t}})
		return tmp, t, false, nil
	case ast.CallExpr:
		if calleeField, ok := e.Callee.(ast.FieldAccessExpr); ok {
			if enumType, variant, ok := c.flattenEnumVariantExpr(calleeField); ok {
				enumValue, resolvedEnumType, enumFound, err := c.resolveEnumVariantConstructor(enumType, variant, e.Arguments)
				if err != nil {
					return "", "", false, err
				}
				if enumFound {
					return enumValue, resolvedEnumType, false, nil
				}
			}
		}
		if ident, ok := e.Callee.(ast.IdentifierExpr); ok && ident.Name == "error" {
			if len(e.Arguments) != 1 {
				return "", "", false, fmt.Errorf("error() expects one argument")
			}
			v, _, _, err := c.lowerExpr(e.Arguments[0])
			if err != nil {
				return "", "", false, err
			}
			return v, "Error", false, nil
		}
		if ident, ok := e.Callee.(ast.IdentifierExpr); ok && ident.Name == "WriteOctagon" {
			args := make([]string, 0, len(e.Arguments))
			for _, a := range e.Arguments {
				v, _, _, err := c.lowerExpr(a)
				if err != nil {
					return "", "", false, err
				}
				args = append(args, v)
			}
			tmp := c.temp("Int")
			c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: tmp, Callee: "WriteOctagon", Args: lowerMIRValues(args, nil), Builtin: true, RetType: "Int"})
			return tmp, "Int", false, nil
		}
		if ident, ok := e.Callee.(ast.IdentifierExpr); ok && ident.Name == "LoadOctagon" {
			args := make([]string, 0, len(e.Arguments))
			for _, a := range e.Arguments {
				v, _, _, err := c.lowerExpr(a)
				if err != nil {
					return "", "", false, err
				}
				args = append(args, v)
			}
			ret := typeRefStringForPackage(c.pkg.Name, e.TypeArguments[0])
			tmp := c.temp(fallibleType(ret))
			c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: tmp, Callee: "LoadOctagon", Args: lowerMIRValues(args, nil), Builtin: true, RetType: ret})
			return tmp, ret, true, nil
		}
		if ident, ok := e.Callee.(ast.IdentifierExpr); ok && ident.Name == "Result" {
			if len(e.Arguments) != 1 {
				return "", "", false, fmt.Errorf("Result expects one argument")
			}
			flowArg, flowType, _, err := c.lowerExpr(e.Arguments[0])
			if err != nil {
				return "", "", false, err
			}
			resultType, ok := parseFlowInstanceType(flowType)
			if !ok {
				return "", "", false, fmt.Errorf("Result expects FlowInstance argument")
			}
			tmp := c.temp(fallibleType(resultType))
			c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: tmp, Callee: "Result", Args: lowerMIRValues([]string{flowArg}, nil), Builtin: true, RetType: resultType})
			return tmp, resultType, true, nil
		}
		if ident, ok := e.Callee.(ast.IdentifierExpr); ok && ident.Name == "Yielded" {
			if len(e.Arguments) != 1 {
				return "", "", false, fmt.Errorf("Yielded expects one argument")
			}
			flowArg, flowType, _, err := c.lowerExpr(e.Arguments[0])
			if err != nil {
				return "", "", false, err
			}
			_, _, yieldType, ok := parseFlowInstanceDetails(flowType)
			if !ok || yieldType == "" {
				return "", "", false, fmt.Errorf("Yielded expects a yielding FlowInstance")
			}
			tmp := c.temp(fallibleType(yieldType))
			c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: tmp, Callee: "Yielded", Args: lowerMIRValues([]string{flowArg}, nil), Builtin: true, RetType: yieldType})
			return tmp, yieldType, true, nil
		}
		if ident, ok := e.Callee.(ast.IdentifierExpr); ok {
			switch ident.Name {
			case "PrometheusMatMul":
				if len(e.Arguments) != 2 {
					return "", "", false, fmt.Errorf("PrometheusMatMul expects 2 arguments")
				}
				leftArg, leftType, _, err := c.lowerExpr(e.Arguments[0])
				if err != nil {
					return "", "", false, err
				}
				rightArg, rightType, _, err := c.lowerExpr(e.Arguments[1])
				if err != nil {
					return "", "", false, err
				}
				if leftType != "Matrix<Float>" || rightType != "Matrix<Float>" {
					return "", "", false, fmt.Errorf("PrometheusMatMul expects Matrix<Float> arguments")
				}
				tmp := c.temp("Matrix<Float>")
				c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{
					Target:  tmp,
					Callee:  "PrometheusMatMulMM",
					Args:    lowerMIRValues([]string{leftArg, rightArg}, nil),
					Builtin: true,
					RetType: "Matrix<Float>",
				})
				return tmp, "Matrix<Float>", false, nil
			case "Abs", "Pi", "E", "I", "Complex", "ComplexPolar", "Real", "Imag", "Arg", "Conj", "Sqrt", "Sin", "Cos", "Tan", "Asin", "Acos", "Atan", "Atan2", "Exp", "Ln", "Pow", "Log10", "Sinh", "Cosh", "Tanh", "Trace", "Grad", "Div", "SymGrad":
				args := make([]string, 0, len(e.Arguments))
				argTypes := make([]string, 0, len(e.Arguments))
				for _, a := range e.Arguments {
					v, t, _, err := c.lowerExpr(a)
					if err != nil {
						return "", "", false, err
					}
					args = append(args, v)
					argTypes = append(argTypes, t)
				}
				ret, err := compiledBuiltinReturnType(ident.Name, argTypes)
				if err != nil {
					return "", "", false, err
				}
				tmp := c.temp(ret)
				c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: tmp, Callee: ident.Name, Args: lowerMIRValues(args, nil), ArgTypes: argTypes, Builtin: true, RetType: ret})
				return tmp, ret, false, nil
			}
		}
		if calleeName, ok := flattenDirectCallName(e.Callee); ok {
			if calleeName == "Assert.LGTM" {
				if len(e.Arguments) != 2 {
					return "", "", false, fmt.Errorf("Assert.LGTM expects 2 arguments, got %d", len(e.Arguments))
				}
				resultVar, resultType, _, err := c.lowerExpr(e.Arguments[0])
				if err != nil {
					return "", "", false, err
				}
				messageVar, _, _, err := c.lowerExpr(e.Arguments[1])
				if err != nil {
					return "", "", false, err
				}
				tmp := c.temp(resultType)
				c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{
					Target:  tmp,
					Callee:  "Assert.LGTM",
					Args:    lowerMIRValues([]string{resultVar, messageVar}, nil),
					Builtin: true,
					RetType: resultType,
				})
				return tmp, resultType, false, nil
			}
			if strings.HasPrefix(calleeName, "Assert.") {
				args := make([]string, 0, len(e.Arguments))
				for _, a := range e.Arguments {
					v, _, _, err := c.lowerExpr(a)
					if err != nil {
						return "", "", false, err
					}
					args = append(args, v)
				}
				switch calleeName {
				case "Assert.True":
					c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: "_", Callee: "Assert.True", Args: lowerMIRValues(args, nil), Builtin: true, RetType: "Void"})
					return "_", "Void", false, nil
				case "Assert.False":
					c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: "_", Callee: "Assert.False", Args: lowerMIRValues(args, nil), Builtin: true, RetType: "Void"})
					return "_", "Void", false, nil
				case "Assert.Equal":
					c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: "_", Callee: "Assert.Equal", Args: lowerMIRValues(args, nil), Builtin: true, RetType: "Void"})
					return "_", "Void", false, nil
				case "Assert.Near":
					c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: "_", Callee: "Assert.Near", Args: lowerMIRValues(args, nil), Builtin: true, RetType: "Void"})
					return "_", "Void", false, nil
				case "Assert.Error":
					c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: "_", Callee: "Assert.Error", Args: lowerMIRValues(args, nil), Builtin: true, RetType: "Void"})
					return "_", "Void", false, nil
				}
			}
			switch calleeName {
			case "Vector.tabulate":
				if len(e.Arguments) != 2 {
					return "", "", false, fmt.Errorf("Vector.tabulate expects 2 arguments")
				}
				lengthArg, _, _, err := c.lowerExpr(e.Arguments[0])
				if err != nil {
					return "", "", false, err
				}
				callbackName, callbackType, callbackFallible, err := c.lowerExpr(e.Arguments[1])
				if err != nil {
					return "", "", false, err
				}
				callbackSignature, ok := parseCompiledFunctionType(callbackType)
				if !ok || len(callbackSignature.Parameters) != 1 || callbackSignature.Parameters[0] != "Int" || callbackFallible || callbackSignature.Fallible {
					return "", "", false, fmt.Errorf("Vector.tabulate callback must have type fn(Int) -> T")
				}
				callbackRet := callbackSignature.ReturnType
				ret := "Vector<" + callbackRet + ">"
				tmp := c.temp(ret)
				c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: tmp, Callee: "Vector.tabulate", Args: lowerMIRValues([]string{lengthArg, callbackName}, nil), Builtin: true, RetType: ret})
				return tmp, ret, false, nil
			case "Matrix.tabulate":
				if len(e.Arguments) != 3 {
					return "", "", false, fmt.Errorf("Matrix.tabulate expects 3 arguments")
				}
				rowsArg, _, _, err := c.lowerExpr(e.Arguments[0])
				if err != nil {
					return "", "", false, err
				}
				colsArg, _, _, err := c.lowerExpr(e.Arguments[1])
				if err != nil {
					return "", "", false, err
				}
				callbackName, callbackType, callbackFallible, err := c.lowerExpr(e.Arguments[2])
				if err != nil {
					return "", "", false, err
				}
				callbackSignature, ok := parseCompiledFunctionType(callbackType)
				if !ok || len(callbackSignature.Parameters) != 2 || callbackSignature.Parameters[0] != "Int" || callbackSignature.Parameters[1] != "Int" || callbackFallible || callbackSignature.Fallible {
					return "", "", false, fmt.Errorf("Matrix.tabulate callback must have type fn(Int, Int) -> T")
				}
				callbackRet := callbackSignature.ReturnType
				ret := "Matrix<" + callbackRet + ">"
				tmp := c.temp(ret)
				c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: tmp, Callee: "Matrix.tabulate", Args: lowerMIRValues([]string{rowsArg, colsArg, callbackName}, nil), Builtin: true, RetType: ret})
				return tmp, ret, false, nil
			case "Matrix.fill":
				if len(e.Arguments) != 3 {
					return "", "", false, fmt.Errorf("Matrix.fill expects 3 arguments")
				}
				args := make([]string, 0, 3)
				elemType := "Int"
				for idx, a := range e.Arguments {
					v, t, _, err := c.lowerExpr(a)
					if err != nil {
						return "", "", false, err
					}
					args = append(args, v)
					if idx == 2 {
						elemType = t
					}
				}
				ret := "Matrix<" + elemType + ">"
				tmp := c.temp(ret)
				c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: tmp, Callee: "Matrix.fill", Args: lowerMIRValues(args, nil), Builtin: true, RetType: ret})
				return tmp, ret, false, nil
			case "Matrix.zeros":
				if len(e.TypeArguments) != 1 {
					return "", "", false, fmt.Errorf("Matrix.zeros expects 1 type argument")
				}
				if len(e.Arguments) != 2 {
					return "", "", false, fmt.Errorf("Matrix.zeros expects 2 arguments")
				}
				rowsArg, _, _, err := c.lowerExpr(e.Arguments[0])
				if err != nil {
					return "", "", false, err
				}
				colsArg, _, _, err := c.lowerExpr(e.Arguments[1])
				if err != nil {
					return "", "", false, err
				}
				elemType := typeRefStringForPackage(c.pkg.Name, e.TypeArguments[0])
				ret := "Matrix<" + elemType + ">"
				tmp := c.temp(ret)
				c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: tmp, Callee: "Matrix.zeros", Args: lowerMIRValues([]string{rowsArg, colsArg}, nil), Builtin: true, RetType: ret})
				return tmp, ret, false, nil
			case "Matrix.identity":
				if len(e.TypeArguments) != 1 {
					return "", "", false, fmt.Errorf("Matrix.identity expects 1 type argument")
				}
				if len(e.Arguments) != 1 {
					return "", "", false, fmt.Errorf("Matrix.identity expects 1 argument")
				}
				sizeArg, _, _, err := c.lowerExpr(e.Arguments[0])
				if err != nil {
					return "", "", false, err
				}
				elemType := typeRefStringForPackage(c.pkg.Name, e.TypeArguments[0])
				ret := "Matrix<" + elemType + ">"
				tmp := c.temp(ret)
				c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: tmp, Callee: "Matrix.identity", Args: lowerMIRValues([]string{sizeArg}, nil), Builtin: true, RetType: ret})
				return tmp, ret, false, nil
			}
		}
		if callTarget, signature, ok, err := c.resolveFunctionValueCall(e.Callee); err != nil {
			return "", "", false, err
		} else if ok {
			args := make([]string, 0, len(e.Arguments))
			argTypes := make([]string, 0, len(e.Arguments))
			for i, a := range e.Arguments {
				var expected string
				if i < len(signature.Parameters) {
					expected = signature.Parameters[i]
				}
				v, at, _, err := c.withExpectedType(expected, func() (string, string, bool, error) { return c.lowerExpr(a) })
				if err != nil {
					return "", "", false, err
				}
				if expected != "" {
					v = goCoerceArg(v, at, expected)
				}
				args = append(args, v)
				argTypes = append(argTypes, at)
			}
			localType := signature.ReturnType
			if signature.Fallible {
				localType = fallibleType(signature.ReturnType)
			}
			tmp := c.temp(localType)
			c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: tmp, Callee: callTarget, Args: lowerMIRValues(args, nil), ArgTypes: argTypes, RetType: signature.ReturnType, FunctionValue: true})
			return tmp, signature.ReturnType, signature.Fallible, nil
		}
		callee, ret, builtin, fallible, err := c.resolveCall(e.Callee)
		if err != nil {
			return "", "", false, err
		}
		expectedArgTypes := c.resolveCallArgTypes(e.Callee)
		args := make([]string, 0, len(e.Arguments))
		argTypes := make([]string, 0, len(e.Arguments))
		for i, a := range e.Arguments {
			var expected string
			if i < len(expectedArgTypes) {
				expected = expectedArgTypes[i]
			}
			v, at, _, err := c.withExpectedType(expected, func() (string, string, bool, error) { return c.lowerExpr(a) })
			if err != nil {
				return "", "", false, err
			}
			if expected != "" {
				v = goCoerceArg(v, at, expected)
			}
			args = append(args, v)
			argTypes = append(argTypes, at)
		}
		if builtin && (callee == "Append" || callee == "ArrayCrossSection" || callee == "Array.CrossSection" || callee == "ArrayWhere" || callee == "Array.Where") && len(argTypes) > 0 {
			ret = argTypes[0]
		}
		if builtin && callee == "Query.First" && len(argTypes) == 1 {
			_, inputType, yieldType, ok := parseFlowInstanceDetails(argTypes[0])
			if !ok || inputType != "" || yieldType == "" {
				return "", "", false, fmt.Errorf("Query.First requires a no-input yielding flow")
			}
			ret = yieldType
		}
		if builtin && isMarkdownCompiledBuiltin(callee) {
			checkedRet, err := compiledBuiltinReturnType(callee, argTypes)
			if err != nil {
				return "", "", false, err
			}
			ret = checkedRet
		}
		if builtin && (callee == "Idx" || callee == "EinMul" || callee == "EinAdd" || callee == "EinSub") {
			checkedRet, err := compiledBuiltinReturnType(callee, argTypes)
			if err != nil {
				return "", "", false, err
			}
			ret = checkedRet
		}
		if meta, ok, err := c.genericWrapperMetadataForCallee(e.Callee); err != nil {
			return "", "", false, err
		} else if ok {
			effectiveReturn := meta.Return
			if !strings.Contains(effectiveReturn, ".") && ret == meta.PackageName+"."+effectiveReturn {
				effectiveReturn = ret
			}
			if ret != effectiveReturn {
				return "", "", false, fmt.Errorf("wrapper function %s.%s manifest return %s does not match Oct stub return %s", meta.PackageName, meta.OctName, meta.Return, ret)
			}
			if fallible != meta.Fallible {
				return "", "", false, fmt.Errorf("wrapper function %s.%s manifest fallible %t does not match Oct stub fallible %t", meta.PackageName, meta.OctName, meta.Fallible, fallible)
			}
			if len(argTypes) != len(meta.Args) {
				return "", "", false, fmt.Errorf("wrapper function %s.%s expects %d arguments, got %d", meta.PackageName, meta.OctName, len(meta.Args), len(argTypes))
			}
			effectiveArgTypes := append([]string(nil), argTypes...)
			for i := range argTypes {
				if !wrapperArgTypeMatches(meta.Args[i], argTypes[i]) {
					return "", "", false, fmt.Errorf("wrapper function %s.%s argument %d expects %s, got %s", meta.PackageName, meta.OctName, i+1, meta.Args[i], argTypes[i])
				}
				effectiveArgTypes[i] = meta.Args[i]
				if !isOctxiliaryTransportType(meta.Args[i]) && !findTransportRecord(meta.TransportTypes, meta.Args[i]).ok {
					return "", "", false, fmt.Errorf("wrapper function %s.%s argument %d uses unsupported transport type %s", meta.PackageName, meta.OctName, i+1, meta.Args[i])
				}
			}
			if transport := findTransportRecord(meta.TransportTypes, meta.Return); !transport.ok && !isOctxiliaryTransportType(meta.Return) {
				return "", "", false, fmt.Errorf("wrapper function %s.%s return uses unsupported transport type %s", meta.PackageName, meta.OctName, meta.Return)
			}
			localType := effectiveReturn
			if meta.Fallible {
				localType = fallibleType(effectiveReturn)
			}
			tmp := c.temp(localType)
			c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRGenericOctxiliaryCall{Target: tmp, PackageName: meta.PackageName, OctName: meta.OctName, Family: meta.Family, WireName: meta.WireName, SidecarCommand: meta.SidecarCommand, Args: lowerMIRValues(args, nil), ArgTypes: effectiveArgTypes, RetType: effectiveReturn, Fallible: meta.Fallible, TransportTypes: meta.TransportTypes})
			return tmp, effectiveReturn, meta.Fallible, nil
		}
		if builtin && callee == "BoardSnapshot" {
			if len(argTypes) != 1 {
				return "", "", false, fmt.Errorf("BoardSnapshot expects 1 argument")
			}
			flowRet, ok := parseFlowInstanceType(argTypes[0])
			if !ok {
				return "", "", false, fmt.Errorf("BoardSnapshot expects FlowInstance argument")
			}
			snapshotType := ""
			for _, flowDecl := range c.pkg.Flows {
				if typeRefStringForPackage(c.pkg.Name, flowDecl.ReturnType) == flowRet && len(flowDecl.Board) > 0 {
					if snapshotType != "" {
						return "", "", false, fmt.Errorf("compiled BoardSnapshot requires unambiguous flow identity for return type %s", flowRet)
					}
					snapshotType = c.pkg.Name + "." + flowDecl.Name + "BoardSnapshot"
				}
			}
			if snapshotType == "" {
				return "", "", false, fmt.Errorf("compiled mode does not yet support builtin BoardSnapshot")
			}
			ret = snapshotType
		}
		if builtin && callee == "Len" && len(argTypes) == 1 {
			if table, _, ok := c.lookupRecordTable(argTypes[0]); ok {
				tmp := c.temp("Int")
				c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: tmp, Value: MIRLength{Value: MIRFieldAccess{Target: lowerMIRValue(args[0], ""), Field: table.Fields[0].Name}}})
				return tmp, "Int", false, nil
			}
		}
		localType := ret
		if fallible {
			localType = fallibleType(ret)
		}
		if !fallible && localType == "Void" {
			c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: "_", Callee: callee, Args: lowerMIRValues(args, nil), ArgTypes: argTypes, Builtin: builtin, RetType: ret})
			return "", ret, false, nil
		}
		tmp := c.temp(localType)
		c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRCall{Target: tmp, Callee: callee, Args: lowerMIRValues(args, nil), ArgTypes: argTypes, Builtin: builtin, RetType: ret})
		return tmp, ret, fallible, nil
	case ast.ArrayLiteralExpr:
		vals := []string{}
		typeName := "Int"
		hint, hasHint := c.expectedArrayElemType()
		if hasHint {
			typeName = hint
		}
		for i, el := range e.Elements {
			var v, t string
			var err error
			if hasHint {
				v, t, _, err = c.withExpectedType(hint, func() (string, string, bool, error) { return c.lowerExpr(el) })
			} else {
				v, t, _, err = c.lowerExpr(el)
			}
			if err != nil {
				return "", "", false, err
			}
			if hasHint {
				v = coerceExprToType(v, t, hint)
			} else if i == 0 {
				typeName = t
			}
			vals = append(vals, v)
		}
		tmp := c.temp(typeName + "[]")
		c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRConstructArray{Target: tmp, ElemType: typeName, Values: lowerMIRValues(vals, nil)})
		return tmp, typeName + "[]", false, nil
	case ast.VectorLiteralExpr:
		vals := make([]string, 0, len(e.Elements))
		elemType := "Int"
		for i, el := range e.Elements {
			v, t, _, err := c.lowerExpr(el)
			if err != nil {
				return "", "", false, err
			}
			vals = append(vals, v)
			if i == 0 {
				elemType = t
			}
		}
		vectorType := "Vector<" + elemType + ">"
		tmp := c.temp(vectorType)
		c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRConstructArray{Target: tmp, ElemType: elemType, Values: lowerMIRValues(vals, nil)})
		return tmp, vectorType, false, nil
	case ast.MatrixLiteralExpr:
		rows := make([]string, 0, len(e.Rows))
		elemType := "Int"
		hint, hasHint := c.expectedMatrixElemType()
		if hasHint {
			elemType = hint
		}
		for _, row := range e.Rows {
			rowVals := make([]string, 0, len(row))
			for j, cell := range row {
				var v, t string
				var err error
				if hasHint {
					v, t, _, err = c.withExpectedType(hint, func() (string, string, bool, error) { return c.lowerExpr(cell) })
				} else {
					v, t, _, err = c.lowerExpr(cell)
				}
				if err != nil {
					return "", "", false, err
				}
				if hasHint {
					v = coerceExprToType(v, t, hint)
				} else if len(rows) == 0 && j == 0 {
					elemType = t
				}
				rowVals = append(rowVals, v)
			}
			rowType := "Vector<" + elemType + ">"
			rowTmp := c.temp(rowType)
			c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRConstructArray{Target: rowTmp, ElemType: elemType, Values: lowerMIRValues(rowVals, nil)})
			rows = append(rows, rowTmp)
		}
		matrixType := "Matrix<" + elemType + ">"
		tmp := c.temp(matrixType)
		c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRConstructArray{Target: tmp, ElemType: "Vector<" + elemType + ">", Values: lowerMIRValues(rows, nil)})
		return tmp, matrixType, false, nil
	case ast.FieldAccessExpr:
		if enumType, variant, ok := c.flattenEnumVariantExpr(e); ok {
			enumValue, resolvedEnumType, enumFound, err := c.resolveEnumVariantValue(enumType, variant)
			if err != nil {
				return "", "", false, err
			}
			if enumFound {
				return enumValue, resolvedEnumType, false, nil
			}
		}
		t, targetType, _, err := c.lowerExpr(e.Target)
		if err != nil {
			return "", "", false, err
		}
		if _, ok := parseMatrixElemType(targetType); ok {
			switch e.Field {
			case "rows":
				tmp := c.temp("Int")
				c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: tmp, Value: MIRLength{Value: lowerMIRValue(t, targetType)}})
				return tmp, "Int", false, nil
			case "cols":
				tmp := c.temp("Int")
				c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: tmp, Value: MIRMatrixColumnCount{Value: lowerMIRValue(t, targetType)}})
				return tmp, "Int", false, nil
			}
		}
		fieldType := "Int"
		if resolvedType, ok := c.lookupRecordFieldType(targetType, e.Field); ok {
			fieldType = resolvedType
		}
		tmp := c.temp(fieldType)
		c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: tmp, Value: MIRFieldAccess{Target: lowerMIRValue(t, ""), Field: e.Field, Type: fieldType}})
		return tmp, fieldType, false, nil
	case ast.IndexExpr:
		target, targetType, _, err := c.lowerExpr(e.Target)
		if err != nil {
			return "", "", false, err
		}
		if matrixElem, ok := parseMatrixElemType(targetType); ok {
			if len(e.Indices) != 2 {
				return "", "", false, fmt.Errorf("compiled mode matrix indexing requires exactly 2 indices")
			}
			first, firstType, _, err := c.withExpectedType("", func() (string, string, bool, error) { return c.lowerExpr(e.Indices[0]) })
			if err != nil {
				return "", "", false, err
			}
			second, secondType, _, err := c.withExpectedType("", func() (string, string, bool, error) { return c.lowerExpr(e.Indices[1]) })
			if err != nil {
				return "", "", false, err
			}
			if firstType == "Int" && secondType == "Int" {
				tmp := c.temp(matrixElem)
				c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: tmp, Value: MIRIndex{Target: MIRIndex{Target: lowerMIRValue(target, targetType), Index: lowerMIRValue(first, firstType)}, Index: lowerMIRValue(second, secondType), Type: matrixElem}})
				return tmp, matrixElem, false, nil
			}
			if firstType == "Index" && secondType == "Index" {
				tmp := c.temp(targetType)
				c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: tmp, Value: lowerMIRValue(target, targetType)})
				c.setEinTermMeta(tmp, []string{first, second}, 2, targetType)
				return tmp, targetType, false, nil
			}
			return "", "", false, fmt.Errorf("compiled mode matrix indexing expects either [Int, Int] element access or [Index, Index] Einstein term access, got [%s, %s]", firstType, secondType)
		}
		if table, tablePkg, ok := c.lookupRecordTable(targetType); ok {
			if len(e.Indices) != 1 {
				return "", "", false, fmt.Errorf("compiled record table indexing requires exactly one Int index")
			}
			idx, idxType, _, err := c.lowerExpr(e.Indices[0])
			if err != nil {
				return "", "", false, err
			}
			if idxType != "Int" {
				return "", "", false, fmt.Errorf("compiled record table row index must be Int, got %s", idxType)
			}
			rowType := tablePkg + ".__oct_table_row_" + table.Name
			parts := make([]string, 0, len(table.Fields))
			for _, field := range table.Fields {
				parts = append(parts, fmt.Sprintf("%s: %s.%s[%s]", field.Name, target, field.Name, idx))
			}
			expr := fmt.Sprintf("func() %s { if %s < 0 || %s >= len(%s.%s) { panic(fmt.Sprintf(\"runtime error [OCT-RTBL005]: row index %%d out of bounds for record table '%s' of length %%d\", %s, len(%s.%s))) }; return %s{%s} }()", goType(rowType), idx, idx, target, table.Fields[0].Name, table.Name, idx, target, table.Fields[0].Name, goType(rowType), strings.Join(parts, ", "))
			tmp := c.temp(rowType)
			c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: tmp, Value: MIRBackendValue{Backend: "go", Expression: expr, Type: rowType, Reason: "checked-record-table-index"}})
			return tmp, rowType, false, nil
		}
		if len(e.Indices) != 1 {
			return "", "", false, fmt.Errorf("compiled mode only supports single-dimension indexing")
		}
		idx, idxType, _, err := c.withExpectedType("", func() (string, string, bool, error) { return c.lowerExpr(e.Indices[0]) })
		if err != nil {
			return "", "", false, err
		}
		if idxType == "Index" {
			if _, ok := parseVectorElemType(targetType); ok {
				tmp := c.temp(targetType)
				c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: tmp, Value: lowerMIRValue(target, targetType)})
				c.setEinTermMeta(tmp, []string{idx}, 1, targetType)
				return tmp, targetType, false, nil
			}
			return "", "", false, fmt.Errorf("compiled mode single-dimension indexing requires Int index, got %s", idxType)
		}
		if idxType != "Int" {
			return "", "", false, fmt.Errorf("compiled mode single-dimension indexing requires Int index, got %s", idxType)
		}
		elemType := strings.TrimSuffix(targetType, "[]")
		valueExpr := fmt.Sprintf("%s[%s]", target, idx)
		if targetType == "Bytes" {
			elemType = "Int"
			valueExpr = fmt.Sprintf("int(%s[%s])", target, idx)
		}
		if vectorElem, ok := parseVectorElemType(targetType); ok {
			elemType = vectorElem
		}
		tmp := c.temp(elemType)
		c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: tmp, Value: lowerMIRValue(valueExpr, elemType)})
		return tmp, elemType, false, nil
	case ast.RecordLiteralExpr:
		vals := []string{}
		names := []string{}
		typeName := e.TypeName
		if !strings.Contains(typeName, ".") {
			typeName = c.pkg.Name + "." + typeName
		}
		for _, f := range e.Fields {
			fieldType, hasFieldType := c.lookupRecordFieldType(typeName, f.Name)
			var v, t string
			var err error
			if hasFieldType {
				v, t, _, err = c.withExpectedType(fieldType, func() (string, string, bool, error) { return c.lowerExpr(f.Value) })
			} else {
				v, t, _, err = c.lowerExpr(f.Value)
			}
			if err != nil {
				return "", "", false, err
			}
			if hasFieldType {
				v = coerceExprToType(v, t, fieldType)
			}
			vals = append(vals, v)
			names = append(names, f.Name)
		}
		tmp := c.temp(typeName)
		if table, _, ok := c.lookupRecordTable(typeName); ok && len(vals) > 0 {
			byName := make(map[string]string, len(names))
			for idx, name := range names {
				byName[name] = vals[idx]
			}
			first := byName[table.Fields[0].Name]
			checks := make([]string, 0, len(table.Fields)-1)
			lengthArgs := make([]string, 0, len(table.Fields))
			literalParts := make([]string, 0, len(table.Fields))
			for idx, field := range table.Fields {
				value := byName[field.Name]
				if idx > 0 {
					checks = append(checks, fmt.Sprintf("len(%s) != len(%s)", value, first))
				}
				lengthArgs = append(lengthArgs, value)
				literalParts = append(literalParts, fmt.Sprintf("%s: %s", field.Name, value))
			}
			formatParts := make([]string, 0, len(table.Fields))
			for _, field := range table.Fields {
				formatParts = append(formatParts, field.Name+": %d")
			}
			guard := "false"
			if len(checks) > 0 {
				guard = strings.Join(checks, " || ")
			}
			lenCalls := make([]string, 0, len(lengthArgs))
			for _, value := range lengthArgs {
				lenCalls = append(lenCalls, "len("+value+")")
			}
			valueExpr := fmt.Sprintf("func() %s { if %s { panic(fmt.Sprintf(\"runtime error [OCT-RTBL003]: record table '%s' columns have inconsistent lengths: %s\", %s)) }; return %s{%s} }()", goType(typeName), guard, table.Name, strings.Join(formatParts, ", "), strings.Join(lenCalls, ", "), goType(typeName), strings.Join(literalParts, ", "))
			c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: tmp, Value: MIRBackendValue{Backend: "go", Expression: valueExpr, Type: typeName, Reason: "checked-record-table-construction"}})
			return tmp, typeName, false, nil
		}
		c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRConstructRecord{Target: tmp, TypeName: typeName, FieldNames: names, FieldVals: lowerMIRValues(vals, nil)})
		return tmp, typeName, false, nil
	case ast.RecordUpdateExpr:
		source, sourceType, _, err := c.lowerExpr(e.Source)
		if err != nil {
			return "", "", false, err
		}
		fieldTypes, ok := c.lookupRecordFields(sourceType)
		if !ok {
			return "", "", false, fmt.Errorf("record update requires record source")
		}
		overrides := make(map[string]string, len(e.Fields))
		for _, field := range e.Fields {
			value, _, _, err := c.lowerExpr(field.Value)
			if err != nil {
				return "", "", false, err
			}
			overrides[field.Name] = value
		}
		names := make([]string, 0, len(fieldTypes))
		values := make([]string, 0, len(fieldTypes))
		for _, field := range fieldTypes {
			names = append(names, field.Name)
			if override, exists := overrides[field.Name]; exists {
				values = append(values, override)
				continue
			}
			values = append(values, fmt.Sprintf("%s.%s", source, field.Name))
		}
		tmp := c.temp(sourceType)
		if table, _, isTable := c.lookupRecordTable(sourceType); isTable && len(table.Fields) > 0 {
			checks := make([]string, 0, len(e.Fields))
			for _, field := range e.Fields {
				checks = append(checks, fmt.Sprintf("if len(%s) != len(%s.%s) { panic(fmt.Sprintf(\"runtime error [OCT-RTBL004]: record table '%s' replacement column '%s' has extent %%d; expected %%d\", len(%s), len(%s.%s))) }", overrides[field.Name], source, table.Fields[0].Name, table.Name, field.Name, overrides[field.Name], source, table.Fields[0].Name))
			}
			parts := make([]string, 0, len(names))
			for index := range names {
				parts = append(parts, fmt.Sprintf("%s: %s", names[index], values[index]))
			}
			valueExpr := fmt.Sprintf("func() %s { %s; return %s{%s} }()", goType(sourceType), strings.Join(checks, "; "), goType(sourceType), strings.Join(parts, ", "))
			c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: tmp, Value: MIRBackendValue{Backend: "go", Expression: valueExpr, Type: sourceType, Reason: "checked-record-table-update"}})
		} else {
			overrideNames := make([]string, 0, len(e.Fields))
			for _, field := range e.Fields {
				overrideNames = append(overrideNames, field.Name)
			}
			c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRConstructRecord{
				Target:              tmp,
				TypeName:            sourceType,
				FieldNames:          names,
				FieldVals:           lowerMIRValues(values, nil),
				TemplateOrigin:      c.recordTemplateOrigin(sourceType),
				TemplateOverrideSet: overrideNames,
			})
		}
		return tmp, sourceType, false, nil
	case ast.EnumValueExpr:
		enumType := e.EnumName
		if !strings.Contains(enumType, ".") {
			enumType = c.pkg.Name + "." + enumType
		}
		return fmt.Sprintf("%s_%s", e.EnumName, e.Variant), enumType, false, nil
	case ast.IfExpr:
		return c.lowerIfExpr(e)
	case ast.SwitchExpr:
		return c.lowerSwitchExpr(e)
	case ast.MatchExpr:
		return c.lowerMatchExpr(e)
	case ast.RangeExpr:
		return c.lowerRangeExpr(e)
	case ast.PropagateExpr:
		return c.lowerPropagateExpr(e)
	case ast.UnwrapExpr:
		return c.lowerUnwrapExpr(e)
	case ast.BatchExpr:
		return c.lowerBatchExpr(e)
	case ast.UtilityWhenExpr:
		if e.EnumTarget != nil && utilityWhenHasPayloadCandidate(e) {
			return "", "", false, unsupported("compiled enum-targeted utility payload candidates require delayed payload lowering")
		}
		h, _, _, err := c.lowerExpr(e.Policy.Hysteresis)
		if err != nil {
			return "", "", false, err
		}
		m, _, _, err := c.lowerExpr(e.Policy.MinCommit)
		if err != nil {
			return "", "", false, err
		}
		elseExpr, resultType, _, err := c.lowerExpr(e.Else)
		if err != nil {
			return "", "", false, err
		}
		cases := make([]string, 0, len(e.Cases))
		valueType := goType(resultType)
		for _, wc := range e.Cases {
			v, _, _, err := c.lowerExpr(wc.Value)
			if err != nil {
				return "", "", false, err
			}
			cond, _, _, err := c.lowerExpr(wc.Condition)
			if err != nil {
				return "", "", false, err
			}
			score, _, _, err := c.lowerExpr(wc.Score)
			if err != nil {
				return "", "", false, err
			}
			cases = append(cases, fmt.Sprintf("{Valid: %s, Value: %s, Score: %s}", cond, v, score))
		}
		c.usesUtilityWhen = true
		return fmt.Sprintf("__octUtilSelect[%s](map[int]__octUtilitySiteState{}, %d, %s, %s, []__octUtilCandidate[%s]{%s}, %s)",
			valueType, e.SiteID, h, m, valueType, strings.Join(cases, ", "), elseExpr), resultType, false, nil
	case ast.ParenExpr:
		return c.lowerExpr(e.Inner)
	default:
		return "", "", false, fmt.Errorf("unsupported expression %T", e)
	}
}

func utilityWhenHasPayloadCandidate(e ast.UtilityWhenExpr) bool {
	if _, ok := e.Else.(ast.CallExpr); ok {
		return true
	}
	for _, c := range e.Cases {
		if _, ok := c.Value.(ast.CallExpr); ok {
			return true
		}
	}
	return false
}

func (c *lowerCtx) lowerBatchExpr(e ast.BatchExpr) (string, string, bool, error) {
	input, inputType, _, err := c.lowerExpr(e.Input)
	if err != nil {
		return "", "", false, err
	}
	if !strings.HasSuffix(inputType, "[]") {
		return "", "", false, fmt.Errorf("batch input must be an array")
	}
	itemType := strings.TrimSuffix(inputType, "[]")
	worker, resultType, captureNames, err := c.lowerBatchWorker(e, itemType)
	if err != nil {
		return "", "", false, err
	}
	c.extra = append(c.extra, worker)

	raw := c.temp(fallibleType(resultType + "[]"))
	c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRBatchMap{
		Target:     raw,
		Input:      lowerMIRValue(input, inputType),
		Worker:     worker.Package + "." + worker.Name,
		InputType:  itemType,
		ResultType: resultType,
		Captures:   lowerMIRValues(captureNames, nil),
		Nested:     c.batchDepth > 0,
	})
	value := c.temp(resultType + "[]")
	okID := len(c.blocks)
	c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", okID)})
	errID := len(c.blocks)
	c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", errID)})
	mergeID := len(c.blocks)
	c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", mergeID)})
	c.blocks[c.cur].Terminator = MIRBranch{Cond: MIRFieldAccess{Target: lowerMIRValue(raw, resultType), Field: "IsErr", Type: "Bool"}, TrueTarget: c.blocks[errID].Label, FalseTarget: c.blocks[okID].Label}

	c.cur = okID
	c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: value, Value: MIRFieldAccess{Target: lowerMIRValue(raw, resultType), Field: "Value", Type: resultType}})
	c.blocks[c.cur].Terminator = MIRJump{Target: c.blocks[mergeID].Label}

	c.cur = errID
	if c.fn.IsFallible {
		c.blocks[c.cur].Terminator = MIRReturn{Value: MIRResultValue{ResultType: c.retType, Error: MIRFieldAccess{Target: lowerMIRValue(raw, resultType), Field: "Err", Type: "Error"}, IsError: true}}
	} else {
		c.blocks[c.cur].Terminator = MIRFail{Value: MIRBinary{Op: "+", Left: mirString("oct error: "), Right: MIRFieldAccess{Target: lowerMIRValue(raw, resultType), Field: "Err", Type: "Error"}, Type: "String"}}
	}
	c.cur = mergeID
	return value, resultType + "[]", false, nil
}

func (c *lowerCtx) lowerFunctionExpr(e ast.FunctionExpr) (string, string, bool, error) {
	id := c.anonymousID
	c.anonymousID++
	name := goSafeName(c.fn.Name) + "_" + internalName(internalAnonymous, id)

	captureFields := make([]MIRField, 0, len(e.Captures))
	captureEnvironment := make([]MIRCapture, 0, len(e.Captures))
	captureArgs := make([]string, 0, len(e.Captures))
	workerLocals := make(map[string]string, len(e.Captures)+len(e.Parameters))
	workerGoNames := make(map[string]string, len(e.Captures)+len(e.Parameters))
	for index, capture := range e.Captures {
		value, typ, fallible, err := c.lowerExpr(capture.Value)
		if err != nil {
			return "", "", false, err
		}
		if fallible {
			return "", "", false, fmt.Errorf("capture '%s' must handle fallible value", capture.Name)
		}
		tmp := c.temp(typ)
		c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: tmp, Value: lowerMIRValueWithClone(value, typ)})
		goName := fmt.Sprintf("%s_%d", internalName(internalBatchCapture, id), index)
		captureFields = append(captureFields, MIRField{Name: goName, Type: typ})
		captureEnvironment = append(captureEnvironment, MIRCapture{Name: capture.Name, Parameter: goName, Type: typ})
		captureArgs = append(captureArgs, tmp)
		workerLocals[capture.Name] = typ
		workerGoNames[capture.Name] = goName
	}

	params := append([]MIRField(nil), captureFields...)
	functionParamNames := make([]string, 0, len(e.Parameters))
	functionParamDecls := make([]string, 0, len(e.Parameters))
	functionParamTypes := make([]string, 0, len(e.Parameters))
	for index, parameter := range e.Parameters {
		typ := typeRefStringForPackage(c.pkg.Name, parameter.Type)
		goName := fmt.Sprintf("%s_param_%d", internalName(internalAnonymous, id), index)
		workerLocals[parameter.Name] = typ
		workerGoNames[parameter.Name] = goName
		params = append(params, MIRField{Name: goName, Type: typ})
		functionParamNames = append(functionParamNames, goName)
		functionParamDecls = append(functionParamDecls, fmt.Sprintf("%s %s", goName, goType(typ)))
		functionParamTypes = append(functionParamTypes, typ)
	}
	returnType := typeRefStringForPackage(c.pkg.Name, e.ReturnType)
	decl := ast.FunctionDecl{Name: name, Parameters: e.Parameters, ReturnType: e.ReturnType, IsFallible: e.IsFallible}
	if e.ErrorType != nil {
		decl.ErrorType = *e.ErrorType
	}
	wctx := &lowerCtx{pkg: c.pkg, program: c.program, locals: workerLocals, goNames: workerGoNames, blocks: []MIRBlock{{Label: "entry"}}, cur: 0, retType: returnType, fn: decl, einTerms: map[string]einsteinTermMeta{}}
	if err := wctx.lowerBlock(e.Body); err != nil {
		return "", "", false, err
	}
	if wctx.blocks[wctx.cur].Terminator == nil {
		if returnType == "Void" {
			if e.IsFallible {
				wctx.blocks[wctx.cur].Terminator = MIRReturn{Value: lowerMIRValue(fallibleOkValue(returnType, ""), fallibleType(returnType))}
			} else {
				wctx.blocks[wctx.cur].Terminator = MIRReturn{}
			}
		} else {
			return "", "", false, fmt.Errorf("anonymous function missing return")
		}
	}
	c.extra = append(c.extra, wctx.extra...)
	worker := MIRFunction{Package: c.pkg.Name, Name: name, Params: params, CaptureEnv: captureEnvironment, Return: returnType, IsFallible: e.IsFallible, ErrorType: typeRefStringForPackage(c.pkg.Name, decl.ErrorType), Blocks: wctx.blocks, UsesUtilityWhen: wctx.usesUtilityWhen}
	paramSet := make(map[string]struct{}, len(params))
	for _, parameter := range params {
		paramSet[parameter.Name] = struct{}{}
	}
	for sourceName, typ := range wctx.locals {
		goName := wctx.goLocalName(sourceName)
		if _, isParam := paramSet[goName]; !isParam {
			worker.Locals = append(worker.Locals, MIRField{Name: goName, Type: typ})
		}
	}
	sort.Slice(worker.Locals, func(i, j int) bool { return worker.Locals[i].Name < worker.Locals[j].Name })
	c.extra = append(c.extra, worker)

	functionType := "fn(" + strings.Join(functionParamTypes, ", ") + ") -> " + returnType
	goReturn := goType(returnType)
	if e.IsFallible {
		functionType += " ! Error"
		goReturn = goResultTypeName(returnType)
	}
	callArgs := append(append([]string(nil), captureArgs...), functionParamNames...)
	workerSymbol := "fn_" + c.pkg.Name + "_" + name
	body := fmt.Sprintf("return %s(%s)", workerSymbol, strings.Join(callArgs, ", "))
	if goReturn == "" {
		body = fmt.Sprintf("%s(%s)", workerSymbol, strings.Join(callArgs, ", "))
	}
	returnClause := ""
	if goReturn != "" {
		returnClause = " " + goReturn
	}
	closure := fmt.Sprintf("func(%s)%s { %s }", strings.Join(functionParamDecls, ", "), returnClause, body)
	if len(captureFields) > 0 {
		captureDecls := make([]string, 0, len(captureFields))
		for _, field := range captureFields {
			captureDecls = append(captureDecls, fmt.Sprintf("%s %s", field.Name, goType(field.Type)))
		}
		closure = fmt.Sprintf("func(%s) %s { return %s }(%s)", strings.Join(captureDecls, ", "), goType(functionType), closure, strings.Join(captureArgs, ", "))
	}
	return closure, functionType, false, nil
}

func (c *lowerCtx) lowerBatchWorker(e ast.BatchExpr, itemType string) (MIRFunction, string, []string, error) {
	name := c.internalBatchWorkerName()
	c.batchID++
	const retPlaceholder = "__oct_batch_ret__"
	workerDecl := ast.FunctionDecl{Name: name, IsFallible: true, ErrorType: ast.TypeRef{Name: "Error"}}
	captureNames := make([]string, 0)
	for _, name := range batchcapture.Names(e.Body, e.ItemName) {
		if _, ok := c.locals[name]; ok {
			captureNames = append(captureNames, name)
		}
	}
	workerLocals := make(map[string]string, len(captureNames)+1)
	workerGoNames := make(map[string]string, len(captureNames)+1)
	workerLocals[e.ItemName] = itemType
	workerGoNames[e.ItemName] = internalName(internalBatchItem, c.batchID)
	captureGoNames := make([]string, 0, len(captureNames))
	for captureIndex, captureName := range captureNames {
		workerLocals[captureName] = c.locals[captureName]
		workerGoNames[captureName] = fmt.Sprintf("%s_%d", internalName(internalBatchCapture, c.batchID), captureIndex)
		captureGoNames = append(captureGoNames, c.goLocalName(captureName))
	}
	wctx := &lowerCtx{
		pkg:        c.pkg,
		program:    c.program,
		locals:     workerLocals,
		goNames:    workerGoNames,
		blocks:     []MIRBlock{{Label: "entry"}},
		cur:        0,
		retType:    retPlaceholder,
		fn:         workerDecl,
		batchDepth: c.batchDepth + 1,
		einTerms:   map[string]einsteinTermMeta{},
	}
	if err := wctx.lowerBlock(e.Body); err != nil {
		return MIRFunction{}, "", nil, err
	}
	if wctx.blocks[wctx.cur].Terminator == nil {
		return MIRFunction{}, "", nil, fmt.Errorf("batch body missing return")
	}
	if wctx.lastRet == "" {
		return MIRFunction{}, "", nil, fmt.Errorf("batch body return type could not be inferred")
	}
	c.extra = append(c.extra, wctx.extra...)
	params := []MIRField{{Name: workerGoNames[e.ItemName], Type: itemType}}
	for _, captureName := range captureNames {
		params = append(params, MIRField{Name: workerGoNames[captureName], Type: c.locals[captureName]})
	}
	worker := MIRFunction{
		Package:    c.pkg.Name,
		Name:       name,
		Params:     params,
		Return:     wctx.lastRet,
		IsFallible: true,
		ErrorType:  "Error",
		Blocks:     patchBatchReturnType(wctx.blocks, retPlaceholder, wctx.lastRet),
	}
	paramNames := map[string]struct{}{}
	for _, param := range worker.Params {
		paramNames[param.Name] = struct{}{}
	}
	for n, t := range wctx.locals {
		if _, isParam := paramNames[wctx.goLocalName(n)]; isParam {
			continue
		}
		worker.Locals = append(worker.Locals, MIRField{Name: wctx.goLocalName(n), Type: t})
	}
	sort.Slice(worker.Locals, func(i, j int) bool { return worker.Locals[i].Name < worker.Locals[j].Name })
	return worker, wctx.lastRet, captureGoNames, nil
}

func (c *lowerCtx) internalBatchWorkerName() string {
	for id := c.batchID; ; id++ {
		// batchID is scoped to one lowering context. Include the owning function
		// so independent batch expressions lowered from different functions do
		// not emit duplicate package-level Go helpers.
		candidate := fmt.Sprintf("%s_%s", internalName(internalBatchWorker, id), c.fn.Name)
		conflicts := false
		for _, fn := range c.pkg.Functions {
			if fn.Name == candidate {
				conflicts = true
				break
			}
		}
		if !conflicts {
			for _, fn := range c.extra {
				if fn.Name == candidate {
					conflicts = true
					break
				}
			}
		}
		if !conflicts {
			return candidate
		}
	}
}

func patchBatchReturnType(blocks []MIRBlock, from, to string) []MIRBlock {
	out := make([]MIRBlock, len(blocks))
	for i, block := range blocks {
		out[i] = block
		if ret, ok := block.Terminator.(MIRReturn); ok {
			ret.Value = rewriteMIRLocal(ret.Value, from, to)
			out[i].Terminator = ret
		}
	}
	return out
}

func (c *lowerCtx) lookupRecordFieldType(recordType, fieldName string) (string, bool) {
	pkgName := c.pkg.Name
	typeName := recordType
	if strings.Contains(typeName, ".") {
		parts := strings.SplitN(typeName, ".", 2)
		pkgName = parts[0]
		typeName = parts[1]
	}
	pkg, ok := c.program.Packages[pkgName]
	if !ok {
		return "", false
	}
	if strings.HasPrefix(typeName, "__flow_board_") {
		flowName := strings.TrimPrefix(typeName, "__flow_board_")
		for _, flow := range pkg.Flows {
			if flow.Name != flowName {
				continue
			}
			for _, field := range flow.Board {
				if field.Name == fieldName {
					return typeRefStringForPackage(pkgName, field.Type), true
				}
			}
			return "", false
		}
	}
	for _, record := range pkg.Records {
		isTableRow := record.IsTable && "__oct_table_row_"+record.Name == typeName
		if record.Name != typeName && !isTableRow {
			continue
		}
		for _, field := range record.Fields {
			if field.Name == fieldName {
				fieldType := typeRefStringForPackage(pkgName, field.Type)
				if record.IsTable && !isTableRow {
					fieldType += "[]"
				}
				return fieldType, true
			}
		}
	}
	for _, flow := range pkg.Flows {
		if flow.Name+"BoardSnapshot" != typeName {
			continue
		}
		for _, field := range flow.Board {
			if field.Name == fieldName {
				return typeRefStringForPackage(pkgName, field.Type), true
			}
		}
		return "", false
	}
	return "", false
}

func (c *lowerCtx) lookupRecordTable(recordType string) (ast.RecordDecl, string, bool) {
	pkgName := c.pkg.Name
	typeName := recordType
	if strings.Contains(typeName, ".") {
		parts := strings.SplitN(typeName, ".", 2)
		pkgName, typeName = parts[0], parts[1]
	}
	pkg, ok := c.program.Packages[pkgName]
	if !ok {
		return ast.RecordDecl{}, "", false
	}
	for _, record := range pkg.Records {
		if record.Name == typeName && record.IsTable {
			return record, pkgName, true
		}
	}
	return ast.RecordDecl{}, "", false
}

func (c *lowerCtx) lookupRecordFields(recordType string) ([]MIRField, bool) {
	pkgName := c.pkg.Name
	typeName := recordType
	if strings.Contains(typeName, ".") {
		parts := strings.SplitN(typeName, ".", 2)
		pkgName = parts[0]
		typeName = parts[1]
	}
	pkg, ok := c.program.Packages[pkgName]
	if !ok {
		return nil, false
	}
	for _, record := range pkg.Records {
		isTableRow := record.IsTable && "__oct_table_row_"+record.Name == typeName
		if record.Name != typeName && !isTableRow {
			continue
		}
		fields := make([]MIRField, 0, len(record.Fields))
		for _, field := range record.Fields {
			fieldType := typeRefStringForPackage(pkgName, field.Type)
			if record.IsTable && !isTableRow {
				fieldType += "[]"
			}
			fields = append(fields, MIRField{Name: field.Name, Type: fieldType})
		}
		return fields, true
	}
	return nil, false
}

func (c *lowerCtx) recordTemplateOrigin(recordType string) string {
	pkgName := c.pkg.Name
	typeName := recordType
	if strings.Contains(typeName, ".") {
		parts := strings.SplitN(typeName, ".", 2)
		pkgName, typeName = parts[0], parts[1]
	}
	pkg, ok := c.program.Packages[pkgName]
	if !ok {
		return ""
	}
	for _, record := range pkg.Records {
		if record.Name != typeName || record.TemplateOrigin == nil {
			continue
		}
		args := make([]string, len(record.TemplateOrigin.TypeArguments))
		for i := range record.TemplateOrigin.TypeArguments {
			args[i] = typeRefStringForPackage(pkgName, record.TemplateOrigin.TypeArguments[i])
		}
		return record.TemplateOrigin.Package + "." + record.TemplateOrigin.Declaration + "<" + strings.Join(args, ", ") + ">"
	}
	return ""
}

func (c *lowerCtx) lowerIfExpr(e ast.IfExpr) (string, string, bool, error) {
	cond, _, _, err := c.lowerExpr(e.Condition)
	if err != nil {
		return "", "", false, err
	}
	thenVal, thenType, _, err := c.lowerExpr(e.ThenExpr)
	if err != nil {
		return "", "", false, err
	}
	elseVal, _, _, err := c.lowerExpr(e.ElseExpr)
	if err != nil {
		return "", "", false, err
	}
	out := c.temp(thenType)
	thenID := len(c.blocks)
	c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", thenID)})
	elseID := len(c.blocks)
	c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", elseID)})
	mergeID := len(c.blocks)
	c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", mergeID)})
	c.blocks[c.cur].Terminator = MIRBranch{Cond: lowerMIRValue(cond, "Bool"), TrueTarget: c.blocks[thenID].Label, FalseTarget: c.blocks[elseID].Label}
	c.cur = thenID
	c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: out, Value: lowerMIRValue(thenVal, thenType)})
	c.blocks[c.cur].Terminator = MIRJump{Target: c.blocks[mergeID].Label}
	c.cur = elseID
	c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: out, Value: lowerMIRValue(elseVal, thenType)})
	c.blocks[c.cur].Terminator = MIRJump{Target: c.blocks[mergeID].Label}
	c.cur = mergeID
	return out, thenType, false, nil
}

func (c *lowerCtx) lowerSwitchExpr(e ast.SwitchExpr) (string, string, bool, error) {
	mergeID := len(c.blocks)
	c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", mergeID)})

	var (
		subject    string
		out        string
		resultType string
	)
	if e.Subject != nil {
		var err error
		subject, _, _, err = c.lowerExpr(e.Subject)
		if err != nil {
			return "", "", false, err
		}
	}

	assignResult := func(valueExpr ast.Expr) error {
		value, valueType, _, err := c.lowerExpr(valueExpr)
		if err != nil {
			return err
		}
		if out == "" {
			out = c.temp(valueType)
			resultType = valueType
		}
		c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: out, Value: lowerMIRValue(value, valueType)})
		return nil
	}

	for _, switchCase := range e.Cases {
		var cond string
		if e.Subject == nil {
			condValue, _, _, err := c.lowerExpr(switchCase.Match)
			if err != nil {
				return "", "", false, err
			}
			cond = condValue
		} else {
			matchValue, _, _, err := c.lowerExpr(switchCase.Match)
			if err != nil {
				return "", "", false, err
			}
			cond = c.temp("Bool")
			c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{
				Target: cond,
				Value:  MIRBinary{Op: "==", Left: lowerMIRValue(subject, ""), Right: lowerMIRValue(matchValue, ""), Type: "Bool"},
			})
		}

		matchID := len(c.blocks)
		c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", matchID)})
		nextID := len(c.blocks)
		c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", nextID)})
		c.blocks[c.cur].Terminator = MIRBranch{
			Cond:        lowerMIRValue(cond, "Bool"),
			TrueTarget:  c.blocks[matchID].Label,
			FalseTarget: c.blocks[nextID].Label,
		}

		c.cur = matchID
		if err := assignResult(switchCase.Value); err != nil {
			return "", "", false, err
		}
		c.blocks[c.cur].Terminator = MIRJump{Target: c.blocks[mergeID].Label}
		c.cur = nextID
	}

	if e.Else != nil {
		if err := assignResult(e.Else); err != nil {
			return "", "", false, err
		}
		c.blocks[c.cur].Terminator = MIRJump{Target: c.blocks[mergeID].Label}
	} else {
		c.blocks[c.cur].Terminator = MIRFail{Value: mirString("non-exhaustive switch reached in compiled mode")}
	}

	c.cur = mergeID
	return out, resultType, false, nil
}

func (c *lowerCtx) lowerMatchExpr(e ast.MatchExpr) (string, string, bool, error) {
	subject, subjectType, _, err := c.lowerExpr(e.Subject)
	if err != nil {
		return "", "", false, err
	}
	mergeID := len(c.blocks)
	c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", mergeID)})
	var out, resultType string
	nextID := c.cur
	for i, matchCase := range e.Cases {
		matchID := len(c.blocks)
		c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", matchID)})
		fallthroughID := len(c.blocks)
		c.blocks = append(c.blocks, MIRBlock{Label: fmt.Sprintf("b%d", fallthroughID)})
		cond := c.temp("Bool")
		c.blocks[nextID].Statements = append(c.blocks[nextID].Statements, MIRAssign{Target: cond, Value: MIRIntrinsicValue{Kind: "enum-is", Type: "Bool", Args: []MIRValue{lowerMIRValue(subject, subjectType)}, Metadata: []string{subjectType, matchCase.Variant}}})
		c.blocks[nextID].Terminator = MIRBranch{Cond: mirLocal(cond, "Bool"), TrueTarget: c.blocks[matchID].Label, FalseTarget: c.blocks[fallthroughID].Label}
		c.cur = matchID
		if matchCase.Binding != "" {
			bindingType, ok := c.lookupEnumVariantPayloadType(subjectType, matchCase.Variant)
			if !ok {
				bindingType = "any"
			}
			c.locals[matchCase.Binding] = bindingType
			c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: matchCase.Binding, Value: MIREnumPayload{Value: lowerMIRValue(subject, subjectType), PayloadType: bindingType}})
		}
		value, valueType, _, err := c.lowerExpr(matchCase.Value)
		if err != nil {
			return "", "", false, err
		}
		if out == "" {
			out = c.temp(valueType)
			resultType = valueType
		}
		c.blocks[c.cur].Statements = append(c.blocks[c.cur].Statements, MIRAssign{Target: out, Value: lowerMIRValue(value, valueType)})
		c.blocks[c.cur].Terminator = MIRJump{Target: c.blocks[mergeID].Label}
		nextID = fallthroughID
		if i == len(e.Cases)-1 {
			c.blocks[nextID].Terminator = MIRFail{Value: mirString("non-exhaustive match reached in compiled mode")}
		}
	}
	c.cur = mergeID
	return out, resultType, false, nil
}

func (c *lowerCtx) genericWrapperMetadataForCallee(callee ast.Expr) (genericWrapperCallMetadata, bool, error) {
	switch x := callee.(type) {
	case ast.IdentifierExpr:
		meta, ok := findGenericWrapperFunction(c.pkg, x.Name)
		return meta, ok, nil
	case ast.FieldAccessExpr:
		pkgIdent, ok := x.Target.(ast.IdentifierExpr)
		if !ok {
			return genericWrapperCallMetadata{}, false, nil
		}
		importPkg, ok := c.program.Packages[pkgIdent.Name]
		if !ok {
			return genericWrapperCallMetadata{}, false, nil
		}
		meta, found := findGenericWrapperFunction(importPkg, x.Field)
		return meta, found, nil
	default:
		return genericWrapperCallMetadata{}, false, nil
	}
}

type compiledFunctionSignature struct {
	Parameters []string
	ReturnType string
	Fallible   bool
}

func (c *lowerCtx) resolveNamedFunctionValue(expr ast.Expr) (string, string, bool) {
	switch fn := expr.(type) {
	case ast.IdentifierExpr:
		for _, declared := range c.pkg.Functions {
			if declared.Name == fn.Name {
				return "fn_" + strings.ReplaceAll(c.pkg.Name+"."+fn.Name, ".", "_"), functionTypeStringForDecl(c.pkg.Name, declared), true
			}
		}
	case ast.FieldAccessExpr:
		pkgIdent, ok := fn.Target.(ast.IdentifierExpr)
		if !ok {
			return "", "", false
		}
		importPkg, ok := c.program.Packages[pkgIdent.Name]
		if !ok {
			return "", "", false
		}
		for _, declared := range importPkg.Functions {
			if declared.Name == fn.Field {
				return "fn_" + strings.ReplaceAll(pkgIdent.Name+"."+fn.Field, ".", "_"), functionTypeStringForDecl(pkgIdent.Name, declared), true
			}
		}
	}
	return "", "", false
}

func functionTypeStringForDecl(pkgName string, fn ast.FunctionDecl) string {
	parts := make([]string, 0, len(fn.Parameters))
	for _, param := range fn.Parameters {
		parts = append(parts, typeRefStringForPackage(pkgName, param.Type))
	}
	result := "fn(" + strings.Join(parts, ", ") + ") -> " + typeRefStringForPackage(pkgName, fn.ReturnType)
	if fn.IsFallible {
		result += " ! " + typeRefStringForPackage(pkgName, fn.ErrorType)
	}
	return result
}

func (c *lowerCtx) resolveFunctionValueCall(callee ast.Expr) (string, compiledFunctionSignature, bool, error) {
	if ident, ok := callee.(ast.IdentifierExpr); ok {
		if typ, local := c.locals[ident.Name]; local {
			signature, ok := parseCompiledFunctionType(typ)
			if !ok {
				return "", compiledFunctionSignature{}, false, nil
			}
			return c.goLocalName(ident.Name), signature, true, nil
		}
		return "", compiledFunctionSignature{}, false, nil
	}
	switch callee.(type) {
	case ast.CallExpr, ast.FunctionExpr, ast.ParenExpr:
	default:
		return "", compiledFunctionSignature{}, false, nil
	}
	value, typ, fallible, err := c.lowerExpr(callee)
	if err != nil {
		return "", compiledFunctionSignature{}, false, err
	}
	if fallible {
		return "", compiledFunctionSignature{}, false, fmt.Errorf("fallible expression cannot be called without handling")
	}
	signature, ok := parseCompiledFunctionType(typ)
	return value, signature, ok, nil
}

func parseCompiledFunctionType(typ string) (compiledFunctionSignature, bool) {
	if !strings.HasPrefix(typ, "fn(") {
		return compiledFunctionSignature{}, false
	}
	depth := 0
	closeIndex := -1
	for index := len("fn"); index < len(typ); index++ {
		switch typ[index] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				closeIndex = index
				index = len(typ)
			}
		}
	}
	if closeIndex < 0 || !strings.HasPrefix(typ[closeIndex:], ") -> ") {
		return compiledFunctionSignature{}, false
	}
	paramsText := typ[len("fn("):closeIndex]
	rest := typ[closeIndex+len(") -> "):]
	fallible := false
	returnType := rest
	if bang := topLevelFunctionBang(rest); bang >= 0 {
		fallible = true
		returnType = rest[:bang]
	}
	params := []string{}
	if strings.TrimSpace(paramsText) != "" {
		for _, part := range splitTopLevelTypes(paramsText) {
			params = append(params, strings.TrimSpace(part))
		}
	}
	return compiledFunctionSignature{Parameters: params, ReturnType: strings.TrimSpace(returnType), Fallible: fallible}, true
}

func topLevelFunctionBang(text string) int {
	depth := 0
	for index := 0; index+2 < len(text); index++ {
		switch text[index] {
		case '(', '<':
			depth++
		case ')', '>':
			depth--
		}
		if depth == 0 && strings.HasPrefix(text[index:], " ! ") {
			return index
		}
	}
	return -1
}

func splitTopLevelTypes(text string) []string {
	parts := []string{}
	depth, start := 0, 0
	for index := 0; index < len(text); index++ {
		switch text[index] {
		case '(', '<':
			depth++
		case ')', '>':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, text[start:index])
				start = index + 1
			}
		}
	}
	return append(parts, text[start:])
}

func (c *lowerCtx) resolveCallArgTypes(callee ast.Expr) []string {
	pkgName := c.pkg.Name
	fnName := ""
	switch x := callee.(type) {
	case ast.IdentifierExpr:
		fnName = x.Name
	case ast.FieldAccessExpr:
		pkgIdent, ok := x.Target.(ast.IdentifierExpr)
		if !ok {
			return nil
		}
		pkgName = pkgIdent.Name
		fnName = x.Field
	default:
		return nil
	}
	pkg, ok := c.program.Packages[pkgName]
	if !ok {
		return nil
	}
	for _, fn := range pkg.Functions {
		if fn.Name != fnName {
			continue
		}
		out := make([]string, 0, len(fn.Parameters))
		for _, param := range fn.Parameters {
			out = append(out, typeRefStringForPackage(pkgName, param.Type))
		}
		return out
	}
	return nil
}

func goCoerceArg(expr string, actual string, expected string) string {
	if isIntScalarTypeString(actual) && isFloatLikeType(expected) {
		return fmt.Sprintf("float64(%s)", expr)
	}
	if isIntArrayTypeString(actual) && isFloatArrayTypeString(expected) {
		return fmt.Sprintf("__octIntArrayToFloat(%s)", expr)
	}
	if expected == "Complex" && isNumericTypeString(actual) {
		return fmt.Sprintf("complex(float64(%s), 0)", expr)
	}
	return expr
}

func isFloatLikeType(t string) bool {
	return t == "Float" || (strings.HasPrefix(t, "Float<") && strings.HasSuffix(t, ">"))
}

func (c *lowerCtx) resolveCall(callee ast.Expr) (string, string, bool, bool, error) {
	switch x := callee.(type) {
	case ast.IdentifierExpr:
		switch x.Name {
		case "Step", "Active", "Result", "Complete", "StateHistory", "ResumeTarget", "BoardSnapshot", "DidYield", "Yielded":
			switch x.Name {
			case "Step":
				return "Step", "Int", true, false, nil
			case "Active":
				return "Active", "String", true, false, nil
			case "Result":
				return "Result", "", true, true, nil
			case "Complete":
				return "Complete", "Bool", true, false, nil
			case "DidYield":
				return "DidYield", "Bool", true, false, nil
			case "Yielded":
				return "Yielded", "", true, true, nil
			case "StateHistory":
				return "StateHistory", "String[]", true, false, nil
			case "ResumeTarget":
				return "ResumeTarget", "String", true, false, nil
			case "BoardSnapshot":
				return "BoardSnapshot", "", true, true, nil
			}
		}
		if x.Name == "Len" {
			return "Len", "Int", true, false, nil
		}
		if x.Name == "Append" {
			return "Append", "", true, false, nil
		}
		if x.Name == "Print" {
			return "Print", "Int", true, false, nil
		}
		if x.Name == "ToString" {
			return "ToString", "String", true, false, nil
		}
		if x.Name == "Float" {
			return "Float", "Float", true, false, nil
		}
		if x.Name == "Idx" {
			return "Idx", "Index", true, false, nil
		}
		if x.Name == "EinMul" || x.Name == "EinAdd" || x.Name == "EinSub" {
			return x.Name, "", true, false, nil
		}
		if x.Name == "Complex" {
			return "Complex", "Complex", true, false, nil
		}
		if x.Name == "FFT" {
			return "FFT", "Complex[]", true, true, nil
		}
		if x.Name == "Contains" || x.Name == "StartsWith" || x.Name == "EndsWith" {
			return x.Name, "Bool", true, false, nil
		}
		if x.Name == "Trim" || x.Name == "Lower" || x.Name == "Upper" || x.Name == "Join" {
			return x.Name, "String", true, false, nil
		}
		if x.Name == "TupleProbe" {
			return "TupleProbe", "(Int, Int)", true, false, nil
		}
		if x.Name == "BoolIntProbe" {
			return "BoolIntProbe", "(Bool, Int)", true, false, nil
		}
		if c.pkg.Name == "Random" {
			switch x.Name {
			case "RngSeed", "RandInt", "RandFloat01", "RandFloatRange", "RandBernoulli", "RandNormal", "Gaussian", "CryptoRandInt", "CryptoRandFloat01", "CryptoRandBytes":
				resolved, ret, _, fallible, err := c.resolveCall(ast.FieldAccessExpr{Target: ast.IdentifierExpr{Name: "Random"}, Field: x.Name})
				if err == nil {
					return resolved, ret, true, fallible, nil
				}
			}
		}
		for _, fn := range c.pkg.Functions {
			if fn.Name == x.Name {
				return c.pkg.Name + "." + x.Name, typeRefStringForPackage(c.pkg.Name, fn.ReturnType), false, fn.IsFallible, nil
			}
		}
		for _, flow := range c.pkg.Flows {
			if flow.Name == x.Name {
				inputType, yieldType := "", ""
				if flow.TurnInput != nil {
					inputType = typeRefStringForPackage(c.pkg.Name, flow.TurnInput.Type)
				}
				if flow.YieldType != nil {
					yieldType = typeRefStringForPackage(c.pkg.Name, *flow.YieldType)
				}
				return c.pkg.Name + "." + x.Name, flowInstanceTypeString(typeRefStringForPackage(c.pkg.Name, flow.ReturnType), inputType, yieldType), false, false, nil
			}
		}
		if meta, ok := findGenericWrapperFunction(c.pkg, x.Name); ok {
			ret := meta.Return
			if !strings.Contains(ret, ".") && findTransportRecord(meta.TransportTypes, ret).ok {
				ret = meta.PackageName + "." + ret
			}
			return x.Name, ret, true, meta.Fallible, nil
		}
		if builtin.IsName(x.Name) {
			normalized := x.Name
			if c.pkg.Name == "Random" {
				switch x.Name {
				case "RngSeed", "RandInt", "RandFloat01", "RandFloatRange", "RandBernoulli", "RandNormal", "Gaussian", "CryptoRandInt", "CryptoRandFloat01", "CryptoRandBytes":
					normalized = "Random." + x.Name
				}
			}
			switch normalized {
			case "Random.RngSeed":
				return normalized, "Random.Rng", true, false, nil
			case "Random.RandInt":
				return normalized, "Random.RandIntResult", true, false, nil
			case "Random.RandFloat01", "Random.RandFloatRange", "Random.RandNormal", "Random.Gaussian":
				return normalized, "Random.RandFloatResult", true, false, nil
			case "Random.RandBernoulli":
				return normalized, "Random.RandBoolResult", true, false, nil
			case "Random.CryptoRandInt":
				return normalized, "Int", true, true, nil
			case "Random.CryptoRandFloat01":
				return normalized, "Float", true, true, nil
			case "Random.CryptoRandBytes":
				return normalized, "Bytes", true, true, nil
			case "StringByteLength", "StringRuneCount", "StringJoin", "StringConcat", "StringFrom", "StringReplaceAll", "StringContains", "StringStartsWith", "StringEndsWith", "StringTrim", "StringSplitLines", "StringEscapeJSON", "StringQuoteJSON":
				ret := "String"
				switch normalized {
				case "StringByteLength", "StringRuneCount":
					ret = "Int"
				case "StringContains", "StringStartsWith", "StringEndsWith":
					ret = "Bool"
				case "StringSplitLines":
					ret = "String[]"
				}
				return normalized, ret, true, false, nil
			case "MarkdownH1", "MarkdownH2", "MarkdownH3", "MarkdownParagraph", "MarkdownBlank", "MarkdownHorizontalRule", "MarkdownBullets", "MarkdownNumbered", "MarkdownCodeBlock", "MarkdownCallout", "MarkdownImage", "MarkdownFigure", "MarkdownTable", "MarkdownTableWithColumns", "MarkdownKeyValueTable", "MarkdownSection", "MarkdownSubsection", "MarkdownReport", "MarkdownEscapeText", "MarkdownEscapeTableCell":
				return normalized, compiledMarkdownBuiltinReturnType(normalized), true, false, nil
			case "RoundToInt", "FloorToInt", "CeilToInt":
				return normalized, "Int", true, false, nil
			case "Pi", "E", "Sqrt", "Sin", "Cos", "Tan", "Asin", "Acos", "Atan", "Atan2", "Exp", "Ln", "Pow", "Log10", "Sinh", "Cosh", "Tanh", "BaseValue", "BaseUnit", "Clamp01":
				return normalized, "Float", true, false, nil
			case "Abs":
				return normalized, "Float", true, false, nil
			case "FormatFloat":
				return normalized, "String", true, false, nil
			case "Require":
				return normalized, "Void", true, false, nil
			case "ArrayCrossSection", "Array.CrossSection":
				return "ArrayCrossSection", "Void", true, false, nil
			case "ArrayWhere", "Array.Where":
				return "ArrayWhere", "Void", true, false, nil
			case "FileReadText":
				return normalized, "String", true, true, nil
			case "FileReadBytes":
				return normalized, "Bytes", true, true, nil
			case "FileReadLines", "DirectoryList":
				return normalized, "String[]", true, true, nil
			case "FileWriteText", "FileWriteLines", "FileWriteBytes":
				return normalized, "Int", true, true, nil
			case "FileExists":
				return normalized, "Bool", true, false, nil
			case "FileDelete", "DirectoryMake", "DirectoryMakeAll", "DirectoryRemoveAll":
				return normalized, "Int", true, true, nil
			case "JsonNormalize", "JsonParse", "JsonStringify", "JsonLoad":
				return normalized, "String", true, true, nil
			case "JsonSave":
				return normalized, "Int", true, true, nil
			case "CsvRead", "CsvReadRows":
				return normalized, "String[][]", true, true, nil
			case "CsvReadTable":
				return normalized, "Csv.Table", true, true, nil
			case "CsvReadMatrix":
				return normalized, "Float[][]", true, true, nil
			case "CsvWrite", "CsvWriteRows":
				return normalized, "Int", true, true, nil
			case "MakeExecRaw", "MakeExecInRaw":
				return normalized, "Make.ProcessResult", true, true, nil
			case "MakeToolRaw", "MakeReadTextRaw", "MakeHashFileRaw":
				return normalized, "String", true, true, nil
			case "MakeEnvRaw":
				return normalized, "Make.EnvValue", true, true, nil
			case "MakeExistsRaw", "MakeIsFileRaw", "MakeIsDirRaw":
				return normalized, "Bool", true, false, nil
			case "MakeMkdirAllRaw", "MakeRemoveRaw", "MakeCopyRaw", "MakeWriteTextRaw", "MakeModifiedTimeRaw":
				return normalized, "Int", true, true, nil
			case "MakeGlobRaw":
				return normalized, "String[]", true, true, nil
			case "PathJoin", "PathBaseName", "PathExtension", "PathStem", "PathParent", "PathClean":
				return normalized, "String", true, false, nil
			default:
				return "", "", false, false, unsupportedBuiltin(x.Name)
			}
		}
		return "", "", false, false, fmt.Errorf("unknown function '%s'", x.Name)
	case ast.FieldAccessExpr:
		pkgIdent, ok := x.Target.(ast.IdentifierExpr)
		if !ok {
			return "", "", false, false, fmt.Errorf("unsupported call target")
		}
		builtinName := pkgIdent.Name + "." + x.Field
		if aliasName, mapped := builtin.ResolveNamespacedAlias(pkgIdent.Name, x.Field); mapped {
			if isMarkdownCompiledBuiltin(aliasName) {
				return aliasName, compiledMarkdownBuiltinReturnType(aliasName), true, false, nil
			}
			if builtin.IsName(aliasName) {
				return c.resolveCompiledBuiltinAlias(aliasName)
			}
		}
		if builtin.IsName(builtinName) {
			canonical := canonicalCompiledBuiltinName(builtinName)
			if isMarkdownCompiledBuiltin(canonical) {
				return canonical, compiledMarkdownBuiltinReturnType(canonical), true, false, nil
			}
			switch builtinName {
			case "Query.First":
				return builtinName, "", true, true, nil
			case "Query.Any":
				return builtinName, "Bool", true, false, nil
			case "Query.Count":
				return builtinName, "Int", true, false, nil
			case "Random.RngSeed":
				return builtinName, "Random.Rng", true, false, nil
			case "Random.RandInt":
				return builtinName, "Random.RandIntResult", true, false, nil
			case "Random.RandFloat01":
				return builtinName, "Random.RandFloatResult", true, false, nil
			case "Random.RandFloatRange":
				return builtinName, "Random.RandFloatResult", true, false, nil
			case "Random.RandBernoulli":
				return builtinName, "Random.RandBoolResult", true, false, nil
			case "Random.RandNormal", "Random.Gaussian":
				return builtinName, "Random.RandFloatResult", true, false, nil
			case "Random.CryptoRandInt":
				return builtinName, "Int", true, true, nil
			case "Random.CryptoRandFloat01":
				return builtinName, "Float", true, true, nil
			case "Random.CryptoRandBytes":
				return builtinName, "Bytes", true, true, nil
			case "Array.CrossSection":
				return "ArrayCrossSection", "Void", true, false, nil
			case "Array.Where":
				return "ArrayWhere", "Void", true, false, nil
			default:
				ret := "String"
				switch canonical {
				case "StringByteLength", "StringRuneCount":
					ret = "Int"
				case "StringContains", "StringStartsWith", "StringEndsWith":
					ret = "Bool"
				case "StringSplitLines":
					ret = "String[]"
				}
				return canonical, ret, true, false, nil
			}
		}
		importPkg, ok := c.program.Packages[pkgIdent.Name]
		if !ok {
			return "", "", false, false, fmt.Errorf("unknown package '%s'", pkgIdent.Name)
		}
		for _, fn := range importPkg.Functions {
			if fn.Name == x.Field {
				return pkgIdent.Name + "." + x.Field, typeRefStringForPackage(pkgIdent.Name, fn.ReturnType), false, fn.IsFallible, nil
			}
		}
		return "", "", false, false, fmt.Errorf("unknown function '%s.%s'", pkgIdent.Name, x.Field)
	default:
		return "", "", false, false, fmt.Errorf("unsupported callee %T", callee)
	}
}

func (c *lowerCtx) resolveCompiledBuiltinAlias(name string) (string, string, bool, bool, error) {
	switch name {
	case "StringByteLength", "StringRuneCount", "StringJoin", "StringConcat", "StringFrom", "StringReplaceAll", "StringContains", "StringStartsWith", "StringEndsWith", "StringTrim", "StringSplitLines", "StringEscapeJSON", "StringQuoteJSON":
		ret := "String"
		switch name {
		case "StringByteLength", "StringRuneCount":
			ret = "Int"
		case "StringContains", "StringStartsWith", "StringEndsWith":
			ret = "Bool"
		case "StringSplitLines":
			ret = "String[]"
		}
		return name, ret, true, false, nil
	case "ArrayCrossSection":
		return "ArrayCrossSection", "Void", true, false, nil
	case "ArrayWhere":
		return "ArrayWhere", "Void", true, false, nil
	default:
		return "", "", false, false, unsupportedBuiltin(name)
	}
}

type genericWrapperCallMetadata struct {
	PackageName    string
	OctName        string
	Family         string
	WireName       string
	SidecarCommand string
	Args           []string
	Return         string
	Fallible       bool
	TransportTypes []project.TransportTypeMetadata
}

func findGenericWrapperFunction(pkg project.Package, fnName string) (genericWrapperCallMetadata, bool) {
	for _, wrapper := range pkg.Wrappers {
		for _, fn := range wrapper.Functions {
			if fn.OctName == fnName {
				return genericWrapperCallMetadata{
					PackageName:    pkg.Name,
					OctName:        fn.OctName,
					Family:         wrapper.Family,
					WireName:       fn.WireName,
					SidecarCommand: wrapper.SidecarCommand,
					Args:           append([]string(nil), fn.Args...),
					Return:         fn.Return,
					Fallible:       fn.Fallible,
					TransportTypes: append([]project.TransportTypeMetadata(nil), wrapper.TransportTypes...),
				}, true
			}
		}
	}
	return genericWrapperCallMetadata{}, false
}

func wrapperArgTypeMatches(manifestType string, actualType string) bool {
	if manifestType == actualType {
		return true
	}
	if strings.HasPrefix(manifestType, "Int<") && strings.HasSuffix(manifestType, ">") && actualType == "Int" {
		return true
	}
	return false
}

func isOctxiliaryTransportType(t string) bool {
	if strings.HasPrefix(t, "Int<") && strings.HasSuffix(t, ">") {
		return true
	}
	switch t {
	case "Void", "Int", "Float", "Bool", "String", "String[]", "String[][]", "Float[]", "Bytes":
		return true
	default:
		return false
	}
}

type transportRecordLookup struct {
	typ project.TransportTypeMetadata
	ok  bool
}

func findTransportRecord(types []project.TransportTypeMetadata, name string) transportRecordLookup {
	for _, typ := range types {
		if typ.Name == name || (!strings.Contains(typ.Name, ".") && strings.HasSuffix(name, "."+typ.Name)) {
			return transportRecordLookup{typ: typ, ok: true}
		}
	}
	return transportRecordLookup{}
}

func transportRuntimeBaseType(t string) string {
	if strings.HasPrefix(t, "Int<") && strings.HasSuffix(t, ">") {
		return "Int"
	}
	return t
}

func flattenDirectCallName(expr ast.Expr) (string, bool) {
	switch node := expr.(type) {
	case ast.IdentifierExpr:
		return node.Name, true
	case ast.FieldAccessExpr:
		left, ok := node.Target.(ast.IdentifierExpr)
		if !ok {
			return "", false
		}
		return left.Name + "." + node.Field, true
	default:
		return "", false
	}
}

func (c *lowerCtx) flattenEnumVariantExpr(expr ast.FieldAccessExpr) (string, string, bool) {
	enumType, ok := flattenEnumTypeExpr(expr.Target)
	if !ok {
		return "", "", false
	}
	return enumType, expr.Field, true
}

func flattenEnumTypeExpr(expr ast.Expr) (string, bool) {
	switch node := expr.(type) {
	case ast.IdentifierExpr:
		return node.Name, true
	case ast.FieldAccessExpr:
		pkgIdent, ok := node.Target.(ast.IdentifierExpr)
		if !ok {
			return "", false
		}
		return pkgIdent.Name + "." + node.Field, true
	default:
		return "", false
	}
}

func (c *lowerCtx) resolveEnumVariantValue(enumType string, variant string) (string, string, bool, error) {
	enumPkg := c.pkg.Name
	enumName := enumType
	if dot := strings.Index(enumType, "."); dot >= 0 {
		enumPkg = enumType[:dot]
		enumName = enumType[dot+1:]
	}
	pkg, ok := c.program.Packages[enumPkg]
	if !ok {
		return "", "", false, nil
	}
	for _, enumDecl := range pkg.Enums {
		if enumDecl.Name != enumName {
			continue
		}
		for _, declaredVariant := range enumDecl.Variants {
			if declaredVariant.Name == variant {
				return fmt.Sprintf("%s_%s{Tag: %s_%s_tag}", enumPkg, enumName, enumName, variant), enumPkg + "." + enumName, true, nil
			}
		}
		return "", "", true, fmt.Errorf("enum '%s' has no variant '%s'", enumType, variant)
	}
	return "", "", false, nil
}

func (c *lowerCtx) resolveEnumVariantConstructor(enumType string, variant string, args []ast.Expr) (string, string, bool, error) {
	enumPkg := c.pkg.Name
	enumName := enumType
	if dot := strings.Index(enumType, "."); dot >= 0 {
		enumPkg = enumType[:dot]
		enumName = enumType[dot+1:]
	}
	pkg, ok := c.program.Packages[enumPkg]
	if !ok {
		return "", "", false, nil
	}
	for _, enumDecl := range pkg.Enums {
		if enumDecl.Name != enumName {
			continue
		}
		for _, declaredVariant := range enumDecl.Variants {
			if declaredVariant.Name != variant {
				continue
			}
			if declaredVariant.Payload == nil {
				if len(args) != 0 {
					return "", "", true, fmt.Errorf("enum '%s' variant '%s' does not accept a payload", enumType, variant)
				}
				return fmt.Sprintf("%s_%s{Tag: %s_%s_tag}", enumPkg, enumName, enumName, variant), enumPkg + "." + enumName, true, nil
			}
			if len(args) != 1 {
				return "", "", true, fmt.Errorf("enum '%s' variant '%s' requires exactly 1 payload argument", enumType, variant)
			}
			payload, _, _, err := c.lowerExpr(args[0])
			if err != nil {
				return "", "", true, err
			}
			return fmt.Sprintf("%s_%s{Tag: %s_%s_tag, Payload: %s}", enumPkg, enumName, enumName, variant, payload), enumPkg + "." + enumName, true, nil
		}
		return "", "", true, fmt.Errorf("enum '%s' has no variant '%s'", enumType, variant)
	}
	return "", "", false, nil
}

func enumShortName(enumType string) string {
	if dot := strings.Index(enumType, "."); dot >= 0 {
		return enumType[dot+1:]
	}
	return enumType
}

func (c *lowerCtx) lookupEnumVariantPayloadType(enumType string, variant string) (string, bool) {
	return lookupEnumVariantPayloadTypeForProgram(c.program, c.pkg.Name, enumType, variant)
}

func lookupEnumVariantPayloadTypeForProgram(program project.Program, currentPkg string, enumType string, variant string) (string, bool) {
	enumPkg := currentPkg
	enumName := enumType
	if dot := strings.Index(enumType, "."); dot >= 0 {
		enumPkg = enumType[:dot]
		enumName = enumType[dot+1:]
	}
	pkg, ok := program.Packages[enumPkg]
	if !ok {
		return "", false
	}
	for _, enumDecl := range pkg.Enums {
		if enumDecl.Name != enumName {
			continue
		}
		for _, declaredVariant := range enumDecl.Variants {
			if declaredVariant.Name == variant && declaredVariant.Payload != nil {
				return typeRefStringForPackage(enumPkg, *declaredVariant.Payload), true
			}
		}
	}
	return "", false
}

func typeRefStringForPackage(currentPkg string, t ast.TypeRef) string {
	if t.FlowInstanceOf != nil {
		return flowInstanceTypeString(typeRefStringForPackage(currentPkg, *t.FlowInstanceOf))
	}
	if t.Function != nil {
		parts := make([]string, 0, len(t.Function.Parameters))
		for _, param := range t.Function.Parameters {
			parts = append(parts, typeRefStringForPackage(currentPkg, param))
		}
		result := "fn(" + strings.Join(parts, ", ") + ") -> " + typeRefStringForPackage(currentPkg, t.Function.ReturnType)
		if t.Function.IsFallible {
			errorType := "Error"
			if t.Function.ErrorType != nil {
				errorType = typeRefStringForPackage(currentPkg, *t.Function.ErrorType)
			}
			result += " ! " + errorType
		}
		return result
	}
	if len(t.TupleOf) > 0 {
		parts := make([]string, 0, len(t.TupleOf))
		for _, elem := range t.TupleOf {
			parts = append(parts, typeRefStringForPackage(currentPkg, elem))
		}
		return "(" + strings.Join(parts, ", ") + ")"
	}
	base := t.Name
	if t.VectorOf != nil {
		base = "Vector<" + typeRefStringForPackage(currentPkg, *t.VectorOf) + ">"
	}
	if t.MatrixOf != nil {
		base = "Matrix<" + typeRefStringForPackage(currentPkg, *t.MatrixOf) + ">"
	}
	if t.Package != "" {
		base = t.Package + "." + base
	} else if currentPkg != "" && base != "" && !isBuiltinTypeName(base) && !strings.Contains(base, "<") {
		base = currentPkg + "." + base
	}
	if base == "" {
		base = "Void"
	}
	if t.HasUnit {
		base = fmt.Sprintf("%s<%s>", base, t.Dimension.String())
	}
	if t.IsArray || t.ArrayDepth > 0 {
		depth := t.ArrayDepth
		if depth == 0 {
			depth = 1
		}
		return base + strings.Repeat("[]", depth)
	}
	return base
}

func isBuiltinTypeName(name string) bool {
	switch name {
	case "Int", "Float", "Complex", "Bool", "String", "Index", "Bytes", "Error", "Void", "Range":
		return true
	default:
		return false
	}
}

func flowInstanceTypeString(resultType string, details ...string) string {
	if len(details) == 2 && (details[0] != "" || details[1] != "") {
		return "FlowInstance<" + resultType + ";" + details[0] + ";" + details[1] + ">"
	}
	return "FlowInstance<" + resultType + ">"
}

func parseGenericType(input, base string) (string, bool) {
	prefix := base + "<"
	if !strings.HasPrefix(input, prefix) || !strings.HasSuffix(input, ">") {
		return "", false
	}
	return strings.TrimSuffix(strings.TrimPrefix(input, prefix), ">"), true
}

func parseVectorElemType(t string) (string, bool) {
	return parseGenericType(t, "Vector")
}

func parseMatrixElemType(t string) (string, bool) {
	return parseGenericType(t, "Matrix")
}

func parseArrayElemType(t string) (string, bool) {
	if !strings.HasSuffix(t, "[]") {
		return "", false
	}
	return strings.TrimSuffix(t, "[]"), true
}

func isFloatScalarTypeString(t string) bool {
	return t == "Float" || (strings.HasPrefix(t, "Float<") && strings.HasSuffix(t, ">"))
}

func isIntScalarTypeString(t string) bool {
	return t == "Int" || (strings.HasPrefix(t, "Int<") && strings.HasSuffix(t, ">"))
}

func isFloatArrayTypeString(t string) bool {
	elem, ok := parseArrayElemType(t)
	return ok && isFloatScalarTypeString(elem)
}

func isIntArrayTypeString(t string) bool {
	elem, ok := parseArrayElemType(t)
	return ok && isIntScalarTypeString(elem)
}

func isNumericTypeString(t string) bool {
	return isIntScalarTypeString(t) || isFloatScalarTypeString(t)
}

func isLinearElementwiseOperatorString(operator string) bool {
	return operator == "+" || operator == "-" || operator == "*" || operator == "/"
}

func isComparisonOperatorString(operator string) bool {
	return operator == "==" || operator == "!=" || operator == "<" || operator == "<=" || operator == ">" || operator == ">="
}

func scalarBinaryResultTypeString(operator string, leftType string, rightType string) string {
	switch operator {
	case "+", "-", "*", "/":
		if isFloatScalarTypeString(leftType) || isFloatScalarTypeString(rightType) {
			if strings.HasPrefix(leftType, "Float<") && strings.HasSuffix(leftType, ">") {
				return leftType
			}
			if strings.HasPrefix(rightType, "Float<") && strings.HasSuffix(rightType, ">") {
				return rightType
			}
			return "Float"
		}
	}
	return leftType
}

func isComplexScalarTypeString(t string) bool {
	return t == "Complex"
}

func isComplexCompatibleScalarTypeString(t string) bool {
	return isNumericTypeString(t) || isComplexScalarTypeString(t)
}

func unifyLinearElemType(leftElem, rightElem string) string {
	if strings.HasPrefix(leftElem, "Float<") || strings.HasPrefix(rightElem, "Float<") {
		if strings.HasPrefix(leftElem, "Float<") {
			return leftElem
		}
		return rightElem
	}
	if leftElem == "Float" || rightElem == "Float" {
		return "Float"
	}
	return leftElem
}

func parseFlowInstanceType(t string) (string, bool) {
	if !strings.HasPrefix(t, "FlowInstance<") || !strings.HasSuffix(t, ">") {
		return "", false
	}
	inner := strings.TrimSuffix(strings.TrimPrefix(t, "FlowInstance<"), ">")
	return strings.SplitN(inner, ";", 2)[0], true
}

func parseFlowInstanceDetails(t string) (result, input, yielded string, ok bool) {
	if !strings.HasPrefix(t, "FlowInstance<") || !strings.HasSuffix(t, ">") {
		return "", "", "", false
	}
	parts := strings.Split(strings.TrimSuffix(strings.TrimPrefix(t, "FlowInstance<"), ">"), ";")
	if len(parts) == 1 {
		return parts[0], "", "", true
	}
	if len(parts) != 3 {
		return "", "", "", false
	}
	return parts[0], parts[1], parts[2], true
}

func parseTupleTypeString(t string) ([]string, bool) {
	if !strings.HasPrefix(t, "(") || !strings.HasSuffix(t, ")") {
		return nil, false
	}
	inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(t, "("), ")"))
	if inner == "" {
		return nil, false
	}
	parts := strings.Split(inner, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		part := strings.TrimSpace(p)
		if part == "" {
			return nil, false
		}
		out = append(out, part)
	}
	if len(out) < 2 {
		return nil, false
	}
	return out, true
}

func compiledBuiltinReturnType(name string, argTypes []string) (string, error) {
	name = canonicalCompiledBuiltinName(name)
	if err := builtin.ValidateArgumentCount(name, len(argTypes)); err != nil {
		return "", err
	}
	switch name {
	case "Idx":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function 'Idx' expects 1 arguments, got %d", len(argTypes))
		}
		if argTypes[0] != "String" {
			return "", fmt.Errorf("compiled mode does not yet support builtin Idx for type %s", argTypes[0])
		}
		return "Index", nil
	case "EinMul", "EinAdd", "EinSub":
		if len(argTypes) != 6 {
			return "", fmt.Errorf("function '%s' expects 6 arguments, got %d", name, len(argTypes))
		}
		leftElem, leftMatrix := parseMatrixElemType(argTypes[0])
		if !leftMatrix {
			return "", fmt.Errorf("function '%s' argument 1 expects Matrix, got %s", name, argTypes[0])
		}
		rightElem, rightMatrix := parseMatrixElemType(argTypes[3])
		if !rightMatrix {
			return "", fmt.Errorf("function '%s' argument 4 expects Matrix, got %s", name, argTypes[3])
		}
		for _, pos := range []int{1, 2, 4, 5} {
			if argTypes[pos] != "Index" {
				return "", fmt.Errorf("function '%s' argument %d expects Index, got %s", name, pos+1, argTypes[pos])
			}
		}
		return "Matrix<" + unifyLinearElemType(leftElem, rightElem) + ">", nil
	case "FFT":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function 'FFT' expects 1 arguments, got %d", len(argTypes))
		}
		if argTypes[0] != "Complex[]" {
			return "", fmt.Errorf("compiled mode does not yet support builtin FFT for type %s", argTypes[0])
		}
		return "Complex[]", nil
	case "Require":
		if len(argTypes) < 1 || len(argTypes) > 2 {
			return "", fmt.Errorf("compile-time function 'Require' expects 1 or 2 arguments, got %d", len(argTypes))
		}
		if argTypes[0] != "Bool" {
			return "", fmt.Errorf("function 'Require' argument 1 expects Bool, got %s", argTypes[0])
		}
		if len(argTypes) == 2 && argTypes[1] != "String" {
			return "", fmt.Errorf("function 'Require' argument 2 expects String, got %s", argTypes[1])
		}
		return "Void", nil
	case "FormatFloat":
		if len(argTypes) != 2 {
			return "", fmt.Errorf("function 'FormatFloat' expects 2 arguments, got %d", len(argTypes))
		}
		if !isFloatScalarTypeString(argTypes[0]) {
			return "", fmt.Errorf("compiled mode does not yet support builtin FormatFloat for type %s", argTypes[0])
		}
		if !isIntScalarTypeString(argTypes[1]) {
			return "", fmt.Errorf("compiled mode does not yet support builtin FormatFloat for type %s", argTypes[1])
		}
		return "String", nil
	case "Pi", "E":
		if len(argTypes) != 0 {
			return "", fmt.Errorf("function '%s' expects 0 arguments, got %d", name, len(argTypes))
		}
		return "Float", nil
	case "Abs":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if isIntScalarTypeString(argTypes[0]) || isFloatScalarTypeString(argTypes[0]) {
			return argTypes[0], nil
		}
		if isComplexScalarTypeString(argTypes[0]) {
			return "Float", nil
		}
		return "", fmt.Errorf("compiled mode does not yet support builtin Abs for type %s", argTypes[0])
	case "I":
		if len(argTypes) != 0 {
			return "", fmt.Errorf("function '%s' expects 0 arguments, got %d", name, len(argTypes))
		}
		return "Complex", nil
	case "Real", "Imag", "Arg":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if isComplexScalarTypeString(argTypes[0]) {
			return "Float", nil
		}
		return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[0])
	case "Conj":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if isComplexScalarTypeString(argTypes[0]) {
			return "Complex", nil
		}
		return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[0])
	case "Sqrt", "Sin", "Cos", "Tan", "Asin", "Acos", "Atan", "Exp", "Ln", "Log10", "Sinh", "Cosh", "Tanh", "FloorToInt", "CeilToInt", "RoundToInt", "Math.FloorToInt", "Math.CeilToInt", "BaseValue", "BaseUnit":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if name == "FloorToInt" || name == "CeilToInt" || name == "RoundToInt" || name == "Math.FloorToInt" || name == "Math.CeilToInt" || name == "BaseValue" || name == "BaseUnit" {
			if isFloatScalarTypeString(argTypes[0]) {
				if name == "FloorToInt" || name == "CeilToInt" || name == "RoundToInt" || name == "Math.FloorToInt" || name == "Math.CeilToInt" {
					return "Int", nil
				}
				return "Float", nil
			}
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[0])
		}
		if name == "Exp" || name == "Ln" {
			if isComplexScalarTypeString(argTypes[0]) {
				return "Complex", nil
			}
		}
		if isIntScalarTypeString(argTypes[0]) || isFloatScalarTypeString(argTypes[0]) {
			return "Float", nil
		}
		return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[0])
	case "Atan2":
		if len(argTypes) != 2 {
			return "", fmt.Errorf("function '%s' expects 2 arguments, got %d", name, len(argTypes))
		}
		for idx := range argTypes {
			if !(isIntScalarTypeString(argTypes[idx]) || isFloatScalarTypeString(argTypes[idx])) {
				return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[idx])
			}
		}
		return "Float", nil
	case "Pow":
		if len(argTypes) != 2 {
			return "", fmt.Errorf("function '%s' expects 2 arguments, got %d", name, len(argTypes))
		}
		for idx := range argTypes {
			if !(isIntScalarTypeString(argTypes[idx]) || isFloatScalarTypeString(argTypes[idx])) {
				return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[idx])
			}
		}
		return "Float", nil
	case "ComplexPolar":
		if len(argTypes) != 2 {
			return "", fmt.Errorf("function 'ComplexPolar' expects 2 arguments, got %d", len(argTypes))
		}
		for idx := range argTypes {
			if !(isIntScalarTypeString(argTypes[idx]) || isFloatScalarTypeString(argTypes[idx])) {
				return "", fmt.Errorf("compiled mode does not yet support builtin ComplexPolar for type %s", argTypes[idx])
			}
		}
		return "Complex", nil
	case "Complex":
		if len(argTypes) != 2 {
			return "", fmt.Errorf("function 'Complex' expects 2 arguments, got %d", len(argTypes))
		}
		for idx := range argTypes {
			if !(isIntScalarTypeString(argTypes[idx]) || isFloatScalarTypeString(argTypes[idx])) {
				return "", fmt.Errorf("compiled mode does not yet support builtin Complex for type %s", argTypes[idx])
			}
		}
		return "Complex", nil
	case "Trace":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		elemType, ok := parseMatrixElemType(argTypes[0])
		if !ok {
			return "", fmt.Errorf("compiled mode does not yet support builtin Trace for type %s", argTypes[0])
		}
		return elemType, nil
	case "Grad":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if elemType, ok := parseVectorElemType(argTypes[0]); ok {
			return "Matrix<" + elemType + ">", nil
		}
		if isIntScalarTypeString(argTypes[0]) || isFloatScalarTypeString(argTypes[0]) {
			return "Vector<" + argTypes[0] + ">", nil
		}
		return "", fmt.Errorf("compiled mode does not yet support builtin Grad for type %s", argTypes[0])
	case "Div":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if elemType, ok := parseMatrixElemType(argTypes[0]); ok {
			return "Vector<" + elemType + ">", nil
		}
		if elemType, ok := parseVectorElemType(argTypes[0]); ok {
			return elemType, nil
		}
		return "", fmt.Errorf("compiled mode does not yet support builtin Div for type %s", argTypes[0])
	case "SymGrad":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if elemType, ok := parseVectorElemType(argTypes[0]); ok {
			return "Matrix<" + elemType + ">", nil
		}
		return "", fmt.Errorf("compiled mode does not yet support builtin SymGrad for type %s", argTypes[0])
	case "JsonNormalize", "JsonParse", "JsonStringify", "JsonLoad":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[0])
		}
		return "String", nil
	case "JsonSave":
		if len(argTypes) != 2 {
			return "", fmt.Errorf("function '%s' expects 2 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String" || argTypes[1] != "String" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for argument types (%s, %s)", name, argTypes[0], argTypes[1])
		}
		return "Int", nil
	case "CsvRead", "CsvReadRows":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[0])
		}
		return "String[][]", nil
	case "CsvReadTable":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[0])
		}
		return "Csv.Table", nil
	case "CsvReadMatrix":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[0])
		}
		return "Float[][]", nil
	case "CsvWrite", "CsvWriteRows":
		if len(argTypes) != 2 {
			return "", fmt.Errorf("function '%s' expects 2 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String" || argTypes[1] != "String[][]" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for argument types (%s, %s)", name, argTypes[0], argTypes[1])
		}
		return "Int", nil
	case "FileReadText":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[0])
		}
		return "String", nil

	case "FileReadBytes":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[0])
		}
		return "Bytes", nil

	case "FileReadLines", "DirectoryList":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[0])
		}
		return "String[]", nil

	case "FileWriteText", "FileWriteLines", "FileWriteBytes":
		if len(argTypes) != 2 {
			return "", fmt.Errorf("function '%s' expects 2 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String" || (name == "FileWriteText" && argTypes[1] != "String") || (name == "FileWriteLines" && argTypes[1] != "String[]") || (name == "FileWriteBytes" && argTypes[1] != "Bytes") {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for argument types (%s, %s)", name, argTypes[0], argTypes[1])
		}
		return "Int", nil
	case "FileExists":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[0])
		}
		return "Bool", nil
	case "FileDelete", "DirectoryMake", "DirectoryMakeAll", "DirectoryRemoveAll":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[0])
		}
		return "Int", nil
	case "PathJoin":
		if len(argTypes) != 1 || argTypes[0] != "String[]" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %v", name, argTypes)
		}
		return "String", nil
	case "PathBaseName", "PathExtension", "PathStem", "PathParent", "PathClean":
		if len(argTypes) != 1 || argTypes[0] != "String" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %v", name, argTypes)
		}
		return "String", nil
	case "StringByteLength":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[0])
		}
		return "Int", nil
	case "StringRuneCount":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[0])
		}
		return "Int", nil
	case "StringConcat":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String[]" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[0])
		}
		return "String", nil
	case "StringFrom":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		switch argTypes[0] {
		case "Int", "Float", "Bool", "String":
			return "String", nil
		default:
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[0])
		}
	case "StringJoin":
		if len(argTypes) != 2 {
			return "", fmt.Errorf("function '%s' expects 2 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String[]" || argTypes[1] != "String" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for types (%s, %s)", name, argTypes[0], argTypes[1])
		}
		return "String", nil
	case "StringReplaceAll":
		if len(argTypes) != 3 {
			return "", fmt.Errorf("function '%s' expects 3 arguments, got %d", name, len(argTypes))
		}
		for _, t := range argTypes {
			if t != "String" {
				return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, t)
			}
		}
		return "String", nil
	case "StringContains", "StringStartsWith", "StringEndsWith":
		if len(argTypes) != 2 {
			return "", fmt.Errorf("function '%s' expects 2 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String" || (name == "FileWriteText" && argTypes[1] != "String") || (name == "FileWriteLines" && argTypes[1] != "String[]") {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for types (%s, %s)", name, argTypes[0], argTypes[1])
		}
		return "Bool", nil
	case "StringTrim", "StringEscapeJSON", "StringQuoteJSON":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[0])
		}
		return "String", nil
	case "StringSplitLines":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[0])
		}
		return "String[]", nil
	case "MarkdownEscapeText", "MarkdownEscapeTableCell":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[0])
		}
		return "String", nil
	case "MarkdownH1", "MarkdownH2", "MarkdownH3", "MarkdownParagraph":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[0])
		}
		return "String[]", nil
	case "MarkdownBlank", "MarkdownHorizontalRule":
		if len(argTypes) != 0 {
			return "", fmt.Errorf("function '%s' expects 0 arguments, got %d", name, len(argTypes))
		}
		return "String[]", nil
	case "MarkdownBullets", "MarkdownNumbered":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String[]" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[0])
		}
		return "String[]", nil
	case "MarkdownCodeBlock", "MarkdownCallout":
		if len(argTypes) != 2 {
			return "", fmt.Errorf("function '%s' expects 2 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String" || argTypes[1] != "String[]" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for types (%s, %s)", name, argTypes[0], argTypes[1])
		}
		return "String[]", nil
	case "MarkdownImage", "MarkdownFigure":
		if len(argTypes) != 2 {
			return "", fmt.Errorf("function '%s' expects 2 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String" || argTypes[1] != "String" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for types (%s, %s)", name, argTypes[0], argTypes[1])
		}
		return "String[]", nil
	case "MarkdownKeyValueTable":
		if len(argTypes) != 2 {
			return "", fmt.Errorf("function '%s' expects 2 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String[]" || argTypes[1] != "String[]" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for types (%s, %s)", name, argTypes[0], argTypes[1])
		}
		return "String[]", nil
	case "MarkdownReport":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String[][]" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[0])
		}
		return "String[]", nil
	case "MarkdownSection", "MarkdownSubsection":
		if len(argTypes) != 2 {
			return "", fmt.Errorf("function '%s' expects 2 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] != "String" || argTypes[1] != "String[][]" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for types (%s, %s)", name, argTypes[0], argTypes[1])
		}
		return "String[]", nil
	case "MarkdownTable":
		if len(argTypes) != 1 {
			return "", fmt.Errorf("function '%s' expects 1 arguments, got %d", name, len(argTypes))
		}
		if argTypes[0] == "String[][]" || argTypes[0] == "String[]" || argTypes[0] == "String" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for type %s", name, argTypes[0])
		}
		return "String[]", nil
	case "MarkdownTableWithColumns":
		if len(argTypes) != 2 {
			return "", fmt.Errorf("function '%s' expects 2 arguments, got %d", name, len(argTypes))
		}
		if argTypes[1] != "String[]" || argTypes[0] == "String[][]" || argTypes[0] == "String[]" || argTypes[0] == "String" {
			return "", fmt.Errorf("compiled mode does not yet support builtin %s for types (%s, %s)", name, argTypes[0], argTypes[1])
		}
		return "String[]", nil
	default:
		return "", fmt.Errorf("compiled mode does not yet support builtin %s", name)
	}
}

// compiledExpressionContext carries the typed ordinary-lowering authority that
// a FLOW activation needs without turning FLOW into a second expression
// language. All ordinary value computation routes through this seam; FLOW-only
// policy state remains outside it.
