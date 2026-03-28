package prometheus

import "fmt"

type Backend string

const (
	BackendCPU        Backend = "cpu"
	BackendPrometheus Backend = "prometheus"
)

type RunStatusKind string

const (
	RunStatusOK       RunStatusKind = "ok"
	RunStatusFallback RunStatusKind = "fallback"
	RunStatusError    RunStatusKind = "error"
)

type ErrorStage string

const (
	StageInit        ErrorStage = "init"
	StageTransferIn  ErrorStage = "transfer_in"
	StageSubmit      ErrorStage = "submit"
	StageTransferOut ErrorStage = "transfer_out"
	StageCleanup     ErrorStage = "cleanup"
	StageUnknown     ErrorStage = "unknown"
)

type RunStatus struct {
	Kind           RunStatusKind
	FallbackReason string
	ErrorStage     ErrorStage
	ErrorCode      int
}

func OkStatus() RunStatus { return RunStatus{Kind: RunStatusOK} }

func FallbackStatus(reason string) RunStatus {
	return RunStatus{Kind: RunStatusFallback, FallbackReason: reason}
}

func ErrorStatus(stage ErrorStage, code int) RunStatus {
	return RunStatus{Kind: RunStatusError, ErrorStage: stage, ErrorCode: code}
}

func (s RunStatus) String() string {
	switch s.Kind {
	case RunStatusOK:
		return "ok"
	case RunStatusFallback:
		return fmt.Sprintf("fallback(%s)", s.FallbackReason)
	case RunStatusError:
		return fmt.Sprintf("error(%s,%d)", s.ErrorStage, s.ErrorCode)
	default:
		return "error(unknown,-1)"
	}
}
