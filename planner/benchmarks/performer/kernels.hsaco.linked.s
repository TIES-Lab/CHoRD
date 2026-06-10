	.text
	.hsa_code_object_version 2,1
	.hsa_code_object_isa 8,0,3,"AMD","AMDGPU"
	.protected	pf_q_proj       ; -- Begin function pf_q_proj
	.globl	pf_q_proj
	.p2align	8
	.type	pf_q_proj,@function
	.amdgpu_hsa_kernel pf_q_proj
pf_q_proj:                              ; @pf_q_proj
pf_q_proj$local:
	.amd_kernel_code_t
		amd_code_version_major = 1
		amd_code_version_minor = 2
		amd_machine_kind = 1
		amd_machine_version_major = 8
		amd_machine_version_minor = 0
		amd_machine_version_stepping = 3
		kernel_code_entry_byte_offset = 256
		kernel_code_prefetch_byte_size = 0
		granulated_workitem_vgpr_count = 2
		granulated_wavefront_sgpr_count = 2
		priority = 0
		float_mode = 192
		priv = 0
		enable_dx10_clamp = 1
		debug_mode = 0
		enable_ieee_mode = 1
		enable_wgp_mode = 0
		enable_mem_ordered = 0
		enable_fwd_progress = 0
		enable_sgpr_private_segment_wave_byte_offset = 0
		user_sgpr_count = 8
		enable_trap_handler = 0
		enable_sgpr_workgroup_id_x = 1
		enable_sgpr_workgroup_id_y = 0
		enable_sgpr_workgroup_id_z = 0
		enable_sgpr_workgroup_info = 0
		enable_vgpr_workitem_id = 0
		enable_exception_msb = 0
		granulated_lds_size = 0
		enable_exception = 0
		enable_sgpr_private_segment_buffer = 1
		enable_sgpr_dispatch_ptr = 1
		enable_sgpr_queue_ptr = 0
		enable_sgpr_kernarg_segment_ptr = 1
		enable_sgpr_dispatch_id = 0
		enable_sgpr_flat_scratch_init = 0
		enable_sgpr_private_segment_size = 0
		enable_sgpr_grid_workgroup_count_x = 0
		enable_sgpr_grid_workgroup_count_y = 0
		enable_sgpr_grid_workgroup_count_z = 0
		enable_wavefront_size32 = 0
		enable_ordered_append_gds = 0
		private_element_size = 1
		is_ptr64 = 1
		is_dynamic_callstack = 0
		is_debug_enabled = 0
		is_xnack_enabled = 0
		workitem_private_segment_byte_size = 0
		workgroup_group_segment_byte_size = 0
		gds_segment_byte_size = 0
		kernarg_segment_byte_size = 96
		workgroup_fbarrier_count = 0
		wavefront_sgpr_count = 18
		workitem_vgpr_count = 10
		reserved_vgpr_first = 0
		reserved_vgpr_count = 0
		reserved_sgpr_first = 0
		reserved_sgpr_count = 0
		debug_wavefront_private_segment_offset_sgpr = 0
		debug_private_segment_buffer_sgpr = 0
		kernarg_segment_alignment = 4
		group_segment_alignment = 4
		private_segment_alignment = 4
		wavefront_size = 6
		call_convention = -1
		runtime_loader_kernel_symbol = 0
	.end_amd_kernel_code_t
; %bb.0:
	s_load_dwordx4 s[12:15], s[6:7], 0x18
	s_load_dwordx2 s[2:3], s[6:7], 0x28
	s_load_dword s0, s[4:5], 0x4
	s_waitcnt lgkmcnt(0)
	s_and_b32 s0, s0, 0xffff
	s_mul_i32 s3, s8, s0
	v_add_u32_e32 v1, vcc, s3, v0
	v_add_u32_e32 v1, vcc, s2, v1
	s_mul_i32 s0, s13, s12
	v_cmp_gt_i32_e32 vcc, s0, v1
	s_and_saveexec_b64 s[0:1], vcc
	s_cbranch_execz BB0_5
; %bb.1:
	s_load_dwordx4 s[8:11], s[6:7], 0x10
	s_cmp_lt_i32 s14, 1
	v_mov_b32_e32 v2, 0
	s_cbranch_scc1 BB0_4
; %bb.2:                                ; %.lr.ph.preheader
	s_waitcnt lgkmcnt(0)
	s_ashr_i32 s10, s13, 31
	s_add_i32 s0, s13, s10
	s_xor_b32 s11, s0, s10
	v_cvt_f32_u32_e32 v2, s11
	v_ashrrev_i32_e32 v5, 31, v1
	s_load_dwordx4 s[4:7], s[6:7], 0x0
	v_rcp_iflag_f32_e32 v2, v2
	v_mul_f32_e32 v2, 0x4f800000, v2
	v_cvt_u32_f32_e32 v2, v2
	v_mul_lo_u32 v3, v2, s11
	v_mul_hi_u32 v4, v2, s11
	v_sub_u32_e32 v6, vcc, 0, v3
	v_cmp_eq_u32_e64 s[0:1], 0, v4
	v_cndmask_b32_e64 v3, v3, v6, s[0:1]
	v_mul_hi_u32 v3, v3, v2
	v_add_u32_e32 v4, vcc, v1, v5
	v_xor_b32_e32 v4, v4, v5
	v_add_u32_e32 v6, vcc, v3, v2
	v_subrev_u32_e32 v2, vcc, v3, v2
	v_cndmask_b32_e64 v2, v2, v6, s[0:1]
	v_mul_hi_u32 v2, v2, v4
	v_xor_b32_e32 v3, s10, v5
	s_waitcnt lgkmcnt(0)
	v_mov_b32_e32 v5, s5
	v_mul_lo_u32 v6, v2, s11
	v_add_u32_e32 v7, vcc, 1, v2
	v_add_u32_e32 v8, vcc, -1, v2
	v_sub_u32_e32 v9, vcc, v4, v6
	v_cmp_ge_u32_e32 vcc, v4, v6
	v_cmp_le_u32_e64 s[0:1], s11, v9
	s_and_b64 s[0:1], s[0:1], vcc
	v_cndmask_b32_e64 v2, v2, v7, s[0:1]
	v_cndmask_b32_e32 v2, v8, v2, vcc
	v_xor_b32_e32 v4, v2, v3
	v_sub_u32_e32 v2, vcc, v4, v3
	v_mul_lo_u32 v2, s14, v2
	v_sub_u32_e32 v3, vcc, v3, v4
	v_mul_lo_u32 v6, s13, v3
	s_add_i32 s0, s2, s3
	v_ashrrev_i32_e32 v3, 31, v2
	v_lshlrev_b64 v[3:4], 2, v[2:3]
	v_mov_b32_e32 v2, 0
	v_add_u32_e32 v3, vcc, s4, v3
	v_addc_u32_e32 v4, vcc, v5, v4, vcc
	v_add_u32_e32 v0, vcc, v0, v6
	v_add_u32_e32 v5, vcc, s0, v0
	s_mov_b32 s0, s14
BB0_3:                                  ; %.lr.ph
                                        ; =>This Inner Loop Header: Depth=1
	v_ashrrev_i32_e32 v6, 31, v5
	flat_load_dword v0, v[3:4]
	v_add_u32_e32 v3, vcc, 4, v3
	v_lshlrev_b64 v[6:7], 2, v[5:6]
	v_addc_u32_e32 v4, vcc, 0, v4, vcc
	v_add_u32_e32 v5, vcc, s13, v5
	v_mov_b32_e32 v8, s7
	v_add_u32_e32 v6, vcc, s6, v6
	v_addc_u32_e32 v7, vcc, v8, v7, vcc
	flat_load_dword v6, v[6:7]
	s_add_i32 s0, s0, -1
	s_cmp_eq_u32 s0, 0
	s_waitcnt vmcnt(0) lgkmcnt(0)
	v_mac_f32_e32 v2, v0, v6
	s_cbranch_scc0 BB0_3
BB0_4:                                  ; %Flow22
	v_mov_b32_e32 v0, 0
	v_ashrrev_i64 v[0:1], 30, v[0:1]
	s_waitcnt lgkmcnt(0)
	v_mov_b32_e32 v3, s9
	v_add_u32_e32 v0, vcc, s8, v0
	v_addc_u32_e32 v1, vcc, v3, v1, vcc
	flat_store_dword v[0:1], v2
BB0_5:
	s_endpgm
.Lfunc_end0:
	.size	pf_q_proj, .Lfunc_end0-pf_q_proj
                                        ; -- End function
	.section	.AMDGPU.csdata
; Kernel info:
; codeLenInByte = 448
; NumSgprs: 18
; NumVgprs: 10
; ScratchSize: 0
; MemoryBound: 0
; FloatMode: 192
; IeeeMode: 1
; LDSByteSize: 0 bytes/workgroup (compile time only)
; SGPRBlocks: 2
; VGPRBlocks: 2
; NumSGPRsForWavesPerEU: 18
; NumVGPRsForWavesPerEU: 10
; Occupancy: 10
; WaveLimiterHint : 1
; COMPUTE_PGM_RSRC2:USER_SGPR: 8
; COMPUTE_PGM_RSRC2:TRAP_HANDLER: 0
; COMPUTE_PGM_RSRC2:TGID_X_EN: 1
; COMPUTE_PGM_RSRC2:TGID_Y_EN: 0
; COMPUTE_PGM_RSRC2:TGID_Z_EN: 0
; COMPUTE_PGM_RSRC2:TIDIG_COMP_CNT: 0
	.text
	.protected	pf_k_proj       ; -- Begin function pf_k_proj
	.globl	pf_k_proj
	.p2align	8
	.type	pf_k_proj,@function
	.amdgpu_hsa_kernel pf_k_proj
pf_k_proj:                              ; @pf_k_proj
pf_k_proj$local:
	.amd_kernel_code_t
		amd_code_version_major = 1
		amd_code_version_minor = 2
		amd_machine_kind = 1
		amd_machine_version_major = 8
		amd_machine_version_minor = 0
		amd_machine_version_stepping = 3
		kernel_code_entry_byte_offset = 256
		kernel_code_prefetch_byte_size = 0
		granulated_workitem_vgpr_count = 2
		granulated_wavefront_sgpr_count = 2
		priority = 0
		float_mode = 192
		priv = 0
		enable_dx10_clamp = 1
		debug_mode = 0
		enable_ieee_mode = 1
		enable_wgp_mode = 0
		enable_mem_ordered = 0
		enable_fwd_progress = 0
		enable_sgpr_private_segment_wave_byte_offset = 0
		user_sgpr_count = 8
		enable_trap_handler = 0
		enable_sgpr_workgroup_id_x = 1
		enable_sgpr_workgroup_id_y = 0
		enable_sgpr_workgroup_id_z = 0
		enable_sgpr_workgroup_info = 0
		enable_vgpr_workitem_id = 0
		enable_exception_msb = 0
		granulated_lds_size = 0
		enable_exception = 0
		enable_sgpr_private_segment_buffer = 1
		enable_sgpr_dispatch_ptr = 1
		enable_sgpr_queue_ptr = 0
		enable_sgpr_kernarg_segment_ptr = 1
		enable_sgpr_dispatch_id = 0
		enable_sgpr_flat_scratch_init = 0
		enable_sgpr_private_segment_size = 0
		enable_sgpr_grid_workgroup_count_x = 0
		enable_sgpr_grid_workgroup_count_y = 0
		enable_sgpr_grid_workgroup_count_z = 0
		enable_wavefront_size32 = 0
		enable_ordered_append_gds = 0
		private_element_size = 1
		is_ptr64 = 1
		is_dynamic_callstack = 0
		is_debug_enabled = 0
		is_xnack_enabled = 0
		workitem_private_segment_byte_size = 0
		workgroup_group_segment_byte_size = 0
		gds_segment_byte_size = 0
		kernarg_segment_byte_size = 96
		workgroup_fbarrier_count = 0
		wavefront_sgpr_count = 18
		workitem_vgpr_count = 10
		reserved_vgpr_first = 0
		reserved_vgpr_count = 0
		reserved_sgpr_first = 0
		reserved_sgpr_count = 0
		debug_wavefront_private_segment_offset_sgpr = 0
		debug_private_segment_buffer_sgpr = 0
		kernarg_segment_alignment = 4
		group_segment_alignment = 4
		private_segment_alignment = 4
		wavefront_size = 6
		call_convention = -1
		runtime_loader_kernel_symbol = 0
	.end_amd_kernel_code_t
; %bb.0:
	s_load_dwordx4 s[12:15], s[6:7], 0x18
	s_load_dwordx2 s[2:3], s[6:7], 0x28
	s_load_dword s0, s[4:5], 0x4
	s_waitcnt lgkmcnt(0)
	s_and_b32 s0, s0, 0xffff
	s_mul_i32 s3, s8, s0
	v_add_u32_e32 v1, vcc, s3, v0
	v_add_u32_e32 v1, vcc, s2, v1
	s_mul_i32 s0, s13, s12
	v_cmp_gt_i32_e32 vcc, s0, v1
	s_and_saveexec_b64 s[0:1], vcc
	s_cbranch_execz BB1_5
; %bb.1:
	s_load_dwordx4 s[8:11], s[6:7], 0x10
	s_cmp_lt_i32 s14, 1
	v_mov_b32_e32 v2, 0
	s_cbranch_scc1 BB1_4
; %bb.2:                                ; %.lr.ph.preheader
	s_waitcnt lgkmcnt(0)
	s_ashr_i32 s10, s13, 31
	s_add_i32 s0, s13, s10
	s_xor_b32 s11, s0, s10
	v_cvt_f32_u32_e32 v2, s11
	v_ashrrev_i32_e32 v5, 31, v1
	s_load_dwordx4 s[4:7], s[6:7], 0x0
	v_rcp_iflag_f32_e32 v2, v2
	v_mul_f32_e32 v2, 0x4f800000, v2
	v_cvt_u32_f32_e32 v2, v2
	v_mul_lo_u32 v3, v2, s11
	v_mul_hi_u32 v4, v2, s11
	v_sub_u32_e32 v6, vcc, 0, v3
	v_cmp_eq_u32_e64 s[0:1], 0, v4
	v_cndmask_b32_e64 v3, v3, v6, s[0:1]
	v_mul_hi_u32 v3, v3, v2
	v_add_u32_e32 v4, vcc, v1, v5
	v_xor_b32_e32 v4, v4, v5
	v_add_u32_e32 v6, vcc, v3, v2
	v_subrev_u32_e32 v2, vcc, v3, v2
	v_cndmask_b32_e64 v2, v2, v6, s[0:1]
	v_mul_hi_u32 v2, v2, v4
	v_xor_b32_e32 v3, s10, v5
	s_waitcnt lgkmcnt(0)
	v_mov_b32_e32 v5, s5
	v_mul_lo_u32 v6, v2, s11
	v_add_u32_e32 v7, vcc, 1, v2
	v_add_u32_e32 v8, vcc, -1, v2
	v_sub_u32_e32 v9, vcc, v4, v6
	v_cmp_ge_u32_e32 vcc, v4, v6
	v_cmp_le_u32_e64 s[0:1], s11, v9
	s_and_b64 s[0:1], s[0:1], vcc
	v_cndmask_b32_e64 v2, v2, v7, s[0:1]
	v_cndmask_b32_e32 v2, v8, v2, vcc
	v_xor_b32_e32 v4, v2, v3
	v_sub_u32_e32 v2, vcc, v4, v3
	v_mul_lo_u32 v2, s14, v2
	v_sub_u32_e32 v3, vcc, v3, v4
	v_mul_lo_u32 v6, s13, v3
	s_add_i32 s0, s2, s3
	v_ashrrev_i32_e32 v3, 31, v2
	v_lshlrev_b64 v[3:4], 2, v[2:3]
	v_mov_b32_e32 v2, 0
	v_add_u32_e32 v3, vcc, s4, v3
	v_addc_u32_e32 v4, vcc, v5, v4, vcc
	v_add_u32_e32 v0, vcc, v0, v6
	v_add_u32_e32 v5, vcc, s0, v0
	s_mov_b32 s0, s14
BB1_3:                                  ; %.lr.ph
                                        ; =>This Inner Loop Header: Depth=1
	v_ashrrev_i32_e32 v6, 31, v5
	flat_load_dword v0, v[3:4]
	v_add_u32_e32 v3, vcc, 4, v3
	v_lshlrev_b64 v[6:7], 2, v[5:6]
	v_addc_u32_e32 v4, vcc, 0, v4, vcc
	v_add_u32_e32 v5, vcc, s13, v5
	v_mov_b32_e32 v8, s7
	v_add_u32_e32 v6, vcc, s6, v6
	v_addc_u32_e32 v7, vcc, v8, v7, vcc
	flat_load_dword v6, v[6:7]
	s_add_i32 s0, s0, -1
	s_cmp_eq_u32 s0, 0
	s_waitcnt vmcnt(0) lgkmcnt(0)
	v_mac_f32_e32 v2, v0, v6
	s_cbranch_scc0 BB1_3
BB1_4:                                  ; %Flow22
	v_mov_b32_e32 v0, 0
	v_ashrrev_i64 v[0:1], 30, v[0:1]
	s_waitcnt lgkmcnt(0)
	v_mov_b32_e32 v3, s9
	v_add_u32_e32 v0, vcc, s8, v0
	v_addc_u32_e32 v1, vcc, v3, v1, vcc
	flat_store_dword v[0:1], v2
BB1_5:
	s_endpgm
.Lfunc_end1:
	.size	pf_k_proj, .Lfunc_end1-pf_k_proj
                                        ; -- End function
	.section	.AMDGPU.csdata
; Kernel info:
; codeLenInByte = 448
; NumSgprs: 18
; NumVgprs: 10
; ScratchSize: 0
; MemoryBound: 0
; FloatMode: 192
; IeeeMode: 1
; LDSByteSize: 0 bytes/workgroup (compile time only)
; SGPRBlocks: 2
; VGPRBlocks: 2
; NumSGPRsForWavesPerEU: 18
; NumVGPRsForWavesPerEU: 10
; Occupancy: 10
; WaveLimiterHint : 1
; COMPUTE_PGM_RSRC2:USER_SGPR: 8
; COMPUTE_PGM_RSRC2:TRAP_HANDLER: 0
; COMPUTE_PGM_RSRC2:TGID_X_EN: 1
; COMPUTE_PGM_RSRC2:TGID_Y_EN: 0
; COMPUTE_PGM_RSRC2:TGID_Z_EN: 0
; COMPUTE_PGM_RSRC2:TIDIG_COMP_CNT: 0
	.text
	.protected	pf_v_proj       ; -- Begin function pf_v_proj
	.globl	pf_v_proj
	.p2align	8
	.type	pf_v_proj,@function
	.amdgpu_hsa_kernel pf_v_proj
pf_v_proj:                              ; @pf_v_proj
pf_v_proj$local:
	.amd_kernel_code_t
		amd_code_version_major = 1
		amd_code_version_minor = 2
		amd_machine_kind = 1
		amd_machine_version_major = 8
		amd_machine_version_minor = 0
		amd_machine_version_stepping = 3
		kernel_code_entry_byte_offset = 256
		kernel_code_prefetch_byte_size = 0
		granulated_workitem_vgpr_count = 2
		granulated_wavefront_sgpr_count = 2
		priority = 0
		float_mode = 192
		priv = 0
		enable_dx10_clamp = 1
		debug_mode = 0
		enable_ieee_mode = 1
		enable_wgp_mode = 0
		enable_mem_ordered = 0
		enable_fwd_progress = 0
		enable_sgpr_private_segment_wave_byte_offset = 0
		user_sgpr_count = 8
		enable_trap_handler = 0
		enable_sgpr_workgroup_id_x = 1
		enable_sgpr_workgroup_id_y = 0
		enable_sgpr_workgroup_id_z = 0
		enable_sgpr_workgroup_info = 0
		enable_vgpr_workitem_id = 0
		enable_exception_msb = 0
		granulated_lds_size = 0
		enable_exception = 0
		enable_sgpr_private_segment_buffer = 1
		enable_sgpr_dispatch_ptr = 1
		enable_sgpr_queue_ptr = 0
		enable_sgpr_kernarg_segment_ptr = 1
		enable_sgpr_dispatch_id = 0
		enable_sgpr_flat_scratch_init = 0
		enable_sgpr_private_segment_size = 0
		enable_sgpr_grid_workgroup_count_x = 0
		enable_sgpr_grid_workgroup_count_y = 0
		enable_sgpr_grid_workgroup_count_z = 0
		enable_wavefront_size32 = 0
		enable_ordered_append_gds = 0
		private_element_size = 1
		is_ptr64 = 1
		is_dynamic_callstack = 0
		is_debug_enabled = 0
		is_xnack_enabled = 0
		workitem_private_segment_byte_size = 0
		workgroup_group_segment_byte_size = 0
		gds_segment_byte_size = 0
		kernarg_segment_byte_size = 96
		workgroup_fbarrier_count = 0
		wavefront_sgpr_count = 18
		workitem_vgpr_count = 10
		reserved_vgpr_first = 0
		reserved_vgpr_count = 0
		reserved_sgpr_first = 0
		reserved_sgpr_count = 0
		debug_wavefront_private_segment_offset_sgpr = 0
		debug_private_segment_buffer_sgpr = 0
		kernarg_segment_alignment = 4
		group_segment_alignment = 4
		private_segment_alignment = 4
		wavefront_size = 6
		call_convention = -1
		runtime_loader_kernel_symbol = 0
	.end_amd_kernel_code_t
