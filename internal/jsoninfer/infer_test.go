package jsoninfer

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/ocfmt"
	"github.com/yuechen-li-dev/oct/internal/octjson"
)

var update = flag.Bool("update", false, "rewrite the golden files with what is inferred now")

func inferText(t *testing.T, text string, options Options) Result {
	t.Helper()
	document, parseErr := octjson.Parse([]byte(text))
	if parseErr != nil {
		t.Fatalf("%s: %s", options.Source, parseErr.Text("parse", options.Source))
	}
	result, err := Infer(document, options)
	if err != nil {
		t.Fatalf("%s: %v", options.Source, err)
	}
	return result
}

// Every document under testdata/cases states one rule of section 3.11, or
// one edge of it. Its golden file is what the command prints for it with
// --explain. Where the output has no refusal, the declarations it prints
// are read back and the document is decoded with them, by the decoder
// Json.Load uses.
func TestCases(t *testing.T) {
	documents, err := filepath.Glob(filepath.Join("testdata", "cases", "*.json"))
	if err != nil || len(documents) == 0 {
		t.Fatalf("no cases found: %v", err)
	}
	for _, path := range documents {
		name := strings.TrimSuffix(filepath.Base(path), ".json")
		t.Run(name, func(t *testing.T) {
			text, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			result := inferText(t, string(text), Options{Source: filepath.Base(path)})
			got := result.Text() + result.Explain()
			golden := strings.TrimSuffix(path, ".json") + ".golden"
			if *update {
				if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatal(err)
			}
			if got != string(want) {
				t.Errorf("output differs from %s\n--- got\n%s--- want\n%s", golden, got, want)
			}
			// The output is laid out as `oct fmt` lays Oct out, so pasting it
			// into a formatted file leaves the file formatted.
			pasted := "package Main\n\n" + result.Text()
			if formatted, err := ocfmt.FormatSource(pasted); err != nil || formatted != pasted {
				t.Errorf("`oct fmt` would change the output (%v):\n%s", err, formatted)
			}
			if result.Loads() {
				mustLoad(t, string(text), result)
			} else if len(result.Refusals) == 0 || result.Refusals[0].Line < 1 || result.Refusals[0].Column < 1 {
				t.Errorf("a result that does not load names no place: %+v", result.Refusals)
			}
		})
	}
}

// mustLoad decodes a document with the declarations inferred for it.
func mustLoad(t *testing.T, text string, result Result) {
	t.Helper()
	schema, err := schemaOf(result.Text(), result.Type)
	if err != nil {
		t.Fatalf("the output does not read back as declarations: %v\n%s", err, result.Text())
	}
	if err := octjson.Check(schema); err != nil {
		t.Fatalf("the inferred type has no JSON form: %v\n%s", err, result.Text())
	}
	document, _ := octjson.Parse([]byte(text))
	if _, loadErr := octjson.Decode(document, schema, nil); loadErr != nil {
		t.Fatalf("the document does not load with its own declarations: %s\n%s\n%s", loadErr.Text("Json.Load", ""), text, result.Text())
	}
}

var declarationLine = regexp.MustCompile(`^(concept|record table) (\p{Lu}[\p{L}\p{Nd}]*) \{$`)
var fieldLine = regexp.MustCompile(`^    (\p{L}[\p{L}\p{Nd}]*): ([^ ]+)( // .*)?$`)

