package interpret

import (
	"fmt"
	"strings"
)

func evalEinsteinBinaryMatrices(op string, left Value, leftLabels []string, right Value, rightLabels []string) (Value, []string, error) {
	return evalEinsteinBinaryTerms(op, einsteinIndexedTerm{value: left, rank: 2, labels: leftLabels}, einsteinIndexedTerm{value: right, rank: 2, labels: rightLabels})
}

func evalEinsteinBinaryTerms(op string, left einsteinIndexedTerm, right einsteinIndexedTerm) (Value, []string, error) {
	if len(left.labels) != left.rank || len(right.labels) != right.rank {
		return Value{}, nil, fmt.Errorf("runtime invariant violation: Einstein term rank/label metadata mismatch")
	}
	if !isEinsteinTermValue(left.value, left.rank) || !isEinsteinTermValue(right.value, right.rank) {
		return Value{}, nil, fmt.Errorf("runtime invariant violation: unsupported Einstein operand ranks/kinds")
	}
	ordered := append(append([]string{}, left.labels...), right.labels...)
	for _, label := range ordered {
		if label == "" {
			return Value{}, nil, fmt.Errorf("runtime error: Einstein indices must be non-empty")
		}
	}
	dimByLabel := map[string]int{}
	if err := collectEinsteinExtents(dimByLabel, left.value, left.labels); err != nil {
		return Value{}, nil, err
	}
	if err := collectEinsteinExtents(dimByLabel, right.value, right.labels); err != nil {
		return Value{}, nil, err
	}
	counts := map[string]int{}
	for _, label := range ordered {
		counts[label]++
	}
	free := make([]string, 0, len(ordered))
	contracted := make([]string, 0, len(ordered))
	for _, label := range ordered {
		switch counts[label] {
		case 1:
			if !containsRuntimeString(free, label) {
				free = append(free, label)
			}
		case 2:
			if !containsRuntimeString(contracted, label) {
				contracted = append(contracted, label)
			}
		default:
			return Value{}, nil, fmt.Errorf("runtime error: index '%s' appears %d times in indexed multiplication %s*%s; only 1 (free) or 2 (contracted) are allowed in M36", label, counts[label], formatRuntimeEinsteinLabels(left.labels), formatRuntimeEinsteinLabels(right.labels))
		}
	}

	switch op {
	case "EinMul":
		if len(free) > 2 {
			return Value{}, nil, fmt.Errorf("runtime error: indexed multiplication result rank %d is not supported in M36; rank-N tensors are deferred", len(free))
		}
		value, err := evalEinsteinMulResult(left, right, free, contracted, dimByLabel)
		return value, free, err
	case "EinAdd", "EinSub":
		operator := "+"
		if op == "EinSub" {
			operator = "-"
		}
		if left.rank != right.rank {
			return Value{}, nil, fmt.Errorf("runtime error: %s requires matching indexed term ranks (left rank=%d, right rank=%d)", op, left.rank, right.rank)
		}
		for pos := range left.labels {
			if left.labels[pos] != right.labels[pos] {
				return Value{}, nil, fmt.Errorf("runtime error: %s requires matching free-index order on both terms (left=%s, right=%s)", op, formatRuntimeEinsteinLabels(left.labels), formatRuntimeEinsteinLabels(right.labels))
			}
		}
		value, err := evalEinsteinAddSubResult(operator, op, left, right)
		return value, append([]string{}, left.labels...), err
	default:
		return Value{}, nil, fmt.Errorf("runtime invariant violation: unsupported Einstein op %s", op)
	}
}

func evalEinsteinIndexedBinaryExpr(operator string, left evalResult, right evalResult) (Value, []string, error) {
	if left.einTerm == nil || right.einTerm == nil {
		return Value{}, nil, fmt.Errorf("runtime error: indexed tensor expressions must appear on both sides of '%s' (left indexed=%t, right indexed=%t)", operator, left.einTerm != nil, right.einTerm != nil)
	}
	switch operator {
	case "*":
		return evalEinsteinBinaryTerms("EinMul", *left.einTerm, *right.einTerm)
	case "+":
		return evalEinsteinBinaryTerms("EinAdd", *left.einTerm, *right.einTerm)
	case "-":
		return evalEinsteinBinaryTerms("EinSub", *left.einTerm, *right.einTerm)
	default:
		return Value{}, nil, fmt.Errorf("runtime error: indexed tensor expressions only support '+', '-', and '*' in M36")
	}
}

func isEinsteinTermValue(value Value, rank int) bool {
	return (rank == 1 && value.Kind == ValueVector) || (rank == 2 && value.Kind == ValueMatrix)
}

func collectEinsteinExtents(dimByLabel map[string]int, value Value, labels []string) error {
	for slot, label := range labels {
		dim, err := einsteinSlotExtent(value, slot)
		if err != nil {
			return err
		}
		if prev, ok := dimByLabel[label]; ok && prev != dim {
			return fmt.Errorf("runtime error: index '%s' has inconsistent extents", label)
		}
		dimByLabel[label] = dim
	}
	return nil
}

func einsteinSlotExtent(value Value, slot int) (int, error) {
	switch value.Kind {
	case ValueVector:
		if slot != 0 {
			return 0, fmt.Errorf("runtime invariant violation: vector Einstein term has invalid slot %d", slot)
		}
		return len(value.Vector), nil
	case ValueMatrix:
		if slot == 0 {
			return value.Matrix.Rows, nil
		}
		if slot == 1 {
			return value.Matrix.Cols, nil
		}
		return 0, fmt.Errorf("runtime invariant violation: matrix Einstein term has invalid slot %d", slot)
	default:
		return 0, fmt.Errorf("runtime invariant violation: unsupported Einstein term kind %s", value.Kind)
	}
}