; %bb.0:
	s_load_dwordx4 s[12:15], s[6:7], 0x18
	s_load_dwordx2 s[2:3], s[6:7], 0x28
	s_load_dword s0, s[4:5], 0x4
	s_waitcnt lgkmcnt(0)
	s_and_b32 s0, s0, 0xffff
	s_mul_i32 s3, s8, s0
	v_add_u32_e32 v1, vcc, s3, v0
	v_add_u32_e32 v1, vcc, s2, v1
	s_mul_i32 s0, s13, s12
	v_cmp_gt_i32_e32 vcc, s0, v1
	s_and_saveexec_b64 s[0:1], vcc
	s_cbranch_execz BB2_5
; %bb.1:
	s_load_dwordx4 s[8:11], s[6:7], 0x10
	s_cmp_lt_i32 s14, 1
	v_mov_b32_e32 v2, 0
	s_cbranch_scc1 BB2_4
; %bb.2:                                ; %.lr.ph.preheader
	s_waitcnt lgkmcnt(0)
	s_ashr_i32 s10, s13, 31
	s_add_i32 s0, s13, s10
	s_xor_b32 s11, s0, s10
	v_cvt_f32_u32_e32 v2, s11
	v_ashrrev_i32_e32 v5, 31, v1
	s_load_dwordx4 s[4:7], s[6:7], 0x0
	v_rcp_iflag_f32_e32 v2, v2
	v_mul_f32_e32 v2, 0x4f800000, v2
	v_cvt_u32_f32_e32 v2, v2
	v_mul_lo_u32 v3, v2, s11
	v_mul_hi_u32 v4, v2, s11
	v_sub_u32_e32 v6, vcc, 0, v3
	v_cmp_eq_u32_e64 s[0:1], 0, v4
	v_cndmask_b32_e64 v3, v3, v6, s[0:1]
	v_mul_hi_u32 v3, v3, v2
	v_add_u32_e32 v4, vcc, v1, v5
	v_xor_b32_e32 v4, v4, v5
	v_add_u32_e32 v6, vcc, v3, v2
	v_subrev_u32_e32 v2, vcc, v3, v2
	v_cndmask_b32_e64 v2, v2, v6, s[0:1]
	v_mul_hi_u32 v2, v2, v4
	v_xor_b32_e32 v3, s10, v5
	s_waitcnt lgkmcnt(0)
	v_mov_b32_e32 v5, s5
	v_mul_lo_u32 v6, v2, s11
	v_add_u32_e32 v7, vcc, 1, v2
	v_add_u32_e32 v8, vcc, -1, v2
	v_sub_u32_e32 v9, vcc, v4, v6
	v_cmp_ge_u32_e32 vcc, v4, v6
	v_cmp_le_u32_e64 s[0:1], s11, v9
	s_and_b64 s[0:1], s[0:1], vcc
	v_cndmask_b32_e64 v2, v2, v7, s[0:1]
	v_cndmask_b32_e32 v2, v8, v2, vcc
	v_xor_b32_e32 v4, v2, v3
	v_sub_u32_e32 v2, vcc, v4, v3
	v_mul_lo_u32 v2, s14, v2
	v_sub_u32_e32 v3, vcc, v3, v4
	v_mul_lo_u32 v6, s13, v3
	s_add_i32 s0, s2, s3
	v_ashrrev_i32_e32 v3, 31, v2
	v_lshlrev_b64 v[3:4], 2, v[2:3]
	v_mov_b32_e32 v2, 0
	v_add_u32_e32 v3, vcc, s4, v3
	v_addc_u32_e32 v4, vcc, v5, v4, vcc
	v_add_u32_e32 v0, vcc, v0, v6
	v_add_u32_e32 v5, vcc, s0, v0
	s_mov_b32 s0, s14
BB2_3:                                  ; %.lr.ph
                                        ; =>This Inner Loop Header: Depth=1
	v_ashrrev_i32_e32 v6, 31, v5
	flat_load_dword v0, v[3:4]
	v_add_u32_e32 v3, vcc, 4, v3
	v_lshlrev_b64 v[6:7], 2, v[5:6]
	v_addc_u32_e32 v4, vcc, 0, v4, vcc
	v_add_u32_e32 v5, vcc, s13, v5
	v_mov_b32_e32 v8, s7
	v_add_u32_e32 v6, vcc, s6, v6
	v_addc_u32_e32 v7, vcc, v8, v7, vcc
	flat_load_dword v6, v[6:7]
	s_add_i32 s0, s0, -1
	s_cmp_eq_u32 s0, 0
	s_waitcnt vmcnt(0) lgkmcnt(0)
	v_mac_f32_e32 v2, v0, v6
	s_cbranch_scc0 BB2_3
BB2_4:                                  ; %Flow22
	v_mov_b32_e32 v0, 0
	v_ashrrev_i64 v[0:1], 30, v[0:1]
	s_waitcnt lgkmcnt(0)
	v_mov_b32_e32 v3, s9
	v_add_u32_e32 v0, vcc, s8, v0
	v_addc_u32_e32 v1, vcc, v3, v1, vcc
	flat_store_dword v[0:1], v2
BB2_5:
	s_endpgm
.Lfunc_end2:
	.size	pf_v_proj, .Lfunc_end2-pf_v_proj
                                        ; -- End function
	.section	.AMDGPU.csdata
; Kernel info:
; codeLenInByte = 448
; NumSgprs: 18
; NumVgprs: 10
; ScratchSize: 0
; MemoryBound: 0
; FloatMode: 192
; IeeeMode: 1
; LDSByteSize: 0 bytes/workgroup (compile time only)
; SGPRBlocks: 2
; VGPRBlocks: 2
; NumSGPRsForWavesPerEU: 18
; NumVGPRsForWavesPerEU: 10
; Occupancy: 10
; WaveLimiterHint : 1
; COMPUTE_PGM_RSRC2:USER_SGPR: 8
; COMPUTE_PGM_RSRC2:TRAP_HANDLER: 0
; COMPUTE_PGM_RSRC2:TGID_X_EN: 1
; COMPUTE_PGM_RSRC2:TGID_Y_EN: 0
; COMPUTE_PGM_RSRC2:TGID_Z_EN: 0
; COMPUTE_PGM_RSRC2:TIDIG_COMP_CNT: 0
	.text
	.protected	pf_phi_q        ; -- Begin function pf_phi_q
	.globl	pf_phi_q
	.p2align	8
	.type	pf_phi_q,@function
	.amdgpu_hsa_kernel pf_phi_q
pf_phi_q:                               ; @pf_phi_q
pf_phi_q$local:
	.amd_kernel_code_t
		amd_code_version_major = 1
		amd_code_version_minor = 2
		amd_machine_kind = 1
		amd_machine_version_major = 8
		amd_machine_version_minor = 0
		amd_machine_version_stepping = 3
		kernel_code_entry_byte_offset = 256
		kernel_code_prefetch_byte_size = 0
		granulated_workitem_vgpr_count = 1
		granulated_wavefront_sgpr_count = 1
		priority = 0
		float_mode = 192
		priv = 0
		enable_dx10_clamp = 1
		debug_mode = 0
		enable_ieee_mode = 1
		enable_wgp_mode = 0
		enable_mem_ordered = 0
		enable_fwd_progress = 0
		enable_sgpr_private_segment_wave_byte_offset = 0
		user_sgpr_count = 8
		enable_trap_handler = 0
		enable_sgpr_workgroup_id_x = 1
		enable_sgpr_workgroup_id_y = 0
		enable_sgpr_workgroup_id_z = 0
		enable_sgpr_workgroup_info = 0
		enable_vgpr_workitem_id = 0
		enable_exception_msb = 0
		granulated_lds_size = 0
		enable_exception = 0
		enable_sgpr_private_segment_buffer = 1
		enable_sgpr_dispatch_ptr = 1
		enable_sgpr_queue_ptr = 0
		enable_sgpr_kernarg_segment_ptr = 1
		enable_sgpr_dispatch_id = 0
		enable_sgpr_flat_scratch_init = 0
		enable_sgpr_private_segment_size = 0
		enable_sgpr_grid_workgroup_count_x = 0
		enable_sgpr_grid_workgroup_count_y = 0
		enable_sgpr_grid_workgroup_count_z = 0
		enable_wavefront_size32 = 0
		enable_ordered_append_gds = 0
		private_element_size = 1
		is_ptr64 = 1
		is_dynamic_callstack = 0
		is_debug_enabled = 0
		is_xnack_enabled = 0
		workitem_private_segment_byte_size = 0
		workgroup_group_segment_byte_size = 0
		gds_segment_byte_size = 0
		kernarg_segment_byte_size = 80
		workgroup_fbarrier_count = 0
		wavefront_sgpr_count = 11
		workitem_vgpr_count = 6
		reserved_vgpr_first = 0
		reserved_vgpr_count = 0
		reserved_sgpr_first = 0
		reserved_sgpr_count = 0
		debug_wavefront_private_segment_offset_sgpr = 0
		debug_private_segment_buffer_sgpr = 0
		kernarg_segment_alignment = 4
		group_segment_alignment = 4
		private_segment_alignment = 4
		wavefront_size = 6
		call_convention = -1
		runtime_loader_kernel_symbol = 0
	.end_amd_kernel_code_t
; %bb.0:
	s_load_dword s0, s[4:5], 0x4
	s_waitcnt lgkmcnt(0)
	s_and_b32 s0, s0, 0xffff
	s_mul_i32 s8, s8, s0
	s_load_dword s2, s[6:7], 0x10
	s_load_dwordx2 s[0:1], s[6:7], 0x18
	v_add_u32_e32 v0, vcc, s8, v0
	s_waitcnt lgkmcnt(0)
	v_add_u32_e32 v0, vcc, s0, v0
	v_cmp_gt_i32_e32 vcc, s2, v0
	s_and_saveexec_b64 s[0:1], vcc
	s_cbranch_execz BB3_4
; %bb.1:
	s_load_dwordx4 s[0:3], s[6:7], 0x0
	v_mov_b32_e32 v2, 0
	v_mov_b32_e32 v3, v0
	v_ashrrev_i64 v[2:3], 30, v[2:3]
	v_ashrrev_i32_e32 v1, 31, v0
	s_waitcnt lgkmcnt(0)
	v_mov_b32_e32 v4, s1
	v_add_u32_e32 v2, vcc, s0, v2
	v_addc_u32_e32 v3, vcc, v4, v3, vcc
	flat_load_dword v2, v[2:3]
	s_waitcnt vmcnt(0) lgkmcnt(0)
	v_cmp_nlt_f32_e32 vcc, 0, v2
	s_and_saveexec_b64 s[0:1], vcc
	s_cbranch_execz BB3_3
; %bb.2:
	v_and_b32_e32 v3, 0xfffff000, v2
	v_sub_f32_e32 v4, v2, v3
	s_mov_b32 s4, 0x39a3b295
	v_mul_f32_e32 v5, s4, v4
	s_mov_b32 s5, 0x3fb8a000
	v_mac_f32_e32 v5, s5, v4
	v_mac_f32_e32 v5, s4, v3
	v_mul_f32_e32 v3, s5, v3
	v_exp_f32_e32 v4, v5
	v_exp_f32_e32 v3, v3
	s_mov_b32 s4, 0xc2aeac50
	v_cmp_ngt_f32_e32 vcc, s4, v2
	s_mov_b32 s4, 0x42b17218
	v_mul_f32_e32 v3, v3, v4
	v_cndmask_b32_e32 v3, 0, v3, vcc
	v_mov_b32_e32 v4, 0x7f800000
	v_cmp_nlt_f32_e32 vcc, s4, v2
	v_cndmask_b32_e32 v2, v4, v3, vcc
BB3_3:
	s_or_b64 exec, exec, s[0:1]
	v_lshlrev_b64 v[0:1], 2, v[0:1]
	v_mov_b32_e32 v3, s3
	v_add_u32_e32 v0, vcc, s2, v0
	v_addc_u32_e32 v1, vcc, v3, v1, vcc
	flat_store_dword v[0:1], v2
BB3_4:
	s_endpgm
.Lfunc_end3:
	.size	pf_phi_q, .Lfunc_end3-pf_phi_q
                                        ; -- End function
	.section	.AMDGPU.csdata
; Kernel info:
; codeLenInByte = 264
; NumSgprs: 11
; NumVgprs: 6
; ScratchSize: 0
; MemoryBound: 0
; FloatMode: 192
; IeeeMode: 1
; LDSByteSize: 0 bytes/workgroup (compile time only)
; SGPRBlocks: 1
; VGPRBlocks: 1
; NumSGPRsForWavesPerEU: 11
; NumVGPRsForWavesPerEU: 6
; Occupancy: 10
; WaveLimiterHint : 1
; COMPUTE_PGM_RSRC2:USER_SGPR: 8
; COMPUTE_PGM_RSRC2:TRAP_HANDLER: 0
; COMPUTE_PGM_RSRC2:TGID_X_EN: 1
; COMPUTE_PGM_RSRC2:TGID_Y_EN: 0
; COMPUTE_PGM_RSRC2:TGID_Z_EN: 0
; COMPUTE_PGM_RSRC2:TIDIG_COMP_CNT: 0
	.text
	.protected	pf_phi_k        ; -- Begin function pf_phi_k
	.globl	pf_phi_k
	.p2align	8
	.type	pf_phi_k,@function
	.amdgpu_hsa_kernel pf_phi_k
pf_phi_k:                               ; @pf_phi_k
pf_phi_k$local:
	.amd_kernel_code_t
		amd_code_version_major = 1
		amd_code_version_minor = 2
		amd_machine_kind = 1
		amd_machine_version_major = 8
		amd_machine_version_minor = 0
		amd_machine_version_stepping = 3
		kernel_code_entry_byte_offset = 256
		kernel_code_prefetch_byte_size = 0
		granulated_workitem_vgpr_count = 1
		granulated_wavefront_sgpr_count = 1
		priority = 0
		float_mode = 192
		priv = 0
		enable_dx10_clamp = 1
		debug_mode = 0
		enable_ieee_mode = 1
		enable_wgp_mode = 0
		enable_mem_ordered = 0
		enable_fwd_progress = 0
		enable_sgpr_private_segment_wave_byte_offset = 0
		user_sgpr_count = 8
		enable_trap_handler = 0
		enable_sgpr_workgroup_id_x = 1
		enable_sgpr_workgroup_id_y = 0
		enable_sgpr_workgroup_id_z = 0
		enable_sgpr_workgroup_info = 0
		enable_vgpr_workitem_id = 0
		enable_exception_msb = 0
		granulated_lds_size = 0
		enable_exception = 0
		enable_sgpr_private_segment_buffer = 1
		enable_sgpr_dispatch_ptr = 1
		enable_sgpr_queue_ptr = 0
		enable_sgpr_kernarg_segment_ptr = 1
		enable_sgpr_dispatch_id = 0
		enable_sgpr_flat_scratch_init = 0
		enable_sgpr_private_segment_size = 0
		enable_sgpr_grid_workgroup_count_x = 0
		enable_sgpr_grid_workgroup_count_y = 0
		enable_sgpr_grid_workgroup_count_z = 0
		enable_wavefront_size32 = 0
		enable_ordered_append_gds = 0
		private_element_size = 1
		is_ptr64 = 1
		is_dynamic_callstack = 0
		is_debug_enabled = 0
		is_xnack_enabled = 0
		workitem_private_segment_byte_size = 0
		workgroup_group_segment_byte_size = 0
		gds_segment_byte_size = 0
		kernarg_segment_byte_size = 80
		workgroup_fbarrier_count = 0
		wavefront_sgpr_count = 11
		workitem_vgpr_count = 6
		reserved_vgpr_first = 0
		reserved_vgpr_count = 0
		reserved_sgpr_first = 0
		reserved_sgpr_count = 0
		debug_wavefront_private_segment_offset_sgpr = 0
		debug_private_segment_buffer_sgpr = 0
		kernarg_segment_alignment = 4
		group_segment_alignment = 4
		private_segment_alignment = 4
		wavefront_size = 6
		call_convention = -1
		runtime_loader_kernel_symbol = 0
	.end_amd_kernel_code_t
; %bb.0:
	s_load_dword s0, s[4:5], 0x4
	s_waitcnt lgkmcnt(0)
	s_and_b32 s0, s0, 0xffff
	s_mul_i32 s8, s8, s0
	s_load_dword s2, s[6:7], 0x10
	s_load_dwordx2 s[0:1], s[6:7], 0x18
	v_add_u32_e32 v0, vcc, s8, v0
	s_waitcnt lgkmcnt(0)
	v_add_u32_e32 v0, vcc, s0, v0
	v_cmp_gt_i32_e32 vcc, s2, v0
	s_and_saveexec_b64 s[0:1], vcc
	s_cbranch_execz BB4_4
; %bb.1:
	s_load_dwordx4 s[0:3], s[6:7], 0x0
	v_mov_b32_e32 v2, 0
	v_mov_b32_e32 v3, v0
	v_ashrrev_i64 v[2:3], 30, v[2:3]
	v_ashrrev_i32_e32 v1, 31, v0
	s_waitcnt lgkmcnt(0)
	v_mov_b32_e32 v4, s1
	v_add_u32_e32 v2, vcc, s0, v2
	v_addc_u32_e32 v3, vcc, v4, v3, vcc
	flat_load_dword v2, v[2:3]
	s_waitcnt vmcnt(0) lgkmcnt(0)
	v_cmp_nlt_f32_e32 vcc, 0, v2
	s_and_saveexec_b64 s[0:1], vcc
	s_cbranch_execz BB4_3
; %bb.2:
	v_and_b32_e32 v3, 0xfffff000, v2
	v_sub_f32_e32 v4, v2, v3
	s_mov_b32 s4, 0x39a3b295
	v_mul_f32_e32 v5, s4, v4
	s_mov_b32 s5, 0x3fb8a000
	v_mac_f32_e32 v5, s5, v4
	v_mac_f32_e32 v5, s4, v3
	v_mul_f32_e32 v3, s5, v3
	v_exp_f32_e32 v4, v5
	v_exp_f32_e32 v3, v3
	s_mov_b32 s4, 0xc2aeac50
	v_cmp_ngt_f32_e32 vcc, s4, v2
	s_mov_b32 s4, 0x42b17218
	v_mul_f32_e32 v3, v3, v4
	v_cndmask_b32_e32 v3, 0, v3, vcc
	v_mov_b32_e32 v4, 0x7f800000
	v_cmp_nlt_f32_e32 vcc, s4, v2
	v_cndmask_b32_e32 v2, v4, v3, vcc
BB4_3:
	s_or_b64 exec, exec, s[0:1]
	v_lshlrev_b64 v[0:1], 2, v[0:1]
	v_mov_b32_e32 v3, s3
	v_add_u32_e32 v0, vcc, s2, v0
	v_addc_u32_e32 v1, vcc, v3, v1, vcc
	flat_store_dword v[0:1], v2
BB4_4:
	s_endpgm
.Lfunc_end4:
	.size	pf_phi_k, .Lfunc_end4-pf_phi_k
                                        ; -- End function
	.section	.AMDGPU.csdata
; Kernel info:
; codeLenInByte = 264
; NumSgprs: 11
; NumVgprs: 6
; ScratchSize: 0
; MemoryBound: 0
; FloatMode: 192
; IeeeMode: 1
; LDSByteSize: 0 bytes/workgroup (compile time only)
; SGPRBlocks: 1
; VGPRBlocks: 1
; NumSGPRsForWavesPerEU: 11
; NumVGPRsForWavesPerEU: 6
; Occupancy: 10
; WaveLimiterHint : 1
; COMPUTE_PGM_RSRC2:USER_SGPR: 8
; COMPUTE_PGM_RSRC2:TRAP_HANDLER: 0
; COMPUTE_PGM_RSRC2:TGID_X_EN: 1
; COMPUTE_PGM_RSRC2:TGID_Y_EN: 0
; COMPUTE_PGM_RSRC2:TGID_Z_EN: 0
; COMPUTE_PGM_RSRC2:TIDIG_COMP_CNT: 0
	.text
	.protected	pf_kv_reduce    ; -- Begin function pf_kv_reduce
	.globl	pf_kv_reduce
	.p2align	8
	.type	pf_kv_reduce,@function
	.amdgpu_hsa_kernel pf_kv_reduce
