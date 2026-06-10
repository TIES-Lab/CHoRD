
#pragma OPENCL EXTENSION cl_khr_fp64 : enable
__kernel void sw_q_proj(__global const float* A,
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

__kernel void sw_k_proj(__global const float* A,
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

__kernel void sw_v_proj(__global const float* A,
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

__kernel void sw_attention_scores(__global const float* Q,
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

__kernel void sw_softmax(__global const float* In,
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

__kernel void sw_attention_context(__global const float* Probs,
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

__kernel void sw_o_proj(__global const float* A,
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

__kernel void sw_add_resid1(__global const float* A, __global const float* B, __global float* Out, int N) { int tid=(int)get_global_id(0); if(tid>=N)return; Out[tid]=A[tid]+B[tid]; }
__kernel void sw_add_resid2(__global const float* A, __global const float* B, __global float* Out, int N) { int tid=(int)get_global_id(0); if(tid>=N)return; Out[tid]=A[tid]+B[tid]; }
__kernel void sw_layernorm1(__global const float* In, __global const float* Gamma, __global const float* Beta, __global float* Out, int SeqLen, int DModel, float Eps) { int row=(int)get_global_id(0); if(row>=SeqLen)return; int base=row*DModel; float mean=0.0f; for(int j=0;j<DModel;j++) mean+=In[base+j]; mean*=native_recip((float)DModel); float var=0.0f; for(int j=0;j<DModel;j++){ float d=In[base+j]-mean; var+=d*d; } var*=native_recip((float)DModel); float inv=native_rsqrt(var+Eps); for(int j=0;j<DModel;j++) Out[base+j]=(In[base+j]-mean)*inv*Gamma[j]+Beta[j]; }
__kernel void sw_layernorm2(__global const float* In, __global const float* Gamma, __global const float* Beta, __global float* Out, int SeqLen, int DModel, float Eps) { int row=(int)get_global_id(0); if(row>=SeqLen)return; int base=row*DModel; float mean=0.0f; for(int j=0;j<DModel;j++) mean+=In[base+j]; mean*=native_recip((float)DModel); float var=0.0f; for(int j=0;j<DModel;j++){ float d=In[base+j]-mean; var+=d*d; } var*=native_recip((float)DModel); float inv=native_rsqrt(var+Eps); for(int j=0;j<DModel;j++) Out[base+j]=(In[base+j]-mean)*inv*Gamma[j]+Beta[j]; }
__kernel void sw_router(__global const float* In, __global const float* W, __global float* Routes, int SeqLen, int DModel, int Experts) { int row=(int)get_global_id(0); if(row>=SeqLen)return; int base=row*DModel; int best=0; float bestv=-1.0e30f; for(int e=0;e<Experts;e++){ float sum=0.0f; for(int k=0;k<DModel;k++) sum += In[base+k]*W[k*Experts+e]; if(sum>bestv){ bestv=sum; best=e; } } Routes[row]=(float)best; }
__kernel void sw_expert_ff1(__global const float* In, __global const float* Routes, __global const float* W, __global float* Out, int SeqLen, int DModel, int DFF, int ExpertID) {
  int col = (int)get_global_id(0);
  int row = (int)get_global_id(1);
  if (col >= DFF || row >= SeqLen) return;
  int tid = row * DFF + col;
  if ((int)Routes[row] != ExpertID) { Out[tid] = 0.0f; return; }
  float sum = 0.0f;
  int abase = row * DModel;
  for (int k = 0; k < DModel; k++) sum += In[abase + k] * W[k * DFF + col];
  if (sum < 0.0f) sum = 0.0f;
  Out[tid] = sum;
}

__kernel void sw_expert_ff2(__global const float* In, __global const float* Routes, __global const float* W, __global float* Out, int SeqLen, int DFF, int DModel, int ExpertID) {
  int col = (int)get_global_id(0);
  int row = (int)get_global_id(1);
  if (col >= DModel || row >= SeqLen) return;
  if ((int)Routes[row] != ExpertID) return;
  float sum = 0.0f;
  int abase = row * DFF;
  for (int k = 0; k < DFF; k++) sum += In[abase + k] * W[k * DModel + col];
  Out[row * DModel + col] = sum;
}

