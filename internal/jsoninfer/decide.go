package jsoninfer

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/yuechen-li-dev/oct/internal/judgment"
	"github.com/yuechen-li-dev/oct/internal/octjson"
)

// The two choices of this package that have several signals and no single
// one that decides. Each is an internal/judgment decision: bounded
// candidates, a reason for each that cannot be, named and weighted
// considerations, and a trace that `--explain` prints.

// The candidates, by the names a trace gives them.
const (
	asRecord     = "record"
	asKeyedTable = "keyed table"
	asTable      = "table"
	asTagged     = "tagged array"
)

// Decision is one choice made while inferring, and how it was made.
type Decision struct {
	// Path is the place of the value the choice is about.
	Path string
	// Trace is the judgment: every candidate, why it could not be or what
	// each consideration gave it, and the winner.
	Trace judgment.Result
}

// only is the score of a consideration that counts for one candidate.
func only(name string, raw float64) func(judgment.Candidate) float64 {
	return func(candidate judgment.Candidate) float64 {
		if candidate.Name == name {
			return raw
		}
		return 0
	}
}

// objectChoice is what decideObject found.
type objectChoice struct {
	trace judgment.Result
	// value is what the members' values have in common, when the object
	// may be a keyed table.
	value *shape
}

// decideObject chooses between a record, whose keys are field names, and a
// keyed table, whose keys are data: one row to a member.
//
// It is asked only where a table may be declared. ok is false when the
// object can be neither, and reason says why.
func decideObject(object *objectShape) (choice objectChoice, ok bool, reason string) {
	_, recordProblem := fieldNames(object)
	value, tableProblem := keyedValue(object)

	keys := float64(0)
	for _, existing := range object.members {
		if readsAsFieldName(existing.key) {
			keys++
		}
	}
	fieldLike := 1.0
	if len(object.members) > 0 {
		fieldLike = keys / float64(len(object.members))
	}
	oneType := 0.0
	if tableProblem == "" && sameKind(object) {
		oneType = 1.0
	}
	// Eight members and more count fully as many.
	many := float64(len(object.members)-1) / 7
	if many > 1 {
		many = 1
	}
	if many < 0 {
		many = 0
	}

	result, err := judgment.Judgment{
		Name: "jsoninfer.object",
		Candidates: []judgment.Candidate{
			// A record is the plain reading: it wins a tie.
			{Name: asRecord, Priority: 1, Eligible: recordProblem == "", Reason: recordProblem},
			{Name: asKeyedTable, Eligible: tableProblem == "", Reason: tableProblem},
		},
		Considerations: []judgment.Consideration{
			// A key such as `read_timeout_ms` reads as a field name;
			// `user.created`, `u-100` and `2024` read as data.
			{Name: "keys read as field names", Weight: 2, Score: only(asRecord, fieldLike)},
			{Name: "keys read as data", Weight: 2, Score: only(asKeyedTable, 1-fieldLike)},
			// Rows of a table have one type; the fields of a record need not.
			{Name: "values differ in type", Weight: 0.5, Score: only(asRecord, 1-oneType)},
			{Name: "values share a type", Weight: 0.5, Score: only(asKeyedTable, oneType)},
			// A record has the members its author wrote; a table grows.
			{Name: "few members", Weight: 1, Score: only(asRecord, 1-many)},
			{Name: "many members", Weight: 1, Score: only(asKeyedTable, many)},
		},
	}.Decide()
	if err != nil {
		return objectChoice{}, false, fmt.Sprintf("an object that is neither a record (%s) nor a keyed table (%s)", recordProblem, tableProblem)
	}
	return objectChoice{trace: result, value: value}, true, ""
}

// keyedValue folds the values of an object's members into one shape, as the
// rows of a keyed table would be. problem says why they cannot be.
func keyedValue(object *objectShape) (value *shape, problem string) {
	if len(object.members) == 0 {
		return nil, "it has no members"
	}
	value = &shape{path: object.members[0].shape.path}
	for _, existing := range object.members {
		value = unify(value, clone(existing.shape))
	}
	if value.kind == kindRefused {
		return nil, "its values do not share a type: " + value.refusal.Reason
	}
	return value, ""
}

