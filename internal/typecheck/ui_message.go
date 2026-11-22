package typecheck

import (
	"fmt"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/ast"
)

// UI<M> (DESIGN.md §5.1, §7 L1). A UI value's type carries the message type
// its buttons send, in Type.UIMessage:
//
//	""      no message of its own (UI.Text, UI.Spacer, a layout of them).
//	        It goes wherever a UI goes, as Option.None goes wherever an
//	        Option goes.
//	"Msg"   UI<Msg>: it sends Msg values. It never becomes a UI<Other>.
//	"?"     the bare `UI` a function's signature or body is written with:
//	        one message type per function, chosen by each call. A call to
//	        `fn Card(title: String, body: UI) -> UI` with a UI<Msg> body
//	        returns UI<Msg>. Inside Card the bare UI is rigid: it is not
//	        known, so Card cannot put a Msg button into it.
//	"*"     internal: an argument place that takes a UI of any message
//	        (a UI builtin's child, a bare-UI parameter at a call). No value
//	        has this type.
//
// The message of a node is the join of its children's messages: messageless
// joins with anything, and two different messages are an error.
const (
	uiMessageNone = ""
	uiMessagePoly = "?"
	uiMessageAny  = "*"
)

// isUIShape reports a UI type with the given array depth, of any message.
func isUIShape(t Type, depth int) bool {
	return t.Base == BaseTypeUI && t.Name == "" && t.Tuple == nil && !t.IsFunction && !t.IsFlowInstance &&
		!t.IsVector && !t.IsMatrix && t.ArrayDepth == depth && t.IsArray == (depth > 0)
}

// uiOf is a UI type of the given message and array depth.
func uiOf(message string, depth int) Type {
	return withArrayDepth(Type{Base: BaseTypeUI, UIMessage: message}, depth)
}

// isConcreteUIMessage reports a message that names an enum.
func isConcreteUIMessage(message string) bool {
	return message != uiMessageNone && message != uiMessagePoly && message != uiMessageAny
}

// uiMessageLabel spells a message for diagnostics.
func uiMessageLabel(message string) string {
	switch message {
	case uiMessageNone:
		return "UI (no messages)"
	case uiMessagePoly:
		return "UI (this function's message type)"
	default:
		return "UI<" + message + ">"
	}
}

// joinUIMessage is the message of a node holding children of messages a and
// b.
func joinUIMessage(a, b string) (string, error) {
	switch {
	case a == uiMessageNone || a == uiMessageAny:
		return b, nil
	case b == uiMessageNone || b == uiMessageAny || a == b:
		return a, nil
	default:
		return "", fmt.Errorf("%s and %s cannot be in one view: a view sends one message type", uiMessageLabel(a), uiMessageLabel(b))
	}
}

// uiMessageAssignable reports whether a UI of message actual may go where
// a UI of message expected goes.
func uiMessageAssignable(actual, expected string) bool {
	return expected == uiMessageAny || actual == uiMessageNone || actual == expected
}

// mentionsUIMessage reports whether t, or a function type within it, uses
// the message m.
func (c checker) mentionsUIMessage(t Type, m string) bool {
	if t.Base == BaseTypeUI && t.UIMessage == m {
		return true
	}
	if t.IsFunction {
		signature, ok := c.functionTypes[t.FunctionSignature]
		if !ok {
			return false
		}
		return c.signatureMentionsUIMessage(signature, m)
	}
	return false
}

func (c checker) signatureMentionsUIMessage(signature functionSignature, m string) bool {
	if c.mentionsUIMessage(signature.returnType, m) {
		return true
	}
	for _, parameter := range signature.parameters {
		if c.mentionsUIMessage(parameter, m) {
			return true
		}
	}
	return false
}

// isUIPolymorphic reports a signature written with bare UI.
func (c checker) isUIPolymorphic(signature functionSignature) bool {
	return c.signatureMentionsUIMessage(signature, uiMessagePoly)
}

// withUIMessage replaces the message of a UI type (of any array depth).
// Other types are returned unchanged.
func withUIMessage(t Type, message string) Type {
	if t.Base == BaseTypeUI && t.Name == "" {
		t.UIMessage = message
	}
	return t
}

// bareUIAsNone makes a bare `UI` written in a place that has no message
// variable of its own (a record field, an enum payload, a local of a
// function whose signature does not use bare UI) mean a messageless UI.
func bareUIAsNone(t Type) Type {
	if t.Base == BaseTypeUI && t.Name == "" && t.UIMessage == uiMessagePoly {
		t.UIMessage = uiMessageNone
	}
	return t
}

