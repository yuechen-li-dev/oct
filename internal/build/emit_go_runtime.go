package build

import (
	"fmt"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/batchplan"
)

const __octArrayCoercionHelpers = `
func __octIntArrayToFloat(values []int) []float64 {
	out := make([]float64, len(values))
	for i, value := range values {
		out[i] = float64(value)
	}
	return out
}
`

const __octComplexHelpers = `
func __octComplexReal(value complex128) float64 { return real(value) }
func __octComplexImag(value complex128) float64 { return imag(value) }
func __octComplexAbs(value complex128) float64 { return math.Hypot(real(value), imag(value)) }
`

const __octSharedOctagonHelpers = `
func __octTypeKey(t reflect.Type) string {
	if t.Name() == "" {
		return t.String()
	}
	if t.PkgPath() == "" {
		return t.Name()
	}
	return t.PkgPath() + "." + t.Name()
}
`

const __octIndexHelpers = `
func __octIdx(label string) string {
	if strings.TrimSpace(label) == "" {
		panic("runtime error: Idx requires non-empty label")
	}
	return label
}
`

const __octLinearAlgebraHelpers = `
type __octNumber interface {
	~int | ~float64
}

func __octMatMulMV[T __octNumber](left [][]T, right []T) []T {
	if len(left) == 0 {
		return []T{}
	}
	if len(left[0]) != len(right) {
		panic(fmt.Sprintf("runtime error: matrix multiplication requires left cols = right rows; got %dx%d and %d", len(left), len(left[0]), len(right)))
	}
	result := make([]T, len(left))
	for r := range left {
		if len(left[r]) != len(right) {
			panic(fmt.Sprintf("runtime error: matrix multiplication requires left cols = right rows; got %dx%d and %d", len(left), len(left[r]), len(right)))
		}
		var acc T
		for c := range right {
			acc += left[r][c] * right[c]
		}
		result[r] = acc
	}
	return result
}


func __octMatMulVM[T __octNumber](left []T, right [][]T) []T {
	if len(right) == 0 {
		if len(left) != 0 {
			panic(fmt.Sprintf("runtime error: matrix multiplication requires left cols = right rows; got %d and %dx%d", len(left), 0, 0))
		}
		return []T{}
	}
	rightCols := len(right[0])
	if len(left) != len(right) {
		panic(fmt.Sprintf("runtime error: matrix multiplication requires left cols = right rows; got %d and %dx%d", len(left), len(right), rightCols))
	}
	for r := range right {
		if len(right[r]) != rightCols {
			panic(fmt.Sprintf("runtime error: matrix multiplication requires left cols = right rows; got %d and %dx%d", len(left), len(right), len(right[r])))
		}
	}
	result := make([]T, rightCols)
	for c := 0; c < rightCols; c++ {
		var acc T
		for r := range left {
			acc += left[r] * right[r][c]
		}
		result[c] = acc
	}
	return result
}

func __octVecDot[T __octNumber](left []T, right []T) T {
	if len(left) != len(right) {
		panic(fmt.Sprintf("runtime error: vector dot product requires matching lengths; got %d and %d", len(left), len(right)))
	}
	if len(left) == 0 {
		panic("runtime error: vector dot product requires non-empty vectors")
	}
	var acc T
	for i := range left {
		acc += left[i] * right[i]
	}
	return acc
}

func __octVecAddVV[T __octNumber](left []T, right []T) []T { out := make([]T, len(left)); for i := range left { out[i] = left[i] + right[i] }; return out }
func __octVecSubVV[T __octNumber](left []T, right []T) []T { out := make([]T, len(left)); for i := range left { out[i] = left[i] - right[i] }; return out }
func __octVecMulVV[T __octNumber](left []T, right []T) []T { out := make([]T, len(left)); for i := range left { out[i] = left[i] * right[i] }; return out }
func __octVecDivVV[T __octNumber](left []T, right []T) []T { out := make([]T, len(left)); for i := range left { out[i] = left[i] / right[i] }; return out }
func __octVecAddVS[T __octNumber](left []T, right T) []T { out := make([]T, len(left)); for i := range left { out[i] = left[i] + right }; return out }
func __octVecSubVS[T __octNumber](left []T, right T) []T { out := make([]T, len(left)); for i := range left { out[i] = left[i] - right }; return out }
func __octVecMulVS[T __octNumber](left []T, right T) []T { out := make([]T, len(left)); for i := range left { out[i] = left[i] * right }; return out }
func __octVecDivVS[T __octNumber](left []T, right T) []T { out := make([]T, len(left)); for i := range left { out[i] = left[i] / right }; return out }
func __octVecAddSV[T __octNumber](left T, right []T) []T { out := make([]T, len(right)); for i := range right { out[i] = left + right[i] }; return out }
func __octVecSubSV[T __octNumber](left T, right []T) []T { out := make([]T, len(right)); for i := range right { out[i] = left - right[i] }; return out }
func __octVecMulSV[T __octNumber](left T, right []T) []T { out := make([]T, len(right)); for i := range right { out[i] = left * right[i] }; return out }
func __octVecDivSV[T __octNumber](left T, right []T) []T { out := make([]T, len(right)); for i := range right { out[i] = left / right[i] }; return out }

func __octMatMulMM[T __octNumber](left [][]T, right [][]T) [][]T {
	if len(left) == 0 || len(right) == 0 {
		return [][]T{}
	}
	leftCols := len(left[0])
	rightRows := len(right)
	if leftCols != rightRows {
		panic(fmt.Sprintf("runtime error: matrix multiplication requires left cols = right rows; got %dx%d and %dx%d", len(left), leftCols, len(right), len(right[0])))
	}
	rightCols := len(right[0])
	for r := range left {
		if len(left[r]) != leftCols {
			panic(fmt.Sprintf("runtime error: matrix multiplication requires left cols = right rows; got %dx%d and %dx%d", len(left), len(left[r]), len(right), rightCols))
		}
	}
	for r := range right {
		if len(right[r]) != rightCols {
			panic(fmt.Sprintf("runtime error: matrix multiplication requires left cols = right rows; got %dx%d and %dx%d", len(left), leftCols, len(right), len(right[r])))
		}
	}
	result := make([][]T, len(left))
	for r := range left {
		row := make([]T, rightCols)
		for c := 0; c < rightCols; c++ {
			var acc T
			for k := 0; k < leftCols; k++ {
				acc += left[r][k] * right[k][c]
			}
			row[c] = acc
		}
		result[r] = row
	}
	return result
}

func __octArrayBinaryValue[L __octNumber, R __octNumber, O any](left L, right R, op string) O {
	lf, rf := float64(left), float64(right)
	var zero O
	switch any(zero).(type) {
	case int:
		switch op {
		case "+":
			return any(int(left) + int(right)).(O)
		case "-":
			return any(int(left) - int(right)).(O)
		case "*":
			return any(int(left) * int(right)).(O)
		case "/":
			return any(int(left) / int(right)).(O)
		}
	}
	switch op {
	case "+":
		return any(lf + rf).(O)
	case "-":
		return any(lf - rf).(O)
	case "*":
		return any(lf * rf).(O)
	case "/":
		return any(lf / rf).(O)
	case "==":
		return any(lf == rf).(O)
	case "!=":
		return any(lf != rf).(O)
	case "<":
		return any(lf < rf).(O)
	case "<=":
		return any(lf <= rf).(O)
	case ">":
		return any(lf > rf).(O)
	case ">=":
		return any(lf >= rf).(O)
	default:
		panic(fmt.Sprintf("runtime invariant violation: unsupported array scalar operator %q", op))
	}
}

func __octArrayBinaryAS[L __octNumber, R __octNumber, O any](left []L, right R, op string) []O {
	out := make([]O, len(left))
	for i := range left { out[i] = __octArrayBinaryValue[L, R, O](left[i], right, op) }
	return out
}

func __octArrayBinarySA[L __octNumber, R __octNumber, O any](left L, right []R, op string) []O {
	out := make([]O, len(right))
	for i := range right { out[i] = __octArrayBinaryValue[L, R, O](left, right[i], op) }
	return out
}

func __octMatBinaryValue[L __octNumber, R __octNumber, O __octNumber](left L, right R, op string) O {
	l := O(left)
	r := O(right)
	switch op {
	case "+":
		return l + r
	case "-":
		return l - r
	case "*":
		return l * r
	case "/":
		return l / r
	default:
		panic(fmt.Sprintf("runtime invariant violation: unsupported matrix binary operator %q", op))
	}
}

func __octMatBinaryMM[L __octNumber, R __octNumber, O __octNumber](left [][]L, right [][]R, op string) [][]O {
	leftRows, leftCols := __octMatrixDims(left, "matrix elementwise "+op, "left")
	rightRows, rightCols := __octMatrixDims(right, "matrix elementwise "+op, "right")
	if leftRows != rightRows || leftCols != rightCols {
		panic(fmt.Sprintf("runtime error: matrix shapes must match; got %dx%d and %dx%d", leftRows, leftCols, rightRows, rightCols))
	}
	out := make([][]O, leftRows)
	for rowIndex := range left {
		row := make([]O, leftCols)
		for colIndex := range left[rowIndex] {
			row[colIndex] = __octMatBinaryValue[L, R, O](left[rowIndex][colIndex], right[rowIndex][colIndex], op)
		}
		out[rowIndex] = row
	}
	return out
}

func __octMatBinaryMS[M __octNumber, S __octNumber, O __octNumber](matrix [][]M, scalar S, op string) [][]O {
	rows, cols := __octMatrixDims(matrix, "matrix-scalar "+op, "left")
	out := make([][]O, rows)
	for rowIndex := range matrix {
		row := make([]O, cols)
		for colIndex := range matrix[rowIndex] {
			row[colIndex] = __octMatBinaryValue[M, S, O](matrix[rowIndex][colIndex], scalar, op)
		}
		out[rowIndex] = row
	}
	return out
}

func __octMatBinarySM[S __octNumber, M __octNumber, O __octNumber](scalar S, matrix [][]M, op string) [][]O {
	rows, cols := __octMatrixDims(matrix, "scalar-matrix "+op, "right")
	out := make([][]O, rows)
	for rowIndex := range matrix {
		row := make([]O, cols)
		for colIndex := range matrix[rowIndex] {
			row[colIndex] = __octMatBinaryValue[S, M, O](scalar, matrix[rowIndex][colIndex], op)
		}
		out[rowIndex] = row
	}
	return out
}

func __octMatrixDims[T __octNumber](m [][]T, op string, side string) (int, int) {
	if len(m) == 0 {
		return 0, 0
	}
	cols := len(m[0])
	for r := range m {
		if len(m[r]) != cols {
			panic(fmt.Sprintf("runtime error: %s requires rectangular %s matrix", op, side))
		}
	}
	return len(m), cols
}

func __octEinDimByLabel(label string, dim int, dims map[string]int) {
	if label == "" {
		panic("runtime error: Einstein indices must be non-empty")
	}
	if prev, ok := dims[label]; ok && prev != dim {
		panic(fmt.Sprintf("runtime error: index '%s' has inconsistent extents", label))
	}
	dims[label] = dim
}


func __octVectorDimByLabel(label string, length int, dims map[string]int) {
	__octEinDimByLabel(label, length, dims)
}

func __octEinAddVV[T __octNumber](left []T, l0 string, right []T, r0 string) []T {
	return __octEinAddSubVV(left, l0, right, r0, false)
}

func __octEinSubVV[T __octNumber](left []T, l0 string, right []T, r0 string) []T {
	return __octEinAddSubVV(left, l0, right, r0, true)
}

func __octEinAddSubVV[T __octNumber](left []T, l0 string, right []T, r0 string, subtract bool) []T {
	op := "EinAdd"
	if subtract {
		op = "EinSub"
	}
	if l0 == "" || r0 == "" {
		panic("runtime error: Einstein indices must be non-empty")
	}
	if l0 != r0 {
		panic(fmt.Sprintf("runtime error: %s requires matching free-index order on both vector terms (left=[%s], right=[%s])", op, l0, r0))
	}
	if len(left) != len(right) {
		panic(fmt.Sprintf("runtime error: %s requires matching vector lengths for index '%s'; got %d and %d", op, l0, len(left), len(right)))
	}
	out := make([]T, len(left))
	for i := range left {
		if subtract {
			out[i] = left[i] - right[i]
		} else {
			out[i] = left[i] + right[i]
		}
	}
	return out
}

func __octEinDotVV[T __octNumber](left []T, l0 string, right []T, r0 string) T {
	if l0 == "" || r0 == "" {
		panic("runtime error: Einstein indices must be non-empty")
	}
	if l0 != r0 {
		panic(fmt.Sprintf("runtime error: EinDot requires matching contracted vector indices (left=[%s], right=[%s])", l0, r0))
	}
	if len(left) != len(right) {
		panic(fmt.Sprintf("runtime error: EinDot requires matching vector lengths for index '%s'; got %d and %d", l0, len(left), len(right)))
	}
	var acc T
	for i := range left {
		acc += left[i] * right[i]
	}
	return acc
}

func __octEinOuterVV[T __octNumber](left []T, l0 string, right []T, r0 string) [][]T {
	if l0 == "" || r0 == "" {
		panic("runtime error: Einstein indices must be non-empty")
	}
	if l0 == r0 {
		panic(fmt.Sprintf("runtime error: EinOuter requires distinct free vector indices (left=[%s], right=[%s])", l0, r0))
	}
	out := make([][]T, len(left))
	for i := range left {
		row := make([]T, len(right))
		for j := range right {
			row[j] = left[i] * right[j]
		}
		out[i] = row
	}
	return out
}

func __octEinMulMV[T __octNumber](left [][]T, l0 string, l1 string, right []T, r0 string, free0 string) []T {
	leftRows, leftCols := __octMatrixDims(left, "EinMulMV", "left")
	if l0 == "" || l1 == "" || r0 == "" || free0 == "" {
		panic("runtime error: Einstein indices must be non-empty")
	}
	if l0 == l1 {
		panic(fmt.Sprintf("runtime error: EinMulMV requires distinct matrix indices (left=[%s,%s])", l0, l1))
	}
	dims := map[string]int{}
	__octEinDimByLabel(l0, leftRows, dims)
	__octEinDimByLabel(l1, leftCols, dims)
	__octVectorDimByLabel(r0, len(right), dims)
	if free0 == l0 && r0 == l1 {
		out := make([]T, leftRows)
		for row := 0; row < leftRows; row++ {
			var acc T
			for col := 0; col < leftCols; col++ {
				acc += left[row][col] * right[col]
			}
			out[row] = acc
		}
		return out
	}
	if free0 == l1 && r0 == l0 {
		out := make([]T, leftCols)
		for col := 0; col < leftCols; col++ {
			var acc T
			for row := 0; row < leftRows; row++ {
				acc += left[row][col] * right[row]
			}
			out[col] = acc
		}
		return out
	}
	panic(fmt.Sprintf("runtime error: EinMulMV requires one matrix index to contract with the vector index (left=[%s,%s], right=[%s], free=[%s])", l0, l1, r0, free0))
}

func __octEinMulVM[T __octNumber](left []T, l0 string, right [][]T, r0 string, r1 string, free0 string) []T {
	rightRows, rightCols := __octMatrixDims(right, "EinMulVM", "right")
	if l0 == "" || r0 == "" || r1 == "" || free0 == "" {
		panic("runtime error: Einstein indices must be non-empty")
	}
	if r0 == r1 {
		panic(fmt.Sprintf("runtime error: EinMulVM requires distinct matrix indices (right=[%s,%s])", r0, r1))
	}
	dims := map[string]int{}
	__octVectorDimByLabel(l0, len(left), dims)
	__octEinDimByLabel(r0, rightRows, dims)
	__octEinDimByLabel(r1, rightCols, dims)
	if free0 == r1 && l0 == r0 {
		out := make([]T, rightCols)
		for col := 0; col < rightCols; col++ {
			var acc T
			for row := 0; row < rightRows; row++ {
				acc += left[row] * right[row][col]
			}
			out[col] = acc
		}
		return out
	}
	if free0 == r0 && l0 == r1 {
		out := make([]T, rightRows)
		for row := 0; row < rightRows; row++ {
			var acc T
			for col := 0; col < rightCols; col++ {
				acc += left[col] * right[row][col]
			}
			out[row] = acc
		}
		return out
	}
	panic(fmt.Sprintf("runtime error: EinMulVM requires the vector index to contract with one matrix index (left=[%s], right=[%s,%s], free=[%s])", l0, r0, r1, free0))
}

func __octEinFreeAndContracted(l0 string, l1 string, r0 string, r1 string) ([]string, []string) {
	ordered := []string{l0, l1, r0, r1}
	counts := map[string]int{}
	for _, label := range ordered {
		counts[label]++
	}
	free := []string{}
	contracted := []string{}
	seenFree := map[string]bool{}
	seenContracted := map[string]bool{}
	for _, label := range ordered {
		switch counts[label] {
		case 1:
			if !seenFree[label] {
				free = append(free, label)
				seenFree[label] = true
			}
		case 2:
			if !seenContracted[label] {
				contracted = append(contracted, label)
				seenContracted[label] = true
			}
		default:
			panic(fmt.Sprintf("runtime error: index '%s' appears %d times in [%s,%s]*[%s,%s]; only 1 (free) or 2 (contracted) are allowed in M33", label, counts[label], l0, l1, r0, r1))
		}
	}
	return free, contracted
}

func __octEinMulMM[T __octNumber](left [][]T, l0 string, l1 string, right [][]T, r0 string, r1 string) [][]T {
	leftRows, leftCols := __octMatrixDims(left, "EinMul", "left")
	rightRows, rightCols := __octMatrixDims(right, "EinMul", "right")
	dims := map[string]int{}
	__octEinDimByLabel(l0, leftRows, dims)
	__octEinDimByLabel(l1, leftCols, dims)
	__octEinDimByLabel(r0, rightRows, dims)
	__octEinDimByLabel(r1, rightCols, dims)
	free, contracted := __octEinFreeAndContracted(l0, l1, r0, r1)
	if len(free) != 2 {
		panic(fmt.Sprintf("runtime error: EinMul requires exactly 2 free indices in M33, got %d for [%s,%s]*[%s,%s]", len(free), l0, l1, r0, r1))
	}
	rows := dims[free[0]]
	cols := dims[free[1]]
	out := make([][]T, rows)
	assignments := map[string]int{}
	for r := 0; r < rows; r++ {
		row := make([]T, cols)
		for c := 0; c < cols; c++ {
			assignments[free[0]] = r
			assignments[free[1]] = c
			var acc T
			accSet := false
			var loop func(int)
			loop = func(pos int) {
				if pos == len(contracted) {
					product := left[assignments[l0]][assignments[l1]] * right[assignments[r0]][assignments[r1]]
					if !accSet {
						acc = product
						accSet = true
						return
					}
					acc += product
					return
				}
				label := contracted[pos]
				for v := 0; v < dims[label]; v++ {
					assignments[label] = v
					loop(pos + 1)
				}
			}
			loop(0)
			if !accSet {
				panic("runtime error: EinMul failed to accumulate contracted terms")
			}
			row[c] = acc
		}
		out[r] = row
	}
	return out
}

func __octEinDoubleMM[T __octNumber](left [][]T, l0 string, l1 string, right [][]T, r0 string, r1 string) T {
	leftRows, leftCols := __octMatrixDims(left, "EinDoubleMM", "left")
	rightRows, rightCols := __octMatrixDims(right, "EinDoubleMM", "right")
	if l0 == l1 || r0 == r1 {
		panic(fmt.Sprintf("runtime error: EinDoubleMM requires distinct indices per matrix term (left=[%s,%s], right=[%s,%s]); use Trace(...) for single-matrix trace", l0, l1, r0, r1))
	}
	dims := map[string]int{}
	__octEinDimByLabel(l0, leftRows, dims)
	__octEinDimByLabel(l1, leftCols, dims)
	__octEinDimByLabel(r0, rightRows, dims)
	__octEinDimByLabel(r1, rightCols, dims)
	free, contracted := __octEinFreeAndContracted(l0, l1, r0, r1)
	if len(free) != 0 {
		panic(fmt.Sprintf("runtime error: EinDoubleMM requires zero free indices, got %d for [%s,%s]*[%s,%s]", len(free), l0, l1, r0, r1))
	}
	if len(contracted) != 2 {
		panic(fmt.Sprintf("runtime error: EinDoubleMM requires exactly 2 contracted indices, got %d for [%s,%s]*[%s,%s]", len(contracted), l0, l1, r0, r1))
	}
	assignments := map[string]int{}
	var acc T
	accSet := false
	var loop func(int)
	loop = func(pos int) {
		if pos == len(contracted) {
			product := left[assignments[l0]][assignments[l1]] * right[assignments[r0]][assignments[r1]]
			if !accSet {
				acc = product
				accSet = true
				return
			}
			acc += product
			return
		}
		label := contracted[pos]
		for v := 0; v < dims[label]; v++ {
			assignments[label] = v
			loop(pos + 1)
		}
	}
	loop(0)
	if !accSet {
		panic("runtime error: EinDoubleMM failed to accumulate contracted terms")
	}
	return acc
}

func __octEinAddMM[T __octNumber](left [][]T, l0 string, l1 string, right [][]T, r0 string, r1 string) [][]T {
	return __octEinAddSubMM(left, l0, l1, right, r0, r1, false)
}

func __octEinSubMM[T __octNumber](left [][]T, l0 string, l1 string, right [][]T, r0 string, r1 string) [][]T {
	return __octEinAddSubMM(left, l0, l1, right, r0, r1, true)
}

func __octEinAddSubMM[T __octNumber](left [][]T, l0 string, l1 string, right [][]T, r0 string, r1 string, subtract bool) [][]T {
	op := "EinAdd"
	if subtract {
		op = "EinSub"
	}
	leftRows, leftCols := __octMatrixDims(left, op, "left")
	rightRows, rightCols := __octMatrixDims(right, op, "right")
	if l0 == "" || l1 == "" || r0 == "" || r1 == "" {
		panic("runtime error: Einstein indices must be non-empty")
	}
	if l0 == l1 || r0 == r1 {
		panic(fmt.Sprintf("runtime error: %s requires distinct free indices per matrix term (left=[%s,%s], right=[%s,%s])", op, l0, l1, r0, r1))
	}
	if l0 != r0 || l1 != r1 {
		panic(fmt.Sprintf("runtime error: %s requires matching free-index order on both terms (left=[%s,%s], right=[%s,%s])", op, l0, l1, r0, r1))
	}
	if leftRows != rightRows || leftCols != rightCols {
		panic(fmt.Sprintf("runtime error: %s requires matching matrix shapes", op))
	}
	out := make([][]T, leftRows)
	for r := range left {
		row := make([]T, leftCols)
		for c := range left[r] {
			if subtract {
				row[c] = left[r][c] - right[r][c]
			} else {
				row[c] = left[r][c] + right[r][c]
			}
		}
		out[r] = row
	}
	return out
}

func __octTrace[T __octNumber](m [][]T) T {
	if len(m) == 0 {
		panic("runtime error: Trace requires non-empty matrix")
	}
	if len(m[0]) != len(m) {
		panic(fmt.Sprintf("runtime error: Trace requires square matrix, got %dx%d", len(m), len(m[0])))
	}
	var out T
	for i := 0; i < len(m); i++ {
		if len(m[i]) != len(m) {
			panic(fmt.Sprintf("runtime error: Trace requires square matrix, got %dx%d", len(m), len(m[i])))
		}
		out += m[i][i]
	}
	return out
}

func __octGrad[T __octNumber](v []T) [][]T {
	out := make([][]T, len(v))
	for i := range v {
		row := make([]T, len(v))
		row[i] = v[i]
		out[i] = row
	}
	return out
}

func __octGradScalar[T __octNumber](v T) []T {
	return []T{v}
}

func __octDivVector[T __octNumber](v []T) T {
	var out T
	for _, cell := range v {
		out += cell
	}
	return out
}

func __octDiv[T __octNumber](m [][]T) []T {
	out := make([]T, len(m))
	for r := range m {
		out[r] = __octDivVector(m[r])
	}
	return out
}

func __octSymGrad[T __octNumber](v []T) [][]T {
	return __octGrad(v)
}
`

