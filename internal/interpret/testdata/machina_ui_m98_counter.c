#include <stdint.h>

#define STATUS_OK 0
#define STATUS_NOT_INITIALIZED 1
#define STATUS_INVALID_INPUT 2

static uint8_t initialized = 0;
static int32_t count = 0;
static int32_t route_stats = 0;
static uint32_t out_len = 0;
static char output[16384];

static uint32_t append_str(uint32_t at, const char* s) {
  while (*s) { output[at++] = *s++; }
  return at;
}

static uint32_t append_digit(uint32_t at, int32_t value) {
  if (value < 0) { output[at++]='-'; value = -value; }
  if (value >= 10) {
    at = append_digit(at, value / 10);
  }
  output[at++] = '0' + (value % 10);
  return at;
}

static uint8_t contains(const char* p, uint32_t len, const char* s) {
  uint32_t slen = 0;
  while (s[slen]) slen++;
  if (slen == 0 || slen > len) return 0;
  for (uint32_t i = 0; i + slen <= len; i++) {
    uint32_t j = 0;
    while (j < slen && p[i + j] == s[j]) j++;
    if (j == slen) return 1;
  }
  return 0;
}

__attribute__((export_name("ExportMachinaUIInit")))
int32_t ExportMachinaUIInit() {
  initialized = 1;
  count = 0;
  route_stats = 0;
  out_len = 0;
  return STATUS_OK;
}

__attribute__((export_name("ExportMachinaUIRender")))
int32_t ExportMachinaUIRender() {
  if (!initialized) return STATUS_NOT_INITIALIZED;
  uint32_t at = 0;
  if (route_stats) {
    at = append_str(at, "{\"abi\":\"machina.uiir.v1\",\"root\":{\"id\":\"0\",\"kind\":\"Column\",\"key\":null,\"text\":null,\"label\":null,\"enabled\":null,\"event\":null,\"box\":null,\"layout\":null,\"children\":[{\"id\":\"0.0\",\"kind\":\"Text\",\"key\":null,\"text\":\"route=stats\",\"label\":null,\"enabled\":null,\"event\":null,\"box\":null,\"layout\":null,\"children\":[]},{\"id\":\"0.1\",\"kind\":\"Text\",\"key\":null,\"text\":\"count=");
    at = append_digit(at, count);
    at = append_str(at, "\",\"label\":null,\"enabled\":null,\"event\":null,\"box\":null,\"layout\":null,\"children\":[]},{\"id\":\"0.2\",\"kind\":\"Button\",\"key\":null,\"text\":null,\"label\":\"Home\",\"enabled\":true,\"event\":{\"token\":\"route.home\",\"payload\":null},\"box\":null,\"layout\":null,\"children\":[]}]}}" );
  } else {
    at = append_str(at, "{\"abi\":\"machina.uiir.v1\",\"root\":{\"id\":\"0\",\"kind\":\"Column\",\"key\":null,\"text\":null,\"label\":null,\"enabled\":null,\"event\":null,\"box\":null,\"layout\":null,\"children\":[{\"id\":\"0.0\",\"kind\":\"Text\",\"key\":null,\"text\":\"route=home\",\"label\":null,\"enabled\":null,\"event\":null,\"box\":null,\"layout\":null,\"children\":[]},{\"id\":\"0.1\",\"kind\":\"Text\",\"key\":null,\"text\":\"count=");
    at = append_digit(at, count);
    at = append_str(at, "\",\"label\":null,\"enabled\":null,\"event\":null,\"box\":null,\"layout\":null,\"children\":[]},{\"id\":\"0.2\",\"kind\":\"Row\",\"key\":null,\"text\":null,\"label\":null,\"enabled\":null,\"event\":null,\"box\":null,\"layout\":null,\"children\":[{\"id\":\"0.2.0\",\"kind\":\"Button\",\"key\":null,\"text\":null,\"label\":\"Increment\",\"enabled\":true,\"event\":{\"token\":\"counter.increment\",\"payload\":null},\"box\":null,\"layout\":null,\"children\":[]},{\"id\":\"0.2.1\",\"kind\":\"Button\",\"key\":null,\"text\":null,\"label\":\"Decrement\",\"enabled\":");
    at = append_str(at, count > 0 ? "true" : "false");
    at = append_str(at, ",\"event\":{\"token\":\"counter.decrement\",\"payload\":null},\"box\":null,\"layout\":null,\"children\":[]},{\"id\":\"0.2.2\",\"kind\":\"Button\",\"key\":null,\"text\":null,\"label\":\"Stats\",\"enabled\":true,\"event\":{\"token\":\"route.stats\",\"payload\":null},\"box\":null,\"layout\":null,\"children\":[]}]}]}}" );
  }
  out_len = at;
  return STATUS_OK;
}

__attribute__((export_name("ExportMachinaUIDispatchEventJSON")))
int32_t ExportMachinaUIDispatchEventJSON(uint32_t ptr, uint32_t len) {
  if (!initialized) return STATUS_NOT_INITIALIZED;
  const char* p = (const char*)(uintptr_t)ptr;
  if (len == 0) return STATUS_INVALID_INPUT;
  if (!contains(p, len, "\"token\"")) return STATUS_INVALID_INPUT;

  if (contains(p, len, "counter.increment")) {
    count += 1;
    return STATUS_OK;
  }
  if (contains(p, len, "counter.decrement")) {
    if (count > 0) count -= 1;
    return STATUS_OK;
  }
  if (contains(p, len, "route.stats")) {
    route_stats = 1;
    return STATUS_OK;
  }
  if (contains(p, len, "route.home")) {
    route_stats = 0;
    return STATUS_OK;
  }
  return STATUS_INVALID_INPUT;
}

__attribute__((export_name("ExportMachinaUIBufferPtr")))
uint32_t ExportMachinaUIBufferPtr() {
  return (uint32_t)(uintptr_t)output;
}

__attribute__((export_name("ExportMachinaUIBufferLen")))
uint32_t ExportMachinaUIBufferLen() {
  return out_len;
}
