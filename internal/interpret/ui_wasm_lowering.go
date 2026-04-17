package interpret

import (
	"fmt"
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
	module, err := decodeMachinaUIWasmArtifactFixture()
	if err != nil {
		return nil, fmt.Errorf("machina ui wasm emission failed decoding direct wasm template: %w", err)
	}
	stamp := buildMachinaUIDirectWasmStamp(fragments)
	builder := newWasmModuleBuilderFromBytes(module)
	builder.appendCustomSection("oct.m98d.lowering", stamp)
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
