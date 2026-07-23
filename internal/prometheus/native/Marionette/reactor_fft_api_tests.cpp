#include "test_harness.h"

#include "../reactor_api.h"
#include "../reactor_vulkan.h"
#include "../reactor_shader_package.h"

#include <algorithm>
#include <cmath>
#include <cstdio>
#include <cstdint>
#include <cstring>
#include <filesystem>
#include <limits>
#include <string>
#include <vector>

namespace {

struct RayQueryOracleHit
{
    std::uint32_t hit = 0u, kind = 0u, primitive = UINT32_MAX;
    double t = -1.0;
    double position[3]{};
    double normal[3]{};
    double bary[2]{-1.0, -1.0};
    std::uint32_t frontFace = UINT32_MAX;
};

struct RayQueryErrorStats { double t=0.0, position=0.0, normal=0.0, barycentric=0.0; const char* tName=""; const char* positionName=""; const char* normalName=""; const char* baryName=""; };
static RayQueryErrorStats g_ray_query_error_stats;
static void rq_record_error(double value, double* maximum, const char** fixture, const char* name) { if (value > *maximum) { *maximum=value; *fixture=name; } }

static double rq_dot(const double a[3], const double b[3]) { return a[0]*b[0] + a[1]*b[1] + a[2]*b[2]; }
static void rq_cross(const double a[3], const double b[3], double out[3]) { out[0]=a[1]*b[2]-a[2]*b[1]; out[1]=a[2]*b[0]-a[0]*b[2]; out[2]=a[0]*b[1]-a[1]*b[0]; }
static void rq_normalize(double v[3]) { const double length = std::sqrt(rq_dot(v,v)); if (length != 0.0) { v[0]/=length; v[1]/=length; v[2]/=length; } }

// Independent double-precision authority. This deliberately has no shared
// shader code or production dependency: Moller-Trumbore for triangles and a
// full quadratic for analytic spheres. Intervals are closed and ties retain
// the earlier triangle fixture order; corpus fixtures avoid cross-kind ties.
static RayQueryOracleHit ray_query_oracle(const PrometheusRayQueryTriangle* triangles, std::uint32_t triangleCount,
                                          const PrometheusRayQuerySphere* spheres, std::uint32_t sphereCount,
                                          const PrometheusRayQueryRawRequest& ray)
{
    RayQueryOracleHit best;
    const double origin[3] = {ray.origin[0], ray.origin[1], ray.origin[2]};
    const double direction[3] = {ray.direction[0], ray.direction[1], ray.direction[2]};
    auto accept = [&](std::uint32_t kind, std::uint32_t primitive, double t, const double normal[3], const double bary[2], std::uint32_t front) {
        if (t < static_cast<double>(ray.t_min) || t > static_cast<double>(ray.t_max) || (best.hit && t >= best.t)) return;
        best.hit=1u; best.kind=kind; best.primitive=primitive; best.t=t; best.frontFace=front;
        for (int k=0;k<3;++k) { best.position[k]=origin[k]+t*direction[k]; best.normal[k]=normal[k]; }
        best.bary[0]=bary[0]; best.bary[1]=bary[1];
    };
    for (std::uint32_t i=0u;i<triangleCount;++i) {
        const float* p=triangles[i].positions;
        const double a[3]={p[0],p[1],p[2]}, e1[3]={p[3]-p[0],p[4]-p[1],p[5]-p[2]}, e2[3]={p[6]-p[0],p[7]-p[1],p[8]-p[2]};
        double pv[3], tv[3], qv[3]; rq_cross(direction,e2,pv); const double determinant=rq_dot(e1,pv);
        if (std::fabs(determinant) <= 1.0e-12) continue;
        const double inverse=1.0/determinant; for(int k=0;k<3;++k) tv[k]=origin[k]-a[k]; const double u=rq_dot(tv,pv)*inverse;
        if (u < 0.0 || u > 1.0) continue;
        rq_cross(tv,e1,qv); const double v=rq_dot(direction,qv)*inverse;
        if (v < 0.0 || u+v > 1.0) continue;
        const double t=rq_dot(e2,qv)*inverse; double n[3]; rq_cross(e1,e2,n); rq_normalize(n); const double bary[2]={u,v};
        accept(1u,i,t,n,bary,rq_dot(n,direction)<0.0 ? 1u : 0u);
    }
    for (std::uint32_t i=0u;i<sphereCount;++i) {
        const double center[3]={spheres[i].center[0],spheres[i].center[1],spheres[i].center[2]}; double oc[3]; for(int k=0;k<3;++k) oc[k]=origin[k]-center[k];
        const double qa=rq_dot(direction,direction), qb=rq_dot(oc,direction), qc=rq_dot(oc,oc)-static_cast<double>(spheres[i].radius)*spheres[i].radius;
        const double discriminant=qb*qb-qa*qc; if (discriminant < 0.0) continue;
        const double root=std::sqrt(discriminant), nearT=(-qb-root)/qa, farT=(-qb+root)/qa;
        const double t=(nearT >= ray.t_min && nearT <= ray.t_max) ? nearT : farT;
        if (t < ray.t_min || t > ray.t_max) continue;
        double position[3], n[3]; for(int k=0;k<3;++k) { position[k]=origin[k]+t*direction[k]; n[k]=position[k]-center[k]; } rq_normalize(n); const double bary[2]={-1.0,-1.0};
        accept(2u,i,t,n,bary,UINT32_MAX);
    }
    return best;
}

static bool rq_near(double expected, double actual) {
    constexpr double absoluteTolerance=2.0e-5, relativeTolerance=2.0e-5;
    const double delta=std::fabs(expected-actual);
    return delta <= absoluteTolerance || delta <= relativeTolerance*std::fabs(expected);
}
static std::uint32_t rq_u32_from_float(float value) { std::uint32_t bits=0u; std::memcpy(&bits,&value,sizeof(bits)); return bits; }
static std::string ray_query_shader_package_root() {
    std::filesystem::path cursor = std::filesystem::current_path();
    for (;;) {
        const std::filesystem::path candidate = cursor / "out" / "prometheus" / "native" / "SerialCanonical" / "shaders";
        if (std::filesystem::exists(candidate / "manifest.json")) return candidate.string();
        if (cursor == cursor.root_path()) break;
        cursor = cursor.parent_path();
    }
    return {};
}
static PrometheusReactorConfig raw_hit_runtime_config() {
    static const std::string root = ray_query_shader_package_root();
    PrometheusReactorConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config));
    config.shader_package_root = root.c_str();
    return config;
}