pf_kv_reduce:                           ; @pf_kv_reduce
pf_kv_reduce$local:
	.amd_kernel_code_t
		amd_code_version_major = 1
		amd_code_version_minor = 2
		amd_machine_kind = 1
		amd_machine_version_major = 8
		amd_machine_version_minor = 0
		amd_machine_version_stepping = 3
		kernel_code_entry_byte_offset = 256
		kernel_code_prefetch_byte_size = 0
		granulated_workitem_vgpr_count = 2
		granulated_wavefront_sgpr_count = 3
		priority = 0
		float_mode = 192
		priv = 0
		enable_dx10_clamp = 1
		debug_mode = 0
		enable_ieee_mode = 1
		enable_wgp_mode = 0
		enable_mem_ordered = 0
		enable_fwd_progress = 0
		enable_sgpr_private_segment_wave_byte_offset = 0
		user_sgpr_count = 8
		enable_trap_handler = 0
		enable_sgpr_workgroup_id_x = 1
		enable_sgpr_workgroup_id_y = 0
		enable_sgpr_workgroup_id_z = 0
		enable_sgpr_workgroup_info = 0
		enable_vgpr_workitem_id = 0
		enable_exception_msb = 0
		granulated_lds_size = 0
		enable_exception = 0
		enable_sgpr_private_segment_buffer = 1
		enable_sgpr_dispatch_ptr = 1
		enable_sgpr_queue_ptr = 0
		enable_sgpr_kernarg_segment_ptr = 1
		enable_sgpr_dispatch_id = 0
		enable_sgpr_flat_scratch_init = 0
		enable_sgpr_private_segment_size = 0
		enable_sgpr_grid_workgroup_count_x = 0
		enable_sgpr_grid_workgroup_count_y = 0
		enable_sgpr_grid_workgroup_count_z = 0
		enable_wavefront_size32 = 0
		enable_ordered_append_gds = 0
		private_element_size = 1
		is_ptr64 = 1
		is_dynamic_callstack = 0
		is_debug_enabled = 0
		is_xnack_enabled = 0
		workitem_private_segment_byte_size = 0
		workgroup_group_segment_byte_size = 0
		gds_segment_byte_size = 0
		kernarg_segment_byte_size = 96
		workgroup_fbarrier_count = 0
		wavefront_sgpr_count = 26
		workitem_vgpr_count = 11
		reserved_vgpr_first = 0
		reserved_vgpr_count = 0
		reserved_sgpr_first = 0
		reserved_sgpr_count = 0
		debug_wavefront_private_segment_offset_sgpr = 0
		debug_private_segment_buffer_sgpr = 0
		kernarg_segment_alignment = 4
		group_segment_alignment = 4
		private_segment_alignment = 4
		wavefront_size = 6
		call_convention = -1
		runtime_loader_kernel_symbol = 0
	.end_amd_kernel_code_t
; %bb.0:
	s_load_dwordx4 s[12:15], s[6:7], 0x18
	s_load_dwordx2 s[16:17], s[6:7], 0x28
	s_load_dword s0, s[4:5], 0x4
	s_waitcnt lgkmcnt(0)
	s_mul_i32 s4, s14, s13
	s_and_b32 s0, s0, 0xffff
	s_mul_i32 s5, s8, s0
	v_add_u32_e32 v1, vcc, s5, v0
	v_add_u32_e32 v1, vcc, s16, v1
	s_mul_i32 s0, s4, s13
	v_cmp_gt_i32_e32 vcc, s0, v1
	s_and_saveexec_b64 s[0:1], vcc
	s_cbranch_execz BB5_5
; %bb.1:
	s_load_dwordx4 s[8:11], s[6:7], 0x10
	s_cmp_lt_i32 s12, 1
	v_mov_b32_e32 v2, 0
	s_cbranch_scc1 BB5_4
; %bb.2:                                ; %.lr.ph.preheader
	s_mul_i32 s2, s13, s13
	v_cvt_f32_u32_e32 v2, s2
	s_waitcnt lgkmcnt(0)
	s_mov_b32 s11, 0x4f800000
	v_ashrrev_i32_e32 v5, 31, v1
	s_ashr_i32 s3, s13, 31
	v_rcp_iflag_f32_e32 v2, v2
	s_load_dwordx4 s[20:23], s[6:7], 0x0
	s_mov_b32 s6, 0
	v_mul_f32_e32 v2, s11, v2
	v_cvt_u32_f32_e32 v2, v2
	v_mul_lo_u32 v3, v2, s2
	v_mul_hi_u32 v4, v2, s2
	v_sub_u32_e32 v6, vcc, 0, v3
	v_cmp_eq_u32_e64 s[0:1], 0, v4
	v_cndmask_b32_e64 v3, v3, v6, s[0:1]
	v_mul_hi_u32 v3, v3, v2
	v_add_u32_e32 v4, vcc, v1, v5
	v_xor_b32_e32 v4, v4, v5
	v_add_u32_e32 v6, vcc, v3, v2
	v_subrev_u32_e32 v2, vcc, v3, v2
	v_cndmask_b32_e64 v2, v2, v6, s[0:1]
	v_mul_hi_u32 v2, v2, v4
	s_add_i32 s0, s13, s3
	s_xor_b32 s10, s0, s3
	v_mul_lo_u32 v3, v2, s2
	v_add_u32_e32 v6, vcc, 1, v2
	v_add_u32_e32 v7, vcc, -1, v2
	v_sub_u32_e32 v8, vcc, v4, v3
	v_cmp_ge_u32_e32 vcc, v4, v3
	v_cvt_f32_u32_e32 v3, s10
	v_cmp_le_u32_e64 s[0:1], s2, v8
	s_and_b64 s[0:1], s[0:1], vcc
	v_cndmask_b32_e64 v2, v2, v6, s[0:1]
	v_rcp_iflag_f32_e32 v3, v3
	v_cndmask_b32_e32 v2, v7, v2, vcc
	v_xor_b32_e32 v2, v2, v5
	v_sub_u32_e32 v4, vcc, v2, v5
	v_mul_f32_e32 v3, s11, v3
	v_cvt_u32_f32_e32 v3, v3
	v_mul_lo_u32 v4, v4, s13
	v_mul_lo_u32 v7, v3, s10
	v_mul_hi_u32 v8, v3, s10
	v_mul_lo_u32 v6, v4, s13
	v_sub_u32_e32 v10, vcc, 0, v7
	v_cmp_eq_u32_e64 s[0:1], 0, v8
	v_cndmask_b32_e64 v7, v7, v10, s[0:1]
	v_mul_hi_u32 v7, v7, v3
	v_sub_u32_e32 v6, vcc, v1, v6
	v_ashrrev_i32_e32 v9, 31, v6
	v_add_u32_e32 v6, vcc, v6, v9
	v_add_u32_e32 v8, vcc, v7, v3
	v_subrev_u32_e32 v3, vcc, v7, v3
	v_cndmask_b32_e64 v3, v3, v8, s[0:1]
	v_xor_b32_e32 v6, v6, v9
	v_mul_hi_u32 v3, v3, v6
	v_xor_b32_e32 v8, s3, v9
	v_mul_lo_u32 v7, v3, s10
	v_sub_u32_e32 v9, vcc, v6, v7
	v_cmp_le_u32_e32 vcc, s10, v9
	v_add_u32_e64 v9, s[0:1], -1, v3
	v_cmp_ge_u32_e64 s[0:1], v6, v7
	v_add_u32_e64 v6, s[2:3], 1, v3
	s_and_b64 vcc, vcc, s[0:1]
	v_cndmask_b32_e32 v3, v3, v6, vcc
	v_cndmask_b32_e64 v3, v9, v3, s[0:1]
	v_add_u32_e32 v2, vcc, v2, v8
	v_xor_b32_e32 v3, v3, v8
	v_sub_u32_e32 v2, vcc, v2, v4
	v_sub_u32_e32 v2, vcc, v2, v3
	v_sub_u32_e32 v2, vcc, v2, v5
	v_mul_lo_u32 v2, s13, v2
	v_add_u32_e32 v6, vcc, v3, v4
	s_add_i32 s0, s16, s5
	v_sub_u32_e32 v3, vcc, v6, v8
	v_add_u32_e32 v0, vcc, v0, v2
	v_add_u32_e32 v0, vcc, s0, v0
	v_mov_b32_e32 v2, 0
BB5_3:                                  ; %.lr.ph
                                        ; =>This Inner Loop Header: Depth=1
	v_add_u32_e32 v4, vcc, s6, v3
	v_ashrrev_i32_e32 v5, 31, v4
	v_add_u32_e32 v6, vcc, s6, v0
	v_lshlrev_b64 v[4:5], 2, v[4:5]
	v_ashrrev_i32_e32 v7, 31, v6
	s_waitcnt lgkmcnt(0)
	v_mov_b32_e32 v8, s21
	v_add_u32_e32 v4, vcc, s20, v4
	v_lshlrev_b64 v[6:7], 2, v[6:7]
	v_addc_u32_e32 v5, vcc, v8, v5, vcc
	v_mov_b32_e32 v9, s23
	v_add_u32_e32 v6, vcc, s22, v6
	v_addc_u32_e32 v7, vcc, v9, v7, vcc
	flat_load_dword v4, v[4:5]
	flat_load_dword v5, v[6:7]
	s_add_i32 s6, s6, s4
	s_add_i32 s12, s12, -1
	s_cmp_eq_u32 s12, 0
	s_waitcnt vmcnt(0) lgkmcnt(0)
	v_mac_f32_e32 v2, v4, v5
	s_cbranch_scc0 BB5_3
BB5_4:                                  ; %Flow20
	v_mov_b32_e32 v0, 0
	v_ashrrev_i64 v[0:1], 30, v[0:1]
	s_waitcnt lgkmcnt(0)
	v_mov_b32_e32 v3, s9
	v_add_u32_e32 v0, vcc, s8, v0
	v_addc_u32_e32 v1, vcc, v3, v1, vcc
	flat_store_dword v[0:1], v2
BB5_5:
	s_endpgm
.Lfunc_end5:
	.size	pf_kv_reduce, .Lfunc_end5-pf_kv_reduce
                                        ; -- End function
	.section	.AMDGPU.csdata
; Kernel info:
; codeLenInByte = 648
; NumSgprs: 26
; NumVgprs: 11
; ScratchSize: 0
; MemoryBound: 0
; FloatMode: 192
; IeeeMode: 1
; LDSByteSize: 0 bytes/workgroup (compile time only)
; SGPRBlocks: 3
; VGPRBlocks: 2
; NumSGPRsForWavesPerEU: 26
; NumVGPRsForWavesPerEU: 11
; Occupancy: 10
; WaveLimiterHint : 1
; COMPUTE_PGM_RSRC2:USER_SGPR: 8
; COMPUTE_PGM_RSRC2:TRAP_HANDLER: 0
; COMPUTE_PGM_RSRC2:TGID_X_EN: 1
; COMPUTE_PGM_RSRC2:TGID_Y_EN: 0
; COMPUTE_PGM_RSRC2:TGID_Z_EN: 0
; COMPUTE_PGM_RSRC2:TIDIG_COMP_CNT: 0
	.text
	.protected	pf_z_reduce     ; -- Begin function pf_z_reduce
	.globl	pf_z_reduce
	.p2align	8
	.type	pf_z_reduce,@function
	.amdgpu_hsa_kernel pf_z_reduce
pf_z_reduce:                            ; @pf_z_reduce
pf_z_reduce$local:
	.amd_kernel_code_t
		amd_code_version_major = 1
		amd_code_version_minor = 2
		amd_machine_kind = 1
		amd_machine_version_major = 8
		amd_machine_version_minor = 0
		amd_machine_version_stepping = 3
		kernel_code_entry_byte_offset = 256
		kernel_code_prefetch_byte_size = 0
		granulated_workitem_vgpr_count = 1
		granulated_wavefront_sgpr_count = 2
		priority = 0
		float_mode = 192
		priv = 0
		enable_dx10_clamp = 1
		debug_mode = 0
		enable_ieee_mode = 1
		enable_wgp_mode = 0
		enable_mem_ordered = 0
		enable_fwd_progress = 0
		enable_sgpr_private_segment_wave_byte_offset = 0
		user_sgpr_count = 8
		enable_trap_handler = 0
		enable_sgpr_workgroup_id_x = 1
		enable_sgpr_workgroup_id_y = 0
		enable_sgpr_workgroup_id_z = 0
		enable_sgpr_workgroup_info = 0
		enable_vgpr_workitem_id = 0
		enable_exception_msb = 0
		granulated_lds_size = 0
		enable_exception = 0
		enable_sgpr_private_segment_buffer = 1
		enable_sgpr_dispatch_ptr = 1
		enable_sgpr_queue_ptr = 0
		enable_sgpr_kernarg_segment_ptr = 1
		enable_sgpr_dispatch_id = 0
		enable_sgpr_flat_scratch_init = 0
		enable_sgpr_private_segment_size = 0
		enable_sgpr_grid_workgroup_count_x = 0
		enable_sgpr_grid_workgroup_count_y = 0
		enable_sgpr_grid_workgroup_count_z = 0
		enable_wavefront_size32 = 0
		enable_ordered_append_gds = 0
		private_element_size = 1
		is_ptr64 = 1
		is_dynamic_callstack = 0
		is_debug_enabled = 0
		is_xnack_enabled = 0
		workitem_private_segment_byte_size = 0
		workgroup_group_segment_byte_size = 0
		gds_segment_byte_size = 0
		kernarg_segment_byte_size = 88
		workgroup_fbarrier_count = 0
		wavefront_sgpr_count = 18
		workitem_vgpr_count = 6
		reserved_vgpr_first = 0
		reserved_vgpr_count = 0
		reserved_sgpr_first = 0
		reserved_sgpr_count = 0
		debug_wavefront_private_segment_offset_sgpr = 0
		debug_private_segment_buffer_sgpr = 0
		kernarg_segment_alignment = 4
		group_segment_alignment = 4
		private_segment_alignment = 4
		wavefront_size = 6
		call_convention = -1
		runtime_loader_kernel_symbol = 0
	.end_amd_kernel_code_t
; %bb.0:
	s_load_dwordx4 s[12:15], s[6:7], 0x10
	s_load_dwordx2 s[0:1], s[6:7], 0x20
	s_waitcnt lgkmcnt(0)
	s_load_dword s1, s[4:5], 0x4
	s_mul_i32 s4, s14, s13
	s_waitcnt lgkmcnt(0)
	s_and_b32 s1, s1, 0xffff
	s_mul_i32 s8, s8, s1
	v_add_u32_e32 v0, vcc, s8, v0
	v_add_u32_e32 v0, vcc, s0, v0
	v_mov_b32_e32 v2, v0
	v_cmp_gt_i32_e32 vcc, s4, v0
	s_and_saveexec_b64 s[0:1], vcc
	s_cbranch_execz BB6_5
; %bb.1:                                ; %.preheader
	s_load_dwordx4 s[0:3], s[6:7], 0x0
	s_cmp_lt_i32 s12, 1
	v_mov_b32_e32 v1, 0
	s_cbranch_scc1 BB6_4
; %bb.2:                                ; %.lr.ph.preheader
	v_mov_b32_e32 v1, 0
BB6_3:                                  ; %.lr.ph
                                        ; =>This Inner Loop Header: Depth=1
	v_ashrrev_i32_e32 v3, 31, v2
	v_lshlrev_b64 v[3:4], 2, v[2:3]
	s_waitcnt lgkmcnt(0)
	v_mov_b32_e32 v5, s1
	v_add_u32_e32 v3, vcc, s0, v3
	v_addc_u32_e32 v4, vcc, v5, v4, vcc
	flat_load_dword v3, v[3:4]
	s_add_i32 s12, s12, -1
	s_cmp_eq_u32 s12, 0
	v_add_u32_e32 v2, vcc, s4, v2
	s_waitcnt vmcnt(0) lgkmcnt(0)
	v_add_f32_e32 v1, v1, v3
	s_cbranch_scc0 BB6_3
BB6_4:                                  ; %Flow18
	v_mov_b32_e32 v3, v0
	v_mov_b32_e32 v2, 0
	v_ashrrev_i64 v[2:3], 30, v[2:3]
	s_waitcnt lgkmcnt(0)
	v_mov_b32_e32 v0, s3
	v_add_u32_e32 v2, vcc, s2, v2
	v_addc_u32_e32 v3, vcc, v0, v3, vcc
	flat_store_dword v[2:3], v1
BB6_5:
	s_endpgm
.Lfunc_end6:
	.size	pf_z_reduce, .Lfunc_end6-pf_z_reduce
                                        ; -- End function
	.section	.AMDGPU.csdata
; Kernel info:
; codeLenInByte = 200
; NumSgprs: 18
; NumVgprs: 6
; ScratchSize: 0
; MemoryBound: 0
; FloatMode: 192
; IeeeMode: 1
; LDSByteSize: 0 bytes/workgroup (compile time only)
; SGPRBlocks: 2
; VGPRBlocks: 1
; NumSGPRsForWavesPerEU: 18
; NumVGPRsForWavesPerEU: 6
; Occupancy: 10
; WaveLimiterHint : 1
; COMPUTE_PGM_RSRC2:USER_SGPR: 8
; COMPUTE_PGM_RSRC2:TRAP_HANDLER: 0
; COMPUTE_PGM_RSRC2:TGID_X_EN: 1
; COMPUTE_PGM_RSRC2:TGID_Y_EN: 0
; COMPUTE_PGM_RSRC2:TGID_Z_EN: 0
; COMPUTE_PGM_RSRC2:TIDIG_COMP_CNT: 0
	.text
	.protected	pf_linear_context ; -- Begin function pf_linear_context
	.globl	pf_linear_context
	.p2align	8
	.type	pf_linear_context,@function
	.amdgpu_hsa_kernel pf_linear_context
pf_linear_context:                      ; @pf_linear_context
pf_linear_context$local:
	.amd_kernel_code_t
		amd_code_version_major = 1
		amd_code_version_minor = 2
		amd_machine_kind = 1
		amd_machine_version_major = 8
		amd_machine_version_minor = 0
		amd_machine_version_stepping = 3
		kernel_code_entry_byte_offset = 256
		kernel_code_prefetch_byte_size = 0
		granulated_workitem_vgpr_count = 2
		granulated_wavefront_sgpr_count = 2
		priority = 0
		float_mode = 192
		priv = 0
		enable_dx10_clamp = 1
		debug_mode = 0
		enable_ieee_mode = 1
		enable_wgp_mode = 0
		enable_mem_ordered = 0
		enable_fwd_progress = 0
		enable_sgpr_private_segment_wave_byte_offset = 0
		user_sgpr_count = 8
		enable_trap_handler = 0
		enable_sgpr_workgroup_id_x = 1
		enable_sgpr_workgroup_id_y = 0
		enable_sgpr_workgroup_id_z = 0
		enable_sgpr_workgroup_info = 0
		enable_vgpr_workitem_id = 0
		enable_exception_msb = 0
		granulated_lds_size = 0
		enable_exception = 0
		enable_sgpr_private_segment_buffer = 1
		enable_sgpr_dispatch_ptr = 1
		enable_sgpr_queue_ptr = 0
		enable_sgpr_kernarg_segment_ptr = 1
		enable_sgpr_dispatch_id = 0
		enable_sgpr_flat_scratch_init = 0
		enable_sgpr_private_segment_size = 0
		enable_sgpr_grid_workgroup_count_x = 0
		enable_sgpr_grid_workgroup_count_y = 0
		enable_sgpr_grid_workgroup_count_z = 0
		enable_wavefront_size32 = 0
		enable_ordered_append_gds = 0
		private_element_size = 1
		is_ptr64 = 1
		is_dynamic_callstack = 0
		is_debug_enabled = 0
		is_xnack_enabled = 0
		workitem_private_segment_byte_size = 0
		workgroup_group_segment_byte_size = 0
		gds_segment_byte_size = 0
		kernarg_segment_byte_size = 104
		workgroup_fbarrier_count = 0
		wavefront_sgpr_count = 20
		workitem_vgpr_count = 11
		reserved_vgpr_first = 0
		reserved_vgpr_count = 0
		reserved_sgpr_first = 0
		reserved_sgpr_count = 0
		debug_wavefront_private_segment_offset_sgpr = 0
		debug_private_segment_buffer_sgpr = 0
		kernarg_segment_alignment = 4
		group_segment_alignment = 4
		private_segment_alignment = 4
		wavefront_size = 6
		call_convention = -1
		runtime_loader_kernel_symbol = 0
	.end_amd_kernel_code_t
; %bb.0:
	s_load_dwordx4 s[12:15], s[6:7], 0x20
	s_load_dwordx2 s[16:17], s[6:7], 0x30
	s_load_dword s0, s[4:5], 0x4
	s_waitcnt lgkmcnt(0)
	s_and_b32 s0, s0, 0xffff
	s_mul_i32 s15, s8, s0
	v_add_u32_e32 v1, vcc, s15, v0
	s_mul_i32 s0, s14, s13
	v_add_u32_e32 v1, vcc, s16, v1
	s_mul_i32 s1, s0, s12
	v_cmp_gt_i32_e32 vcc, s1, v1
	s_and_saveexec_b64 s[2:3], vcc
	s_cbranch_execz BB7_5
