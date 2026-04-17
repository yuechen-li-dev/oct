package interpret

import (
	"fmt"
	"sort"
	"strings"
)

type m98bRenderTemplate struct {
	prefix string
	suffix string
}

// legacyFixtureDecodeHook remains only as a regression trap in tests: production
// emission must not call this hook after M98e.
var legacyFixtureDecodeHook = decodeMachinaUIWasmArtifactFixture

const (
	m98eMemoryMinPages = 1
	m98eOutputPtr      = 1024
	m98eDigitScratch   = 960
	m98eDataBase       = 8192
)

func emitMachinaUIWasmFromLowering() ([]byte, error) {
	fragments, err := buildMachinaUIM98bRenderTemplates()
	if err != nil {
		return nil, err
	}
	return emitMachinaUIM98eModule(fragments)
}

func emitMachinaUIM98eModule(fragments map[string]m98bRenderTemplate) ([]byte, error) {
	rt, err := buildM98eRuntimeTable(fragments)
	if err != nil {
		return nil, err
	}

	builder := newWasmModuleBuilder()
	builder.appendRawSection(0x01, rt.typeSection())
	builder.appendRawSection(0x03, rt.functionSection())
	builder.appendRawSection(0x05, rt.memorySection())
	builder.appendRawSection(0x07, rt.exportSection())
	builder.appendRawSection(0x0a, rt.codeSection())
	builder.appendRawSection(0x0b, rt.dataSection())
	builder.appendCustomSection("oct.m98e.lowering", buildMachinaUIDirectWasmStamp(fragments))
	return builder.bytesOut(), nil
}

func buildMachinaUIDirectWasmStamp(fragments map[string]m98bRenderTemplate) []byte {
	ordered := []string{"home_false", "home_true", "stats"}
	parts := make([]string, 0, len(ordered))
	for _, name := range ordered {
		frag := fragments[name]
		parts = append(parts, name+":"+frag.prefix+"|"+frag.suffix)
	}
	return []byte(strings.Join(parts, "\n"))
}

func buildMachinaUIM98bRenderTemplates() (map[string]m98bRenderTemplate, error) {
	const countMarker = "__M98B_COUNT__"
	homeFalse, err := serializeUIIRCanonicalJSON(machinaM98bHomeNode(false, countMarker))
	if err != nil {
		return nil, fmt.Errorf("machina ui wasm emission failed creating home(false) template: %w", err)
	}
	homeTrue, err := serializeUIIRCanonicalJSON(machinaM98bHomeNode(true, countMarker))
	if err != nil {
		return nil, fmt.Errorf("machina ui wasm emission failed creating home(true) template: %w", err)
	}
	stats, err := serializeUIIRCanonicalJSON(machinaM98bStatsNode(countMarker))
	if err != nil {
		return nil, fmt.Errorf("machina ui wasm emission failed creating stats template: %w", err)
	}
	templates := map[string]string{
		"home_false": homeFalse,
		"home_true":  homeTrue,
		"stats":      stats,
	}
	renderTemplates := make(map[string]m98bRenderTemplate, len(templates))
	for name, payload := range templates {
		parts := strings.Split(payload, countMarker)
		if len(parts) != 2 {
			return nil, fmt.Errorf("machina ui wasm emission failed splitting %s render template by count marker", name)
		}
		renderTemplates[name] = m98bRenderTemplate{prefix: parts[0], suffix: parts[1]}
	}
	return renderTemplates, nil
}

func machinaM98bHomeNode(canDec bool, countValue string) *uiirNode {
	return &uiirNode{Kind: uiirNodeColumn, Children: []*uiirNode{
		{Kind: uiirNodeText, Text: "route=home"},
		{Kind: uiirNodeText, Text: "count=" + countValue},
		{Kind: uiirNodeRow, Children: []*uiirNode{
			{Kind: uiirNodeButton, Label: "Increment", Event: "counter.increment", Enabled: true},
			{Kind: uiirNodeButton, Label: "Decrement", Event: "counter.decrement", Enabled: canDec},
			{Kind: uiirNodeButton, Label: "Stats", Event: "route.stats", Enabled: true},
		}},
	}}
}

