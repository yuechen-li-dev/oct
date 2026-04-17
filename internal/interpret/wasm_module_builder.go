package interpret

// wasmModuleBuilder emits a bounded Wasm module byte stream from explicit
// section payloads.
//
// NOTE(M98d): this builder currently owns section framing + LEB encoding and is
// used by the Machina UI emission path for direct custom-section stamping.
// The remaining convergent slice is replacing the reference runtime/template
// bytes with direct code/data/function section emission.
type wasmModuleBuilder struct {
	bytes []byte
}

func newWasmModuleBuilder() *wasmModuleBuilder {
	b := &wasmModuleBuilder{}
	b.bytes = append(b.bytes, 0x00, 0x61, 0x73, 0x6d) // \0asm
	b.bytes = append(b.bytes, 0x01, 0x00, 0x00, 0x00) // version 1
	return b
}

func newWasmModuleBuilderFromBytes(module []byte) *wasmModuleBuilder {
	out := make([]byte, len(module))
	copy(out, module)
	return &wasmModuleBuilder{bytes: out}
}

func (b *wasmModuleBuilder) appendRawSection(sectionID byte, payload []byte) {
	b.bytes = append(b.bytes, sectionID)
	b.bytes = append(b.bytes, encodeULEB128(uint32(len(payload)))...)
	b.bytes = append(b.bytes, payload...)
}

func (b *wasmModuleBuilder) appendCustomSection(name string, payload []byte) {
	content := make([]byte, 0, len(name)+len(payload)+8)
	content = append(content, encodeULEB128(uint32(len(name)))...)
	content = append(content, []byte(name)...)
	content = append(content, payload...)
	b.appendRawSection(0x00, content)
}

func (b *wasmModuleBuilder) bytesOut() []byte {
	out := make([]byte, len(b.bytes))
	copy(out, b.bytes)
	return out
}

func encodeULEB128(v uint32) []byte {
	if v == 0 {
		return []byte{0}
	}
	out := make([]byte, 0, 5)
	for v > 0 {
		part := byte(v & 0x7f)
		v >>= 7
		if v != 0 {
			part |= 0x80
		}
		out = append(out, part)
	}
	return out
}
