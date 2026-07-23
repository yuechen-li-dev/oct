#include "test_harness.h"

#include "../reactor_vulkan.h"

#include <cstdint>
#include <filesystem>
#include <string>
#include <vector>

namespace {
int Submit(void* runtime, std::uint64_t scene, const std::vector<PrometheusRayQueryRay>& rays,
           std::vector<PrometheusRayQueryHit>* hits)
{
    PrometheusRayQueryBatchRequest request{};
    request.struct_size = static_cast<std::uint32_t>(sizeof(request));
    request.rays = rays.empty() ? nullptr : rays.data();
    request.ray_count = static_cast<std::uint32_t>(rays.size());
    request.hits = hits->empty() ? nullptr : hits->data();
    PrometheusRayQueryBatchResult result{};
    result.struct_size = static_cast<std::uint32_t>(sizeof(result));
    return prometheus_reactor_runtime_ray_query_scene_submit_batch(runtime, scene, &request, &result);
}
}

FACT(PrometheusRayQuery_BatchUsesOnePhysicalDispatchAndRetainsCapacity)
{
    const std::filesystem::path packageRoot = std::filesystem::path(MARIONETTE_TEST_REPO_ROOT) /
        "out" / "prometheus" / "native" / "rqm1_shader_package";
    const std::string packageRootText = packageRoot.string();
    PrometheusRayQueryRuntimeConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config));
    config.shader_package_root = packageRootText.c_str();
    void* runtime = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_ray_query_runtime_create(&config, &runtime), "runtime create");
    std::uint64_t scene = 0u;
    ASSERT_EQUAL(PROM_OK,
                 prometheus_reactor_runtime_ray_query_scene_create_empty(runtime, &scene), "scene create");
    const PrometheusRayQuerySphere sphere[] = {{{0.0f, 0.0f, 3.0f}, 1.0f, {0.2f, 0.4f, 0.6f}, 9u}};
    ASSERT_EQUAL(PROM_OK,
                 prometheus_reactor_runtime_ray_query_scene_add_spheres(runtime, scene, sphere, 1u), "sphere add");
    if (prometheus_reactor_runtime_ray_query_scene_commit(runtime, scene) != PROM_OK) {
        prometheus_reactor_runtime_ray_query_scene_destroy(runtime, scene);
        prometheus_reactor_runtime_destroy(runtime);
        SKIP("ray-query capability is unavailable on this device");
    }
    std::vector<PrometheusRayQueryRay> rays(1u);
    rays[0].direction[2] = 1.0f;
    rays[0].t_max = 100.0f;
    rays[0].visibility_mask = PROM_RAY_QUERY_VISIBILITY_MASK_ALL;
    std::vector<PrometheusRayQueryHit> hits(1u);
    ASSERT_EQUAL(PROM_OK, Submit(runtime, scene, rays, &hits), "one-ray semantic batch");
    ASSERT_EQUAL(1u, hits[0].hit, "one-ray batch hit");
    prom_ray_query_batch_diagnostics initial{};
    ASSERT_EQUAL(PROM_OK, prom_ray_query_scene_batch_diagnostics_impl(runtime, scene, &initial), "diagnostics after one ray");
    ASSERT_EQUAL(1u, initial.last_dispatch_groups_x, "one API ray records one X workgroup");
    ASSERT_EQUAL(1u, initial.physical_dispatch_count, "one API batch records one dispatch");
    ASSERT_EQUAL(1u, initial.physical_submission_count, "one API batch submits once");

    ASSERT_EQUAL(PROM_OK, Submit(runtime, scene, rays, &hits), "equal-size semantic batch");
    prom_ray_query_batch_diagnostics reused{};
    ASSERT_EQUAL(PROM_OK, prom_ray_query_scene_batch_diagnostics_impl(runtime, scene, &reused), "diagnostics after reuse");
    ASSERT_EQUAL(initial.retained_capacity, reused.retained_capacity, "equal batch retains capacity");
    ASSERT_EQUAL(initial.buffer_reallocation_count, reused.buffer_reallocation_count, "equal batch does not grow");
    ASSERT_EQUAL(2u, reused.physical_dispatch_count, "second API batch adds exactly one dispatch");

    rays.resize(9u);
    hits.resize(9u);
    for (PrometheusRayQueryRay& ray : rays) { ray.direction[2] = 1.0f; ray.t_max = 100.0f; ray.visibility_mask = PROM_RAY_QUERY_VISIBILITY_MASK_ALL; }
    ASSERT_EQUAL(PROM_OK, Submit(runtime, scene, rays, &hits), "growing semantic batch");
    prom_ray_query_batch_diagnostics grown{};
    ASSERT_EQUAL(PROM_OK, prom_ray_query_scene_batch_diagnostics_impl(runtime, scene, &grown), "diagnostics after growth");
    ASSERT_TRUE(grown.retained_capacity >= 9u, "growth retains enough paired capacity");
    ASSERT_EQUAL(reused.buffer_reallocation_count + 1u, grown.buffer_reallocation_count, "large batch replaces paired buffers once");
    ASSERT_EQUAL(reused.descriptor_rebind_count + 1u, grown.descriptor_rebind_count, "replacement rebinds both batch descriptors once");
    ASSERT_EQUAL(9u, grown.last_dispatch_groups_x, "nine API rays record nine X workgroups in one dispatch");
    ASSERT_EQUAL(3u, grown.physical_dispatch_count, "large API batch adds one physical dispatch, not nine");
    ASSERT_EQUAL(3u, grown.physical_submission_count, "large API batch adds one physical submission");

    rays.resize(2u); hits.resize(2u);
    ASSERT_EQUAL(PROM_OK, Submit(runtime, scene, rays, &hits), "shrinking semantic batch");
    prom_ray_query_batch_diagnostics shrunk{};
    ASSERT_EQUAL(PROM_OK, prom_ray_query_scene_batch_diagnostics_impl(runtime, scene, &shrunk), "diagnostics after shrink");
    ASSERT_EQUAL(grown.retained_capacity, shrunk.retained_capacity, "smaller batch reuses grown capacity");
    ASSERT_EQUAL(grown.buffer_reallocation_count, shrunk.buffer_reallocation_count, "smaller batch does not reallocate");
    ASSERT_EQUAL(2u, shrunk.last_dispatch_groups_x, "last dispatch reflects full smaller batch");
    ASSERT_EQUAL(4u, shrunk.physical_dispatch_count, "smaller API batch adds one physical dispatch");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_ray_query_scene_destroy(runtime, scene), "scene destroy");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(runtime), "runtime destroy");
}