static void assert_ray_query_matches_oracle(::marionette::tests::TestContext& context, void* handle, const char* name,
                                             const PrometheusRayQueryTriangle* triangles, std::uint32_t triangleCount,
                                             const PrometheusRayQuerySphere* spheres, std::uint32_t sphereCount,
                                             const PrometheusRayQueryRawRequest& ray)
{
    PrometheusRayQuerySceneCreateRequest create{}; create.struct_size=static_cast<std::uint32_t>(sizeof(create)); create.triangles=triangles; create.triangle_count=triangleCount; create.spheres=spheres; create.sphere_count=sphereCount;
    const RayQueryOracleHit expected=ray_query_oracle(triangles,triangleCount,spheres,sphereCount,ray);
    std::uint64_t scene=0u; ASSERT_EQUAL(PROM_OK,prometheus_reactor_runtime_ray_query_scene_create(handle,&create,&scene),std::string(name)+" scene create");
    PrometheusRayQueryRawHit actual{}; ASSERT_EQUAL(PROM_OK,prometheus_reactor_runtime_ray_query_scene_trace(handle,scene,&ray,&actual),std::string(name)+" trace");
    ASSERT_EQUAL(expected.hit,actual.meta[0],std::string(name)+" hit"); ASSERT_EQUAL(expected.kind,actual.meta[1],std::string(name)+" kind");
    if (expected.hit) {
        ASSERT_EQUAL(expected.kind==1u?0u:1u,actual.meta[2],std::string(name)+" instance"); ASSERT_EQUAL(0u,actual.meta[3],std::string(name)+" geometry");
        ASSERT_EQUAL(expected.primitive,rq_u32_from_float(actual.t_primitive[1]),std::string(name)+" primitive");
        ASSERT_TRUE(rq_near(expected.t,actual.t_primitive[0]),std::string(name)+" t");
        rq_record_error(std::fabs(expected.t-actual.t_primitive[0]),&g_ray_query_error_stats.t,&g_ray_query_error_stats.tName,name);
        for(int k=0;k<3;++k) { rq_record_error(std::fabs(expected.position[k]-actual.position[k]),&g_ray_query_error_stats.position,&g_ray_query_error_stats.positionName,name); rq_record_error(std::fabs(expected.normal[k]-actual.normal[k]),&g_ray_query_error_stats.normal,&g_ray_query_error_stats.normalName,name); ASSERT_TRUE(rq_near(expected.position[k],actual.position[k]),std::string(name)+" position"); ASSERT_TRUE(rq_near(expected.normal[k],actual.normal[k]),std::string(name)+" normal"); }
        if (expected.kind==1u) { rq_record_error(std::fabs(expected.bary[0]-actual.barycentrics[0]),&g_ray_query_error_stats.barycentric,&g_ray_query_error_stats.baryName,name); rq_record_error(std::fabs(expected.bary[1]-actual.barycentrics[1]),&g_ray_query_error_stats.barycentric,&g_ray_query_error_stats.baryName,name); ASSERT_TRUE(rq_near(expected.bary[0],actual.barycentrics[0]),std::string(name)+" bary u"); ASSERT_TRUE(rq_near(expected.bary[1],actual.barycentrics[1]),std::string(name)+" bary v"); ASSERT_EQUAL(expected.frontFace,static_cast<std::uint32_t>(actual.t_primitive[2]),std::string(name)+" front face"); }
        if (expected.kind==2u) { ASSERT_NEAR(spheres[expected.primitive].albedo[0],actual.albedo_material[0],0.0f,std::string(name)+" albedo r"); ASSERT_NEAR(spheres[expected.primitive].albedo[1],actual.albedo_material[1],0.0f,std::string(name)+" albedo g"); ASSERT_NEAR(spheres[expected.primitive].albedo[2],actual.albedo_material[2],0.0f,std::string(name)+" albedo b"); ASSERT_NEAR(static_cast<float>(spheres[expected.primitive].material_id),actual.albedo_material[3],0.0f,std::string(name)+" material"); }
    } else { ASSERT_EQUAL(UINT32_MAX,actual.meta[2],std::string(name)+" miss instance sentinel"); ASSERT_EQUAL(UINT32_MAX,actual.meta[3],std::string(name)+" miss geometry sentinel"); ASSERT_NEAR(-1.0f,actual.t_primitive[0],0.0f,std::string(name)+" miss t sentinel"); }
    ASSERT_EQUAL(PROM_OK,prometheus_reactor_runtime_ray_query_scene_destroy(handle,scene),std::string(name)+" scene destroy");
}

} // namespace

FACT(PrometheusReactor_FftDiagnosticsDefaultState)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");

    PrometheusFftDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft_diagnostics(handle, &diag), "fft diagnostics should succeed");
    ASSERT_EQUAL(1u, diag.api_declared, "fft api should be declared");
    ASSERT_EQUAL(0u, diag.capability_reported, "fft capability must remain unclaimed");
    ASSERT_EQUAL(0u, diag.production_enabled, "fft production path must remain disabled");
    ASSERT_EQUAL(0u, diag.benchmark_enabled, "fft benchmark path defaults off");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_FFT_PATH_UNAVAILABLE), diag.executed_path_id, "executed path should default unavailable");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}


FACT(PrometheusReactor_FftVkServiceSeamRejectsInvalidAndDestroyedHandle)
{
    prom_vk_runtime_services services{};
    ASSERT_EQUAL(PROM_INVALID_HANDLE,
                 prom_reactor_runtime_get_vk_services(nullptr, &services),
                 "null handle should be rejected");

    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");

    ASSERT_EQUAL(PROM_INVALID_HANDLE,
                 prom_reactor_runtime_get_vk_services(handle, &services),
                 "destroyed handle should be rejected");
}

FACT(PrometheusReactor_FftVkServiceSeamReportsAvailabilityTruthfully)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");

    prom_vk_runtime_services services{};
    int status = prom_reactor_runtime_get_vk_services(handle, &services);
    if (status == PROM_OK)
    {
        ASSERT_EQUAL(1u, services.backend_available, "available runtime should report available backend");
        ASSERT_TRUE(services.device != VK_NULL_HANDLE, "service seam should expose device");
        ASSERT_TRUE(services.compute_queue != VK_NULL_HANDLE, "service seam should expose compute queue");
        ASSERT_TRUE(services.compute_command_pool != VK_NULL_HANDLE, "service seam should expose command pool");
    }
    else
    {
        ASSERT_EQUAL(PROM_ERROR, status, "unavailable backend should return PROM_ERROR");
        ASSERT_EQUAL(0u, services.backend_available, "unavailable runtime should report unavailable backend");
        ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_REASON_VULKAN_UNAVAILABLE), services.backend_reason_code,
                     "unavailable runtime should report vulkan unavailable reason");
    }

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

static PrometheusFftRequest make_valid_req(PrometheusComplex32* in, PrometheusComplex32* out)
{
    PrometheusFftRequest req{};
    req.struct_size = static_cast<std::uint32_t>(sizeof(req));
    req.input = in;
    req.output = out;
    req.element_count = 2u;
    req.batch_count = 1u;
    req.stride_elements = 0u;
    req.flags = 0u;
    return req;
}

