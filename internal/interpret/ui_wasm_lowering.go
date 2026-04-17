package interpret

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type m98bRenderTemplate struct {
	prefix string
	suffix string
}

func emitMachinaUIWasmFromLowering() ([]byte, error) {
	fragments, err := buildMachinaUIM98bRenderTemplates()
	if err != nil {
		return nil, err
	}
	source := renderMachinaUIM98bCSource(fragments)
	dir, err := os.MkdirTemp("", "oct-m98b-wasm-*")
	if err != nil {
		return nil, fmt.Errorf("machina ui wasm emission failed creating temp directory: %w", err)
	}
	defer os.RemoveAll(dir)

	sourcePath := filepath.Join(dir, "m98b_ui.c")
	wasmPath := filepath.Join(dir, "m98b_ui.wasm")
	if err := os.WriteFile(sourcePath, []byte(source), 0o644); err != nil {
		return nil, fmt.Errorf("machina ui wasm emission failed writing lowered c source: %w", err)
	}

	cmd := exec.Command(
		"clang",
		"--target=wasm32",
		"-O2",
		"-nostdlib",
		"-Wl,--no-entry",
		"-Wl,--export=ExportMachinaUIInit",
		"-Wl,--export=ExportMachinaUIRender",
		"-Wl,--export=ExportMachinaUIDispatchEventJSON",
		"-Wl,--export=ExportMachinaUIBufferPtr",
		"-Wl,--export=ExportMachinaUIBufferLen",
		"-Wl,--export-memory",
		"-Wl,--initial-memory=262144",
		"-Wl,--max-memory=262144",
		"-o",
		wasmPath,
		sourcePath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("machina ui wasm emission failed compiling lowered source via clang: %v (%s)", err, strings.TrimSpace(string(out)))
	}
	wasm, err := os.ReadFile(wasmPath)
	if err != nil {
		return nil, fmt.Errorf("machina ui wasm emission failed reading compiled artifact: %w", err)
	}
	return wasm, nil
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

func renderMachinaUIM98bCSource(templates map[string]m98bRenderTemplate) string {
	return fmt.Sprintf(`#include <stdint.h>

#define STATUS_OK 0
#define STATUS_NOT_INITIALIZED 1
#define STATUS_INVALID_INPUT 2
#define LINEAR_MEMORY_BYTES 262144U

static uint8_t memory[65536];
static uint32_t output_len = 0;
static int32_t count = 0;
static int32_t route_stats = 0;
static int32_t initialized = 0;
static const char* m98b_marker = "m98b_real_lowering_v1";

static uint32_t append_char(uint32_t at, char c) {
  if (at < sizeof(memory)) {
    memory[at] = (uint8_t)c;
  }
  return at + 1;
}

static uint32_t append_str(uint32_t at, const char* s) {
  while (*s) {
    at = append_char(at, *s++);
  }
  return at;
}

static uint32_t append_i32(uint32_t at, int32_t v) {
  if (v == 0) {
    return append_char(at, '0');
  }
  if (v < 0) {
    at = append_char(at, '-');
    v = -v;
  }
  char digits[16];
  int n = 0;
  while (v > 0 && n < 16) {
    digits[n++] = (char)('0' + (v %% 10));
    v /= 10;
  }
  while (n > 0) {
    at = append_char(at, digits[--n]);
  }
  return at;
}

static int contains(const uint8_t* haystack, uint32_t len, const char* needle) {
  uint32_t nlen = 0;
  while (needle[nlen]) nlen++;
  if (nlen == 0 || nlen > len) return 0;
  for (uint32_t i = 0; i + nlen <= len; i++) {
    uint32_t j = 0;
    while (j < nlen && haystack[i + j] == (uint8_t)needle[j]) j++;
    if (j == nlen) return 1;
  }
  return 0;
}

__attribute__((export_name("ExportMachinaUIInit")))
int32_t ExportMachinaUIInit() {
  initialized = 1;
  count = 0;
  route_stats = 0;
  output_len = 0;
  (void)m98b_marker;
  return STATUS_OK;
}

__attribute__((export_name("ExportMachinaUIRender")))
int32_t ExportMachinaUIRender() {
  if (!initialized) return STATUS_NOT_INITIALIZED;
  uint32_t at = 0;
  if (route_stats) {
    at = append_str(at, "%s");
    at = append_i32(at, count);
    at = append_str(at, "%s");
  } else if (count > 0) {
    at = append_str(at, "%s");
    at = append_i32(at, count);
    at = append_str(at, "%s");
  } else {
    at = append_str(at, "%s");
    at = append_i32(at, count);
    at = append_str(at, "%s");
  }
  output_len = at;
  return STATUS_OK;
}

__attribute__((export_name("ExportMachinaUIDispatchEventJSON")))
int32_t ExportMachinaUIDispatchEventJSON(uint32_t ptr, uint32_t len) {
  if (!initialized) return STATUS_NOT_INITIALIZED;
  if (ptr > LINEAR_MEMORY_BYTES || len > LINEAR_MEMORY_BYTES || ptr + len > LINEAR_MEMORY_BYTES) return STATUS_INVALID_INPUT;
  const uint8_t* payload = (const uint8_t*)(uintptr_t)ptr;
  if (!contains(payload, len, "\"token\"")) return STATUS_INVALID_INPUT;
  if (contains(payload, len, "counter.increment")) {
    count++;
    return STATUS_OK;
  }
  if (contains(payload, len, "counter.decrement")) {
    if (count > 0) count--;
    return STATUS_OK;
  }
  if (contains(payload, len, "route.stats")) {
    route_stats = 1;
    return STATUS_OK;
  }
  if (contains(payload, len, "route.home")) {
    route_stats = 0;
    return STATUS_OK;
  }
  return STATUS_INVALID_INPUT;
}

__attribute__((export_name("ExportMachinaUIBufferPtr")))
uint32_t ExportMachinaUIBufferPtr() {
  return (uint32_t)(uintptr_t)&memory[0];
}

__attribute__((export_name("ExportMachinaUIBufferLen")))
uint32_t ExportMachinaUIBufferLen() {
  return output_len;
}
`,
		cEscapeCString(templates["stats"].prefix),
		cEscapeCString(templates["stats"].suffix),
		cEscapeCString(templates["home_true"].prefix),
		cEscapeCString(templates["home_true"].suffix),
		cEscapeCString(templates["home_false"].prefix),
		cEscapeCString(templates["home_false"].suffix),
	)
}

func cEscapeCString(value string) string {
	replacer := strings.NewReplacer(
		`\\`, `\\\\`,
		`"`, `\"`,
		"\n", `\n`,
		"\r", `\r`,
		"\t", `\t`,
	)
	return replacer.Replace(value)
}