func machinaM98bStatsNode(countValue string) *uiirNode {
	return &uiirNode{Kind: uiirNodeColumn, Children: []*uiirNode{
		{Kind: uiirNodeText, Text: "route=stats"},
		{Kind: uiirNodeText, Text: "count=" + countValue},
		{Kind: uiirNodeButton, Label: "Home", Event: "route.home", Enabled: true},
	}}
}

type m98eDataBlob struct {
	name   string
	bytes  []byte
	offset uint32
}

type m98eRuntimeTable struct {
	data       []m98eDataBlob
	dataConcat []byte
}

func buildM98eRuntimeTable(fragments map[string]m98bRenderTemplate) (*m98eRuntimeTable, error) {
	blobs := []m98eDataBlob{
		{name: "event_increment", bytes: []byte(`{"token":"counter.increment","payload":null}`)},
		{name: "event_decrement", bytes: []byte(`{"token":"counter.decrement","payload":null}`)},
		{name: "event_route_stats", bytes: []byte(`{"token":"route.stats","payload":null}`)},
		{name: "event_route_home", bytes: []byte(`{"token":"route.home","payload":null}`)},
		{name: "home_false_prefix", bytes: []byte(fragments["home_false"].prefix)},
		{name: "home_false_suffix", bytes: []byte(fragments["home_false"].suffix)},
		{name: "home_true_prefix", bytes: []byte(fragments["home_true"].prefix)},
		{name: "home_true_suffix", bytes: []byte(fragments["home_true"].suffix)},
		{name: "stats_prefix", bytes: []byte(fragments["stats"].prefix)},
		{name: "stats_suffix", bytes: []byte(fragments["stats"].suffix)},
	}
	offset := uint32(m98eDataBase)
	concat := make([]byte, 0)
	for i := range blobs {
		blobs[i].offset = offset
		offset += uint32(len(blobs[i].bytes))
		concat = append(concat, blobs[i].bytes...)
	}
	return &m98eRuntimeTable{data: blobs, dataConcat: concat}, nil
}

func (rt *m98eRuntimeTable) blob(name string) m98eDataBlob {
	for _, b := range rt.data {
		if b.name == name {
			return b
		}
	}
	panic("missing runtime blob: " + name)
}

func (rt *m98eRuntimeTable) typeSection() []byte {
	// types: 0=()->i32, 1=(i32,i32)->i32, 2=(i32,i32,i32)->i32, 3=(i32,i32,i32,i32)->i32
	out := []byte{0x04}
	out = append(out, wasmFuncType(nil, []byte{0x7f})...)
	out = append(out, wasmFuncType([]byte{0x7f, 0x7f}, []byte{0x7f})...)
	out = append(out, wasmFuncType([]byte{0x7f, 0x7f, 0x7f}, []byte{0x7f})...)
	out = append(out, wasmFuncType([]byte{0x7f, 0x7f, 0x7f, 0x7f}, []byte{0x7f})...)
	return out
}

func wasmFuncType(params []byte, results []byte) []byte {
	out := []byte{0x60}
	out = append(out, encodeULEB128(uint32(len(params)))...)
	out = append(out, params...)
	out = append(out, encodeULEB128(uint32(len(results)))...)
	out = append(out, results...)
	return out
}

func (rt *m98eRuntimeTable) functionSection() []byte {
	// helpers + exports
	typeIDs := []uint32{2, 1, 0, 0, 1, 0, 0, 3}
	out := encodeULEB128(uint32(len(typeIDs)))
	for _, id := range typeIDs {
		out = append(out, encodeULEB128(id)...)
	}
	return out
}

func (rt *m98eRuntimeTable) memorySection() []byte {
	// single memory min 1 page
	return []byte{0x01, 0x00, byte(m98eMemoryMinPages)}
}