FACT(PrometheusReactor_FftExecutionDispatchesRadix2)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");
    PrometheusComplex32 in[2]{{1.0f, 0.0f}, {2.0f, 0.0f}};
    PrometheusComplex32 out[2]{};
    PrometheusFftRequest req = make_valid_req(in, out);

    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft(handle, &req, &stage, &detail), "fft should dispatch through Vulkan");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_STAGE_TRANSFER_OUT), stage, "fft should finish with a readback stage");
    ASSERT_EQUAL(0, detail, "successful fft should not retain a failure detail");
    ASSERT_NEAR(3.0f, out[0].real, 1.0e-5f, "radix-2 GPU dc bin");
    ASSERT_NEAR(-1.0f, out[1].real, 1.0e-5f, "radix-2 GPU Nyquist bin");

    PrometheusFftDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft_diagnostics(handle, &diag), "fft diagnostics should succeed");
    ASSERT_EQUAL(2u, diag.last_element_count, "diag should snapshot element count");
    ASSERT_EQUAL(1u, diag.last_batch_count, "diag should snapshot batch count");
    ASSERT_EQUAL(1u, diag.plan_valid, "valid request should build deterministic plan metadata");
    ASSERT_EQUAL(1u, diag.plan_pass_count, "n=2 should require one radix-2 pass");
    ASSERT_EQUAL(2u, diag.plan_first_span, "first span should be 2");
    ASSERT_EQUAL(2u, diag.plan_last_span, "last span should be 2 for n=2");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_FFT_PATH_VULKAN_RADIX2_RESERVED), diag.executed_path_id, "diag must identify the dispatched Vulkan route");

    PrometheusComplex32 multiIn[4]{{1.0f, 0.0f}, {2.0f, 0.0f}, {3.0f, 0.0f}, {4.0f, 0.0f}};
    PrometheusComplex32 multiOut[4]{};
    req.input = multiIn; req.output = multiOut; req.element_count = 4u;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft(handle, &req, &stage, &detail), "two-pass N=4 FFT should execute");
    ASSERT_NEAR(10.0f, multiOut[0].real, 2.0e-5f, "N=4 DC");
    ASSERT_NEAR(-2.0f, multiOut[1].real, 2.0e-5f, "N=4 bin one real");
    ASSERT_NEAR(2.0f, multiOut[1].imag, 2.0e-5f, "N=4 bin one imag uses forward negative sign");
    ASSERT_NEAR(-2.0f, multiOut[2].real, 2.0e-5f, "N=4 Nyquist");
    ASSERT_NEAR(-2.0f, multiOut[3].real, 2.0e-5f, "N=4 bin three real");
    ASSERT_NEAR(-2.0f, multiOut[3].imag, 2.0e-5f, "N=4 bin three imag uses forward negative sign");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_RayQueryAdmissionIsOptionalAndCompleteWhenEnabled)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");

    prom_vk_runtime_services services{};
    int status = prom_reactor_runtime_get_vk_services(handle, &services);
    if (status == PROM_OK)
    {
        if (services.ray_query_state == PROM_VK_RAY_QUERY_DEVICE_FEATURE_ENABLED)
        {
            ASSERT_EQUAL(1u, services.ray_query_acceleration_structure_extension_supported, "enabled ray query requires acceleration-structure extension");
            ASSERT_EQUAL(1u, services.ray_query_extension_supported, "enabled ray query requires ray-query extension");
            ASSERT_EQUAL(1u, services.ray_query_deferred_host_operations_extension_supported, "enabled ray query requires deferred-host-operations extension");
            ASSERT_EQUAL(1u, services.ray_query_buffer_device_address_supported, "enabled ray query requires buffer device address");
            ASSERT_EQUAL(1u, services.ray_query_acceleration_structure_supported, "enabled ray query requires acceleration structures");
            ASSERT_EQUAL(1u, services.ray_query_supported, "enabled ray query requires rayQuery feature");
            ASSERT_TRUE(services.create_acceleration_structure != nullptr, "enabled ray query loads create entry point");
            ASSERT_TRUE(services.destroy_acceleration_structure != nullptr, "enabled ray query loads destroy entry point");
            ASSERT_TRUE(services.get_acceleration_structure_build_sizes != nullptr, "enabled ray query loads build-size entry point");
            ASSERT_TRUE(services.cmd_build_acceleration_structures != nullptr, "enabled ray query loads build entry point");
            ASSERT_TRUE(services.get_acceleration_structure_device_address != nullptr, "enabled ray query loads address entry point");
        }
        else
        {
            ASSERT_TRUE(services.ray_query_state == PROM_VK_RAY_QUERY_UNSUPPORTED ||
                        services.ray_query_state == PROM_VK_RAY_QUERY_EXTENSION_MISSING ||
                        services.ray_query_state == PROM_VK_RAY_QUERY_FEATURE_MISSING ||
                        services.ray_query_state == PROM_VK_RAY_QUERY_ENTRY_POINT_MISSING,
                        "unsupported ray-query admission must retain a precise optional state");
        }
    }

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_RayQueryTriangleBlasTlasProbeUsesPersistentHardwareTraversal)
{
    void* handle = nullptr;
    PrometheusReactorConfig config = raw_hit_runtime_config();
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create should succeed");

    prom_vk_runtime_services services{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_get_vk_services(handle, &services), "runtime services should be available");
    if (services.ray_query_state != PROM_VK_RAY_QUERY_DEVICE_FEATURE_ENABLED)
    {
        SKIP("ray-query feature bundle is unavailable on this device");
    }

    const PrometheusRayQueryTriangle triangles[] = {{
        {-1.0f, -1.0f, 2.0f,
          1.0f, -1.0f, 2.0f,
          0.0f,  1.0f, 2.0f}
    }};
    PrometheusRayQueryTriangleSceneCreateRequest request{};
    request.struct_size = static_cast<std::uint32_t>(sizeof(request));
    request.triangles = triangles;
    request.triangle_count = 1u;
    std::uint64_t sceneId = 0u;
    ASSERT_EQUAL(PROM_OK,
                 prometheus_reactor_runtime_ray_query_triangle_scene_create(handle, &request, &sceneId),
                 "triangle scene should build persistent BLAS and TLAS");
    ASSERT_TRUE(sceneId != 0u, "created scene must have a non-zero opaque handle");

    PrometheusRayQueryProbeResult first{};
    ASSERT_EQUAL(PROM_OK,
                 prometheus_reactor_runtime_ray_query_triangle_scene_probe(handle, sceneId, &first),
                 "first probe must dispatch the production SDSL-V ray-query shader");
    ASSERT_EQUAL(1u, first.hit, "canonical M0 ray should hit the z=2 triangle");
    ASSERT_EQUAL(1u, first.triangle_count, "scene retains its triangle count");
    ASSERT_EQUAL(1u, first.blas_built, "triangle BLAS must remain live for warm probes");
    ASSERT_EQUAL(1u, first.tlas_built, "TLAS must remain live for warm probes");
    ASSERT_TRUE(first.vertex_device_address != 0u, "triangle geometry must be device-addressable");
    ASSERT_TRUE(first.blas_device_address != 0u, "BLAS must expose a device address to TLAS build");
    ASSERT_TRUE(first.tlas_device_address != 0u, "TLAS must expose a device address to traversal");

    PrometheusRayQueryProbeResult warm{};
    ASSERT_EQUAL(PROM_OK,
                 prometheus_reactor_runtime_ray_query_triangle_scene_probe(handle, sceneId, &warm),
                 "warm probe should reuse scene acceleration structures and pipeline");
    ASSERT_EQUAL(1u, warm.hit, "warm probe must retain the hardware traversal result");
    ASSERT_EQUAL(first.blas_device_address, warm.blas_device_address, "warm probe must retain the same BLAS");
    ASSERT_EQUAL(first.tlas_device_address, warm.tlas_device_address, "warm probe must retain the same TLAS");

    prom_vk_runtime_services afterProbe{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_get_vk_services(handle, &afterProbe), "services should remain queryable after traversal");
    ASSERT_EQUAL(services.validation_error_count, afterProbe.validation_error_count,
                 "BLAS/TLAS build and traversal must introduce no validation errors");

    ASSERT_EQUAL(PROM_OK,
                 prometheus_reactor_runtime_ray_query_triangle_scene_destroy(handle, sceneId),
                 "scene destroy must release acceleration structures before backing buffers");
    ASSERT_EQUAL(PROM_INVALID_HANDLE,
                 prometheus_reactor_runtime_ray_query_triangle_scene_probe(handle, sceneId, &warm),
                 "destroyed scene handles must be rejected");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_RayQueryTriangleScenesRemainIsolated)
{
    void* handle = nullptr;
    PrometheusReactorConfig config = raw_hit_runtime_config();
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create should succeed");
    prom_vk_runtime_services services{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_get_vk_services(handle, &services), "runtime services should be available");
    if (services.ray_query_state != PROM_VK_RAY_QUERY_DEVICE_FEATURE_ENABLED)
    {
        SKIP("ray-query feature bundle is unavailable on this device");
    }

    const PrometheusRayQueryTriangle hitTriangle[] = {{
        {-1.0f, -1.0f, 2.0f, 1.0f, -1.0f, 2.0f, 0.0f, 1.0f, 2.0f}
    }};
    const PrometheusRayQueryTriangle missTriangle[] = {{
        {10.0f, -1.0f, 2.0f, 12.0f, -1.0f, 2.0f, 11.0f, 1.0f, 2.0f}
    }};
    PrometheusRayQueryTriangleSceneCreateRequest hitRequest{};
    hitRequest.struct_size = static_cast<std::uint32_t>(sizeof(hitRequest));
    hitRequest.triangles = hitTriangle;
    hitRequest.triangle_count = 1u;
    PrometheusRayQueryTriangleSceneCreateRequest missRequest = hitRequest;
    missRequest.triangles = missTriangle;
    std::uint64_t hitScene = 0u;
    std::uint64_t missScene = 0u;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_ray_query_triangle_scene_create(handle, &hitRequest, &hitScene),
                 "first live scene should build");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_ray_query_triangle_scene_create(handle, &missRequest, &missScene),
                 "second live scene should build independently");
    PrometheusRayQueryProbeResult hit{};
    PrometheusRayQueryProbeResult miss{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_ray_query_triangle_scene_probe(handle, hitScene, &hit),
                 "hit scene should dispatch");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_ray_query_triangle_scene_probe(handle, missScene, &miss),
                 "miss scene should dispatch");
    ASSERT_EQUAL(1u, hit.hit, "first scene must retain its own hit geometry");
    ASSERT_EQUAL(0u, miss.hit, "second scene must retain its own miss geometry");
    ASSERT_NOT_EQUAL(hit.tlas_device_address, miss.tlas_device_address, "live scenes must not alias their TLAS objects");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_ray_query_triangle_scene_destroy(handle, missScene),
                 "second scene destroy should succeed");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_ray_query_triangle_scene_destroy(handle, hitScene),
                 "first scene destroy should succeed");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_RayQueryRawSphereAndTriangleTraversal)
{
    void* handle = nullptr;
    PrometheusReactorConfig config = raw_hit_runtime_config();
    ASSERT_TRUE(config.shader_package_root != nullptr && config.shader_package_root[0] != '\0', "raw-hit test receives staged shader package root");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&config, &handle), "runtime create should succeed");
    prom_vk_runtime_services services{};
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_get_vk_services(handle, &services), "runtime services should be available");
    if (services.ray_query_state != PROM_VK_RAY_QUERY_DEVICE_FEATURE_ENABLED)
    {
        SKIP("ray-query feature bundle is unavailable on this device");
    }
    const PrometheusRayQueryTriangle triangles[] = {{{-1.0f, -1.0f, 4.0f, 1.0f, -1.0f, 4.0f, 0.0f, 1.0f, 4.0f}}};
    const PrometheusRayQuerySphere spheres[] = {{{0.0f, 0.0f, 3.0f}, 1.0f, {0.25f, 0.5f, 0.75f}, 7u}};
    PrometheusRayQuerySceneCreateRequest sceneRequest{};
    sceneRequest.struct_size = static_cast<std::uint32_t>(sizeof(sceneRequest));
    sceneRequest.triangles = triangles; sceneRequest.triangle_count = 1u;
    sceneRequest.spheres = spheres; sceneRequest.sphere_count = 1u;
    prom_shader_package* package = nullptr;
    ASSERT_EQUAL(PROM_OK, prom_reactor_runtime_get_shader_package(handle, &package), "raw-hit runtime owns a package");
    const std::uint64_t objectOpensBeforePipelineCreate = prom_shader_package_artifact_open_count(package);
    std::uint64_t scene = 0u;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_ray_query_scene_create(handle, &sceneRequest, &scene), "mixed scene must build triangle and procedural BLAS before TLAS");
    const std::uint64_t objectOpensAfterPipelineCreate = prom_shader_package_artifact_open_count(package);
    ASSERT_EQUAL(objectOpensBeforePipelineCreate + 1u, objectOpensAfterPipelineCreate,
                 "raw-hit pipeline opens exactly its verified external object once");
    PrometheusRayQueryRawRequest ray{};
    ray.struct_size = static_cast<std::uint32_t>(sizeof(ray));
    ray.origin[0] = 0.0f; ray.origin[1] = 0.0f; ray.origin[2] = 0.0f; ray.t_min = 0.0f;
    ray.direction[0] = 0.0f; ray.direction[1] = 0.0f; ray.direction[2] = 1.0f; ray.t_max = 100.0f;
    PrometheusRayQueryRawHit hit{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_ray_query_scene_trace(handle, scene, &ray, &hit), "stateful production traversal should dispatch");
    ASSERT_EQUAL(1u, hit.meta[0], "analytic sphere must commit a hit");
    ASSERT_EQUAL(2u, hit.meta[1], "nearest procedural candidate must be reported as a sphere");
    ASSERT_NEAR(2.0f, hit.t_primitive[0], 2.0e-5f, "sphere entry root must be committed");
    ASSERT_NEAR(0.0f, hit.position[0], 2.0e-5f, "sphere x position");
    ASSERT_NEAR(0.0f, hit.position[1], 2.0e-5f, "sphere y position");
    ASSERT_NEAR(2.0f, hit.position[2], 2.0e-5f, "sphere z position");
    ASSERT_NEAR(0.0f, hit.normal[0], 2.0e-5f, "analytic sphere normal x");
    ASSERT_NEAR(0.0f, hit.normal[1], 2.0e-5f, "analytic sphere normal y");
    ASSERT_NEAR(-1.0f, hit.normal[2], 2.0e-5f, "analytic sphere normal z");
    PrometheusRayQueryRawHit warm{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_ray_query_scene_trace(handle, scene, &ray, &warm), "warm traversal must reuse the persistent AS");
    ASSERT_NEAR(hit.t_primitive[0], warm.t_primitive[0], 0.0f, "warm trace retains committed t");
    ASSERT_EQUAL(objectOpensAfterPipelineCreate, prom_shader_package_artifact_open_count(package), "warm raw-hit dispatch does not reopen or rehash the object");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_ray_query_scene_destroy(handle, scene), "mixed scene destroy should release TLAS before both BLAS objects");
    ASSERT_EQUAL(PROM_INVALID_HANDLE, prometheus_reactor_runtime_ray_query_scene_trace(handle, scene, &ray, &warm), "destroyed mixed scene must be rejected");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusReactor_RayQueryRawNumericalCorpusAgreesWithDoubleOracle)
{
    PrometheusReactorConfig config = raw_hit_runtime_config();
    ASSERT_TRUE(config.shader_package_root != nullptr && config.shader_package_root[0] != '\0', "raw corpus receives staged shader package root");
    void* handle=nullptr; ASSERT_EQUAL(PROM_OK,prometheus_reactor_runtime_create(&config,&handle),"runtime create");
    prom_vk_runtime_services services{}; ASSERT_EQUAL(PROM_OK,prom_reactor_runtime_get_vk_services(handle,&services),"services");
    if (services.ray_query_state != PROM_VK_RAY_QUERY_DEVICE_FEATURE_ENABLED) { SKIP("ray-query feature bundle is unavailable on this device"); }
    g_ray_query_error_stats = {};
    const PrometheusRayQueryTriangle triangle[] = {{{-1.0f,-1.0f,2.0f, 1.0f,-1.0f,2.0f, 0.0f,1.0f,2.0f}}};
    const PrometheusRayQueryTriangle depthTriangles[] = {{{-1.0f,-1.0f,2.0f,1.0f,-1.0f,2.0f,0.0f,1.0f,2.0f}},{{-1.0f,-1.0f,6.0f,1.0f,-1.0f,6.0f,0.0f,1.0f,6.0f}}};
    const PrometheusRayQuerySphere sphere[] = {{{0.0f,0.0f,4.0f},1.0f,{0.2f,0.4f,0.6f},3u}};
    const PrometheusRayQuerySphere overlap[] = {{{0.0f,0.0f,5.0f},1.5f,{1,0,0},1u},{{0.0f,0.0f,4.0f},1.0f,{0,1,0},2u}};
    const PrometheusRayQuerySphere aabbFalse[] = {{{0.9f,0.9f,4.0f},1.0f,{1,0,0},1u}};
    const PrometheusRayQuerySphere tangent[] = {{{1.0f,0.0f,4.0f},1.0f,{1,0,0},1u}};
    const PrometheusRayQuerySphere nearTangentHit[] = {{{0.999f,0.0f,4.0f},1.0f,{1,0,0},1u}};
    const PrometheusRayQuerySphere nearTangentMiss[] = {{{1.001f,0.0f,4.0f},1.0f,{1,0,0},1u}};
    auto ray = [](float ox,float oy,float oz,float dx,float dy,float dz,float tmin,float tmax) { PrometheusRayQueryRawRequest r{}; r.struct_size=static_cast<std::uint32_t>(sizeof(r)); r.origin[0]=ox;r.origin[1]=oy;r.origin[2]=oz;r.direction[0]=dx;r.direction[1]=dy;r.direction[2]=dz;r.t_min=tmin;r.t_max=tmax;return r; };
    // Triangle hit/miss, both faces, nontrivial barycentrics, normal, nearest and interval cases.
    assert_ray_query_matches_oracle(context,handle,"triangle-centered",triangle,1u,nullptr,0u,ray(0,0,0,0,0,1,0,100));
    assert_ray_query_matches_oracle(context,handle,"triangle-miss",triangle,1u,nullptr,0u,ray(3,0,0,0,0,1,0,100));
    assert_ray_query_matches_oracle(context,handle,"triangle-back-face",triangle,1u,nullptr,0u,ray(0,0,0,0,0,1,0,100));
    assert_ray_query_matches_oracle(context,handle,"triangle-front-face",triangle,1u,nullptr,0u,ray(0,0,5,0,0,-1,0,100));
    assert_ray_query_matches_oracle(context,handle,"triangle-interior-bary",triangle,1u,nullptr,0u,ray(0.2f,0.1f,0,0,0,1,0,100));
    assert_ray_query_matches_oracle(context,handle,"triangle-nearest",depthTriangles,2u,nullptr,0u,ray(0,0,0,0,0,1,0,100));
    assert_ray_query_matches_oracle(context,handle,"triangle-before-tmin",triangle,1u,nullptr,0u,ray(0,0,0,0,0,1,2.1f,100));
    assert_ray_query_matches_oracle(context,handle,"triangle-after-tmax",triangle,1u,nullptr,0u,ray(0,0,0,0,0,1,0,1.9f));
    // Sphere external/miss/AABB false-positive/inside/interval/tangent/near-tangent/depth cases.
    assert_ray_query_matches_oracle(context,handle,"sphere-exterior",nullptr,0u,sphere,1u,ray(0,0,0,0,0,1,0,100));
    assert_ray_query_matches_oracle(context,handle,"sphere-nonunit-direction",nullptr,0u,sphere,1u,ray(0,0,0,0,0,2,0,100));
    assert_ray_query_matches_oracle(context,handle,"sphere-miss",nullptr,0u,sphere,1u,ray(3,0,0,0,0,1,0,100));
    assert_ray_query_matches_oracle(context,handle,"sphere-inside-exit",nullptr,0u,sphere,1u,ray(0,0,4,0,0,1,0,100));
    assert_ray_query_matches_oracle(context,handle,"sphere-behind",nullptr,0u,sphere,1u,ray(0,0,0,0,0,-1,0,100));
    assert_ray_query_matches_oracle(context,handle,"sphere-exit-after-tmin",nullptr,0u,sphere,1u,ray(0,0,0,0,0,1,3.5f,100));
    assert_ray_query_matches_oracle(context,handle,"sphere-both-before-tmin",nullptr,0u,sphere,1u,ray(0,0,0,0,0,1,5.1f,100));
    assert_ray_query_matches_oracle(context,handle,"sphere-after-tmax",nullptr,0u,sphere,1u,ray(0,0,0,0,0,1,0,2.9f));
    assert_ray_query_matches_oracle(context,handle,"sphere-tangent",nullptr,0u,tangent,1u,ray(0,0,0,0,0,1,0,100));
    assert_ray_query_matches_oracle(context,handle,"sphere-near-tangent-hit",nullptr,0u,nearTangentHit,1u,ray(0,0,0,0,0,1,0,100));
    assert_ray_query_matches_oracle(context,handle,"sphere-near-tangent-miss",nullptr,0u,nearTangentMiss,1u,ray(0,0,0,0,0,1,0,100));
    assert_ray_query_matches_oracle(context,handle,"sphere-overlap-nearest",nullptr,0u,overlap,2u,ray(0,0,0,0,0,1,0,100));
    assert_ray_query_matches_oracle(context,handle,"aabb-false-positive",triangle,1u,aabbFalse,1u,ray(0,0,0,0,0,1,0,100));
    // Mixed nearest ordering and topology cases.
    const PrometheusRayQuerySphere nearSphere[] = {{{0,0,2},1,{1,1,1},1u}};
    const PrometheusRayQuerySphere farSphere[] = {{{0,0,8},1,{1,1,1},1u}};
    assert_ray_query_matches_oracle(context,handle,"mixed-sphere-wins",triangle,1u,nearSphere,1u,ray(0,0,0,0,0,1,0,100));
    assert_ray_query_matches_oracle(context,handle,"mixed-triangle-wins",triangle,1u,farSphere,1u,ray(0,0,0,0,0,1,0,100));
    std::fprintf(stderr,"ray-query numerical maxima: t=%.9g (%s), position=%.9g (%s), normal=%.9g (%s), barycentric=%.9g (%s)\n",g_ray_query_error_stats.t,g_ray_query_error_stats.tName,g_ray_query_error_stats.position,g_ray_query_error_stats.positionName,g_ray_query_error_stats.normal,g_ray_query_error_stats.normalName,g_ray_query_error_stats.barycentric,g_ray_query_error_stats.baryName);
    ASSERT_EQUAL(PROM_OK,prometheus_reactor_runtime_destroy(handle),"runtime destroy");
}

