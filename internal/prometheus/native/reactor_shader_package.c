#include "reactor_shader_package.h"

#include <ctype.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#if defined(_WIN32)
#include <sys/stat.h>
#define PROM_STAT _stat64
#define prom_stat _stat64
#else
#include <sys/stat.h>
#define PROM_STAT stat
#endif

/* This is intentionally a narrow JSON reader for the package schema.  It does
   not construct a general JSON DOM or accept executable package concepts. */
typedef struct prom_json_range { const char* begin; const char* end; } prom_json_range;
typedef struct prom_package_artifact { char digest[65]; uint64_t byte_count; } prom_package_artifact;
typedef struct prom_package_kernel { uint32_t id; char* name; char* stage; char* authority; } prom_package_kernel;
typedef struct prom_package_variant {
  char* id; uint32_t kernel_id; uint32_t artifact_index; char* entry_point;
  uint32_t has_workgroup; uint32_t workgroup[3]; uint32_t descriptor_bindings;
  uint32_t push_constant_bytes; char* stage_role;
} prom_package_variant;
typedef struct prom_package_requirement { uint32_t variant_index; char* kind; char* name; } prom_package_requirement;
typedef struct prom_package_implementation { uint32_t id; char* name; uint32_t variant_index; char* authority; } prom_package_implementation;

struct prom_shader_package {
  char* root;
  prom_package_artifact* artifacts; size_t artifact_count;
  prom_package_kernel* kernels; size_t kernel_count;
  prom_package_variant* variants; size_t variant_count;
  prom_package_requirement* requirements; size_t requirement_count;
  prom_package_implementation* implementations; size_t implementation_count;
  uint64_t artifact_open_count;
};

static void prom_diag(prom_shader_package_diagnostic* d, prom_shader_package_error code, const char* message) {
  if (d == NULL) return;
  d->code = code;
  if (message == NULL) { d->message[0] = '\0'; return; }
  (void)snprintf(d->message, sizeof(d->message), "%s", message);
}
static const char* jws(const char* p, const char* end) { while (p < end && isspace((unsigned char)*p)) ++p; return p; }
static int jstring_end(const char* p, const char* end, const char** out) {
  if (p >= end || *p != '"') return 0; ++p;
  while (p < end) { if (*p == '"') { *out = p + 1; return 1; } if ((unsigned char)*p < 0x20) return 0;
    if (*p == '\\') { ++p; if (p >= end || !strchr("\\\"/bfnrtu", *p)) return 0; if (*p == 'u') { int i; for (i=0;i<4;i++) { ++p; if (p >= end || !isxdigit((unsigned char)*p)) return 0; } } } ++p; }
  return 0;
}
static int jvalue_end(const char* p, const char* end, const char** out) {
  const char* q; int depth;
  p=jws(p,end); if (p>=end) return 0;
  if (*p=='"') return jstring_end(p,end,out);
  if (*p=='{' || *p=='[') { char open=*p, close=open=='{'?'}':']'; depth=1; ++p;
    while (p<end && depth) { if (*p=='"') { if(!jstring_end(p,end,&p)) return 0; continue; }
      if (*p==open) ++depth; else if (*p==close) --depth; ++p; }
    if(depth) return 0; *out=p; return 1; }
  q=p; while(q<end && !isspace((unsigned char)*q) && *q!=',' && *q!='}' && *q!=']') ++q;
  if(q==p) return 0; *out=q; return 1;
}
static int jrange_type(prom_json_range r, char c) { const char* p=jws(r.begin,r.end); return p<r.end && *p==c; }
static int jkey_equals(const char* b, const char* e, const char* wanted) {
  size_t n=strlen(wanted); return (size_t)(e-b)==n && memcmp(b,wanted,n)==0 && memchr(b,'\\',(size_t)(e-b))==NULL;
}
/* Returns a single object member and rejects duplicate fields. */
static int jobject_get(prom_json_range object, const char* wanted, prom_json_range* out, int* found) {
  const char *p=jws(object.begin,object.end), *kb, *ke, *ve; int seen=0;
  if(p>=object.end || *p!='{') return 0; p=jws(p+1,object.end);
  if(p<object.end && *p=='}') { *found=0; return 1; }
  for (;;) { kb=p+1; if(!jstring_end(p,object.end,&ke)) return 0; if(jkey_equals(kb,ke-1,wanted)) { if(seen) return 0; seen=1; }
    p=jws(ke,object.end); if(p>=object.end || *p!=':') return 0; p=jws(p+1,object.end); if(!jvalue_end(p,object.end,&ve)) return 0;
    if(jkey_equals(kb,ke-1,wanted)) { out->begin=p; out->end=ve; }
    p=jws(ve,object.end);
    if(p>=object.end) return 0; if(*p=='}') { *found=seen; return 1; } if(*p!=',') return 0; p=jws(p+1,object.end);
  }
}
static int jstring_dup(prom_json_range r, char** out) {
  const char *p=jws(r.begin,r.end), *e; size_t n; char* s;
  if(!jstring_end(p,r.end,&e) || jws(e,r.end)!=r.end) return 0;
  n=(size_t)((e-1)-(p+1)); if(memchr(p+1,'\\',n)!=NULL) return 0;
  s=(char*)malloc(n+1u); if(s==NULL) return 0; memcpy(s,p+1,n); s[n]='\0'; *out=s; return 1;
}
static int ju64(prom_json_range r, uint64_t* out) {
  const char* p=jws(r.begin,r.end); uint64_t v=0u;
  if(p>=r.end || !isdigit((unsigned char)*p)) return 0;
  if(*p=='0' && p+1<r.end && isdigit((unsigned char)p[1])) return 0;
  while(p<r.end && isdigit((unsigned char)*p)) { uint32_t d=(uint32_t)(*p-'0'); if(v>(UINT64_MAX-d)/10u) return 0; v=v*10u+d; ++p; }
  if(jws(p,r.end)!=r.end) return 0; *out=v; return 1;
}
static int ju32(prom_json_range r, uint32_t* out) { uint64_t v; if(!ju64(r,&v)||v>UINT32_MAX) return 0; *out=(uint32_t)v; return 1; }
static int jbool(prom_json_range r, int* out) { const char* p=jws(r.begin,r.end); if((size_t)(r.end-p)==4u&&memcmp(p,"true",4u)==0){*out=1;return 1;} if((size_t)(r.end-p)==5u&&memcmp(p,"false",5u)==0){*out=0;return 1;} return 0; }
static int jfield(prom_json_range o, const char* name, prom_json_range* value) { int found=0; return jobject_get(o,name,value,&found) && found; }
/* Known schemas are closed: an unknown member is an error, not a hint for an
   individual reactor to interpret later.  All schema member names are ASCII. */