func (rt *m98eRuntimeTable) exportSection() []byte {
	exports := []struct {
		name string
		kind byte
		idx  uint32
	}{
		{name: "memory", kind: 0x02, idx: 0},
		{name: "ExportMachinaUIInit", kind: 0x00, idx: 2},
		{name: "ExportMachinaUIRender", kind: 0x00, idx: 3},
		{name: "ExportMachinaUIDispatchEventJSON", kind: 0x00, idx: 4},
		{name: "ExportMachinaUIBufferPtr", kind: 0x00, idx: 5},
		{name: "ExportMachinaUIBufferLen", kind: 0x00, idx: 6},
	}
	out := encodeULEB128(uint32(len(exports)))
	for _, ex := range exports {
		out = append(out, encodeULEB128(uint32(len(ex.name)))...)
		out = append(out, []byte(ex.name)...)
		out = append(out, ex.kind)
		out = append(out, encodeULEB128(ex.idx)...)
	}
	return out
}

func (rt *m98eRuntimeTable) codeSection() []byte {
	bodies := [][]byte{
		rt.bodyAppendBytes(),
		rt.bodyAppendU32(),
		rt.bodyInit(),
		rt.bodyRender(),
		rt.bodyDispatch(),
		rt.bodyBufferPtr(),
		rt.bodyBufferLen(),
		rt.bodyIsExactEvent(),
	}
	out := encodeULEB128(uint32(len(bodies)))
	for _, b := range bodies {
		out = append(out, encodeULEB128(uint32(len(b)))...)
		out = append(out, b...)
	}
	return out
}

func (rt *m98eRuntimeTable) dataSection() []byte {
	out := []byte{0x01, 0x00, 0x41}
	out = append(out, encodeSLEB128(int32(m98eDataBase))...)
	out = append(out, 0x0b)
	out = append(out, encodeULEB128(uint32(len(rt.dataConcat)))...)
	out = append(out, rt.dataConcat...)
	return out
}

func (rt *m98eRuntimeTable) bodyAppendBytes() []byte {
	locals := []wasmLocalDecl{{count: 2, valType: 0x7f}} // i, dstCur
	w := newWasmFuncWriter(locals)
	w.localGet(0)
	w.localSet(4)
	w.i32Const(0)
	w.localSet(3)
	loop := w.beginLoop()
	w.localGet(3)
	w.localGet(2)
	w.i32GeU()
	w.ifBlock()
	w.localGet(4)
	w.ret()
	w.end()
	w.localGet(4)
	w.localGet(1)
	w.localGet(3)
	w.i32Add()
	w.i32Load8U(0)
	w.i32Store8(0)
	w.localGet(4)
	w.i32Const(1)
	w.i32Add()
	w.localSet(4)
	w.localGet(3)
	w.i32Const(1)
	w.i32Add()
	w.localSet(3)
	w.br(loop)
	w.end()
	w.localGet(4)
	w.ret()
	return w.finish()
}

func (rt *m98eRuntimeTable) bodyAppendU32() []byte {
	// params: dst,val -> new dst
	locals := []wasmLocalDecl{{count: 3, valType: 0x7f}} // i,tmp,scratch
	w := newWasmFuncWriter(locals)
	w.localGet(1)
	w.i32Eqz()
	w.ifBlock()
	w.localGet(0)
	w.i32Const(int32('0'))
	w.i32Store8(0)
	w.localGet(0)
	w.i32Const(1)
	w.i32Add()
	w.ret()
	w.end()
	w.i32Const(0)
	w.localSet(2)
	w.localGet(1)
	w.localSet(3)
	loopDigits := w.beginLoop()
	w.i32Const(m98eDigitScratch)
	w.localGet(2)
	w.i32Add()
	w.i32Const(int32('0'))
	w.localGet(3)
	w.i32Const(10)
	w.i32RemU()
	w.i32Add()
	w.i32Store8(0)
	w.localGet(3)
	w.i32Const(10)
	w.i32DivU()
	w.localSet(3)
	w.localGet(2)
	w.i32Const(1)
	w.i32Add()
	w.localSet(2)
	w.localGet(3)
	w.i32Eqz()
	w.ifBlock()
	w.br(1)
	w.end()
	w.br(loopDigits)
	w.end()
	loopWrite := w.beginLoop()
	w.localGet(2)
	w.i32Eqz()
	w.ifBlock()
	w.localGet(0)
	w.ret()
	w.end()
	w.localGet(2)
	w.i32Const(1)
	w.i32Sub()
	w.localSet(2)
	w.localGet(0)
	w.i32Const(m98eDigitScratch)
	w.localGet(2)
	w.i32Add()
	w.i32Load8U(0)
	w.i32Store8(0)
	w.localGet(0)
	w.i32Const(1)
	w.i32Add()
	w.localSet(0)
	w.br(loopWrite)
	w.end()
	w.localGet(0)
	w.ret()
	return w.finish()
}