const __octFFTHelpers = `
func __octFFT(values []complex128) octResult_ComplexSlice {
	n := len(values)
	if n == 0 {
		return octResult_ComplexSlice{Err: "runtime error: FFT requires non-empty input", IsErr: true}
	}
	if n&(n-1) != 0 {
		return octResult_ComplexSlice{Err: "runtime error: FFT requires power-of-two input length", IsErr: true}
	}

	out := make([]complex128, n)
	copy(out, values)

	for i := 0; i < n; i++ {
		j := __octReverseBits(i, n)
		if j > i {
			out[i], out[j] = out[j], out[i]
		}
	}

	for span := 2; span <= n; span *= 2 {
		halfSpan := span / 2
		phaseStep := -2.0 * math.Pi / float64(span)
		for blockStart := 0; blockStart < n; blockStart += span {
			for j := 0; j < halfSpan; j++ {
				angle := phaseStep * float64(j)
				twiddle := complex(math.Cos(angle), math.Sin(angle))
				odd := out[blockStart+j+halfSpan]
				t := twiddle * odd
				even := out[blockStart+j]
				out[blockStart+j] = even + t
				out[blockStart+j+halfSpan] = even - t
			}
		}
	}

	return octResult_ComplexSlice{Value: out}
}

func __octReverseBits(index int, width int) int {
	value := index
	reversed := 0
	n := width
	for n > 1 {
		lowBit := value & 1
		reversed = reversed*2 + lowBit
		value /= 2
		n /= 2
	}
	return reversed
}
`