; %bb.1:
	s_load_dwordx8 s[4:11], s[6:7], 0x0
	s_cmp_lt_i32 s13, 1
	v_mov_b32_e32 v2, 0
	v_mov_b32_e32 v3, 0x358637bd
	s_cbranch_scc1 BB7_4
; %bb.2:                                ; %.lr.ph.preheader
	s_ashr_i32 s1, s0, 31
	s_add_i32 s0, s0, s1
	s_xor_b32 s2, s0, s1
	v_cvt_f32_u32_e32 v2, s2
	s_mov_b32 s14, 0x4f800000
	v_ashrrev_i32_e32 v5, 31, v1
	v_rcp_iflag_f32_e32 v2, v2
	v_mul_f32_e32 v2, s14, v2
	v_cvt_u32_f32_e32 v2, v2
	v_mul_lo_u32 v3, v2, s2
	v_mul_hi_u32 v4, v2, s2
	v_sub_u32_e32 v6, vcc, 0, v3
	v_cmp_eq_u32_e64 s[0:1], 0, v4
	v_cndmask_b32_e64 v3, v3, v6, s[0:1]
	v_mul_hi_u32 v3, v3, v2
	v_add_u32_e32 v4, vcc, v1, v5
	v_xor_b32_e32 v4, v4, v5
	v_add_u32_e32 v6, vcc, v3, v2
	v_subrev_u32_e32 v2, vcc, v3, v2
	v_cndmask_b32_e64 v2, v2, v6, s[0:1]
	s_ashr_i32 s0, s13, 31
	v_mul_hi_u32 v2, v2, v4
	s_add_i32 s1, s13, s0
	s_xor_b32 s12, s1, s0
	v_cvt_f32_u32_e32 v3, s12
	v_mul_lo_u32 v2, v2, s2
	v_rcp_iflag_f32_e32 v3, v3
	v_sub_u32_e32 v6, vcc, v4, v2
	v_add_u32_e64 v7, s[0:1], s2, v6
	v_cmp_le_u32_e32 vcc, s2, v6
	v_cmp_ge_u32_e64 s[0:1], v4, v2
	v_mul_f32_e32 v3, s14, v3
	v_cvt_u32_f32_e32 v3, v3
	v_subrev_u32_e64 v2, s[2:3], s2, v6
	s_and_b64 vcc, vcc, s[0:1]
	v_cndmask_b32_e32 v2, v6, v2, vcc
	v_cndmask_b32_e64 v2, v7, v2, s[0:1]
	v_xor_b32_e32 v2, v2, v5
	v_sub_u32_e32 v2, vcc, v2, v5
	v_mul_lo_u32 v4, v3, s12
	v_mul_hi_u32 v5, v3, s12
	v_ashrrev_i32_e32 v7, 31, v2
	v_sub_u32_e32 v6, vcc, 0, v4
	v_cmp_eq_u32_e64 s[0:1], 0, v5
	v_cndmask_b32_e64 v4, v4, v6, s[0:1]
	v_mul_hi_u32 v4, v4, v3
	v_add_u32_e32 v5, vcc, v2, v7
	v_xor_b32_e32 v5, v5, v7
	v_add_u32_e32 v6, vcc, v4, v3
	v_subrev_u32_e32 v3, vcc, v4, v3
	v_cndmask_b32_e64 v3, v3, v6, s[0:1]
	v_mul_hi_u32 v3, v3, v5
	v_mul_lo_u32 v3, v3, s12
	v_sub_u32_e32 v4, vcc, v5, v3
	v_add_u32_e64 v6, s[0:1], s12, v4
	v_cmp_ge_u32_e64 s[0:1], v5, v3
	v_cmp_le_u32_e32 vcc, s12, v4
	v_subrev_u32_e64 v3, s[2:3], s12, v4
	s_and_b64 vcc, vcc, s[0:1]
	v_cndmask_b32_e32 v3, v4, v3, vcc
	v_cndmask_b32_e64 v3, v6, v3, s[0:1]
	v_xor_b32_e32 v3, v3, v7
	v_sub_u32_e32 v4, vcc, v7, v3
	v_add_u32_e32 v4, vcc, v2, v4
	v_mul_lo_u32 v2, s13, v4
	v_add_u32_e32 v0, vcc, v0, v7
	s_add_i32 s0, s16, s15
	v_add_u32_e32 v0, vcc, s0, v0
	v_add_u32_e32 v2, vcc, v3, v2
	v_sub_u32_e32 v6, vcc, v2, v7
	v_sub_u32_e32 v8, vcc, v0, v3
	v_mov_b32_e32 v2, 0
	v_mov_b32_e32 v3, 0x358637bd
	s_mov_b32 s0, s13
BB7_3:                                  ; %.lr.ph
                                        ; =>This Inner Loop Header: Depth=1
	v_ashrrev_i32_e32 v9, 31, v8
	v_lshlrev_b64 v[9:10], 2, v[8:9]
	s_waitcnt lgkmcnt(0)
	v_mov_b32_e32 v0, s5
	v_add_u32_e32 v9, vcc, s4, v9
	v_addc_u32_e32 v10, vcc, v0, v10, vcc
	v_ashrrev_i32_e32 v7, 31, v6
	flat_load_dword v0, v[9:10]
	v_lshlrev_b64 v[9:10], 2, v[6:7]
	v_mov_b32_e32 v5, s7
	v_add_u32_e32 v9, vcc, s6, v9
	v_addc_u32_e32 v10, vcc, v5, v10, vcc
	v_ashrrev_i32_e32 v5, 31, v4
	flat_load_dword v7, v[9:10]
	v_lshlrev_b64 v[9:10], 2, v[4:5]
	v_mov_b32_e32 v5, s9
	v_add_u32_e32 v9, vcc, s8, v9
	v_addc_u32_e32 v10, vcc, v5, v10, vcc
	flat_load_dword v5, v[9:10]
	s_add_i32 s0, s0, -1
	v_add_u32_e32 v6, vcc, s13, v6
	v_add_u32_e32 v8, vcc, 1, v8
	s_cmp_eq_u32 s0, 0
	v_add_u32_e32 v4, vcc, 1, v4
	s_waitcnt vmcnt(1) lgkmcnt(1)
	v_mac_f32_e32 v2, v0, v7
	s_waitcnt vmcnt(0) lgkmcnt(0)
	v_mac_f32_e32 v3, v0, v5
	s_cbranch_scc0 BB7_3
BB7_4:                                  ; %Flow34
	v_rcp_f32_e32 v3, v3
	v_mov_b32_e32 v0, 0
	v_ashrrev_i64 v[0:1], 30, v[0:1]
	v_mul_f32_e32 v2, v2, v3
	s_waitcnt lgkmcnt(0)
	v_mov_b32_e32 v3, s11
	v_add_u32_e32 v0, vcc, s10, v0
	v_addc_u32_e32 v1, vcc, v3, v1, vcc
	flat_store_dword v[0:1], v2
BB7_5:
	s_endpgm
.Lfunc_end7:
	.size	pf_linear_context, .Lfunc_end7-pf_linear_context
                                        ; -- End function
	.section	.AMDGPU.csdata
; Kernel info:
; codeLenInByte = 688
; NumSgprs: 20
; NumVgprs: 11
; ScratchSize: 0
; MemoryBound: 0
; FloatMode: 192
; IeeeMode: 1
; LDSByteSize: 0 bytes/workgroup (compile time only)
; SGPRBlocks: 2
; VGPRBlocks: 2
; NumSGPRsForWavesPerEU: 20
; NumVGPRsForWavesPerEU: 11
; Occupancy: 10
; WaveLimiterHint : 1
; COMPUTE_PGM_RSRC2:USER_SGPR: 8
; COMPUTE_PGM_RSRC2:TRAP_HANDLER: 0
; COMPUTE_PGM_RSRC2:TGID_X_EN: 1
; COMPUTE_PGM_RSRC2:TGID_Y_EN: 0
; COMPUTE_PGM_RSRC2:TGID_Z_EN: 0
; COMPUTE_PGM_RSRC2:TIDIG_COMP_CNT: 0
	.text
	.protected	pf_o_proj       ; -- Begin function pf_o_proj
	.globl	pf_o_proj
	.p2align	8
	.type	pf_o_proj,@function
	.amdgpu_hsa_kernel pf_o_proj
pf_o_proj:                              ; @pf_o_proj
pf_o_proj$local:
	.amd_kernel_code_t
		amd_code_version_major = 1
		amd_code_version_minor = 2
		amd_machine_kind = 1
		amd_machine_version_major = 8
		amd_machine_version_minor = 0
		amd_machine_version_stepping = 3
		kernel_code_entry_byte_offset = 256
		kernel_code_prefetch_byte_size = 0
		granulated_workitem_vgpr_count = 2
		granulated_wavefront_sgpr_count = 2
		priority = 0
		float_mode = 192
		priv = 0
		enable_dx10_clamp = 1
		debug_mode = 0
		enable_ieee_mode = 1
		enable_wgp_mode = 0
		enable_mem_ordered = 0
		enable_fwd_progress = 0
		enable_sgpr_private_segment_wave_byte_offset = 0
		user_sgpr_count = 8
		enable_trap_handler = 0
		enable_sgpr_workgroup_id_x = 1
		enable_sgpr_workgroup_id_y = 0
		enable_sgpr_workgroup_id_z = 0
		enable_sgpr_workgroup_info = 0
		enable_vgpr_workitem_id = 0
		enable_exception_msb = 0
		granulated_lds_size = 0
		enable_exception = 0
		enable_sgpr_private_segment_buffer = 1
		enable_sgpr_dispatch_ptr = 1
		enable_sgpr_queue_ptr = 0
		enable_sgpr_kernarg_segment_ptr = 1
		enable_sgpr_dispatch_id = 0
		enable_sgpr_flat_scratch_init = 0
		enable_sgpr_private_segment_size = 0
		enable_sgpr_grid_workgroup_count_x = 0
		enable_sgpr_grid_workgroup_count_y = 0
		enable_sgpr_grid_workgroup_count_z = 0
		enable_wavefront_size32 = 0
		enable_ordered_append_gds = 0
		private_element_size = 1
		is_ptr64 = 1
		is_dynamic_callstack = 0
		is_debug_enabled = 0
		is_xnack_enabled = 0
		workitem_private_segment_byte_size = 0
		workgroup_group_segment_byte_size = 0
		gds_segment_byte_size = 0
		kernarg_segment_byte_size = 96
		workgroup_fbarrier_count = 0
		wavefront_sgpr_count = 18
		workitem_vgpr_count = 10
		reserved_vgpr_first = 0
		reserved_vgpr_count = 0
		reserved_sgpr_first = 0
		reserved_sgpr_count = 0
		debug_wavefront_private_segment_offset_sgpr = 0
		debug_private_segment_buffer_sgpr = 0
		kernarg_segment_alignment = 4
		group_segment_alignment = 4
		private_segment_alignment = 4
		wavefront_size = 6
		call_convention = -1
		runtime_loader_kernel_symbol = 0
	.end_amd_kernel_code_t
; %bb.0:
	s_load_dwordx4 s[12:15], s[6:7], 0x18
	s_load_dwordx2 s[2:3], s[6:7], 0x28
	s_load_dword s0, s[4:5], 0x4
	s_waitcnt lgkmcnt(0)
	s_and_b32 s0, s0, 0xffff
	s_mul_i32 s3, s8, s0
	v_add_u32_e32 v1, vcc, s3, v0
	v_add_u32_e32 v1, vcc, s2, v1
	s_mul_i32 s0, s13, s12
	v_cmp_gt_i32_e32 vcc, s0, v1
	s_and_saveexec_b64 s[0:1], vcc
	s_cbranch_execz BB8_5
; %bb.1:
	s_load_dwordx4 s[8:11], s[6:7], 0x10
	s_cmp_lt_i32 s14, 1
	v_mov_b32_e32 v2, 0
	s_cbranch_scc1 BB8_4
; %bb.2:                                ; %.lr.ph.preheader
	s_waitcnt lgkmcnt(0)
	s_ashr_i32 s10, s13, 31
	s_add_i32 s0, s13, s10
	s_xor_b32 s11, s0, s10
	v_cvt_f32_u32_e32 v2, s11
	v_ashrrev_i32_e32 v5, 31, v1
	s_load_dwordx4 s[4:7], s[6:7], 0x0
	v_rcp_iflag_f32_e32 v2, v2
	v_mul_f32_e32 v2, 0x4f800000, v2
	v_cvt_u32_f32_e32 v2, v2
	v_mul_lo_u32 v3, v2, s11
	v_mul_hi_u32 v4, v2, s11
	v_sub_u32_e32 v6, vcc, 0, v3
	v_cmp_eq_u32_e64 s[0:1], 0, v4
	v_cndmask_b32_e64 v3, v3, v6, s[0:1]
	v_mul_hi_u32 v3, v3, v2
	v_add_u32_e32 v4, vcc, v1, v5
	v_xor_b32_e32 v4, v4, v5
	v_add_u32_e32 v6, vcc, v3, v2
	v_subrev_u32_e32 v2, vcc, v3, v2
	v_cndmask_b32_e64 v2, v2, v6, s[0:1]
	v_mul_hi_u32 v2, v2, v4
	v_xor_b32_e32 v3, s10, v5
	s_waitcnt lgkmcnt(0)
	v_mov_b32_e32 v5, s5
	v_mul_lo_u32 v6, v2, s11
	v_add_u32_e32 v7, vcc, 1, v2
	v_add_u32_e32 v8, vcc, -1, v2
	v_sub_u32_e32 v9, vcc, v4, v6
	v_cmp_ge_u32_e32 vcc, v4, v6
	v_cmp_le_u32_e64 s[0:1], s11, v9
	s_and_b64 s[0:1], s[0:1], vcc
	v_cndmask_b32_e64 v2, v2, v7, s[0:1]
	v_cndmask_b32_e32 v2, v8, v2, vcc
	v_xor_b32_e32 v4, v2, v3
	v_sub_u32_e32 v2, vcc, v4, v3
	v_mul_lo_u32 v2, s14, v2
	v_sub_u32_e32 v3, vcc, v3, v4
	v_mul_lo_u32 v6, s13, v3
	s_add_i32 s0, s2, s3
	v_ashrrev_i32_e32 v3, 31, v2
	v_lshlrev_b64 v[3:4], 2, v[2:3]
	v_mov_b32_e32 v2, 0
	v_add_u32_e32 v3, vcc, s4, v3
	v_addc_u32_e32 v4, vcc, v5, v4, vcc
	v_add_u32_e32 v0, vcc, v0, v6
	v_add_u32_e32 v5, vcc, s0, v0
	s_mov_b32 s0, s14
BB8_3:                                  ; %.lr.ph
                                        ; =>This Inner Loop Header: Depth=1
	v_ashrrev_i32_e32 v6, 31, v5
	flat_load_dword v0, v[3:4]
	v_add_u32_e32 v3, vcc, 4, v3
	v_lshlrev_b64 v[6:7], 2, v[5:6]
	v_addc_u32_e32 v4, vcc, 0, v4, vcc
	v_add_u32_e32 v5, vcc, s13, v5
	v_mov_b32_e32 v8, s7
	v_add_u32_e32 v6, vcc, s6, v6
	v_addc_u32_e32 v7, vcc, v8, v7, vcc
	flat_load_dword v6, v[6:7]
	s_add_i32 s0, s0, -1
	s_cmp_eq_u32 s0, 0
	s_waitcnt vmcnt(0) lgkmcnt(0)
	v_mac_f32_e32 v2, v0, v6
	s_cbranch_scc0 BB8_3
BB8_4:                                  ; %Flow22
	v_mov_b32_e32 v0, 0
	v_ashrrev_i64 v[0:1], 30, v[0:1]
	s_waitcnt lgkmcnt(0)
	v_mov_b32_e32 v3, s9
	v_add_u32_e32 v0, vcc, s8, v0
	v_addc_u32_e32 v1, vcc, v3, v1, vcc
	flat_store_dword v[0:1], v2
BB8_5:
	s_endpgm
.Lfunc_end8:
	.size	pf_o_proj, .Lfunc_end8-pf_o_proj
                                        ; -- End function
	.section	.AMDGPU.csdata
; Kernel info:
; codeLenInByte = 448
; NumSgprs: 18
; NumVgprs: 10
; ScratchSize: 0
; MemoryBound: 0
; FloatMode: 192
; IeeeMode: 1
; LDSByteSize: 0 bytes/workgroup (compile time only)
; SGPRBlocks: 2
; VGPRBlocks: 2
; NumSGPRsForWavesPerEU: 18
; NumVGPRsForWavesPerEU: 10
; Occupancy: 10
; WaveLimiterHint : 1
; COMPUTE_PGM_RSRC2:USER_SGPR: 8
; COMPUTE_PGM_RSRC2:TRAP_HANDLER: 0
; COMPUTE_PGM_RSRC2:TGID_X_EN: 1
; COMPUTE_PGM_RSRC2:TGID_Y_EN: 0
; COMPUTE_PGM_RSRC2:TGID_Z_EN: 0
; COMPUTE_PGM_RSRC2:TIDIG_COMP_CNT: 0
	.text
	.protected	pf_add_resid1   ; -- Begin function pf_add_resid1
	.globl	pf_add_resid1
	.p2align	8
	.type	pf_add_resid1,@function
	.amdgpu_hsa_kernel pf_add_resid1
pf_add_resid1:                          ; @pf_add_resid1
pf_add_resid1$local:
	.amd_kernel_code_t
		amd_code_version_major = 1
		amd_code_version_minor = 2
		amd_machine_kind = 1
		amd_machine_version_major = 8
		amd_machine_version_minor = 0
		amd_machine_version_stepping = 3
		kernel_code_entry_byte_offset = 256
		kernel_code_prefetch_byte_size = 0
		granulated_workitem_vgpr_count = 1
		granulated_wavefront_sgpr_count = 1
		priority = 0
		float_mode = 192
		priv = 0
		enable_dx10_clamp = 1
		debug_mode = 0
		enable_ieee_mode = 1
		enable_wgp_mode = 0
		enable_mem_ordered = 0
		enable_fwd_progress = 0
		enable_sgpr_private_segment_wave_byte_offset = 0
		user_sgpr_count = 8
		enable_trap_handler = 0
		enable_sgpr_workgroup_id_x = 1
		enable_sgpr_workgroup_id_y = 0
		enable_sgpr_workgroup_id_z = 0
		enable_sgpr_workgroup_info = 0
		enable_vgpr_workitem_id = 0
		enable_exception_msb = 0
		granulated_lds_size = 0
		enable_exception = 0
		enable_sgpr_private_segment_buffer = 1
		enable_sgpr_dispatch_ptr = 1
		enable_sgpr_queue_ptr = 0
		enable_sgpr_kernarg_segment_ptr = 1
		enable_sgpr_dispatch_id = 0
		enable_sgpr_flat_scratch_init = 0
		enable_sgpr_private_segment_size = 0
		enable_sgpr_grid_workgroup_count_x = 0
		enable_sgpr_grid_workgroup_count_y = 0
		enable_sgpr_grid_workgroup_count_z = 0
		enable_wavefront_size32 = 0
		enable_ordered_append_gds = 0
		private_element_size = 1
		is_ptr64 = 1
		is_dynamic_callstack = 0
		is_debug_enabled = 0
		is_xnack_enabled = 0
		workitem_private_segment_byte_size = 0
		workgroup_group_segment_byte_size = 0
		gds_segment_byte_size = 0
		kernarg_segment_byte_size = 88
		workgroup_fbarrier_count = 0
		wavefront_sgpr_count = 11
		workitem_vgpr_count = 6
		reserved_vgpr_first = 0
		reserved_vgpr_count = 0
		reserved_sgpr_first = 0
		reserved_sgpr_count = 0
		debug_wavefront_private_segment_offset_sgpr = 0
		debug_private_segment_buffer_sgpr = 0
		kernarg_segment_alignment = 4
		group_segment_alignment = 4
		private_segment_alignment = 4
		wavefront_size = 6
		call_convention = -1
		runtime_loader_kernel_symbol = 0
	.end_amd_kernel_code_t
; %bb.0:
	s_load_dword s0, s[4:5], 0x4
	s_waitcnt lgkmcnt(0)
	s_and_b32 s0, s0, 0xffff
	s_mul_i32 s8, s8, s0
	s_load_dword s2, s[6:7], 0x18
	s_load_dwordx2 s[0:1], s[6:7], 0x20
	v_add_u32_e32 v0, vcc, s8, v0
	s_waitcnt lgkmcnt(0)
	v_add_u32_e32 v0, vcc, s0, v0
	v_cmp_gt_i32_e32 vcc, s2, v0
	s_and_saveexec_b64 s[0:1], vcc
	s_cbranch_execz BB9_2
