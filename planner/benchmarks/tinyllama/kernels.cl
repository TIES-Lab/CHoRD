#pragma OPENCL EXTENSION cl_khr_fp64 : enable

__kernel void tlm_q_proj(__global const float* A,
                        __global const float* B,
                        __global float* C,
                        int M, int N, int K) {
  int col = (int)get_global_id(0);
  int row = (int)get_global_id(1);
  if (row >= M || col >= N) return;
  float sum = 0.0f;
  int aBase = row * K;
  for (int k = 0; k < K; k++) { sum += A[aBase + k] * B[k * N + col]; }
  C[row * N + col] = sum;
}


__kernel void tlm_k_proj(__global const float* A,
                        __global const float* B,
                        __global float* C,
                        int M, int N, int K) {
  int col = (int)get_global_id(0);
  int row = (int)get_global_id(1);
  if (row >= M || col >= N) return;
  float sum = 0.0f;
  int aBase = row * K;
  for (int k = 0; k < K; k++) { sum += A[aBase + k] * B[k * N + col]; }
  C[row * N + col] = sum;
}


__kernel void tlm_v_proj(__global const float* A,
                        __global const float* B,
                        __global float* C,
                        int M, int N, int K) {
  int col = (int)get_global_id(0);
  int row = (int)get_global_id(1);
  if (row >= M || col >= N) return;
  float sum = 0.0f;
  int aBase = row * K;
  for (int k = 0; k < K; k++) { sum += A[aBase + k] * B[k * N + col]; }
  C[row * N + col] = sum;
}


// RoPE with host-precomputed cos/sin tables.
// Tables are shaped [SeqLen, DHead/2] and shared across heads.
__kernel void tlm_rope_q(__global float* X,
                     __global const float* Cos,
                     __global const float* Sin,
                     int SeqLen, int DModel, int NumHeads) {
  int col = (int)get_global_id(0);
  int row = (int)get_global_id(1);
  if (row >= SeqLen || col >= DModel) return;
  int dh = DModel / NumHeads;
  int inner = col % dh;
  if ((inner & 1) != 0) return;
  int idx0 = row * DModel + col;
  int idx1 = idx0 + 1;
  if (idx1 >= (row + 1) * DModel) return;
  int halfDim = dh >> 1;
  int pair = inner >> 1;
  int ropeBase = row * halfDim + pair;
  float c = Cos[ropeBase];
  float s = Sin[ropeBase];
  float x0 = X[idx0];
  float x1 = X[idx1];
  X[idx0] = x0 * c + (-(x1 * s));
  X[idx1] = x0 * s + x1 * c;
}


__kernel void tlm_rope_k(__global float* X,
                     __global const float* Cos,
                     __global const float* Sin,
                     int SeqLen, int DModel, int NumHeads) {
  int col = (int)get_global_id(0);
  int row = (int)get_global_id(1);
  if (row >= SeqLen || col >= DModel) return;
  int dh = DModel / NumHeads;
  int inner = col % dh;
  if ((inner & 1) != 0) return;
  int idx0 = row * DModel + col;
  int idx1 = idx0 + 1;
  if (idx1 >= (row + 1) * DModel) return;
  int halfDim = dh >> 1;
  int pair = inner >> 1;
  int ropeBase = row * halfDim + pair;
  float c = Cos[ropeBase];
  float s = Sin[ropeBase];
  float x0 = X[idx0];
  float x1 = X[idx1];
  X[idx0] = x0 * c + (-(x1 * s));
  X[idx1] = x0 * s + x1 * c;
}


__kernel void tlm_attention_scores_masked(__global const float* Q,
                                        __global const float* K,
                                        __global float* Scores,
                                        int SeqLen, int DHead, int NumHeads) {
  int j = (int)get_global_id(0);
  int row = (int)get_global_id(1);
  if (j >= SeqLen || row >= NumHeads * SeqLen) return;
  int h = row / SeqLen;
  int i = row - h * SeqLen;
  int tid = (h * SeqLen + i) * SeqLen + j;
  if (j > i) { Scores[tid] = -1.0e30f; return; }
  int dModel = NumHeads * DHead;
  float sum = 0.0f;
  int qBase = i * dModel + h * DHead;
  int kBase = j * dModel + h * DHead;
  for (int k = 0; k < DHead; k++) { sum += Q[qBase + k] * K[kBase + k]; }
  float scale = native_rsqrt((float)DHead);
  Scores[tid] = sum * scale;
}


__kernel void tlm_softmax(__global const float* In,
                         __global float* Out,
                         int SeqLen, int NumHeads) {
  int i = (int)get_global_id(0);
  int h = (int)get_global_id(1);
  if (i >= SeqLen || h >= NumHeads) return;
  int base = (h * SeqLen + i) * SeqLen;
  float maxv = -1.0e30f;
  for (int j = 0; j < SeqLen; j++) { float v = In[base + j]; if (v > maxv) maxv = v; }
  float sumExp = 0.0f;
  for (int j = 0; j < SeqLen; j++) { float shifted = In[base + j] + (-maxv); float e = exp(shifted); Out[base + j] = e; sumExp += e; }
  float inv = native_recip(sumExp);
  for (int j = 0; j < SeqLen; j++) { Out[base + j] *= inv; }
}