const __octRandomHelpers = `
func __octRandomRotl(x uint64, k int) uint64 { return (x << k) | (x >> (64 - k)) }
func __octRandomSplitMix64(x uint64) uint64 {
	x += 0x9e3779b97f4a7c15
	z := x
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}
func __octRandomSeedState(seed int) Random_Rng {
	x := uint64(seed)
	return Random_Rng{_State0: int(__octRandomSplitMix64(x)), _State1: int(__octRandomSplitMix64(x + 1)), _State2: int(__octRandomSplitMix64(x + 2)), _State3: int(__octRandomSplitMix64(x + 3))}
}
func __octRandomNext(s Random_Rng) (Random_Rng, uint64) {
	s0 := uint64(s._State0)
	s1 := uint64(s._State1)
	s2 := uint64(s._State2)
	s3 := uint64(s._State3)
	result := __octRandomRotl(s1*5, 7) * 9
	t := s1 << 17
	s2 ^= s0
	s3 ^= s1
	s1 ^= s2
	s0 ^= s3
	s2 ^= t
	s3 = __octRandomRotl(s3, 45)
	return Random_Rng{_State0: int(s0), _State1: int(s1), _State2: int(s2), _State3: int(s3)}, result
}
func __octRandomFloat01(x uint64) float64 { return float64(x>>11) * (1.0 / (1 << 53)) }
func __octRandomRngSeed(seed int) Random_Rng { return __octRandomSeedState(seed) }
func __octRandomRandInt(rng Random_Rng, min int, max int) Random_RandIntResult {
	if min > max { panic("runtime error: min must be <= max") }
	span := uint64(max - min + 1)
	threshold := ^uint64(0) - (^uint64(0) % span)
	var x uint64
	for {
		rng, x = __octRandomNext(rng)
		if x < threshold { break }
	}
	return Random_RandIntResult{Next: rng, Value: min + int(x%span)}
}
func __octRandomRandFloat01(rng Random_Rng) Random_RandFloatResult {
	rng, x := __octRandomNext(rng)
	return Random_RandFloatResult{Next: rng, Value: __octRandomFloat01(x)}
}
func __octRandomRandFloatRange(rng Random_Rng, min float64, max float64) Random_RandFloatResult {
	if min > max { panic("runtime error: min must be <= max") }
	if min == max { return Random_RandFloatResult{Next: rng, Value: min} }
	rng, x := __octRandomNext(rng)
	return Random_RandFloatResult{Next: rng, Value: min + (max-min)*__octRandomFloat01(x)}
}
func __octRandomRandBernoulli(rng Random_Rng, p float64) Random_RandBoolResult {
	if p < 0 || p > 1 { panic("runtime error: p must be in [0,1]") }
	if p == 0 || p == 1 { return Random_RandBoolResult{Next: rng, Value: p == 1} }
	rng, x := __octRandomNext(rng)
	return Random_RandBoolResult{Next: rng, Value: __octRandomFloat01(x) < p}
}
func __octRandomRandNormal(rng Random_Rng, mean float64, stddev float64) Random_RandFloatResult {
	if stddev < 0 { panic("runtime error: stddev must be >= 0") }
	if stddev == 0 { return Random_RandFloatResult{Next: rng, Value: mean} }
	rng, u1 := __octRandomNext(rng)
	rng, u2 := __octRandomNext(rng)
	z := math.Sqrt(-2*math.Log(math.Max(__octRandomFloat01(u1), 1e-12))) * math.Cos(2*math.Pi*__octRandomFloat01(u2))
	return Random_RandFloatResult{Next: rng, Value: mean + stddev*z}
}
func __octCryptoRandBytes(count int) ([]byte, error) {
	if count < 0 { return nil, fmt.Errorf("count must be >= 0") }
	out := make([]byte, count)
	_, err := rand.Read(out)
	return out, err
}
func __octCryptoRandInt(min int, max int) (int, error) {
	if min > max { return 0, fmt.Errorf("runtime error: min must be <= max") }
	span := max - min + 1
	n, err := rand.Int(rand.Reader, big.NewInt(int64(span)))
	if err != nil { return 0, err }
	return min + int(n.Int64()), nil
}
func __octCryptoRandFloat01() (float64, error) {
	var b [8]byte
	_, err := rand.Read(b[:])
	if err != nil { return 0, err }
	return __octRandomFloat01(binary.LittleEndian.Uint64(b[:])), nil
}
`

func builtinImportDeps(name string) []string {
	name = canonicalCompiledBuiltinName(name)
	if isMarkdownCompiledBuiltin(name) {
		if name == "MarkdownTable" || name == "MarkdownTableWithColumns" {
			return []string{"reflect", "strings"}
		}
		return []string{"strings"}
	}
	switch name {
	case "Contains", "StartsWith", "EndsWith", "Trim", "Lower", "Upper", "Join":
		return []string{"strings"}
	case "StringRuneCount":
		return []string{"unicode/utf8"}
	case "StringContains", "StringStartsWith", "StringEndsWith", "StringTrim", "StringJoin", "StringConcat", "StringReplaceAll", "StringSplitLines":
		return []string{"strings"}
	case "StringEscapeJSON", "StringQuoteJSON", "StringFrom":
		return []string{"strconv"}
	case "FormatFloat":
		return []string{"strconv"}
	case "ArrayCrossSection", "Array.CrossSection":
		return []string{"fmt"}
	case "CsvReadMatrix":
		return []string{"strconv"}
	case "PathJoin", "PathBaseName", "PathExtension", "PathStem", "PathParent", "PathClean":
		if name == "PathStem" {
			return []string{"path/filepath", "strings"}
		}
		return []string{"path/filepath"}
	}
	return nil
}

const __octPrometheusHelpers = `
var __octPrometheusVulkanEnvOnce sync.Once
var __octPrometheusVulkanEnvValue string

func __octPrometheusVulkanEnv() string {
	__octPrometheusVulkanEnvOnce.Do(func() {
		__octPrometheusVulkanEnvValue = __octDetectPrometheusVulkanEnv()
	})
	return __octPrometheusVulkanEnvValue
}

func __octDetectPrometheusVulkanEnv() string {
	if os.Getenv("WSL_DISTRO_NAME") == "" {
		return "not_applicable"
	}
	output, err := exec.Command("vulkaninfo", "--summary").CombinedOutput()
	if err != nil {
		return "wsl_vulkan_unknown"
	}
	text := strings.ToLower(string(output))
	if strings.Contains(text, "driver_id_mesa_dozen") || strings.Contains(text, "drivername         = dozen") {
		return "vulkan_wsl_dzn"
	}
	if strings.Contains(text, "llvmpipe") || strings.Contains(text, "physical_device_type_cpu") {
		return "software_vulkan_llvmpipe_or_cpu"
	}
	if strings.Contains(text, "vulkan") {
		return "vulkan_wsl_other"
	}
	return "wsl_vulkan_unknown"
}

func __octPrometheusMatMulMM(left [][]float64, right [][]float64) [][]float64 {
	out, run, err := prometheus.RunCompiledMatMulMM(left, right)
	env := __octPrometheusVulkanEnv()
	if env == "not_applicable" && run.VulkanEnv != "" {
		env = run.VulkanEnv
	}
	fmt.Printf("backend_requested=%s backend_used=%s status=%s correctness=%t detail_code=%d detail_name=%s vulkan_env=%s wall=%dns\n",
		run.RequestedBackend, run.UsedBackend, run.Status.String(), run.Correctness.Pass, run.DetailCode, run.DetailName, env, run.WallTimeNs)
	if err != nil {
		panic(fmt.Sprintf("PrometheusMatMulMM failed: %v", err))
	}
	return out
}
`

