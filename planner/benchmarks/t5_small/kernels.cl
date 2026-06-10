
#pragma OPENCL EXTENSION cl_khr_fp64 : enable
__kernel void t5_q_proj(__global const float* A,
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

__kernel void t5_k_proj(__global const float* A,
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

__kernel void t5_v_proj(__global const float* A,
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

__kernel void t5_attention_scores(__global const float* Q,
                                 __global const float* K,
                                 __global float* Scores,
                                 int QLen, int KLen, int DHead, int NumHeads) {
  int j = (int)get_global_id(0);
  int row = (int)get_global_id(1);
  if (j >= KLen || row >= NumHeads * QLen) return;
  int h = row / QLen;
  int i = row - h * QLen;
  int tid = (h * QLen + i) * KLen + j;

  int dModel = NumHeads * DHead;
  float sum = 0.0f;
  int qBase = i * dModel + h * DHead;
  int kBase = j * dModel + h * DHead;
  for (int k = 0; k < DHead; k++) { sum += Q[qBase + k] * K[kBase + k]; }
  Scores[tid] = sum * (1.0f / sqrt((float)DHead));
}

__kernel void t5_attention_scores_masked(__global const float* Q,
                                 __global const float* K,
                                 __global float* Scores,
                                 int QLen, int KLen, int DHead, int NumHeads) {
  int j = (int)get_global_id(0);
  int row = (int)get_global_id(1);
  if (j >= KLen || row >= NumHeads * QLen) return;
  int h = row / QLen;
  int i = row - h * QLen;
  int tid = (h * QLen + i) * KLen + j;
  if (j > i) { Scores[tid] = -1.0e30f; return; }
  int dModel = NumHeads * DHead;
  float sum = 0.0f;
  int qBase = i * dModel + h * DHead;
  int kBase = j * dModel + h * DHead;
  for (int k = 0; k < DHead; k++) { sum += Q[qBase + k] * K[kBase + k]; }
  Scores[tid] = sum * (1.0f / sqrt((float)DHead));
}

__kernel void t5_softmax(__global const float* In,
                     __global float* Out,
                     int QLen, int KLen, int NumHeads) {
  int i = (int)get_global_id(0);
  int h = (int)get_global_id(1);
  if (i >= QLen || h >= NumHeads) return;
  int base = (h * QLen + i) * KLen;
  float maxv = -1.0e30f;
  for (int j = 0; j < KLen; j++) { float v = In[base + j]; if (v > maxv) maxv = v; }
  float sum = 0.0f;
  for (int j = 0; j < KLen; j++) { float e = exp(In[base + j] - maxv); Out[base + j] = e; sum += e; }
  float inv = 1.0f / sum;
  for (int j = 0; j < KLen; j++) { Out[base + j] *= inv; }
}

__kernel void t5_attention_context(__global const float* Probs,
                     __global const float* V,
                     __global float* Context,
                     int QLen, int KLen, int DHead, int NumHeads) {
  int k = (int)get_global_id(0);
  int row = (int)get_global_id(1);
  if (k >= DHead || row >= NumHeads * QLen) return;
  int h = row / QLen;
  int i = row - h * QLen;
  int dModel = NumHeads * DHead;
  int outIdx = i * dModel + h * DHead + k;
  int pBase = (h * QLen + i) * KLen;
  float sum = 0.0f;
  for (int j = 0; j < KLen; j++) { sum += Probs[pBase + j] * V[j * dModel + h * DHead + k]; }
  Context[outIdx] = sum;
}

__kernel void t5_o_proj(__global const float* A,
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

__kernel void t5_add_resid1(__global const float* A, __global const float* B, __global float* Out, int N) { int tid=(int)get_global_id(0); if(tid>=N)return; Out[tid]=A[tid]+B[tid]; }
__kernel void t5_add_resid_x(__global const float* A, __global const float* B, __global float* Out, int N) { int tid=(int)get_global_id(0); if(tid>=N)return; Out[tid]=A[tid]+B[tid]; }
__kernel void t5_add_resid2(__global const float* A, __global const float* B, __global float* Out, int N) { int tid=(int)get_global_id(0); if(tid>=N)return; Out[tid]=A[tid]+B[tid]; }
__kernel void t5_layernorm1(__global const float* In, __global const float* Gamma, __global const float* Beta, __global float* Out, int SeqLen, int DModel, float Eps) { int row=(int)get_global_id(0); if(row>=SeqLen)return; int base=row*DModel; float mean=0.0f; for(int j=0;j<DModel;j++) mean+=In[base+j]; mean*=native_recip((float)DModel); float var=0.0f; for(int j=0;j<DModel;j++){ float d=In[base+j]-mean; var+=d*d; } var*=native_recip((float)DModel); float inv=native_rsqrt(var+Eps); for(int j=0;j<DModel;j++) Out[base+j] = (In[base+j]-mean)*inv*Gamma[j]+Beta[j]; }
__kernel void t5_layernorm_x(__global const float* In, __global const float* Gamma, __global const float* Beta, __global float* Out, int SeqLen, int DModel, float Eps) { int row=(int)get_global_id(0); if(row>=SeqLen)return; int base=row*DModel; float mean=0.0f; for(int j=0;j<DModel;j++) mean+=In[base+j]; mean*=native_recip((float)DModel); float var=0.0f; for(int j=0;j<DModel;j++){ float d=In[base+j]-mean; var+=d*d; } var*=native_recip((float)DModel); float inv=native_rsqrt(var+Eps); for(int j=0;j<DModel;j++) Out[base+j] = (In[base+j]-mean)*inv*Gamma[j]+Beta[j]; }
__kernel void t5_layernorm2(__global const float* In, __global const float* Gamma, __global const float* Beta, __global float* Out, int SeqLen, int DModel, float Eps) { int row=(int)get_global_id(0); if(row>=SeqLen)return; int base=row*DModel; float mean=0.0f; for(int j=0;j<DModel;j++) mean+=In[base+j]; mean*=native_recip((float)DModel); float var=0.0f; for(int j=0;j<DModel;j++){ float d=In[base+j]-mean; var+=d*d; } var*=native_recip((float)DModel); float inv=native_rsqrt(var+Eps); for(int j=0;j<DModel;j++) Out[base+j] = (In[base+j]-mean)*inv*Gamma[j]+Beta[j]; }
__kernel void t5_ff1(__global const float* A,
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

__kernel void t5_relu(__global float* X, int N) { int tid=(int)get_global_id(0); if(tid>=N)return; if(X[tid]<0.0f) X[tid]=0.0f; }
__kernel void t5_ff2(__global const float* A,
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

__kernel void t5_lm_head(__global const float* A,
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

