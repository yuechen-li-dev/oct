package validate

import (
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/sdslv/lex"
	"github.com/yuechen-li-dev/oct/internal/sdslv/parse"
	"github.com/yuechen-li-dev/oct/internal/source"
)

func TestModuleRejectsDuplicateRecordFields(t *testing.T) {
	err := validateSource(`record Params { M: u32; M: u32; }`)
	if err == nil || !strings.Contains(err.Error(), "duplicate record field") {
		t.Fatalf("error = %v, want duplicate record field", err)
	}
}

func TestModuleRejectsUnknownTypes(t *testing.T) {
	err := validateSource(`record Params { M: Missing; }`)
	if err == nil || !strings.Contains(err.Error(), "unknown type Missing") {
		t.Fatalf("error = %v, want unknown type", err)
	}
}

func TestSdslvNdarrayTypeIdentityUsesOrderedShapeAndKind(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "shape order differs",
			src: `fn F() -> void {
let a: ndarray<u32, [2u, 3u]>;
let b: ndarray<u32, [3u, 2u]> = a;
return;
}`,
			want: "cannot assign ndarray<u32, [2u, 3u]> to local b of type ndarray<u32, [3u, 2u]>",
		},
		{
			name: "nested arrays remain distinct",
			src: `fn F() -> void {
let a: ndarray<u32, [2u, 3u]>;
let b: array<array<u32, 3u>, 2u> = a;
return;
}`,
			want: "cannot assign ndarray<u32, [2u, 3u]> to local b of type array<array<u32,N>,N>",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateSource(tc.src)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestSdslvNdarrayWholeValueAssignmentUsesExactTypeSemantics(t *testing.T) {
	err := validateSource(`fn F() -> void {
let a: ndarray<u32, [2u, 2u]>;
let b: ndarray<u32, [2u, 2u]> = a;
b = a;
return;
}`)
	if err != nil {
		t.Fatalf("validateSource() error = %v", err)
	}
}

func TestSdslvFixedShapeConstructionValidation(t *testing.T) {
	valid := `fn F(seed: u32) -> void {
let a: ndarray<u32, [4u]> = Fill(seed);
let b: ndarray<u32, [2u, 3u]> = Generate[i, j](i * 3u + j);
return;
}`
	if err := validateSource(valid); err != nil {
		t.Fatalf("valid construction: %v", err)
	}
	for _, tc := range []struct{ src, want string }{
		{`fn F() -> void { Fill(0u); }`, "SDSL-V3313"},
		{`fn F() -> void { let a: ndarray<u32, [2u]> = Fill(); }`, "SDSL-V3315"},
		{`fn F() -> void { let a: ndarray<u32, [2u]> = Fill(1u, 2u); }`, "SDSL-V3315"},
		{`fn F() -> void { let a: ndarray<u32, [2u, 2u]> = Generate[i](i); }`, "SDSL-V3319"},
		{`fn F() -> void { let a: ndarray<u32, [2u]> = Generate[i, i](i); }`, "SDSL-V3320"},
		{`fn F() -> void { let a: ndarray<u32, [2u]> = Generate[i](i = 1u); }`, "expected ')'"},
	} {
		err := validateSource(tc.src)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("error = %v, want %q", err, tc.want)
		}
	}
}

func TestModuleValidatesShaderLocalBoardValues(t *testing.T) {
	err := validateSource(`board LoadCoord {
linear: u32;
row: u32;
col: u32;
}
fn MakeLoadCoord(localThreadLinear: u32, lane: u32, tileK: u32) -> LoadCoord {
let linear: u32 = localThreadLinear * 4u + lane;
return LoadCoord { linear: linear; row: linear / tileK; col: linear % tileK; };
}
shader S {
resources { A: readonly array<f32>; C: readwrite array<f32>; }
workgroup Tile: tile<f32, 16u, 16u>;
stage compute [numthreads(1, 1, 1)] fn CS(fullTile: bool) -> void {
let localThreadLinear: u32 = GroupThreadID.x;
let AView: matrix_view<f32> = row_major(A, 16u, 16u);
comptime for lane in 0u..4u {
let p: LoadCoord = MakeLoadCoord(localThreadLinear, lane, 16u);
when {
case fullTile -> {
Tile[p.row, p.col] = AView[p.row, p.col];
}
else -> {
Tile[p.row, p.col] = read AView[p.row, p.col] when p.row < 16u and p.col < 16u else 0.0;
}
}
}
return;
}
}`)
	if err != nil {
		t.Fatalf("validateSource() error = %v", err)
	}
}

func TestModuleValidatesOrderedDerive(t *testing.T) {
	err := validateSource(`record LoadFacts {
linear: u32;
row: u32;
col: u32;
valid: bool;
}
board LoadCoord {
linear: u32;
row: u32;
col: u32;
}
shader S {
resources { A: readonly array<f32>; }
workgroup Tile: tile<f32, 4u, 4u>;
stage compute [numthreads(1, 1, 1)] fn CS(limit: u32, tileK: u32) -> void {
let facts: LoadFacts = derive {
linear = GroupThreadID.x;
row = linear / tileK;
col = linear % tileK;
valid = row < limit and col < tileK;
};
let coord: LoadCoord = derive {
linear = facts.linear;
row = facts.row;
col = facts.col;
};
when {
case facts.valid -> {
Tile[coord.row, coord.col] = A[(coord.row * tileK) + coord.col];
}
else -> {
return;
}
}
return;
}
}`)
	if err != nil {
		t.Fatalf("validateSource() error = %v", err)
	}
}