const __octStringHelpers = `
func __octStringSplitLines(text string) []string {
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	if normalized == "" {
		return []string{}
	}
	lines := strings.Split(normalized, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func __octStringEscapeJSON(text string) string {
	quoted := strconv.Quote(text)
	return quoted[1 : len(quoted)-1]
}
`

const __octMarkdownHelpers = `
func __octMarkdownNormalizeInline(text string) string {
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	normalized = strings.ReplaceAll(normalized, "\n", " ")
	return strings.TrimSpace(normalized)
}

func __octMarkdownEscapeTableCell(text string) string {
	return strings.ReplaceAll(__octMarkdownNormalizeInline(text), "|", "\\|")
}

func __octMarkdownLongestBacktickRun(text string) int {
	longest := 0
	current := 0
	for _, r := range text {
		if r == 96 {
			current++
			if current > longest { longest = current }
			continue
		}
		current = 0
	}
	return longest
}

func __octMarkdownCodeFence(language string, body []string) string {
	longest := __octMarkdownLongestBacktickRun(strings.TrimSpace(language))
	for _, line := range body {
		if run := __octMarkdownLongestBacktickRun(line); run > longest { longest = run }
	}
	fenceLength := 3
	if longest + 1 > fenceLength { fenceLength = longest + 1 }
	return strings.Repeat(string(rune(96)), fenceLength)
}

func __octMarkdownList(items []string, numbered bool) []string {
	out := make([]string, 0, len(items))
	for idx, item := range items {
		prefix := "- "
		if numbered { prefix = fmt.Sprintf("%d. ", idx+1) }
		out = append(out, prefix + __octMarkdownNormalizeInline(item))
	}
	return out
}

func __octMarkdownCodeBlock(language string, body []string) []string {
	fence := __octMarkdownCodeFence(language, body)
	out := []string{fence + strings.TrimSpace(language)}
	out = append(out, body...)
	out = append(out, fence)
	return out
}

func __octMarkdownCallout(kind string, lines []string) []string {
	labelMap := map[string]string{"note": "Note", "info": "Info", "warning": "Warning", "danger": "Danger", "success": "Success"}
	label, ok := labelMap[kind]
	if !ok { panic(fmt.Sprintf("runtime error: MarkdownCallout unsupported kind '%s' (supported: note, info, warning, danger, success)", kind)) }
	if len(lines) == 0 { return []string{"> **" + label + ":**"} }
	out := make([]string, 0, len(lines))
	for idx, line := range lines {
		normalized := __octMarkdownNormalizeInline(line)
		if idx == 0 {
			if normalized == "" { out = append(out, "> **" + label + ":**") } else { out = append(out, "> **" + label + ":** " + normalized) }
			continue
		}
		if normalized == "" { out = append(out, ">") } else { out = append(out, "> " + normalized) }
	}
	return out
}

func __octMarkdownImage(path string, caption string) []string {
	alt := strings.ReplaceAll(__octMarkdownNormalizeInline(caption), "]", "\\]")
	return []string{"![" + alt + "](" + path + ")"}
}

func __octMarkdownFigure(path string, caption string) []string {
	image := __octMarkdownImage(path, caption)[0]
	return []string{image, "", "*Figure: " + __octMarkdownNormalizeInline(caption) + "*"}
}

func __octMarkdownFlattenBlocks(blocks [][]string) []string {
	out := []string{}
	for _, block := range blocks {
		if len(block) == 0 { continue }
		if len(out) > 0 { out = append(out, "") }
		out = append(out, block...)
	}
	return out
}

func __octMarkdownSection(title string, blocks [][]string, subsection bool) []string {
	prefix := "## "
	if subsection { prefix = "### " }
	out := []string{prefix + __octMarkdownNormalizeInline(title)}
	lines := __octMarkdownFlattenBlocks(blocks)
	if len(lines) == 0 { return out }
	out = append(out, "")
	out = append(out, lines...)
	return out
}

func __octMarkdownKeyValueTable(keys []string, values []string) []string {
	if len(keys) != len(values) { panic("runtime error: MarkdownKeyValueTable keys and values must have equal lengths") }
	out := []string{"| key | value |", "| --- | --- |"}
	for idx := range keys {
		out = append(out, "| " + __octMarkdownEscapeTableCell(keys[idx]) + " | " + __octMarkdownEscapeTableCell(values[idx]) + " |")
	}
	return out
}

func __octMarkdownRecordColumns(table any, requested []string, explicit bool) ([]string, [][]string) {
	v := reflect.ValueOf(table)
	if v.Kind() == reflect.Pointer { v = v.Elem() }
	if v.Kind() != reflect.Struct { panic("runtime error: MarkdownTable expects record-of-string-columns") }
	t := v.Type()
	columns := make([]string, 0, t.NumField())
	fieldByName := map[string]reflect.Value{}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		columns = append(columns, field.Name)
		fieldByName[field.Name] = v.Field(i)
	}
	if len(columns) == 0 { panic("runtime error: MarkdownTable requires at least one column") }
	if explicit {
		seen := map[string]struct{}{}
		columns = []string{}
		for _, name := range requested {
			if strings.TrimSpace(name) == "" { panic("runtime error: MarkdownTableWithColumns column names must be non-empty") }
			if _, ok := fieldByName[name]; !ok { panic(fmt.Sprintf("runtime error: MarkdownTableWithColumns unknown column '%s'", name)) }
			if _, dup := seen[name]; dup { panic(fmt.Sprintf("runtime error: MarkdownTableWithColumns duplicate requested column '%s'", name)) }
			seen[name] = struct{}{}
			columns = append(columns, name)
		}
		if len(columns) == 0 { panic("runtime error: MarkdownTableWithColumns requires at least one requested column") }
	}
	rowCount := -1
	colArrays := make([][]string, 0, len(columns))
	for _, name := range columns {
		field := fieldByName[name]
		if field.Kind() != reflect.Slice || field.Type().Elem().Kind() != reflect.String { panic(fmt.Sprintf("runtime error: Markdown table column '%s' must be String[]", name)) }
		arr := make([]string, field.Len())
		for i := 0; i < field.Len(); i++ { arr[i] = field.Index(i).String() }
		if rowCount == -1 { rowCount = len(arr) } else if rowCount != len(arr) { panic("runtime error: Markdown table columns must have equal lengths") }
		colArrays = append(colArrays, arr)
	}
	return columns, colArrays
}

func __octMarkdownTable(table any) []string {
	columns, colArrays := __octMarkdownRecordColumns(table, nil, false)
	return __octMarkdownRenderTable(columns, colArrays)
}

func __octMarkdownTableWithColumns(table any, requested []string) []string {
	columns, colArrays := __octMarkdownRecordColumns(table, requested, true)
	return __octMarkdownRenderTable(columns, colArrays)
}

func __octMarkdownRenderTable(columns []string, colArrays [][]string) []string {
	header := "|"
	sep := "|"
	for _, name := range columns {
		header += " " + __octMarkdownEscapeTableCell(name) + " |"
		sep += " --- |"
	}
	out := []string{header, sep}
	rowCount := 0
	if len(colArrays) > 0 { rowCount = len(colArrays[0]) }
	for r := 0; r < rowCount; r++ {
		line := "|"
		for c := 0; c < len(columns); c++ { line += " " + __octMarkdownEscapeTableCell(colArrays[c][r]) + " |" }
		out = append(out, line)
	}
	return out
}
`

const __octWriteHelpers = `
func __octWriteOctagon(path string, value any) {
	path = __octAttributedOutputPath(path)
	if !strings.HasSuffix(path, ".octagon") {
		panic("WriteOctagon path must end with .octagon")
	}
	rendered, err := __octSerialize(reflect.ValueOf(value), 0)
	if err != nil {
		panic(fmt.Sprintf("WriteOctagon cannot serialize value: %v", err))
	}
	if err := os.WriteFile(path, []byte(rendered+"\n"), 0o644); err != nil {
		panic(fmt.Sprintf("WriteOctagon write %s: %v", path, err))
	}
}

func __octAttributedOutputPath(path string) string {
	prefix := os.Getenv("OCT_OUTPUT_PATH_PREFIX")
	if prefix == "" {
		return path
	}
	return filepath.Join(filepath.Dir(path), prefix+"."+filepath.Base(path))
}

func __octSerialize(v reflect.Value, depth int) (string, error) {
	if !v.IsValid() {
		return "", fmt.Errorf("invalid value")
	}
	if v.Kind() == reflect.Interface {
		return __octSerialize(v.Elem(), depth)
	}
	if meta, ok := __octEnumMetaByGoType[__octTypeKey(v.Type())]; ok {
		idx := int(v.FieldByName("Tag").Int())
		if idx < 0 || idx >= len(meta.Variants) {
			return "", fmt.Errorf("enum %s variant index %d out of range", meta.ShortName, idx)
		}
		name := meta.ShortName + "." + meta.Variants[idx]
		if meta.PayloadTypes[idx] == nil { return name, nil }
		payload := v.FieldByName("Payload")
		if !payload.IsValid() || payload.IsNil() { return "", fmt.Errorf("enum %s variant %s missing payload", meta.ShortName, meta.Variants[idx]) }
		rendered, err := __octSerialize(payload.Elem(), depth)
		if err != nil { return "", err }
		return name + "(" + rendered + ")", nil
	}
	switch v.Kind() {
	case reflect.Int:
		return strconv.FormatInt(v.Int(), 10), nil
	case reflect.Float64:
		return strconv.FormatFloat(v.Float(), 'g', -1, 64), nil
	case reflect.Bool:
		return strconv.FormatBool(v.Bool()), nil
	case reflect.String:
		return strconv.Quote(v.String()), nil
	case reflect.Slice:
		parts := make([]string, 0, v.Len())
		for i := 0; i < v.Len(); i++ {
			part, err := __octSerialize(v.Index(i), depth)
			if err != nil {
				return "", err
			}
			parts = append(parts, part)
		}
		return "[" + strings.Join(parts, ", ") + "]", nil
	case reflect.Struct:
		meta, ok := __octRecordMetaByGoType[__octTypeKey(v.Type())]
		if !ok {
			return "", fmt.Errorf("record type %q is not representable in .octagon output", v.Type().String())
		}
		fields := make([]string, 0, len(meta.Fields))
		for _, field := range meta.Fields {
			fieldValue := v.FieldByName(field)
			value, err := __octSerialize(fieldValue, depth+1)
			if err != nil {
				return "", err
			}
			if __octNeedsFieldParens(fieldValue) {
				value = "(" + value + ")"
			}
			fields = append(fields, fmt.Sprintf("%s%s: %s", __octIndent(depth+1), field, value))
		}
		return fmt.Sprintf("%s {\n%s\n%s}", meta.ShortName, strings.Join(fields, "\n"), __octIndent(depth)), nil
	default:
		return "", fmt.Errorf("value kind %s is not representable in .octagon output", v.Kind().String())
	}
}

func __octIndent(depth int) string {
	return strings.Repeat("    ", depth)
}

func __octNeedsFieldParens(v reflect.Value) bool {
	for v.Kind() == reflect.Interface {
		if !v.IsValid() || v.IsNil() {
			return false
		}
		v = v.Elem()
	}
	return v.Kind() == reflect.Int || v.Kind() == reflect.Float64
}
`