FACT(PrometheusReactor_RayQueryMixedScenesRemainIsolated)
{
    PrometheusReactorConfig config = raw_hit_runtime_config();
    ASSERT_TRUE(config.shader_package_root != nullptr && config.shader_package_root[0] != '\0', "raw isolation receives staged shader package root");
    void* handle=nullptr; ASSERT_EQUAL(PROM_OK,prometheus_reactor_runtime_create(&config,&handle),"runtime create");
    prom_vk_runtime_services services{}; ASSERT_EQUAL(PROM_OK,prom_reactor_runtime_get_vk_services(handle,&services),"services");
    if (services.ray_query_state != PROM_VK_RAY_QUERY_DEVICE_FEATURE_ENABLED) { SKIP("ray-query feature bundle is unavailable on this device"); }
    const PrometheusRayQueryTriangle tri[] = {{{-1,-1,4,1,-1,4,0,1,4}}};
    const PrometheusRayQuerySphere hitSphere[] = {{{0,0,3},1,{1,0,0},1u}};
    const PrometheusRayQuerySphere missSphere[] = {{{10,0,3},1,{0,1,0},2u}};
    PrometheusRayQuerySceneCreateRequest a{}; a.struct_size=static_cast<std::uint32_t>(sizeof(a)); a.triangles=tri;a.triangle_count=1u;a.spheres=hitSphere;a.sphere_count=1u;
    PrometheusRayQuerySceneCreateRequest b=a; b.spheres=missSphere;
    std::uint64_t sceneA=0u,sceneB=0u; ASSERT_EQUAL(PROM_OK,prometheus_reactor_runtime_ray_query_scene_create(handle,&a,&sceneA),"first mixed scene"); ASSERT_EQUAL(PROM_OK,prometheus_reactor_runtime_ray_query_scene_create(handle,&b,&sceneB),"second mixed scene");
    PrometheusRayQueryRawRequest ray{};ray.struct_size=static_cast<std::uint32_t>(sizeof(ray));ray.direction[2]=1.0f;ray.t_max=100.0f;
    PrometheusRayQueryRawHit hitA{},hitB{},againA{}; ASSERT_EQUAL(PROM_OK,prometheus_reactor_runtime_ray_query_scene_trace(handle,sceneA,&ray,&hitA),"first scene trace"); ASSERT_EQUAL(PROM_OK,prometheus_reactor_runtime_ray_query_scene_trace(handle,sceneB,&ray,&hitB),"second scene trace"); ASSERT_EQUAL(PROM_OK,prometheus_reactor_runtime_ray_query_scene_trace(handle,sceneA,&ray,&againA),"alternate first scene trace");
    ASSERT_EQUAL(2u,hitA.meta[1],"first scene retains its sphere"); ASSERT_EQUAL(1u,hitB.meta[1],"second scene retains its triangle"); ASSERT_NEAR(hitA.t_primitive[0],againA.t_primitive[0],0.0f,"alternating trace does not cross-contaminate scene state");
    ASSERT_EQUAL(PROM_OK,prometheus_reactor_runtime_ray_query_scene_destroy(handle,sceneA),"destroy first mixed scene"); ASSERT_EQUAL(PROM_OK,prometheus_reactor_runtime_ray_query_scene_trace(handle,sceneB,&ray,&hitB),"destroying first scene leaves second scene live"); ASSERT_EQUAL(PROM_INVALID_HANDLE,prometheus_reactor_runtime_ray_query_scene_trace(handle,sceneA,&ray,&againA),"destroyed mixed handle rejected");
    ASSERT_EQUAL(PROM_OK,prometheus_reactor_runtime_ray_query_scene_destroy(handle,sceneB),"destroy second mixed scene"); ASSERT_EQUAL(PROM_OK,prometheus_reactor_runtime_destroy(handle),"runtime destroy");
}

