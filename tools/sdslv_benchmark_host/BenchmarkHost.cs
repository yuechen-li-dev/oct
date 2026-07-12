using Godot;
using System.Diagnostics;
using System.Text.Json;
using GodotFile = Godot.FileAccess;

public partial class BenchmarkHost : Node
{
    private sealed record Request(int SchemaVersion, string SpirvPath, string SpirvHash, string EntryPoint,
        string BenchmarkId, string ReplayId, uint[] WorkgroupSize, uint[] DispatchGroups, uint Warmup, uint Iterations, Resource[] Resources);
    private sealed record Resource(uint Set, uint Binding, string Access, string ElementType, uint ByteLength, string PayloadBase64, bool Readback, string? SentinelBase64);
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
        Rid shader = default, pipeline = default; var buffers = new List<Rid>(); var sets = new List<Rid>();
        try
        {
            var spirv = new RDShaderSpirV { BytecodeCompute = GodotFile.GetFileAsBytes(request.SpirvPath) };
            shader = rd.ShaderCreateFromSpirV(spirv);
            if (!shader.IsValid) throw new InvalidOperationException("RenderingDevice rejected SPIR-V compute shader");
            pipeline = rd.ComputePipelineCreate(shader);
            if (!pipeline.IsValid) throw new InvalidOperationException("RenderingDevice failed to create compute pipeline");
            foreach (var group in (request.Resources ?? []).GroupBy(r => r.Set))
            {
                var uniforms = new Godot.Collections.Array<RDUniform>();
                foreach (var resource in group.OrderBy(r => r.Binding))
                {
                    if (resource.ElementType is not ("u32" or "f32" or "float2" or "float4") || resource.ByteLength == 0) throw new InvalidOperationException("unsupported benchmark storage resource");
                    var payload = Convert.FromBase64String(resource.PayloadBase64 ?? ""); if (payload.Length != resource.ByteLength) throw new InvalidOperationException("storage payload byte length mismatch");
                    var buffer = rd.StorageBufferCreate(resource.ByteLength, payload); if (!buffer.IsValid) throw new InvalidOperationException("storage buffer creation failed"); buffers.Add(buffer);
                    var uniform = new RDUniform { UniformType = RenderingDevice.UniformType.StorageBuffer, Binding = (int)resource.Binding }; uniform.AddId(buffer); uniforms.Add(uniform);
                }
                var set = rd.UniformSetCreate(uniforms, shader, group.Key); if (!set.IsValid) throw new InvalidOperationException("uniform set creation failed"); sets.Add(set);
            }
            for (var i = 0u; i < request.Warmup; i++) Dispatch(rd, pipeline, request.DispatchGroups, sets);
            var samples = new ulong[request.Iterations];
            for (var i = 0; i < samples.Length; i++)
            {
                var timer = Stopwatch.StartNew(); Dispatch(rd, pipeline, request.DispatchGroups, sets); timer.Stop();
                samples[i] = (ulong)(timer.ElapsedTicks * 1_000_000_000L / Stopwatch.Frequency);
            }
            return new Response(1, true, new Device(rd.GetDeviceName(), rd.GetDeviceVendorName(), Engine.GetVersionInfo()["string"].ToString(), "RenderingDevice"), samples,
                "synchronized_host_elapsed", ["compute dispatch", "queue submit", "GPU synchronization"], [], null);
        }
        finally { foreach (var set in sets) if (set.IsValid) rd.FreeRid(set); foreach (var buffer in buffers) if (buffer.IsValid) rd.FreeRid(buffer); if (pipeline.IsValid) rd.FreeRid(pipeline); if (shader.IsValid) rd.FreeRid(shader); rd.Free(); }
    }
    private static void Dispatch(RenderingDevice rd, Rid pipeline, uint[] groups, IEnumerable<Rid> sets)
    {
        var list = rd.ComputeListBegin(); rd.ComputeListBindComputePipeline(list, pipeline); uint index=0; foreach(var set in sets) rd.ComputeListBindUniformSet(list, set, index++); rd.ComputeListDispatch(list, groups[0], groups[1], groups[2]); rd.ComputeListEnd(); rd.Submit(); rd.Sync();
    }
    private static string? ValueAfter(string[] args, string key) { for (var i=0;i+1<args.Length;i++) if (args[i]==key) return args[i+1]; return null; }
}