const __octLoadHelpers = `
type __octParser struct {
	input string
	pos int
}

func __octLoadOctagonTyped(path string, target reflect.Type, expectedType string) (any, error) {
	if !strings.HasSuffix(path, ".octagon") {
		return nil, errors.New("LoadOctagon path must end with .octagon")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("LoadOctagon %s: %v", path, err)
	}
	value, err := (__octParser{input: string(data)}).parse()
	if err != nil {
		return nil, fmt.Errorf("LoadOctagon %s: %v", path, err)
	}
	out, err := __octMaterialize(value, target, expectedType)
	if err != nil {
		return nil, fmt.Errorf("LoadOctagon %s: %v", path, err)
	}
	return out.Interface(), nil
}

func (p __octParser) parse() (__octParsedValue, error) {
	p.skipWS()
	v, err := p.parseValue()
	if err != nil {
		return __octParsedValue{}, err
	}
	p.skipWS()
	if p.pos != len(p.input) {
		return __octParsedValue{}, fmt.Errorf("expected end of file after top-level value")
	}
	return v, nil
}

func (p *__octParser) parseValue() (__octParsedValue, error) {
	p.skipWS()
	if p.pos >= len(p.input) {
		return __octParsedValue{}, fmt.Errorf("expected expression")
	}
	switch p.input[p.pos] {
	case '"':
		s, err := p.parseString()
		if err != nil {
			return __octParsedValue{}, err
		}
		return __octParsedValue{Kind: __octParsedString, Text: s}, nil
	case '[':
		return p.parseArray()
	}
	r, _ := utf8.DecodeRuneInString(p.input[p.pos:])
	if r == '-' || unicode.IsDigit(r) {
		return p.parseNumber()
	}
	id, err := p.parseIdentifier()
	if err != nil {
		return __octParsedValue{}, err
	}
	switch id {
	case "true":
		return __octParsedValue{Kind: __octParsedBool, Bool: true}, nil
	case "false":
		return __octParsedValue{Kind: __octParsedBool, Bool: false}, nil
	}
	p.skipWS()
	if p.pos < len(p.input) && p.input[p.pos] == '{' {
		return p.parseRecord(id)
	}
	if dot := strings.LastIndex(id, "."); dot > 0 && dot < len(id)-1 {
		out := __octParsedValue{Kind: __octParsedEnum, EnumType: id[:dot], EnumVariant: id[dot+1:]}
		p.skipWS()
		if p.pos < len(p.input) && p.input[p.pos] == '(' {
			out.EnumHasPayload = true
			p.pos++
			for {
				p.skipWS()
				if p.pos < len(p.input) && p.input[p.pos] == ')' { p.pos++; break }
				item, err := p.parseValue()
				if err != nil { return __octParsedValue{}, err }
				out.EnumPayload = append(out.EnumPayload, item)
				p.skipWS()
				if p.pos < len(p.input) && p.input[p.pos] == ')' { p.pos++; break }
				if p.pos >= len(p.input) || p.input[p.pos] != ',' { return __octParsedValue{}, fmt.Errorf("expected ',' or ')' after enum payload") }
				p.pos++
			}
		}
		return out, nil
	}
	return __octParsedValue{}, fmt.Errorf("expected expression")
}

func (p *__octParser) parseArray() (__octParsedValue, error) {
	p.pos++
	items := []__octParsedValue{}
	for {
		p.skipWS()
		if p.pos < len(p.input) && p.input[p.pos] == ']' {
			p.pos++
			return __octParsedValue{Kind: __octParsedArray, Array: items}, nil
		}
		v, err := p.parseValue()
		if err != nil {
			return __octParsedValue{}, err
		}
		items = append(items, v)
		p.skipWS()
		if p.pos < len(p.input) && p.input[p.pos] == ',' {
			p.pos++
			continue
		}
		if p.pos < len(p.input) && p.input[p.pos] == ']' {
			p.pos++
			return __octParsedValue{Kind: __octParsedArray, Array: items}, nil
		}
		return __octParsedValue{}, fmt.Errorf("expected ',' or ']'")
	}
}

func (p *__octParser) parseRecord(name string) (__octParsedValue, error) {
	p.pos++
	fields := map[string]__octParsedValue{}
	for {
		p.skipWS()
		if p.pos < len(p.input) && p.input[p.pos] == '}' {
			p.pos++
			return __octParsedValue{Kind: __octParsedRecord, RecordType: name, RecordFields: fields}, nil
		}
		field, err := p.parseIdentifier()
		if err != nil {
			return __octParsedValue{}, err
		}
		p.skipWS()
		if p.pos >= len(p.input) || p.input[p.pos] != ':' {
			return __octParsedValue{}, fmt.Errorf("expected ':'")
		}
		p.pos++
		value, err := p.parseValue()
		if err != nil {
			return __octParsedValue{}, err
		}
		fields[field] = value
	}
}

func (p *__octParser) parseNumber() (__octParsedValue, error) {
	start := p.pos
	if p.input[p.pos] == '-' {
		p.pos++
	}
	dot := false
	exp := false
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if ch >= '0' && ch <= '9' {
			p.pos++
			continue
		}
		if ch == '.' {
			dot = true
			p.pos++
			continue
		}
		if ch == 'e' || ch == 'E' {
			exp = true
			p.pos++
			if p.pos < len(p.input) && (p.input[p.pos] == '+' || p.input[p.pos] == '-') {
				p.pos++
			}
			continue
		}
		break
	}
	num := p.input[start:p.pos]
	if dot || exp {
		v, err := strconv.ParseFloat(num, 64)
		if err != nil {
			return __octParsedValue{}, fmt.Errorf("invalid Float literal %q", num)
		}
		return __octParsedValue{Kind: __octParsedFloat, Float: v}, nil
	}
	v, err := strconv.Atoi(num)
	if err != nil {
		return __octParsedValue{}, fmt.Errorf("invalid Int literal %q", num)
	}
	return __octParsedValue{Kind: __octParsedInt, Int: v}, nil
}

func (p *__octParser) parseIdentifier() (string, error) {
	p.skipWS()
	start := p.pos
	for p.pos < len(p.input) {
		r, width := utf8.DecodeRuneInString(p.input[p.pos:])
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '.' {
			p.pos += width
			continue
		}
		break
	}
	if start == p.pos {
		return "", fmt.Errorf("expected identifier")
	}
	return p.input[start:p.pos], nil
}

func (p *__octParser) parseString() (string, error) {
	start := p.pos
	p.pos++
	escape := false
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if escape {
			escape = false
			p.pos++
			continue
		}
		if ch == '\\' {
			escape = true
			p.pos++
			continue
		}
		if ch == '"' {
			p.pos++
			return strconv.Unquote(p.input[start:p.pos])
		}
		p.pos++
	}
	return "", fmt.Errorf("unterminated string literal")
}

func (p *__octParser) skipWS() {
	for p.pos < len(p.input) {
		r, width := utf8.DecodeRuneInString(p.input[p.pos:])
		if !unicode.IsSpace(r) {
			break
		}
		p.pos += width
	}
}

func __octMaterialize(value __octParsedValue, target reflect.Type, expectedType string) (reflect.Value, error) {
	if meta, ok := __octEnumMetaByGoType[__octTypeKey(target)]; ok {
		if value.Kind != __octParsedEnum {
			return reflect.Value{}, fmt.Errorf("expected %s, got non-enum value", expectedType)
		}
		if value.EnumType != meta.ShortName && value.EnumType != meta.FullName {
			return reflect.Value{}, fmt.Errorf("expected enum %s, got %s", expectedType, value.EnumType)
		}
		for i, v := range meta.Variants {
			if v == value.EnumVariant {
				if meta.PayloadTypes[i] == nil && value.EnumHasPayload { return reflect.Value{}, fmt.Errorf("enum %s variant %s does not accept a payload", expectedType, v) }
				if meta.PayloadTypes[i] != nil && len(value.EnumPayload) != 1 { return reflect.Value{}, fmt.Errorf("enum %s variant %s requires exactly 1 payload argument, got %d", expectedType, v, len(value.EnumPayload)) }
				out := reflect.New(target).Elem()
				tag := out.FieldByName("Tag")
				if !tag.IsValid() || !tag.CanSet() {
					return reflect.Value{}, fmt.Errorf("enum %s has unsupported compiled representation", expectedType)
				}
				tag.SetInt(int64(i))
				if meta.PayloadTypes[i] != nil {
					payload, err := __octMaterialize(value.EnumPayload[0], meta.PayloadTypes[i], meta.PayloadNames[i])
					if err != nil { return reflect.Value{}, fmt.Errorf("enum %s variant %s payload mismatch: %w", expectedType, v, err) }
					out.FieldByName("Payload").Set(payload)
				}
				return out, nil
			}
		}
		return reflect.Value{}, fmt.Errorf("enum %s has no variant %s", expectedType, value.EnumVariant)
	}
	switch target.Kind() {
	case reflect.Int:
		if value.Kind != __octParsedInt {
			return reflect.Value{}, fmt.Errorf("expected %s, got non-int value", expectedType)
		}
		out := reflect.New(target).Elem()
		out.SetInt(int64(value.Int))
		if err := __octValidateRefinement(expectedType, out); err != nil { return reflect.Value{}, err }
		return out, nil
	case reflect.Float64:
		if value.Kind != __octParsedFloat {
			return reflect.Value{}, fmt.Errorf("expected %s, got non-float value", expectedType)
		}
		out := reflect.New(target).Elem()
		out.SetFloat(value.Float)
		if err := __octValidateRefinement(expectedType, out); err != nil { return reflect.Value{}, err }
		return out, nil
	case reflect.Bool:
		if value.Kind != __octParsedBool {
			return reflect.Value{}, fmt.Errorf("expected Bool, got non-bool value")
		}
		out := reflect.New(target).Elem()
		out.SetBool(value.Bool)
		if err := __octValidateRefinement(expectedType, out); err != nil { return reflect.Value{}, err }
		return out, nil
	case reflect.String:
		if value.Kind != __octParsedString {
			return reflect.Value{}, fmt.Errorf("expected String, got non-string value")
		}
		out := reflect.New(target).Elem()
		out.SetString(value.Text)
		if err := __octValidateRefinement(expectedType, out); err != nil { return reflect.Value{}, err }
		return out, nil
	case reflect.Slice:
		if value.Kind != __octParsedArray {
			return reflect.Value{}, fmt.Errorf("expected %s, got non-array value", expectedType)
		}
		out := reflect.MakeSlice(target, 0, len(value.Array))
		elementExpectedType := strings.TrimSuffix(expectedType, "[]")
		for i, item := range value.Array {
			element, err := __octMaterialize(item, target.Elem(), elementExpectedType)
			if err != nil {
				return reflect.Value{}, fmt.Errorf("array element %d mismatch: %w", i, err)
			}
			out = reflect.Append(out, element)
		}
		return out, nil
	case reflect.Struct:
		meta, ok := __octRecordMetaByGoType[__octTypeKey(target)]
		if !ok {
			return reflect.Value{}, fmt.Errorf("unsupported expected type %s", expectedType)
		}
		if value.Kind != __octParsedRecord {
			return reflect.Value{}, fmt.Errorf("expected %s, got non-record value", expectedType)
		}
		if value.RecordType != meta.ShortName && value.RecordType != meta.FullName {
			return reflect.Value{}, fmt.Errorf("expected record %s, got %s", expectedType, value.RecordType)
		}
		out := reflect.New(target).Elem()
		for _, field := range meta.Fields {
			fieldValue, ok := value.RecordFields[field]
			if !ok {
				return reflect.Value{}, fmt.Errorf("record %s missing field %s", expectedType, field)
			}
			fieldExpectedType := meta.FieldTypes[field]
			materialized, err := __octMaterialize(fieldValue, out.FieldByName(field).Type(), fieldExpectedType)
			if err != nil {
				return reflect.Value{}, fmt.Errorf("record field %s mismatch: %w", field, err)
			}
			out.FieldByName(field).Set(materialized)
		}
		keys := make([]string, 0, len(value.RecordFields))
		for k := range value.RecordFields {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			found := false
			for _, field := range meta.Fields {
				if k == field {
					found = true
					break
				}
			}
			if !found {
				return reflect.Value{}, fmt.Errorf("record %s has unexpected field %s", expectedType, k)
			}
		}
		return out, nil
	default:
		return reflect.Value{}, fmt.Errorf("unsupported expected type %s", expectedType)
	}
}
`

