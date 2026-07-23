#include "test_harness.h"

#include "../include/prometheus_ray_query.h"

#include <cstddef>
#include <cstdint>
#include <cstring>
#include <filesystem>
#include <string>

static_assert(sizeof(PrometheusRayQueryRay) == 48u);
static_assert(sizeof(PrometheusRayQueryHit) == 80u);
static_assert(offsetof(PrometheusRayQueryBatchRequest, hits) >
              offsetof(PrometheusRayQueryBatchRequest, rays));

FACT(PrometheusRayQuery_PublicHeaderOwnsTheSemanticBoundary)
{
    PrometheusRayQueryBatchResult result{};
    result.struct_size = static_cast<std::uint32_t>(sizeof(result));
    PrometheusRayQueryBatchRequest batch{};
    batch.struct_size = static_cast<std::uint32_t>(sizeof(batch));

    const std::filesystem::path packageRoot = std::filesystem::path(MARIONETTE_TEST_REPO_ROOT) /
        "out" / "prometheus" / "native" / "rqm1_shader_package";
    const std::string packageRootText = packageRoot.string();
    PrometheusRayQueryRuntimeConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config));
    config.shader_package_root = packageRootText.c_str();
    void* runtime = nullptr;
    ASSERT_EQUAL(PROM_RAY_QUERY_OK, prometheus_ray_query_runtime_create(&config, &runtime),
                 "public runtime creation needs no Vulkan declaration");
    std::uint64_t scene = 0u;
    ASSERT_EQUAL(PROM_RAY_QUERY_OK,
                 prometheus_reactor_runtime_ray_query_scene_create_empty(runtime, &scene),
                 "public caller creates an opaque semantic scene");
    ASSERT_EQUAL(PROM_RAY_QUERY_ERROR,
                 prometheus_reactor_runtime_ray_query_scene_submit_batch(runtime, scene, &batch, &result),
                 "uncommitted scenes are rejected before a query can observe stale output");
    ASSERT_EQUAL(PROM_RAY_QUERY_OK, prometheus_reactor_runtime_ray_query_scene_destroy(runtime, scene),
                 "public caller destroys only the semantic scene handle");
    ASSERT_EQUAL(PROM_RAY_QUERY_OK, prometheus_reactor_runtime_destroy(runtime),
                 "public caller destroys the opaque runtime handle");
}

FACT(PrometheusRayQuery_PublicBatchTraversesMixedCommittedScene)
{
    const std::filesystem::path packageRoot = std::filesystem::path(MARIONETTE_TEST_REPO_ROOT) /
        "out" / "prometheus" / "native" / "rqm1_shader_package";
    const std::string packageRootText = packageRoot.string();
    PrometheusRayQueryRuntimeConfig config{};
    config.struct_size = static_cast<std::uint32_t>(sizeof(config));
    config.shader_package_root = packageRootText.c_str();
    void* runtime = nullptr;
    ASSERT_EQUAL(PROM_RAY_QUERY_OK, prometheus_ray_query_runtime_create(&config, &runtime), "runtime create");
    std::uint64_t scene = 0u;
    ASSERT_EQUAL(PROM_RAY_QUERY_OK, prometheus_reactor_runtime_ray_query_scene_create_empty(runtime, &scene), "scene create");
    const PrometheusRayQueryTriangle triangle[] = {{{-1.0f, -1.0f, 4.0f, 1.0f, -1.0f, 4.0f, 0.0f, 1.0f, 4.0f}}};
    const PrometheusRayQuerySphere sphere[] = {{{3.0f, 0.0f, 3.0f}, 1.0f, {0.25f, 0.5f, 0.75f}, 7u}};
    ASSERT_EQUAL(PROM_RAY_QUERY_OK, prometheus_reactor_runtime_ray_query_scene_add_triangles(runtime, scene, triangle, 1u), "triangle copy");
    ASSERT_EQUAL(PROM_RAY_QUERY_OK, prometheus_reactor_runtime_ray_query_scene_add_spheres(runtime, scene, sphere, 1u), "sphere copy");
    if (prometheus_reactor_runtime_ray_query_scene_commit(runtime, scene) != PROM_RAY_QUERY_OK) {
        prometheus_reactor_runtime_ray_query_scene_destroy(runtime, scene);
        prometheus_reactor_runtime_destroy(runtime);
        SKIP("ray-query capability is unavailable on this device");
    }
    PrometheusRayQueryRay rays[3]{};
    rays[0].direction[2] = 1.0f; rays[0].t_max = 100.0f; rays[0].visibility_mask = PROM_RAY_QUERY_VISIBILITY_MASK_ALL;
    rays[1].origin[0] = 3.0f; rays[1].direction[2] = 1.0f; rays[1].t_max = 100.0f; rays[1].visibility_mask = PROM_RAY_QUERY_VISIBILITY_MASK_ALL;
    rays[2].origin[0] = 20.0f; rays[2].direction[2] = 1.0f; rays[2].t_max = 100.0f; rays[2].visibility_mask = PROM_RAY_QUERY_VISIBILITY_MASK_ALL;
    PrometheusRayQueryHit hits[3];
    std::memset(hits, 0xa5, sizeof(hits));
    PrometheusRayQueryBatchRequest batch{};
    batch.struct_size = static_cast<std::uint32_t>(sizeof(batch));
    batch.rays = rays; batch.ray_count = 3u; batch.hits = hits;
    PrometheusRayQueryBatchResult result{};
    result.struct_size = static_cast<std::uint32_t>(sizeof(result));
    ASSERT_EQUAL(PROM_RAY_QUERY_OK, prometheus_reactor_runtime_ray_query_scene_submit_batch(runtime, scene, &batch, &result), "public batch execute");
    ASSERT_EQUAL(1u, hits[0].hit, "central ray hits triangle");
    ASSERT_EQUAL(PROM_RAY_QUERY_GEOMETRY_TRIANGLE, hits[0].geometry_kind, "triangle kind");
    ASSERT_EQUAL(0u, hits[0].instance_id, "triangle instance");
    ASSERT_EQUAL(0u, hits[0].primitive_id, "triangle primitive");
    ASSERT_NEAR(4.0f, hits[0].distance, 2.0e-5f, "triangle distance");
    ASSERT_EQUAL(1u, hits[1].hit, "offset ray hits analytic sphere");
    ASSERT_EQUAL(PROM_RAY_QUERY_GEOMETRY_ANALYTIC_SPHERE, hits[1].geometry_kind, "sphere kind");
    ASSERT_EQUAL(1u, hits[1].instance_id, "sphere instance");
    ASSERT_EQUAL(7u, hits[1].material_id, "sphere material");
    ASSERT_NEAR(2.0f, hits[1].distance, 2.0e-5f, "sphere distance");
    ASSERT_EQUAL(0u, hits[2].hit, "miss remains explicit");
    ASSERT_EQUAL(UINT32_MAX, hits[2].instance_id, "miss instance sentinel replaces prior output");
    ASSERT_EQUAL(UINT32_MAX, hits[2].primitive_id, "miss primitive sentinel replaces prior output");
    ASSERT_NEAR(-1.0f, hits[2].distance, 0.0f, "miss distance sentinel");
    ASSERT_EQUAL(PROM_RAY_QUERY_OK, prometheus_reactor_runtime_ray_query_scene_destroy(runtime, scene), "scene destroy");
    ASSERT_EQUAL(PROM_RAY_QUERY_OK, prometheus_reactor_runtime_destroy(runtime), "runtime destroy");
}
