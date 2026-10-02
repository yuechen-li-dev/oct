package builtin

import "testing"

func TestRandomTableEntriesAreWellFormed(t *testing.T) {
	seen := map[string]struct{}{}
	for _, random := range RandomBuiltins() {
		if random.Symbol == "" {
			t.Fatalf("Random table has an entry with no symbol: %#v", random)
		}
		if _, duplicate := seen[random.Symbol]; duplicate {
			t.Errorf("Random builtin %q is listed twice", random.Symbol)
		}
		seen[random.Symbol] = struct{}{}

		switch random.Kind {
		case RandomSeed, RandomDraw, RandomEntropy:
		default:
			t.Errorf("Random builtin %q has unknown kind %q", random.Symbol, random.Kind)
		}
		switch random.ArityCheck {
		case RandomArityCounted, RandomArityMismatch, RandomArityUnchecked:
		default:
			t.Errorf("Random builtin %q has unknown arity check %q", random.Symbol, random.ArityCheck)
		}
		if random.Arguments < 0 {
			t.Errorf("Random builtin %q has negative argument count %d", random.Symbol, random.Arguments)
		}
		if random.Result == "" {
			t.Errorf("Random builtin %q has no result type", random.Symbol)
		}
		if (random.Kind == RandomEntropy) != random.Fallible {
			t.Errorf("Random builtin %q: entropy reads are fallible and nothing else is; kind %q, fallible %v", random.Symbol, random.Kind, random.Fallible)
		}
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

func TestRandomBuiltinsReserveBothSpellings(t *testing.T) {
	for _, random := range RandomBuiltins() {
		for _, spelling := range []string{random.Symbol, random.Name()} {
			if !IsName(spelling) {
				t.Errorf("Random builtin spelling %q is not a reserved builtin name", spelling)
			}
			if _, ok := Lookup(spelling); !ok {
				t.Errorf("Random builtin spelling %q has no semantic definition", spelling)
			}
		}
	}
	reserved := 0
	for name := range names {
		if _, ok := LookupRandom(name); ok {
			reserved++
		}
	}
	if want := 2 * len(RandomBuiltins()); reserved != want {
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
	if !IsRandomSymbol("RandInt") || IsRandomSymbol("Random.RandInt") || IsRandomSymbol("Len") {
		t.Error("IsRandomSymbol must accept exactly the unqualified Random symbols")
	}
}

// The execution lanes resolve the qualified spelling from any package and the
// unqualified spelling only inside package Random.
func TestResolveRandomCallScopesUnqualifiedNamesToPackageRandom(t *testing.T) {
	cases := []struct {
		callee        string
		callerPackage string
		want          bool
	}{
		{"Random.RandInt", "Main", true},
		{"Random.RandInt", "Random", true},
		{"RandInt", "Random", true},
		{"RandInt", "Main", false},
		{"RandInt", "", false},
		{"Len", "Random", false},
		{"Random.Missing", "Random", false},
	}
	for _, c := range cases {
		if _, got := ResolveRandomCall(c.callee, c.callerPackage); got != c.want {
			t.Errorf("ResolveRandomCall(%q, %q) = %v, want %v", c.callee, c.callerPackage, got, c.want)
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
