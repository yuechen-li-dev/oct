//go:build octentropyfail

package octrandom

import "errors"

// With the build tag octentropyfail every read of the entropy source fails.
// The tag exists for one host-side test: a compiled Oct program runs in its
// own process, so building it with this tag is the only way to make its
// random source fail. Nothing is built with the tag in normal use, and a
// program built with it gets errors, never weaker randomness.

type failingEntropySource struct{}

func (failingEntropySource) Read([]byte) (int, error) {
	return 0, errors.New("injected source failure")
}

func init() {
	entropySource = failingEntropySource{}
}