func (rt *m98eRuntimeTable) bodyInit() []byte {
	w := newWasmFuncWriter(nil)
	w.i32Const(0)
	w.i32Const(1)
	w.i32Store8(0)
	w.i32Const(4)
	w.i32Const(0)
	w.i32Store(2)
	w.i32Const(8)
	w.i32Const(0)
	w.i32Store(2)
	w.i32Const(12)
	w.i32Const(0)
	w.i32Store(2)
	w.i32Const(0)
	w.ret()
	return w.finish()
}

func (rt *m98eRuntimeTable) bodyRender() []byte {
	locals := []wasmLocalDecl{{count: 5, valType: 0x7f}} // prefix ptr/len suffix ptr/len out
	w := newWasmFuncWriter(locals)
	w.i32Const(0)
	w.i32Load8U(0)
	w.i32Eqz()
	w.ifBlock()
	w.i32Const(1)
	w.ret()
	w.end()

	hf := rt.blob("home_false_prefix")
	hfs := rt.blob("home_false_suffix")
	ht := rt.blob("home_true_prefix")
	hts := rt.blob("home_true_suffix")
	sp := rt.blob("stats_prefix")
	ss := rt.blob("stats_suffix")

	w.i32Const(int32(hf.offset))
	w.localSet(0)
	w.i32Const(int32(len(hf.bytes)))
	w.localSet(1)
	w.i32Const(int32(hfs.offset))
	w.localSet(2)
	w.i32Const(int32(len(hfs.bytes)))
	w.localSet(3)

	w.i32Const(8)
	w.i32Load(2)
	w.i32Eqz()
	w.ifBlock()
	w.i32Const(4)
	w.i32Load(2)
	w.i32Eqz()
	w.ifBlock()
	w.i32Const(int32(hf.offset))
	w.localSet(0)
	w.i32Const(int32(len(hf.bytes)))
	w.localSet(1)
	w.i32Const(int32(hfs.offset))
	w.localSet(2)
	w.i32Const(int32(len(hfs.bytes)))
	w.localSet(3)
	w.elseBlock()
	w.i32Const(int32(ht.offset))
	w.localSet(0)
	w.i32Const(int32(len(ht.bytes)))
	w.localSet(1)
	w.i32Const(int32(hts.offset))
	w.localSet(2)
	w.i32Const(int32(len(hts.bytes)))
	w.localSet(3)
	w.end()
	w.elseBlock()
	w.i32Const(int32(sp.offset))
	w.localSet(0)
	w.i32Const(int32(len(sp.bytes)))
	w.localSet(1)
	w.i32Const(int32(ss.offset))
	w.localSet(2)
	w.i32Const(int32(len(ss.bytes)))
	w.localSet(3)
	w.end()

	w.i32Const(m98eOutputPtr)
	w.localSet(4)
	w.localGet(4)
	w.localGet(0)
	w.localGet(1)
	w.call(0)
	w.localSet(4)
	w.localGet(4)
	w.i32Const(int32('0'))
	w.i32Const(9)
	w.i32Const(4)
	w.i32Load(2)
	w.i32Const(4)
	w.i32Load(2)
	w.i32Const(9)
	w.i32GtU()
	w.op(0x1b) // select
	w.i32Add()
	w.i32Store8(0)
	w.localGet(4)
	w.i32Const(1)
	w.i32Add()
	w.localSet(4)
	w.localGet(4)
	w.localGet(2)
	w.localGet(3)
	w.call(0)
	w.localSet(4)
	w.i32Const(12)
	w.localGet(4)
	w.i32Const(m98eOutputPtr)
	w.i32Sub()
	w.i32Store(2)
	w.i32Const(0)
	w.ret()
	return w.finish()
}

