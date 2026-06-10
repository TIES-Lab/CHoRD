
__kernel void rn_conv2d(__global const float* In, __global const float* W, __global const float* B, __global float* Out,
                        int Cin, int Hin, int Win, int Cout, int K, int Stride, int Pad, int Hout, int Wout) {
  int ow = (int)get_global_id(0);
  int yz = (int)get_global_id(1);
  if (ow >= Wout || yz >= Cout * Hout) return;
  int co = yz / Hout;
  int oh = yz - co * Hout;
  int outIdx = (co * Hout + oh) * Wout + ow;
  float sum = B[co];
  for (int ci = 0; ci < Cin; ci++) {
    for (int kh = 0; kh < K; kh++) {
      for (int kw = 0; kw < K; kw++) {
        int ih = oh * Stride + kh - Pad;
        int iw = ow * Stride + kw - Pad;
        if (ih < 0 || ih >= Hin || iw < 0 || iw >= Win) continue;
        int inIdx = (ci * Hin + ih) * Win + iw;
        int wIdx = ((co * Cin + ci) * K + kh) * K + kw;
        sum += In[inIdx] * W[wIdx];
      }
    }
  }
  Out[outIdx] = sum;
}

__kernel void rn_relu(__global float* X, int N) {
  int i = (int)get_global_id(0); if (i >= N) return; float v = X[i]; if (v < 0.0f) v = 0.0f; X[i] = v;
}
__kernel void rn_add_relu(__global const float* A, __global const float* B, __global float* Out, int N) {
  int i = (int)get_global_id(0); if (i >= N) return; float v = A[i] + B[i]; if (v < 0.0f) v = 0.0f; Out[i] = v;
}
// Stage 1 GAP: each work-item handles one (channel, tile) chunk.
__kernel void rn_gap_partial(__global const float* In, __global float* Partial, int C, int H, int W, int Tiles) {
  int t = (int)get_global_id(0);
  int c = (int)get_global_id(1);
  if (t >= Tiles || c >= C) return;
  int tid = c * Tiles + t;
  int hw = H * W;
  int start = (t * hw) / Tiles;
  int end = ((t + 1) * hw) / Tiles;
  int base = c * hw;
  float sum = 0.0f;
  for (int i = start; i < end; i++) sum += In[base + i];
  Partial[tid] = sum;
}

__kernel void rn_gap_finalize(__global const float* Partial, __global float* Out, int C, int Tiles, float Denom) {
  int c = (int)get_global_id(0);
  if (c >= C) return;
  float sum = 0.0f;
  int base = c * Tiles;
  for (int t = 0; t < Tiles; t++) sum += Partial[base + t];
  Out[c] = sum / Denom;
}
__kernel void rn_fc(__global const float* In, __global const float* W, __global const float* B, __global float* Out,
                    int InDim, int OutDim) {
  int o = (int)get_global_id(0);
  if (o >= OutDim) return;
  float sum = B[o];
  int base = o * InDim;
  for (int i = 0; i < InDim; i++) sum += W[base + i] * In[i];
  Out[o] = sum;
}