func evalEinsteinMulResult(left einsteinIndexedTerm, right einsteinIndexedTerm, free []string, contracted []string, dimByLabel map[string]int) (Value, error) {
	assignments := map[string]int{}
	evalAtAssignment := func() (Value, error) {
		accSet := false
		var acc Value
		var loop func(int) error
		loop = func(pos int) error {
			if pos == len(contracted) {
				leftVal, err := einsteinValueAt(left.value, left.labels, assignments)
				if err != nil {
					return err
				}
				rightVal, err := einsteinValueAt(right.value, right.labels, assignments)
				if err != nil {
					return err
				}
				product, err := evalBinaryExpr("*", leftVal, rightVal)
				if err != nil {
					return err
				}
				if !accSet {
					acc = product
					accSet = true
					return nil
				}
				next, err := evalBinaryExpr("+", acc, product)
				if err != nil {
					return err
				}
				acc = next
				return nil
			}
			label := contracted[pos]
			for v := 0; v < dimByLabel[label]; v++ {
				assignments[label] = v
				if err := loop(pos + 1); err != nil {
					return err
				}
			}
			return nil
		}
		if err := loop(0); err != nil {
			return Value{}, err
		}
		if !accSet {
			return Value{}, fmt.Errorf("runtime invariant violation: EinMul failed to accumulate contracted terms")
		}
		return acc, nil
	}

	switch len(free) {
	case 0:
		return evalAtAssignment()
	case 1:
		length := dimByLabel[free[0]]
		result := make([]Value, 0, length)
		for idx := 0; idx < length; idx++ {
			assignments[free[0]] = idx
			cell, err := evalAtAssignment()
			if err != nil {
				return Value{}, err
			}
			result = append(result, cell)
		}
		return Value{Kind: ValueVector, Vector: result}, nil
	case 2:
		rows := dimByLabel[free[0]]
		cols := dimByLabel[free[1]]
		result := make([]Value, 0, rows*cols)
		for r := 0; r < rows; r++ {
			for c := 0; c < cols; c++ {
				assignments[free[0]] = r
				assignments[free[1]] = c
				cell, err := evalAtAssignment()
				if err != nil {
					return Value{}, err
				}
				result = append(result, cell)
			}
		}
		return Value{Kind: ValueMatrix, Matrix: MatrixValue{Rows: rows, Cols: cols, Elements: result}}, nil
	default:
		return Value{}, fmt.Errorf("runtime error: indexed multiplication result rank %d is not supported in M36", len(free))
	}
}

func evalEinsteinAddSubResult(operator string, opName string, left einsteinIndexedTerm, right einsteinIndexedTerm) (Value, error) {
	switch left.rank {
	case 1:
		if len(left.value.Vector) != len(right.value.Vector) {
			return Value{}, fmt.Errorf("runtime error: %s requires matching vector lengths", opName)
		}
		result := make([]Value, len(left.value.Vector))
		for idx := range left.value.Vector {
			cell, err := evalBinaryExpr(operator, left.value.Vector[idx], right.value.Vector[idx])
			if err != nil {
				return Value{}, err
			}
			result[idx] = cell
		}
		return Value{Kind: ValueVector, Vector: result}, nil
	case 2:
		if left.value.Matrix.Rows != right.value.Matrix.Rows || left.value.Matrix.Cols != right.value.Matrix.Cols {
			return Value{}, fmt.Errorf("runtime error: %s requires matching matrix shapes", opName)
		}
		result := make([]Value, len(left.value.Matrix.Elements))
		for idx := range left.value.Matrix.Elements {
			cell, err := evalBinaryExpr(operator, left.value.Matrix.Elements[idx], right.value.Matrix.Elements[idx])
			if err != nil {
				return Value{}, err
			}
			result[idx] = cell
		}
		return Value{Kind: ValueMatrix, Matrix: MatrixValue{Rows: left.value.Matrix.Rows, Cols: left.value.Matrix.Cols, Elements: result}}, nil
	default:
		return Value{}, fmt.Errorf("runtime error: %s supports only rank-1 vectors and rank-2 matrices in M36, got rank %d", opName, left.rank)
	}
}

func einsteinValueAt(value Value, labels []string, assignments map[string]int) (Value, error) {
	switch value.Kind {
	case ValueVector:
		idx, ok := assignments[labels[0]]
		if !ok {
			return Value{}, fmt.Errorf("runtime invariant violation: missing assignment for index '%s'", labels[0])
		}
		return value.Vector[idx], nil
	case ValueMatrix:
		r, ok := assignments[labels[0]]
		if !ok {
			return Value{}, fmt.Errorf("runtime invariant violation: missing assignment for index '%s'", labels[0])
		}
		c, ok := assignments[labels[1]]
		if !ok {
			return Value{}, fmt.Errorf("runtime invariant violation: missing assignment for index '%s'", labels[1])
		}
		return value.Matrix.Elements[r*value.Matrix.Cols+c], nil
	default:
		return Value{}, fmt.Errorf("runtime invariant violation: unsupported Einstein term kind %s", value.Kind)
	}
}

func formatRuntimeEinsteinLabels(labels []string) string {
	if len(labels) == 0 {
		return "[]"
	}
	return "[" + strings.Join(labels, ",") + "]"
}

func containsRuntimeString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
