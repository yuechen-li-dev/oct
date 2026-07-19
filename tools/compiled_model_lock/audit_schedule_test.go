package main

import (
	"os"
	"strings"
	"testing"
)

func TestAuditScheduleIsLockDerivedBoundedAndStable(t *testing.T) {
	lock, err := os.ReadFile("../../internal/prometheus/models/zimage-turbo/lock-tagon.octagon")
	if err != nil {
		t.Fatal(err)
	}
	header, layout, err := auditScheduleProjection(lock)
	if err != nil {
		t.Fatalf("audit schedule: %v", err)
	}
	if !strings.Contains(header, "PROM_ZIMAGE_TURBO_AUDIT_LOCK_ID 0x71ef202b4e34b562ull") {
		t.Fatal("schedule does not preserve the accepted lock identity")
	}
	if !strings.Contains(header, "PROM_ZIMAGE_TURBO_AUDIT_STAGE_COUNT 29u") ||
		!strings.Contains(header, "attention_modulated") || !strings.Contains(header, "PROM_ZIMAGE_AUDIT_SOURCE_W3_DECLARED_VIEWS") {
		t.Fatal("schedule is missing required persistent-stage facts")
	}
	if !strings.Contains(layout, "\"ceiling_bytes\": 47186176") ||
		!strings.Contains(layout, "\"required_bytes\": 189696") ||
		!strings.Contains(layout, "\"slack_bytes\": 46996480") ||
		!strings.Contains(layout, "\"audit_destination_offset\": 189440") {
		t.Fatal("arena proof drifted from the fixed bounded contract")
	}
	secondHeader, secondLayout, err := auditScheduleProjection(lock)
	if err != nil || header != secondHeader || layout != secondLayout {
		t.Fatal("schedule generation is not deterministic")
	}
}

func TestManifestIdentityIsLineEndingStableButContentSensitive(t *testing.T) {
	manifest, err := os.ReadFile("../../internal/prometheus/models/zimage-turbo/manifest.oct")
	if err != nil {
		t.Fatal(err)
	}
	lf := strings.ReplaceAll(string(manifest), "\r\n", "\n")
	crlf := strings.ReplaceAll(lf, "\n", "\r\n")
	if got := resolvedManifestIdentity([]byte(lf)); got != "a5693690f1242462f03843da7d48ba0b198c64c8294cda528e6e7062bd495f92" {
		t.Fatalf("accepted manifest identity changed: %s", got)
	}
	if resolvedManifestIdentity([]byte(lf)) != resolvedManifestIdentity([]byte(crlf)) {
		t.Fatal("line endings changed the resolved manifest identity")
	}
	if resolvedManifestIdentity([]byte(lf+" ")) == resolvedManifestIdentity([]byte(lf)) {
		t.Fatal("authored content mutation did not change the resolved manifest identity")
	}
}

func TestNativeProjectionFreezesAllThirtyMainTransformerParameterSets(t *testing.T) {
	manifest, err := os.ReadFile("../../internal/prometheus/models/zimage-turbo/manifest.oct")
	if err != nil {
		t.Fatal(err)
	}
	projection, err := nativeProjection([]byte(render(resolvedManifestIdentity(manifest))))
	if err != nil {
		t.Fatal(err)
	}
	start := strings.Index(projection, "static const PrometheusMainTransformerResolvedDescriptor")
	if start < 0 {
		t.Fatal("native projection is missing the MainTransformer descriptor table")
	}
	end := strings.Index(projection[start:], "};")
	if end < 0 || strings.Count(projection[start:start+end], "PROM_MAIN_TRANSFORMER_FAMILY_Z_IMAGE_TURBO") != 30 {
		t.Fatal("native projection did not emit exactly thirty MainTransformer descriptors")
	}
	for index, aggregate := range mainTransformerAggregates {
		if !strings.Contains(projection, aggregate[:16]) ||
			!strings.Contains(projection, "model_local_block_id >= 30u") {
			t.Fatalf("native projection is missing closed MainTransformer%d authority", index)
		}
	}
}

func TestAuditScheduleRejectsAlteredProfileAndBudget(t *testing.T) {
	lock, err := os.ReadFile("../../internal/prometheus/models/zimage-turbo/lock-tagon.octagon")
	if err != nil {
		t.Fatal(err)
	}
	for name, altered := range map[string]string{
		"schema":                strings.Replace(string(lock), "oct.sdslv.compiled-model-lock-tagon.v1", "wrong", 1),
		"profile":               strings.Replace(string(lock), "NoiseRefinerPersistentProjectionSummary.v1", "foreign", 1),
		"budget":                strings.Replace(string(lock), "AuditBudgetBytes: 47186176", "AuditBudgetBytes: 1", 1),
		"policy":                strings.Replace(string(lock), "no repeated prefix replay", "runtime mutation", 1),
		"semantic identity":     strings.Replace(string(lock), "ed6a7b765d0d7ece", "0000000000000000", 1),
		"production identity":   strings.Replace(string(lock), "6a883d7797b0ebe3", "0000000000000000", 1),
		"audit identity":        strings.Replace(string(lock), "df3a1340b6999dae", "0000000000000000", 1),
		"foreign complete lock": string(lock) + " ",
	} {
		if _, _, err := auditScheduleProjection([]byte(altered)); err == nil {
			t.Fatalf("%s lock mutation unexpectedly generated a schedule", name)
		}
	}
}

func TestAuditScheduleRejectsMalformedResolvedStages(t *testing.T) {
	lock, err := os.ReadFile("../../internal/prometheus/models/zimage-turbo/lock-tagon.octagon")
	if err != nil {
		t.Fatal(err)
	}
	mutate := func(change func([]auditStageSpec), ceiling uint64) error {
		stages := resolvedAuditStages()
		change(stages)
		_, _, projectionErr := auditScheduleProjectionForStages(lock, stages, ceiling)
		return projectionErr
	}
	cases := map[string]func([]auditStageSpec){
		"duplicate stage ID":     func(stages []auditStageSpec) { stages[1].ID = stages[0].ID },
		"invalid projection key": func(stages []auditStageSpec) { stages[10].Keys[0] = stages[10].Count },
		"out of bounds source":   func(stages []auditStageSpec) { stages[10].Base = 2 },
		"capture after lifetime": func(stages []auditStageSpec) { stages[19].Lifetime = "before_qkv" },
		"unknown source":         func(stages []auditStageSpec) { stages[10].Source = "Foreign" },
		"unknown layout":         func(stages []auditStageSpec) { stages[10].Layout = "Strided" },
	}
	for name, change := range cases {
		if err := mutate(change, auditCeilingBytes); err == nil {
			t.Fatalf("%s unexpectedly generated a native schedule", name)
		}
	}
	if err := mutate(func([]auditStageSpec) {}, 1024); err == nil {
		t.Fatal("audit ceiling overflow unexpectedly generated a native schedule")
	}
}
