package token

type Kind string

const (
	EOF Kind = "EOF"

	Identifier    Kind = "Identifier"
	IntLiteral    Kind = "IntLiteral"
	FloatLiteral  Kind = "FloatLiteral"
	StringLiteral Kind = "StringLiteral"

	KeywordNamespace Kind = "KeywordNamespace"
	KeywordUse       Kind = "KeywordUse"
	KeywordType      Kind = "KeywordType"
	KeywordRecord    Kind = "KeywordRecord"
	KeywordEnum      Kind = "KeywordEnum"
	KeywordShader    Kind = "KeywordShader"
	KeywordResources Kind = "KeywordResources"
	KeywordReadonly  Kind = "KeywordReadonly"
	KeywordReadwrite Kind = "KeywordReadwrite"
	KeywordStage     Kind = "KeywordStage"
	KeywordCompute   Kind = "KeywordCompute"
	KeywordFn        Kind = "KeywordFn"
	KeywordLet       Kind = "KeywordLet"
	KeywordReturn    Kind = "KeywordReturn"
	KeywordIf        Kind = "KeywordIf"
	KeywordElse      Kind = "KeywordElse"
	KeywordFor       Kind = "KeywordFor"
	KeywordIn        Kind = "KeywordIn"
	KeywordStep      Kind = "KeywordStep"
	KeywordTrue      Kind = "KeywordTrue"
	KeywordFalse     Kind = "KeywordFalse"
	KeywordWhen      Kind = "KeywordWhen"
	KeywordUtility   Kind = "KeywordUtility"
	KeywordCase      Kind = "KeywordCase"
	KeywordScore     Kind = "KeywordScore"
	KeywordWith      Kind = "KeywordWith"
	KeywordInterface Kind = "KeywordInterface"
	KeywordStream    Kind = "KeywordStream"
	KeywordFlow      Kind = "KeywordFlow"
	KeywordCompile   Kind = "KeywordCompile"
	KeywordVertex    Kind = "KeywordVertex"
	KeywordPixel     Kind = "KeywordPixel"

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
	case "enum":
		return KeywordEnum
	case "shader":
		return KeywordShader
	case "resources":
		return KeywordResources
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
	case "true":
		return KeywordTrue
	case "false":
		return KeywordFalse
	case "when":
		return KeywordWhen
	case "utility":
		return KeywordUtility
	case "case":
		return KeywordCase
	case "score":
		return KeywordScore
	case "with":
		return KeywordWith
	case "interface":
		return KeywordInterface
	case "stream":
		return KeywordStream
	case "flow":
		return KeywordFlow
	case "compile":
		return KeywordCompile
	case "vertex":
		return KeywordVertex
	case "pixel":
		return KeywordPixel
	default:
		return Identifier
	}
}