// sameKind reports whether every member of an object holds one kind of
// value, nulls aside.
func sameKind(object *objectShape) bool {
	found := kindUnknown
	for _, existing := range object.members {
		current := existing.shape.kind
		if current == kindUnknown {
			continue
		}
		if found != kindUnknown && found != current {
			return false
		}
		found = current
	}
	return true
}

// fieldNames gives each member of an object the field it is read into, in
// member order. problem says why the object cannot be a record.
func fieldNames(object *objectShape) (names []string, problem string) {
	taken := map[string]string{}
	for _, existing := range object.members {
		name := fieldName(existing.key)
		if name == "" {
			return nil, fmt.Sprintf("the key %s cannot be a field name", strconv.Quote(existing.key))
		}
		folded := octjson.FoldName(name)
		if other, exists := taken[folded]; exists {
			return nil, fmt.Sprintf("the keys %s and %s are one field name", strconv.Quote(other), strconv.Quote(existing.key))
		}
		taken[folded] = existing.key
		names = append(names, name)
	}
	return names, ""
}

// fieldName is the field a key is read into: the key in the capitalisation
// Oct fields have, without the separators Json ignores when it matches a
// key to a field. It is "" for a key no field can match: a field name is
// letters and digits and begins with a letter.
func fieldName(key string) string {
	var name strings.Builder
	capitalise := true
	for _, r := range key {
		switch {
		case r == '_' || r == '-' || r == '.' || r == ' ':
			capitalise = true
		case unicode.IsLetter(r) && capitalise:
			name.WriteRune(unicode.ToUpper(r))
			capitalise = false
		case unicode.IsLetter(r) || (unicode.IsDigit(r) && name.Len() > 0):
			name.WriteRune(r)
			capitalise = false
		default:
			return ""
		}
	}
	return name.String()
}

// readsAsFieldName reports whether a key looks like a name an author gave a
// field: words of letters and digits, each beginning with a letter, joined
// by `_` or `-`. `user.created`, `u-100`, `eu-west-1` and `2024` do not.
func readsAsFieldName(key string) bool {
	startOfWord := true
	for _, r := range key {
		switch {
		case unicode.IsLetter(r):
			startOfWord = false
		case unicode.IsDigit(r) && !startOfWord:
		case (r == '_' || r == '-') && !startOfWord:
			startOfWord = true
		default:
			return false
		}
	}
	return len(key) > 0 && !startOfWord
}

// isPlainWord reports whether a string is one word of letters, digits and
// underscores that begins with a letter: what a tag or an enum's variant is
// written as.
func isPlainWord(text string) bool {
	for index, r := range text {
		if !unicode.IsLetter(r) && (index == 0 || (!unicode.IsDigit(r) && r != '_')) {
			return false
		}
	}
	return text != ""
}

// tagNames are the keys a discriminator is commonly given.
var tagNames = map[string]bool{"type": true, "kind": true, "tag": true, "variant": true, "op": true}

// rowsChoice is what decideRows found.
type rowsChoice struct {
	trace judgment.Result
	// tag and values are the member that says which members a row has, and
	// the values it takes, when the rows may be a tagged array.
	tag    string
	values []string
}

