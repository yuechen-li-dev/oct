[[vk::binding(0, 0)]] StructuredBuffer<float> Input;
[[vk::binding(1, 0)]] RWStructuredBuffer<float> Output;

[numthreads(1, 1, 1)]
void main(uint3 id : SV_DispatchThreadID) {
    Output[id.x] = Input[id.x];
}