FACT(PrometheusReactor_RayQueryRawRejectsInvalidInputs)
{
    PrometheusReactorConfig config = raw_hit_runtime_config();
    ASSERT_TRUE(config.shader_package_root != nullptr && config.shader_package_root[0] != '\0', "raw validation receives staged shader package root");
    void* handle=nullptr; ASSERT_EQUAL(PROM_OK,prometheus_reactor_runtime_create(&config,&handle),"runtime create");
    prom_vk_runtime_services services{}; ASSERT_EQUAL(PROM_OK,prom_reactor_runtime_get_vk_services(handle,&services),"services");
    if (services.ray_query_state != PROM_VK_RAY_QUERY_DEVICE_FEATURE_ENABLED) { SKIP("ray-query feature bundle is unavailable on this device"); }
    const PrometheusRayQuerySphere valid[] = {{{0,0,3},1,{1,1,1},1u}};
    PrometheusRayQuerySceneCreateRequest request{}; request.struct_size=static_cast<std::uint32_t>(sizeof(request)); request.spheres=valid; request.sphere_count=1u;
    std::uint64_t scene=0u; ASSERT_EQUAL(PROM_OK,prometheus_reactor_runtime_ray_query_scene_create(handle,&request,&scene),"valid scene");
    PrometheusRayQueryRawRequest ray{}; ray.struct_size=static_cast<std::uint32_t>(sizeof(ray)); ray.direction[2]=1.0f;ray.t_max=100.0f; PrometheusRayQueryRawHit out{};
    const float nan=std::numeric_limits<float>::quiet_NaN();
    ray.origin[0]=nan; ASSERT_EQUAL(PROM_ERROR,prometheus_reactor_runtime_ray_query_scene_trace(handle,scene,&ray,&out),"nonfinite origin"); ray.origin[0]=0.0f;
    ray.direction[1]=nan; ASSERT_EQUAL(PROM_ERROR,prometheus_reactor_runtime_ray_query_scene_trace(handle,scene,&ray,&out),"nonfinite direction"); ray.direction[1]=0.0f;
    ray.direction[2]=0.0f; ASSERT_EQUAL(PROM_ERROR,prometheus_reactor_runtime_ray_query_scene_trace(handle,scene,&ray,&out),"zero direction"); ray.direction[2]=1.0f;
    ray.t_min=nan; ASSERT_EQUAL(PROM_ERROR,prometheus_reactor_runtime_ray_query_scene_trace(handle,scene,&ray,&out),"nonfinite tmin"); ray.t_min=0.0f;
    ray.t_max=nan; ASSERT_EQUAL(PROM_ERROR,prometheus_reactor_runtime_ray_query_scene_trace(handle,scene,&ray,&out),"nonfinite tmax"); ray.t_max=100.0f;
    ray.t_min=-1.0f; ASSERT_EQUAL(PROM_ERROR,prometheus_reactor_runtime_ray_query_scene_trace(handle,scene,&ray,&out),"negative tmin"); ray.t_min=2.0f;ray.t_max=1.0f; ASSERT_EQUAL(PROM_ERROR,prometheus_reactor_runtime_ray_query_scene_trace(handle,scene,&ray,&out),"inverted interval"); ray.t_min=0.0f; ray.t_max=100.0f;
    ASSERT_EQUAL(PROM_ERROR,prometheus_reactor_runtime_ray_query_scene_trace(handle,scene,&ray,nullptr),"malformed raw output");
    PrometheusRayQuerySphere invalidSphere=valid[0]; invalidSphere.radius=0.0f; request.spheres=&invalidSphere; std::uint64_t bad=0u; ASSERT_EQUAL(PROM_ERROR,prometheus_reactor_runtime_ray_query_scene_create(handle,&request,&bad),"zero sphere radius"); invalidSphere.radius=-1.0f; ASSERT_EQUAL(PROM_ERROR,prometheus_reactor_runtime_ray_query_scene_create(handle,&request,&bad),"negative sphere radius"); invalidSphere.radius=1.0f;invalidSphere.center[0]=nan; ASSERT_EQUAL(PROM_ERROR,prometheus_reactor_runtime_ray_query_scene_create(handle,&request,&bad),"nonfinite sphere center");
    ASSERT_EQUAL(PROM_OK,prometheus_reactor_runtime_ray_query_scene_destroy(handle,scene),"destroy valid scene"); ASSERT_EQUAL(PROM_INVALID_HANDLE,prometheus_reactor_runtime_ray_query_scene_trace(handle,scene,&ray,&out),"destroyed handle"); ASSERT_EQUAL(PROM_INVALID_HANDLE,prometheus_reactor_runtime_ray_query_scene_trace(handle,0u,&ray,&out),"invalid handle"); ASSERT_EQUAL(PROM_OK,prometheus_reactor_runtime_destroy(handle),"runtime destroy");
}