static int jobject_allowed(prom_json_range object, const char* const* names, size_t name_count) {
  const char *p=jws(object.begin,object.end), *key_end, *value_end;
  size_t i;
  if (p>=object.end || *p!='{') return 0;
  p=jws(p+1,object.end);
  if (p<object.end && *p=='}') return 1;
  for (;;) {
    const char* key_begin=p+1;
    int allowed=0;
    if(!jstring_end(p,object.end,&key_end)) return 0;
    for(i=0u;i<name_count;++i) if(jkey_equals(key_begin,key_end-1,names[i])) { allowed=1; break; }
    if(!allowed) return 0;
    p=jws(key_end,object.end); if(p>=object.end||*p!=':') return 0;
    p=jws(p+1,object.end); if(!jvalue_end(p,object.end,&value_end)) return 0;
    p=jws(value_end,object.end); if(p>=object.end) return 0;
    if(*p=='}') return 1; if(*p!=',') return 0; p=jws(p+1,object.end);
  }
}
static int jarray_item(prom_json_range array, size_t index, prom_json_range* out) {
  const char *p=jws(array.begin,array.end), *e; size_t i=0u;
  if(p>=array.end||*p!='[') return 0; p=jws(p+1,array.end); if(p<array.end&&*p==']') return 0;
  for (;;) { if(!jvalue_end(p,array.end,&e)) return 0; if(i==index){out->begin=p;out->end=e;return 1;} ++i; p=jws(e,array.end); if(p>=array.end) return 0; if(*p==']')return 0; if(*p!=',')return 0; p=jws(p+1,array.end); }
}
static int jarray_count(prom_json_range array, size_t* out_count) {
  const char *p=jws(array.begin,array.end), *e; size_t n=0u;
  if(p>=array.end||*p!='[') return 0; p=jws(p+1,array.end); if(p<array.end&&*p==']'){*out_count=0;return 1;}
  for (;;) { if(!jvalue_end(p,array.end,&e)) return 0; ++n; p=jws(e,array.end); if(p>=array.end)return 0; if(*p==']'){*out_count=n;return 1;} if(*p!=',')return 0; p=jws(p+1,array.end); }
}
static int jworkgroup(prom_json_range r, uint32_t out[3]) { size_t n,i; prom_json_range x; if(!jarray_count(r,&n)||n!=3u)return 0; for(i=0;i<3u;++i){if(!jarray_item(r,i,&x)||!ju32(x,&out[i])||out[i]==0u)return 0;} return 1; }
static int prom_digest_valid(const char* d) { size_t i; if(d==NULL||strlen(d)!=64u)return 0; for(i=0u;i<64u;++i)if(!(d[i]>='0'&&d[i]<='9')&&!(d[i]>='a'&&d[i]<='f'))return 0; return 1; }
static int prom_file_read(const char* path, unsigned char** out, size_t* out_size) {
  FILE* f; struct PROM_STAT st; unsigned char* bytes; size_t n;
  *out=NULL;*out_size=0u; if(PROM_STAT(path,&st)!=0 || st.st_size<0) return 0;
  if((uint64_t)st.st_size > (uint64_t)SIZE_MAX) return 0; f=fopen(path,"rb"); if(f==NULL)return 0;
  bytes=(unsigned char*)malloc((size_t)st.st_size==0u?1u:(size_t)st.st_size); if(bytes==NULL){fclose(f);return 0;}
  n=fread(bytes,1u,(size_t)st.st_size,f); if(fclose(f)!=0||n!=(size_t)st.st_size){free(bytes);return 0;} *out=bytes;*out_size=n;return 1;
}