; %bb.1:
	s_load_dwordx4 s[0:3], s[6:7], 0x0
	s_load_dwordx4 s[4:7], s[6:7], 0x10
	v_mov_b32_e32 v1, 0
	v_mov_b32_e32 v2, v0
	v_ashrrev_i64 v[0:1], 30, v[1:2]
	s_waitcnt lgkmcnt(0)
	v_mov_b32_e32 v3, s1
	v_add_u32_e32 v2, vcc, s0, v0
	v_addc_u32_e32 v3, vcc, v3, v1, vcc
	flat_load_dword v4, v[2:3]
	v_mov_b32_e32 v3, s3
	v_add_u32_e32 v2, vcc, s2, v0
	v_addc_u32_e32 v3, vcc, v3, v1, vcc
	flat_load_dword v2, v[2:3]
	v_mov_b32_e32 v5, s5
	v_add_u32_e32 v0, vcc, s4, v0
	v_addc_u32_e32 v1, vcc, v5, v1, vcc
	s_waitcnt vmcnt(0) lgkmcnt(0)
	v_add_f32_e32 v2, v4, v2
	flat_store_dword v[0:1], v2
BB9_2:
	s_endpgm
.Lfunc_end9:
	.size	pf_add_resid1, .Lfunc_end9-pf_add_resid1
                                        ; -- End function
	.section	.AMDGPU.csdata
; Kernel info:
; codeLenInByte = 172
; NumSgprs: 11
; NumVgprs: 6
; ScratchSize: 0
; MemoryBound: 0
; FloatMode: 192
; IeeeMode: 1
; LDSByteSize: 0 bytes/workgroup (compile time only)
; SGPRBlocks: 1
; VGPRBlocks: 1
; NumSGPRsForWavesPerEU: 11
; NumVGPRsForWavesPerEU: 6
; Occupancy: 10
; WaveLimiterHint : 1
; COMPUTE_PGM_RSRC2:USER_SGPR: 8
; COMPUTE_PGM_RSRC2:TRAP_HANDLER: 0
; COMPUTE_PGM_RSRC2:TGID_X_EN: 1
; COMPUTE_PGM_RSRC2:TGID_Y_EN: 0
; COMPUTE_PGM_RSRC2:TGID_Z_EN: 0
; COMPUTE_PGM_RSRC2:TIDIG_COMP_CNT: 0
	.text
	.protected	pf_add_resid2   ; -- Begin function pf_add_resid2
	.globl	pf_add_resid2
	.p2align	8
	.type	pf_add_resid2,@function
	.amdgpu_hsa_kernel pf_add_resid2
pf_add_resid2:                          ; @pf_add_resid2
pf_add_resid2$local:
	.amd_kernel_code_t
		amd_code_version_major = 1
		amd_code_version_minor = 2
		amd_machine_kind = 1
		amd_machine_version_major = 8
		amd_machine_version_minor = 0
		amd_machine_version_stepping = 3
		kernel_code_entry_byte_offset = 256
		kernel_code_prefetch_byte_size = 0
		granulated_workitem_vgpr_count = 1
		granulated_wavefront_sgpr_count = 1
		priority = 0
		float_mode = 192
		priv = 0
		enable_dx10_clamp = 1
		debug_mode = 0
		enable_ieee_mode = 1
		enable_wgp_mode = 0
		enable_mem_ordered = 0
		enable_fwd_progress = 0
		enable_sgpr_private_segment_wave_byte_offset = 0
		user_sgpr_count = 8
		enable_trap_handler = 0
		enable_sgpr_workgroup_id_x = 1
		enable_sgpr_workgroup_id_y = 0
		enable_sgpr_workgroup_id_z = 0
		enable_sgpr_workgroup_info = 0
		enable_vgpr_workitem_id = 0
		enable_exception_msb = 0
		granulated_lds_size = 0
		enable_exception = 0
		enable_sgpr_private_segment_buffer = 1
		enable_sgpr_dispatch_ptr = 1
		enable_sgpr_queue_ptr = 0
		enable_sgpr_kernarg_segment_ptr = 1
		enable_sgpr_dispatch_id = 0
		enable_sgpr_flat_scratch_init = 0
		enable_sgpr_private_segment_size = 0
		enable_sgpr_grid_workgroup_count_x = 0
		enable_sgpr_grid_workgroup_count_y = 0
		enable_sgpr_grid_workgroup_count_z = 0
		enable_wavefront_size32 = 0
		enable_ordered_append_gds = 0
		private_element_size = 1
		is_ptr64 = 1
		is_dynamic_callstack = 0
		is_debug_enabled = 0
		is_xnack_enabled = 0
		workitem_private_segment_byte_size = 0
		workgroup_group_segment_byte_size = 0
		gds_segment_byte_size = 0
		kernarg_segment_byte_size = 88
		workgroup_fbarrier_count = 0
		wavefront_sgpr_count = 11
		workitem_vgpr_count = 6
		reserved_vgpr_first = 0
		reserved_vgpr_count = 0
		reserved_sgpr_first = 0
		reserved_sgpr_count = 0
		debug_wavefront_private_segment_offset_sgpr = 0
		debug_private_segment_buffer_sgpr = 0
		kernarg_segment_alignment = 4
		group_segment_alignment = 4
		private_segment_alignment = 4
		wavefront_size = 6
		call_convention = -1
		runtime_loader_kernel_symbol = 0
	.end_amd_kernel_code_t
; %bb.0:
	s_load_dword s0, s[4:5], 0x4
	s_waitcnt lgkmcnt(0)
	s_and_b32 s0, s0, 0xffff
	s_mul_i32 s8, s8, s0
	s_load_dword s2, s[6:7], 0x18
	s_load_dwordx2 s[0:1], s[6:7], 0x20
	v_add_u32_e32 v0, vcc, s8, v0
	s_waitcnt lgkmcnt(0)
	v_add_u32_e32 v0, vcc, s0, v0
	v_cmp_gt_i32_e32 vcc, s2, v0
	s_and_saveexec_b64 s[0:1], vcc
	s_cbranch_execz BB10_2
; %bb.1:
	s_load_dwordx4 s[0:3], s[6:7], 0x0
	s_load_dwordx4 s[4:7], s[6:7], 0x10
	v_mov_b32_e32 v1, 0
	v_mov_b32_e32 v2, v0
	v_ashrrev_i64 v[0:1], 30, v[1:2]
	s_waitcnt lgkmcnt(0)
	v_mov_b32_e32 v3, s1
	v_add_u32_e32 v2, vcc, s0, v0
	v_addc_u32_e32 v3, vcc, v3, v1, vcc
	flat_load_dword v4, v[2:3]
	v_mov_b32_e32 v3, s3
	v_add_u32_e32 v2, vcc, s2, v0
	v_addc_u32_e32 v3, vcc, v3, v1, vcc
	flat_load_dword v2, v[2:3]
	v_mov_b32_e32 v5, s5
	v_add_u32_e32 v0, vcc, s4, v0
	v_addc_u32_e32 v1, vcc, v5, v1, vcc
	s_waitcnt vmcnt(0) lgkmcnt(0)
	v_add_f32_e32 v2, v4, v2
	flat_store_dword v[0:1], v2
BB10_2:
	s_endpgm
.Lfunc_end10:
	.size	pf_add_resid2, .Lfunc_end10-pf_add_resid2
                                        ; -- End function
	.section	.AMDGPU.csdata
; Kernel info:
; codeLenInByte = 172
; NumSgprs: 11
; NumVgprs: 6
; ScratchSize: 0
; MemoryBound: 0
; FloatMode: 192
; IeeeMode: 1
; LDSByteSize: 0 bytes/workgroup (compile time only)
; SGPRBlocks: 1
; VGPRBlocks: 1
; NumSGPRsForWavesPerEU: 11
; NumVGPRsForWavesPerEU: 6
; Occupancy: 10
; WaveLimiterHint : 1
; COMPUTE_PGM_RSRC2:USER_SGPR: 8
; COMPUTE_PGM_RSRC2:TRAP_HANDLER: 0
; COMPUTE_PGM_RSRC2:TGID_X_EN: 1
; COMPUTE_PGM_RSRC2:TGID_Y_EN: 0
; COMPUTE_PGM_RSRC2:TGID_Z_EN: 0
; COMPUTE_PGM_RSRC2:TIDIG_COMP_CNT: 0
	.text
	.protected	pf_layernorm1   ; -- Begin function pf_layernorm1
	.globl	pf_layernorm1
	.p2align	8
	.type	pf_layernorm1,@function
	.amdgpu_hsa_kernel pf_layernorm1
pf_layernorm1:                          ; @pf_layernorm1
pf_layernorm1$local:
	.amd_kernel_code_t
		amd_code_version_major = 1
		amd_code_version_minor = 2
		amd_machine_kind = 1
		amd_machine_version_major = 8
		amd_machine_version_minor = 0
		amd_machine_version_stepping = 3
		kernel_code_entry_byte_offset = 256
		kernel_code_prefetch_byte_size = 0
		granulated_workitem_vgpr_count = 2
		granulated_wavefront_sgpr_count = 1
		priority = 0
		float_mode = 192
		priv = 0
		enable_dx10_clamp = 1
		debug_mode = 0
		enable_ieee_mode = 1
		enable_wgp_mode = 0
		enable_mem_ordered = 0
		enable_fwd_progress = 0
		enable_sgpr_private_segment_wave_byte_offset = 0
		user_sgpr_count = 8
		enable_trap_handler = 0
		enable_sgpr_workgroup_id_x = 1
		enable_sgpr_workgroup_id_y = 0
		enable_sgpr_workgroup_id_z = 0
		enable_sgpr_workgroup_info = 0
		enable_vgpr_workitem_id = 0
		enable_exception_msb = 0
		granulated_lds_size = 0
		enable_exception = 0
		enable_sgpr_private_segment_buffer = 1
		enable_sgpr_dispatch_ptr = 1
		enable_sgpr_queue_ptr = 0
		enable_sgpr_kernarg_segment_ptr = 1
		enable_sgpr_dispatch_id = 0
		enable_sgpr_flat_scratch_init = 0
		enable_sgpr_private_segment_size = 0
		enable_sgpr_grid_workgroup_count_x = 0
		enable_sgpr_grid_workgroup_count_y = 0
		enable_sgpr_grid_workgroup_count_z = 0
		enable_wavefront_size32 = 0
		enable_ordered_append_gds = 0
		private_element_size = 1
		is_ptr64 = 1
		is_dynamic_callstack = 0
		is_debug_enabled = 0
		is_xnack_enabled = 0
		workitem_private_segment_byte_size = 0
		workgroup_group_segment_byte_size = 0
		gds_segment_byte_size = 0
		kernarg_segment_byte_size = 104
		workgroup_fbarrier_count = 0
		wavefront_sgpr_count = 14
		workitem_vgpr_count = 11
		reserved_vgpr_first = 0
		reserved_vgpr_count = 0
		reserved_sgpr_first = 0
		reserved_sgpr_count = 0
		debug_wavefront_private_segment_offset_sgpr = 0
		debug_private_segment_buffer_sgpr = 0
		kernarg_segment_alignment = 4
		group_segment_alignment = 4
		private_segment_alignment = 4
		wavefront_size = 6
		call_convention = -1
		runtime_loader_kernel_symbol = 0
	.end_amd_kernel_code_t
; %bb.0:
	s_load_dwordx4 s[0:3], s[6:7], 0x20
	s_waitcnt lgkmcnt(0)
	s_load_dword s3, s[6:7], 0x30
	s_load_dword s4, s[4:5], 0x4
	s_waitcnt lgkmcnt(0)
	s_and_b32 s4, s4, 0xffff
	s_mul_i32 s8, s8, s4
	s_add_i32 s3, s3, s8
	v_add_u32_e32 v0, vcc, s3, v0
	v_cmp_gt_i32_e32 vcc, s0, v0
	s_and_saveexec_b64 s[4:5], vcc
	s_cbranch_execz BB11_8
; %bb.1:
	s_cmp_lt_i32 s1, 1
	s_cbranch_scc1 BB11_8
; %bb.2:                                ; %.lr.ph71.preheader
	v_mul_lo_u32 v3, s1, v0
	s_load_dwordx8 s[4:11], s[6:7], 0x0
	s_mov_b32 s0, s1
	v_ashrrev_i32_e32 v4, 31, v3
	v_lshlrev_b64 v[0:1], 2, v[3:4]
	s_waitcnt lgkmcnt(0)
	v_mov_b32_e32 v2, s5
	v_add_u32_e32 v0, vcc, s4, v0
	v_addc_u32_e32 v1, vcc, v2, v1, vcc
	v_mov_b32_e32 v6, v1
	v_mov_b32_e32 v2, 0
	v_mov_b32_e32 v5, v0
BB11_3:                                 ; %.lr.ph71
                                        ; =>This Inner Loop Header: Depth=1
	flat_load_dword v7, v[5:6]
	v_add_u32_e32 v5, vcc, 4, v5
	s_add_i32 s0, s0, -1
	v_addc_u32_e32 v6, vcc, 0, v6, vcc
	s_cmp_eq_u32 s0, 0
	s_waitcnt vmcnt(0) lgkmcnt(0)
	v_add_f32_e32 v2, v2, v7
	s_cbranch_scc0 BB11_3
; %bb.4:                                ; %.lr.ph67.preheader
	v_cvt_f32_i32_e32 v5, s1
	v_mov_b32_e32 v8, v1
	v_mov_b32_e32 v6, 0
	s_mov_b32 s0, s1
	v_rcp_iflag_f32_e32 v5, v5
	v_mov_b32_e32 v7, v0
	v_mul_f32_e32 v2, v5, v2
BB11_5:                                 ; %.lr.ph67
                                        ; =>This Inner Loop Header: Depth=1
	flat_load_dword v9, v[7:8]
	v_add_u32_e32 v7, vcc, 4, v7
	s_add_i32 s0, s0, -1
	v_addc_u32_e32 v8, vcc, 0, v8, vcc
	s_cmp_eq_u32 s0, 0
	s_waitcnt vmcnt(0) lgkmcnt(0)
	v_sub_f32_e32 v9, v9, v2
	v_mac_f32_e32 v6, v9, v9
	s_cbranch_scc0 BB11_5
; %bb.6:                                ; %.lr.ph.preheader
	v_mad_f32 v5, v5, v6, s2
	v_lshlrev_b64 v[3:4], 2, v[3:4]
	v_rsq_f32_e32 v5, v5
	v_mov_b32_e32 v6, s11
	v_add_u32_e32 v3, vcc, s10, v3
	v_addc_u32_e32 v4, vcc, v6, v4, vcc
BB11_7:                                 ; %.lr.ph
                                        ; =>This Inner Loop Header: Depth=1
	flat_load_dword v10, v[0:1]
	v_mov_b32_e32 v6, s6
	v_mov_b32_e32 v8, s8
	v_mov_b32_e32 v7, s7
	v_mov_b32_e32 v9, s9
	flat_load_dword v6, v[6:7]
	flat_load_dword v7, v[8:9]
	s_add_i32 s1, s1, -1
	s_add_u32 s6, s6, 4
	v_add_u32_e32 v0, vcc, 4, v0
	s_addc_u32 s7, s7, 0
	s_add_u32 s8, s8, 4
	v_addc_u32_e32 v1, vcc, 0, v1, vcc
	s_addc_u32 s9, s9, 0
	s_cmp_lg_u32 s1, 0
	s_waitcnt vmcnt(2) lgkmcnt(2)
	v_sub_f32_e32 v8, v10, v2
	v_mul_f32_e32 v8, v5, v8
	s_waitcnt vmcnt(0) lgkmcnt(0)
	v_mac_f32_e32 v7, v8, v6
	flat_store_dword v[3:4], v7
	v_add_u32_e32 v3, vcc, 4, v3
	v_addc_u32_e32 v4, vcc, 0, v4, vcc
	s_cbranch_scc1 BB11_7
BB11_8:                                 ; %.loopexit
	s_endpgm
.Lfunc_end11:
	.size	pf_layernorm1, .Lfunc_end11-pf_layernorm1
                                        ; -- End function
	.section	.AMDGPU.csdata
; Kernel info:
; codeLenInByte = 384
; NumSgprs: 14
; NumVgprs: 11
; ScratchSize: 0
; MemoryBound: 0
; FloatMode: 192
; IeeeMode: 1
; LDSByteSize: 0 bytes/workgroup (compile time only)
; SGPRBlocks: 1
; VGPRBlocks: 2
; NumSGPRsForWavesPerEU: 14
; NumVGPRsForWavesPerEU: 11
; Occupancy: 10
; WaveLimiterHint : 0
; COMPUTE_PGM_RSRC2:USER_SGPR: 8
; COMPUTE_PGM_RSRC2:TRAP_HANDLER: 0
; COMPUTE_PGM_RSRC2:TGID_X_EN: 1
; COMPUTE_PGM_RSRC2:TGID_Y_EN: 0
; COMPUTE_PGM_RSRC2:TGID_Z_EN: 0
; COMPUTE_PGM_RSRC2:TIDIG_COMP_CNT: 0
	.text
	.protected	pf_layernorm2   ; -- Begin function pf_layernorm2
	.globl	pf_layernorm2
	.p2align	8
	.type	pf_layernorm2,@function
	.amdgpu_hsa_kernel pf_layernorm2
pf_layernorm2:                          ; @pf_layernorm2
pf_layernorm2$local:
	.amd_kernel_code_t
		amd_code_version_major = 1
		amd_code_version_minor = 2
		amd_machine_kind = 1
		amd_machine_version_major = 8
		amd_machine_version_minor = 0
		amd_machine_version_stepping = 3
		kernel_code_entry_byte_offset = 256
		kernel_code_prefetch_byte_size = 0
		granulated_workitem_vgpr_count = 2
		granulated_wavefront_sgpr_count = 1
		priority = 0
		float_mode = 192
		priv = 0
		enable_dx10_clamp = 1
		debug_mode = 0
		enable_ieee_mode = 1
		enable_wgp_mode = 0
		enable_mem_ordered = 0
		enable_fwd_progress = 0
		enable_sgpr_private_segment_wave_byte_offset = 0
		user_sgpr_count = 8
		enable_trap_handler = 0
		enable_sgpr_workgroup_id_x = 1
		enable_sgpr_workgroup_id_y = 0
		enable_sgpr_workgroup_id_z = 0
		enable_sgpr_workgroup_info = 0
		enable_vgpr_workitem_id = 0
		enable_exception_msb = 0
		granulated_lds_size = 0
		enable_exception = 0
		enable_sgpr_private_segment_buffer = 1
		enable_sgpr_dispatch_ptr = 1
		enable_sgpr_queue_ptr = 0
		enable_sgpr_kernarg_segment_ptr = 1
		enable_sgpr_dispatch_id = 0
		enable_sgpr_flat_scratch_init = 0
		enable_sgpr_private_segment_size = 0
		enable_sgpr_grid_workgroup_count_x = 0
		enable_sgpr_grid_workgroup_count_y = 0
		enable_sgpr_grid_workgroup_count_z = 0
		enable_wavefront_size32 = 0
		enable_ordered_append_gds = 0
		private_element_size = 1
		is_ptr64 = 1
		is_dynamic_callstack = 0
		is_debug_enabled = 0
		is_xnack_enabled = 0
		workitem_private_segment_byte_size = 0
		workgroup_group_segment_byte_size = 0
		gds_segment_byte_size = 0
		kernarg_segment_byte_size = 104
		workgroup_fbarrier_count = 0
		wavefront_sgpr_count = 14
		workitem_vgpr_count = 11
		reserved_vgpr_first = 0
		reserved_vgpr_count = 0
		reserved_sgpr_first = 0
		reserved_sgpr_count = 0
		debug_wavefront_private_segment_offset_sgpr = 0
		debug_private_segment_buffer_sgpr = 0
		kernarg_segment_alignment = 4
		group_segment_alignment = 4
		private_segment_alignment = 4
		wavefront_size = 6
		call_convention = -1
		runtime_loader_kernel_symbol = 0
	.end_amd_kernel_code_t
; %bb.0:
	s_load_dwordx4 s[0:3], s[6:7], 0x20
	s_waitcnt lgkmcnt(0)
	s_load_dword s3, s[6:7], 0x30
	s_load_dword s4, s[4:5], 0x4
	s_waitcnt lgkmcnt(0)
	s_and_b32 s4, s4, 0xffff
	s_mul_i32 s8, s8, s4
	s_add_i32 s3, s3, s8
	v_add_u32_e32 v0, vcc, s3, v0
	v_cmp_gt_i32_e32 vcc, s0, v0
	s_and_saveexec_b64 s[4:5], vcc
	s_cbranch_execz BB12_8
; %bb.1:
	s_cmp_lt_i32 s1, 1
	s_cbranch_scc1 BB12_8
; %bb.2:                                ; %.lr.ph71.preheader
	v_mul_lo_u32 v3, s1, v0
	s_load_dwordx8 s[4:11], s[6:7], 0x0
	s_mov_b32 s0, s1
	v_ashrrev_i32_e32 v4, 31, v3
	v_lshlrev_b64 v[0:1], 2, v[3:4]
	s_waitcnt lgkmcnt(0)
	v_mov_b32_e32 v2, s5
	v_add_u32_e32 v0, vcc, s4, v0
	v_addc_u32_e32 v1, vcc, v2, v1, vcc
	v_mov_b32_e32 v6, v1
	v_mov_b32_e32 v2, 0
	v_mov_b32_e32 v5, v0