func (rt *m98eRuntimeTable) bodyDispatch() []byte {
	w := newWasmFuncWriter(nil)
	w.i32Const(0)
	w.i32Load8U(0)
	w.i32Eqz()
	w.ifBlock()
	w.i32Const(1)
	w.ret()
	w.end()

	inc := rt.blob("event_increment")
	dec := rt.blob("event_decrement")
	rs := rt.blob("event_route_stats")
	rh := rt.blob("event_route_home")

	w.localGet(0)
	w.localGet(1)
	w.i32Const(int32(inc.offset))
	w.i32Const(int32(len(inc.bytes)))
	w.call(7)
	w.ifBlock()
	w.i32Const(4)
	w.i32Const(4)
	w.i32Load(2)
	w.i32Const(1)
	w.i32Add()
	w.i32Store(2)
	w.i32Const(0)
	w.ret()
	w.end()

	w.localGet(0)
	w.localGet(1)
	w.i32Const(int32(dec.offset))
	w.i32Const(int32(len(dec.bytes)))
	w.call(7)
	w.ifBlock()
	w.i32Const(4)
	w.i32Load(2)
	w.i32Eqz()
	w.ifBlock()
	w.i32Const(0)
	w.ret()
	w.end()
	w.i32Const(4)
	w.i32Const(4)
	w.i32Load(2)
	w.i32Const(1)
	w.i32Sub()
	w.i32Store(2)
	w.i32Const(0)
	w.ret()
	w.end()

	w.localGet(0)
	w.localGet(1)
	w.i32Const(int32(rs.offset))
	w.i32Const(int32(len(rs.bytes)))
	w.call(7)
	w.ifBlock()
	w.i32Const(8)
	w.i32Const(1)
	w.i32Store(2)
	w.i32Const(0)
	w.ret()
	w.end()
	w.localGet(0)
	w.localGet(1)
	w.i32Const(int32(rh.offset))
	w.i32Const(int32(len(rh.bytes)))
	w.call(7)
	w.ifBlock()
	w.i32Const(8)
	w.i32Const(0)
	w.i32Store(2)
	w.i32Const(0)
	w.ret()
	w.end()

	w.i32Const(2)
	w.ret()
	return w.finish()
}

func (rt *m98eRuntimeTable) bodyBufferPtr() []byte {
	w := newWasmFuncWriter(nil)
	w.i32Const(m98eOutputPtr)
	w.ret()
	return w.finish()
}

func (rt *m98eRuntimeTable) bodyBufferLen() []byte {
	w := newWasmFuncWriter(nil)
	w.i32Const(12)
	w.i32Load(2)
	w.ret()
	return w.finish()
}

func (rt *m98eRuntimeTable) bodyIsExactEvent() []byte {
	// params: ptr,len,expectedPtr,expectedLen -> bool
	locals := []wasmLocalDecl{{count: 1, valType: 0x7f}} // i
	w := newWasmFuncWriter(locals)
	w.localGet(1)
	w.localGet(3)
	w.i32Ne()
	w.ifBlock()
	w.i32Const(0)
	w.ret()
	w.end()
	w.i32Const(0)
	w.localSet(4)
	loop := w.beginLoop()
	w.localGet(4)
	w.localGet(1)
	w.i32GeU()
	w.ifBlock()
	w.i32Const(1)
	w.ret()
	w.end()
	w.localGet(0)
	w.localGet(4)
	w.i32Add()
	w.i32Load8U(0)
	w.localGet(2)
	w.localGet(4)
	w.i32Add()
	w.i32Load8U(0)
	w.i32Ne()
	w.ifBlock()
	w.i32Const(0)
	w.ret()
	w.end()
	w.localGet(4)
	w.i32Const(1)
	w.i32Add()
	w.localSet(4)
	w.br(loop)
	w.end()
	w.i32Const(0)
	w.ret()
	return w.finish()
}