typedef struct prom_sha256 { uint32_t h[8]; uint64_t bits; unsigned char block[64]; size_t used; } prom_sha256;
static uint32_t prom_rotr(uint32_t x,uint32_t n){return(x>>n)|(x<<(32u-n));}
static void prom_sha256_block(prom_sha256* s,const unsigned char* p){
  static const uint32_t k[64]={0x428a2f98u,0x71374491u,0xb5c0fbcfu,0xe9b5dba5u,0x3956c25bu,0x59f111f1u,0x923f82a4u,0xab1c5ed5u,0xd807aa98u,0x12835b01u,0x243185beu,0x550c7dc3u,0x72be5d74u,0x80deb1feu,0x9bdc06a7u,0xc19bf174u,0xe49b69c1u,0xefbe4786u,0x0fc19dc6u,0x240ca1ccu,0x2de92c6fu,0x4a7484aau,0x5cb0a9dcu,0x76f988dau,0x983e5152u,0xa831c66du,0xb00327c8u,0xbf597fc7u,0xc6e00bf3u,0xd5a79147u,0x06ca6351u,0x14292967u,0x27b70a85u,0x2e1b2138u,0x4d2c6dfcu,0x53380d13u,0x650a7354u,0x766a0abbu,0x81c2c92eu,0x92722c85u,0xa2bfe8a1u,0xa81a664bu,0xc24b8b70u,0xc76c51a3u,0xd192e819u,0xd6990624u,0xf40e3585u,0x106aa070u,0x19a4c116u,0x1e376c08u,0x2748774cu,0x34b0bcb5u,0x391c0cb3u,0x4ed8aa4au,0x5b9cca4fu,0x682e6ff3u,0x748f82eeu,0x78a5636fu,0x84c87814u,0x8cc70208u,0x90befffau,0xa4506cebu,0xbef9a3f7u,0xc67178f2u};
  uint32_t w[64],a,b,c,d,e,f,g,h,t1,t2; size_t i;
  for(i=0;i<16u;++i)w[i]=((uint32_t)p[4*i]<<24)|((uint32_t)p[4*i+1]<<16)|((uint32_t)p[4*i+2]<<8)|p[4*i+3];
  for(i=16u;i<64u;++i){uint32_t x=w[i-15],y=w[i-2];w[i]=(prom_rotr(x,7)^prom_rotr(x,18)^(x>>3))+w[i-16]+(prom_rotr(y,17)^prom_rotr(y,19)^(y>>10))+w[i-7];}
  a=s->h[0];b=s->h[1];c=s->h[2];d=s->h[3];e=s->h[4];f=s->h[5];g=s->h[6];h=s->h[7];
  for(i=0;i<64u;++i){uint32_t S1=prom_rotr(e,6)^prom_rotr(e,11)^prom_rotr(e,25),ch=(e&f)^((~e)&g),S0=prom_rotr(a,2)^prom_rotr(a,13)^prom_rotr(a,22),maj=(a&b)^(a&c)^(b&c);t1=h+S1+ch+k[i]+w[i];t2=S0+maj;h=g;g=f;f=e;e=d+t1;d=c;c=b;b=a;a=t1+t2;} s->h[0]+=a;s->h[1]+=b;s->h[2]+=c;s->h[3]+=d;s->h[4]+=e;s->h[5]+=f;s->h[6]+=g;s->h[7]+=h;
}
static void prom_sha256_init(prom_sha256* s){static const uint32_t h[8]={0x6a09e667u,0xbb67ae85u,0x3c6ef372u,0xa54ff53au,0x510e527fu,0x9b05688cu,0x1f83d9abu,0x5be0cd19u};memcpy(s->h,h,sizeof(h));s->bits=0u;s->used=0u;}
static void prom_sha256_update(prom_sha256* s,const unsigned char* p,size_t n){s->bits+=(uint64_t)n*8u;while(n){size_t take=64u-s->used;if(take>n)take=n;memcpy(s->block+s->used,p,take);s->used+=take;p+=take;n-=take;if(s->used==64u){prom_sha256_block(s,s->block);s->used=0u;}}}
static void prom_sha256_final(prom_sha256* s,unsigned char out[32]){size_t i;s->block[s->used++]=0x80u;if(s->used>56u){while(s->used<64u)s->block[s->used++]=0;prom_sha256_block(s,s->block);s->used=0u;}while(s->used<56u)s->block[s->used++]=0;for(i=0;i<8u;++i)s->block[63u-i]=(unsigned char)(s->bits>>(8u*i));prom_sha256_block(s,s->block);for(i=0;i<8u;++i){out[4*i]=(unsigned char)(s->h[i]>>24);out[4*i+1]=(unsigned char)(s->h[i]>>16);out[4*i+2]=(unsigned char)(s->h[i]>>8);out[4*i+3]=(unsigned char)s->h[i];}}
static void prom_hex_digest(const unsigned char in[32],char out[65]){static const char hex[]="0123456789abcdef";size_t i;for(i=0;i<32u;++i){out[2*i]=hex[in[i]>>4];out[2*i+1]=hex[in[i]&15u];}out[64]='\0';}