BB12_3:                                 ; %.lr.ph71
                                        ; =>This Inner Loop Header: Depth=1
	flat_load_dword v7, v[5:6]
	v_add_u32_e32 v5, vcc, 4, v5
	s_add_i32 s0, s0, -1
	v_addc_u32_e32 v6, vcc, 0, v6, vcc
	s_cmp_eq_u32 s0, 0
	s_waitcnt vmcnt(0) lgkmcnt(0)
	v_add_f32_e32 v2, v2, v7
	s_cbranch_scc0 BB12_3
; %bb.4:                                ; %.lr.ph67.preheader
	v_cvt_f32_i32_e32 v5, s1
	v_mov_b32_e32 v8, v1
	v_mov_b32_e32 v6, 0
	s_mov_b32 s0, s1
	v_rcp_iflag_f32_e32 v5, v5
	v_mov_b32_e32 v7, v0
	v_mul_f32_e32 v2, v5, v2
BB12_5:                                 ; %.lr.ph67
                                        ; =>This Inner Loop Header: Depth=1
	flat_load_dword v9, v[7:8]
	v_add_u32_e32 v7, vcc, 4, v7
	s_add_i32 s0, s0, -1
	v_addc_u32_e32 v8, vcc, 0, v8, vcc
	s_cmp_eq_u32 s0, 0
	s_waitcnt vmcnt(0) lgkmcnt(0)
	v_sub_f32_e32 v9, v9, v2
	v_mac_f32_e32 v6, v9, v9
	s_cbranch_scc0 BB12_5
; %bb.6:                                ; %.lr.ph.preheader
	v_mad_f32 v5, v5, v6, s2
	v_lshlrev_b64 v[3:4], 2, v[3:4]
	v_rsq_f32_e32 v5, v5
	v_mov_b32_e32 v6, s11
	v_add_u32_e32 v3, vcc, s10, v3
	v_addc_u32_e32 v4, vcc, v6, v4, vcc
BB12_7:                                 ; %.lr.ph
                                        ; =>This Inner Loop Header: Depth=1
	flat_load_dword v10, v[0:1]
	v_mov_b32_e32 v6, s6
	v_mov_b32_e32 v8, s8
	v_mov_b32_e32 v7, s7
	v_mov_b32_e32 v9, s9
	flat_load_dword v6, v[6:7]
	flat_load_dword v7, v[8:9]
	s_add_i32 s1, s1, -1
	s_add_u32 s6, s6, 4
	v_add_u32_e32 v0, vcc, 4, v0
	s_addc_u32 s7, s7, 0
	s_add_u32 s8, s8, 4
	v_addc_u32_e32 v1, vcc, 0, v1, vcc
	s_addc_u32 s9, s9, 0
	s_cmp_lg_u32 s1, 0
	s_waitcnt vmcnt(2) lgkmcnt(2)
	v_sub_f32_e32 v8, v10, v2
	v_mul_f32_e32 v8, v5, v8
	s_waitcnt vmcnt(0) lgkmcnt(0)
	v_mac_f32_e32 v7, v8, v6
	flat_store_dword v[3:4], v7
	v_add_u32_e32 v3, vcc, 4, v3
	v_addc_u32_e32 v4, vcc, 0, v4, vcc
	s_cbranch_scc1 BB12_7
BB12_8:                                 ; %.loopexit
	s_endpgm
.Lfunc_end12:
	.size	pf_layernorm2, .Lfunc_end12-pf_layernorm2
                                        ; -- End function
	.section	.AMDGPU.csdata
; Kernel info:
; codeLenInByte = 384
; NumSgprs: 14
; NumVgprs: 11
; ScratchSize: 0
; MemoryBound: 0
; FloatMode: 192
; IeeeMode: 1
; LDSByteSize: 0 bytes/workgroup (compile time only)
; SGPRBlocks: 1
; VGPRBlocks: 2
; NumSGPRsForWavesPerEU: 14
; NumVGPRsForWavesPerEU: 11
; Occupancy: 10
; WaveLimiterHint : 0
; COMPUTE_PGM_RSRC2:USER_SGPR: 8
; COMPUTE_PGM_RSRC2:TRAP_HANDLER: 0
; COMPUTE_PGM_RSRC2:TGID_X_EN: 1
; COMPUTE_PGM_RSRC2:TGID_Y_EN: 0
; COMPUTE_PGM_RSRC2:TGID_Z_EN: 0
; COMPUTE_PGM_RSRC2:TIDIG_COMP_CNT: 0
	.text
	.protected	pf_ff1          ; -- Begin function pf_ff1
	.globl	pf_ff1
	.p2align	8
	.type	pf_ff1,@function
	.amdgpu_hsa_kernel pf_ff1
pf_ff1:                                 ; @pf_ff1
pf_ff1$local:
	.amd_kernel_code_t
		amd_code_version_major = 1
		amd_code_version_minor = 2
		amd_machine_kind = 1
		amd_machine_version_major = 8
		amd_machine_version_minor = 0
		amd_machine_version_stepping = 3
		kernel_code_entry_byte_offset = 256
		kernel_code_prefetch_byte_size = 0
		granulated_workitem_vgpr_count = 2
		granulated_wavefront_sgpr_count = 2
		priority = 0
		float_mode = 192
		priv = 0
		enable_dx10_clamp = 1
		debug_mode = 0
		enable_ieee_mode = 1
		enable_wgp_mode = 0
		enable_mem_ordered = 0
		enable_fwd_progress = 0
		enable_sgpr_private_segment_wave_byte_offset = 0
		user_sgpr_count = 8
		enable_trap_handler = 0
		enable_sgpr_workgroup_id_x = 1
		enable_sgpr_workgroup_id_y = 0
		enable_sgpr_workgroup_id_z = 0
		enable_sgpr_workgroup_info = 0
		enable_vgpr_workitem_id = 0
		enable_exception_msb = 0
		granulated_lds_size = 0
		enable_exception = 0
		enable_sgpr_private_segment_buffer = 1
		enable_sgpr_dispatch_ptr = 1
		enable_sgpr_queue_ptr = 0
		enable_sgpr_kernarg_segment_ptr = 1
		enable_sgpr_dispatch_id = 0
		enable_sgpr_flat_scratch_init = 0
		enable_sgpr_private_segment_size = 0
		enable_sgpr_grid_workgroup_count_x = 0
		enable_sgpr_grid_workgroup_count_y = 0
		enable_sgpr_grid_workgroup_count_z = 0
		enable_wavefront_size32 = 0
		enable_ordered_append_gds = 0
		private_element_size = 1
		is_ptr64 = 1
		is_dynamic_callstack = 0
		is_debug_enabled = 0
		is_xnack_enabled = 0
		workitem_private_segment_byte_size = 0
		workgroup_group_segment_byte_size = 0
		gds_segment_byte_size = 0
		kernarg_segment_byte_size = 96
		workgroup_fbarrier_count = 0
		wavefront_sgpr_count = 18
		workitem_vgpr_count = 10
		reserved_vgpr_first = 0
		reserved_vgpr_count = 0
		reserved_sgpr_first = 0
		reserved_sgpr_count = 0
		debug_wavefront_private_segment_offset_sgpr = 0
		debug_private_segment_buffer_sgpr = 0
		kernarg_segment_alignment = 4
		group_segment_alignment = 4
		private_segment_alignment = 4
		wavefront_size = 6
		call_convention = -1
		runtime_loader_kernel_symbol = 0
	.end_amd_kernel_code_t
; %bb.0:
	s_load_dwordx4 s[12:15], s[6:7], 0x18
	s_load_dwordx2 s[2:3], s[6:7], 0x28
	s_load_dword s0, s[4:5], 0x4
	s_waitcnt lgkmcnt(0)
	s_and_b32 s0, s0, 0xffff
	s_mul_i32 s3, s8, s0
	v_add_u32_e32 v1, vcc, s3, v0
	v_add_u32_e32 v1, vcc, s2, v1
	s_mul_i32 s0, s13, s12
	v_cmp_gt_i32_e32 vcc, s0, v1
	s_and_saveexec_b64 s[0:1], vcc
	s_cbranch_execz BB13_5
; %bb.1:
	s_load_dwordx4 s[8:11], s[6:7], 0x10
	s_cmp_lt_i32 s14, 1
	v_mov_b32_e32 v2, 0
	s_cbranch_scc1 BB13_4
; %bb.2:                                ; %.lr.ph.preheader
	s_waitcnt lgkmcnt(0)
	s_ashr_i32 s10, s13, 31
	s_add_i32 s0, s13, s10
	s_xor_b32 s11, s0, s10
	v_cvt_f32_u32_e32 v2, s11
	v_ashrrev_i32_e32 v5, 31, v1
	s_load_dwordx4 s[4:7], s[6:7], 0x0
	v_rcp_iflag_f32_e32 v2, v2
	v_mul_f32_e32 v2, 0x4f800000, v2
	v_cvt_u32_f32_e32 v2, v2
	v_mul_lo_u32 v3, v2, s11
	v_mul_hi_u32 v4, v2, s11
	v_sub_u32_e32 v6, vcc, 0, v3
	v_cmp_eq_u32_e64 s[0:1], 0, v4
	v_cndmask_b32_e64 v3, v3, v6, s[0:1]
	v_mul_hi_u32 v3, v3, v2
	v_add_u32_e32 v4, vcc, v1, v5
	v_xor_b32_e32 v4, v4, v5
	v_add_u32_e32 v6, vcc, v3, v2
	v_subrev_u32_e32 v2, vcc, v3, v2
	v_cndmask_b32_e64 v2, v2, v6, s[0:1]
	v_mul_hi_u32 v2, v2, v4
	v_xor_b32_e32 v3, s10, v5
	s_waitcnt lgkmcnt(0)
	v_mov_b32_e32 v5, s5
	v_mul_lo_u32 v6, v2, s11
	v_add_u32_e32 v7, vcc, 1, v2
	v_add_u32_e32 v8, vcc, -1, v2
	v_sub_u32_e32 v9, vcc, v4, v6
	v_cmp_ge_u32_e32 vcc, v4, v6
	v_cmp_le_u32_e64 s[0:1], s11, v9
	s_and_b64 s[0:1], s[0:1], vcc
	v_cndmask_b32_e64 v2, v2, v7, s[0:1]
	v_cndmask_b32_e32 v2, v8, v2, vcc
	v_xor_b32_e32 v4, v2, v3
	v_sub_u32_e32 v2, vcc, v4, v3
	v_mul_lo_u32 v2, s14, v2
	v_sub_u32_e32 v3, vcc, v3, v4
	v_mul_lo_u32 v6, s13, v3
	s_add_i32 s0, s2, s3
	v_ashrrev_i32_e32 v3, 31, v2
	v_lshlrev_b64 v[3:4], 2, v[2:3]
	v_mov_b32_e32 v2, 0
	v_add_u32_e32 v3, vcc, s4, v3
	v_addc_u32_e32 v4, vcc, v5, v4, vcc
	v_add_u32_e32 v0, vcc, v0, v6
	v_add_u32_e32 v5, vcc, s0, v0
	s_mov_b32 s0, s14
BB13_3:                                 ; %.lr.ph
                                        ; =>This Inner Loop Header: Depth=1
	v_ashrrev_i32_e32 v6, 31, v5
	flat_load_dword v0, v[3:4]
	v_add_u32_e32 v3, vcc, 4, v3
	v_lshlrev_b64 v[6:7], 2, v[5:6]
	v_addc_u32_e32 v4, vcc, 0, v4, vcc
	v_add_u32_e32 v5, vcc, s13, v5
	v_mov_b32_e32 v8, s7
	v_add_u32_e32 v6, vcc, s6, v6
	v_addc_u32_e32 v7, vcc, v8, v7, vcc
	flat_load_dword v6, v[6:7]
	s_add_i32 s0, s0, -1
	s_cmp_eq_u32 s0, 0
	s_waitcnt vmcnt(0) lgkmcnt(0)
	v_mac_f32_e32 v2, v0, v6
	s_cbranch_scc0 BB13_3
BB13_4:                                 ; %Flow22
	v_mov_b32_e32 v0, 0
	v_ashrrev_i64 v[0:1], 30, v[0:1]
	s_waitcnt lgkmcnt(0)
	v_mov_b32_e32 v3, s9
	v_add_u32_e32 v0, vcc, s8, v0
	v_addc_u32_e32 v1, vcc, v3, v1, vcc
	flat_store_dword v[0:1], v2
BB13_5:
	s_endpgm
.Lfunc_end13:
	.size	pf_ff1, .Lfunc_end13-pf_ff1
                                        ; -- End function
	.section	.AMDGPU.csdata
; Kernel info:
; codeLenInByte = 448
; NumSgprs: 18
; NumVgprs: 10
; ScratchSize: 0
; MemoryBound: 0
; FloatMode: 192
; IeeeMode: 1
; LDSByteSize: 0 bytes/workgroup (compile time only)
; SGPRBlocks: 2
; VGPRBlocks: 2
; NumSGPRsForWavesPerEU: 18
; NumVGPRsForWavesPerEU: 10
; Occupancy: 10
; WaveLimiterHint : 1
; COMPUTE_PGM_RSRC2:USER_SGPR: 8
; COMPUTE_PGM_RSRC2:TRAP_HANDLER: 0
; COMPUTE_PGM_RSRC2:TGID_X_EN: 1
; COMPUTE_PGM_RSRC2:TGID_Y_EN: 0
; COMPUTE_PGM_RSRC2:TGID_Z_EN: 0
; COMPUTE_PGM_RSRC2:TIDIG_COMP_CNT: 0
	.text
	.protected	pf_relu         ; -- Begin function pf_relu
	.globl	pf_relu
	.p2align	8
	.type	pf_relu,@function
	.amdgpu_hsa_kernel pf_relu
pf_relu:                                ; @pf_relu
pf_relu$local:
	.amd_kernel_code_t
		amd_code_version_major = 1
		amd_code_version_minor = 2
		amd_machine_kind = 1
		amd_machine_version_major = 8
		amd_machine_version_minor = 0
		amd_machine_version_stepping = 3
		kernel_code_entry_byte_offset = 256
		kernel_code_prefetch_byte_size = 0
		granulated_workitem_vgpr_count = 0
		granulated_wavefront_sgpr_count = 1
		priority = 0
		float_mode = 192
		priv = 0
		enable_dx10_clamp = 1
		debug_mode = 0
		enable_ieee_mode = 1
		enable_wgp_mode = 0
		enable_mem_ordered = 0
		enable_fwd_progress = 0
		enable_sgpr_private_segment_wave_byte_offset = 0
		user_sgpr_count = 8
		enable_trap_handler = 0
		enable_sgpr_workgroup_id_x = 1
		enable_sgpr_workgroup_id_y = 0
		enable_sgpr_workgroup_id_z = 0
		enable_sgpr_workgroup_info = 0
		enable_vgpr_workitem_id = 0
		enable_exception_msb = 0
		granulated_lds_size = 0
		enable_exception = 0
		enable_sgpr_private_segment_buffer = 1
		enable_sgpr_dispatch_ptr = 1
		enable_sgpr_queue_ptr = 0
		enable_sgpr_kernarg_segment_ptr = 1
		enable_sgpr_dispatch_id = 0
		enable_sgpr_flat_scratch_init = 0
		enable_sgpr_private_segment_size = 0
		enable_sgpr_grid_workgroup_count_x = 0
		enable_sgpr_grid_workgroup_count_y = 0
		enable_sgpr_grid_workgroup_count_z = 0
		enable_wavefront_size32 = 0
		enable_ordered_append_gds = 0
		private_element_size = 1
		is_ptr64 = 1
		is_dynamic_callstack = 0
		is_debug_enabled = 0
		is_xnack_enabled = 0
		workitem_private_segment_byte_size = 0
		workgroup_group_segment_byte_size = 0
		gds_segment_byte_size = 0
		kernarg_segment_byte_size = 72
		workgroup_fbarrier_count = 0
		wavefront_sgpr_count = 11
		workitem_vgpr_count = 3
		reserved_vgpr_first = 0
		reserved_vgpr_count = 0
		reserved_sgpr_first = 0
		reserved_sgpr_count = 0
		debug_wavefront_private_segment_offset_sgpr = 0
		debug_private_segment_buffer_sgpr = 0
		kernarg_segment_alignment = 4
		group_segment_alignment = 4
		private_segment_alignment = 4
		wavefront_size = 6
		call_convention = -1
		runtime_loader_kernel_symbol = 0
	.end_amd_kernel_code_t
; %bb.0:
	s_load_dword s0, s[4:5], 0x4
	s_waitcnt lgkmcnt(0)
	s_and_b32 s0, s0, 0xffff
	s_mul_i32 s8, s8, s0
	s_load_dword s2, s[6:7], 0x8
	s_load_dwordx2 s[0:1], s[6:7], 0x10
	v_add_u32_e32 v0, vcc, s8, v0
	s_waitcnt lgkmcnt(0)
	v_add_u32_e32 v0, vcc, s0, v0
	v_cmp_gt_i32_e32 vcc, s2, v0
	s_and_saveexec_b64 s[0:1], vcc
	s_cbranch_execz BB14_3
; %bb.1:
	s_load_dwordx2 s[0:1], s[6:7], 0x0
	v_mov_b32_e32 v1, 0
	v_mov_b32_e32 v2, v0
	v_ashrrev_i64 v[0:1], 30, v[1:2]
	s_waitcnt lgkmcnt(0)
	v_mov_b32_e32 v2, s1
	v_add_u32_e32 v0, vcc, s0, v0
	v_addc_u32_e32 v1, vcc, v2, v1, vcc
	flat_load_dword v2, v[0:1]
	s_waitcnt vmcnt(0) lgkmcnt(0)
	v_cmp_gt_f32_e32 vcc, 0, v2
	s_and_b64 exec, exec, vcc
	s_cbranch_execz BB14_3
; %bb.2:
	v_mov_b32_e32 v2, 0
	flat_store_dword v[0:1], v2
BB14_3:
	s_endpgm
.Lfunc_end14:
	.size	pf_relu, .Lfunc_end14-pf_relu
                                        ; -- End function
	.section	.AMDGPU.csdata
; Kernel info:
; codeLenInByte = 144
; NumSgprs: 11
; NumVgprs: 3
; ScratchSize: 0
; MemoryBound: 0
; FloatMode: 192
; IeeeMode: 1
; LDSByteSize: 0 bytes/workgroup (compile time only)
; SGPRBlocks: 1
; VGPRBlocks: 0
; NumSGPRsForWavesPerEU: 11
; NumVGPRsForWavesPerEU: 3
; Occupancy: 10
; WaveLimiterHint : 1
; COMPUTE_PGM_RSRC2:USER_SGPR: 8
; COMPUTE_PGM_RSRC2:TRAP_HANDLER: 0
; COMPUTE_PGM_RSRC2:TGID_X_EN: 1
; COMPUTE_PGM_RSRC2:TGID_Y_EN: 0
; COMPUTE_PGM_RSRC2:TGID_Z_EN: 0
; COMPUTE_PGM_RSRC2:TIDIG_COMP_CNT: 0
	.text
	.protected	pf_ff2          ; -- Begin function pf_ff2
	.globl	pf_ff2
	.p2align	8
	.type	pf_ff2,@function
	.amdgpu_hsa_kernel pf_ff2
pf_ff2:                                 ; @pf_ff2
pf_ff2$local:
	.amd_kernel_code_t
		amd_code_version_major = 1
		amd_code_version_minor = 2
		amd_machine_kind = 1
		amd_machine_version_major = 8
		amd_machine_version_minor = 0
		amd_machine_version_stepping = 3
		kernel_code_entry_byte_offset = 256
		kernel_code_prefetch_byte_size = 0
		granulated_workitem_vgpr_count = 2
		granulated_wavefront_sgpr_count = 2
		priority = 0
		float_mode = 192
		priv = 0
		enable_dx10_clamp = 1
		debug_mode = 0
		enable_ieee_mode = 1
		enable_wgp_mode = 0
		enable_mem_ordered = 0
		enable_fwd_progress = 0
		enable_sgpr_private_segment_wave_byte_offset = 0
		user_sgpr_count = 8
		enable_trap_handler = 0
		enable_sgpr_workgroup_id_x = 1
		enable_sgpr_workgroup_id_y = 0
		enable_sgpr_workgroup_id_z = 0
		enable_sgpr_workgroup_info = 0
		enable_vgpr_workitem_id = 0
		enable_exception_msb = 0
		granulated_lds_size = 0
		enable_exception = 0
		enable_sgpr_private_segment_buffer = 1
		enable_sgpr_dispatch_ptr = 1
		enable_sgpr_queue_ptr = 0
		enable_sgpr_kernarg_segment_ptr = 1
		enable_sgpr_dispatch_id = 0
		enable_sgpr_flat_scratch_init = 0
		enable_sgpr_private_segment_size = 0
		enable_sgpr_grid_workgroup_count_x = 0
		enable_sgpr_grid_workgroup_count_y = 0
		enable_sgpr_grid_workgroup_count_z = 0
		enable_wavefront_size32 = 0
		enable_ordered_append_gds = 0
		private_element_size = 1
		is_ptr64 = 1
		is_dynamic_callstack = 0
		is_debug_enabled = 0
		is_xnack_enabled = 0
		workitem_private_segment_byte_size = 0
		workgroup_group_segment_byte_size = 0
		gds_segment_byte_size = 0
		kernarg_segment_byte_size = 96
		workgroup_fbarrier_count = 0
		wavefront_sgpr_count = 18
		workitem_vgpr_count = 10
		reserved_vgpr_first = 0
		reserved_vgpr_count = 0
		reserved_sgpr_first = 0
		reserved_sgpr_count = 0
		debug_wavefront_private_segment_offset_sgpr = 0
		debug_private_segment_buffer_sgpr = 0
		kernarg_segment_alignment = 4
		group_segment_alignment = 4
		private_segment_alignment = 4
		wavefront_size = 6
		call_convention = -1
		runtime_loader_kernel_symbol = 0
	.end_amd_kernel_code_t
