package bench

import (
	"fmt"
	"strings"
)

const (
	BackendAuto  = "auto"
	BackendGodot = "godot"
	BackendKaiju = "kaiju"
)

func run(path string, manifest Manifest, selected []Case, options Options) (RunReport, error) {
	backend := options.Backend
	if backend == "" {
		backend = BackendAuto
	}
	switch backend {
	case BackendKaiju:
		return runKaiju(path, manifest, selected)
	case BackendGodot:
		return runGodot(path, manifest, selected)
	case BackendAuto:
		if report, err := runKaiju(path, manifest, selected); err == nil {
			return report, nil
		} else if !isKaijuUnavailableError(err) {
			return RunReport{}, err
		}
		return runGodot(path, manifest, selected)
	default:
		return RunReport{}, fmt.Errorf("unknown backend %q", backend)
	}
}

func isKaijuUnavailableError(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, `Kaiju sidecar "octxiliary-kaiju-vulkan" not found`) ||
		strings.Contains(message, "does not advertise dispatch+benchmark support")
}