static int prom_find_kernel(const prom_shader_package* p,uint32_t id){size_t i;for(i=0;i<p->kernel_count;++i)if(p->kernels[i].id==id)return (int)i;return -1;}
static int prom_find_artifact(const prom_shader_package* p,const char* digest){size_t i;for(i=0;i<p->artifact_count;++i)if(strcmp(p->artifacts[i].digest,digest)==0)return (int)i;return -1;}
static int prom_find_variant(const prom_shader_package* p,const char* id){size_t i;for(i=0;i<p->variant_count;++i)if(strcmp(p->variants[i].id,id)==0)return (int)i;return -1;}
static int prom_requirement_kind(const char* s){return strcmp(s,"vulkan_extension")==0||strcmp(s,"vulkan_feature")==0||strcmp(s,"spirv_extension")==0||strcmp(s,"spirv_capability")==0;}
static void prom_free_package_fields(prom_shader_package* p){size_t i;if(p==NULL)return;for(i=0;i<p->kernel_count;++i){free(p->kernels[i].name);free(p->kernels[i].stage);free(p->kernels[i].authority);}for(i=0;i<p->variant_count;++i){free(p->variants[i].id);free(p->variants[i].entry_point);free(p->variants[i].stage_role);}for(i=0;i<p->requirement_count;++i){free(p->requirements[i].kind);free(p->requirements[i].name);}for(i=0;i<p->implementation_count;++i){free(p->implementations[i].name);free(p->implementations[i].authority);}free(p->artifacts);free(p->kernels);free(p->variants);free(p->requirements);free(p->implementations);}
void prom_shader_package_destroy(prom_shader_package* package){if(package==NULL)return;prom_free_package_fields(package);free(package->root);free(package);}
static const char* const k_manifest_fields[] = {"schema", "package", "target", "tables"};
static const char* const k_package_fields[] = {"id", "version", "runtime_abi"};
static const char* const k_target_fields[] = {"object_layout", "media_type"};
static const char* const k_tables_fields[] = {"artifacts", "kernels", "variants", "requirements", "implementations", "provenance"};
static const char* const k_artifact_fields[] = {"sha256", "byte_count", "media_type"};
static const char* const k_kernel_fields[] = {"id", "name", "stage", "authority"};
static const char* const k_variant_fields[] = {"id", "kernel_id", "artifact_sha256", "entry_point", "workgroup", "descriptor_bindings", "push_constant_bytes", "stage_role", "row_width_envelope"};
static const char* const k_requirement_fields[] = {"variant_id", "kind", "name"};
static const char* const k_implementation_fields[] = {"id", "name", "authority", "operation", "variant_id", "dispatch", "benchmark_enabled", "selector_eligible", "dispatchable"};
static const char* const k_provenance_fields[] = {"variant_id", "source", "source_sha256", "generator"};
static int prom_manifest_tables(prom_shader_package* p,prom_json_range tables,prom_shader_package_diagnostic* d){
  prom_json_range a,k,v,r,im,pr,x; size_t i,n; int found;
  if(!jobject_allowed(tables,k_tables_fields,sizeof(k_tables_fields)/sizeof(k_tables_fields[0]))||!jfield(tables,"artifacts",&a)||!jfield(tables,"kernels",&k)||!jfield(tables,"variants",&v)||!jfield(tables,"requirements",&r)||!jfield(tables,"implementations",&im)||!jfield(tables,"provenance",&pr))goto malformed;
  if(!jarray_count(a,&p->artifact_count)||!jarray_count(k,&p->kernel_count)||!jarray_count(v,&p->variant_count)||!jarray_count(r,&p->requirement_count)||!jarray_count(im,&p->implementation_count)||!jarray_count(pr,&n)||p->artifact_count==0u||p->kernel_count==0u||p->variant_count==0u)goto malformed;
  p->artifacts=(prom_package_artifact*)calloc(p->artifact_count,sizeof(*p->artifacts));p->kernels=(prom_package_kernel*)calloc(p->kernel_count,sizeof(*p->kernels));p->variants=(prom_package_variant*)calloc(p->variant_count,sizeof(*p->variants));p->requirements=(prom_package_requirement*)calloc(p->requirement_count?p->requirement_count:1u,sizeof(*p->requirements));p->implementations=(prom_package_implementation*)calloc(p->implementation_count?p->implementation_count:1u,sizeof(*p->implementations));
  if(!p->artifacts||!p->kernels||!p->variants||!p->requirements||!p->implementations){prom_diag(d,PROM_SHADER_PACKAGE_TABLE_INVALID,"package table allocation failed");return 0;}
  for(i=0;i<p->artifact_count;++i){char* digest=NULL;char* media=NULL;uint64_t bytes;if(!jarray_item(a,i,&x)||!jobject_allowed(x,k_artifact_fields,sizeof(k_artifact_fields)/sizeof(k_artifact_fields[0]))||!jfield(x,"sha256",&x)||!jstring_dup(x,&digest)){free(digest);goto malformed;}if(!jobject_get(jarray_item(a,i,&x)?x:x,"byte_count",&x,&found)||!found||!ju64(x,&bytes)||bytes==0u||(bytes&3u)!=0u){free(digest);goto malformed;}if(!jfield(jarray_item(a,i,&x)?x:x,"media_type",&x)||!jstring_dup(x,&media)){free(digest);free(media);goto malformed;}if(!prom_digest_valid(digest)||strcmp(media,"application/vnd.khronos.spirv")!=0){free(digest);free(media);goto invalid;}memcpy(p->artifacts[i].digest,digest,65u);p->artifacts[i].byte_count=bytes;free(digest);free(media);if(prom_find_artifact(p,p->artifacts[i].digest)!=(int)i)goto invalid;}
  for(i=0;i<p->kernel_count;++i){prom_package_kernel* q=&p->kernels[i];if(!jarray_item(k,i,&x)||!jobject_allowed(x,k_kernel_fields,sizeof(k_kernel_fields)/sizeof(k_kernel_fields[0]))||!jfield(x,"id",&x)||!ju32(x,&q->id)||q->id==0u||!jarray_item(k,i,&x)||!jfield(x,"name",&x)||!jstring_dup(x,&q->name)||!jarray_item(k,i,&x)||!jfield(x,"stage",&x)||!jstring_dup(x,&q->stage)||!jarray_item(k,i,&x)||!jfield(x,"authority",&x)||!jstring_dup(x,&q->authority))goto malformed;if(strcmp(q->stage,"compute")!=0||(strcmp(q->authority,"production")!=0&&strcmp(q->authority,"experimental")!=0)||prom_find_kernel(p,q->id)!=(int)i)goto invalid;}
  for(i=0;i<p->variant_count;++i){prom_package_variant* q=&p->variants[i];char* digest=NULL;int artifact;if(!jarray_item(v,i,&x)||!jfield(x,"id",&x)||!jstring_dup(x,&q->id)||!jarray_item(v,i,&x)||!jfield(x,"kernel_id",&x)||!ju32(x,&q->kernel_id)||prom_find_kernel(p,q->kernel_id)<0||!jarray_item(v,i,&x)||!jfield(x,"artifact_sha256",&x)||!jstring_dup(x,&digest)||!jarray_item(v,i,&x)||!jfield(x,"entry_point",&x)||!jstring_dup(x,&q->entry_point)||!jarray_item(v,i,&x)||!jfield(x,"descriptor_bindings",&x)||!ju32(x,&q->descriptor_bindings)||!jarray_item(v,i,&x)||!jfield(x,"push_constant_bytes",&x)||!ju32(x,&q->push_constant_bytes)){free(digest);goto malformed;}artifact=prom_find_artifact(p,digest);free(digest);if(artifact<0||prom_find_variant(p,q->id)!=(int)i)goto invalid;q->artifact_index=(uint32_t)artifact;if(jobject_get(jarray_item(v,i,&x)?x:x,"workgroup",&x,&found)&&found){if(!jworkgroup(x,q->workgroup))goto malformed;q->has_workgroup=1u;}if(jobject_get(jarray_item(v,i,&x)?x:x,"stage_role",&x,&found)&&found&&!jstring_dup(x,&q->stage_role))goto malformed;}
  for(i=0;i<p->requirement_count;++i){char* id=NULL;prom_package_requirement* q=&p->requirements[i];if(!jarray_item(r,i,&x)||!jfield(x,"variant_id",&x)||!jstring_dup(x,&id)||!jarray_item(r,i,&x)||!jfield(x,"kind",&x)||!jstring_dup(x,&q->kind)||!jarray_item(r,i,&x)||!jfield(x,"name",&x)||!jstring_dup(x,&q->name)){free(id);goto malformed;}q->variant_index=(uint32_t)prom_find_variant(p,id);free(id);if(q->variant_index==UINT32_MAX||!prom_requirement_kind(q->kind)||q->name[0]=='\0')goto invalid;}
  for(i=0;i<p->implementation_count;++i){char* id=NULL;prom_package_implementation* q=&p->implementations[i];if(!jarray_item(im,i,&x)||!jfield(x,"id",&x)||!ju32(x,&q->id)||!jarray_item(im,i,&x)||!jfield(x,"name",&x)||!jstring_dup(x,&q->name)||!jarray_item(im,i,&x)||!jfield(x,"variant_id",&x)||!jstring_dup(x,&id)||!jarray_item(im,i,&x)||!jfield(x,"authority",&x)||!jstring_dup(x,&q->authority)){free(id);goto malformed;}q->variant_index=(uint32_t)prom_find_variant(p,id);free(id);if(q->id==0u||q->variant_index==UINT32_MAX||(strcmp(q->authority,"production")!=0&&strcmp(q->authority,"experimental")!=0))goto invalid;{size_t j;for(j=0;j<i;++j)if(q->id==p->implementations[j].id||strcmp(q->name,p->implementations[j].name)==0)goto invalid;}}
  for(i=0;i<n;++i){char* id=NULL;if(!jarray_item(pr,i,&x)||!jfield(x,"variant_id",&x)||!jstring_dup(x,&id)){free(id);goto malformed;}if(prom_find_variant(p,id)<0){free(id);goto invalid;}free(id);}return 1;
malformed:prom_diag(d,PROM_SHADER_PACKAGE_MANIFEST_MALFORMED,"malformed package table");return 0;
invalid:prom_diag(d,PROM_SHADER_PACKAGE_TABLE_INVALID,"invalid package table relationship");return 0;
}