// schemaOf reads printed declarations back, as octjson describes types. It
// reads the text the command prints, so it checks what a person pastes.
func schemaOf(output string, root string) (*octjson.Schema, error) {
	type pending struct {
		schema *octjson.Schema
		fields [][2]string
	}
	declared := map[string]*pending{}
	var current *pending
	for _, line := range strings.Split(output, "\n") {
		switch {
		case line == "" || strings.HasPrefix(line, "//"):
		case line == "}":
			current = nil
		case declarationLine.MatchString(line):
			match := declarationLine.FindStringSubmatch(line)
			kind := octjson.KindRecord
			if match[1] == "record table" {
				kind = octjson.KindTable
			}
			if _, twice := declared[match[2]]; twice || reserved[match[2]] {
				return nil, fmt.Errorf("%s is declared twice or is a builtin type", match[2])
			}
			current = &pending{schema: &octjson.Schema{Kind: kind, Name: match[2]}}
			declared[match[2]] = current
		case current != nil && fieldLine.MatchString(line):
			match := fieldLine.FindStringSubmatch(line)
			current.fields = append(current.fields, [2]string{match[1], match[2]})
		default:
			return nil, fmt.Errorf("a line that is neither a declaration, a field nor a comment: %q", line)
		}
	}
	var typeOf func(expr string, cell bool) (*octjson.Schema, error)
	typeOf = func(expr string, cell bool) (*octjson.Schema, error) {
		switch {
		case strings.HasSuffix(expr, "[]"):
			elem, err := typeOf(strings.TrimSuffix(expr, "[]"), true)
			return &octjson.Schema{Kind: octjson.KindArray, Elem: elem}, err
		case strings.HasPrefix(expr, "Option<") && strings.HasSuffix(expr, ">"):
			elem, err := typeOf(expr[len("Option<"):len(expr)-1], cell)
			return &octjson.Schema{Kind: octjson.KindOption, Elem: elem}, err
		case expr == "Matrix<Float>":
			return &octjson.Schema{Kind: octjson.KindMatrix, Elem: &octjson.Schema{Kind: octjson.KindFloat}}, nil
		case expr == "Bool":
			return &octjson.Schema{Kind: octjson.KindBool}, nil
		case expr == "Int":
			return &octjson.Schema{Kind: octjson.KindInt}, nil
		case expr == "Float":
			return &octjson.Schema{Kind: octjson.KindFloat}, nil
		case expr == "String":
			return &octjson.Schema{Kind: octjson.KindString}, nil
		}
		named, ok := declared[expr]
		if !ok {
			return nil, fmt.Errorf("the type %s is not declared", expr)
		}
		if cell && named.schema.Kind == octjson.KindTable {
			// OCT-RTBL006, and no array of tables.
			return nil, fmt.Errorf("the table %s is used as a cell or an element", expr)
		}
		return named.schema, nil
	}
	names := make([]string, 0, len(declared))
	for name := range declared {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		d := declared[name]
		if d.schema.Kind == octjson.KindTable && len(d.fields) == 0 {
			return nil, fmt.Errorf("the table %s has no column", name)
		}
		for _, f := range d.fields {
			fieldType, err := typeOf(f[1], d.schema.Kind == octjson.KindTable)
			if err != nil {
				return nil, fmt.Errorf("%s.%s: %w", name, f[0], err)
			}
			d.schema.Fields = append(d.schema.Fields, octjson.Field{Name: f[0], Type: fieldType})
		}
	}
	return typeOf(root, false)
}

// Whatever the document, the inference is the same twice, and declarations
// said to load it do. The documents are generated from a fixed seed, from
// keys and values chosen to meet every rule: keys that are field names, keys
// that are data, keys no field can take, keys that are one field, objects
// with members others lack, nulls, empty arrays and rows of numbers.
func TestGeneratedDocumentsLoadWithWhatIsInferred(t *testing.T) {
	random := rand.New(rand.NewSource(20261007))
	loaded, refused, declared, chosen := 0, 0, 0, map[string]int{}
	for index := 0; index < 4000; index++ {
		text := generate(random, 0)
		result := inferText(t, text, Options{Source: "generated.json"})
		again := inferText(t, text, Options{Source: "generated.json"})
		if result.Text()+result.Explain() != again.Text()+again.Explain() {
			t.Fatalf("two inferences of one document differ:\n%s", text)
		}
		if !result.Loads() {
			refused++
			continue
		}
		loaded++
		declared += len(result.declarations)
		for _, decision := range result.Decisions {
			chosen[decision.Trace.Winner]++
		}
		mustLoad(t, text, result)
	}
	t.Logf("%d documents loaded with what was inferred (%d declarations; choices %v), %d had a value with no declaration", loaded, declared, chosen, refused)
	// Every outcome must be common, or the generator tests some of them only.
	if loaded < 1000 || refused < 300 || chosen[asRecord] < 500 || chosen[asKeyedTable] < 50 || chosen[asTable] < 50 {
		t.Errorf("%d documents loaded (choices %v) and %d were refused; the generator no longer covers every outcome", loaded, chosen, refused)
	}
}

