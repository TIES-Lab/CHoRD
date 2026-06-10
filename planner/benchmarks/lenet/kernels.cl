// Tiny LeNet kernels for MGPUSim (OpenCL C).
//
// Layout (row-major, flattened):
//   Input:  [Cin, Hin, Win]
//   Weight: [Cout, Cin, K, K]
//   Bias:   [Cout]
//   Output: [Cout, Hout, Wout]
//
// Notes:
//   - Simple (no tiling) for correctness and locality visibility.

__kernel void ln_conv2d(__global const float* In,
                        __global const float* W,
                        __global const float* B,
                        __global float* Out,
                        int Cin, int Hin, int Win,
                        int Cout, int K, int Stride, int Pad,
                        int Hout, int Wout) {
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


__kernel void ln_relu(__global float* X, int N) {
  int i = (int)get_global_id(0);
  if (i >= N) return;
  float v = X[i];
  if (v < 0.0f) v = 0.0f;
  X[i] = v;
}

__kernel void ln_maxpool2d(__global const float* In,
                           __global float* Out,
                           int C, int Hin, int Win,
                           int Pool, int Stride,
                           int Hout, int Wout) {
  int ow = (int)get_global_id(0);
  int yz = (int)get_global_id(1);
  if (ow >= Wout || yz >= C * Hout) return;
  int c = yz / Hout;
  int oh = yz - c * Hout;
  int outIdx = (c * Hout + oh) * Wout + ow;
  float maxv = -1.0e30f;
  for (int ph = 0; ph < Pool; ph++) {
    for (int pw = 0; pw < Pool; pw++) {
      int ih = oh * Stride + ph;
      int iw = ow * Stride + pw;
      if (ih >= Hin || iw >= Win) continue;
      float v = In[(c * Hin + ih) * Win + iw];
      if (v > maxv) maxv = v;
    }
  }
  Out[outIdx] = maxv;
}


// Fully-connected: Out[OutDim] = W[OutDim, InDim] * In[InDim] + B[OutDim]
__kernel void ln_fc(__global const float* In,
                    __global const float* W,
                    __global const float* B,
                    __global float* Out,
                    int InDim, int OutDim) {
  int o = (int)get_global_id(0);
  if (o >= OutDim) return;

  float sum = B[o];
  int base = o * InDim;
  for (int i = 0; i < InDim; i++) {
    sum += W[base + i] * In[i];
  }
  Out[o] = sum;
}
