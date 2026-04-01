package interpret

import "fmt"

func evalEinsteinBinaryMatrices(op string, left Value, leftLabels []string, right Value, rightLabels []string) (Value, error) {
	if len(leftLabels) != 2 || len(rightLabels) != 2 {
		return Value{}, fmt.Errorf("runtime invariant violation: Einstein matrix helpers require rank-2 labels")
	}
	for _, label := range append(append([]string{}, leftLabels...), rightLabels...) {
		if label == "" {
			return Value{}, fmt.Errorf("runtime error: Einstein indices must be non-empty")
		}
	}

	dimByLabel := map[string]int{}
	for slot, label := range leftLabels {
		dim := left.Matrix.Rows
		if slot == 1 {
			dim = left.Matrix.Cols
		}
		if prev, ok := dimByLabel[label]; ok && prev != dim {
			return Value{}, fmt.Errorf("runtime error: index '%s' has inconsistent extents", label)
		}
		dimByLabel[label] = dim
	}
	for slot, label := range rightLabels {
		dim := right.Matrix.Rows
		if slot == 1 {
			dim = right.Matrix.Cols
		}
		if prev, ok := dimByLabel[label]; ok && prev != dim {
			return Value{}, fmt.Errorf("runtime error: index '%s' has inconsistent extents", label)
		}
		dimByLabel[label] = dim
	}

	ordered := []string{leftLabels[0], leftLabels[1], rightLabels[0], rightLabels[1]}
	counts := map[string]int{}
	for _, label := range ordered {
		counts[label]++
	}
	free := make([]string, 0, len(counts))
	contracted := make([]string, 0, len(counts))
	for _, label := range ordered {
		if counts[label] == 1 {
			if len(free) == 0 || free[len(free)-1] != label {
				free = append(free, label)
			}
		} else if counts[label] == 2 {
			if len(contracted) == 0 || contracted[len(contracted)-1] != label {
				contracted = append(contracted, label)
			}
		} else {
			return Value{}, fmt.Errorf("runtime error: index '%s' appears %d times; only 1 (free) or 2 (contracted) are allowed in M0", label, counts[label])
		}
	}

	switch op {
	case "EinMul":
		if len(free) != 2 {
			return Value{}, fmt.Errorf("runtime error: EinMul requires exactly 2 free indices in M0, got %d", len(free))
		}
		rows := dimByLabel[free[0]]
		cols := dimByLabel[free[1]]
		result := make([]Value, 0, rows*cols)
		assignments := map[string]int{}
		for r := 0; r < rows; r++ {
			for c := 0; c < cols; c++ {
				assignments[free[0]] = r
				assignments[free[1]] = c
				accSet := false
				var acc Value
				var loop func(int) error
				loop = func(pos int) error {
					if pos == len(contracted) {
						leftVal := left.Matrix.Elements[assignments[leftLabels[0]]*left.Matrix.Cols+assignments[leftLabels[1]]]
						rightVal := right.Matrix.Elements[assignments[rightLabels[0]]*right.Matrix.Cols+assignments[rightLabels[1]]]
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
				result = append(result, acc)
			}
		}
		return Value{Kind: ValueMatrix, Matrix: MatrixValue{Rows: rows, Cols: cols, Elements: result}}, nil
	case "EinAdd":
		if leftLabels[0] == leftLabels[1] || rightLabels[0] == rightLabels[1] {
			return Value{}, fmt.Errorf("runtime error: EinAdd requires distinct free indices per matrix term")
		}
		if leftLabels[0] != rightLabels[0] || leftLabels[1] != rightLabels[1] {
			return Value{}, fmt.Errorf("runtime error: EinAdd requires matching free-index order on both terms")
		}
		if left.Matrix.Rows != right.Matrix.Rows || left.Matrix.Cols != right.Matrix.Cols {
			return Value{}, fmt.Errorf("runtime error: EinAdd requires matching matrix shapes")
		}
		result := make([]Value, len(left.Matrix.Elements))
		for idx := range left.Matrix.Elements {
			sum, err := evalBinaryExpr("+", left.Matrix.Elements[idx], right.Matrix.Elements[idx])
			if err != nil {
				return Value{}, err
			}
			result[idx] = sum
		}
		return Value{Kind: ValueMatrix, Matrix: MatrixValue{Rows: left.Matrix.Rows, Cols: left.Matrix.Cols, Elements: result}}, nil
	default:
		return Value{}, fmt.Errorf("runtime invariant violation: unsupported Einstein op %s", op)
	}
}
