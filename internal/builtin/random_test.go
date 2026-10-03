package builtin

import "testing"

func TestRandomTableEntriesAreWellFormed(t *testing.T) {
	seen := map[string]struct{}{}
	for _, random := range RandomBuiltins() {
		if random.Symbol == "" {
			t.Fatalf("the table has an entry with no symbol: %#v", random)
		}
		if !IsRandomNamespace(random.Namespace) {
			t.Errorf("builtin %q is in namespace %q, which the table does not describe", random.Symbol, random.Namespace)
		}
		name := random.Name()
		if _, duplicate := seen[name]; duplicate {
			t.Errorf("builtin %q is listed twice", name)
		}
		seen[name] = struct{}{}

		switch random.Kind {
		case RandomSeed, RandomDraw, RandomEntropy:
		default:
			t.Errorf("builtin %q has unknown kind %q", name, random.Kind)
		}
		if random.Result == "" {
			t.Errorf("builtin %q has no result type", name)
		}
		if (random.Kind == RandomEntropy) != random.Fallible {
			t.Errorf("builtin %q: entropy reads are fallible and nothing else is; kind %q, fallible %v", name, random.Kind, random.Fallible)
		}
		for index, parameter := range random.Parameters {
			switch parameter {
			case RandomParameterInt, RandomParameterFloat, RandomParameterString:
			case RandomParameterStream:
				if random.Namespace != RandomNamespace {
					t.Errorf("builtin %q parameter %d is a Stream; only package Random has streams", name, index+1)
				}
			default:
				t.Errorf("builtin %q parameter %d has unknown type %q", name, index+1, parameter)
			}
		}
		// The artifact and discovery guards reject by kind, so the kind must
		// follow the namespace: every Entropy builtin reads entropy, and
		// package Random is deterministic.
		if (random.Namespace == EntropyNamespace) != (random.Kind == RandomEntropy) {
			t.Errorf("builtin %q has kind %q; exactly the Entropy builtins read entropy", name, random.Kind)
		}
		wantInPackage := random.Namespace == RandomNamespace && random.Result == string(RandomParameterStream)
		if random.ResultInPackage != wantInPackage {
			t.Errorf("builtin %q result %q: only Random's Stream is a package record", name, random.Result)
		}
	}
}

// tableSignatures renders the builtins of one namespace as
// "(parameters) -> result", with "! Error" for a fallible builtin.
func tableSignatures(namespace string) map[string]string {
	got := map[string]string{}
	for _, random := range RandomBuiltins() {
		if random.Namespace != namespace {
			continue
		}
		signature := "("
		for index, parameter := range random.Parameters {
			if index > 0 {
				signature += ", "
			}
			signature += string(parameter)
		}
		signature += ") -> " + random.Result
		if random.Fallible {
			signature += " ! Error"
		}
		got[random.Symbol] = signature
	}
	return got
}

func checkTableSignatures(t *testing.T, namespace string, want map[string]string) {
	t.Helper()
	got := tableSignatures(namespace)
	for symbol, signature := range want {
		if got[symbol] != signature {
			t.Errorf("%s.%s signature = %q, want %q", namespace, symbol, got[symbol], signature)
		}
	}
	for symbol := range got {
		if _, ok := want[symbol]; !ok {
			t.Errorf("%s.%s is in the table but not in the specification", namespace, symbol)
		}
	}
}

// The Random table must match internal/random/Random.md exactly: these seven
// signatures and no others.
func TestRandomStreamBuiltinSignatures(t *testing.T) {
	checkTableSignatures(t, RandomNamespace, map[string]string{
		"Seeded":     "(Int) -> Stream",
		"Fork":       "(Stream, String) -> Stream",
		"Child":      "(Stream, Int) -> Stream",
		"Unit":       "(Stream, Int) -> Float",
		"Between":    "(Stream, Int, Float, Float) -> Float",
		"IntBetween": "(Stream, Int, Int, Int) -> Int",
		"Normal":     "(Stream, Int, Float, Float) -> Float",
	})
}

// The Entropy table must match internal/random/Random.md exactly: these four
// signatures and no others.
func TestEntropyBuiltinSignatures(t *testing.T) {
	checkTableSignatures(t, EntropyNamespace, map[string]string{
		"Seed":       "() -> Int ! Error",
		"IntBetween": "(Int, Int) -> Int ! Error",
		"Unit":       "() -> Float ! Error",
		"Bytes":      "(Int) -> Bytes ! Error",
	})
}

// Entropy needs no import, like Artifact. Random does.
func TestEntropyIsACompilerOwnedNamespace(t *testing.T) {
	if !IsCompilerOwnedNamespace(EntropyNamespace) {
		t.Error("Entropy must be a compiler-owned namespace")
	}
	if IsCompilerOwnedNamespace(RandomNamespace) {
		t.Error("Random must require an import")
	}
	if !IsRandomNamespace(RandomNamespace) || !IsRandomNamespace(EntropyNamespace) || IsRandomNamespace("Main") || IsRandomNamespace("") {
		t.Error("IsRandomNamespace must accept exactly Random and Entropy")
	}
}

// Every builtin reserves its qualified name and nothing else: a name such as
// "Unit" or "Normal" must stay available to every other package.
func TestRandomBuiltinNameReservation(t *testing.T) {
	for _, random := range RandomBuiltins() {
		if !IsName(random.Name()) {
			t.Errorf("builtin %q is not a reserved builtin name", random.Name())
		}
		if _, ok := Lookup(random.Name()); !ok {
			t.Errorf("builtin %q has no semantic definition", random.Name())
		}
		if IsName(random.Symbol) {
			t.Errorf("unqualified name %q is reserved", random.Symbol)
		}
	}
	reserved := 0
	for name := range names {
		if _, ok := LookupRandomQualified(name); ok {
			reserved++
		}
	}
	if reserved != len(RandomBuiltins()) {
		t.Errorf("%d reserved names resolve to table builtins, want %d", reserved, len(RandomBuiltins()))
	}
}