var generatedKeys = [][]string{
	{"id", "name", "kind", "type", "status", "read_timeout_ms", "items", "meta"},
	{"u-100", "u-101", "user.created", "invoice.failed", "2024", "eu-west-1"},
	{"$schema", "user_id", "userId", "", "a b", "Option", "string"},
}
var generatedWords = []string{"open", "closed", "credit", "debit", "Avery Chen", "u-100", ""}

func generate(random *rand.Rand, depth int) string {
	choice := random.Intn(10)
	switch {
	case depth == 0:
		// The document itself is an array or an object.
		choice = 6 + random.Intn(4)
	case depth >= 4 && choice >= 6:
		choice = random.Intn(6)
	}
	switch choice {
	case 0:
		return "null"
	case 1:
		return strconv.FormatBool(random.Intn(2) == 0)
	case 2:
		return strconv.Itoa(random.Intn(7) - 2)
	case 3:
		return strconv.FormatFloat(float64(random.Intn(9))/4, 'f', -1, 64)
	case 4, 5:
		return strconv.Quote(generatedWords[random.Intn(len(generatedWords))])
	case 6:
		// Rows of numbers, sometimes ragged.
		rows := make([]string, random.Intn(3))
		width := 1 + random.Intn(2)
		for index := range rows {
			if random.Intn(6) == 0 {
				width++
			}
			cells := make([]string, width)
			for cell := range cells {
				cells[cell] = strconv.Itoa(random.Intn(5))
			}
			rows[index] = "[" + strings.Join(cells, ", ") + "]"
		}
		return "[" + strings.Join(rows, ", ") + "]"
	case 7:
		elements := make([]string, random.Intn(4))
		// Most arrays hold one kind of thing.
		if random.Intn(4) > 0 {
			keys := generatedKeys[0][:1+random.Intn(4)]
			for index := range elements {
				elements[index] = generateObject(random, depth+1, keys, random.Intn(3) == 0)
			}
		} else {
			for index := range elements {
				elements[index] = generate(random, depth+1)
			}
		}
		return "[" + strings.Join(elements, ", ") + "]"
	default:
		pool := generatedKeys[0]
		if random.Intn(3) == 0 {
			pool = generatedKeys[1+random.Intn(2)]
		}
		keys := append([]string(nil), pool...)
		random.Shuffle(len(keys), func(i, j int) { keys[i], keys[j] = keys[j], keys[i] })
		return generateObject(random, depth+1, keys[:random.Intn(len(keys)+1)], false)
	}
}

// generateObject writes an object with the keys given, leaving some out
// when sparse is set, as the rows of a table with optional cells do.
func generateObject(random *rand.Rand, depth int, keys []string, sparse bool) string {
	members := make([]string, 0, len(keys))
	for _, key := range keys {
		if sparse && random.Intn(3) == 0 {
			continue
		}
		value := generate(random, depth)
		if key == "kind" || key == "type" || key == "status" {
			value = strconv.Quote(generatedWords[random.Intn(4)])
		}
		members = append(members, strconv.Quote(key)+": "+value)
	}
	return "{" + strings.Join(members, ", ") + "}"
}

