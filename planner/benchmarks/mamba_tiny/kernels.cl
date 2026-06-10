#pragma OPENCL EXTENSION cl_khr_fp64 : enable

static inline float mb_fast_rsqrt(float x) {
  if (x <= 0.0f) return 0.0f;
  float xhalf = 0.5f * x;
  uint i = as_uint(x);
  i = (uint)0x5f3759df - (i >> 1);
  float y = as_float(i);
  y = y * (1.5f - xhalf * y * y);
  return y;
}

static inline float mb_hard_sigmoid(float x) {
  float y = 0.2f * x + 0.5f;
  if (y < 0.0f) y = 0.0f;
  if (y > 1.0f) y = 1.0f;
  return y;
}

__kernel void mb_rmsnorm(__global const float* In, __global const float* Weight, __global float* Out, int SeqLen, int DModel, int InvDModelQ, float Eps) {
  int row=(int)get_global_id(0); if(row>=SeqLen)return; int base=row*DModel; float mean2=0.0f;
  for(int j=0;j<DModel;j++){ float v=In[base+j]; mean2+=v*v; }
  float invCount = ((float)InvDModelQ) * 0.000001f;
  mean2 *= invCount;
  float inv = mb_fast_rsqrt(mean2 + Eps);
  for(int j=0;j<DModel;j++) Out[base+j]=In[base+j]*inv*Weight[j];
}

__kernel void mb_inproj_x(__global const float* A,
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

__kernel void mb_inproj_z(__global const float* A,
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

__kernel void mb_conv1d(__global const float* In, __global const float* Weight, __global float* Out, int SeqLen, int DModel, int K) {
  int d = (int)get_global_id(0);
  int t = (int)get_global_id(1);
  if (d >= DModel || t >= SeqLen) return;
  int tid = t * DModel + d;
  float sum = 0.0f;
  for (int k = 0; k < K; k++) { int tt = t - k; if (tt < 0) continue; sum += In[tt * DModel + d] * Weight[d * K + k]; }
  Out[tid] = sum;
}

// one work-item per channel, scans the whole sequence (structurally faithful to recurrent selective scan)
__kernel void mb_selective_scan(__global const float* In, __global const float* A, __global const float* B, __global const float* C, __global const float* Delta, __global float* Out, int SeqLen, int DModel) { int d=(int)get_global_id(0); if(d>=DModel)return; float state=0.0f; float a=A[d], b=B[d], c=C[d], delta=Delta[d]; for(int t=0;t<SeqLen;t++){ float x=In[t*DModel+d]; state = state + delta * (-a * state + b * x); Out[t*DModel+d] = c * state; } }
__kernel void mb_gate_mul(__global const float* X, __global const float* Z, __global float* Out, int N) { int tid=(int)get_global_id(0); if(tid>=N)return; float z=Z[tid]; float sig=mb_hard_sigmoid(z); Out[tid] = X[tid] * (z * sig); }
__kernel void mb_outproj(__global const float* A,
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

__kernel void mb_add_resid(__global const float* A, __global const float* B, __global float* Out, int N) { int tid=(int)get_global_id(0); if(tid>=N)return; Out[tid]=A[tid]+B[tid]; }