// Random v1 is gone. Its names must not be builtins in any spelling, so that
// a program still using one gets an ordinary "unknown" diagnostic.
func TestRandomV1NamesAreNotBuiltins(t *testing.T) {
	for _, symbol := range []string{"RngSeed", "RandInt", "RandFloat01", "RandFloatRange", "RandBernoulli", "RandNormal", "Gaussian", "CryptoRandInt", "CryptoRandFloat01", "CryptoRandBytes"} {
		if IsName(symbol) || IsName("Random."+symbol) {
			t.Errorf("Random v1 name %q is still reserved", symbol)
		}
		if _, ok := ResolveRandomCall(symbol, RandomNamespace); ok {
			t.Errorf("Random v1 name %q still resolves inside package Random", symbol)
		}
		if _, ok := ResolveRandomCall("Random."+symbol, "Main"); ok {
			t.Errorf("Random v1 name Random.%s still resolves", symbol)
		}
	}
}

func TestLookupRandomQualifiedSpellings(t *testing.T) {
	if random, ok := LookupRandomQualified("Random.Unit"); !ok || random.Name() != "Random.Unit" {
		t.Errorf("LookupRandomQualified(Random.Unit) = %#v, %v", random, ok)
	}
	if random, ok := LookupRandomQualified("Entropy.Seed"); !ok || random.Name() != "Entropy.Seed" {
		t.Errorf("LookupRandomQualified(Entropy.Seed) = %#v, %v", random, ok)
	}
	for _, name := range []string{"", "Random", "Random.", "Random.Missing", "Unit", "Seed", "Other.Unit", "Random.Random.Unit", "Entropy.Seeded", "Random.Seed"} {
		if _, ok := LookupRandomQualified(name); ok {
			t.Errorf("LookupRandomQualified(%q) resolved", name)
		}
	}
}

// Random and Entropy share the symbols Unit and IntBetween, so an unqualified
// symbol resolves only together with its package.
func TestLookupRandomInResolvesASymbolInsideItsOwnPackage(t *testing.T) {
	cases := []struct {
		namespace string
		symbol    string
		want      string
	}{
		{"Random", "Unit", "Random.Unit"},
		{"Entropy", "Unit", "Entropy.Unit"},
		{"Random", "IntBetween", "Random.IntBetween"},
		{"Entropy", "IntBetween", "Entropy.IntBetween"},
		{"Entropy", "Seed", "Entropy.Seed"},
		{"Random", "Seed", ""},
		{"Entropy", "Seeded", ""},
		{"Main", "Unit", ""},
		{"", "Unit", ""},
		{"Random", "Random.Unit", ""},
		{"Random", "Len", ""},
	}
	for _, c := range cases {
		random, ok := LookupRandomIn(c.namespace, c.symbol)
		got := ""
		if ok {
			got = random.Name()
		}
		if got != c.want {
			t.Errorf("LookupRandomIn(%q, %q) = %q, want %q", c.namespace, c.symbol, got, c.want)
		}
	}
}

// The execution lanes resolve the qualified spelling from any package and the
// unqualified spelling only inside the builtin's own package. want is the
// qualified name the call resolves to, or empty when it does not resolve.
func TestResolveRandomCallScopesUnqualifiedNamesToTheirOwnPackage(t *testing.T) {
	cases := []struct {
		callee        string
		callerPackage string
		want          string
	}{
		{"Random.Unit", "Main", "Random.Unit"},
		{"Random.Unit", "Random", "Random.Unit"},
		{"Random.Unit", "Entropy", "Random.Unit"},
		{"Unit", "Random", "Random.Unit"},
		{"Unit", "Main", ""},
		{"Unit", "", ""},
		{"Normal", "Statistics", ""},
		{"Len", "Random", ""},
		{"Random.Missing", "Random", ""},
		{"Entropy.Seed", "Main", "Entropy.Seed"},
		{"Entropy.Unit", "Random", "Entropy.Unit"},
		{"Seed", "Entropy", "Entropy.Seed"},
		{"Unit", "Entropy", "Entropy.Unit"},
		{"IntBetween", "Entropy", "Entropy.IntBetween"},
		{"IntBetween", "Random", "Random.IntBetween"},
		{"Seed", "Random", ""},
		{"Seed", "Main", ""},
		{"Seeded", "Entropy", ""},
		{"Entropy.Missing", "Entropy", ""},
	}
	for _, c := range cases {
		random, ok := ResolveRandomCall(c.callee, c.callerPackage)
		got := ""
		if ok {
			got = random.Name()
		}
		if got != c.want {
			t.Errorf("ResolveRandomCall(%q, %q) = %q, want %q", c.callee, c.callerPackage, got, c.want)
		}
	}
}

func TestRandomResultTypeSpelling(t *testing.T) {
	record, _ := LookupRandomQualified("Random.Seeded")
	if got := record.ResultType(); got != "Random.Stream" {
		t.Errorf("record result type = %q, want Random.Stream", got)
	}
	base, _ := LookupRandomQualified("Entropy.Seed")
	if got := base.ResultType(); got != "Int" {
		t.Errorf("base result type = %q, want Int", got)
	}
}