static std::vector<PrometheusComplex32> fft_dft_oracle(const std::vector<PrometheusComplex32>& input,
                                                        std::uint32_t elementCount,
                                                        std::uint32_t batchCount,
                                                        std::uint32_t stride,
                                                        bool inverse)
{
    constexpr double pi = 3.14159265358979323846264338327950288;
    std::vector<PrometheusComplex32> expected = input;
    for (std::uint32_t batch = 0u; batch < batchCount; ++batch) {
        const std::uint32_t base = batch * stride;
        for (std::uint32_t k = 0u; k < elementCount; ++k) {
            double real = 0.0;
            double imag = 0.0;
            for (std::uint32_t sample = 0u; sample < elementCount; ++sample) {
                const double angle = (inverse ? 2.0 : -2.0) * pi * static_cast<double>(k * sample) / static_cast<double>(elementCount);
                const double cosine = std::cos(angle);
                const double sine = std::sin(angle);
                const PrometheusComplex32 value = input[base + sample];
                real += static_cast<double>(value.real) * cosine - static_cast<double>(value.imag) * sine;
                imag += static_cast<double>(value.real) * sine + static_cast<double>(value.imag) * cosine;
            }
            if (inverse) {
                real /= static_cast<double>(elementCount);
                imag /= static_cast<double>(elementCount);
            }
            expected[base + k] = {static_cast<float>(real), static_cast<float>(imag)};
        }
    }
    return expected;
}

static void assert_fft_matches_dft(::marionette::tests::TestContext& context,
                                   void* handle,
                                   const char* name,
                                   const std::vector<PrometheusComplex32>& input,
                                   std::uint32_t elementCount,
                                   std::uint32_t batchCount,
                                   std::uint32_t stride,
                                   bool inverse)
{
    std::vector<PrometheusComplex32> output(input.size(), {77.0f, -77.0f});
    const std::vector<PrometheusComplex32> expected = fft_dft_oracle(input, elementCount, batchCount, stride, inverse);
    PrometheusFftRequest request = make_valid_req(const_cast<PrometheusComplex32*>(input.data()), output.data());
    request.element_count = elementCount;
    request.batch_count = batchCount;
    request.stride_elements = stride == elementCount ? 0u : stride;
    request.flags = inverse ? PROM_FFT_FLAG_INVERSE : 0u;
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft(handle, &request, &stage, &detail), std::string(name) + " must execute");
    if (detail != 0) return;

    float maxAbsoluteError = 0.0f;
    for (std::uint32_t batch = 0u; batch < batchCount; ++batch) {
        const std::uint32_t base = batch * stride;
        for (std::uint32_t k = 0u; k < elementCount; ++k) {
            maxAbsoluteError = std::max(maxAbsoluteError, std::fabs(output[base + k].real - expected[base + k].real));
            maxAbsoluteError = std::max(maxAbsoluteError, std::fabs(output[base + k].imag - expected[base + k].imag));
        }
        for (std::uint32_t padding = elementCount; padding < stride; ++padding) {
            ASSERT_NEAR(77.0f, output[base + padding].real, 0.0f, std::string(name) + " must preserve output padding");
            ASSERT_NEAR(-77.0f, output[base + padding].imag, 0.0f, std::string(name) + " must preserve output padding");
        }
    }
    ASSERT_TRUE(maxAbsoluteError <= 3.0e-3f, std::string(name) + " independent DFT maximum absolute error is within Complex32 acceptance");
}