func TestSdslvM35aVectorAndIntrinsicValidationMatrix(t *testing.T) {
	valid := `fn F(bits: u32, raw: u32, value: f32, signed: i32, fv2: float2, fv3: float3, fv4: float4, uv4: uint4) -> void {
let a: f32 = fv2.x;
let b: f32 = fv3.z;
let c: f32 = fv4.w;
let d: u32 = uv4.y;
let dot2: f32 = Dot(fv2, fv2);
let dot3: f32 = Dot(fv3, fv3);
let dot4: f32 = Dot(fv4, fv4);
let unpacked: float2 = Unpack<F16x2>(bits);
let repacked: u32 = Pack<F16x2>(unpacked);
let bitsAgain: u32 = Bitcast<u32>(value);
let floatAgain: f32 = Bitcast<f32>(bitsAgain);
let intAgain: i32 = Bitcast<i32>(raw);
let uintAgain: u32 = Bitcast<u32>(signed);
let fromUInt: f32 = Convert<f32>(raw);
let fromInt: f32 = Convert<f32>(signed);
let toUInt: u32 = Convert<u32>(value);
let toInt: i32 = Convert<i32>(value);
let intToUInt: u32 = Convert<u32>(signed);
let uintToInt: i32 = Convert<i32>(raw);
let nested: f32 = Dot(Unpack<F16x2>(repacked), Unpack<F16x2>(bits));
return;
}`
	if err := validateSource(valid); err != nil {
		t.Fatalf("validateSource() error = %v", err)
	}

	cases := []struct {
		name string
		src  string
		want string
	}{
		{"scalar component", `fn F(x: f32) -> void { let y: f32 = x.x; }`, "SDSL-V3501"},
		{"float2 z", `fn F(v: float2) -> void { let z: f32 = v.z; }`, "SDSL-V3501"},
		{"float3 w", `fn F(v: float3) -> void { let w: f32 = v.w; }`, "SDSL-V3501"},
		{"unknown component", `record R { value: f32; } fn F(r: R) -> void { let x: f32 = r.q; }`, "SDSL-V1506"},
		{"dot arity", `fn F(v: float2) -> void { let x: f32 = Dot(v); }`, "SDSL-V3509"},
		{"dot scalar", `fn F(x: f32) -> void { let y: f32 = Dot(x, x); }`, "SDSL-V3510"},
		{"dot mixed width", `fn F(a: float2, b: float3) -> void { let y: f32 = Dot(a, b); }`, "SDSL-V3510"},
		{"dot mixed type", `fn F(a: float2, b: uint2) -> void { let y: f32 = Dot(a, b); }`, "SDSL-V3510"},
		{"unknown format", `fn F(bits: u32) -> void { let x: float2 = Unpack<UNorm8x4>(bits); }`, "SDSL-V3504"},
		{"pack wrong source", `fn F(bits: u32) -> void { let x: u32 = Pack<F16x2>(bits); }`, "SDSL-V3505"},
		{"unpack wrong source", `fn F(pair: float2) -> void { let x: float2 = Unpack<F16x2>(pair); }`, "SDSL-V3506"},
		{"bitcast unsupported", `fn F(value: f32) -> void { let x: float2 = Bitcast<float2>(value); }`, "SDSL-V3507"},
		{"convert unsupported", `fn F(pair: float2) -> void { let x: u32 = Convert<u32>(pair); }`, "SDSL-V3508"},
		{"ordinary user generic", `fn Helper(x: u32) -> u32 { return x; } fn F(bits: u32) -> void { let x: u32 = Helper<F16x2>(bits); }`, "SDSL-V3502"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateSource(tc.src)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestModuleRejectsInvalidDerive(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "missing expected type",
			src: `fn F() -> void {
derive {
linear = 1u;
};
return;
}`,
			want: "derive requires an explicit record or board target type",
		},
		{
			name: "scalar target",
			src: `fn F() -> void {
let value: u32 = derive {
linear = 1u;
};
return;
}`,
			want: "derive requires an explicit record or board target type",
		},
		{
			name: "missing field",
			src: `record LoadFacts { linear: u32; row: u32; }
fn F() -> LoadFacts {
return derive {
linear = 1u;
};
}`,
			want: "missing derive field `row`",
		},
		{
			name: "unknown field",
			src: `board LoadCoord { row: u32; }
fn F() -> LoadCoord {
return derive {
row = 1u;
stride = 2u;
};
}`,
			want: "unknown derive field `stride`",
		},
		{
			name: "duplicate field",
			src: `board LoadCoord { row: u32; }
fn F() -> LoadCoord {
return derive {
row = 1u;
row = 2u;
};
}`,
			want: "duplicate derive field `row`",
		},
		{
			name: "forward reference",
			src: `record LoadFacts { row: u32; linear: u32; }
fn F() -> LoadFacts {
return derive {
row = linear;
linear = 1u;
};
}`,
			want: "derive field `row` references later field `linear`",
		},
		{
			name: "self reference",
			src: `record LoadFacts { row: u32; }
fn F() -> LoadFacts {
return derive {
row = row + 1u;
};
}`,
			want: "derive field `row` cannot reference itself",
		},
		{
			name: "type mismatch",
			src: `record LoadFacts { valid: bool; }
fn F() -> LoadFacts {
return derive {
valid = 1u;
};
}`,
			want: "derive field `valid` requires bool, got u32",
		},
		{
			name: "comptime rejected",
			src: `record LoadFacts { row: u32; }
fn F() -> void {
comptime let load: LoadFacts = derive {
row = 1u;
};
return;
}`,
			want: "derive is not a compile-time expression in SDSL-V M25",
		},
		{
			name: "flow board rejected",
			src: `board LoadCoord { row: u32; }
shader S {
stage compute [numthreads(1,1,1)] fn CS() -> void {
flow TileLoad {
board Load: LoadCoord = derive {
row = 0u;
};
state Compute { return; }
}
return;
}
}`,
			want: "derive constructs immutable values; flow-owned mutable boards require an explicit board initializer",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateSource(tc.src)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestModuleRejectsInvalidBoardValues(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "duplicate board field",
			src:  `board LoadCoord { row: u32; row: u32; }`,
			want: "duplicate board field",
		},
		{
			name: "unsupported field type",
			src:  `board Bad { View: matrix_view<f32>; }`,
			want: "board field Bad.View type matrix_view<f32> is not supported",
		},
		{
			name: "missing literal field",
			src:  `board LoadCoord { row: u32; col: u32; } fn F() -> LoadCoord { return LoadCoord { row: 1u; }; }`,
			want: "missing board literal field col",
		},
		{
			name: "duplicate literal field",
			src:  `board LoadCoord { row: u32; } fn F() -> LoadCoord { return LoadCoord { row: 1u; row: 2u; }; }`,
			want: "duplicate board literal field row",
		},
		{
			name: "unknown literal field",
			src:  `board LoadCoord { row: u32; } fn F() -> LoadCoord { return LoadCoord { row: 1u; col: 2u; }; }`,
			want: "unknown board literal field col",
		},
		{
			name: "field type mismatch",
			src:  `board LoadCoord { row: u32; } fn F() -> LoadCoord { return LoadCoord { row: true; }; }`,
			want: "board literal field row on LoadCoord expects u32, got bool",
		},
		{
			name: "unknown field access",
			src:  `board LoadCoord { row: u32; } fn F(p: LoadCoord) -> u32 { return p.col; }`,
			want: "unknown field col on LoadCoord",
		},
		{
			name: "field assignment",
			src:  `board LoadCoord { row: u32; } fn F() -> LoadCoord { let p: LoadCoord = LoadCoord { row: 1u; }; p.row = 2u; return p; }`,
			want: "board values are immutable in SDSL-V M21",
		},
		{
			name: "comptime board",
			src:  `board LoadCoord { row: u32; } fn F() -> void { comptime let p: LoadCoord = LoadCoord { row: 1u; }; return; }`,
			want: "structured consteval boards are not supported",
		},
		{
			name: "resource board element",
			src:  `board LoadCoord { row: u32; } shader S { resources { P: readonly array<LoadCoord>; } stage compute [numthreads(1, 1, 1)] fn CS() -> void { return; } }`,
			want: "boards are shader-local values",
		},
		{
			name: "stage board parameter",
			src:  `board LoadCoord { row: u32; } shader S { stage compute [numthreads(1, 1, 1)] fn CS(p: LoadCoord) -> void { return; } }`,
			want: "stage parameter p cannot use board type LoadCoord",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateSource(tc.src)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestModuleValidatesTileAndMatrixViewIndexing(t *testing.T) {
	err := validateSource(`shader S {
resources { A: readonly array<f32>; C: readwrite array<f32>; }
workgroup Tile: tile<f32, 16, 16>;
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
let AView: matrix_view<f32> = row_major(A, 16u, 16u);
let CView: matrix_view<f32> = row_major(C, 16u, 16u);
Tile[0u, 1u] = AView[0u, 1u];
CView[0u, 1u] = Tile[0u, 1u];
return;
}
}`)
	if err != nil {
		t.Fatalf("validateSource() error = %v", err)
	}
}

func TestModuleRejectsInvalidTileAndMatrixViewUse(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "tile one dimensional index",
			src:  `shader S { workgroup Tile: tile<f32, 16, 16>; stage compute [numthreads(1, 1, 1)] fn CS() -> void { let x: f32 = Tile[0u]; return; } }`,
			want: "tile values require explicit 2D indexing",
		},
		{
			name: "readonly view write",
			src:  `shader S { resources { A: readonly array<f32>; } stage compute [numthreads(1, 1, 1)] fn CS() -> void { let AView: matrix_view<f32> = row_major(A, 4u, 4u); AView[0u, 0u] = 1.0; return; } }`,
			want: "cannot assign through readonly matrix_view",
		},
		{
			name: "row major local array",
			src:  `fn F() -> void { let values: array<f32, 4>; let V: matrix_view<f32> = row_major(values, 2u, 2u); return; }`,
			want: "row_major first argument must be a resource array",
		},
		{
			name: "row major wrong arg count",
			src:  `shader S { resources { A: readonly array<f32>; } stage compute [numthreads(1, 1, 1)] fn CS() -> void { let AView: matrix_view<f32> = row_major(A, 4u); return; } }`,
			want: "row_major expects 3 arguments",
		},
		{
			name: "non integer 2D index",
			src:  `shader S { workgroup Tile: tile<f32, 4, 4>; stage compute [numthreads(1, 1, 1)] fn CS() -> void { let x: f32 = Tile[true, 0u]; return; } }`,
			want: "array index must be integer",
		},
		{
			name: "zero tile dimension",
			src:  `shader S { workgroup Tile: tile<f32, 0, 16>; stage compute [numthreads(1, 1, 1)] fn CS() -> void { return; } }`,
			want: "tile rows must be positive",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateSource(tc.src)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestModuleValidatesRegTileUse(t *testing.T) {
	err := validateSource(`concept MicroConfig {
Outputs: {
M: u32 = 2u;
N: u32 = 2u;
};
}
config Micro2x2: MicroConfig {}
template<C: MicroConfig>
shader S {
resources { C: readwrite array<f32>; }
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
comptime let RM: u32 = C.Outputs.M;
comptime let RN: u32 = C.Outputs.N;
let Acc: reg_tile<f32, RM, RN> = reg_tile_zero();
let CView: matrix_view<f32> = row_major(C, 2u, 2u);
Acc[0u, 1u] = Acc[0u, 1u] + 1.0;
CView[0u, 1u] = Acc[0u, 1u];
return;
}
}
compile S<Micro2x2> as S2x2;`)
	if err != nil {
		t.Fatalf("error = %v, want nil", err)
	}
}

func TestModuleRejectsInvalidRegTileUse(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "runtime dimension",
			src:  `shader S { stage compute [numthreads(1, 1, 1)] fn CS() -> void { let n: u32 = 2u; let Acc: reg_tile<f32, n, 2u> = reg_tile_zero(); return; } }`,
			want: "local reg_tile Acc rows must be a compile-time positive integer expression",
		},
		{
			name: "too many elements",
			src:  `shader S { stage compute [numthreads(1, 1, 1)] fn CS() -> void { let Acc: reg_tile<f32, 8u, 9u> = reg_tile_zero(); return; } }`,
			want: "reg_tile has 72 elements; M15 limit is 64",
		},
		{
			name: "unsupported element",
			src:  `shader S { stage compute [numthreads(1, 1, 1)] fn CS() -> void { let Acc: reg_tile<bool, 2u, 2u> = reg_tile_zero(); return; } }`,
			want: "element type bool is not supported",
		},
		{
			name: "workgroup rejected",
			src:  `shader S { workgroup Acc: reg_tile<f32, 2u, 2u>; stage compute [numthreads(1, 1, 1)] fn CS() -> void { return; } }`,
			want: "cannot use reg_tile",
		},
		{
			name: "resource rejected",
			src:  `stream IO { Acc: readonly reg_tile<f32, 2u, 2u>; } shader S { resources IO; stage compute [numthreads(1, 1, 1)] fn CS() -> void { return; } }`,
			want: "cannot use reg_tile",
		},
		{
			name: "one dimensional index",
			src:  `shader S { stage compute [numthreads(1, 1, 1)] fn CS() -> void { let Acc: reg_tile<f32, 2u, 2u> = reg_tile_zero(); let x: f32 = Acc[0u]; return; } }`,
			want: "reg_tile values require explicit 2D indexing",
		},
		{
			name: "wrong initializer",
			src:  `shader S { stage compute [numthreads(1, 1, 1)] fn CS() -> void { let Acc: reg_tile<f32, 2u, 2u>; return; } }`,
			want: "reg_tile locals must be initialized with reg_tile_zero()",
		},
		{
			name: "whole tile copy rejected",
			src:  `shader S { stage compute [numthreads(1, 1, 1)] fn CS() -> void { let A: reg_tile<f32, 2u, 2u> = reg_tile_zero(); let B: reg_tile<f32, 2u, 2u> = A; return; } }`,
			want: "reg_tile locals must be initialized with reg_tile_zero()",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateSource(tc.src)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestModuleValidatesGuardedMemoryAccess(t *testing.T) {
	err := validateSource(`shader S {
resources { A: readonly array<f32>; C: readwrite array<f32>; }
stage compute [numthreads(1, 1, 1)] fn CS(row: u32, col: u32, guard: bool) -> void {
let AView: matrix_view<f32> = row_major(A, 4u, 4u);
let CView: matrix_view<f32> = row_major(C, 4u, 4u);
let value: f32 = read AView[row, col] when guard and not false else 0.0;
write CView[row, col] = value when guard or false;
return;
}
}`)
	if err != nil {
		t.Fatalf("error = %v, want nil", err)
	}
}

func TestModuleValidatesGuardWhen(t *testing.T) {
	err := validateSource(`shader S {
resources { A: readonly array<f32>; C: readwrite array<f32>; }
stage compute [numthreads(1, 1, 1)] fn CS(row: u32, col: u32, full: bool) -> void {
let AView: matrix_view<f32> = row_major(A, 4u, 4u);
let CView: matrix_view<f32> = row_major(C, 4u, 4u);
when {
case full -> {
let value: f32 = AView[row, col];
write CView[row, col] = value when row < 4u and col < 4u;
}
case not full -> {
let value: f32 = read AView[row, col] when row < 4u and col < 4u else 0.0;
write CView[row, col] = value when true;
}
}
return;
}
}`)
	if err != nil {
		t.Fatalf("error = %v, want nil", err)
	}
}

func TestModuleValidatesFlowStateBlocks(t *testing.T) {
	err := validateSource(`board LoadCoord {
row: u32;
col: u32;
}
fn Make(row: u32, col: u32) -> LoadCoord {
return LoadCoord { row: row; col: col; };
}
shader S {
resources { A: readonly array<f32>; C: readwrite array<f32>; }
workgroup Tile: tile<f32, 4u, 4u>;
stage compute [numthreads(1, 1, 1)] fn CS(fullTile: bool) -> void {
let AView: matrix_view<f32> = row_major(A, 4u, 4u);
let CView: matrix_view<f32> = row_major(C, 4u, 4u);
flow TileLoad {
state Load {
comptime for lane in 0u..1u {
let p: LoadCoord = Make(lane, lane);
when {
case fullTile -> {
Tile[p.row, p.col] = AView[p.row, p.col];
}
else -> {
write CView[p.row, p.col] = read AView[p.row, p.col] when true else 0.0 when true;
}
}
}
}
state Sync {
WorkgroupMemoryBarrierWithSync();
}
}
return;
}
}`)
	if err != nil {
		t.Fatalf("error = %v, want nil", err)
	}
}

func TestModuleRejectsInvalidFlowStateBlocks(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "duplicate state",
			src: `shader S {
stage compute [numthreads(1,1,1)] fn CS() -> void {
flow TileLoad {
state Load { return; }
state Load { return; }
}
return;
}
}`,
			want: "flow TileLoad: duplicate state name Load",
		},
		{
			name: "empty flow",
			src: `shader S {
stage compute [numthreads(1,1,1)] fn CS() -> void {
flow Empty {}
return;
}
}`,
			want: "flow Empty must declare at least one state",
		},
		{
			name: "nested flow",
			src: `shader S {
stage compute [numthreads(1,1,1)] fn CS() -> void {
flow Outer {
state A {
flow Inner {
state B { return; }
}
}
}
return;
}
}`,
			want: "nested flow blocks are not supported in SDSL-V M22",
		},
		{
			name: "duplicate flow name same scope",
			src: `shader S {
stage compute [numthreads(1,1,1)] fn CS() -> void {
flow TileLoad { state A { return; } }
flow TileLoad { state B { return; } }
return;
}
}`,
			want: "duplicate flow block name TileLoad",
		},
		{
			name: "immutable board field assignment in state",
			src: `board LoadCoord { row: u32; }
shader S {
stage compute [numthreads(1,1,1)] fn CS() -> void {
let p: LoadCoord = LoadCoord { row: 1u; };
flow TileLoad {
state Load {
p.row = 2u;
}
}
return;
}
}`,
			want: "only flow-owned board instances may be mutated in SDSL-V M23",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateSource(tc.src)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestModuleValidatesFlowBoundMutableBoards(t *testing.T) {
	err := validateSource(`board LoadCoord {
row: u32;
col: u32;
valid: bool;
}
shader S {
resources { A: readonly array<f32>; C: readwrite array<f32>; }
workgroup Tile: tile<f32, 4u, 4u>;
stage compute [numthreads(1,1,1)] fn CS(flag: bool) -> void {
let AView: matrix_view<f32> = row_major(A, 4u, 4u);
let CView: matrix_view<f32> = row_major(C, 4u, 4u);
flow TileLoad {
board Load: LoadCoord = LoadCoord { row: 0u; col: 0u; valid: false; };
state Compute {
comptime for lane in 0u..1u {
Load.row = lane;
Load.col = lane;
when {
case flag -> {
Load.valid = true;
}
else -> {
Load.valid = false;
}
}
Tile[Load.row, Load.col] = read AView[Load.row, Load.col] when Load.valid else 0.0;
write CView[Load.row, Load.col] = Tile[Load.row, Load.col] when Load.valid;
}
}
}
return;
}
}`)
	if err != nil {
		t.Fatalf("error = %v, want nil", err)
	}
}

func TestModuleRejectsInvalidFlowBoundMutableBoards(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "initializer required",
			src: `board LoadCoord { row: u32; }
shader S {
stage compute [numthreads(1,1,1)] fn CS() -> void {
flow TileLoad {
board Load: LoadCoord;
state Compute { return; }
}
return;
}
}`,
			want: "expected '=' after flow board instance type",
		},
		{
			name: "initializer type mismatch",
			src: `board LoadCoord { row: u32; }
shader S {
stage compute [numthreads(1,1,1)] fn CS() -> void {
flow TileLoad {
board Load: LoadCoord = true;
state Compute { return; }
}
return;
}
}`,
			want: "flow TileLoad board Load initializer expects LoadCoord, got bool",
		},
		{
			name: "duplicate flow board names",
			src: `board LoadCoord { row: u32; }
shader S {
stage compute [numthreads(1,1,1)] fn CS() -> void {
flow TileLoad {
board Load: LoadCoord = LoadCoord { row: 0u; };
board Load: LoadCoord = LoadCoord { row: 1u; };
state Compute { return; }
}
return;
}
}`,
			want: "flow TileLoad: duplicate board instance name Load",
		},
		{
			name: "unknown board type",
			src: `shader S {
stage compute [numthreads(1,1,1)] fn CS() -> void {
flow TileLoad {
board Load: Missing = 1u;
state Compute { return; }
}
return;
}
}`,
			want: "flow TileLoad board Load must use a board type, got Missing",
		},
		{
			name: "whole board reassignment rejected",
			src: `board LoadCoord { row: u32; }
shader S {
stage compute [numthreads(1,1,1)] fn CS() -> void {
flow TileLoad {
board Load: LoadCoord = LoadCoord { row: 0u; };
state Compute {
Load = LoadCoord { row: 1u; };
}
}
return;
}
}`,
			want: "whole-board reassignment is not supported for flow-owned board Load in SDSL-V M23",
		},
		{
			name: "board not visible outside flow",
			src: `board LoadCoord { row: u32; }
shader S {
stage compute [numthreads(1,1,1)] fn CS() -> void {
flow TileLoad {
board Load: LoadCoord = LoadCoord { row: 0u; };
state Compute { return; }
}
Load.row = 1u;
return;
}
}`,
			want: "unknown identifier Load",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateSource(tc.src)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestModuleRejectsInvalidGuardWhen(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "non bool guard",
			src:  `shader S { stage compute [numthreads(1,1,1)] fn CS() -> void { when { case 1u -> { return; } } return; } }`,
			want: "guard when case condition must be bool",
		},
		{
			name: "runtime guard when expression",
			src:  `fn F(flag: bool) -> u32 { let x: u32 = when { case flag -> { return 1u; } }; return x; }`,
			want: "runtime guard when is a statement-only form",
		},
		{
			name: "c style logical rejected",
			src:  `fn F(a: bool, b: bool) -> void { when { case a && b -> { return; } } return; }`,
			want: "use `and` instead of `&&`",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateSource(tc.src)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestModuleRejectsInvalidGuardedMemoryAccess(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "guarded read target must be indexed memory",
			src:  `fn F(flag: bool) -> f32 { return read (1.0 + 2.0) when flag else 0.0; }`,
			want: "guarded read target must be an indexed memory expression",
		},
		{
			name: "guarded read guard must be bool",
			src:  `shader S { resources { A: readonly array<f32>; } stage compute [numthreads(1,1,1)] fn CS() -> void { let AView: matrix_view<f32> = row_major(A, 4u, 4u); let x: f32 = read AView[0u, 0u] when 1u else 0.0; return; } }`,
			want: "guarded read condition must be bool",
		},
		{
			name: "guarded read fallback type mismatch",
			src:  `shader S { resources { A: readonly array<f32>; } stage compute [numthreads(1,1,1)] fn CS() -> void { let AView: matrix_view<f32> = row_major(A, 4u, 4u); let x: f32 = read AView[0u, 0u] when true else 1u; return; } }`,
			want: "guarded read fallback type does not match target element type",
		},
		{
			name: "guarded write readonly rejected",
			src:  `shader S { resources { A: readonly array<f32>; } stage compute [numthreads(1,1,1)] fn CS() -> void { let AView: matrix_view<f32> = row_major(A, 4u, 4u); write AView[0u, 0u] = 1.0 when true; return; } }`,
			want: "cannot guarded-write to readonly matrix view",
		},
		{
			name: "guarded write condition must be bool",
			src:  `shader S { resources { C: readwrite array<f32>; } stage compute [numthreads(1,1,1)] fn CS() -> void { let CView: matrix_view<f32> = row_major(C, 4u, 4u); write CView[0u, 0u] = 1.0 when 1u; return; } }`,
			want: "guarded write condition must be bool",
		},
		{
			name: "guarded write value mismatch",
			src:  `shader S { resources { C: readwrite array<f32>; } stage compute [numthreads(1,1,1)] fn CS() -> void { let CView: matrix_view<f32> = row_major(C, 4u, 4u); write CView[0u, 0u] = 1u when true; return; } }`,
			want: "guarded write value type does not match target element type",
		},
		{
			name: "guarded write target must be writable indexed memory",
			src:  `fn F(flag: bool) -> void { write flag = flag when true; return; }`,
			want: "guarded write target must be a writable indexed memory expression",
		},
		{
			name: "guarded read rejected in comptime",
			src:  `shader S { resources { A: readonly array<f32>; } stage compute [numthreads(1,1,1)] fn CS() -> void { let AView: matrix_view<f32> = row_major(A, 4u, 4u); comptime let X: f32 = read AView[0u, 0u] when true else 0.0; return; } }`,
			want: "guarded read is not a compile-time expression",
		},
		{
			name: "guarded read nested placement rejected",
			src:  `shader S { resources { A: readonly array<f32>; } stage compute [numthreads(1,1,1)] fn CS() -> void { let AView: matrix_view<f32> = row_major(A, 4u, 4u); let x: f32 = 1.0 + read AView[0u, 0u] when true else 0.0; return; } }`,
			want: "guarded read expression is only supported as a direct let initializer",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateSource(tc.src)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestModuleRejectsBadReturnType(t *testing.T) {
	err := validateSource(`fn F() -> u32 { return 1.0; }`)
	if err == nil || !strings.Contains(err.Error(), "return type mismatch") {
		t.Fatalf("error = %v, want return type mismatch", err)
	}
}

func TestModuleRejectsUnsupportedStage(t *testing.T) {
	err := validateSource(`shader S { stage vertex fn VS() -> void { return; } }`)
	if err == nil || !strings.Contains(err.Error(), "not implemented in GoOct SDSL-V M0") {
		t.Fatalf("error = %v, want M0 diagnostic", err)
	}
}

func TestModuleRejectsDuplicateStreamFields(t *testing.T) {
	err := validateSource(`stream ComputeThread { GroupIndex: u32; GroupIndex: u32; }`)
	if err == nil || !strings.Contains(err.Error(), "duplicate stream field") {
		t.Fatalf("error = %v, want duplicate stream field", err)
	}
}

func TestModuleRejectsUnknownWithField(t *testing.T) {
	err := validateSource(`record Surface { Roughness: f32; }
fn F(s: Surface) -> Surface { return s with { Missing: 0.5 }; }`)
	if err == nil || !strings.Contains(err.Error(), "unknown with field Missing") {
		t.Fatalf("error = %v, want unknown with field", err)
	}
}

func TestModuleRejectsDuplicateEnumVariant(t *testing.T) {
	err := validateSource(`enum LoadValue { Zero; Zero; }`)
	if err == nil || !strings.Contains(err.Error(), "duplicate enum variant") {
		t.Fatalf("error = %v, want duplicate enum variant", err)
	}
}

func TestModuleRejectsDuplicateEnumPayloadField(t *testing.T) {
	err := validateSource(`enum LoadValue { Value { X: f32; X: f32; } }`)
	if err == nil || !strings.Contains(err.Error(), "duplicate enum payload field") {
		t.Fatalf("error = %v, want duplicate enum payload field", err)
	}
}

func TestModuleRejectsEnumPayloadConstructionShapeErrors(t *testing.T) {
	err := validateSource(`enum LoadValue { Zero; Value { X: f32; } }
fn F() -> LoadValue { return LoadValue.Value; }`)
	if err == nil || !strings.Contains(err.Error(), "requires payload construction") {
		t.Fatalf("error = %v, want payload construction diagnostic", err)
	}
	err = validateSource(`enum LoadValue { Zero; Value { X: f32; } }
fn F() -> LoadValue { return LoadValue.Value { Y: 1.0 }; }`)
	if err == nil || !strings.Contains(err.Error(), "unknown payload field Y") {
		t.Fatalf("error = %v, want unknown payload field diagnostic", err)
	}
	err = validateSource(`enum Bounds { Tail { Rows: u32; Cols: u32; } }
fn F() -> Bounds { return Bounds.Tail { Rows: 1u }; }`)
	if err == nil || !strings.Contains(err.Error(), "missing payload field Cols") {
		t.Fatalf("error = %v, want missing payload field diagnostic", err)
	}
}

func TestModuleRejectsEnumMatchErrors(t *testing.T) {
	err := validateSource(`enum LoadValue { Zero; Value { X: f32; } }
fn F(v: LoadValue) -> f32 {
return match v {
LoadValue.Zero => 0.0
};
}`)
	if err == nil || !strings.Contains(err.Error(), "match missing arm") {
		t.Fatalf("error = %v, want missing arm diagnostic", err)
	}
	err = validateSource(`enum LoadValue { Zero; Value { X: f32; } }
fn F(v: LoadValue) -> f32 {
return match v {
LoadValue.Zero => 0.0
LoadValue.Zero => 1.0
};
}`)
	if err == nil || !strings.Contains(err.Error(), "duplicate match arm") {
		t.Fatalf("error = %v, want duplicate match arm diagnostic", err)
	}
	err = validateSource(`enum LoadValue { Zero; Value { X: f32; } }
enum Other { Zero; }
fn F(v: LoadValue) -> f32 {
return match v {
Other.Zero => 0.0
LoadValue.Value(payload) => payload.X
};
}`)
	if err == nil || !strings.Contains(err.Error(), "does not match subject enum") {
		t.Fatalf("error = %v, want wrong enum diagnostic", err)
	}
}

func TestModuleRejectsEnumMatchTypeAndScopeErrors(t *testing.T) {
	err := validateSource(`enum LoadValue { Zero; Value { X: f32; } }
fn F(v: LoadValue) -> f32 {
return match v {
LoadValue.Zero => 0.0
LoadValue.Value(payload) => true
};
}`)
	if err == nil || !strings.Contains(err.Error(), "uniform type") {
		t.Fatalf("error = %v, want arm type mismatch diagnostic", err)
	}
	err = validateSource(`enum LoadValue { Zero; Value { X: f32; } }
fn F(v: LoadValue) -> f32 {
let out: f32 = match v {
LoadValue.Zero => payload.X
LoadValue.Value(payload) => payload.X
};
return out;
}`)
	if err == nil || !strings.Contains(err.Error(), "unknown identifier payload") {
		t.Fatalf("error = %v, want payload scope diagnostic", err)
	}
	err = validateSource(`enum LoadValue { Zero; Value { X: f32; } }
fn F(v: LoadValue) -> f32 {
let out: f32 = match v {
LoadValue.Zero(payload) => 0.0
LoadValue.Value(bound) => bound.X
};
return out;
}`)
	if err == nil || !strings.Contains(err.Error(), "must not bind payload") {
		t.Fatalf("error = %v, want simple variant binding diagnostic", err)
	}
}

func TestModuleRejectsNestedMatchPlacement(t *testing.T) {
	err := validateSource(`enum LoadValue { Zero; Value { X: f32; } }
fn F(v: LoadValue) -> f32 { return G(match v { LoadValue.Zero => 0.0 LoadValue.Value(payload) => payload.X }); }
fn G(x: f32) -> f32 { return x; }`)
	if err == nil || !strings.Contains(err.Error(), "match expression is only supported as a direct let initializer, assignment RHS, or return value") {
		t.Fatalf("error = %v, want match placement diagnostic", err)
	}
}

func TestModuleRejectsDuplicateWithField(t *testing.T) {
	err := validateSource(`record Surface { Roughness: f32; }
fn F(s: Surface) -> Surface { return s with { Roughness: 0.5, Roughness: 1.0 }; }`)
	if err == nil || !strings.Contains(err.Error(), "duplicate with field Roughness") {
		t.Fatalf("error = %v, want duplicate with field", err)
	}
}

func TestModuleRejectsWrongWithFieldType(t *testing.T) {
	err := validateSource(`record Surface { Roughness: f32; }
fn F(s: Surface) -> Surface { return s with { Roughness: true }; }`)
	if err == nil || !strings.Contains(err.Error(), "with field Roughness expects f32, got bool") {
		t.Fatalf("error = %v, want wrong with field type", err)
	}
}

func TestModuleRejectsRecordParameterFieldAssignment(t *testing.T) {
	err := validateSource(`record Surface { Roughness: f32; }
fn Bad(s: Surface) -> Surface { s.Roughness = 0.5; return s; }`)
	if err == nil || !strings.Contains(err.Error(), "use with instead") {
		t.Fatalf("error = %v, want immutable record parameter diagnostic", err)
	}
}

func TestModuleRejectsStreamParameterFieldAssignment(t *testing.T) {
	err := validateSource(`stream ComputeThread {
DispatchId: uint3;
GroupId: uint3;
GroupThreadId: uint3;
GroupIndex: u32;
}
fn Bad(t: ComputeThread) -> u32 { t.DispatchId.x = 1u; return 0u; }`)
	if err == nil || !strings.Contains(err.Error(), "immutable stream parameter t") {
		t.Fatalf("error = %v, want immutable stream parameter diagnostic", err)
	}
}

func TestModuleAllowsLocalRecordFieldAssignment(t *testing.T) {
	err := validateSource(`record Surface { Roughness: f32; }
fn Good(s: Surface) -> Surface {
let local: Surface = s;
local.Roughness = 0.5;
return local;
}`)
	if err != nil {
		t.Fatalf("error = %v, want nil", err)
	}
}

func TestModuleResolvesNamedResourceBundleStream(t *testing.T) {
	err := validateSource(`stream VectorAddIO {
A: readonly array<f32>;
C: readwrite array<f32>;
}
record Params { Count: u32; }
shader VectorAdd {
resources VectorAddIO;
stage compute [numthreads(16, 1, 1)] fn CS(params: Params) -> void { return; }
}`)
	if err != nil {
		t.Fatalf("error = %v, want nil", err)
	}
}

func TestModuleRejectsWithInUnsupportedNestedPosition(t *testing.T) {
	err := validateSource(`record Surface { Roughness: f32; }
fn G(s: Surface) -> Surface { return F(s with { Roughness: 0.5 }); }
fn F(s: Surface) -> Surface { return s; }`)
	if err == nil || !strings.Contains(err.Error(), "with expression is only supported as a direct let initializer, assignment RHS, or return value") {
		t.Fatalf("error = %v, want with placement diagnostic", err)
	}
}

func TestModuleAllowsWorkgroupArrayUseInComputeShader(t *testing.T) {
	err := validateSource(`stream ComputeThread {
DispatchId: uint3;
GroupId: uint3;
GroupThreadId: uint3;
GroupIndex: u32;
}
stream TileCopyIO {
A: readonly array<f32>;
C: readwrite array<f32>;
}
record Params { Count: u32; }
shader TileCopy {
resources TileCopyIO;
workgroup Tile: array<f32, 256>;
stage compute [numthreads(16, 16, 1)] fn CS(thread: ComputeThread, params: Params) -> void {
let idx: u32 = thread.DispatchId.x;
let local: u32 = thread.GroupIndex;
if idx < params.Count {
Tile[local] = A[idx];
}
WorkgroupMemoryBarrierWithSync();
if idx < params.Count {
C[idx] = Tile[local];
}
return;
}
}`)
	if err != nil {
		t.Fatalf("error = %v, want nil", err)
	}
}

func TestModuleRejectsRuntimeSizedWorkgroupArray(t *testing.T) {
	err := validateSource(`shader S {
workgroup Tile: array<f32>;
stage compute [numthreads(1, 1, 1)] fn CS() -> void { return; }
}`)
	if err == nil || !strings.Contains(err.Error(), "fixed-size array") {
		t.Fatalf("error = %v, want fixed-size workgroup diagnostic", err)
	}
}

func TestModuleRejectsDuplicateWorkgroupNames(t *testing.T) {
	err := validateSource(`shader S {
workgroup Tile: array<f32, 16>;
workgroup Tile: array<f32, 16>;
stage compute [numthreads(1, 1, 1)] fn CS() -> void { return; }
}`)
	if err == nil || !strings.Contains(err.Error(), "duplicate shader workgroup") {
		t.Fatalf("error = %v, want duplicate workgroup diagnostic", err)
	}
}

func TestModuleRejectsInvalidWorkgroupElementType(t *testing.T) {
	err := validateSource(`record Surface { Roughness: f32; }
shader S {
workgroup Tile: array<Surface, 16>;
stage compute [numthreads(1, 1, 1)] fn CS() -> void { return; }
}`)
	if err == nil || !strings.Contains(err.Error(), "element type") {
		t.Fatalf("error = %v, want invalid workgroup element diagnostic", err)
	}
}

func TestModuleRejectsBarrierWrongArgCount(t *testing.T) {
	err := validateSource(`shader S {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
WorkgroupBarrier(1u);
return;
}
}`)
	if err == nil || !strings.Contains(err.Error(), "expects 0 arguments") {
		t.Fatalf("error = %v, want barrier arg count diagnostic", err)
	}
}

func TestModuleRejectsBarrierInValuePosition(t *testing.T) {
	err := validateSource(`shader S {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
let x: void = WorkgroupBarrier();
return;
}
}`)
	if err == nil || !strings.Contains(err.Error(), "may only be used as an expression statement") {
		t.Fatalf("error = %v, want barrier placement diagnostic", err)
	}
}

func TestModuleRejectsWrongWorkgroupAssignmentType(t *testing.T) {
	err := validateSource(`shader S {
workgroup Tile: array<f32, 16>;
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
Tile[0u] = true;
return;
}
}`)
	if err == nil || !strings.Contains(err.Error(), "assignment type mismatch") {
		t.Fatalf("error = %v, want workgroup assignment mismatch", err)
	}
}

func TestModuleAllowsTemplateConfigCompileSpecialization(t *testing.T) {
	err := validateSource(`concept TileCopyConfig {
THREADS_X: u32;
THREADS_Y: u32;
TILE_SIZE: u32;
}
config Tile16x16: TileCopyConfig {
THREADS_X: 16u;
THREADS_Y: 16u;
TILE_SIZE: 256u;
}
template<C: TileCopyConfig>
shader TileCopy {
workgroup Tile: array<f32, C.TILE_SIZE>;
stage compute [numthreads(C.THREADS_X, C.THREADS_Y, 1u)] fn CS() -> void {
let tileElements: u32 = C.TILE_SIZE;
return;
}
}
compile TileCopy<Tile16x16> as TileCopy16x16;`)
	if err != nil {
		t.Fatalf("error = %v, want nil", err)
	}
}

func TestModuleAllowsComptimeLetAndIfSyntaxValidation(t *testing.T) {
	err := validateSource(`concept TileConfig {
Tile: { M: u32; N: u32; };
UseFastPath: bool = true;
}
config Tile16: TileConfig {
Tile.M => 16u;
Tile.N => 16u;
}
template<C: TileConfig>
shader TileCopy {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
comptime let TileElements: u32 = C.Tile.M * C.Tile.N;
comptime let IsFull: bool = TileElements == 256u;
comptime if C.UseFastPath and IsFull {
let value: u32 = TileElements;
} else {
let value: u32 = 0u;
}
return;
}
}
compile TileCopy<Tile16> as TileCopy16;`)
	if err != nil {
		t.Fatalf("error = %v, want nil", err)
	}
}

func TestModuleAllowsComptimeForSyntaxValidation(t *testing.T) {
	err := validateSource(`concept MicroConfig {
Outputs: { M: u32 = 2u; N: u32 = 2u; };
}
config Micro2x2: MicroConfig {}
template<C: MicroConfig>
shader TileCopy {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
let Acc: reg_tile<f32, C.Outputs.M, C.Outputs.N> = reg_tile_zero();
comptime for i in 0u..C.Outputs.M {
comptime for j in 0u..C.Outputs.N {
static assert i < C.Outputs.M and j < C.Outputs.N;
Acc[i, j] = 1.0;
}
}
return;
}
}
compile TileCopy<Micro2x2> as TileCopy2x2;`)
	if err != nil {
		t.Fatalf("error = %v, want nil", err)
	}
}

func TestModuleValidatesSemanticBooleanOperators(t *testing.T) {
	err := validateSource(`concept TileConfig {
Tile: { K: u32; };
UseFastPath: bool = true;
DisableFastPath: bool = false;
}
config Tile16: TileConfig {
Tile.K => 16u;
}
template<C: TileConfig>
shader TileCopy {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
comptime let UseFast: bool = C.UseFastPath and not C.DisableFastPath;
comptime if UseFast and C.Tile.K == 16u {
static assert UseFast or C.Tile.K == 8u;
}
let runtimeFlag: bool = true and not false;
if runtimeFlag or false {
return;
}
return;
}
}
compile TileCopy<Tile16> as TileCopy16;`)
	if err != nil {
		t.Fatalf("error = %v, want nil", err)
	}
}

func TestModuleRejectsSemanticBooleanOperatorTypeErrors(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{name: "and non bool", src: `fn F() -> bool { return 1u and true; }`, want: "operator `and` requires bool operands"},
		{name: "or non bool", src: `fn F() -> bool { return 1.0 or false; }`, want: "operator `or` requires bool operands"},
		{name: "not non bool", src: `fn F() -> bool { return not 1u; }`, want: "operator `not` requires bool operand"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateSource(tc.src)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestModuleRejectsComptimeTypeErrors(t *testing.T) {
	err := validateSource(`shader S {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
comptime let Bad: u32 = true;
return;
}
}`)
	if err == nil || !strings.Contains(err.Error(), "cannot assign bool to comptime local Bad of type u32") {
		t.Fatalf("error = %v, want comptime type mismatch", err)
	}
}

func TestModuleRejectsComptimeForValidationErrors(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "runtime start",
			src: `record Params { M: u32; }
shader S {
stage compute [numthreads(1, 1, 1)] fn CS(params: Params) -> void {
comptime for i in params.M..4u { return; }
return;
}
}`,
			want: "comptime for bounds must be compile-time integers",
		},
		{
			name: "non integer end",
			src: `shader S {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
comptime for i in 0u..true { return; }
return;
}
}`,
			want: "comptime for bounds must be compile-time integers",
		},
		{
			name: "negative bound",
			src: `shader S {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
comptime for i in -1..2u { return; }
return;
}
}`,
			want: "comptime for bounds must be non-negative in SDSL-V M16",
		},
		{
			name: "start greater than end",
			src: `shader S {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
comptime for i in 3u..2u { return; }
return;
}
}`,
			want: "comptime for range start must be <= end",
		},
		{
			name: "assignment to index",
			src: `shader S {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
comptime for i in 0u..2u {
i = 1u;
}
return;
}
}`,
			want: "cannot assign to comptime binding i",
		},
		{
			name: "out of scope after loop",
			src: `shader S {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
comptime for i in 0u..2u { return; }
let x: u32 = i;
return;
}
}`,
			want: "unknown identifier i",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateSource(tc.src)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestModuleAllowsSumAndProductReductions(t *testing.T) {
	err := validateSource(`fn Reduce(values: array<f32>) -> f32 {
let total: f32 = sum i in 0u..4u { values[i] };
let productValue: f32 = product j in 1..4 step 2 { values[j] };
return total + productValue;
}`)
	if err != nil {
		t.Fatalf("error = %v, want nil", err)
	}
}

func TestModuleRejectsReductionValidationErrors(t *testing.T) {
	err := validateSource(`fn Reduce(values: array<f32>) -> f32 {
let total: f32 = sum i in 0.0..4u { values[i] };
return total;
}`)
	if err == nil || !strings.Contains(err.Error(), "sum reduction bounds must be integer") {
		t.Fatalf("error = %v, want integer bounds diagnostic", err)
	}
	err = validateSource(`fn Reduce(values: array<f32>) -> f32 {
let total: f32 = sum i in 0u..4u step 0 { values[i] };
return total;
}`)
	if err == nil || !strings.Contains(err.Error(), "sum reduction step must be a positive integer literal") {
		t.Fatalf("error = %v, want positive step diagnostic", err)
	}
	err = validateSource(`fn Reduce() -> f32 {
let total: f32 = sum i in 0u..4u { true };
return total;
}`)
	if err == nil || !strings.Contains(err.Error(), "sum reduction body must be numeric") {
		t.Fatalf("error = %v, want numeric body diagnostic", err)
	}
}

func TestModuleRejectsNestedReductionPlacement(t *testing.T) {
	err := validateSource(`fn Reduce(values: array<f32>) -> f32 {
return 1.0 + sum i in 0u..4u { values[i] };
}`)
	if err == nil || !strings.Contains(err.Error(), "reduction expression is only supported as a direct let initializer, assignment RHS, or return value") {
		t.Fatalf("error = %v, want reduction placement diagnostic", err)
	}
}

func TestModuleAcceptsReductionAttributes(t *testing.T) {
	err := validateSource(`fn Reduce(values: array<f32>) -> f32 {
let total: f32 = [unroll] sum i in 0u..4u { values[i] };
let productValue: f32 = [loop] product j in 1..4 step 2 { values[j] };
return total + productValue;
}`)
	if err != nil {
		t.Fatalf("error = %v, want nil", err)
	}
}

func TestModuleRejectsConflictingReductionAttributes(t *testing.T) {
	err := validateSource(`fn Reduce(values: array<f32>) -> f32 {
let total: f32 = [unroll][loop] sum i in 0u..4u { values[i] };
return total;
}`)
	if err == nil || !strings.Contains(err.Error(), "reduction cannot declare both [unroll] and [loop]") {
		t.Fatalf("error = %v, want reduction attribute conflict", err)
	}
}

func TestModuleRejectsUnknownReductionAttribute(t *testing.T) {
	err := validateSource(`fn Reduce(values: array<f32>) -> f32 {
let total: f32 = [mystery] sum i in 0u..4u { values[i] };
return total;
}`)
	if err == nil || !strings.Contains(err.Error(), "unknown attribute [mystery]") {
		t.Fatalf("error = %v, want unknown reduction attribute", err)
	}
}

func TestModuleRejectsReductionIndexOutOfScopeAndDeferredMax(t *testing.T) {
	err := validateSource(`fn Reduce(values: array<f32>) -> f32 {
let total: f32 = sum i in 0u..4u { values[i] };
return i;
}`)
	if err == nil || !strings.Contains(err.Error(), "unknown identifier i") {
		t.Fatalf("error = %v, want index scope diagnostic", err)
	}
	err = validateSource(`fn Reduce(values: array<f32>) -> f32 {
return max i in 0u..4u { values[i] };
}`)
	if err == nil || !strings.Contains(err.Error(), "max reduction is reserved but not yet implemented") {
		t.Fatalf("error = %v, want deferred max diagnostic", err)
	}
}

func TestModuleRejectsConfigMissingField(t *testing.T) {
	err := validateSource(`concept TileConfig { TILE_M: u32; TILE_N: u32; }
config Tile16: TileConfig { TILE_M: 16u; }`)
	if err == nil || !strings.Contains(err.Error(), "config field TILE_N missing") {
		t.Fatalf("error = %v, want missing config field", err)
	}
}

func TestModuleRejectsConfigExtraField(t *testing.T) {
	err := validateSource(`concept TileConfig { TILE_M: u32; }
config Tile16: TileConfig { TILE_M: 16u; TILE_N: 16u; }`)
	if err == nil || !strings.Contains(err.Error(), "unknown config field Tile16.TILE_N") {
		t.Fatalf("error = %v, want extra config field", err)
	}
}

func TestModuleRejectsUnknownTemplateField(t *testing.T) {
	err := validateSource(`concept TileConfig { TILE_M: u32; }
template<C: TileConfig>
shader TileCopy {
workgroup Tile: array<f32, 16>;
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
let x: u32 = C.TILE_N;
return;
}
}`)
	if err == nil || !strings.Contains(err.Error(), "unknown template field TILE_N") {
		t.Fatalf("error = %v, want unknown template field", err)
	}
}

func TestModuleRejectsCompileTargetNotTemplate(t *testing.T) {
	err := validateSource(`concept TileConfig { TILE_M: u32; }
config Tile16: TileConfig { TILE_M: 16u; }
shader TileCopy { stage compute [numthreads(1, 1, 1)] fn CS() -> void { return; } }
compile TileCopy<Tile16> as TileCopy16;`)
	if err == nil || !strings.Contains(err.Error(), "must be a template shader") {
		t.Fatalf("error = %v, want compile target template diagnostic", err)
	}
}

func TestModuleRejectsCompileConfigWrongConcept(t *testing.T) {
	err := validateSource(`concept TileConfig { TILE_M: u32; }
concept OtherConfig { TILE_M: u32; }
config Tile16: OtherConfig { TILE_M: 16u; }
template<C: TileConfig>
shader TileCopy { stage compute [numthreads(1, 1, 1)] fn CS() -> void { return; } }
compile TileCopy<Tile16> as TileCopy16;`)
	if err == nil || !strings.Contains(err.Error(), "does not satisfy concept") {
		t.Fatalf("error = %v, want wrong concept diagnostic", err)
	}
}

func TestModuleRejectsCompileAliasCollision(t *testing.T) {
	err := validateSource(`concept TileConfig { TILE_M: u32; }
config Tile16: TileConfig { TILE_M: 16u; }
template<C: TileConfig>
shader TileCopy { stage compute [numthreads(1, 1, 1)] fn CS() -> void { return; } }
record TileCopy16 {}
compile TileCopy<Tile16> as TileCopy16;`)
	if err == nil || !strings.Contains(err.Error(), "collides with top-level declaration") {
		t.Fatalf("error = %v, want alias collision diagnostic", err)
	}
}

func TestModuleValidatesConceptRequirementsAndStaticAsserts(t *testing.T) {
	err := validateSource(`stream IO {
[binding(0)] A: readonly array<f32>;
[binding(1)] C: readwrite array<f32>;
}
concept TileConfig {
THREADS_X: u32;
THREADS_Y: u32;
TILE_SIZE: u32;
require THREADS_X > 0u;
require TILE_SIZE == THREADS_X * THREADS_Y;
}
config Tile16x16: TileConfig {
THREADS_X: 16u;
THREADS_Y: 16u;
TILE_SIZE: 256u;
}
template<C: TileConfig>
shader TileCopy {
resources IO;
static assert C.TILE_SIZE <= 1024u;
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
[unroll]
for i in 0u..1u { return; }
return;
}
}
compile TileCopy<Tile16x16> as TileCopy16x16;`)
	if err != nil {
		t.Fatalf("error = %v, want nil", err)
	}
}

func TestModuleAllowsStructuredConfigDefaultsAndDottedTemplateRefs(t *testing.T) {
	err := validateSource(`stream ComputeThread {
DispatchId: uint3;
GroupId: uint3;
GroupThreadId: uint3;
GroupIndex: u32;
}
stream IO {
[binding(0)] A: readonly array<f32>;
[binding(1)] C: readwrite array<f32>;
}
record Params { Count: u32; }
concept TileConfig {
Threads: {
X: u32;
Y: u32;
};
OutputsPerInvocation: {
M: u32 = 1u;
N: u32 = 1u;
};
Tile: {
M: u32 = Threads.X * OutputsPerInvocation.M;
N: u32 = Threads.Y * OutputsPerInvocation.N;
K: u32;
};
Padding: {
K: u32! = 0u;
};
require Threads.X * Threads.Y <= 1024u;
}
config Tile16: TileConfig {
Threads.X => 16u;
Threads.Y => 16u;
Tile.K => 16u;
}
template<C: TileConfig>
shader TileCopy {
resources IO;
workgroup Tile: array<f32, C.Tile.M * C.Tile.N>;
stage compute [numthreads(C.Threads.X, C.Threads.Y, 1u)] fn CS(thread: ComputeThread, params: Params) -> void {
let tileK: u32 = C.Tile.K + C.Padding.K;
if thread.DispatchId.x < params.Count {
C[thread.DispatchId.x] = A[thread.DispatchId.x];
}
return;
}
}
compile TileCopy<Tile16> as TileCopy16;`)
	if err != nil {
		t.Fatalf("error = %v, want nil", err)
	}
}

func TestModuleRejectsConfigDefaultForwardReference(t *testing.T) {
	err := validateSource(`concept BadConfig {
Tile: { M: u32 = Threads.X; };
Threads: { X: u32; };
}
config Bad: BadConfig { Threads.X => 4u; }`)
	if err == nil || !strings.Contains(err.Error(), "unknown constant field Threads.X") {
		t.Fatalf("error = %v, want forward-reference diagnostic", err)
	}
}

func TestModuleRejectsZeroForPlainU32ConfigFieldAndAllowsU32Bang(t *testing.T) {
	err := validateSource(`concept BadConfig {
Tile: { K: u32 = 0u; };
}
config Bad: BadConfig {}`)
	if err == nil || !strings.Contains(err.Error(), "config field Tile.K is nonzero by default") {
		t.Fatalf("error = %v, want nonzero default diagnostic", err)
	}
	err = validateSource(`concept OkayConfig {
Padding: { K: u32! = 0u; };
}
config Okay: OkayConfig {}`)
	if err != nil {
		t.Fatalf("error = %v, want nil", err)
	}
}

func TestModuleRejectsStructuredConfigDuplicateUnknownAndMixedAssignments(t *testing.T) {
	err := validateSource(`concept TileConfig {
Threads: { X: u32; };
}
config Bad: TileConfig {
Threads.X => 4u;
Threads.X => 8u;
}`)
	if err == nil || !strings.Contains(err.Error(), "duplicate config field Bad.Threads.X") {
		t.Fatalf("error = %v, want duplicate dotted assignment diagnostic", err)
	}
	err = validateSource(`concept TileConfig {
Threads: { X: u32; };
}
config Bad: TileConfig {
Threads.Y => 4u;
}`)
	if err == nil || !strings.Contains(err.Error(), "unknown config field Bad.Threads.Y") {
		t.Fatalf("error = %v, want unknown dotted field diagnostic", err)
	}
	err = validateSource(`concept TileConfig {
Threads: { X: u32; };
Y: u32;
}
config Bad: TileConfig {
Threads.X => 4u;
Y: 2u;
}`)
	if err == nil || !strings.Contains(err.Error(), "must not mix legacy ':' assignments with fat-arrow") {
		t.Fatalf("error = %v, want mixed assignment style diagnostic", err)
	}
}

func TestModuleRejectsFailedConceptRequirement(t *testing.T) {
	err := validateSource(`concept TileConfig {
TILE_SIZE: u32;
require TILE_SIZE > 0u;
}
config Bad: TileConfig {
TILE_SIZE: 0u;
}`)
	if err == nil || !strings.Contains(err.Error(), "failed requirement") {
		t.Fatalf("error = %v, want failed requirement", err)
	}
}

func TestModuleRejectsStaticAssertFailure(t *testing.T) {
	err := validateSource(`concept TileConfig { TILE_SIZE: u32; }
config Tile16: TileConfig { TILE_SIZE: 2048u; }
template<C: TileConfig>
shader TileCopy {
static assert C.TILE_SIZE <= 1024u;
stage compute [numthreads(1, 1, 1)] fn CS() -> void { return; }
}
compile TileCopy<Tile16> as TileCopy16;`)
	if err == nil || !strings.Contains(err.Error(), "failed static assert") {
		t.Fatalf("error = %v, want failed static assert", err)
	}
}

func TestModuleRejectsConflictingLoopAttributes(t *testing.T) {
	err := validateSource(`shader S {
stage compute [numthreads(1, 1, 1)] fn CS() -> void {
[unroll][loop]
for i in 0u..1u { return; }
return;
}
}`)
	if err == nil || !strings.Contains(err.Error(), "both [unroll] and [loop]") {
		t.Fatalf("error = %v, want loop attribute conflict", err)
	}
}

func TestModuleRejectsDuplicateExplicitBindings(t *testing.T) {
	err := validateSource(`shader S {
resources {
[binding(0)] A: readonly array<f32>;
[binding(0)] C: readwrite array<f32>;
}
stage compute [numthreads(1, 1, 1)] fn CS() -> void { return; }
}`)
	if err == nil || !strings.Contains(err.Error(), "duplicate explicit binding 0") {
		t.Fatalf("error = %v, want duplicate binding diagnostic", err)
	}
}

func TestModuleRejectsUnknownAttribute(t *testing.T) {
	err := validateSource(`shader S {
resources {
[mystery] A: readonly array<f32>;
}
stage compute [numthreads(1, 1, 1)] fn CS() -> void { return; }
}`)
	if err == nil || !strings.Contains(err.Error(), "unknown attribute") {
		t.Fatalf("error = %v, want unknown attribute diagnostic", err)
	}
}

func validateSource(text string) error {
	tokens, err := lex.Analyze(source.File{Path: "test.sdslv", Text: text})
	if err != nil {
		return err
	}
	module, err := parse.BuildModule(tokens)
	if err != nil {
		return err
	}
	return Module(module)
}
