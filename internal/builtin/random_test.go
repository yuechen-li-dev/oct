package builtin

import "testing"

func TestRandomTableEntriesAreWellFormed(t *testing.T) {
	seen := map[string]struct{}{}
	for _, random := range RandomBuiltins() {
		if random.Symbol == "" {
			t.Fatalf("Random table has an entry with no symbol: %#v", random)
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
		switch random.ArityCheck {
		case RandomArityCounted, RandomArityMismatch, RandomArityUnchecked:
		default:
			t.Errorf("builtin %q has unknown arity check %q", name, random.ArityCheck)
		}
		if random.Arguments < 0 {
			t.Errorf("builtin %q has negative argument count %d", name, random.Arguments)
		}
		if random.Result == "" {
			t.Errorf("builtin %q has no result type", name)
		}
		if (random.Kind == RandomEntropy) != random.Fallible {
			t.Errorf("builtin %q: entropy reads are fallible and nothing else is; kind %q, fallible %v", name, random.Kind, random.Fallible)
		}
		if random.Legacy {
			if random.Namespace != RandomNamespace {
				t.Errorf("legacy builtin %q is outside package Random; only Random v1 is legacy", name)
			}
			if len(random.Parameters) != 0 {
				t.Errorf("legacy builtin %q declares parameter types; v1 arguments are not type-checked", name)
			}
			continue
		}
		if random.ArityCheck != RandomArityCounted {
			t.Errorf("builtin %q must use the counted arity check, got %q", name, random.ArityCheck)
		}
		if random.Arguments != len(random.Parameters) {
			t.Errorf("builtin %q declares %d arguments and %d parameter types", name, random.Arguments, len(random.Parameters))
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
		if !random.HasOwnImplementation() {
			t.Errorf("builtin %q must have its own implementation", name)
		}
		// The artifact and discovery guards reject by kind, so the kind must
		// follow the namespace: every Entropy builtin reads entropy, and
		// package Random is deterministic in v2.
		if (random.Namespace == EntropyNamespace) != (random.Kind == RandomEntropy) {
			t.Errorf("builtin %q has kind %q; exactly the Entropy builtins read entropy", name, random.Kind)
		}
		wantInPackage := random.Namespace == RandomNamespace && random.Result == string(RandomParameterStream)
		if random.ResultInPackage != wantInPackage {
			t.Errorf("builtin %q result %q: only Random's Stream is a package record", name, random.Result)
		}
	}
}

// tableSignatures renders the non-legacy builtins of one namespace as
// "(parameters) -> result", with "! Error" for a fallible builtin.
func tableSignatures(namespace string) map[string]string {
	got := map[string]string{}
	for _, random := range RandomBuiltins() {
		if random.Legacy || random.Namespace != namespace {
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

// The v2 table must match ladder section 3.3 exactly: these seven signatures
// and no others.
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

// The Entropy table must match ladder section 3.5 exactly: these four
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

func TestRandomImplementationsResolveToOwnImplementations(t *testing.T) {
	for _, random := range RandomBuiltins() {
		target, ok := LookupRandomQualified(random.Implementation())
		if !ok {
			t.Errorf("Random builtin %q is implemented by %q, which is not in the table", random.Symbol, random.Implementation())
			continue
		}
		if !target.HasOwnImplementation() {
			t.Errorf("Random builtin %q is implemented by %q, which is itself an alias", random.Symbol, target.Symbol)
		}
		if random.HasOwnImplementation() {
			if random.Implementation() != random.Name() {
				t.Errorf("Random builtin %q implements itself as %q", random.Name(), random.Implementation())
			}
			continue
		}
		if target.Kind != random.Kind || target.Arguments != random.Arguments || target.Result != random.Result || target.ResultInPackage != random.ResultInPackage || target.Fallible != random.Fallible {
			t.Errorf("Random builtin %q does not share the signature of its implementation %q", random.Symbol, target.Symbol)
		}
	}
}

// Every Random builtin reserves its qualified name. Only a legacy builtin also
// reserves its unqualified name: a v2 name such as "Unit" or "Normal" must stay
// available to every other package.
func TestRandomBuiltinNameReservation(t *testing.T) {
	want := 0
	for _, random := range RandomBuiltins() {
		want++
		if !IsName(random.Name()) {
			t.Errorf("Random builtin %q is not a reserved builtin name", random.Name())
		}
		if _, ok := Lookup(random.Name()); !ok {
			t.Errorf("Random builtin %q has no semantic definition", random.Name())
		}
		if random.Legacy {
			want++
		}
		if IsName(random.Symbol) != random.Legacy {
			t.Errorf("unqualified name %q reserved = %v, want %v (legacy = %v)", random.Symbol, IsName(random.Symbol), random.Legacy, random.Legacy)
		}
	}
	reserved := 0
	for name := range names {
		if _, ok := LookupRandom(name); ok {
			reserved++
		}
	}
	if reserved != want {
		t.Errorf("%d reserved names resolve to Random builtins, want %d", reserved, want)
	}
}

func TestRandomLookupSpellings(t *testing.T) {
	if random, ok := LookupRandom("Random.RandInt"); !ok || random.Symbol != "RandInt" {
		t.Errorf("LookupRandom(qualified) = %#v, %v", random, ok)
	}
	if random, ok := LookupRandom("RandInt"); !ok || random.Symbol != "RandInt" {
		t.Errorf("LookupRandom(unqualified) = %#v, %v", random, ok)
	}
	if _, ok := LookupRandomQualified("RandInt"); ok {
		t.Error("LookupRandomQualified accepted an unqualified name")
	}
	for _, name := range []string{"", "Random", "Random.", "Random.Missing", "Missing", "Other.RandInt", "Random.Random.RandInt"} {
		if _, ok := LookupRandom(name); ok {
			t.Errorf("LookupRandom(%q) resolved", name)
		}
	}
	if random, ok := LookupRandom("Entropy.Seed"); !ok || random.Name() != "Entropy.Seed" {
		t.Errorf("LookupRandom(Entropy.Seed) = %#v, %v", random, ok)
	}
	// An unqualified non-legacy name is not reserved, so it does not resolve
	// without the calling package.
	for _, name := range []string{"Unit", "Seeded", "Seed", "Bytes", "IntBetween", "Entropy.Seeded", "Random.Seed", "Entropy.RandInt"} {
		if _, ok := LookupRandom(name); ok {
			t.Errorf("LookupRandom(%q) resolved", name)
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
		{"Random", "RandInt", "Random.RandInt"},
		{"Entropy", "Seed", "Entropy.Seed"},
		{"Random", "Seed", ""},
		{"Entropy", "Seeded", ""},
		{"Entropy", "RandInt", ""},
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
		{"Random.RandInt", "Main", "Random.RandInt"},
		{"Random.RandInt", "Random", "Random.RandInt"},
		{"RandInt", "Random", "Random.RandInt"},
		{"RandInt", "Main", ""},
		{"RandInt", "Entropy", ""},
		{"RandInt", "", ""},
		{"Len", "Random", ""},
		{"Random.Missing", "Random", ""},
		{"Random.Unit", "Main", "Random.Unit"},
		{"Random.Unit", "Random", "Random.Unit"},
		{"Random.Unit", "Entropy", "Random.Unit"},
		{"Unit", "Random", "Random.Unit"},
		{"Unit", "Main", ""},
		{"Normal", "Statistics", ""},
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
	record, _ := LookupRandom("RandInt")
	if got := record.ResultType(); got != "Random.RandIntResult" {
		t.Errorf("record result type = %q, want Random.RandIntResult", got)
	}
	base, _ := LookupRandom("CryptoRandInt")
	if got := base.ResultType(); got != "Int" {
		t.Errorf("base result type = %q, want Int", got)
	}
	alias, _ := LookupRandom("Gaussian")
	if alias.Name() != "Random.Gaussian" || alias.Implementation() != "Random.RandNormal" || alias.HasOwnImplementation() {
		t.Errorf("Gaussian must be served by Random.RandNormal: name %q, implementation %q", alias.Name(), alias.Implementation())
	}
}