// localUIType is the type a let or var annotation means in ctx.
func localUIType(t Type, ctx functionContext) Type {
	if ctx.uiPolymorphic {
		return t
	}
	return bareUIAsNone(t)
}

// uiMessageJoin accumulates the message of a UI node from its parts.
type uiMessageJoin struct {
	callee  string
	message string
	err     error
}

func newUIMessageJoin(callee string) *uiMessageJoin {
	return &uiMessageJoin{callee: callee}
}

// add joins the message of argument index (0-based) of type t, a UI of any
// array depth.
func (j *uiMessageJoin) add(index int, t Type) {
	if j.err != nil || t.Base != BaseTypeUI {
		return
	}
	joined, err := joinUIMessage(j.message, t.UIMessage)
	if err != nil {
		j.err = fmt.Errorf("function '%s' argument %d: %w", j.callee, index+1, err)
		return
	}
	j.message = joined
}

// addMessage joins a concrete message (a button's).
func (j *uiMessageJoin) addMessage(index int, message string) {
	j.add(index, uiOf(message, 0))
}

func (j *uiMessageJoin) result(depth int) (ExprType, error) {
	if j.err != nil {
		return ExprType{}, j.err
	}
	return ExprType{ValueType: uiOf(j.message, depth)}, nil
}

// checkUIValueArgument checks a builtin argument that is a UI of the given
// array depth, of any message, and joins its message into j.
func (c checker) checkUIValueArgument(scope *scope, j *uiMessageJoin, index int, depth int, argType Type) error {
	if !isUIShape(argType, depth) {
		return fmt.Errorf("function '%s' argument %d expects %s, got %s", j.callee, index+1, "UI"+strings.Repeat("[]", depth), argType)
	}
	j.add(index, argType)
	return j.err
}

// uiMessageOfFunctionResult is the message a function value of the given
// signature produces when called. A bare-UI return with no bare-UI
// parameter cannot have received a message, so it has none.
func (c checker) uiMessageOfFunctionResult(signature functionSignature) string {
	message := signature.returnType.UIMessage
	if message != uiMessagePoly {
		return message
	}
	for _, parameter := range signature.parameters {
		if c.mentionsUIMessage(parameter, uiMessagePoly) {
			return message
		}
	}
	return uiMessageNone
}

// resolveUIType resolves `UI<M>` (with any array suffix): M is an enum, the
// type the view's buttons send.
func (c checker) resolveUIType(typeRef ast.TypeRef) (Type, error) {
	if len(typeRef.TypeArguments) != 1 {
		return Type{}, fmt.Errorf("UI takes one type argument, the message enum, as in UI<Msg>; got %d", len(typeRef.TypeArguments))
	}
	message, err := c.resolveType(typeRef.TypeArguments[0], false)
	if err != nil {
		return Type{}, fmt.Errorf("UI<...> message type: %w", err)
	}
	if !c.isMessageEnum(message) {
		return Type{}, fmt.Errorf("UI<...> takes the app's message enum, as in UI<Msg>; %s is not an enum", message)
	}
	depth := typeRef.ArrayDepth
	if typeRef.IsArray && depth == 0 {
		depth = 1
	}
	return uiOf(message.Name, depth), nil
}

// qualifyImportedUIType names the message of a UI type from package pkgName
// as the checked package names it.
func (c checker) qualifyImportedUIType(pkgName string, t Type) Type {
	if !isConcreteUIMessage(t.UIMessage) || strings.Contains(t.UIMessage, ".") {
		return t
	}
	if _, ok := c.importedPackages[pkgName].enums[t.UIMessage]; ok {
		t.UIMessage = pkgName + "." + t.UIMessage
	}
	return t
}

// uiAnnotationHint tells the author how to make room for a message.
func uiAnnotationHint(actual Type) string {
	if !isConcreteUIMessage(actual.UIMessage) {
		return ""
	}
	return fmt.Sprintf("; a view that sends %s is written UI<%s>, as in `-> UI<%s>` or `var items: UI<%s>[] = []`", actual.UIMessage, actual.UIMessage, actual.UIMessage, actual.UIMessage)
}

// joinArmTypes joins the type of one arm of a switch, match or when into the
// result type so far. Arms must have one type; UI arms of different
// messages join as a view's children do.
func joinArmTypes(result *Type, arm Type) (bool, error) {
	if arm == *result {
		return true, nil
	}
	if !isUIShape(arm, arm.ArrayDepth) || !isUIShape(*result, arm.ArrayDepth) {
		return false, nil
	}
	joined, err := joinUIMessage(result.UIMessage, arm.UIMessage)
	if err != nil {
		return false, fmt.Errorf("arms of different views: %w", err)
	}
	result.UIMessage = joined
	return true, nil
}
