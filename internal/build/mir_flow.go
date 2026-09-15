package build

type MIRFlow struct {
	Package    string
	Name       string
	Parameters []MIRField
	TurnInput  *MIRField
	YieldType  string
	Board      []MIRField
	Return     string
	EntryState string
	States     []MIRFlowState
}

type MIRFlowState struct {
	Name       string
	Statements []MIRFlowStmt
}

type MIRFlowStmt interface{ mirFlowStmt() }

type MIRFlowGoto struct{ Target string }

func (MIRFlowGoto) mirFlowStmt() {}

type MIRFlowSuspend struct{}

func (MIRFlowSuspend) mirFlowStmt() {}

type MIRFlowYield struct{ Value MIRFlowExpr }

func (MIRFlowYield) mirFlowStmt() {}

type MIRFlowRemember struct{}

func (MIRFlowRemember) mirFlowStmt() {}

type MIRFlowResume struct{}

func (MIRFlowResume) mirFlowStmt() {}

type MIRFlowFieldAssign struct {
	Target string
	Field  string
	Value  MIRFlowExpr
}

func (MIRFlowFieldAssign) mirFlowStmt() {}

type MIRFlowFieldIndexAssign struct {
	Target  string
	Field   string
	Indices []MIRFlowExpr
	Value   MIRFlowExpr
}

func (MIRFlowFieldIndexAssign) mirFlowStmt() {}

type MIRFlowLetStmt struct {
	Name  string
	Type  string
	Value MIRFlowExpr
}

func (MIRFlowLetStmt) mirFlowStmt() {}

type MIRFlowLocalAssign struct {
	Name  string
	Value MIRFlowExpr
}

func (MIRFlowLocalAssign) mirFlowStmt() {}

type MIRFlowWhile struct {
	Condition MIRFlowExpr
	Body      []MIRFlowStmt
}

func (MIRFlowWhile) mirFlowStmt() {}

type MIRFlowFor struct {
	Name       string
	Start      MIRFlowExpr
	End        MIRFlowExpr
	Step       MIRFlowExpr
	Descending bool
	Body       []MIRFlowStmt
}

func (MIRFlowFor) mirFlowStmt() {}

type MIRFlowFallibleMatch struct {
	Subject    MIRFlowExpr
	ResultType string
	OkName     string
	OkBody     []MIRFlowStmt
	ErrName    string
	ErrBody    []MIRFlowStmt
}

func (MIRFlowFallibleMatch) mirFlowStmt() {}

type MIRFlowReturn struct {
	Value MIRFlowExpr
}

func (MIRFlowReturn) mirFlowStmt() {}

type MIRFlowIf struct {
	Condition MIRFlowExpr
	Then      []MIRFlowStmt
	Else      []MIRFlowStmt
}

func (MIRFlowIf) mirFlowStmt() {}

type MIRFlowWhen struct {
	Cases []MIRFlowWhenCase
	Else  MIRFlowWhenAction
}

func (MIRFlowWhen) mirFlowStmt() {}

type MIRFlowWhenCase struct {
	Condition MIRFlowExpr
	Action    MIRFlowWhenAction
}

type MIRFlowWhenAction interface{ mirFlowWhenAction() }

type MIRFlowWhenGoto struct{ Target string }

func (MIRFlowWhenGoto) mirFlowWhenAction() {}

type MIRFlowWhenSuspend struct{}

func (MIRFlowWhenSuspend) mirFlowWhenAction() {}

type MIRFlowWhenReturn struct {
	Value MIRFlowExpr
}

func (MIRFlowWhenReturn) mirFlowWhenAction() {}

type MIRFlowWhenBlock struct {
	Statements []MIRFlowStmt
}

func (MIRFlowWhenBlock) mirFlowWhenAction() {}

type MIRFlowExpr interface{ mirFlowExpr() }

type MIRFlowUtilityWhenExpr struct {
	SiteID          int
	ControllerBound bool
	ResultType      string
	Hysteresis      MIRFlowExpr
	MinCommit       MIRFlowExpr
	Cases           []MIRFlowUtilityCase
	Else            MIRFlowExpr
}

func (MIRFlowUtilityWhenExpr) mirFlowExpr() {}

type MIRFlowUtilityCase struct {
	Value     MIRFlowExpr
	Condition MIRFlowExpr
	Score     MIRFlowExpr
}

// MIRFlowSharedExpr is an ordinary compiled-expression computation embedded in
// a FLOW activation. Its blocks and locals are the same MIR used by ordinary
// functions; FLOW contributes only binding expressions such as f.board.X and
// the surrounding machine-control statement.
type MIRFlowSharedExpr struct {
	Type     string
	Fallible bool
	Locals   []MIRField
	Blocks   []MIRBlock
}

func (MIRFlowSharedExpr) mirFlowExpr() {}
