package token

type Kind string

const (
	EOF Kind = "EOF"

	Identifier       Kind = "Identifier"
	IntLiteral       Kind = "IntLiteral"
	FloatLiteral     Kind = "FloatLiteral"
	StringLiteral    Kind = "StringLiteral"
	RawForeignSource Kind = "RawForeignSource"

	KeywordNamespace Kind = "KeywordNamespace"
	KeywordUse       Kind = "KeywordUse"
	KeywordType      Kind = "KeywordType"
	KeywordRecord    Kind = "KeywordRecord"
	KeywordBoard     Kind = "KeywordBoard"
	KeywordEnum      Kind = "KeywordEnum"
	KeywordShader    Kind = "KeywordShader"
	KeywordResources Kind = "KeywordResources"
	KeywordWorkgroup Kind = "KeywordWorkgroup"
	KeywordReadonly  Kind = "KeywordReadonly"
	KeywordReadwrite Kind = "KeywordReadwrite"
	KeywordStage     Kind = "KeywordStage"
	KeywordCompute   Kind = "KeywordCompute"
	KeywordFn        Kind = "KeywordFn"
	KeywordLet       Kind = "KeywordLet"
	KeywordReturn    Kind = "KeywordReturn"
	KeywordRead      Kind = "KeywordRead"
	KeywordWrite     Kind = "KeywordWrite"
	KeywordIf        Kind = "KeywordIf"
	KeywordElse      Kind = "KeywordElse"
	KeywordFor       Kind = "KeywordFor"
	KeywordIn        Kind = "KeywordIn"
	KeywordStep      Kind = "KeywordStep"
	KeywordSum       Kind = "KeywordSum"
	KeywordProduct   Kind = "KeywordProduct"
	KeywordMax       Kind = "KeywordMax"
	KeywordMin       Kind = "KeywordMin"
	KeywordTrue      Kind = "KeywordTrue"
	KeywordFalse     Kind = "KeywordFalse"
	KeywordWhen      Kind = "KeywordWhen"
	KeywordUtility   Kind = "KeywordUtility"
	KeywordMatch     Kind = "KeywordMatch"
	KeywordCase      Kind = "KeywordCase"
	KeywordScore     Kind = "KeywordScore"
	KeywordWith      Kind = "KeywordWith"
	KeywordDerive    Kind = "KeywordDerive"
	KeywordInterface Kind = "KeywordInterface"
	KeywordStream    Kind = "KeywordStream"
	KeywordFlow      Kind = "KeywordFlow"
	KeywordCompile   Kind = "KeywordCompile"
	KeywordConcept   Kind = "KeywordConcept"
	KeywordConfig    Kind = "KeywordConfig"
	KeywordTemplate  Kind = "KeywordTemplate"
	KeywordRequire   Kind = "KeywordRequire"
	KeywordStatic    Kind = "KeywordStatic"
	KeywordAssert    Kind = "KeywordAssert"
	KeywordComptime  Kind = "KeywordComptime"
	KeywordVertex    Kind = "KeywordVertex"
	KeywordPixel     Kind = "KeywordPixel"
	KeywordAnd       Kind = "KeywordAnd"
	KeywordOr        Kind = "KeywordOr"
	KeywordNot       Kind = "KeywordNot"
	KeywordHLSL      Kind = "KeywordHLSL"

	LeftParen    Kind = "LeftParen"
	RightParen   Kind = "RightParen"
	LeftBrace    Kind = "LeftBrace"
	RightBrace   Kind = "RightBrace"
	LeftBracket  Kind = "LeftBracket"
	RightBracket Kind = "RightBracket"
	LeftAngle    Kind = "LeftAngle"
	RightAngle   Kind = "RightAngle"
	Comma        Kind = "Comma"
	Dot          Kind = "Dot"
	DotDot       Kind = "DotDot"
	Colon        Kind = "Colon"
	Semicolon    Kind = "Semicolon"
	Assign       Kind = "Assign"
	Arrow        Kind = "Arrow"
	Plus         Kind = "Plus"
	Minus        Kind = "Minus"
	Star         Kind = "Star"
	Slash        Kind = "Slash"
	Percent      Kind = "Percent"
	AndAnd       Kind = "AndAnd"
	OrOr         Kind = "OrOr"
	Bang         Kind = "Bang"
	BangEqual    Kind = "BangEqual"
	EqualEqual   Kind = "EqualEqual"
	LeftEqual    Kind = "LeftEqual"
	RightEqual   Kind = "RightEqual"
)

type Token struct {
	Kind   Kind
	Lexeme string
	Line   int
	Column int
}

func LookupKeyword(lexeme string) Kind {
	switch lexeme {
	case "namespace":
		return KeywordNamespace
	case "use":
		return KeywordUse
	case "type":
		return KeywordType
	case "record":
		return KeywordRecord
	case "board":
		return KeywordBoard
	case "enum":
		return KeywordEnum
	case "shader":
		return KeywordShader
	case "resources":
		return KeywordResources
	case "workgroup":
		return KeywordWorkgroup
	case "readonly":
		return KeywordReadonly
	case "readwrite":
		return KeywordReadwrite
	case "stage":
		return KeywordStage
	case "compute":
		return KeywordCompute
	case "fn":
		return KeywordFn
	case "let":
		return KeywordLet
	case "return":
		return KeywordReturn
	case "read":
		return KeywordRead
	case "write":
		return KeywordWrite
	case "if":
		return KeywordIf
	case "else":
		return KeywordElse
	case "for":
		return KeywordFor
	case "in":
		return KeywordIn
	case "step":
		return KeywordStep
	case "sum":
		return KeywordSum
	case "product":
		return KeywordProduct
	case "max":
		return KeywordMax
	case "min":
		return KeywordMin
	case "true":
		return KeywordTrue
	case "false":
		return KeywordFalse
	case "when":
		return KeywordWhen
	case "utility":
		return KeywordUtility
	case "match":
		return KeywordMatch
	case "case":
		return KeywordCase
	case "score":
		return KeywordScore
	case "with":
		return KeywordWith
	case "derive":
		return KeywordDerive
	case "interface":
		return KeywordInterface
	case "stream":
		return KeywordStream
	case "flow":
		return KeywordFlow
	case "compile":
		return KeywordCompile
	case "concept":
		return KeywordConcept
	case "config":
		return KeywordConfig
	case "template":
		return KeywordTemplate
	case "require":
		return KeywordRequire
	case "static":
		return KeywordStatic
	case "assert":
		return KeywordAssert
	case "comptime":
		return KeywordComptime
	case "vertex":
		return KeywordVertex
	case "pixel":
		return KeywordPixel
	case "and":
		return KeywordAnd
	case "or":
		return KeywordOr
	case "not":
		return KeywordNot
	case "HLSL":
		return KeywordHLSL
	default:
		return Identifier
	}
}
