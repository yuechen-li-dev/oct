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
	if !strings.Contains(header, "PROM_ZIMAGE_TURBO_AUDIT_LOCK_ID 0xb3660657c5546e9cull") {
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
	if got := resolvedManifestIdentity([]byte(lf)); got != "7b9ff0f3cb66bce9bff83cc4bb28f78c5cdc6f35a1781ae6a4608052500d9d32" {
		t.Fatalf("accepted manifest identity changed: %s", got)
	}
	if resolvedManifestIdentity([]byte(lf)) != resolvedManifestIdentity([]byte(crlf)) {
		t.Fatal("line endings changed the resolved manifest identity")
	}
	if resolvedManifestIdentity([]byte(lf+" ")) == resolvedManifestIdentity([]byte(lf)) {
		t.Fatal("authored content mutation did not change the resolved manifest identity")
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
		"semantic identity":     strings.Replace(string(lock), "a8faf8923ab4c748", "0000000000000000", 1),
		"production identity":   strings.Replace(string(lock), "0868bd2b1127fa75", "0000000000000000", 1),
		"audit identity":        strings.Replace(string(lock), "de558800cbe07b4a", "0000000000000000", 1),
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