FACT(PrometheusReactor_FftBatchedStrideAndInverseNormalization)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");
    // Two N=2 transforms, with two Complex32 padding values between batches.
    PrometheusComplex32 in[6]{{1.0f, 0.0f}, {2.0f, 0.0f}, {91.0f, -91.0f}, {92.0f, -92.0f}, {3.0f, 0.0f}, {5.0f, 0.0f}};
    PrometheusComplex32 out[6]{{-1.0f, -1.0f}, {-1.0f, -1.0f}, {77.0f, -77.0f}, {78.0f, -78.0f}, {-1.0f, -1.0f}, {-1.0f, -1.0f}};
    PrometheusFftRequest req = make_valid_req(in, out);
    req.batch_count = 2u;
    req.stride_elements = 4u;
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft(handle, &req, &stage, &detail), "batched strided forward FFT should execute");
    ASSERT_NEAR(3.0f, out[0].real, 1.0e-5f, "batch zero DC");
    ASSERT_NEAR(-1.0f, out[1].real, 1.0e-5f, "batch zero Nyquist");
    ASSERT_NEAR(8.0f, out[4].real, 1.0e-5f, "batch one DC");
    ASSERT_NEAR(-2.0f, out[5].real, 1.0e-5f, "batch one Nyquist");
    ASSERT_NEAR(77.0f, out[2].real, 0.0f, "first padding sentinel must survive");
    ASSERT_NEAR(78.0f, out[3].real, 0.0f, "second padding sentinel must survive");

    req.input = out;
    req.output = in;
    req.flags = PROM_FFT_FLAG_INVERSE;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft(handle, &req, &stage, &detail), "inverse-normalized FFT should execute");
    ASSERT_NEAR(1.0f, in[0].real, 1.0e-5f, "inverse restores batch zero sample zero");
    ASSERT_NEAR(2.0f, in[1].real, 1.0e-5f, "inverse restores batch zero sample one");
    ASSERT_NEAR(3.0f, in[4].real, 1.0e-5f, "inverse restores batch one sample zero");
    ASSERT_NEAR(5.0f, in[5].real, 1.0e-5f, "inverse restores batch one sample one");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_FftNumericalCorpusUsesIndependentDftAuthority)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");

    for (const std::uint32_t elementCount : {1u, 2u, 8u, 16u, 64u, 256u, 1024u}) {
        std::vector<PrometheusComplex32> random(elementCount);
        std::uint32_t state = 0x9e3779b9u ^ elementCount;
        for (PrometheusComplex32& value : random) {
            state = state * 1664525u + 1013904223u;
            value.real = static_cast<float>(static_cast<int>((state >> 8u) & 0xffffu) - 32768) / 32768.0f;
            state = state * 1664525u + 1013904223u;
            value.imag = static_cast<float>(static_cast<int>((state >> 8u) & 0xffffu) - 32768) / 32768.0f;
        }
        assert_fft_matches_dft(context, handle, "deterministic pseudo-random forward", random, elementCount, 1u, elementCount, false);
        assert_fft_matches_dft(context, handle, "deterministic pseudo-random inverse", random, elementCount, 1u, elementCount, true);
    }

    std::vector<PrometheusComplex32> tone(32u);
    for (std::uint32_t i = 0u; i < 32u; ++i) {
        const float angle = 2.0f * 3.14159265358979323846f * 5.0f * static_cast<float>(i) / 32.0f;
        tone[i] = {std::cos(angle), std::sin(angle)};
    }
    assert_fft_matches_dft(context, handle, "single-frequency complex tone", tone, 32u, 1u, 32u, false);

    std::vector<PrometheusComplex32> realNonSymmetric(16u);
    for (std::uint32_t i = 0u; i < 16u; ++i) realNonSymmetric[i] = {static_cast<float>((i * i + 3u * i + 1u) % 11u) - 5.0f, 0.0f};
    assert_fft_matches_dft(context, handle, "real-only non-symmetric", realNonSymmetric, 16u, 1u, 16u, false);

    std::vector<PrometheusComplex32> constant(8u, {2.0f, -0.5f});
    assert_fft_matches_dft(context, handle, "complex constant", constant, 8u, 1u, 8u, false);
    std::vector<PrometheusComplex32> alternating(8u);
    for (std::uint32_t i = 0u; i < 8u; ++i) alternating[i] = {(i & 1u) == 0u ? 1.0f : -1.0f, 0.0f};
    assert_fft_matches_dft(context, handle, "alternating signs", alternating, 8u, 1u, 8u, false);

    std::vector<PrometheusComplex32> batched(57u, {31.0f, -31.0f});
    for (std::uint32_t batch = 0u; batch < 3u; ++batch) {
        for (std::uint32_t i = 0u; i < 16u; ++i) {
            batched[batch * 19u + i] = {static_cast<float>(batch * 7u + i) / 9.0f, static_cast<float>(static_cast<int>(batch * 5u) - static_cast<int>(i)) / 11.0f};
        }
    }
    assert_fft_matches_dft(context, handle, "three distinct padded batches", batched, 16u, 3u, 19u, false);

    std::vector<PrometheusComplex32> roundTrip(64u);
    for (std::uint32_t i = 0u; i < 64u; ++i) roundTrip[i] = {static_cast<float>(i % 9u) - 4.0f, static_cast<float>((3u * i) % 13u) - 6.0f};
    const std::vector<PrometheusComplex32> original = roundTrip;
    std::vector<PrometheusComplex32> transformed(64u);
    PrometheusFftRequest request = make_valid_req(roundTrip.data(), transformed.data());
    request.element_count = 64u;
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft(handle, &request, &stage, &detail), "round-trip forward must execute");
    request.input = transformed.data();
    request.output = roundTrip.data();
    request.flags = PROM_FFT_FLAG_INVERSE;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft(handle, &request, &stage, &detail), "round-trip inverse must execute");
    float roundTripMaximumAbsoluteError = 0.0f;
    for (std::uint32_t i = 0u; i < 64u; ++i) {
        roundTripMaximumAbsoluteError = std::max(roundTripMaximumAbsoluteError, std::fabs(roundTrip[i].real - original[i].real));
        roundTripMaximumAbsoluteError = std::max(roundTripMaximumAbsoluteError, std::fabs(roundTrip[i].imag - original[i].imag));
    }
    ASSERT_TRUE(roundTripMaximumAbsoluteError <= 3.0e-3f, "forward then inverse Complex32 round trip must stay within acceptance");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_FftValidationFailures)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");
    PrometheusComplex32 in[4]{};
    PrometheusComplex32 out[4]{};
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;

    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_fft(handle, nullptr, &stage, &detail), "null request should fail");
    ASSERT_EQUAL(PROM_FFT_DETAIL_INVALID_REQUEST, detail, "null request detail");

    PrometheusFftRequest req = make_valid_req(in, out);
    req.struct_size = 0u;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_fft(handle, &req, &stage, &detail), "bad struct size should fail");
    ASSERT_EQUAL(PROM_FFT_DETAIL_INVALID_REQUEST, detail, "bad struct size detail");

    req = make_valid_req(in, out); req.input = nullptr;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_fft(handle, &req, &stage, &detail), "null input should fail");
    ASSERT_EQUAL(PROM_FFT_DETAIL_NULL_INPUT, detail, "null input detail");

    req = make_valid_req(in, out); req.output = nullptr;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_fft(handle, &req, &stage, &detail), "null output should fail");
    ASSERT_EQUAL(PROM_FFT_DETAIL_NULL_OUTPUT, detail, "null output detail");

    req = make_valid_req(in, out); req.element_count = 0u;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_fft(handle, &req, &stage, &detail), "zero element should fail");
    ASSERT_EQUAL(PROM_FFT_DETAIL_ZERO_ELEMENT_COUNT, detail, "zero element detail");

    req = make_valid_req(in, out); req.element_count = 3u;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_fft(handle, &req, &stage, &detail), "non power of two should fail");
    ASSERT_EQUAL(PROM_FFT_DETAIL_NON_POWER_OF_TWO, detail, "non power detail");

    req = make_valid_req(in, out); req.flags = PROM_FFT_FLAG_FORWARD | PROM_FFT_FLAG_INVERSE;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_fft(handle, &req, &stage, &detail), "dual direction should fail");
    ASSERT_EQUAL(PROM_FFT_DETAIL_INVALID_DIRECTION_FLAGS, detail, "direction detail");

    req = make_valid_req(in, out); req.stride_elements = 1u;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_fft(handle, &req, &stage, &detail), "short stride should fail");
    ASSERT_EQUAL(PROM_FFT_DETAIL_INVALID_STRIDE, detail, "stride detail");

    req = make_valid_req(in, out); req.flags = PROM_FFT_FLAG_INVERSE_NORMALIZE;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_fft(handle, &req, &stage, &detail), "inverse normalize without inverse should fail");
    ASSERT_EQUAL(PROM_FFT_DETAIL_INVERSE_NORMALIZE_REQUIRES_INVERSE, detail, "inverse normalize direction rule");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_FftBenchmarkVariantExecutesSmallRadix2)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");
    PrometheusComplex32 in[2]{};
    PrometheusComplex32 out[2]{};
    PrometheusFftRequest req = make_valid_req(in, out);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft_benchmark_variant(handle, &req, PROM_FFT_BENCHMARK_VARIANT_RADIX2, &stage, &detail),
                 "benchmark variant must invoke the production Vulkan radix-2 route");
    ASSERT_EQUAL(0, detail, "benchmark execution must report success only after real execution");

    PrometheusFftDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft_diagnostics(handle, &diag), "fft diagnostics should succeed");
    ASSERT_EQUAL(2u, diag.requested_radix, "diag should record radix-2 planning");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_FFT_PATH_VULKAN_RADIX2_RESERVED), diag.requested_path_id,
                 "requested path should reflect benchmark request");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_FFT_PATH_VULKAN_RADIX2_RESERVED), diag.executed_path_id,
                 "benchmark execution must identify the Vulkan route");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_FftInvalidAndDestroyedHandle)
{
    PrometheusComplex32 in[2]{};
    PrometheusComplex32 out[2]{};
    PrometheusFftRequest req = make_valid_req(in, out);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;

    ASSERT_EQUAL(PROM_INVALID_HANDLE,
                 prometheus_reactor_runtime_fft(nullptr, &req, &stage, &detail),
                 "null handle should be rejected");

    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");

    ASSERT_EQUAL(PROM_INVALID_HANDLE,
                 prometheus_reactor_runtime_fft(handle, &req, &stage, &detail),
                 "destroyed handle should be rejected");
}