; %bb.0:
	s_load_dwordx4 s[12:15], s[6:7], 0x18
	s_load_dwordx2 s[2:3], s[6:7], 0x28
	s_load_dword s0, s[4:5], 0x4
	s_waitcnt lgkmcnt(0)
	s_and_b32 s0, s0, 0xffff
	s_mul_i32 s3, s8, s0
	v_add_u32_e32 v1, vcc, s3, v0
	v_add_u32_e32 v1, vcc, s2, v1
	s_mul_i32 s0, s13, s12
	v_cmp_gt_i32_e32 vcc, s0, v1
	s_and_saveexec_b64 s[0:1], vcc
	s_cbranch_execz BB15_5
; %bb.1:
	s_load_dwordx4 s[8:11], s[6:7], 0x10
	s_cmp_lt_i32 s14, 1
	v_mov_b32_e32 v2, 0
	s_cbranch_scc1 BB15_4
; %bb.2:                                ; %.lr.ph.preheader
	s_waitcnt lgkmcnt(0)
	s_ashr_i32 s10, s13, 31
	s_add_i32 s0, s13, s10
	s_xor_b32 s11, s0, s10
	v_cvt_f32_u32_e32 v2, s11
	v_ashrrev_i32_e32 v5, 31, v1
	s_load_dwordx4 s[4:7], s[6:7], 0x0
	v_rcp_iflag_f32_e32 v2, v2
	v_mul_f32_e32 v2, 0x4f800000, v2
	v_cvt_u32_f32_e32 v2, v2
	v_mul_lo_u32 v3, v2, s11
	v_mul_hi_u32 v4, v2, s11
	v_sub_u32_e32 v6, vcc, 0, v3
	v_cmp_eq_u32_e64 s[0:1], 0, v4
	v_cndmask_b32_e64 v3, v3, v6, s[0:1]
	v_mul_hi_u32 v3, v3, v2
	v_add_u32_e32 v4, vcc, v1, v5
	v_xor_b32_e32 v4, v4, v5
	v_add_u32_e32 v6, vcc, v3, v2
	v_subrev_u32_e32 v2, vcc, v3, v2
	v_cndmask_b32_e64 v2, v2, v6, s[0:1]
	v_mul_hi_u32 v2, v2, v4
	v_xor_b32_e32 v3, s10, v5
	s_waitcnt lgkmcnt(0)
	v_mov_b32_e32 v5, s5
	v_mul_lo_u32 v6, v2, s11
	v_add_u32_e32 v7, vcc, 1, v2
	v_add_u32_e32 v8, vcc, -1, v2
	v_sub_u32_e32 v9, vcc, v4, v6
	v_cmp_ge_u32_e32 vcc, v4, v6
	v_cmp_le_u32_e64 s[0:1], s11, v9
	s_and_b64 s[0:1], s[0:1], vcc
	v_cndmask_b32_e64 v2, v2, v7, s[0:1]
	v_cndmask_b32_e32 v2, v8, v2, vcc
	v_xor_b32_e32 v4, v2, v3
	v_sub_u32_e32 v2, vcc, v4, v3
	v_mul_lo_u32 v2, s14, v2
	v_sub_u32_e32 v3, vcc, v3, v4
	v_mul_lo_u32 v6, s13, v3
	s_add_i32 s0, s2, s3
	v_ashrrev_i32_e32 v3, 31, v2
	v_lshlrev_b64 v[3:4], 2, v[2:3]
	v_mov_b32_e32 v2, 0
	v_add_u32_e32 v3, vcc, s4, v3
	v_addc_u32_e32 v4, vcc, v5, v4, vcc
	v_add_u32_e32 v0, vcc, v0, v6
	v_add_u32_e32 v5, vcc, s0, v0
	s_mov_b32 s0, s14
BB15_3:                                 ; %.lr.ph
                                        ; =>This Inner Loop Header: Depth=1
	v_ashrrev_i32_e32 v6, 31, v5
	flat_load_dword v0, v[3:4]
	v_add_u32_e32 v3, vcc, 4, v3
	v_lshlrev_b64 v[6:7], 2, v[5:6]
	v_addc_u32_e32 v4, vcc, 0, v4, vcc
	v_add_u32_e32 v5, vcc, s13, v5
	v_mov_b32_e32 v8, s7
	v_add_u32_e32 v6, vcc, s6, v6
	v_addc_u32_e32 v7, vcc, v8, v7, vcc
	flat_load_dword v6, v[6:7]
	s_add_i32 s0, s0, -1
	s_cmp_eq_u32 s0, 0
	s_waitcnt vmcnt(0) lgkmcnt(0)
	v_mac_f32_e32 v2, v0, v6
	s_cbranch_scc0 BB15_3
BB15_4:                                 ; %Flow22
	v_mov_b32_e32 v0, 0
	v_ashrrev_i64 v[0:1], 30, v[0:1]
	s_waitcnt lgkmcnt(0)
	v_mov_b32_e32 v3, s9
	v_add_u32_e32 v0, vcc, s8, v0
	v_addc_u32_e32 v1, vcc, v3, v1, vcc
	flat_store_dword v[0:1], v2
BB15_5:
	s_endpgm
.Lfunc_end15:
	.size	pf_ff2, .Lfunc_end15-pf_ff2
                                        ; -- End function
	.section	.AMDGPU.csdata
; Kernel info:
; codeLenInByte = 448
; NumSgprs: 18
; NumVgprs: 10
; ScratchSize: 0
; MemoryBound: 0
; FloatMode: 192
; IeeeMode: 1
; LDSByteSize: 0 bytes/workgroup (compile time only)
; SGPRBlocks: 2
; VGPRBlocks: 2
; NumSGPRsForWavesPerEU: 18
; NumVGPRsForWavesPerEU: 10
; Occupancy: 10
; WaveLimiterHint : 1
; COMPUTE_PGM_RSRC2:USER_SGPR: 8
; COMPUTE_PGM_RSRC2:TRAP_HANDLER: 0
; COMPUTE_PGM_RSRC2:TGID_X_EN: 1
; COMPUTE_PGM_RSRC2:TGID_Y_EN: 0
; COMPUTE_PGM_RSRC2:TGID_Z_EN: 0
; COMPUTE_PGM_RSRC2:TIDIG_COMP_CNT: 0
	.ident	"clang version 11.0.0 (https://github.com/ROCm/llvm-project.git b98349b12ffa706d0e863a3f1176b20d2a6c438b)"
	.section	".note.GNU-stack"
	.addrsig
	.amd_amdgpu_isa "amdgcn-amd-amdhsa--gfx803"
	.amd_amdgpu_hsa_metadata