func TestFieldName(t *testing.T) {
	for key, want := range map[string]string{
		"id": "Id", "read_timeout_ms": "ReadTimeoutMs", "readTimeoutMs": "ReadTimeoutMs", "cta-href": "CtaHref",
		"user.created": "UserCreated", "u-100": "U100", "SKU": "SKU", "_private": "Private", "a b": "AB", "x2y": "X2y",
		"größe": "Größe", "naïve_wert": "NaïveWert", "名前": "名前", "ß": "ß", "x٣": "X٣",
		"": "", "3d": "", "$schema": "", "__": "", "2024": "", "a/b": "", "٣x": "",
	} {
		got := fieldName(key)
		if got != want {
			t.Errorf("fieldName(%q) = %q, want %q", key, got, want)
		}
		// Json matches the field to the key it came from.
		if got != "" && octjson.FoldName(got) != octjson.FoldName(key) {
			t.Errorf("fieldName(%q) = %q, which Json does not match to the key", key, got)
		}
	}
}

func TestReadsAsFieldName(t *testing.T) {
	for key, want := range map[string]bool{
		"id": true, "read_timeout_ms": true, "readTimeoutMs": true, "cta-href": true, "http2_enabled": true, "X": true,
		"größe": true, "名前": true,
		"user.created": false, "u-100": false, "eu-west-1": false, "2024": false, "a b": false, "": false, "name_": false, "_name": false, "$schema": false,
	} {
		if got := readsAsFieldName(key); got != want {
			t.Errorf("readsAsFieldName(%q) = %v, want %v", key, got, want)
		}
	}
}

// The root declaration takes the name it is given, or the file's.
func TestRootName(t *testing.T) {
	document, _ := octjson.Parse([]byte(`{"people": [{"name": "a"}]}`))
	for _, c := range []struct{ source, name, want string }{
		{"data/tickets.json", "", "Tickets"},
		{"example_01_people.json", "", "Example01People"},
		{"2024-data.json", "", "Document"},
		{"string.json", "", "Document"},
		{"x.json", "Backlog", "Backlog"},
		// The name is the root's, though a member asks for it first.
		{"x.json", "People", "People"},
	} {
		result, err := Infer(document, Options{Source: c.source, Name: c.name})
		if err != nil || result.Type != c.want {
			t.Errorf("%s --name %q: the document loads into %q (%v), want %q", c.source, c.name, result.Type, err, c.want)
		}
	}
	if result, err := Infer(document, Options{Source: "x.json", Name: "Größen"}); err != nil || result.Type != "Größen" {
		t.Errorf("--name Größen: %q, %v", result.Type, err)
	}
	for _, name := range []string{"backlog", "Back log", "2Fast", "String", "Option", "Matrix", "Tick-et", "名前"} {
		if _, err := Infer(document, Options{Source: "x.json", Name: name}); err == nil || !strings.Contains(err.Error(), "--name") {
			t.Errorf("--name %q is accepted: %v", name, err)
		}
	}
}

// The documents milestone M6 of the ladder names: the seven of the
// acceptance corpus, the fourteen summaries the experiments record, and the
// seven under Libraries/IO/testdata.
var ladderDocuments = []string{
	filepath.Join("Experiments", "JsonIntentRecoveryLab", "M0", "corpus", "*.json"),
	filepath.Join("Experiments", "*", "M*", "*.json"),
	filepath.Join("Libraries", "IO", "testdata", "*.json"),
}

// refusedDocuments are the two of them with a value that has no
// declaration: the tagged arrays of the corpus (ladder decision D8). What
// the command prints for each is in testdata/refused.
var refusedDocuments = []string{
	filepath.Join("Experiments", "JsonIntentRecoveryLab", "M0", "corpus", "example_06_ui_like.json"),
	filepath.Join("Experiments", "JsonIntentRecoveryLab", "M0", "corpus", "example_07_tagged.json"),
}

var loadLine = regexp.MustCompile(`(?m)^// Json\.Load<(.+)>\("(.+)"\)\?$`)

