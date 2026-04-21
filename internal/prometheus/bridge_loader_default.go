//go:build (!linux && !windows) || !cgo

package prometheus

func defaultReactorLoader() reactorLoader {
	return unavailableLoader{}
}
