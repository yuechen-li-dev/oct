using Godot;
using System.Diagnostics;
using System.Text.Json;
using GodotFile = Godot.FileAccess;

public partial class BenchmarkHost : Node
{
    private sealed record Request(int SchemaVersion, string SpirvPath, string SpirvHash, string EntryPoint,
        string BenchmarkId, string ReplayId, uint[] WorkgroupSize, uint[] DispatchGroups, uint Warmup, uint Iterations);
    private sealed record Device(string Name, string Vendor, string GodotVersion, string Backend);
    private sealed record Response(int SchemaVersion, bool Success, Device? Device, ulong[] SamplesNs,
        string TimingSource, string[] TimingIncludes, string[] Warnings, string? Error);

    public override void _Ready()
    {
        var args = OS.GetCmdlineUserArgs();
        var requestPath = ValueAfter(args, "--request");
        var responsePath = ValueAfter(args, "--response");
        Response response;
        try { response = Run(requestPath); }
        catch (Exception e) { response = new Response(1, false, null, [], "", [], [], e.Message); }
        if (string.IsNullOrEmpty(responsePath)) { GD.PushError("--response is required"); GetTree().Quit(2); return; }
        GodotFile.Open(responsePath, GodotFile.ModeFlags.Write)?.StoreString(JsonSerializer.Serialize(response));
        GetTree().Quit(response.Success ? 0 : 1);
    }

    private static Response Run(string? requestPath)
    {
        if (string.IsNullOrEmpty(requestPath) || !GodotFile.FileExists(requestPath)) throw new InvalidOperationException("benchmark request file is missing");
        var request = JsonSerializer.Deserialize<Request>(GodotFile.GetFileAsString(requestPath), new JsonSerializerOptions { PropertyNameCaseInsensitive = true }) ?? throw new InvalidOperationException("malformed benchmark request");
        if (request.SchemaVersion != 1 || request.Warmup == 0 || request.Iterations == 0 || request.DispatchGroups.Length != 3 || request.DispatchGroups.Any(v => v == 0)) throw new InvalidOperationException("invalid benchmark request metadata");
        if (!GodotFile.FileExists(request.SpirvPath)) throw new InvalidOperationException("SPIR-V file is missing");
        // Compatibility/OpenGL cannot create RD. The project asks Forward+; this
        // guard makes a driver fallback explicit rather than silently timing CPU work.
        var rd = RenderingServer.CreateLocalRenderingDevice();
        if (rd is null) throw new InvalidOperationException("RenderingDevice creation failed; run without --headless using a Vulkan/Forward+ capable renderer");
        Rid shader = default, pipeline = default;
        try
        {
            var spirv = new RDShaderSpirV { BytecodeCompute = GodotFile.GetFileAsBytes(request.SpirvPath) };
            shader = rd.ShaderCreateFromSpirV(spirv);
            if (!shader.IsValid) throw new InvalidOperationException("RenderingDevice rejected SPIR-V compute shader");
            pipeline = rd.ComputePipelineCreate(shader);
            if (!pipeline.IsValid) throw new InvalidOperationException("RenderingDevice failed to create compute pipeline");
            for (var i = 0u; i < request.Warmup; i++) Dispatch(rd, pipeline, request.DispatchGroups);
            var samples = new ulong[request.Iterations];
            for (var i = 0; i < samples.Length; i++)
            {
                var timer = Stopwatch.StartNew(); Dispatch(rd, pipeline, request.DispatchGroups); timer.Stop();
                samples[i] = (ulong)(timer.ElapsedTicks * 1_000_000_000L / Stopwatch.Frequency);
            }
            return new Response(1, true, new Device(rd.GetDeviceName(), rd.GetDeviceVendorName(), Engine.GetVersionInfo()["string"].ToString(), "RenderingDevice"), samples,
                "synchronized_host_elapsed", ["compute dispatch", "queue submit", "GPU synchronization"], [], null);
        }
        finally { if (pipeline.IsValid) rd.FreeRid(pipeline); if (shader.IsValid) rd.FreeRid(shader); rd.Free(); }
    }
    private static void Dispatch(RenderingDevice rd, Rid pipeline, uint[] groups)
    {
        var list = rd.ComputeListBegin(); rd.ComputeListBindComputePipeline(list, pipeline); rd.ComputeListDispatch(list, groups[0], groups[1], groups[2]); rd.ComputeListEnd(); rd.Submit(); rd.Sync();
    }
    private static string? ValueAfter(string[] args, string key) { for (var i=0;i+1<args.Length;i++) if (args[i]==key) return args[i+1]; return null; }
}