const __octBatchHelpersTemplate = `
type __octBatchStrategy uint8

const (
	__octBatchSequential __octBatchStrategy = iota
	__octBatchChunkedParallel
)

type __octBatchPlan struct {
	itemCount int
	workerCount int
	chunkSize int
	strategy __octBatchStrategy
}

func __octMakeBatchPlan(itemCount int, parallelAllowed bool) __octBatchPlan {
	plan := __octBatchPlan{itemCount: itemCount, workerCount: 1, chunkSize: itemCount, strategy: __octBatchSequential}
	if itemCount <= __OCT_BATCH_SEQUENTIAL_MAX__ || !parallelAllowed {
		return plan
	}
	workerCount := runtime.GOMAXPROCS(0)
	if workerCount < 1 {
		workerCount = 1
	}
	workersForSize := (itemCount + __OCT_BATCH_MIN_ITEMS_PER_WORKER__ - 1) / __OCT_BATCH_MIN_ITEMS_PER_WORKER__
	if workerCount > workersForSize {
		workerCount = workersForSize
	}
	if workerCount <= 1 {
		return plan
	}
	plan.workerCount = workerCount
	plan.chunkSize = (itemCount + workerCount - 1) / workerCount
	plan.strategy = __octBatchChunkedParallel
	return plan
}

func __octBatchRun[T any, U any, R any](items []T, worker func(T) R, isErr func(R) bool, errMsg func(R) string, value func(R) U, parallelAllowed bool) ([]U, string, bool) {
	if len(items) == 0 {
		return []U{}, "", false
	}
	plan := __octMakeBatchPlan(len(items), parallelAllowed)
	ordered := make([]U, len(items))
	if plan.strategy == __octBatchSequential {
		failedIndex := len(items)
		var failedErr string
		for index := range items {
			out := worker(items[index])
			if isErr(out) {
				if index < failedIndex {
					failedIndex = index
					failedErr = errMsg(out)
				}
				continue
			}
			ordered[index] = value(out)
		}
		if failedIndex != len(items) {
			return nil, failedErr, true
		}
		return ordered, "", false
	}
	type __octBatchFailure struct {
		index int
		err string
		failed bool
	}
	failures := make([]__octBatchFailure, plan.workerCount)
	var wg sync.WaitGroup
	for slot := 0; slot < plan.workerCount; slot++ {
		start := slot * plan.chunkSize
		end := start + plan.chunkSize
		if end > len(items) {
			end = len(items)
		}
		if start >= end {
			continue
		}
		wg.Add(1)
		go func(slot, start, end int) {
			defer wg.Done()
			failure := __octBatchFailure{index: end}
			for index := start; index < end; index++ {
				out := worker(items[index])
				if isErr(out) {
					if !failure.failed {
						failure = __octBatchFailure{index: index, err: errMsg(out), failed: true}
					}
					continue
				}
				ordered[index] = value(out)
			}
			failures[slot] = failure
		}(slot, start, end)
	}
	wg.Wait()
	failedIndex := len(items)
	var failedErr string
	for _, failure := range failures {
		if failure.failed && failure.index < failedIndex {
			failedIndex = failure.index
			failedErr = failure.err
		}
	}
	if failedIndex != len(items) {
		return nil, failedErr, true
	}
	return ordered, "", false
}
`

func octBatchHelpers() string {
	helpers := strings.ReplaceAll(__octBatchHelpersTemplate, "__OCT_BATCH_SEQUENTIAL_MAX__", fmt.Sprint(batchplan.SequentialMaxItems))
	return strings.ReplaceAll(helpers, "__OCT_BATCH_MIN_ITEMS_PER_WORKER__", fmt.Sprint(batchplan.MinItemsPerWorker))
}

const __octUtilityCandidate = `
type __octUtilCandidate[T any] struct {
	Valid bool
	Value T
	Score int
}
`

const __octGenericUtilityHelpers = `
type __octUtilitySiteState struct {
	HasCurrent bool
	Current any
	Score int
	CommitAge int
}

func __octUtilSelect[T any](sites map[int]__octUtilitySiteState, siteID int, hysteresis int, minCommit int, candidates []__octUtilCandidate[T], elseValue T) T {
	valid := make([]__octUtilCandidate[T], 0, len(candidates))
	for _, c := range candidates {
		if c.Valid {
			valid = append(valid, c)
		}
	}
	next := __octUtilCandidate[T]{Valid: true, Value: elseValue, Score: 0}
	if len(valid) > 0 {
		next = valid[0]
		for _, c := range valid[1:] {
			if c.Score > next.Score {
				next = c
			}
		}
	}
	site := sites[siteID]
	if site.HasCurrent {
		currentStillValid := false
		for _, c := range valid {
			if reflect.DeepEqual(c.Value, site.Current) {
				currentStillValid = true
				break
			}
		}
		if currentStillValid {
			commitActive := site.CommitAge < minCommit
			hysteresisBlocks := next.Score <= site.Score+hysteresis
			if commitActive || hysteresisBlocks {
				next = __octUtilCandidate[T]{Valid: true, Value: site.Current.(T), Score: site.Score}
			}
		}
	}
	if !site.HasCurrent || !reflect.DeepEqual(site.Current, next.Value) {
		sites[siteID] = __octUtilitySiteState{HasCurrent: true, Current: next.Value, Score: next.Score, CommitAge: 1}
	} else {
		site.Score = next.Score
		site.CommitAge++
		sites[siteID] = site
	}
	return next.Value
}
`

const __octScalarUtilityHelpers = `
type __octScalarUtilitySiteState[T comparable] struct {
	HasCurrent bool
	Current T
	Score int
	CommitAge int
}

func __octUtilSelectScalar[T comparable](site *__octScalarUtilitySiteState[T], hysteresis int, minCommit int, candidates []__octUtilCandidate[T], elseValue T) T {
	next := __octUtilCandidate[T]{}
	for _, candidate := range candidates {
		if candidate.Valid && (!next.Valid || candidate.Score > next.Score) {
			next = candidate
		}
	}
	if !next.Valid {
		next = __octUtilCandidate[T]{Valid: true, Value: elseValue, Score: 0}
	}
	if site.HasCurrent {
		currentStillValid := false
		for _, candidate := range candidates {
			if candidate.Valid && candidate.Value == site.Current {
				currentStillValid = true
				break
			}
		}
		if currentStillValid {
			commitActive := site.CommitAge < minCommit
			hysteresisBlocks := next.Score <= site.Score+hysteresis
			if commitActive || hysteresisBlocks {
				next = __octUtilCandidate[T]{Valid: true, Value: site.Current, Score: site.Score}
			}
		}
	}
	if !site.HasCurrent || site.Current != next.Value {
		*site = __octScalarUtilitySiteState[T]{HasCurrent: true, Current: next.Value, Score: next.Score, CommitAge: 1}
	} else {
		site.Score = next.Score
		site.CommitAge++
	}
	return next.Value
}
`