// "Infer, paste, load." Each contract under Language/Tooling/JsonInfer is
// what the command prints for one document, pasted, and a fact that loads
// the document with it; the corpus test runs those in both lanes. This test
// holds the command to what was pasted, and checks that every document the
// ladder names is either loaded by a contract or refused here.
func TestLadderDocuments(t *testing.T) {
	root := filepath.Join("..", "..")
	covered := map[string]bool{}

	contracts, err := filepath.Glob(filepath.Join(root, "Language", "Tooling", "JsonInfer", "*", "*.octest"))
	if err != nil || len(contracts) == 0 {
		t.Fatalf("no contracts found: %v", err)
	}
	for _, contract := range contracts {
		pasted, err := os.ReadFile(contract)
		if err != nil {
			t.Fatal(err)
		}
		match := loadLine.FindStringSubmatch(string(pasted))
		if match == nil {
			t.Errorf("%s has no `// Json.Load<...>(...)?` line", contract)
			continue
		}
		options := Options{Source: match[2]}
		if isDeclarationName(match[1]) {
			options.Name = match[1]
		}
		text, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(options.Source)))
		if err != nil {
			t.Errorf("%s: %v", contract, err)
			continue
		}
		result := inferText(t, string(text), options)
		if !result.Loads() {
			t.Errorf("%s: the command now refuses %s", contract, options.Source)
		}
		if !strings.Contains(string(pasted), "\n\n"+result.Text()+"\n[Fact]") {
			t.Errorf("%s does not hold what the command prints for %s:\n%s", contract, options.Source, result.Text())
		}
		mustLoad(t, string(text), result)
		covered[filepath.FromSlash(options.Source)] = true
	}

	for _, source := range refusedDocuments {
		text, err := os.ReadFile(filepath.Join(root, source))
		if err != nil {
			t.Fatal(err)
		}
		result := inferText(t, string(text), Options{Source: filepath.ToSlash(source)})
		got := result.Text() + result.Explain()
		golden := filepath.Join("testdata", "refused", strings.TrimSuffix(filepath.Base(source), ".json")+".golden")
		if *update {
			if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		want, err := os.ReadFile(golden)
		if err != nil {
			t.Fatal(err)
		}
		if got != string(want) {
			t.Errorf("output differs from %s\n--- got\n%s--- want\n%s", golden, got, want)
		}
		if result.Loads() {
			t.Errorf("%s is no longer refused; it needs a contract under Language/Tooling/JsonInfer", source)
		}
		covered[source] = true
	}

	documents := map[string]bool{}
	for _, pattern := range ladderDocuments {
		found, err := filepath.Glob(filepath.Join(root, pattern))
		if err != nil {
			t.Fatal(err)
		}
		for _, path := range found {
			source, _ := filepath.Rel(root, path)
			documents[source] = true
			if !covered[source] {
				t.Errorf("%s has no contract under Language/Tooling/JsonInfer and is not refused in testdata/refused", source)
			}
		}
	}
	if len(documents) != 28 || len(covered) != len(documents) {
		t.Errorf("the ladder names 28 documents; %d were found and %d are covered", len(documents), len(covered))
	}
}

// A declaration is never given the name of a builtin type, which Oct
// refuses or, for some, quietly reads as the builtin.
func TestBuiltinTypeNamesAreNotDeclared(t *testing.T) {
	builtins := []string{"Int", "Float", "Complex", "Bool", "String", "Bytes", "Error", "Void", "UI", "Index", "Range", "Option", "Vector", "Matrix"}
	if len(builtins) != len(reserved) {
		t.Errorf("%d builtin types are listed here and %d are reserved", len(builtins), len(reserved))
	}
	for _, name := range builtins {
		text := `{"` + name + `": {"a": 1}, "rows": {"` + name + `": [{"b": 2}]}}`
		result := inferText(t, text, Options{Source: "x.json"})
		mustLoad(t, text, result)
		for _, declared := range result.declarations {
			if declared.name == name {
				t.Errorf("a declaration is named %s:\n%s", name, result.Text())
			}
		}
		if !strings.Contains(result.Text(), "    "+name+": X"+name+"\n") || !strings.Contains(result.Text(), "    "+name+": Rows"+name+"\n") {
			t.Errorf("the declarations for a key %s are not named after their parents:\n%s", name, result.Text())
		}
	}
}
