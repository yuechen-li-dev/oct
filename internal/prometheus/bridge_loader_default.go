//go:build !linux || !cgo

package prometheus

func defaultReactorLoader() reactorLoader {
	return unavailableLoader{}
}