---
Version:         [ 1, 0 ]
Kernels:
  - Name:            pf_q_proj
    SymbolName:      'pf_q_proj@kd'
    Language:        OpenCL C
    LanguageVersion: [ 1, 2 ]
    Args:
      - Name:            A
        TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        ValueType:       F32
        AddrSpaceQual:   Global
        AccQual:         Default
        IsConst:         true
      - Name:            B
        TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        ValueType:       F32
        AddrSpaceQual:   Global
        AccQual:         Default
        IsConst:         true
      - Name:            C
        TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        ValueType:       F32
        AddrSpaceQual:   Global
        AccQual:         Default
      - Name:            M
        TypeName:        int
        Size:            4
        Align:           4
        ValueKind:       ByValue
        ValueType:       I32
        AccQual:         Default
      - Name:            N
        TypeName:        int
        Size:            4
        Align:           4
        ValueKind:       ByValue
        ValueType:       I32
        AccQual:         Default
      - Name:            K
        TypeName:        int
        Size:            4
        Align:           4
        ValueKind:       ByValue
        ValueType:       I32
        AccQual:         Default
      - Size:            8
        Align:           8
        ValueKind:       HiddenGlobalOffsetX
        ValueType:       I64
      - Size:            8
        Align:           8
        ValueKind:       HiddenGlobalOffsetY
        ValueType:       I64
      - Size:            8
        Align:           8
        ValueKind:       HiddenGlobalOffsetZ
        ValueType:       I64
      - Size:            8
        Align:           8
        ValueKind:       HiddenNone
        ValueType:       I8
        AddrSpaceQual:   Global
      - Size:            8
        Align:           8
        ValueKind:       HiddenNone
        ValueType:       I8
        AddrSpaceQual:   Global
      - Size:            8
        Align:           8
        ValueKind:       HiddenNone
        ValueType:       I8
        AddrSpaceQual:   Global
      - Size:            8
        Align:           8
        ValueKind:       HiddenMultiGridSyncArg
        ValueType:       I8
        AddrSpaceQual:   Global
    CodeProps:
      KernargSegmentSize: 96
      GroupSegmentFixedSize: 0
      PrivateSegmentFixedSize: 0
      KernargSegmentAlign: 8
      WavefrontSize:   64
      NumSGPRs:        18
      NumVGPRs:        10
      MaxFlatWorkGroupSize: 256
  - Name:            pf_k_proj
    SymbolName:      'pf_k_proj@kd'
    Language:        OpenCL C
    LanguageVersion: [ 1, 2 ]
    Args:
      - Name:            A
        TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        ValueType:       F32
        AddrSpaceQual:   Global
        AccQual:         Default
        IsConst:         true
      - Name:            B
        TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        ValueType:       F32
        AddrSpaceQual:   Global
        AccQual:         Default
        IsConst:         true
      - Name:            C
        TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        ValueType:       F32
        AddrSpaceQual:   Global
        AccQual:         Default
      - Name:            M
        TypeName:        int
        Size:            4
        Align:           4
        ValueKind:       ByValue
        ValueType:       I32
        AccQual:         Default
      - Name:            N
        TypeName:        int
        Size:            4
        Align:           4
        ValueKind:       ByValue
        ValueType:       I32
        AccQual:         Default
      - Name:            K
        TypeName:        int
        Size:            4
        Align:           4
        ValueKind:       ByValue
        ValueType:       I32
        AccQual:         Default
      - Size:            8
        Align:           8
        ValueKind:       HiddenGlobalOffsetX
        ValueType:       I64
      - Size:            8
        Align:           8
        ValueKind:       HiddenGlobalOffsetY
        ValueType:       I64
      - Size:            8
        Align:           8
        ValueKind:       HiddenGlobalOffsetZ
        ValueType:       I64
      - Size:            8
        Align:           8
        ValueKind:       HiddenNone
        ValueType:       I8
        AddrSpaceQual:   Global
      - Size:            8
        Align:           8
        ValueKind:       HiddenNone
        ValueType:       I8
        AddrSpaceQual:   Global
      - Size:            8
        Align:           8
        ValueKind:       HiddenNone
        ValueType:       I8
        AddrSpaceQual:   Global
      - Size:            8
        Align:           8
        ValueKind:       HiddenMultiGridSyncArg
        ValueType:       I8
        AddrSpaceQual:   Global
    CodeProps:
      KernargSegmentSize: 96
      GroupSegmentFixedSize: 0
      PrivateSegmentFixedSize: 0
      KernargSegmentAlign: 8
      WavefrontSize:   64
      NumSGPRs:        18
      NumVGPRs:        10
      MaxFlatWorkGroupSize: 256
  - Name:            pf_v_proj
    SymbolName:      'pf_v_proj@kd'
    Language:        OpenCL C
    LanguageVersion: [ 1, 2 ]
    Args:
      - Name:            A
        TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        ValueType:       F32
        AddrSpaceQual:   Global
        AccQual:         Default
        IsConst:         true
      - Name:            B
        TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        ValueType:       F32
        AddrSpaceQual:   Global
        AccQual:         Default
        IsConst:         true
      - Name:            C
        TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        ValueType:       F32
        AddrSpaceQual:   Global
        AccQual:         Default
      - Name:            M
        TypeName:        int
        Size:            4
        Align:           4
        ValueKind:       ByValue
        ValueType:       I32
        AccQual:         Default
      - Name:            N
        TypeName:        int
        Size:            4
        Align:           4
        ValueKind:       ByValue
        ValueType:       I32
        AccQual:         Default
      - Name:            K
        TypeName:        int
        Size:            4
        Align:           4
        ValueKind:       ByValue
        ValueType:       I32
        AccQual:         Default
      - Size:            8
        Align:           8
        ValueKind:       HiddenGlobalOffsetX
        ValueType:       I64
      - Size:            8
        Align:           8
        ValueKind:       HiddenGlobalOffsetY
        ValueType:       I64
      - Size:            8
        Align:           8
        ValueKind:       HiddenGlobalOffsetZ
        ValueType:       I64
      - Size:            8
        Align:           8
        ValueKind:       HiddenNone
        ValueType:       I8
        AddrSpaceQual:   Global
      - Size:            8
        Align:           8
        ValueKind:       HiddenNone
        ValueType:       I8
        AddrSpaceQual:   Global
      - Size:            8
        Align:           8
        ValueKind:       HiddenNone
        ValueType:       I8
        AddrSpaceQual:   Global
      - Size:            8
        Align:           8
        ValueKind:       HiddenMultiGridSyncArg
        ValueType:       I8
        AddrSpaceQual:   Global
    CodeProps:
      KernargSegmentSize: 96
      GroupSegmentFixedSize: 0
      PrivateSegmentFixedSize: 0
      KernargSegmentAlign: 8
      WavefrontSize:   64
      NumSGPRs:        18
      NumVGPRs:        10
      MaxFlatWorkGroupSize: 256
  - Name:            pf_phi_q
    SymbolName:      'pf_phi_q@kd'
    Language:        OpenCL C
    LanguageVersion: [ 1, 2 ]
    Args:
      - Name:            In
        TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        ValueType:       F32
        AddrSpaceQual:   Global
        AccQual:         Default
        IsConst:         true
      - Name:            Out
        TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        ValueType:       F32
        AddrSpaceQual:   Global
        AccQual:         Default
      - Name:            N
        TypeName:        int
        Size:            4
        Align:           4
        ValueKind:       ByValue
        ValueType:       I32
        AccQual:         Default
      - Size:            8
        Align:           8
        ValueKind:       HiddenGlobalOffsetX
        ValueType:       I64
      - Size:            8
        Align:           8
        ValueKind:       HiddenGlobalOffsetY
        ValueType:       I64
      - Size:            8
        Align:           8
        ValueKind:       HiddenGlobalOffsetZ
        ValueType:       I64
      - Size:            8
        Align:           8
        ValueKind:       HiddenNone
        ValueType:       I8
        AddrSpaceQual:   Global
      - Size:            8
        Align:           8
        ValueKind:       HiddenNone
        ValueType:       I8
        AddrSpaceQual:   Global
      - Size:            8
        Align:           8
        ValueKind:       HiddenNone
        ValueType:       I8
        AddrSpaceQual:   Global
      - Size:            8
        Align:           8
        ValueKind:       HiddenMultiGridSyncArg
        ValueType:       I8
        AddrSpaceQual:   Global
    CodeProps:
      KernargSegmentSize: 80
      GroupSegmentFixedSize: 0
      PrivateSegmentFixedSize: 0
      KernargSegmentAlign: 8
      WavefrontSize:   64
      NumSGPRs:        11
      NumVGPRs:        6
      MaxFlatWorkGroupSize: 256
  - Name:            pf_phi_k
    SymbolName:      'pf_phi_k@kd'
    Language:        OpenCL C
    LanguageVersion: [ 1, 2 ]
    Args:
      - Name:            In
        TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        ValueType:       F32
        AddrSpaceQual:   Global
        AccQual:         Default
        IsConst:         true
      - Name:            Out
        TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        ValueType:       F32
        AddrSpaceQual:   Global
        AccQual:         Default
      - Name:            N
        TypeName:        int
        Size:            4
        Align:           4
        ValueKind:       ByValue
        ValueType:       I32
        AccQual:         Default
      - Size:            8
        Align:           8
        ValueKind:       HiddenGlobalOffsetX
        ValueType:       I64
      - Size:            8
        Align:           8
        ValueKind:       HiddenGlobalOffsetY
        ValueType:       I64
      - Size:            8
        Align:           8
        ValueKind:       HiddenGlobalOffsetZ
        ValueType:       I64
      - Size:            8
        Align:           8
        ValueKind:       HiddenNone
        ValueType:       I8
        AddrSpaceQual:   Global
      - Size:            8
        Align:           8
        ValueKind:       HiddenNone
        ValueType:       I8
        AddrSpaceQual:   Global
      - Size:            8
        Align:           8
        ValueKind:       HiddenNone
        ValueType:       I8
        AddrSpaceQual:   Global
      - Size:            8
        Align:           8
        ValueKind:       HiddenMultiGridSyncArg
        ValueType:       I8
        AddrSpaceQual:   Global
    CodeProps:
      KernargSegmentSize: 80
      GroupSegmentFixedSize: 0
      PrivateSegmentFixedSize: 0
      KernargSegmentAlign: 8
      WavefrontSize:   64
      NumSGPRs:        11
      NumVGPRs:        6
      MaxFlatWorkGroupSize: 256
  - Name:            pf_kv_reduce
    SymbolName:      'pf_kv_reduce@kd'
    Language:        OpenCL C
    LanguageVersion: [ 1, 2 ]
    Args:
      - Name:            PhiK
        TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        ValueType:       F32
        AddrSpaceQual:   Global
        AccQual:         Default
        IsConst:         true
      - Name:            V
        TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        ValueType:       F32
        AddrSpaceQual:   Global
        AccQual:         Default
        IsConst:         true
      - Name:            KV
        TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        ValueType:       F32
        AddrSpaceQual:   Global
        AccQual:         Default
      - Name:            SeqLen
        TypeName:        int
        Size:            4
        Align:           4
        ValueKind:       ByValue
        ValueType:       I32
        AccQual:         Default
      - Name:            DHead
        TypeName:        int
        Size:            4
        Align:           4
        ValueKind:       ByValue
        ValueType:       I32
        AccQual:         Default
      - Name:            NumHeads
        TypeName:        int
        Size:            4
        Align:           4
        ValueKind:       ByValue
        ValueType:       I32
        AccQual:         Default
      - Size:            8
        Align:           8
        ValueKind:       HiddenGlobalOffsetX
        ValueType:       I64
      - Size:            8
        Align:           8
        ValueKind:       HiddenGlobalOffsetY
        ValueType:       I64
      - Size:            8
        Align:           8
        ValueKind:       HiddenGlobalOffsetZ
        ValueType:       I64
      - Size:            8
        Align:           8
        ValueKind:       HiddenNone
        ValueType:       I8
        AddrSpaceQual:   Global
      - Size:            8
        Align:           8
        ValueKind:       HiddenNone
        ValueType:       I8
        AddrSpaceQual:   Global
      - Size:            8
        Align:           8
        ValueKind:       HiddenNone
        ValueType:       I8
        AddrSpaceQual:   Global
      - Size:            8
        Align:           8
        ValueKind:       HiddenMultiGridSyncArg
        ValueType:       I8
        AddrSpaceQual:   Global
    CodeProps:
      KernargSegmentSize: 96
      GroupSegmentFixedSize: 0
      PrivateSegmentFixedSize: 0
      KernargSegmentAlign: 8
      WavefrontSize:   64
      NumSGPRs:        26
      NumVGPRs:        11
      MaxFlatWorkGroupSize: 256
  - Name:            pf_z_reduce
    SymbolName:      'pf_z_reduce@kd'
    Language:        OpenCL C
    LanguageVersion: [ 1, 2 ]
    Args:
      - Name:            PhiK
        TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        ValueType:       F32
        AddrSpaceQual:   Global
        AccQual:         Default
        IsConst:         true
      - Name:            Z
        TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        ValueType:       F32
        AddrSpaceQual:   Global
        AccQual:         Default
      - Name:            SeqLen
        TypeName:        int
        Size:            4
        Align:           4
        ValueKind:       ByValue
        ValueType:       I32
        AccQual:         Default
      - Name:            DHead
        TypeName:        int
        Size:            4
        Align:           4
        ValueKind:       ByValue
        ValueType:       I32
        AccQual:         Default
      - Name:            NumHeads
        TypeName:        int
        Size:            4
        Align:           4
        ValueKind:       ByValue
        ValueType:       I32
        AccQual:         Default
      - Size:            8
        Align:           8
        ValueKind:       HiddenGlobalOffsetX
        ValueType:       I64
      - Size:            8
        Align:           8
        ValueKind:       HiddenGlobalOffsetY
        ValueType:       I64
      - Size:            8
        Align:           8
        ValueKind:       HiddenGlobalOffsetZ
        ValueType:       I64
      - Size:            8
        Align:           8
        ValueKind:       HiddenNone
        ValueType:       I8
        AddrSpaceQual:   Global
      - Size:            8
        Align:           8
        ValueKind:       HiddenNone
        ValueType:       I8
        AddrSpaceQual:   Global
      - Size:            8
        Align:           8
        ValueKind:       HiddenNone
        ValueType:       I8
        AddrSpaceQual:   Global
      - Size:            8
        Align:           8
        ValueKind:       HiddenMultiGridSyncArg
        ValueType:       I8
        AddrSpaceQual:   Global
    CodeProps:
      KernargSegmentSize: 88
      GroupSegmentFixedSize: 0
      PrivateSegmentFixedSize: 0
      KernargSegmentAlign: 8
      WavefrontSize:   64
      NumSGPRs:        18
      NumVGPRs:        6
      MaxFlatWorkGroupSize: 256
  - Name:            pf_linear_context
    SymbolName:      'pf_linear_context@kd'
    Language:        OpenCL C
    LanguageVersion: [ 1, 2 ]
    Args:
      - Name:            PhiQ
        TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        ValueType:       F32
        AddrSpaceQual:   Global
        AccQual:         Default
        IsConst:         true
      - Name:            KV
        TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        ValueType:       F32
        AddrSpaceQual:   Global
        AccQual:         Default
        IsConst:         true
      - Name:            Z
        TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        ValueType:       F32
        AddrSpaceQual:   Global
        AccQual:         Default
        IsConst:         true
      - Name:            Context
        TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        ValueType:       F32
        AddrSpaceQual:   Global
        AccQual:         Default
      - Name:            SeqLen
        TypeName:        int
        Size:            4
        Align:           4
        ValueKind:       ByValue
        ValueType:       I32
        AccQual:         Default
      - Name:            DHead
        TypeName:        int
        Size:            4
        Align:           4
        ValueKind:       ByValue
        ValueType:       I32
        AccQual:         Default
      - Name:            NumHeads
        TypeName:        int
        Size:            4
        Align:           4
        ValueKind:       ByValue
        ValueType:       I32
        AccQual:         Default
      - Size:            8
        Align:           8
        ValueKind:       HiddenGlobalOffsetX
        ValueType:       I64
      - Size:            8
        Align:           8
        ValueKind:       HiddenGlobalOffsetY
        ValueType:       I64
      - Size:            8
        Align:           8
        ValueKind:       HiddenGlobalOffsetZ
        ValueType:       I64
      - Size:            8
        Align:           8
        ValueKind:       HiddenNone
        ValueType:       I8
        AddrSpaceQual:   Global
      - Size:            8
        Align:           8
        ValueKind:       HiddenNone
        ValueType:       I8
        AddrSpaceQual:   Global
      - Size:            8
        Align:           8
        ValueKind:       HiddenNone
        ValueType:       I8
        AddrSpaceQual:   Global
      - Size:            8
        Align:           8
        ValueKind:       HiddenMultiGridSyncArg
        ValueType:       I8
        AddrSpaceQual:   Global
    CodeProps:
      KernargSegmentSize: 104
      GroupSegmentFixedSize: 0
      PrivateSegmentFixedSize: 0
      KernargSegmentAlign: 8
      WavefrontSize:   64
      NumSGPRs:        20
      NumVGPRs:        11
      MaxFlatWorkGroupSize: 256
  - Name:            pf_o_proj
    SymbolName:      'pf_o_proj@kd'
    Language:        OpenCL C
    LanguageVersion: [ 1, 2 ]
    Args:
      - Name:            A
        TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        ValueType:       F32
        AddrSpaceQual:   Global
        AccQual:         Default
        IsConst:         true
      - Name:            B
        TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        ValueType:       F32
        AddrSpaceQual:   Global
        AccQual:         Default
        IsConst:         true
      - Name:            C
        TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        ValueType:       F32
        AddrSpaceQual:   Global
        AccQual:         Default
      - Name:            M
        TypeName:        int
        Size:            4
        Align:           4
        ValueKind:       ByValue
        ValueType:       I32
        AccQual:         Default
      - Name:            N
        TypeName:        int
        Size:            4
        Align:           4
        ValueKind:       ByValue
        ValueType:       I32
        AccQual:         Default
      - Name:            K
        TypeName:        int
        Size:            4
        Align:           4
        ValueKind:       ByValue
        ValueType:       I32
        AccQual:         Default
      - Size:            8
        Align:           8
        ValueKind:       HiddenGlobalOffsetX
        ValueType:       I64
      - Size:            8
        Align:           8
        ValueKind:       HiddenGlobalOffsetY
        ValueType:       I64
      - Size:            8
        Align:           8
        ValueKind:       HiddenGlobalOffsetZ
        ValueType:       I64
      - Size:            8
        Align:           8
        ValueKind:       HiddenNone
        ValueType:       I8
        AddrSpaceQual:   Global
      - Size:            8
        Align:           8
        ValueKind:       HiddenNone
        ValueType:       I8
        AddrSpaceQual:   Global
      - Size:            8
        Align:           8
        ValueKind:       HiddenNone
        ValueType:       I8
        AddrSpaceQual:   Global
      - Size:            8
        Align:           8
        ValueKind:       HiddenMultiGridSyncArg
        ValueType:       I8
        AddrSpaceQual:   Global
    CodeProps:
      KernargSegmentSize: 96
      GroupSegmentFixedSize: 0
      PrivateSegmentFixedSize: 0
      KernargSegmentAlign: 8
      WavefrontSize:   64
      NumSGPRs:        18
      NumVGPRs:        10
      MaxFlatWorkGroupSize: 256
  - Name:            pf_add_resid1
    SymbolName:      'pf_add_resid1@kd'
    Language:        OpenCL C
    LanguageVersion: [ 1, 2 ]
    Args:
      - Name:            A
        TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        ValueType:       F32
        AddrSpaceQual:   Global
        AccQual:         Default
        IsConst:         true
      - Name:            B
        TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        ValueType:       F32
        AddrSpaceQual:   Global
        AccQual:         Default
        IsConst:         true
      - Name:            Out
        TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        ValueType:       F32
        AddrSpaceQual:   Global
        AccQual:         Default
      - Name:            N
        TypeName:        int
        Size:            4
        Align:           4
        ValueKind:       ByValue
        ValueType:       I32
        AccQual:         Default
      - Size:            8
        Align:           8
        ValueKind:       HiddenGlobalOffsetX
        ValueType:       I64
      - Size:            8
        Align:           8
        ValueKind:       HiddenGlobalOffsetY
        ValueType:       I64
      - Size:            8
        Align:           8
        ValueKind:       HiddenGlobalOffsetZ
        ValueType:       I64
      - Size:            8
        Align:           8
        ValueKind:       HiddenNone
        ValueType:       I8
        AddrSpaceQual:   Global
      - Size:            8
        Align:           8
        ValueKind:       HiddenNone
        ValueType:       I8
        AddrSpaceQual:   Global
      - Size:            8
        Align:           8
        ValueKind:       HiddenNone
        ValueType:       I8
        AddrSpaceQual:   Global
      - Size:            8
        Align:           8
        ValueKind:       HiddenMultiGridSyncArg
        ValueType:       I8
        AddrSpaceQual:   Global
    CodeProps:
      KernargSegmentSize: 88
      GroupSegmentFixedSize: 0
      PrivateSegmentFixedSize: 0
      KernargSegmentAlign: 8
      WavefrontSize:   64
      NumSGPRs:        11
      NumVGPRs:        6
      MaxFlatWorkGroupSize: 256
  - Name:            pf_add_resid2
    SymbolName:      'pf_add_resid2@kd'
    Language:        OpenCL C
    LanguageVersion: [ 1, 2 ]
    Args:
      - Name:            A
        TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        ValueType:       F32
        AddrSpaceQual:   Global
        AccQual:         Default
        IsConst:         true
      - Name:            B
        TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        ValueType:       F32
        AddrSpaceQual:   Global
        AccQual:         Default
        IsConst:         true
      - Name:            Out
        TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        ValueType:       F32
        AddrSpaceQual:   Global
        AccQual:         Default
      - Name:            N
        TypeName:        int
        Size:            4
        Align:           4
        ValueKind:       ByValue
        ValueType:       I32
        AccQual:         Default
      - Size:            8
        Align:           8
        ValueKind:       HiddenGlobalOffsetX
        ValueType:       I64
      - Size:            8
        Align:           8
        ValueKind:       HiddenGlobalOffsetY
        ValueType:       I64
      - Size:            8
        Align:           8
        ValueKind:       HiddenGlobalOffsetZ
        ValueType:       I64
      - Size:            8
        Align:           8
        ValueKind:       HiddenNone
        ValueType:       I8
        AddrSpaceQual:   Global
      - Size:            8
        Align:           8
        ValueKind:       HiddenNone
        ValueType:       I8
        AddrSpaceQual:   Global
      - Size:            8
        Align:           8
        ValueKind:       HiddenNone
        ValueType:       I8
        AddrSpaceQual:   Global
      - Size:            8
        Align:           8
        ValueKind:       HiddenMultiGridSyncArg
        ValueType:       I8
        AddrSpaceQual:   Global
    CodeProps:
      KernargSegmentSize: 88
      GroupSegmentFixedSize: 0
      PrivateSegmentFixedSize: 0
      KernargSegmentAlign: 8
      WavefrontSize:   64
      NumSGPRs:        11
      NumVGPRs:        6
      MaxFlatWorkGroupSize: 256
  - Name:            pf_layernorm1
    SymbolName:      'pf_layernorm1@kd'
    Language:        OpenCL C
    LanguageVersion: [ 1, 2 ]
    Args:
      - Name:            In
        TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        ValueType:       F32
        AddrSpaceQual:   Global
        AccQual:         Default
        IsConst:         true
      - Name:            Gamma
        TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        ValueType:       F32
        AddrSpaceQual:   Global
        AccQual:         Default
        IsConst:         true
      - Name:            Beta
        TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        ValueType:       F32
        AddrSpaceQual:   Global
        AccQual:         Default
        IsConst:         true
      - Name:            Out
        TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        ValueType:       F32
        AddrSpaceQual:   Global
        AccQual:         Default
      - Name:            SeqLen
        TypeName:        int
        Size:            4
        Align:           4
        ValueKind:       ByValue
        ValueType:       I32
        AccQual:         Default
      - Name:            DModel
        TypeName:        int
        Size:            4
        Align:           4
        ValueKind:       ByValue
        ValueType:       I32
        AccQual:         Default
      - Name:            Eps
        TypeName:        float
        Size:            4
        Align:           4
        ValueKind:       ByValue
        ValueType:       F32
        AccQual:         Default
      - Size:            8
        Align:           8
        ValueKind:       HiddenGlobalOffsetX
        ValueType:       I64
      - Size:            8
        Align:           8
        ValueKind:       HiddenGlobalOffsetY
        ValueType:       I64
      - Size:            8
        Align:           8
        ValueKind:       HiddenGlobalOffsetZ
        ValueType:       I64
      - Size:            8
        Align:           8
        ValueKind:       HiddenNone
        ValueType:       I8
        AddrSpaceQual:   Global
      - Size:            8
        Align:           8
        ValueKind:       HiddenNone
        ValueType:       I8
        AddrSpaceQual:   Global
      - Size:            8
        Align:           8
        ValueKind:       HiddenNone
        ValueType:       I8
        AddrSpaceQual:   Global
      - Size:            8
        Align:           8
        ValueKind:       HiddenMultiGridSyncArg
        ValueType:       I8
        AddrSpaceQual:   Global
    CodeProps:
      KernargSegmentSize: 104
      GroupSegmentFixedSize: 0
      PrivateSegmentFixedSize: 0
      KernargSegmentAlign: 8
      WavefrontSize:   64
      NumSGPRs:        14
      NumVGPRs:        11
      MaxFlatWorkGroupSize: 256
  - Name:            pf_layernorm2
    SymbolName:      'pf_layernorm2@kd'
    Language:        OpenCL C
    LanguageVersion: [ 1, 2 ]
    Args:
      - Name:            In
        TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        ValueType:       F32
        AddrSpaceQual:   Global
        AccQual:         Default
        IsConst:         true
      - Name:            Gamma
        TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        ValueType:       F32
        AddrSpaceQual:   Global
        AccQual:         Default
        IsConst:         true
      - Name:            Beta
        TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        ValueType:       F32
        AddrSpaceQual:   Global
        AccQual:         Default
        IsConst:         true
      - Name:            Out
        TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        ValueType:       F32
        AddrSpaceQual:   Global
        AccQual:         Default
      - Name:            SeqLen
        TypeName:        int
        Size:            4
        Align:           4
        ValueKind:       ByValue
        ValueType:       I32
        AccQual:         Default
      - Name:            DModel
        TypeName:        int
        Size:            4
        Align:           4
        ValueKind:       ByValue
        ValueType:       I32
        AccQual:         Default
      - Name:            Eps
        TypeName:        float
        Size:            4
        Align:           4
        ValueKind:       ByValue
        ValueType:       F32
        AccQual:         Default
      - Size:            8
        Align:           8
        ValueKind:       HiddenGlobalOffsetX
        ValueType:       I64
      - Size:            8
        Align:           8
        ValueKind:       HiddenGlobalOffsetY
        ValueType:       I64
      - Size:            8
        Align:           8
        ValueKind:       HiddenGlobalOffsetZ
        ValueType:       I64
      - Size:            8
        Align:           8
        ValueKind:       HiddenNone
        ValueType:       I8
        AddrSpaceQual:   Global
      - Size:            8
        Align:           8
        ValueKind:       HiddenNone
        ValueType:       I8
        AddrSpaceQual:   Global
      - Size:            8
        Align:           8
        ValueKind:       HiddenNone
        ValueType:       I8
        AddrSpaceQual:   Global
      - Size:            8
        Align:           8
        ValueKind:       HiddenMultiGridSyncArg
        ValueType:       I8
        AddrSpaceQual:   Global
    CodeProps:
      KernargSegmentSize: 104
      GroupSegmentFixedSize: 0
      PrivateSegmentFixedSize: 0
      KernargSegmentAlign: 8
      WavefrontSize:   64
      NumSGPRs:        14
      NumVGPRs:        11
      MaxFlatWorkGroupSize: 256
  - Name:            pf_ff1
    SymbolName:      'pf_ff1@kd'
    Language:        OpenCL C
    LanguageVersion: [ 1, 2 ]
    Args:
      - Name:            A
        TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        ValueType:       F32
        AddrSpaceQual:   Global
        AccQual:         Default
        IsConst:         true
      - Name:            B
        TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        ValueType:       F32
        AddrSpaceQual:   Global
        AccQual:         Default
        IsConst:         true
      - Name:            C
        TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        ValueType:       F32
        AddrSpaceQual:   Global
        AccQual:         Default
      - Name:            M
        TypeName:        int
        Size:            4
        Align:           4
        ValueKind:       ByValue
        ValueType:       I32
        AccQual:         Default
      - Name:            N
        TypeName:        int
        Size:            4
        Align:           4
        ValueKind:       ByValue
        ValueType:       I32
        AccQual:         Default
      - Name:            K
        TypeName:        int
        Size:            4
        Align:           4
        ValueKind:       ByValue
        ValueType:       I32
        AccQual:         Default
      - Size:            8
        Align:           8
        ValueKind:       HiddenGlobalOffsetX
        ValueType:       I64
      - Size:            8
        Align:           8
        ValueKind:       HiddenGlobalOffsetY
        ValueType:       I64
      - Size:            8
        Align:           8
        ValueKind:       HiddenGlobalOffsetZ
        ValueType:       I64
      - Size:            8
        Align:           8
        ValueKind:       HiddenNone
        ValueType:       I8
        AddrSpaceQual:   Global
      - Size:            8
        Align:           8
        ValueKind:       HiddenNone
        ValueType:       I8
        AddrSpaceQual:   Global
      - Size:            8
        Align:           8
        ValueKind:       HiddenNone
        ValueType:       I8
        AddrSpaceQual:   Global
      - Size:            8
        Align:           8
        ValueKind:       HiddenMultiGridSyncArg
        ValueType:       I8
        AddrSpaceQual:   Global
    CodeProps:
      KernargSegmentSize: 96
      GroupSegmentFixedSize: 0
      PrivateSegmentFixedSize: 0
      KernargSegmentAlign: 8
      WavefrontSize:   64
      NumSGPRs:        18
      NumVGPRs:        10
      MaxFlatWorkGroupSize: 256
  - Name:            pf_relu
    SymbolName:      'pf_relu@kd'
    Language:        OpenCL C
    LanguageVersion: [ 1, 2 ]
    Args:
      - Name:            X
        TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        ValueType:       F32
        AddrSpaceQual:   Global
        AccQual:         Default
      - Name:            N
        TypeName:        int
        Size:            4
        Align:           4
        ValueKind:       ByValue
        ValueType:       I32
        AccQual:         Default
      - Size:            8
        Align:           8
        ValueKind:       HiddenGlobalOffsetX
        ValueType:       I64
      - Size:            8
        Align:           8
        ValueKind:       HiddenGlobalOffsetY
        ValueType:       I64
      - Size:            8
        Align:           8
        ValueKind:       HiddenGlobalOffsetZ
        ValueType:       I64
      - Size:            8
        Align:           8
        ValueKind:       HiddenNone
        ValueType:       I8
        AddrSpaceQual:   Global
      - Size:            8
        Align:           8
        ValueKind:       HiddenNone
        ValueType:       I8
        AddrSpaceQual:   Global
      - Size:            8
        Align:           8
        ValueKind:       HiddenNone
        ValueType:       I8
        AddrSpaceQual:   Global
      - Size:            8
        Align:           8
        ValueKind:       HiddenMultiGridSyncArg
        ValueType:       I8
        AddrSpaceQual:   Global
    CodeProps:
      KernargSegmentSize: 72
      GroupSegmentFixedSize: 0
      PrivateSegmentFixedSize: 0
      KernargSegmentAlign: 8
      WavefrontSize:   64
      NumSGPRs:        11
      NumVGPRs:        3
      MaxFlatWorkGroupSize: 256
  - Name:            pf_ff2
    SymbolName:      'pf_ff2@kd'
    Language:        OpenCL C
    LanguageVersion: [ 1, 2 ]
    Args:
      - Name:            A
        TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        ValueType:       F32
        AddrSpaceQual:   Global
        AccQual:         Default
        IsConst:         true
      - Name:            B
        TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        ValueType:       F32
        AddrSpaceQual:   Global
        AccQual:         Default
        IsConst:         true
      - Name:            C
        TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        ValueType:       F32
        AddrSpaceQual:   Global
        AccQual:         Default
      - Name:            M
        TypeName:        int
        Size:            4
        Align:           4
        ValueKind:       ByValue
        ValueType:       I32
        AccQual:         Default
      - Name:            N
        TypeName:        int
        Size:            4
        Align:           4
        ValueKind:       ByValue
        ValueType:       I32
        AccQual:         Default
      - Name:            K
        TypeName:        int
        Size:            4
        Align:           4
        ValueKind:       ByValue
        ValueType:       I32
        AccQual:         Default
      - Size:            8
        Align:           8
        ValueKind:       HiddenGlobalOffsetX
        ValueType:       I64
      - Size:            8
        Align:           8
        ValueKind:       HiddenGlobalOffsetY
        ValueType:       I64
      - Size:            8
        Align:           8
        ValueKind:       HiddenGlobalOffsetZ
        ValueType:       I64
      - Size:            8
        Align:           8
        ValueKind:       HiddenNone
        ValueType:       I8
        AddrSpaceQual:   Global
      - Size:            8
        Align:           8
        ValueKind:       HiddenNone
        ValueType:       I8
        AddrSpaceQual:   Global
      - Size:            8
        Align:           8
        ValueKind:       HiddenNone
        ValueType:       I8
        AddrSpaceQual:   Global
      - Size:            8
        Align:           8
        ValueKind:       HiddenMultiGridSyncArg
        ValueType:       I8
        AddrSpaceQual:   Global
    CodeProps:
      KernargSegmentSize: 96
      GroupSegmentFixedSize: 0
      PrivateSegmentFixedSize: 0
      KernargSegmentAlign: 8
      WavefrontSize:   64
      NumSGPRs:        18
      NumVGPRs:        10
      MaxFlatWorkGroupSize: 256
...

	.end_amd_amdgpu_hsa_metadata
