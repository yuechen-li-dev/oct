package makecmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/ast"
	"github.com/yuechen-li-dev/oct/internal/builtin"
	"github.com/yuechen-li-dev/oct/internal/judgment"
)

type makePurityEvidenceKind string

const (
	makePurityPureData             makePurityEvidenceKind = "PureData"
	makePurityHostAuthority        makePurityEvidenceKind = "HostAuthority"
	makePurityObservableEffect     makePurityEvidenceKind = "ObservableEffect"
	makePurityUnknownCall          makePurityEvidenceKind = "UnknownCall"
	makePurityDeterministicFailure makePurityEvidenceKind = "DeterministicFailure"
	makePurityControlFlow          makePurityEvidenceKind = "ControlFlow"
)

type makePuritySeverity string

const (
	makePurityAllow   makePuritySeverity = "Allow"
	makePurityInfo    makePuritySeverity = "Info"
	makePurityWarning makePuritySeverity = "Warning"
	makePurityError   makePuritySeverity = "Error"
)

type makePurityEvidence struct {
	Kind     makePurityEvidenceKind
	Severity makePuritySeverity
	Subject  string
	Message  string
	Order    int
}

type makePurityDecision struct {
	Primary makePurityEvidence
	Trace   judgment.Result
	OK      bool
}

func writePureDiagnostics(out interface{ Write([]byte) (int, error) }, functions []ast.FunctionDecl) {
	fmt.Fprintln(out, "Pure diagnostics:")
	pureFunctions := []ast.FunctionDecl{}
	for _, fn := range functions {
		if fn.IsMakePure {
			pureFunctions = append(pureFunctions, fn)
		}
	}
	if len(pureFunctions) == 0 {
		fmt.Fprintln(out, "  no [Pure] functions found")
		return
	}
	pureNames := map[string]bool{}
	localNames := map[string]bool{}
	for _, fn := range functions {
		localNames[fn.Name] = true
		if fn.IsMakePure {
			pureNames[fn.Name] = true
		}
	}
	for _, fn := range pureFunctions {
		decision := decideMakePurity(fn, localNames, pureNames)
		if decision.OK {
			fmt.Fprintf(out, "  %s: ok\n", fn.Name)
			continue
		}
		sev := strings.ToLower(string(decision.Primary.Severity))
		fmt.Fprintf(out, "  %s: %s: %s\n", fn.Name, sev, decision.Primary.Message)
	}
}

func decideMakePurity(fn ast.FunctionDecl, localNames, pureNames map[string]bool) makePurityDecision {
	evidence := collectMakePurityEvidence(fn, localNames, pureNames)
	concerning := []makePurityEvidence{}
	for _, ev := range evidence {
		if ev.Severity == makePurityWarning || ev.Severity == makePurityError {
			concerning = append(concerning, ev)
		}
	}
	if len(concerning) == 0 {
		return makePurityDecision{OK: true}
	}
	candidates := make([]judgment.Candidate, len(concerning))
	byName := map[string]makePurityEvidence{}
	for i, ev := range concerning {
		name := fmt.Sprintf("%03d:%s:%s", i, ev.Kind, ev.Subject)
		candidates[i] = judgment.Candidate{Name: name, Eligible: true, Priority: -ev.Order}
		byName[name] = ev
	}
	result, err := (judgment.Judgment{
		Name:       "make-purity-primary-diagnostic",
		Candidates: candidates,
		Considerations: []judgment.Consideration{{Name: "severity", Weight: 1, Score: func(c judgment.Candidate) float64 {
			return makePurityUtility(byName[c.Name])
		}}},
	}).Decide()
	if err != nil {
		return makePurityDecision{Primary: concerning[0], OK: false}
	}
	return makePurityDecision{Primary: byName[result.Winner], Trace: result, OK: false}
}

func collectMakePurityEvidence(fn ast.FunctionDecl, localNames, pureNames map[string]bool) []makePurityEvidence {
	evidence := []makePurityEvidence{{Kind: makePurityPureData, Severity: makePurityAllow, Subject: fn.Name, Message: "data construction and literals are allowed", Order: 0}}
	order := 1
	seen := map[string]bool{}
	walkBlock(fn.Body, func(call ast.CallExpr) {
		name, ok := flattenDoctorCallName(call.Callee)
		if !ok {
			return
		}
		key := name
		if seen[key] {
			return
		}
		seen[key] = true
		ev := makePurityEvidence{Subject: name, Order: order}
		order++
		switch {
		case name == "error":
			ev.Kind, ev.Severity, ev.Message = makePurityDeterministicFailure, makePurityAllow, "error(...) is deterministic failure data and is allowed"
		case name == "Print":
			ev.Kind, ev.Severity, ev.Message = makePurityObservableEffect, makePurityWarning, "calls Print, an observable output operation inside a [Pure] function"
		case strings.HasPrefix(name, "Make."):
			primitive := strings.TrimPrefix(name, "Make.")
			if makeHostPrimitives[primitive] {
				ev.Kind, ev.Severity, ev.Message = makePurityHostAuthority, makePurityError, fmt.Sprintf("calls %s, which requires host authority", name)
			} else {
				ev.Kind, ev.Severity, ev.Message = makePurityPureData, makePurityAllow, fmt.Sprintf("%s data construction is allowed", name)
			}
		case localNames[name]:
			if pureNames[name] {
				ev.Kind, ev.Severity, ev.Message = makePurityPureData, makePurityAllow, fmt.Sprintf("calls [Pure] helper %s", name)
			} else {
				ev.Kind, ev.Severity, ev.Message = makePurityUnknownCall, makePurityWarning, fmt.Sprintf("calls helper %s without [Pure]; transitive purity is not checked in this release", name)
			}
		case strings.Contains(name, "."):
			ev.Kind, ev.Severity, ev.Message = makePurityUnknownCall, makePurityWarning, fmt.Sprintf("calls %s whose purity is not known; transitive purity is not checked in this release", name)
		default:
			if builtin.IsName(name) {
				ev.Kind, ev.Severity, ev.Message = makePurityPureData, makePurityAllow, fmt.Sprintf("builtin %s is allowed", name)
			} else {
				ev.Kind, ev.Severity, ev.Message = makePurityUnknownCall, makePurityWarning, fmt.Sprintf("calls helper %s without [Pure]; transitive purity is not checked in this release", name)
			}
		}
		evidence = append(evidence, ev)
	})
	return evidence
}

func makePurityUtility(ev makePurityEvidence) float64 {
	switch ev.Kind {
	case makePurityHostAuthority:
		return 100
	case makePurityObservableEffect:
		return 60
	case makePurityUnknownCall:
		return 40
	default:
		return 0
	}
}

func flattenDoctorCallName(expr ast.Expr) (string, bool) {
	switch e := expr.(type) {
	case ast.IdentifierExpr:
		return e.Name, true
	case ast.FieldAccessExpr:
		left, ok := flattenDoctorCallName(e.Target)
		if !ok || e.Field == "" {
			return "", false
		}
		return left + "." + e.Field, true
	default:
		return "", false
	}
}

func sortedPurityEvidenceMessages(evidence []makePurityEvidence) []string {
	messages := []string{}
	for _, ev := range evidence {
		messages = append(messages, ev.Message)
	}
	sort.Strings(messages)
	return messages
}