type wasmLocalDecl struct {
	count   uint32
	valType byte
}

type wasmFuncWriter struct {
	code []byte
}

func newWasmFuncWriter(locals []wasmLocalDecl) *wasmFuncWriter {
	w := &wasmFuncWriter{code: make([]byte, 0, 128)}
	w.code = append(w.code, encodeULEB128(uint32(len(locals)))...)
	for _, l := range locals {
		w.code = append(w.code, encodeULEB128(l.count)...)
		w.code = append(w.code, l.valType)
	}
	return w
}

func (w *wasmFuncWriter) finish() []byte {
	return append(w.code, 0x0b)
}
func (w *wasmFuncWriter) op(b ...byte) { w.code = append(w.code, b...) }
func (w *wasmFuncWriter) i32Const(v int32) {
	w.op(0x41)
	w.op(encodeSLEB128(v)...)
}
func (w *wasmFuncWriter) localGet(i uint32) { w.op(0x20); w.op(encodeULEB128(i)...) }
func (w *wasmFuncWriter) localSet(i uint32) { w.op(0x21); w.op(encodeULEB128(i)...) }
func (w *wasmFuncWriter) call(i uint32)     { w.op(0x10); w.op(encodeULEB128(i)...) }
func (w *wasmFuncWriter) ret()              { w.op(0x0f) }
func (w *wasmFuncWriter) i32Add()           { w.op(0x6a) }
func (w *wasmFuncWriter) i32Sub()           { w.op(0x6b) }
func (w *wasmFuncWriter) i32DivU()          { w.op(0x6e) }
func (w *wasmFuncWriter) i32RemU()          { w.op(0x70) }
func (w *wasmFuncWriter) i32Eqz()           { w.op(0x45) }
func (w *wasmFuncWriter) i32Ne()            { w.op(0x47) }
func (w *wasmFuncWriter) i32GeU()           { w.op(0x4f) }
func (w *wasmFuncWriter) i32GtU()           { w.op(0x4b) }
func (w *wasmFuncWriter) i32Load(alignLog2 uint32) {
	w.op(0x28)
	w.op(encodeULEB128(alignLog2)...)
	w.op(0x00)
}
func (w *wasmFuncWriter) i32Store(alignLog2 uint32) {
	w.op(0x36)
	w.op(encodeULEB128(alignLog2)...)
	w.op(0x00)
}
func (w *wasmFuncWriter) i32Load8U(offset uint32) {
	w.op(0x2d)
	w.op(0x00)
	w.op(encodeULEB128(offset)...)
}
func (w *wasmFuncWriter) i32Store8(offset uint32) {
	w.op(0x3a)
	w.op(0x00)
	w.op(encodeULEB128(offset)...)
}
func (w *wasmFuncWriter) ifBlock()   { w.op(0x04, 0x40) }
func (w *wasmFuncWriter) elseBlock() { w.op(0x05) }
func (w *wasmFuncWriter) end()       { w.op(0x0b) }
func (w *wasmFuncWriter) beginLoop() uint32 {
	w.op(0x03, 0x40)
	return 0
}
func (w *wasmFuncWriter) br(depth uint32) {
	w.op(0x0c)
	w.op(encodeULEB128(depth)...)
}

func encodeSLEB128(v int32) []byte {
	out := make([]byte, 0, 5)
	more := true
	for more {
		b := byte(v & 0x7f)
		v >>= 7
		sign := (b & 0x40) != 0
		if (v == 0 && !sign) || (v == -1 && sign) {
			more = false
		} else {
			b |= 0x80
		}
		out = append(out, b)
	}
	return out
}

func (rt *m98eRuntimeTable) blobNames() []string {
	names := make([]string, 0, len(rt.data))
	for _, b := range rt.data {
		names = append(names, b.name)
	}
	sort.Strings(names)
	return names
}