const __octOctxiliaryHelpers = `
var __octOctxiliaryOnce sync.Once
var __octOctxiliaryCmd *exec.Cmd
var __octOctxiliaryIn io.WriteCloser
var __octOctxiliaryOut io.ReadCloser
var __octOctxiliaryErr error
var __octOctxiliaryMu sync.Mutex
var __octOctxiliaryReqID int
type __octOctxiliaryClient struct { cmd *exec.Cmd; in io.WriteCloser; out io.ReadCloser; mu sync.Mutex; reqID int; err error; closed bool }
var __octOctxiliaryGenericMu sync.Mutex
var __octOctxiliaryGenericClients = map[string]*__octOctxiliaryClient{}

func __octOctxiliaryWaitOrKill(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil { return }
	waitDone := make(chan error, 1)
	go func(){ waitDone <- cmd.Wait() }()
	select {
	case <-waitDone:
	case <-time.After(2 * time.Second):
		_ = cmd.Process.Kill()
		<-waitDone
	}
}

func __octOctxiliaryCloseClient(client *__octOctxiliaryClient) {
	if client == nil { return }
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.closed { return }
	client.closed = true
	if client.in != nil { _ = client.in.Close(); client.in = nil }
	__octOctxiliaryWaitOrKill(client.cmd)
	client.cmd = nil
	if client.out != nil { _ = client.out.Close(); client.out = nil }
}

func __octOctxiliaryClose() {
	__octOctxiliaryMu.Lock()
	cmd, in, out := __octOctxiliaryCmd, __octOctxiliaryIn, __octOctxiliaryOut
	__octOctxiliaryCmd, __octOctxiliaryIn, __octOctxiliaryOut = nil, nil, nil
	__octOctxiliaryMu.Unlock()
	if in != nil { _ = in.Close() }
	__octOctxiliaryWaitOrKill(cmd)
	if out != nil { _ = out.Close() }
	__octOctxiliaryGenericMu.Lock()
	clients := make([]*__octOctxiliaryClient, 0, len(__octOctxiliaryGenericClients))
	for _, client := range __octOctxiliaryGenericClients { clients = append(clients, client) }
	__octOctxiliaryGenericClients = map[string]*__octOctxiliaryClient{}
	__octOctxiliaryGenericMu.Unlock()
	for _, client := range clients { __octOctxiliaryCloseClient(client) }
}

func __octGenericFallible(sidecarCommand string, family string, function string, args []octxiliary.Value, expected octxiliary.ValueKind) (octxiliary.Value, error) {
	return __octOctxiliaryGenericCall(sidecarCommand, family, function, args, expected)
}

func __octJsonString(function string, input string) octResult_String {
	value, err := __octGenericFallible("octxiliary-json", "Json", function, []octxiliary.Value{{Kind: octxiliary.ValueString, String: input}}, octxiliary.ValueString)
	if err != nil { return octResult_String{Err: err.Error(), IsErr: true} }
	return octResult_String{Value: value.String}
}

func __octJsonSave(path string, input string) octResult_Int {
	value, err := __octGenericFallible("octxiliary-json", "Json", "JsonSave", []octxiliary.Value{{Kind: octxiliary.ValueString, String: path}, {Kind: octxiliary.ValueString, String: input}}, octxiliary.ValueInt)
	if err != nil { return octResult_Int{Err: err.Error(), IsErr: true} }
	return octResult_Int{Value: value.Int}
}

func __octCsvReadRows(path string) octResult_StringSliceSlice {
	value, err := __octGenericFallible("octxiliary-csv", "Csv", "CsvReadRows", []octxiliary.Value{{Kind: octxiliary.ValueString, String: path}}, octxiliary.ValueStringMatrix)
	if err != nil { return octResult_StringSliceSlice{Err: err.Error(), IsErr: true} }
	return octResult_StringSliceSlice{Value: value.Strings2}
}

func __octCsvWriteRows(path string, rows [][]string) octResult_Int {
	value, err := __octGenericFallible("octxiliary-csv", "Csv", "CsvWriteRows", []octxiliary.Value{{Kind: octxiliary.ValueString, String: path}, {Kind: octxiliary.ValueStringMatrix, Strings2: rows}}, octxiliary.ValueInt)
	if err != nil { return octResult_Int{Err: err.Error(), IsErr: true} }
	return octResult_Int{Value: value.Int}
}

func __octCsvReadMatrix(path string) octResult_FloatSliceSlice {
	rows := __octCsvReadRows(path)
	if rows.IsErr { return octResult_FloatSliceSlice{Err: rows.Err, IsErr: true} }
	if len(rows.Value) == 0 { return octResult_FloatSliceSlice{Err: "InvalidData: csv matrix requires at least one row", IsErr: true} }
	matrix := make([][]float64, 0, len(rows.Value))
	for rowIndex, row := range rows.Value {
		floatRow := make([]float64, 0, len(row))
		for colIndex, cell := range row {
			parsed, err := strconv.ParseFloat(cell, 64)
			if err != nil { return octResult_FloatSliceSlice{Err: fmt.Sprintf("InvalidData: non-numeric cell at row %d column %d: %q", rowIndex+1, colIndex+1, cell), IsErr: true} }
			floatRow = append(floatRow, parsed)
		}
		matrix = append(matrix, floatRow)
	}
	return octResult_FloatSliceSlice{Value: matrix}
}

func __octCsvReadTable(path string) octResult_Csv_Table {
	rows := __octCsvReadRows(path)
	if rows.IsErr { return octResult_Csv_Table{Err: rows.Err, IsErr: true} }
	if len(rows.Value) == 0 { return octResult_Csv_Table{Err: "InvalidData: csv table requires at least one header row", IsErr: true} }
	headerRow := rows.Value[0]
	if len(headerRow) == 0 { return octResult_Csv_Table{Err: "InvalidData: csv table header row cannot be empty", IsErr: true} }
	seen := map[string]int{}
	for idx, header := range headerRow {
		if header == "" { return octResult_Csv_Table{Err: fmt.Sprintf("InvalidData: csv table header %d is empty", idx+1), IsErr: true} }
		if prior, ok := seen[header]; ok { return octResult_Csv_Table{Err: fmt.Sprintf("InvalidData: duplicate csv table header %q at columns %d and %d", header, prior+1, idx+1), IsErr: true} }
		seen[header] = idx
	}
	for rowIndex := 1; rowIndex < len(rows.Value); rowIndex++ {
		if len(rows.Value[rowIndex]) != len(headerRow) { return octResult_Csv_Table{Err: fmt.Sprintf("InvalidData: inconsistent column count at row %d", rowIndex+1), IsErr: true} }
	}
	return octResult_Csv_Table{Value: Csv_Table{}}
}

func __octFileReadText(path string) octResult_String {
	__octOctxiliaryMu.Lock()
	defer __octOctxiliaryMu.Unlock()
	if err := __octOctxiliaryEnsure(); err != nil { return octResult_String{Err: err.Error(), IsErr: true} }
	__octOctxiliaryReqID++
	req := octxiliary.Request{ID: __octOctxiliaryReqID, Family: "IO.File", Function: "FileReadText", Path: path}
	if err := octxiliary.WriteFrame(__octOctxiliaryIn, octxiliary.EncodeRequest(req)); err != nil { return octResult_String{Err: err.Error(), IsErr: true} }
	frame, err := octxiliary.ReadFrame(__octOctxiliaryOut); if err != nil { return octResult_String{Err: err.Error(), IsErr: true} }
	resp, _ := octxiliary.ParseResponse(frame)
	if !resp.OK { return octResult_String{Err: resp.Error, IsErr: true} }
	return octResult_String{Value: resp.Text}
}

func __octFileWriteText(path string, text string) octResult_Int {
	__octOctxiliaryMu.Lock()
	defer __octOctxiliaryMu.Unlock()
	if err := __octOctxiliaryEnsure(); err != nil { return octResult_Int{Err: err.Error(), IsErr: true} }
	__octOctxiliaryReqID++
	req := octxiliary.Request{ID: __octOctxiliaryReqID, Family: "IO.File", Function: "FileWriteText", Path: path, Text: text}
	if err := octxiliary.WriteFrame(__octOctxiliaryIn, octxiliary.EncodeRequest(req)); err != nil { return octResult_Int{Err: err.Error(), IsErr: true} }
	frame, err := octxiliary.ReadFrame(__octOctxiliaryOut); if err != nil { return octResult_Int{Err: err.Error(), IsErr: true} }
	resp, _ := octxiliary.ParseResponse(frame)
	if !resp.OK { return octResult_Int{Err: resp.Error, IsErr: true} }
	return octResult_Int{Value: 0}
}

func __octFileReadBytes(path string) octResult_Bytes {
	__octOctxiliaryMu.Lock()
	defer __octOctxiliaryMu.Unlock()
	if err := __octOctxiliaryEnsure(); err != nil { return octResult_Bytes{Err: err.Error(), IsErr: true} }
	__octOctxiliaryReqID++
	req := octxiliary.Request{ID: __octOctxiliaryReqID, Family: "IO.File", Function: "FileReadBytes", Path: path}
	if err := octxiliary.WriteFrame(__octOctxiliaryIn, octxiliary.EncodeRequest(req)); err != nil { return octResult_Bytes{Err: err.Error(), IsErr: true} }
	frame, err := octxiliary.ReadFrame(__octOctxiliaryOut); if err != nil { return octResult_Bytes{Err: err.Error(), IsErr: true} }
	resp, _ := octxiliary.ParseResponse(frame)
	if !resp.OK { return octResult_Bytes{Err: resp.Error, IsErr: true} }
	return octResult_Bytes{Value: resp.Bytes}
}

func __octFileWriteBytes(path string, data []byte) octResult_Int {
	__octOctxiliaryMu.Lock()
	defer __octOctxiliaryMu.Unlock()
	if err := __octOctxiliaryEnsure(); err != nil { return octResult_Int{Err: err.Error(), IsErr: true} }
	__octOctxiliaryReqID++
	req := octxiliary.Request{ID: __octOctxiliaryReqID, Family: "IO.File", Function: "FileWriteBytes", Path: path, Bytes: data, HasBytes: true}
	if err := octxiliary.WriteFrame(__octOctxiliaryIn, octxiliary.EncodeRequest(req)); err != nil { return octResult_Int{Err: err.Error(), IsErr: true} }
	frame, err := octxiliary.ReadFrame(__octOctxiliaryOut); if err != nil { return octResult_Int{Err: err.Error(), IsErr: true} }
	resp, _ := octxiliary.ParseResponse(frame)
	if !resp.OK { return octResult_Int{Err: resp.Error, IsErr: true} }
	return octResult_Int{Value: 0}
}

func __octFileReadLines(path string) octResult_StringSlice {
	__octOctxiliaryMu.Lock()
	defer __octOctxiliaryMu.Unlock()
	if err := __octOctxiliaryEnsure(); err != nil { return octResult_StringSlice{Err: err.Error(), IsErr: true} }
	__octOctxiliaryReqID++
	req := octxiliary.Request{ID: __octOctxiliaryReqID, Family: "IO.File", Function: "FileReadLines", Path: path}
	if err := octxiliary.WriteFrame(__octOctxiliaryIn, octxiliary.EncodeRequest(req)); err != nil { return octResult_StringSlice{Err: err.Error(), IsErr: true} }
	frame, err := octxiliary.ReadFrame(__octOctxiliaryOut); if err != nil { return octResult_StringSlice{Err: err.Error(), IsErr: true} }
	resp, _ := octxiliary.ParseResponse(frame)
	if !resp.OK { return octResult_StringSlice{Err: resp.Error, IsErr: true} }
	return octResult_StringSlice{Value: resp.Lines}
}

func __octFileWriteLines(path string, lines []string) octResult_Int {
	__octOctxiliaryMu.Lock()
	defer __octOctxiliaryMu.Unlock()
	if err := __octOctxiliaryEnsure(); err != nil { return octResult_Int{Err: err.Error(), IsErr: true} }
	__octOctxiliaryReqID++
	req := octxiliary.Request{ID: __octOctxiliaryReqID, Family: "IO.File", Function: "FileWriteLines", Path: path, Lines: lines, HasLines: true}
	if err := octxiliary.WriteFrame(__octOctxiliaryIn, octxiliary.EncodeRequest(req)); err != nil { return octResult_Int{Err: err.Error(), IsErr: true} }
	frame, err := octxiliary.ReadFrame(__octOctxiliaryOut); if err != nil { return octResult_Int{Err: err.Error(), IsErr: true} }
	resp, _ := octxiliary.ParseResponse(frame)
	if !resp.OK { return octResult_Int{Err: resp.Error, IsErr: true} }
	return octResult_Int{Value: 0}
}

func __octFileDelete(path string) octResult_Int {
	__octOctxiliaryMu.Lock()
	defer __octOctxiliaryMu.Unlock()
	if err := __octOctxiliaryEnsure(); err != nil { return octResult_Int{Err: err.Error(), IsErr: true} }
	__octOctxiliaryReqID++
	req := octxiliary.Request{ID: __octOctxiliaryReqID, Family: "IO.File", Function: "FileDelete", Path: path}
	if err := octxiliary.WriteFrame(__octOctxiliaryIn, octxiliary.EncodeRequest(req)); err != nil { return octResult_Int{Err: err.Error(), IsErr: true} }
	frame, err := octxiliary.ReadFrame(__octOctxiliaryOut); if err != nil { return octResult_Int{Err: err.Error(), IsErr: true} }
	resp, _ := octxiliary.ParseResponse(frame)
	if !resp.OK { return octResult_Int{Err: resp.Error, IsErr: true} }
	return octResult_Int{Value: 0}
}

func __octDirectoryList(path string) octResult_StringSlice {
	__octOctxiliaryMu.Lock()
	defer __octOctxiliaryMu.Unlock()
	if err := __octOctxiliaryEnsure(); err != nil { return octResult_StringSlice{Err: err.Error(), IsErr: true} }
	__octOctxiliaryReqID++
	req := octxiliary.Request{ID: __octOctxiliaryReqID, Family: "Directory", Function: "DirectoryList", Path: path}
	if err := octxiliary.WriteFrame(__octOctxiliaryIn, octxiliary.EncodeRequest(req)); err != nil { return octResult_StringSlice{Err: err.Error(), IsErr: true} }
	frame, err := octxiliary.ReadFrame(__octOctxiliaryOut); if err != nil { return octResult_StringSlice{Err: err.Error(), IsErr: true} }
	resp, _ := octxiliary.ParseResponse(frame)
	if !resp.OK { return octResult_StringSlice{Err: resp.Error, IsErr: true} }
	return octResult_StringSlice{Value: resp.Lines}
}

func __octDirectoryMake(path string) octResult_Int {
	__octOctxiliaryMu.Lock()
	defer __octOctxiliaryMu.Unlock()
	if err := __octOctxiliaryEnsure(); err != nil { return octResult_Int{Err: err.Error(), IsErr: true} }
	__octOctxiliaryReqID++
	req := octxiliary.Request{ID: __octOctxiliaryReqID, Family: "Directory", Function: "DirectoryMake", Path: path}
	if err := octxiliary.WriteFrame(__octOctxiliaryIn, octxiliary.EncodeRequest(req)); err != nil { return octResult_Int{Err: err.Error(), IsErr: true} }
	frame, err := octxiliary.ReadFrame(__octOctxiliaryOut); if err != nil { return octResult_Int{Err: err.Error(), IsErr: true} }
	resp, _ := octxiliary.ParseResponse(frame)
	if !resp.OK { return octResult_Int{Err: resp.Error, IsErr: true} }
	return octResult_Int{Value: 0}
}

func __octDirectoryMakeAll(path string) octResult_Int {
	__octOctxiliaryMu.Lock()
	defer __octOctxiliaryMu.Unlock()
	if err := __octOctxiliaryEnsure(); err != nil { return octResult_Int{Err: err.Error(), IsErr: true} }
	__octOctxiliaryReqID++
	req := octxiliary.Request{ID: __octOctxiliaryReqID, Family: "Directory", Function: "DirectoryMakeAll", Path: path}
	if err := octxiliary.WriteFrame(__octOctxiliaryIn, octxiliary.EncodeRequest(req)); err != nil { return octResult_Int{Err: err.Error(), IsErr: true} }
	frame, err := octxiliary.ReadFrame(__octOctxiliaryOut); if err != nil { return octResult_Int{Err: err.Error(), IsErr: true} }
	resp, _ := octxiliary.ParseResponse(frame)
	if !resp.OK { return octResult_Int{Err: resp.Error, IsErr: true} }
	return octResult_Int{Value: 0}
}

func __octDirectoryRemoveAll(path string) octResult_Int {
	__octOctxiliaryMu.Lock()
	defer __octOctxiliaryMu.Unlock()
	if err := __octOctxiliaryEnsure(); err != nil { return octResult_Int{Err: err.Error(), IsErr: true} }
	__octOctxiliaryReqID++
	req := octxiliary.Request{ID: __octOctxiliaryReqID, Family: "Directory", Function: "DirectoryRemoveAll", Path: path}
	if err := octxiliary.WriteFrame(__octOctxiliaryIn, octxiliary.EncodeRequest(req)); err != nil { return octResult_Int{Err: err.Error(), IsErr: true} }
	frame, err := octxiliary.ReadFrame(__octOctxiliaryOut); if err != nil { return octResult_Int{Err: err.Error(), IsErr: true} }
	resp, _ := octxiliary.ParseResponse(frame)
	if !resp.OK { return octResult_Int{Err: resp.Error, IsErr: true} }
	return octResult_Int{Value: 0}
}


func __octOctxiliaryGenericCall(sidecarCommand string, family string, function string, args []octxiliary.Value, expected octxiliary.ValueKind) (octxiliary.Value, error) {
	client := __octOctxiliaryGenericClient(sidecarCommand)
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.err != nil { return octxiliary.Value{}, client.err }
	client.reqID++
	req := octxiliary.Request{ID: client.reqID, Family: family, Function: function, Args: args, HasArgs: true}
	if err := octxiliary.ValidateRequest(req); err != nil { return octxiliary.Value{}, err }
	if err := octxiliary.WriteFrame(client.in, octxiliary.EncodeRequest(req)); err != nil { return octxiliary.Value{}, err }
	frame, err := octxiliary.ReadFrame(client.out); if err != nil { return octxiliary.Value{}, err }
	resp, err := octxiliary.ParseResponse(frame); if err != nil { return octxiliary.Value{}, err }
	if err := octxiliary.ValidateResponse(resp); err != nil { return octxiliary.Value{}, err }
	if !resp.OK { return octxiliary.Value{}, errors.New(resp.Error) }
	if !resp.HasValue { return octxiliary.Value{}, errors.New("Octxiliary generic response missing typed value") }
	if resp.Value.Kind != expected { return octxiliary.Value{}, fmt.Errorf("Octxiliary generic response kind mismatch: expected %s, got %s", expected, resp.Value.Kind) }
	return resp.Value, nil
}

func __octOctxiliaryValidateHandle(value octxiliary.Value, family string, handleType string) error {
	if value.Kind != octxiliary.ValueHandle { return fmt.Errorf("Octxiliary handle response kind mismatch: expected Handle, got %s", value.Kind) }
	if value.HandleFamily != family { return fmt.Errorf("Octxiliary handle response family mismatch: expected %s, got %s", family, value.HandleFamily) }
	if value.HandleType != handleType { return fmt.Errorf("Octxiliary handle response type mismatch: expected %s, got %s", handleType, value.HandleType) }
	if value.HandleID <= 0 { return fmt.Errorf("Octxiliary handle response ID must be positive") }
	return nil
}

func __octOctxiliaryGenericClient(sidecarCommand string) *__octOctxiliaryClient {
	__octOctxiliaryGenericMu.Lock()
	defer __octOctxiliaryGenericMu.Unlock()
	if client, ok := __octOctxiliaryGenericClients[sidecarCommand]; ok { return client }
	client := &__octOctxiliaryClient{}
	path, err := __octOctxiliarySidecarPath(sidecarCommand)
	if err != nil { client.err = err; __octOctxiliaryGenericClients[sidecarCommand] = client; return client }
	cmd := exec.Command(path)
	in, _ := cmd.StdinPipe(); out, _ := cmd.StdoutPipe(); if err := cmd.Start(); err != nil { client.err = err; __octOctxiliaryGenericClients[sidecarCommand] = client; return client }
	client.cmd, client.in, client.out = cmd, in, out
	if err := octxiliary.WriteHandshake(in); err != nil { client.err = err; __octOctxiliaryGenericClients[sidecarCommand] = client; return client }
	if err := octxiliary.ReadHandshake(out); err != nil { client.err = err; __octOctxiliaryGenericClients[sidecarCommand] = client; return client }
	__octOctxiliaryGenericClients[sidecarCommand] = client
	return client
}

func __octOctxiliarySidecarPath(sidecarCommand string) (string, error) {
	if sidecarCommand == "" { return "", errors.New("Octxiliary sidecar command is empty") }
	if path, ok := __octOctxiliaryResolveSidecarInDir(filepath.Dir(os.Args[0]), sidecarCommand); ok { return path, nil }
	wrapperPath := os.Getenv("OCT_WRAPPER_PATH")
	if wrapperPath != "" {
		if path, ok := __octOctxiliaryResolveSidecarFromWrapperPath(wrapperPath, sidecarCommand); ok { return path, nil }
	}
	return "", fmt.Errorf("Octxiliary sidecar %q not found; set OCT_WRAPPER_PATH or place it beside the compiled executable", sidecarCommand)
}

func __octOctxiliaryResolveSidecarFromWrapperPath(wrapperPath string, sidecarCommand string) (string, bool) {
	info, err := os.Stat(wrapperPath)
	if err != nil { return "", false }
	if info.IsDir() { return __octOctxiliaryResolveSidecarInDir(wrapperPath, sidecarCommand) }
	if __octOctxiliarySidecarBasenameMatches(filepath.Base(wrapperPath), sidecarCommand) { return wrapperPath, true }
	return "", false
}

func __octOctxiliaryResolveSidecarInDir(dir string, sidecarCommand string) (string, bool) {
	for _, name := range __octOctxiliarySidecarCommandCandidates(sidecarCommand) {
		candidate := filepath.Join(dir, name)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() { return candidate, true }
	}
	return "", false
}

func __octOctxiliarySidecarCommandCandidates(sidecarCommand string) []string {
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(sidecarCommand), ".exe") { return []string{sidecarCommand, sidecarCommand + ".exe"} }
	return []string{sidecarCommand}
}

func __octOctxiliarySidecarBasenameMatches(base string, sidecarCommand string) bool {
	for _, candidate := range __octOctxiliarySidecarCommandCandidates(sidecarCommand) {
		if base == candidate { return true }
	}
	return false
}

func __octOctxiliaryEnsure() error {
	__octOctxiliaryOnce.Do(func(){
		path, ok := __octOctxiliaryResolveSidecarInDir(filepath.Dir(os.Args[0]), "octxiliary-io")
		if !ok {
			wrapperPath := os.Getenv("OCT_WRAPPER_PATH")
			if wrapperPath != "" { path, ok = __octOctxiliaryResolveSidecarFromWrapperPath(wrapperPath, "octxiliary-io") }
		}
		if !ok { __octOctxiliaryErr = errors.New("Octxiliary sidecar not found; set OCT_WRAPPER_PATH or place octxiliary-io beside the compiled executable") ; return }
		cmd := exec.Command(path)
		in, _ := cmd.StdinPipe(); out, _ := cmd.StdoutPipe(); if err := cmd.Start(); err != nil { __octOctxiliaryErr = err; return }
		__octOctxiliaryCmd, __octOctxiliaryIn, __octOctxiliaryOut = cmd, in, out
		if err := octxiliary.WriteHandshake(in); err != nil { __octOctxiliaryErr = err; return }
		if err := octxiliary.ReadHandshake(out); err != nil { __octOctxiliaryErr = err; return }
	})
	return __octOctxiliaryErr
}
`