FACT(PrometheusReactor_FftDiagnosticsSizedAndCallFreshness)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");

    PrometheusFftDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK,
                 prometheus_reactor_runtime_fft_diagnostics_sized(handle, &diag, 1u),
                 "undersized diagnostics struct should return partial snapshot like SGEMM sized diagnostics");

    PrometheusComplex32 in[8]{};
    PrometheusComplex32 out[8]{};
    PrometheusFftRequest req = make_valid_req(in, out);
    req.element_count = 8u;
    req.stride_elements = 0u;

    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft(handle, &req, &stage, &detail), "fft execution should complete");
    ASSERT_EQUAL(0, detail, "successful fft should have no failure detail");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft_diagnostics(handle, &diag), "fft diagnostics should succeed");
    ASSERT_EQUAL(8u, diag.last_effective_stride_elements, "stride_elements=0 should record contiguous effective stride");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_FFT_PATH_VULKAN_RADIX2_RESERVED), diag.requested_path_id,
                 "first call should select the Vulkan radix-2 production route");

    req = make_valid_req(in, out);
    req.input = nullptr;
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_fft(handle, &req, &stage, &detail), "second call should fail validation");
    ASSERT_EQUAL(PROM_FFT_DETAIL_NULL_INPUT, detail, "second call should report current validation failure");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft_diagnostics(handle, &diag), "fft diagnostics should succeed");
    ASSERT_EQUAL(PROM_FFT_DETAIL_NULL_INPUT, diag.last_failure_detail,
                 "diagnostics should report latest failure and not stale unavailable detail");
    ASSERT_EQUAL(0u, diag.plan_valid, "invalid request should clear plan validity");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_FFT_PATH_NONE), diag.requested_path_id,
                 "failed validation should not keep stale requested path from prior call");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_FFT_PATH_UNAVAILABLE), diag.executed_path_id,
                 "invalid request must clear the previous executed route");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_FftPlanDeterministicN1N8N16AndFlags)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");
    PrometheusComplex32 in[16]{};
    PrometheusComplex32 out[16]{};
    PrometheusFftRequest req = make_valid_req(in, out);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    PrometheusFftDiagnostics diag{};

    req.element_count = 1u;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft(handle, &req, &stage, &detail), "n=1 must execute as a bit-reversal identity");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft_diagnostics(handle, &diag), "diagnostics should succeed");
    ASSERT_EQUAL(1u, diag.plan_valid, "n=1 should still produce deterministic plan metadata");
    ASSERT_EQUAL(0u, diag.plan_pass_count, "n=1 pass count");
    ASSERT_EQUAL(0u, diag.plan_log2_element_count, "n=1 log2");
    ASSERT_EQUAL(0u, diag.ping_pong_swap_count, "n=1 swap count");

    req = make_valid_req(in, out);
    req.element_count = 8u;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft(handle, &req, &stage, &detail), "n=8 must execute its three planned passes");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft_diagnostics(handle, &diag), "diagnostics should succeed");
    ASSERT_EQUAL(3u, diag.plan_pass_count, "n=8 pass count");
    ASSERT_EQUAL(2u, diag.plan_first_span, "n=8 first span");
    ASSERT_EQUAL(8u, diag.plan_last_span, "n=8 last span");
    ASSERT_EQUAL(1u, diag.plan_bit_reversal_required, "n>1 requires bit reversal step");
    ASSERT_EQUAL(3u, diag.ping_pong_swap_count, "n=8 swap count equals pass count");

    req = make_valid_req(in, out);
    req.element_count = 16u;
    req.flags = PROM_FFT_FLAG_INVERSE;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft(handle, &req, &stage, &detail), "n=16 inverse must execute its four planned passes");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft_diagnostics(handle, &diag), "diagnostics should succeed");
    ASSERT_EQUAL(4u, diag.plan_pass_count, "n=16 pass count");
    ASSERT_EQUAL(2u, diag.plan_first_span, "n=16 first span");
    ASSERT_EQUAL(16u, diag.plan_last_span, "n=16 last span");
    ASSERT_EQUAL(4u, diag.ping_pong_swap_count, "n=16 swap count equals pass count");
    ASSERT_EQUAL(2u, diag.plan_direction, "inverse flag should set inverse direction");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusReactor_FftExecutableAndProductionQualified)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");
    PrometheusComplex32 in[32]{};
    PrometheusComplex32 out[32]{};
    PrometheusFftRequest req = make_valid_req(in, out);
    req.element_count = 32u;
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft_benchmark_variant(handle, &req, PROM_FFT_BENCHMARK_VARIANT_RADIX2, &stage, &detail), "bounded N=32 benchmark route should execute");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft(handle, &req, &stage, &detail), "bounded production fft should execute");
    PrometheusFftDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_fft_diagnostics(handle, &diag), "diag");
    ASSERT_EQUAL(1u, diag.production_enabled, "production admission follows the qualified executable route");
    ASSERT_EQUAL(1u, diag.capability_reported, "FFT capability should report the registered production route");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}