int prom_shader_package_open(const char* root, prom_shader_package** out_package, prom_shader_package_diagnostic* d) {
  char manifest_path[4096]; unsigned char* bytes=NULL; size_t size=0u; prom_json_range top,schema,package,target,tables,value; char* text=NULL; char* id=NULL; char* schema_text=NULL; char* version=NULL; uint32_t abi; prom_shader_package* p=NULL;
  if(out_package==NULL){prom_diag(d,PROM_SHADER_PACKAGE_ROOT_UNAVAILABLE,"package output is unavailable");return 0;}*out_package=NULL;prom_diag(d,PROM_SHADER_PACKAGE_OK,"");
  if(root==NULL||root[0]=='\0'||strlen(root)>3800u){prom_diag(d,PROM_SHADER_PACKAGE_ROOT_UNAVAILABLE,"shader package root is unavailable");return 0;}
  (void)snprintf(manifest_path,sizeof(manifest_path),"%s/manifest.json",root);
  if(!prom_file_read(manifest_path,&bytes,&size)){prom_diag(d,PROM_SHADER_PACKAGE_MANIFEST_UNAVAILABLE,"shader package manifest is unavailable");return 0;}
  text=(char*)malloc(size+1u);if(text==NULL){free(bytes);prom_diag(d,PROM_SHADER_PACKAGE_MANIFEST_MALFORMED,"package manifest allocation failed");return 0;}memcpy(text,bytes,size);text[size]='\0';free(bytes);
  top.begin=text;top.end=text+size;if(!jrange_type(top,'{')||!jobject_allowed(top,k_manifest_fields,sizeof(k_manifest_fields)/sizeof(k_manifest_fields[0]))||!jfield(top,"schema",&schema)||!jstring_dup(schema,&schema_text)||!jfield(top,"package",&package)||!jrange_type(package,'{')||!jobject_allowed(package,k_package_fields,sizeof(k_package_fields)/sizeof(k_package_fields[0]))||!jfield(top,"target",&target)||!jrange_type(target,'{')||!jobject_allowed(target,k_target_fields,sizeof(k_target_fields)/sizeof(k_target_fields[0]))||!jfield(top,"tables",&tables)||!jrange_type(tables,'{'))goto malformed;
  if(strcmp(schema_text,"prometheus.shader-package.v1")!=0){prom_diag(d,PROM_SHADER_PACKAGE_IDENTITY_MISMATCH,"unsupported shader package schema");goto fail;}
  if(!jfield(package,"id",&value)||!jstring_dup(value,&id)||!jfield(package,"version",&value)||!jstring_dup(value,&version)||strcmp(version,"1")!=0||!jfield(package,"runtime_abi",&value)||!ju32(value,&abi)){goto malformed;}
  if(strcmp(id,"prometheus.core")!=0||abi!=1u){prom_diag(d,PROM_SHADER_PACKAGE_IDENTITY_MISMATCH,"shader package identity or runtime ABI mismatch");goto fail;}
  /* Target is factual metadata. Verify its required media type but never use an
     artifact path from the manifest. */
  if(!jfield(target,"object_layout",&value)||!jrange_type(value,'"')||!jfield(target,"media_type",&value)){goto malformed;}{char* media=NULL;if(!jstring_dup(value,&media)){goto malformed;}if(strcmp(media,"application/vnd.khronos.spirv")!=0){free(media);goto malformed;}free(media);}
  p=(prom_shader_package*)calloc(1u,sizeof(*p));if(p==NULL){prom_diag(d,PROM_SHADER_PACKAGE_TABLE_INVALID,"package allocation failed");goto fail;}p->root=(char*)malloc(strlen(root)+1u);if(p->root==NULL){prom_diag(d,PROM_SHADER_PACKAGE_TABLE_INVALID,"package root allocation failed");goto fail;}strcpy(p->root,root);
  if(!prom_manifest_tables(p,tables,d))goto fail;free(schema_text);free(version);free(id);free(text);*out_package=p;return 1;
malformed:prom_diag(d,PROM_SHADER_PACKAGE_MANIFEST_MALFORMED,"malformed shader package manifest");
fail:free(schema_text);free(version);free(id);free(text);prom_shader_package_destroy(p);return 0;
}

