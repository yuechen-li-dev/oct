package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	"github.com/yuechen-li-dev/oct/pkg/octxiliary"
	"github.com/yuechen-li-dev/oct/pkg/octxiliary/kaijuvulkan"
)

const (
	sidecarName      = "octxiliary-kaiju-vulkan"
	sidecarVersion   = "m36b"
	upstreamCommit   = "ed509b23ed2b230fefe1c6c4ed00f9fa27315ab2"
	forkCommit       = "local-snapshot-ed509b23ed2b230fefe1c6c4ed00f9fa27315ab2"
	linuxStatus      = "source-build-lane-only-runtime-not-proven"
	windowsProven    = true
)

func main() {
	version := flag.Bool("version", false, "print sidecar version")
	flag.Parse()
	if *version {
		fmt.Printf("%s %s protocol=%d kaiju=%s fork=%s go=%s\n", sidecarName, sidecarVersion, kaijuvulkan.ProtocolVersion, upstreamCommit, forkCommit, runtime.Version())
		return
	}
	if octxiliary.Main(os.Stdin, os.Stdout, handleRequest) != 0 {
		os.Exit(1)
	}
}

func handleRequest(req octxiliary.Request) octxiliary.Response {
	if req.Family != kaijuvulkan.Family {
		return octxiliary.ErrString(req.ID, fmt.Sprintf("unsupported family %q", req.Family))
	}
	switch req.Function {
	case kaijuvulkan.OperationCapabilities:
		return capabilities(req)
	case kaijuvulkan.OperationDispatch:
		request, err := kaijuvulkan.ParseDispatchRequestArg(req, 0)
		if err != nil {
			return octxiliary.OkValue(req.ID, kaijuvulkan.DispatchResponseValue(protocolFailure(errorUnsupportedProtocol, "", err.Error())))
		}
		return dispatch(req.ID, request)
	case kaijuvulkan.OperationBenchmark:
		request, err := kaijuvulkan.ParseBenchmarkRequestArg(req, 0)
		if err != nil {
			return octxiliary.OkValue(req.ID, kaijuvulkan.DispatchResponseValue(protocolFailure(errorUnsupportedProtocol, "", err.Error())))
		}
		return benchmark(req.ID, request)
	default:
		return octxiliary.OkValue(req.ID, kaijuvulkan.DispatchResponseValue(protocolFailure(errorUnknownOperation, "", fmt.Sprintf("unknown operation %q", req.Function))))
	}
}

func capabilities(req octxiliary.Request) octxiliary.Response {
	runtimeInfo, err := discoverRuntime(false)
	if err != nil {
		response := protocolFailure(errorVulkanUnavailable, "", err.Error())
		return octxiliary.OkValue(req.ID, kaijuvulkan.DispatchResponseValue(response))
	}
	validationInfo, _ := queryValidationLayer()
	value := kaijuvulkan.CapabilitiesValue(kaijuvulkan.Capabilities{
		Protocol: kaijuvulkan.ProtocolVersion,
		KaijuUpstreamCommit: upstreamCommit,
		KaijuForkCommit: forkCommit,
		OS: runtime.GOOS,
		Architecture: runtime.GOARCH,
		Headless: true,
		DispatchSupported: true,
		BenchmarkSupported: true,
		Set0Only: true,
		StorageBuffers: true,
		ExplicitEntryPoints: true,
		PushConstants: true,
		QueryTimestamps: runtimeInfo.Device.TimestampValidBits > 0,
		ValidationAvailable: validationInfo.available,
		ValidationEnabled: false,
		WindowsProven: windowsProven,
		LinuxStatus: linuxStatus,
		SupportedAccesses: []string{kaijuvulkan.ResourceAccessReadonly, kaijuvulkan.ResourceAccessReadwrite},
		SupportedKinds: []string{kaijuvulkan.ResourceKindStorageBuffer},
		SupportedElements: []string{
			kaijuvulkan.ElementTypeBytes,
			kaijuvulkan.ElementTypeU32,
			kaijuvulkan.ElementTypeI32,
			kaijuvulkan.ElementTypeF32,
			kaijuvulkan.ElementTypeFloat2,
			kaijuvulkan.ElementTypeFloat4,
		},
		Limits: sidecarLimits(),
		Device: runtimeInfo.Device,
	})
	return octxiliary.OkValue(req.ID, value)
}

func dispatch(id int, request kaijuvulkan.DispatchRequest) octxiliary.Response {
	validated, failure := validateDispatchRequest(request)
	if failure != nil {
		return octxiliary.OkValue(id, kaijuvulkan.DispatchResponseValue(*failure))
	}
	result := execute(validated)
	return octxiliary.OkValue(id, kaijuvulkan.DispatchResponseValue(result))
}

func benchmark(id int, request kaijuvulkan.BenchmarkRequest) octxiliary.Response {
	validated, failure := validateBenchmarkRequest(request)
	if failure != nil {
		return octxiliary.OkValue(id, kaijuvulkan.DispatchResponseValue(*failure))
	}
	result := execute(validated)
	return octxiliary.OkValue(id, kaijuvulkan.DispatchResponseValue(result))
}