// decideRows chooses between a table, whose rows are one record with some
// members optional, and a tagged array, where a member says which other
// members an object has. Json reads no declaration for the second (ladder
// decision D8), so that outcome is a refusal.
//
// It is asked when there are at least two rows.
func decideRows(rows []row) rowsChoice {
	signatures := make([]map[string]bool, len(rows))
	spelled := make([]string, len(rows))
	every := map[string]int{}
	for index, current := range rows {
		signatures[index] = map[string]bool{}
		memberPaths(current.node, "", signatures[index])
		spelled[index] = spell(signatures[index])
		for path := range signatures[index] {
			every[path]++
		}
	}
	shared := 0
	for _, count := range every {
		if count == len(rows) {
			shared++
		}
	}
	sharedPart := 1.0
	if len(every) > 0 {
		sharedPart = float64(shared) / float64(len(every))
	}

	// The distinct sets of members, and how many pairs of them each have a
	// member the other lacks.
	distinct := map[string]map[string]bool{}
	for index, text := range spelled {
		distinct[text] = signatures[index]
	}
	order := make([]string, 0, len(distinct))
	for text := range distinct {
		order = append(order, text)
	}
	sort.Strings(order)
	pairs, exclusive := 0, 0
	for i := 0; i < len(order); i++ {
		for j := i + 1; j < len(order); j++ {
			pairs++
			if !within(distinct[order[i]], distinct[order[j]]) && !within(distinct[order[j]], distinct[order[i]]) {
				exclusive++
			}
		}
	}
	exclusivePart := 0.0
	if pairs > 0 {
		exclusivePart = float64(exclusive) / float64(pairs)
	}

	tag, values, repeats := findTag(rows, spelled)
	problem := ""
	switch {
	case len(distinct) < 2:
		problem = "every object has the same members"
	case tag == "":
		problem = "no member says which members an object has"
	}
	named := 0.0
	if tagNames[octjson.FoldName(tag)] {
		named = 1
	}
	decides := 0.5
	if repeats {
		// Two objects with one tag and one set of members are evidence.
		// With every tag different, the tag decides the members trivially.
		decides = 1
	}

	result, _ := judgment.Judgment{
		Name: "jsoninfer.rows",
		Candidates: []judgment.Candidate{
			// A table is the plain reading: it wins a tie. It is always
			// eligible, so the judgment always has a winner.
			{Name: asTable, Priority: 1, Eligible: true},
			{Name: asTagged, Eligible: problem == "", Reason: problem},
		},
		Considerations: []judgment.Consideration{
			{Name: "members every object has", Weight: 2, Score: only(asTable, sharedPart)},
			{Name: "objects differ only by members left out", Weight: 2, Score: only(asTable, 1-exclusivePart)},
			{Name: "objects have members the others lack", Weight: 2, Score: only(asTagged, exclusivePart)},
			{Name: "a member named like a tag", Weight: 1, Score: only(asTagged, named)},
			{Name: "the tag decides the members", Weight: 0.5, Score: only(asTagged, decides)},
		},
	}.Decide()
	return rowsChoice{trace: result, tag: tag, values: values}
}

// memberPaths collects the members of an object, following objects inside
// it: `content.headline`. An array is one member, whatever it holds.
func memberPaths(node *octjson.Node, prefix string, into map[string]bool) {
	for _, written := range node.Members {
		if written.Value.Kind == octjson.NodeObject && len(written.Value.Members) > 0 {
			memberPaths(written.Value, prefix+written.Key+"\x00", into)
			continue
		}
		into[prefix+written.Key] = true
	}
}

func spell(set map[string]bool) string {
	paths := make([]string, 0, len(set))
	for path := range set {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return strings.Join(paths, "\x01")
}

func within(part map[string]bool, whole map[string]bool) bool {
	for path := range part {
		if !whole[path] {
			return false
		}
	}
	return true
}

// findTag looks for a member that says which members a row has: a plain
// word in every row, such that two rows with one value have one set of
// members. A key that is commonly a tag is preferred, then the first in
// the first row. repeats reports whether some value is taken by two rows.
func findTag(rows []row, spelled []string) (tag string, values []string, repeats bool) {
	found := false
	for _, candidate := range rows[0].node.Members {
		membersOf := map[string]string{}
		var taken []string
		decides, twice := true, false
		for index, current := range rows {
			value := memberOf(current.node, candidate.Key)
			if value == nil || value.Kind != octjson.NodeString || !isPlainWord(value.Text) {
				decides = false
				break
			}
			if before, seen := membersOf[value.Text]; seen {
				twice = true
				if before != spelled[index] {
					decides = false
					break
				}
				continue
			}
			membersOf[value.Text] = spelled[index]
			taken = append(taken, value.Text)
		}
		if !decides {
			continue
		}
		if !found || (tagNames[octjson.FoldName(candidate.Key)] && !tagNames[octjson.FoldName(tag)]) {
			tag, values, repeats, found = candidate.Key, taken, twice, true
		}
	}
	return tag, values, repeats
}

func memberOf(node *octjson.Node, key string) *octjson.Node {
	for _, written := range node.Members {
		if written.Key == key {
			return written.Value
		}
	}
	return nil
}
