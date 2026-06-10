// Minimal "real Transformer" kernels for MGPUSim.
//
// Layout conventions (row-major):
//   X, Q, K, V, Context, Out, Y*: [seq_len, d_model]
//     with heads interleaved inside d_model: index = i*d_model + h*d_head + k
//
//   Scores/Probs: [num_heads, seq_len, seq_len]
//     index = h*seq_len*seq_len + i*seq_len + j
//
// This file intentionally uses simple kernels (no heavy tiling) to keep
// correctness and make memory placement effects visible.

#pragma OPENCL EXTENSION cl_khr_fp64 : enable

__kernel void bt_q_proj(__global const float* A,
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


__kernel void bt_k_proj(__global const float* A,
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


__kernel void bt_v_proj(__global const float* A,
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


// scores[h,i,j] = dot(Q[i,h,:], K[j,h,:]) / sqrt(d_head)
__kernel void bt_attention_scores(__global const float* Q,
                                 __global const float* K,
                                 __global float* Scores,
                                 int SeqLen, int DHead, int NumHeads) {
  int j = (int)get_global_id(0);
  int row = (int)get_global_id(1);
  if (j >= SeqLen || row >= NumHeads * SeqLen) return;
  int h = row / SeqLen;
  int i = row - h * SeqLen;
  int tid = (h * SeqLen + i) * SeqLen + j;
  int dModel = NumHeads * DHead;
  float sum = 0.0f;
  int qBase = i * dModel + h * DHead;
  int kBase = j * dModel + h * DHead;
  for (int k = 0; k < DHead; k++) { sum += Q[qBase + k] * K[kBase + k]; }
  Scores[tid] = sum * (1.0f / sqrt((float)DHead));
}


// probs = softmax(scores) along j for each (h,i).
// One work-item handles one row (h,i).
__kernel void bt_softmax(__global const float* In,
                         __global float* Out,
                         int SeqLen, int NumHeads) {
  int i = (int)get_global_id(0);
  int h = (int)get_global_id(1);
  if (i >= SeqLen || h >= NumHeads) return;
  int base = (h * SeqLen + i) * SeqLen;
  float maxv = -1.0e30f;
  for (int j = 0; j < SeqLen; j++) { float v = In[base + j]; if (v > maxv) maxv = v; }
  float sumExp = 0.0f;
  for (int j = 0; j < SeqLen; j++) { float e = exp(In[base + j] - maxv); Out[base + j] = e; sumExp += e; }
  float inv = 1.0f / sumExp;
  for (int j = 0; j < SeqLen; j++) { Out[base + j] *= inv; }
}


// context[i,h,k] = sum_j probs[h,i,j] * V[j,h,k]
// Flattened output has shape [SeqLen, DModel].
__kernel void bt_attention_context(__global const float* Probs,
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


// Output projection: out = context * Wo
__kernel void bt_o_proj(__global const float* A,
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


__kernel void bt_add_resid1(__global const float* A,
                            __global const float* B,
                            __global float* Out,
                            int N) {
  int tid = (int)get_global_id(0);
  if (tid >= N) return;
  Out[tid] = A[tid] + B[tid];
}

__kernel void bt_add_resid2(__global const float* A,
                            __global const float* B,
                            __global float* Out,
                            int N) {
  int tid = (int)get_global_id(0);
  if (tid >= N) return;
  Out[tid] = A[tid] + B[tid];
}

// LayerNorm over the last dimension (DModel) for each token row.
// One work-item processes one row (SeqLen).
__kernel void bt_layernorm1(__global const float* In,
                            __global const float* Gamma,
                            __global const float* Beta,
                            __global float* Out,
                            int SeqLen, int DModel, float Eps) {
  int row = (int)get_global_id(0);
  if (row >= SeqLen) return;

  int base = row * DModel;

  float mean = 0.0f;
  for (int j = 0; j < DModel; j++) {
    mean += In[base + j];
  }
  mean = mean / (float)DModel;

  float var = 0.0f;
  for (int j = 0; j < DModel; j++) {
    float d = In[base + j] - mean;
    var += d * d;
  }
  var = var / (float)DModel;

  float invStd = 1.0f / sqrt(var + Eps);

  for (int j = 0; j < DModel; j++) {
    float n = (In[base + j] - mean) * invStd;
    Out[base + j] = n * Gamma[j] + Beta[j];
  }
}

__kernel void bt_layernorm2(__global const float* In,
                            __global const float* Gamma,
                            __global const float* Beta,
                            __global float* Out,
                            int SeqLen, int DModel, float Eps) {
  int row = (int)get_global_id(0);
  if (row >= SeqLen) return;

  int base = row * DModel;

  float mean = 0.0f;
  for (int j = 0; j < DModel; j++) {
    mean += In[base + j];
  }
  mean = mean / (float)DModel;

  float var = 0.0f;
  for (int j = 0; j < DModel; j++) {
    float d = In[base + j] - mean;
    var += d * d;
  }
  var = var / (float)DModel;

  float invStd = 1.0f / sqrt(var + Eps);

  for (int j = 0; j < DModel; j++) {
    float n = (In[base + j] - mean) * invStd;
    Out[base + j] = n * Gamma[j] + Beta[j];
  }
}

// FFN GEMM 1: (seq x dm) * (dm x dff) -> (seq x dff)
__kernel void bt_ff1(__global const float* A,
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


__kernel void bt_relu(__global float* X, int N) {
  int tid = (int)get_global_id(0);
  if (tid >= N) return;
  float v = X[tid];
  if (v < 0.0f) v = 0.0f;
  X[tid] = v;
}

// FFN GEMM 2: (seq x dff) * (dff x dm) -> (seq x dm)
__kernel void bt_ff2(__global const float* A,
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