__kernel void tlm_attention_context(__global const float* Probs,
                                  __global const float* V,
                                  __global float* Context,
                                  int SeqLen, int DHead, int NumHeads) {
  int k = (int)get_global_id(0);
  int row = (int)get_global_id(1);
  if (k >= DHead || row >= NumHeads * SeqLen) return;
  int h = row / SeqLen;
  int i = row - h * SeqLen;
  int dModel = NumHeads * DHead;
  int outIdx = i * dModel + h * DHead + k;
  int pBase = (h * SeqLen + i) * SeqLen;
  float sum = 0.0f;
  for (int j = 0; j < SeqLen; j++) { sum += Probs[pBase + j] * V[j * dModel + h * DHead + k]; }
  Context[outIdx] = sum;
}


__kernel void tlm_o_proj(__global const float* A,
                        __global const float* B,
                        __global float* C,
                        int M, int N, int K) {
  int col = (int)get_global_id(0);
  int row = (int)get_global_id(1);
  if (row >= M || col >= N) return;
  float sum = 0.0f;
  int aBase = row * K;
  for (int k = 0; k < K; k++) { sum += A[aBase + k] * B[k * N + col]; }
  C[row * N + col] = sum;
}


__kernel void tlm_add_resid1(__global const float* A,
                             __global const float* B,
                             __global float* Out,
                             int N) {
  int tid = (int)get_global_id(0);
  if (tid >= N) return;
  Out[tid] = A[tid] + B[tid];
}

__kernel void tlm_add_resid2(__global const float* A,
                             __global const float* B,
                             __global float* Out,
                             int N) {
  int tid = (int)get_global_id(0);
  if (tid >= N) return;
  Out[tid] = A[tid] + B[tid];
}

__kernel void tlm_rmsnorm1(__global const float* In,
                           __global const float* Weight,
                           __global float* Out,
                           int SeqLen, int DModel, float Eps) {
  int row = (int)get_global_id(0);
  if (row >= SeqLen) return;

  int base = row * DModel;
  float mean2 = 0.0f;
  for (int j = 0; j < DModel; j++) {
    float v = In[base + j];
    mean2 += v * v;
  }
  mean2 *= native_recip((float)DModel);
  float inv = native_rsqrt(mean2 + Eps);
  for (int j = 0; j < DModel; j++) {
    Out[base + j] = In[base + j] * inv * Weight[j];
  }
}

__kernel void tlm_rmsnorm2(__global const float* In,
                           __global const float* Weight,
                           __global float* Out,
                           int SeqLen, int DModel, float Eps) {
  int row = (int)get_global_id(0);
  if (row >= SeqLen) return;

  int base = row * DModel;
  float mean2 = 0.0f;
  for (int j = 0; j < DModel; j++) {
    float v = In[base + j];
    mean2 += v * v;
  }
  mean2 *= native_recip((float)DModel);
  float inv = native_rsqrt(mean2 + Eps);
  for (int j = 0; j < DModel; j++) {
    Out[base + j] = In[base + j] * inv * Weight[j];
  }
}

__kernel void tlm_ff_gate(__global const float* A,
                        __global const float* B,
                        __global float* C,
                        int M, int N, int K) {
  int col = (int)get_global_id(0);
  int row = (int)get_global_id(1);
  if (row >= M || col >= N) return;
  float sum = 0.0f;
  int aBase = row * K;
  for (int k = 0; k < K; k++) { sum += A[aBase + k] * B[k * N + col]; }
  C[row * N + col] = sum;
}


__kernel void tlm_ff_up(__global const float* A,
                        __global const float* B,
                        __global float* C,
                        int M, int N, int K) {
  int col = (int)get_global_id(0);
  int row = (int)get_global_id(1);
  if (row >= M || col >= N) return;
  float sum = 0.0f;
  int aBase = row * K;
  for (int k = 0; k < K; k++) { sum += A[aBase + k] * B[k * N + col]; }
  C[row * N + col] = sum;
}


__kernel void tlm_swiglu(__global const float* Gate,
                         __global const float* Up,
                         __global float* Out,
                         int N) {
  int tid = (int)get_global_id(0);
  if (tid >= N) return;
  float g = Gate[tid];
  float sig = native_recip(1.0f + exp(-g));
  Out[tid] = (g * sig) * Up[tid];
}

__kernel void tlm_ff_down(__global const float* A,
                        __global const float* B,
                        __global float* C,
                        int M, int N, int K) {
  int col = (int)get_global_id(0);
  int row = (int)get_global_id(1);
  if (row >= M || col >= N) return;
  float sum = 0.0f;
  int aBase = row * K;
  for (int k = 0; k < K; k++) { sum += A[aBase + k] * B[k * N + col]; }
  C[row * N + col] = sum;
}