static size_t prom_bounded_strlen(const char* text, size_t limit){size_t length=0u;if(text==NULL)return 0u;while(length<limit&&text[length]!='\0')++length;return length;}
static uint32_t prom_word(const unsigned char* p){return (uint32_t)p[0]|((uint32_t)p[1]<<8)|((uint32_t)p[2]<<16)|((uint32_t)p[3]<<24);}
static int prom_spirv_entry_and_local_size(const unsigned char* bytes,size_t size,const prom_package_variant* v){
  size_t at=20u;uint32_t entry_id=0u,local[3]={0u,0u,0u};int entry=0,local_found=0;
  if(size<20u||(size&3u)!=0u||prom_word(bytes)!=0x07230203u)return 0;
  while(at<size){uint32_t instruction=prom_word(bytes+at),words=instruction>>16,opcode=instruction&0xffffu;if(words==0u||at+(size_t)words*4u>size)return 0;
    if(opcode==15u&&words>=4u){const char* name=(const char*)(bytes+at+12u);size_t limit=(size_t)(words-3u)*4u,n=prom_bounded_strlen(name,limit);if(n<limit&&strcmp(name,v->entry_point)==0){entry_id=prom_word(bytes+at+8u);entry=1;}}
    if(opcode==16u&&words>=6u&&entry&&prom_word(bytes+at+4u)==entry_id&&prom_word(bytes+at+8u)==17u){local[0]=prom_word(bytes+at+12u);local[1]=prom_word(bytes+at+16u);local[2]=prom_word(bytes+at+20u);local_found=1;}
    at+=(size_t)words*4u;
  }
  if(!entry)return 0;if(v->has_workgroup&&(!local_found||local[0]!=v->workgroup[0]||local[1]!=v->workgroup[1]||local[2]!=v->workgroup[2]))return 0;return 1;
}
int prom_shader_package_create_module(prom_shader_package* p,VkDevice device,const char* variant_id,VkShaderModule* out_module,const char** out_entry,prom_shader_package_diagnostic* d){
  int vi;prom_package_variant* v;prom_package_artifact* a;char path[4096],actual[65];unsigned char* bytes=NULL,digest[32];size_t size;prom_sha256 sha;VkShaderModuleCreateInfo info;VkResult result;
  if(out_module==NULL||p==NULL||device==VK_NULL_HANDLE||variant_id==NULL){prom_diag(d,PROM_SHADER_PACKAGE_VARIANT_UNAVAILABLE,"shader package module request is invalid");return 0;}*out_module=VK_NULL_HANDLE;if(out_entry!=NULL)*out_entry=NULL;vi=prom_find_variant(p,variant_id);if(vi<0){prom_diag(d,PROM_SHADER_PACKAGE_VARIANT_UNAVAILABLE,"requested shader variant is absent");return 0;}v=&p->variants[vi];a=&p->artifacts[v->artifact_index];
  (void)snprintf(path,sizeof(path),"%s/objects/sha256/%s.spv",p->root,a->digest);++p->artifact_open_count;if(!prom_file_read(path,&bytes,&size)){prom_diag(d,PROM_SHADER_PACKAGE_ARTIFACT_UNAVAILABLE,"shader artifact is unavailable");return 0;}if(size!=(size_t)a->byte_count){free(bytes);prom_diag(d,PROM_SHADER_PACKAGE_ARTIFACT_SIZE_MISMATCH,"shader artifact byte count mismatch");return 0;}prom_sha256_init(&sha);prom_sha256_update(&sha,bytes,size);prom_sha256_final(&sha,digest);prom_hex_digest(digest,actual);if(strcmp(actual,a->digest)!=0){free(bytes);prom_diag(d,PROM_SHADER_PACKAGE_ARTIFACT_DIGEST_MISMATCH,"shader artifact digest mismatch");return 0;}if(!prom_spirv_entry_and_local_size(bytes,size,v)){free(bytes);prom_diag(d,PROM_SHADER_PACKAGE_SPIRV_INVALID,"shader artifact SPIR-V entry point or LocalSize is invalid");return 0;}memset(&info,0,sizeof(info));info.sType=VK_STRUCTURE_TYPE_SHADER_MODULE_CREATE_INFO;info.codeSize=size;info.pCode=(const uint32_t*)bytes;result=vkCreateShaderModule(device,&info,NULL,out_module);free(bytes);if(result!=VK_SUCCESS){prom_diag(d,PROM_SHADER_PACKAGE_VULKAN_FAILURE,"Vulkan shader module creation failed");return 0;}if(out_entry!=NULL)*out_entry=v->entry_point;prom_diag(d,PROM_SHADER_PACKAGE_OK,"");return 1;
}
uint64_t prom_shader_package_artifact_open_count(const prom_shader_package* p){return p==NULL?0u:p->artifact_open_count;}
const char* prom_shader_package_root(const prom_shader_package* p){return p==NULL?NULL:p->root;}
