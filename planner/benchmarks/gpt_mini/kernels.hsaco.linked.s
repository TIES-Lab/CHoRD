	.text
	.hsa_code_object_version 2,1
	.hsa_code_object_isa 8,0,3,"AMD","AMDGPU"
	.protected	gm_q_proj       ; -- Begin function gm_q_proj
	.globl	gm_q_proj
	.p2align	8
	.type	gm_q_proj,@function
	.amdgpu_hsa_kernel gm_q_proj
gm_q_proj:                              ; @gm_q_proj
gm_q_proj$local:
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
		enable_sgpr_workgroup_id_y = 1
		enable_sgpr_workgroup_id_z = 0
		enable_sgpr_workgroup_info = 0
		enable_vgpr_workitem_id = 1
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
	s_load_dwordx4 s[0:3], s[6:7], 0x28
	s_waitcnt lgkmcnt(0)
	s_load_dword s1, s[4:5], 0x4
	s_waitcnt lgkmcnt(0)
	s_and_b32 s3, s1, 0xffff
	s_mul_i32 s8, s8, s3
	s_add_i32 s0, s0, s8
	v_add_u32_e32 v0, vcc, s0, v0
	s_lshr_b32 s0, s1, 16
	s_mul_i32 s9, s9, s0
	s_add_i32 s0, s2, s9
	v_add_u32_e32 v1, vcc, s0, v1
	v_cmp_gt_i32_e32 vcc, s12, v1
	v_cmp_gt_i32_e64 s[0:1], s13, v0
	s_and_b64 s[0:1], s[0:1], vcc
	s_and_saveexec_b64 s[2:3], s[0:1]
	s_cbranch_execz BB0_5
; %bb.1:
	s_load_dwordx4 s[0:3], s[6:7], 0x10
	s_cmp_lt_i32 s14, 1
	v_mov_b32_e32 v2, 0
	s_cbranch_scc1 BB0_4
; %bb.2:                                ; %.lr.ph.preheader
	v_mul_lo_u32 v2, s14, v1
	s_load_dwordx4 s[4:7], s[6:7], 0x0
	s_waitcnt lgkmcnt(0)
	s_mov_b32 s2, s14
	v_mov_b32_e32 v5, v0
	v_ashrrev_i32_e32 v3, 31, v2
	v_lshlrev_b64 v[3:4], 2, v[2:3]
	v_mov_b32_e32 v2, s5
	v_add_u32_e32 v3, vcc, s4, v3
	v_addc_u32_e32 v4, vcc, v2, v4, vcc
	v_mov_b32_e32 v2, 0
BB0_3:                                  ; %.lr.ph
                                        ; =>This Inner Loop Header: Depth=1
	v_ashrrev_i32_e32 v6, 31, v5
	flat_load_dword v8, v[3:4]
	v_add_u32_e32 v3, vcc, 4, v3
	v_lshlrev_b64 v[6:7], 2, v[5:6]
	v_addc_u32_e32 v4, vcc, 0, v4, vcc
	v_add_u32_e32 v5, vcc, s13, v5
	v_mov_b32_e32 v9, s7
	v_add_u32_e32 v6, vcc, s6, v6
	v_addc_u32_e32 v7, vcc, v9, v7, vcc
	flat_load_dword v6, v[6:7]
	s_add_i32 s2, s2, -1
	s_cmp_eq_u32 s2, 0
	s_waitcnt vmcnt(0) lgkmcnt(0)
	v_mac_f32_e32 v2, v8, v6
	s_cbranch_scc0 BB0_3
BB0_4:                                  ; %Flow24
	v_mul_lo_u32 v1, v1, s13
	s_waitcnt lgkmcnt(0)
	v_mov_b32_e32 v3, s1
	v_add_u32_e32 v0, vcc, v1, v0
	v_ashrrev_i32_e32 v1, 31, v0
	v_lshlrev_b64 v[0:1], 2, v[0:1]
	v_add_u32_e32 v0, vcc, s0, v0
	v_addc_u32_e32 v1, vcc, v3, v1, vcc
	flat_store_dword v[0:1], v2
BB0_5:
	s_endpgm
.Lfunc_end0:
	.size	gm_q_proj, .Lfunc_end0-gm_q_proj
                                        ; -- End function
	.section	.AMDGPU.csdata
; Kernel info:
; codeLenInByte = 292
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
; COMPUTE_PGM_RSRC2:TGID_Y_EN: 1
; COMPUTE_PGM_RSRC2:TGID_Z_EN: 0
; COMPUTE_PGM_RSRC2:TIDIG_COMP_CNT: 1
	.text
	.protected	gm_k_proj       ; -- Begin function gm_k_proj
	.globl	gm_k_proj
	.p2align	8
	.type	gm_k_proj,@function
	.amdgpu_hsa_kernel gm_k_proj
gm_k_proj:                              ; @gm_k_proj
gm_k_proj$local:
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
		enable_sgpr_workgroup_id_y = 1
		enable_sgpr_workgroup_id_z = 0
		enable_sgpr_workgroup_info = 0
		enable_vgpr_workitem_id = 1
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
	s_load_dwordx4 s[0:3], s[6:7], 0x28
	s_waitcnt lgkmcnt(0)
	s_load_dword s1, s[4:5], 0x4
	s_waitcnt lgkmcnt(0)
	s_and_b32 s3, s1, 0xffff
	s_mul_i32 s8, s8, s3
	s_add_i32 s0, s0, s8
	v_add_u32_e32 v0, vcc, s0, v0
	s_lshr_b32 s0, s1, 16
	s_mul_i32 s9, s9, s0
	s_add_i32 s0, s2, s9
	v_add_u32_e32 v1, vcc, s0, v1
	v_cmp_gt_i32_e32 vcc, s12, v1
	v_cmp_gt_i32_e64 s[0:1], s13, v0
	s_and_b64 s[0:1], s[0:1], vcc
	s_and_saveexec_b64 s[2:3], s[0:1]
	s_cbranch_execz BB1_5
; %bb.1:
	s_load_dwordx4 s[0:3], s[6:7], 0x10
	s_cmp_lt_i32 s14, 1
	v_mov_b32_e32 v2, 0
	s_cbranch_scc1 BB1_4
; %bb.2:                                ; %.lr.ph.preheader
	v_mul_lo_u32 v2, s14, v1
	s_load_dwordx4 s[4:7], s[6:7], 0x0
	s_waitcnt lgkmcnt(0)
	s_mov_b32 s2, s14
	v_mov_b32_e32 v5, v0
	v_ashrrev_i32_e32 v3, 31, v2
	v_lshlrev_b64 v[3:4], 2, v[2:3]
	v_mov_b32_e32 v2, s5
	v_add_u32_e32 v3, vcc, s4, v3
	v_addc_u32_e32 v4, vcc, v2, v4, vcc
	v_mov_b32_e32 v2, 0
BB1_3:                                  ; %.lr.ph
                                        ; =>This Inner Loop Header: Depth=1
	v_ashrrev_i32_e32 v6, 31, v5
	flat_load_dword v8, v[3:4]
	v_add_u32_e32 v3, vcc, 4, v3
	v_lshlrev_b64 v[6:7], 2, v[5:6]
	v_addc_u32_e32 v4, vcc, 0, v4, vcc
	v_add_u32_e32 v5, vcc, s13, v5
	v_mov_b32_e32 v9, s7
	v_add_u32_e32 v6, vcc, s6, v6
	v_addc_u32_e32 v7, vcc, v9, v7, vcc
	flat_load_dword v6, v[6:7]
	s_add_i32 s2, s2, -1
	s_cmp_eq_u32 s2, 0
	s_waitcnt vmcnt(0) lgkmcnt(0)
	v_mac_f32_e32 v2, v8, v6
	s_cbranch_scc0 BB1_3
BB1_4:                                  ; %Flow24
	v_mul_lo_u32 v1, v1, s13
	s_waitcnt lgkmcnt(0)
	v_mov_b32_e32 v3, s1
	v_add_u32_e32 v0, vcc, v1, v0
	v_ashrrev_i32_e32 v1, 31, v0
	v_lshlrev_b64 v[0:1], 2, v[0:1]
	v_add_u32_e32 v0, vcc, s0, v0
	v_addc_u32_e32 v1, vcc, v3, v1, vcc
	flat_store_dword v[0:1], v2
BB1_5:
	s_endpgm
.Lfunc_end1:
	.size	gm_k_proj, .Lfunc_end1-gm_k_proj
                                        ; -- End function
	.section	.AMDGPU.csdata
; Kernel info:
; codeLenInByte = 292
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
; COMPUTE_PGM_RSRC2:TGID_Y_EN: 1
; COMPUTE_PGM_RSRC2:TGID_Z_EN: 0
; COMPUTE_PGM_RSRC2:TIDIG_COMP_CNT: 1
	.text
	.protected	gm_v_proj       ; -- Begin function gm_v_proj
	.globl	gm_v_proj
	.p2align	8
	.type	gm_v_proj,@function
	.amdgpu_hsa_kernel gm_v_proj
gm_v_proj:                              ; @gm_v_proj
gm_v_proj$local:
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
		enable_sgpr_workgroup_id_y = 1
		enable_sgpr_workgroup_id_z = 0
		enable_sgpr_workgroup_info = 0
		enable_vgpr_workitem_id = 1
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
	s_load_dwordx4 s[0:3], s[6:7], 0x28
	s_waitcnt lgkmcnt(0)
	s_load_dword s1, s[4:5], 0x4
	s_waitcnt lgkmcnt(0)
	s_and_b32 s3, s1, 0xffff
	s_mul_i32 s8, s8, s3
	s_add_i32 s0, s0, s8
	v_add_u32_e32 v0, vcc, s0, v0
	s_lshr_b32 s0, s1, 16
	s_mul_i32 s9, s9, s0
	s_add_i32 s0, s2, s9
	v_add_u32_e32 v1, vcc, s0, v1
	v_cmp_gt_i32_e32 vcc, s12, v1
	v_cmp_gt_i32_e64 s[0:1], s13, v0
	s_and_b64 s[0:1], s[0:1], vcc
	s_and_saveexec_b64 s[2:3], s[0:1]
	s_cbranch_execz BB2_5
; %bb.1:
	s_load_dwordx4 s[0:3], s[6:7], 0x10
	s_cmp_lt_i32 s14, 1
	v_mov_b32_e32 v2, 0
	s_cbranch_scc1 BB2_4
; %bb.2:                                ; %.lr.ph.preheader
	v_mul_lo_u32 v2, s14, v1
	s_load_dwordx4 s[4:7], s[6:7], 0x0
	s_waitcnt lgkmcnt(0)
	s_mov_b32 s2, s14
	v_mov_b32_e32 v5, v0
	v_ashrrev_i32_e32 v3, 31, v2
	v_lshlrev_b64 v[3:4], 2, v[2:3]
	v_mov_b32_e32 v2, s5
	v_add_u32_e32 v3, vcc, s4, v3
	v_addc_u32_e32 v4, vcc, v2, v4, vcc
	v_mov_b32_e32 v2, 0
BB2_3:                                  ; %.lr.ph
                                        ; =>This Inner Loop Header: Depth=1
	v_ashrrev_i32_e32 v6, 31, v5
	flat_load_dword v8, v[3:4]
	v_add_u32_e32 v3, vcc, 4, v3
	v_lshlrev_b64 v[6:7], 2, v[5:6]
	v_addc_u32_e32 v4, vcc, 0, v4, vcc
	v_add_u32_e32 v5, vcc, s13, v5
	v_mov_b32_e32 v9, s7
	v_add_u32_e32 v6, vcc, s6, v6
	v_addc_u32_e32 v7, vcc, v9, v7, vcc
	flat_load_dword v6, v[6:7]
	s_add_i32 s2, s2, -1
	s_cmp_eq_u32 s2, 0
	s_waitcnt vmcnt(0) lgkmcnt(0)
	v_mac_f32_e32 v2, v8, v6
	s_cbranch_scc0 BB2_3
BB2_4:                                  ; %Flow24
	v_mul_lo_u32 v1, v1, s13
	s_waitcnt lgkmcnt(0)
	v_mov_b32_e32 v3, s1
	v_add_u32_e32 v0, vcc, v1, v0
	v_ashrrev_i32_e32 v1, 31, v0
	v_lshlrev_b64 v[0:1], 2, v[0:1]
	v_add_u32_e32 v0, vcc, s0, v0
	v_addc_u32_e32 v1, vcc, v3, v1, vcc
	flat_store_dword v[0:1], v2
BB2_5:
	s_endpgm
.Lfunc_end2:
	.size	gm_v_proj, .Lfunc_end2-gm_v_proj
                                        ; -- End function
	.section	.AMDGPU.csdata
; Kernel info:
; codeLenInByte = 292
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
; COMPUTE_PGM_RSRC2:TGID_Y_EN: 1
; COMPUTE_PGM_RSRC2:TGID_Z_EN: 0
; COMPUTE_PGM_RSRC2:TIDIG_COMP_CNT: 1
	.text
	.protected	gm_attention_scores_masked ; -- Begin function gm_attention_scores_masked
	.globl	gm_attention_scores_masked
	.p2align	8
	.type	gm_attention_scores_masked,@function
	.amdgpu_hsa_kernel gm_attention_scores_masked
gm_attention_scores_masked:             ; @gm_attention_scores_masked
gm_attention_scores_masked$local:
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
		enable_sgpr_workgroup_id_y = 1
		enable_sgpr_workgroup_id_z = 0
		enable_sgpr_workgroup_info = 0
		enable_vgpr_workitem_id = 1
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
		workitem_vgpr_count = 9
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
	s_load_dwordx4 s[0:3], s[6:7], 0x28
	s_waitcnt lgkmcnt(0)
	s_load_dword s1, s[4:5], 0x4
	s_waitcnt lgkmcnt(0)
	s_and_b32 s3, s1, 0xffff
	s_mul_i32 s8, s8, s3
	s_add_i32 s0, s0, s8
	v_add_u32_e32 v0, vcc, s0, v0
	s_lshr_b32 s0, s1, 16
	s_mul_i32 s9, s9, s0
	s_add_i32 s0, s2, s9
	v_add_u32_e32 v1, vcc, s0, v1
	s_mul_i32 s0, s14, s12
	v_cmp_gt_i32_e32 vcc, s12, v0
	v_cmp_gt_i32_e64 s[0:1], s0, v1
	s_and_b64 s[0:1], vcc, s[0:1]
	s_and_saveexec_b64 s[2:3], s[0:1]
	s_cbranch_execz BB3_7
; %bb.1:
	s_ashr_i32 s2, s12, 31
	s_add_i32 s0, s12, s2
	s_xor_b32 s3, s0, s2
	v_cvt_f32_u32_e32 v2, s3
	v_ashrrev_i32_e32 v5, 31, v1
	v_rcp_iflag_f32_e32 v2, v2
	v_mul_f32_e32 v2, 0x4f800000, v2
	v_cvt_u32_f32_e32 v2, v2
	v_mul_lo_u32 v3, v2, s3
	v_mul_hi_u32 v4, v2, s3
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
	v_xor_b32_e32 v3, s2, v5
	v_mul_lo_u32 v5, v2, s3
	v_add_u32_e32 v6, vcc, 1, v2
	v_add_u32_e32 v7, vcc, -1, v2
	v_sub_u32_e32 v8, vcc, v4, v5
	v_cmp_ge_u32_e32 vcc, v4, v5
	v_cmp_le_u32_e64 s[0:1], s3, v8
	s_and_b64 s[0:1], s[0:1], vcc
	v_cndmask_b32_e64 v2, v2, v6, s[0:1]
	v_cndmask_b32_e32 v2, v7, v2, vcc
	v_xor_b32_e32 v5, v2, v3
	v_sub_u32_e32 v2, vcc, v5, v3
	v_mul_lo_u32 v2, v2, s12
	s_load_dwordx4 s[0:3], s[6:7], 0x10
	v_sub_u32_e32 v4, vcc, v1, v2
	v_cmp_ge_i32_e32 vcc, v4, v0
	v_mov_b32_e32 v2, 0xf149f2ca
	s_waitcnt lgkmcnt(0)
	s_and_saveexec_b64 s[2:3], vcc
	s_cbranch_execz BB3_6
; %bb.2:
	s_cmp_lt_i32 s13, 1
	v_mov_b32_e32 v2, 0
	s_cbranch_scc1 BB3_5
; %bb.3:                                ; %.lr.ph.preheader
	v_mul_lo_u32 v2, s14, v0
	v_mul_lo_u32 v6, s14, v4
	s_load_dwordx4 s[4:7], s[6:7], 0x0
	s_mov_b32 s8, s13
	v_add_u32_e32 v2, vcc, v5, v2
	v_sub_u32_e32 v2, vcc, v2, v3
	v_mul_lo_u32 v4, s13, v2
	v_add_u32_e32 v2, vcc, v5, v6
	v_sub_u32_e32 v2, vcc, v2, v3
	v_mul_lo_u32 v6, s13, v2
	v_mov_b32_e32 v2, 0
BB3_4:                                  ; %.lr.ph
                                        ; =>This Inner Loop Header: Depth=1
	v_ashrrev_i32_e32 v7, 31, v6
	v_lshlrev_b64 v[7:8], 2, v[6:7]
	s_waitcnt lgkmcnt(0)
	v_mov_b32_e32 v3, s5
	v_add_u32_e32 v7, vcc, s4, v7
	v_addc_u32_e32 v8, vcc, v3, v8, vcc
	v_ashrrev_i32_e32 v5, 31, v4
	flat_load_dword v3, v[7:8]
	v_lshlrev_b64 v[7:8], 2, v[4:5]
	v_mov_b32_e32 v5, s7
	v_add_u32_e32 v7, vcc, s6, v7
	v_addc_u32_e32 v8, vcc, v5, v8, vcc
	flat_load_dword v5, v[7:8]
	s_add_i32 s8, s8, -1
	v_add_u32_e32 v4, vcc, 1, v4
	s_cmp_eq_u32 s8, 0
	v_add_u32_e32 v6, vcc, 1, v6
	s_waitcnt vmcnt(0) lgkmcnt(0)
	v_mac_f32_e32 v2, v3, v5
	s_cbranch_scc0 BB3_4
BB3_5:                                  ; %._crit_edge
	v_cvt_f32_i32_e32 v3, s13
	v_rsq_f32_e32 v3, v3
	v_mul_f32_e32 v2, v3, v2
BB3_6:                                  ; %Flow26
	s_or_b64 exec, exec, s[2:3]
	v_mul_lo_u32 v1, v1, s12
	v_mov_b32_e32 v3, s1
	v_add_u32_e32 v0, vcc, v1, v0
	v_ashrrev_i32_e32 v1, 31, v0
	v_lshlrev_b64 v[0:1], 2, v[0:1]
	v_add_u32_e32 v0, vcc, s0, v0
	v_addc_u32_e32 v1, vcc, v3, v1, vcc
	flat_store_dword v[0:1], v2
BB3_7:
	s_endpgm
.Lfunc_end3:
	.size	gm_attention_scores_masked, .Lfunc_end3-gm_attention_scores_masked
                                        ; -- End function
	.section	.AMDGPU.csdata
; Kernel info:
; codeLenInByte = 548
; NumSgprs: 18
; NumVgprs: 9
; ScratchSize: 0
; MemoryBound: 0
; FloatMode: 192
; IeeeMode: 1
; LDSByteSize: 0 bytes/workgroup (compile time only)
; SGPRBlocks: 2
; VGPRBlocks: 2
; NumSGPRsForWavesPerEU: 18
; NumVGPRsForWavesPerEU: 9
; Occupancy: 10
; WaveLimiterHint : 1
; COMPUTE_PGM_RSRC2:USER_SGPR: 8
; COMPUTE_PGM_RSRC2:TRAP_HANDLER: 0
; COMPUTE_PGM_RSRC2:TGID_X_EN: 1
; COMPUTE_PGM_RSRC2:TGID_Y_EN: 1
; COMPUTE_PGM_RSRC2:TGID_Z_EN: 0
; COMPUTE_PGM_RSRC2:TIDIG_COMP_CNT: 1
	.text
	.protected	gm_softmax      ; -- Begin function gm_softmax
	.globl	gm_softmax
	.p2align	8
	.type	gm_softmax,@function
	.amdgpu_hsa_kernel gm_softmax
gm_softmax:                             ; @gm_softmax
gm_softmax$local:
	.amd_kernel_code_t
		amd_code_version_major = 1
		amd_code_version_minor = 2
		amd_machine_kind = 1
		amd_machine_version_major = 8
		amd_machine_version_minor = 0
		amd_machine_version_stepping = 3
		kernel_code_entry_byte_offset = 256
		kernel_code_prefetch_byte_size = 0
		granulated_workitem_vgpr_count = 3
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
		enable_sgpr_workgroup_id_y = 1
		enable_sgpr_workgroup_id_z = 0
		enable_sgpr_workgroup_info = 0
		enable_vgpr_workitem_id = 1
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
		wavefront_sgpr_count = 18
		workitem_vgpr_count = 14
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
	s_load_dwordx2 s[2:3], s[6:7], 0x10
	s_load_dwordx4 s[12:15], s[6:7], 0x18
	s_load_dword s0, s[4:5], 0x4
	s_waitcnt lgkmcnt(0)
	s_and_b32 s1, s0, 0xffff
	s_lshr_b32 s0, s0, 16
	s_mul_i32 s9, s9, s0
	s_mul_i32 s8, s8, s1
	s_add_i32 s4, s12, s8
	s_add_i32 s0, s14, s9
	v_add_u32_e32 v1, vcc, s0, v1
	v_add_u32_e32 v2, vcc, s4, v0
	v_cmp_gt_i32_e32 vcc, s2, v2
	v_cmp_gt_i32_e64 s[0:1], s3, v1
	s_and_b64 s[0:1], vcc, s[0:1]
	s_and_saveexec_b64 s[8:9], s[0:1]
	s_cbranch_execz BB4_8
; %bb.1:
	s_cmp_lt_i32 s2, 1
	s_cbranch_scc1 BB4_8
; %bb.2:                                ; %.lr.ph73.preheader
	v_mul_lo_u32 v1, s2, v1
	s_mov_b32 s0, s2
	v_add_u32_e32 v0, vcc, v0, v1
	v_add_u32_e32 v0, vcc, s4, v0
	v_mul_lo_u32 v3, s2, v0
	s_load_dwordx4 s[4:7], s[6:7], 0x0
	v_ashrrev_i32_e32 v4, 31, v3
	v_lshlrev_b64 v[0:1], 2, v[3:4]
	s_waitcnt lgkmcnt(0)
	v_mov_b32_e32 v2, s5
	v_add_u32_e32 v0, vcc, s4, v0
	v_addc_u32_e32 v1, vcc, v2, v1, vcc
	v_mov_b32_e32 v6, v1
	v_mov_b32_e32 v2, 0xf149f2ca
	v_mov_b32_e32 v5, v0
BB4_3:                                  ; %.lr.ph73
                                        ; =>This Inner Loop Header: Depth=1
	flat_load_dword v7, v[5:6]
	v_add_u32_e32 v5, vcc, 4, v5
	v_addc_u32_e32 v6, vcc, 0, v6, vcc
	s_add_i32 s0, s0, -1
	s_cmp_lg_u32 s0, 0
	s_waitcnt vmcnt(0) lgkmcnt(0)
	v_cmp_gt_f32_e32 vcc, v7, v2
	v_cndmask_b32_e32 v2, v2, v7, vcc
	s_cbranch_scc1 BB4_3
; %bb.4:                                ; %.lr.ph69.preheader
	v_lshlrev_b64 v[3:4], 2, v[3:4]
	v_mov_b32_e32 v5, s7
	v_add_u32_e32 v3, vcc, s6, v3
	v_addc_u32_e32 v4, vcc, v5, v4, vcc
	v_mov_b32_e32 v7, v4
	v_mov_b32_e32 v5, 0
	s_mov_b32 s0, 0x39a3b295
	s_mov_b32 s1, 0x3fb8a000
	s_mov_b32 s3, 0xc2aeac50
	s_mov_b32 s4, 0x42b17218
	s_mov_b32 s5, s2
	v_mov_b32_e32 v6, v3
	v_mov_b32_e32 v9, 0x7f800000
BB4_5:                                  ; %.lr.ph69
                                        ; =>This Inner Loop Header: Depth=1
	flat_load_dword v8, v[0:1]
	v_add_u32_e32 v0, vcc, 4, v0
	v_addc_u32_e32 v1, vcc, 0, v1, vcc
	s_add_i32 s5, s5, -1
	s_cmp_eq_u32 s5, 0
	s_waitcnt vmcnt(0) lgkmcnt(0)
	v_sub_f32_e32 v8, v8, v2
	v_and_b32_e32 v10, 0xfffff000, v8
	v_sub_f32_e32 v11, v8, v10
	v_mul_f32_e32 v13, s0, v11
	v_mac_f32_e32 v13, s1, v11
	v_mul_f32_e32 v12, s1, v10
	v_mac_f32_e32 v13, s0, v10
	v_exp_f32_e32 v12, v12
	v_exp_f32_e32 v10, v13
	v_cmp_ngt_f32_e32 vcc, s3, v8
	v_mul_f32_e32 v10, v12, v10
	v_cndmask_b32_e32 v10, 0, v10, vcc
	v_cmp_nlt_f32_e32 vcc, s4, v8
	v_cndmask_b32_e32 v8, v9, v10, vcc
	flat_store_dword v[6:7], v8
	v_add_u32_e32 v6, vcc, 4, v6
	v_add_f32_e32 v5, v5, v8
	v_addc_u32_e32 v7, vcc, 0, v7, vcc
	s_cbranch_scc0 BB4_5
; %bb.6:                                ; %._crit_edge
	v_rcp_f32_e32 v0, v5
BB4_7:                                  ; %.lr.ph
                                        ; =>This Inner Loop Header: Depth=1
	flat_load_dword v1, v[3:4]
	s_add_i32 s2, s2, -1
	s_cmp_lg_u32 s2, 0
	s_waitcnt vmcnt(0) lgkmcnt(0)
	v_mul_f32_e32 v1, v0, v1
	flat_store_dword v[3:4], v1
	v_add_u32_e32 v3, vcc, 4, v3
	v_addc_u32_e32 v4, vcc, 0, v4, vcc
	s_cbranch_scc1 BB4_7
BB4_8:                                  ; %.loopexit
	s_endpgm
.Lfunc_end4:
	.size	gm_softmax, .Lfunc_end4-gm_softmax
                                        ; -- End function
	.section	.AMDGPU.csdata
; Kernel info:
; codeLenInByte = 456
; NumSgprs: 18
; NumVgprs: 14
; ScratchSize: 0
; MemoryBound: 0
; FloatMode: 192
; IeeeMode: 1
; LDSByteSize: 0 bytes/workgroup (compile time only)
; SGPRBlocks: 2
; VGPRBlocks: 3
; NumSGPRsForWavesPerEU: 18
; NumVGPRsForWavesPerEU: 14
; Occupancy: 10
; WaveLimiterHint : 0
; COMPUTE_PGM_RSRC2:USER_SGPR: 8
; COMPUTE_PGM_RSRC2:TRAP_HANDLER: 0
; COMPUTE_PGM_RSRC2:TGID_X_EN: 1
; COMPUTE_PGM_RSRC2:TGID_Y_EN: 1
; COMPUTE_PGM_RSRC2:TGID_Z_EN: 0
; COMPUTE_PGM_RSRC2:TIDIG_COMP_CNT: 1
	.text
	.protected	gm_attention_context ; -- Begin function gm_attention_context
	.globl	gm_attention_context
	.p2align	8
	.type	gm_attention_context,@function
	.amdgpu_hsa_kernel gm_attention_context
gm_attention_context:                   ; @gm_attention_context
gm_attention_context$local:
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
		enable_sgpr_workgroup_id_y = 1
		enable_sgpr_workgroup_id_z = 0
		enable_sgpr_workgroup_info = 0
		enable_vgpr_workitem_id = 1
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
	s_load_dwordx4 s[0:3], s[6:7], 0x28
	s_waitcnt lgkmcnt(0)
	s_load_dword s1, s[4:5], 0x4
	s_waitcnt lgkmcnt(0)
	s_and_b32 s3, s1, 0xffff
	s_mul_i32 s8, s8, s3
	s_add_i32 s0, s0, s8
	v_add_u32_e32 v2, vcc, s0, v0
	s_lshr_b32 s0, s1, 16
	s_mul_i32 s9, s9, s0
	s_add_i32 s0, s2, s9
	v_add_u32_e32 v0, vcc, s0, v1
	s_mul_i32 s0, s14, s12
	v_cmp_gt_i32_e32 vcc, s13, v2
	v_cmp_gt_i32_e64 s[0:1], s0, v0
	s_and_b64 s[0:1], vcc, s[0:1]
	s_and_saveexec_b64 s[2:3], s[0:1]
	s_cbranch_execz BB5_5
; %bb.1:
	s_ashr_i32 s2, s12, 31
	s_add_i32 s0, s12, s2
	s_xor_b32 s3, s0, s2
	v_cvt_f32_u32_e32 v1, s3
	v_ashrrev_i32_e32 v5, 31, v0
	s_load_dwordx4 s[8:11], s[6:7], 0x10
	v_rcp_iflag_f32_e32 v1, v1
	v_mul_f32_e32 v1, 0x4f800000, v1
	v_cvt_u32_f32_e32 v1, v1
	v_mul_lo_u32 v3, v1, s3
	v_mul_hi_u32 v4, v1, s3
	v_sub_u32_e32 v6, vcc, 0, v3
	v_cmp_eq_u32_e64 s[0:1], 0, v4
	v_cndmask_b32_e64 v3, v3, v6, s[0:1]
	v_mul_hi_u32 v3, v3, v1
	v_add_u32_e32 v4, vcc, v0, v5
	v_xor_b32_e32 v4, v4, v5
	v_add_u32_e32 v6, vcc, v3, v1
	v_subrev_u32_e32 v1, vcc, v3, v1
	v_cndmask_b32_e64 v1, v1, v6, s[0:1]
	v_mul_hi_u32 v1, v1, v4
	v_xor_b32_e32 v3, s2, v5
	v_mul_lo_u32 v5, v1, s3
	v_add_u32_e32 v6, vcc, 1, v1
	v_add_u32_e32 v7, vcc, -1, v1
	v_sub_u32_e32 v8, vcc, v4, v5
	v_cmp_ge_u32_e32 vcc, v4, v5
	v_cmp_le_u32_e64 s[0:1], s3, v8
	s_and_b64 s[0:1], s[0:1], vcc
	v_cndmask_b32_e64 v1, v1, v6, s[0:1]
	v_cndmask_b32_e32 v1, v7, v1, vcc
	v_xor_b32_e32 v1, v1, v3
	v_sub_u32_e32 v1, vcc, v1, v3
	v_mul_lo_u32 v3, v1, s13
	s_mul_i32 s0, s14, s13
	s_cmp_lt_i32 s12, 1
	v_add_u32_e32 v2, vcc, v3, v2
	v_mov_b32_e32 v3, 0
	s_cbranch_scc1 BB5_4
; %bb.2:                                ; %.lr.ph.preheader
	v_mul_lo_u32 v3, s12, v0
	s_load_dwordx4 s[4:7], s[6:7], 0x0
	s_mov_b32 s1, s12
	v_mov_b32_e32 v6, v2
	v_ashrrev_i32_e32 v4, 31, v3
	v_lshlrev_b64 v[4:5], 2, v[3:4]
	s_waitcnt lgkmcnt(0)
	v_mov_b32_e32 v3, s5
	v_add_u32_e32 v4, vcc, s4, v4
	v_addc_u32_e32 v5, vcc, v3, v5, vcc
	v_mov_b32_e32 v3, 0
BB5_3:                                  ; %.lr.ph
                                        ; =>This Inner Loop Header: Depth=1
	v_ashrrev_i32_e32 v7, 31, v6
	flat_load_dword v9, v[4:5]
	v_add_u32_e32 v4, vcc, 4, v4
	v_lshlrev_b64 v[7:8], 2, v[6:7]
	v_addc_u32_e32 v5, vcc, 0, v5, vcc
	v_add_u32_e32 v6, vcc, s0, v6
	v_mov_b32_e32 v10, s7
	v_add_u32_e32 v7, vcc, s6, v7
	v_addc_u32_e32 v8, vcc, v10, v8, vcc
	flat_load_dword v7, v[7:8]
	s_add_i32 s1, s1, -1
	s_cmp_eq_u32 s1, 0
	s_waitcnt vmcnt(0) lgkmcnt(0)
	v_mac_f32_e32 v3, v9, v7
	s_cbranch_scc0 BB5_3
BB5_4:                                  ; %Flow24
	v_mul_lo_u32 v1, v1, s12
	v_sub_u32_e32 v0, vcc, v0, v1
	v_mul_lo_u32 v0, v0, s0
	v_add_u32_e32 v0, vcc, v0, v2
	v_ashrrev_i32_e32 v1, 31, v0
	v_lshlrev_b64 v[0:1], 2, v[0:1]
	s_waitcnt lgkmcnt(0)
	v_mov_b32_e32 v2, s9
	v_add_u32_e32 v0, vcc, s8, v0
	v_addc_u32_e32 v1, vcc, v2, v1, vcc
	flat_store_dword v[0:1], v3
BB5_5:
	s_endpgm
.Lfunc_end5:
	.size	gm_attention_context, .Lfunc_end5-gm_attention_context
                                        ; -- End function
	.section	.AMDGPU.csdata
; Kernel info:
; codeLenInByte = 496
; NumSgprs: 18
; NumVgprs: 11
; ScratchSize: 0
; MemoryBound: 0
; FloatMode: 192
; IeeeMode: 1
; LDSByteSize: 0 bytes/workgroup (compile time only)
; SGPRBlocks: 2
; VGPRBlocks: 2
; NumSGPRsForWavesPerEU: 18
; NumVGPRsForWavesPerEU: 11
; Occupancy: 10
; WaveLimiterHint : 1
; COMPUTE_PGM_RSRC2:USER_SGPR: 8
; COMPUTE_PGM_RSRC2:TRAP_HANDLER: 0
; COMPUTE_PGM_RSRC2:TGID_X_EN: 1
; COMPUTE_PGM_RSRC2:TGID_Y_EN: 1
; COMPUTE_PGM_RSRC2:TGID_Z_EN: 0
; COMPUTE_PGM_RSRC2:TIDIG_COMP_CNT: 1
	.text
	.protected	gm_o_proj       ; -- Begin function gm_o_proj
	.globl	gm_o_proj
	.p2align	8
	.type	gm_o_proj,@function
	.amdgpu_hsa_kernel gm_o_proj
gm_o_proj:                              ; @gm_o_proj
gm_o_proj$local:
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
		enable_sgpr_workgroup_id_y = 1
		enable_sgpr_workgroup_id_z = 0
		enable_sgpr_workgroup_info = 0
		enable_vgpr_workitem_id = 1
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
	s_load_dwordx4 s[0:3], s[6:7], 0x28
	s_waitcnt lgkmcnt(0)
	s_load_dword s1, s[4:5], 0x4
	s_waitcnt lgkmcnt(0)
	s_and_b32 s3, s1, 0xffff
	s_mul_i32 s8, s8, s3
	s_add_i32 s0, s0, s8
	v_add_u32_e32 v0, vcc, s0, v0
	s_lshr_b32 s0, s1, 16
	s_mul_i32 s9, s9, s0
	s_add_i32 s0, s2, s9
	v_add_u32_e32 v1, vcc, s0, v1
	v_cmp_gt_i32_e32 vcc, s12, v1
	v_cmp_gt_i32_e64 s[0:1], s13, v0
	s_and_b64 s[0:1], s[0:1], vcc
	s_and_saveexec_b64 s[2:3], s[0:1]
	s_cbranch_execz BB6_5
; %bb.1:
	s_load_dwordx4 s[0:3], s[6:7], 0x10
	s_cmp_lt_i32 s14, 1
	v_mov_b32_e32 v2, 0
	s_cbranch_scc1 BB6_4
; %bb.2:                                ; %.lr.ph.preheader
	v_mul_lo_u32 v2, s14, v1
	s_load_dwordx4 s[4:7], s[6:7], 0x0
	s_waitcnt lgkmcnt(0)
	s_mov_b32 s2, s14
	v_mov_b32_e32 v5, v0
	v_ashrrev_i32_e32 v3, 31, v2
	v_lshlrev_b64 v[3:4], 2, v[2:3]
	v_mov_b32_e32 v2, s5
	v_add_u32_e32 v3, vcc, s4, v3
	v_addc_u32_e32 v4, vcc, v2, v4, vcc
	v_mov_b32_e32 v2, 0
BB6_3:                                  ; %.lr.ph
                                        ; =>This Inner Loop Header: Depth=1
	v_ashrrev_i32_e32 v6, 31, v5
	flat_load_dword v8, v[3:4]
	v_add_u32_e32 v3, vcc, 4, v3
	v_lshlrev_b64 v[6:7], 2, v[5:6]
	v_addc_u32_e32 v4, vcc, 0, v4, vcc
	v_add_u32_e32 v5, vcc, s13, v5
	v_mov_b32_e32 v9, s7
	v_add_u32_e32 v6, vcc, s6, v6
	v_addc_u32_e32 v7, vcc, v9, v7, vcc
	flat_load_dword v6, v[6:7]
	s_add_i32 s2, s2, -1
	s_cmp_eq_u32 s2, 0
	s_waitcnt vmcnt(0) lgkmcnt(0)
	v_mac_f32_e32 v2, v8, v6
	s_cbranch_scc0 BB6_3
BB6_4:                                  ; %Flow24
	v_mul_lo_u32 v1, v1, s13
	s_waitcnt lgkmcnt(0)
	v_mov_b32_e32 v3, s1
	v_add_u32_e32 v0, vcc, v1, v0
	v_ashrrev_i32_e32 v1, 31, v0
	v_lshlrev_b64 v[0:1], 2, v[0:1]
	v_add_u32_e32 v0, vcc, s0, v0
	v_addc_u32_e32 v1, vcc, v3, v1, vcc
	flat_store_dword v[0:1], v2
BB6_5:
	s_endpgm
.Lfunc_end6:
	.size	gm_o_proj, .Lfunc_end6-gm_o_proj
                                        ; -- End function
	.section	.AMDGPU.csdata
; Kernel info:
; codeLenInByte = 292
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
; COMPUTE_PGM_RSRC2:TGID_Y_EN: 1
; COMPUTE_PGM_RSRC2:TGID_Z_EN: 0
; COMPUTE_PGM_RSRC2:TIDIG_COMP_CNT: 1
	.text
	.protected	gm_add_resid1   ; -- Begin function gm_add_resid1
	.globl	gm_add_resid1
	.p2align	8
	.type	gm_add_resid1,@function
	.amdgpu_hsa_kernel gm_add_resid1
gm_add_resid1:                          ; @gm_add_resid1
gm_add_resid1$local:
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
	s_cbranch_execz BB7_2
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
BB7_2:
	s_endpgm
.Lfunc_end7:
	.size	gm_add_resid1, .Lfunc_end7-gm_add_resid1
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
	.protected	gm_add_resid2   ; -- Begin function gm_add_resid2
	.globl	gm_add_resid2
	.p2align	8
	.type	gm_add_resid2,@function
	.amdgpu_hsa_kernel gm_add_resid2
gm_add_resid2:                          ; @gm_add_resid2
gm_add_resid2$local:
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
	s_cbranch_execz BB8_2
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
BB8_2:
	s_endpgm
.Lfunc_end8:
	.size	gm_add_resid2, .Lfunc_end8-gm_add_resid2
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
	.protected	gm_layernorm1   ; -- Begin function gm_layernorm1
	.globl	gm_layernorm1
	.p2align	8
	.type	gm_layernorm1,@function
	.amdgpu_hsa_kernel gm_layernorm1
gm_layernorm1:                          ; @gm_layernorm1
gm_layernorm1$local:
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
	s_cbranch_execz BB9_8
; %bb.1:
	s_cmp_lt_i32 s1, 1
	s_cbranch_scc1 BB9_8
; %bb.2:                                ; %.lr.ph74.preheader
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
BB9_3:                                  ; %.lr.ph74
                                        ; =>This Inner Loop Header: Depth=1
	flat_load_dword v7, v[5:6]
	v_add_u32_e32 v5, vcc, 4, v5
	s_add_i32 s0, s0, -1
	v_addc_u32_e32 v6, vcc, 0, v6, vcc
	s_cmp_eq_u32 s0, 0
	s_waitcnt vmcnt(0) lgkmcnt(0)
	v_add_f32_e32 v2, v2, v7
	s_cbranch_scc0 BB9_3
; %bb.4:                                ; %._crit_edge75
	v_cvt_f32_i32_e32 v5, s1
	s_mov_b32 s0, 0x6f800000
	v_mov_b32_e32 v6, 0x2f800000
	v_cmp_gt_f32_e64 vcc, |v5|, s0
	v_cndmask_b32_e32 v7, 1.0, v6, vcc
	v_mul_f32_e32 v6, v5, v7
	v_rcp_f32_e32 v8, v6
	v_mov_b32_e32 v6, 0
	s_mov_b32 s0, s1
	v_mul_f32_e32 v2, v2, v8
	v_mul_f32_e32 v2, v7, v2
	v_mov_b32_e32 v8, v1
	v_mov_b32_e32 v7, v0
BB9_5:                                  ; %.lr.ph70
                                        ; =>This Inner Loop Header: Depth=1
	flat_load_dword v9, v[7:8]
	v_add_u32_e32 v7, vcc, 4, v7
	s_add_i32 s0, s0, -1
	v_addc_u32_e32 v8, vcc, 0, v8, vcc
	s_cmp_eq_u32 s0, 0
	s_waitcnt vmcnt(0) lgkmcnt(0)
	v_sub_f32_e32 v9, v9, v2
	v_mac_f32_e32 v6, v9, v9
	s_cbranch_scc0 BB9_5
; %bb.6:                                ; %.lr.ph.preheader
	s_mov_b32 s0, 0x6f800000
	v_mov_b32_e32 v7, 0x2f800000
	v_cmp_gt_f32_e64 vcc, |v5|, s0
	v_cndmask_b32_e32 v7, 1.0, v7, vcc
	v_mul_f32_e32 v5, v5, v7
	v_rcp_f32_e32 v8, v5
	v_lshlrev_b64 v[4:5], 2, v[3:4]
	v_add_u32_e32 v4, vcc, s10, v4
	v_mul_f32_e32 v3, v6, v8
	v_mad_f32 v3, v7, v3, s2
	v_rsq_f32_e32 v3, v3
	v_mov_b32_e32 v6, s11
	v_addc_u32_e32 v5, vcc, v6, v5, vcc
BB9_7:                                  ; %.lr.ph
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
	v_mul_f32_e32 v8, v3, v8
	s_waitcnt vmcnt(0) lgkmcnt(0)
	v_mac_f32_e32 v7, v8, v6
	flat_store_dword v[4:5], v7
	v_add_u32_e32 v4, vcc, 4, v4
	v_addc_u32_e32 v5, vcc, 0, v5, vcc
	s_cbranch_scc1 BB9_7
BB9_8:                                  ; %.loopexit
	s_endpgm
.Lfunc_end9:
	.size	gm_layernorm1, .Lfunc_end9-gm_layernorm1
                                        ; -- End function
	.section	.AMDGPU.csdata
; Kernel info:
; codeLenInByte = 460
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
	.protected	gm_layernorm2   ; -- Begin function gm_layernorm2
	.globl	gm_layernorm2
	.p2align	8
	.type	gm_layernorm2,@function
	.amdgpu_hsa_kernel gm_layernorm2
gm_layernorm2:                          ; @gm_layernorm2
gm_layernorm2$local:
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
	s_cbranch_execz BB10_8
; %bb.1:
	s_cmp_lt_i32 s1, 1
	s_cbranch_scc1 BB10_8
; %bb.2:                                ; %.lr.ph74.preheader
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
BB10_3:                                 ; %.lr.ph74
                                        ; =>This Inner Loop Header: Depth=1
	flat_load_dword v7, v[5:6]
	v_add_u32_e32 v5, vcc, 4, v5
	s_add_i32 s0, s0, -1
	v_addc_u32_e32 v6, vcc, 0, v6, vcc
	s_cmp_eq_u32 s0, 0
	s_waitcnt vmcnt(0) lgkmcnt(0)
	v_add_f32_e32 v2, v2, v7
	s_cbranch_scc0 BB10_3
; %bb.4:                                ; %._crit_edge75
	v_cvt_f32_i32_e32 v5, s1
	s_mov_b32 s0, 0x6f800000
	v_mov_b32_e32 v6, 0x2f800000
	v_cmp_gt_f32_e64 vcc, |v5|, s0
	v_cndmask_b32_e32 v7, 1.0, v6, vcc
	v_mul_f32_e32 v6, v5, v7
	v_rcp_f32_e32 v8, v6
	v_mov_b32_e32 v6, 0
	s_mov_b32 s0, s1
	v_mul_f32_e32 v2, v2, v8
	v_mul_f32_e32 v2, v7, v2
	v_mov_b32_e32 v8, v1
	v_mov_b32_e32 v7, v0
BB10_5:                                 ; %.lr.ph70
                                        ; =>This Inner Loop Header: Depth=1
	flat_load_dword v9, v[7:8]
	v_add_u32_e32 v7, vcc, 4, v7
	s_add_i32 s0, s0, -1
	v_addc_u32_e32 v8, vcc, 0, v8, vcc
	s_cmp_eq_u32 s0, 0
	s_waitcnt vmcnt(0) lgkmcnt(0)
	v_sub_f32_e32 v9, v9, v2
	v_mac_f32_e32 v6, v9, v9
	s_cbranch_scc0 BB10_5
; %bb.6:                                ; %.lr.ph.preheader
	s_mov_b32 s0, 0x6f800000
	v_mov_b32_e32 v7, 0x2f800000
	v_cmp_gt_f32_e64 vcc, |v5|, s0
	v_cndmask_b32_e32 v7, 1.0, v7, vcc
	v_mul_f32_e32 v5, v5, v7
	v_rcp_f32_e32 v8, v5
	v_lshlrev_b64 v[4:5], 2, v[3:4]
	v_add_u32_e32 v4, vcc, s10, v4
	v_mul_f32_e32 v3, v6, v8
	v_mad_f32 v3, v7, v3, s2
	v_rsq_f32_e32 v3, v3
	v_mov_b32_e32 v6, s11
	v_addc_u32_e32 v5, vcc, v6, v5, vcc
BB10_7:                                 ; %.lr.ph
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
	v_mul_f32_e32 v8, v3, v8
	s_waitcnt vmcnt(0) lgkmcnt(0)
	v_mac_f32_e32 v7, v8, v6
	flat_store_dword v[4:5], v7
	v_add_u32_e32 v4, vcc, 4, v4
	v_addc_u32_e32 v5, vcc, 0, v5, vcc
	s_cbranch_scc1 BB10_7
BB10_8:                                 ; %.loopexit
	s_endpgm
.Lfunc_end10:
	.size	gm_layernorm2, .Lfunc_end10-gm_layernorm2
                                        ; -- End function
	.section	.AMDGPU.csdata
; Kernel info:
; codeLenInByte = 460
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
	.protected	gm_ff1          ; -- Begin function gm_ff1
	.globl	gm_ff1
	.p2align	8
	.type	gm_ff1,@function
	.amdgpu_hsa_kernel gm_ff1
gm_ff1:                                 ; @gm_ff1
gm_ff1$local:
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
		enable_sgpr_workgroup_id_y = 1
		enable_sgpr_workgroup_id_z = 0
		enable_sgpr_workgroup_info = 0
		enable_vgpr_workitem_id = 1
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
	s_load_dwordx4 s[0:3], s[6:7], 0x28
	s_waitcnt lgkmcnt(0)
	s_load_dword s1, s[4:5], 0x4
	s_waitcnt lgkmcnt(0)
	s_and_b32 s3, s1, 0xffff
	s_mul_i32 s8, s8, s3
	s_add_i32 s0, s0, s8
	v_add_u32_e32 v0, vcc, s0, v0
	s_lshr_b32 s0, s1, 16
	s_mul_i32 s9, s9, s0
	s_add_i32 s0, s2, s9
	v_add_u32_e32 v1, vcc, s0, v1
	v_cmp_gt_i32_e32 vcc, s12, v1
	v_cmp_gt_i32_e64 s[0:1], s13, v0
	s_and_b64 s[0:1], s[0:1], vcc
	s_and_saveexec_b64 s[2:3], s[0:1]
	s_cbranch_execz BB11_5
; %bb.1:
	s_load_dwordx4 s[0:3], s[6:7], 0x10
	s_cmp_lt_i32 s14, 1
	v_mov_b32_e32 v2, 0
	s_cbranch_scc1 BB11_4
; %bb.2:                                ; %.lr.ph.preheader
	v_mul_lo_u32 v2, s14, v1
	s_load_dwordx4 s[4:7], s[6:7], 0x0
	s_waitcnt lgkmcnt(0)
	s_mov_b32 s2, s14
	v_mov_b32_e32 v5, v0
	v_ashrrev_i32_e32 v3, 31, v2
	v_lshlrev_b64 v[3:4], 2, v[2:3]
	v_mov_b32_e32 v2, s5
	v_add_u32_e32 v3, vcc, s4, v3
	v_addc_u32_e32 v4, vcc, v2, v4, vcc
	v_mov_b32_e32 v2, 0
BB11_3:                                 ; %.lr.ph
                                        ; =>This Inner Loop Header: Depth=1
	v_ashrrev_i32_e32 v6, 31, v5
	flat_load_dword v8, v[3:4]
	v_add_u32_e32 v3, vcc, 4, v3
	v_lshlrev_b64 v[6:7], 2, v[5:6]
	v_addc_u32_e32 v4, vcc, 0, v4, vcc
	v_add_u32_e32 v5, vcc, s13, v5
	v_mov_b32_e32 v9, s7
	v_add_u32_e32 v6, vcc, s6, v6
	v_addc_u32_e32 v7, vcc, v9, v7, vcc
	flat_load_dword v6, v[6:7]
	s_add_i32 s2, s2, -1
	s_cmp_eq_u32 s2, 0
	s_waitcnt vmcnt(0) lgkmcnt(0)
	v_mac_f32_e32 v2, v8, v6
	s_cbranch_scc0 BB11_3
BB11_4:                                 ; %Flow24
	v_mul_lo_u32 v1, v1, s13
	s_waitcnt lgkmcnt(0)
	v_mov_b32_e32 v3, s1
	v_add_u32_e32 v0, vcc, v1, v0
	v_ashrrev_i32_e32 v1, 31, v0
	v_lshlrev_b64 v[0:1], 2, v[0:1]
	v_add_u32_e32 v0, vcc, s0, v0
	v_addc_u32_e32 v1, vcc, v3, v1, vcc
	flat_store_dword v[0:1], v2
BB11_5:
	s_endpgm
.Lfunc_end11:
	.size	gm_ff1, .Lfunc_end11-gm_ff1
                                        ; -- End function
	.section	.AMDGPU.csdata
; Kernel info:
; codeLenInByte = 292
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
; COMPUTE_PGM_RSRC2:TGID_Y_EN: 1
; COMPUTE_PGM_RSRC2:TGID_Z_EN: 0
; COMPUTE_PGM_RSRC2:TIDIG_COMP_CNT: 1
	.text
	.protected	gm_relu         ; -- Begin function gm_relu
	.globl	gm_relu
	.p2align	8
	.type	gm_relu,@function
	.amdgpu_hsa_kernel gm_relu
gm_relu:                                ; @gm_relu
gm_relu$local:
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
	s_cbranch_execz BB12_2
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
	v_cmp_ngt_f32_e32 vcc, 0, v2
	v_cndmask_b32_e32 v2, 0, v2, vcc
	flat_store_dword v[0:1], v2
BB12_2:
	s_endpgm
.Lfunc_end12:
	.size	gm_relu, .Lfunc_end12-gm_relu
                                        ; -- End function
	.section	.AMDGPU.csdata
; Kernel info:
; codeLenInByte = 136
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
	.protected	gm_ff2          ; -- Begin function gm_ff2
	.globl	gm_ff2
	.p2align	8
	.type	gm_ff2,@function
	.amdgpu_hsa_kernel gm_ff2
gm_ff2:                                 ; @gm_ff2
gm_ff2$local:
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
		enable_sgpr_workgroup_id_y = 1
		enable_sgpr_workgroup_id_z = 0
		enable_sgpr_workgroup_info = 0
		enable_vgpr_workitem_id = 1
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
	s_load_dwordx4 s[0:3], s[6:7], 0x28
	s_waitcnt lgkmcnt(0)
	s_load_dword s1, s[4:5], 0x4
	s_waitcnt lgkmcnt(0)
	s_and_b32 s3, s1, 0xffff
	s_mul_i32 s8, s8, s3
	s_add_i32 s0, s0, s8
	v_add_u32_e32 v0, vcc, s0, v0
	s_lshr_b32 s0, s1, 16
	s_mul_i32 s9, s9, s0
	s_add_i32 s0, s2, s9
	v_add_u32_e32 v1, vcc, s0, v1
	v_cmp_gt_i32_e32 vcc, s12, v1
	v_cmp_gt_i32_e64 s[0:1], s13, v0
	s_and_b64 s[0:1], s[0:1], vcc
	s_and_saveexec_b64 s[2:3], s[0:1]
	s_cbranch_execz BB13_5
; %bb.1:
	s_load_dwordx4 s[0:3], s[6:7], 0x10
	s_cmp_lt_i32 s14, 1
	v_mov_b32_e32 v2, 0
	s_cbranch_scc1 BB13_4
; %bb.2:                                ; %.lr.ph.preheader
	v_mul_lo_u32 v2, s14, v1
	s_load_dwordx4 s[4:7], s[6:7], 0x0
	s_waitcnt lgkmcnt(0)
	s_mov_b32 s2, s14
	v_mov_b32_e32 v5, v0
	v_ashrrev_i32_e32 v3, 31, v2
	v_lshlrev_b64 v[3:4], 2, v[2:3]
	v_mov_b32_e32 v2, s5
	v_add_u32_e32 v3, vcc, s4, v3
	v_addc_u32_e32 v4, vcc, v2, v4, vcc
	v_mov_b32_e32 v2, 0
BB13_3:                                 ; %.lr.ph
                                        ; =>This Inner Loop Header: Depth=1
	v_ashrrev_i32_e32 v6, 31, v5
	flat_load_dword v8, v[3:4]
	v_add_u32_e32 v3, vcc, 4, v3
	v_lshlrev_b64 v[6:7], 2, v[5:6]
	v_addc_u32_e32 v4, vcc, 0, v4, vcc
	v_add_u32_e32 v5, vcc, s13, v5
	v_mov_b32_e32 v9, s7
	v_add_u32_e32 v6, vcc, s6, v6
	v_addc_u32_e32 v7, vcc, v9, v7, vcc
	flat_load_dword v6, v[6:7]
	s_add_i32 s2, s2, -1
	s_cmp_eq_u32 s2, 0
	s_waitcnt vmcnt(0) lgkmcnt(0)
	v_mac_f32_e32 v2, v8, v6
	s_cbranch_scc0 BB13_3
BB13_4:                                 ; %Flow24
	v_mul_lo_u32 v1, v1, s13
	s_waitcnt lgkmcnt(0)
	v_mov_b32_e32 v3, s1
	v_add_u32_e32 v0, vcc, v1, v0
	v_ashrrev_i32_e32 v1, 31, v0
	v_lshlrev_b64 v[0:1], 2, v[0:1]
	v_add_u32_e32 v0, vcc, s0, v0
	v_addc_u32_e32 v1, vcc, v3, v1, vcc
	flat_store_dword v[0:1], v2
BB13_5:
	s_endpgm
.Lfunc_end13:
	.size	gm_ff2, .Lfunc_end13-gm_ff2
                                        ; -- End function
	.section	.AMDGPU.csdata
; Kernel info:
; codeLenInByte = 292
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
; COMPUTE_PGM_RSRC2:TGID_Y_EN: 1
; COMPUTE_PGM_RSRC2:TGID_Z_EN: 0
; COMPUTE_PGM_RSRC2:TIDIG_COMP_CNT: 1
	.ident	"clang version 11.0.0 (https://github.com/ROCm/llvm-project.git b98349b12ffa706d0e863a3f1176b20d2a6c438b)"
	.section	".note.GNU-stack"
	.addrsig
	.amd_amdgpu_isa "amdgcn-amd-amdhsa--gfx803"
	.amd_amdgpu_hsa_metadata
---
Version:         [ 1, 0 ]
Kernels:
  - Name:            gm_q_proj
    SymbolName:      'gm_q_proj@kd'
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
  - Name:            gm_k_proj
    SymbolName:      'gm_k_proj@kd'
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
  - Name:            gm_v_proj
    SymbolName:      'gm_v_proj@kd'
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
  - Name:            gm_attention_scores_masked
    SymbolName:      'gm_attention_scores_masked@kd'
    Language:        OpenCL C
    LanguageVersion: [ 1, 2 ]
    Args:
      - Name:            Q
        TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        ValueType:       F32
        AddrSpaceQual:   Global
        AccQual:         Default
        IsConst:         true
      - Name:            K
        TypeName:        'float*'
        Size:            8
        Align:           8
        ValueKind:       GlobalBuffer
        ValueType:       F32
        AddrSpaceQual:   Global
        AccQual:         Default
        IsConst:         true
      - Name:            Scores
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
      NumSGPRs:        18
      NumVGPRs:        9
      MaxFlatWorkGroupSize: 256
  - Name:            gm_softmax
    SymbolName:      'gm_softmax@kd'
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
      - Name:            SeqLen
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
      KernargSegmentSize: 80
      GroupSegmentFixedSize: 0
      PrivateSegmentFixedSize: 0
      KernargSegmentAlign: 8
      WavefrontSize:   64
      NumSGPRs:        18
      NumVGPRs:        14
      MaxFlatWorkGroupSize: 256
  - Name:            gm_attention_context
    SymbolName:      'gm_attention_context@kd'
    Language:        OpenCL C
    LanguageVersion: [ 1, 2 ]
    Args:
      - Name:            Probs
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
      KernargSegmentSize: 96
      GroupSegmentFixedSize: 0
      PrivateSegmentFixedSize: 0
      KernargSegmentAlign: 8
      WavefrontSize:   64
      NumSGPRs:        18
      NumVGPRs:        11
      MaxFlatWorkGroupSize: 256
  - Name:            gm_o_proj
    SymbolName:      'gm_o_proj@kd'
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
  - Name:            gm_add_resid1
    SymbolName:      'gm_add_resid1@kd'
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
  - Name:            gm_add_resid2
    SymbolName:      'gm_add_resid2@kd'
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
  - Name:            gm_layernorm1
    SymbolName:      'gm_layernorm1@kd'
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
  - Name:            gm_layernorm2
    SymbolName:      'gm_layernorm2@kd'
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
  - Name:            gm_ff1
    SymbolName:      'gm_ff1@kd'
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
  - Name:            gm_relu
    SymbolName:      'gm_relu@kd'
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
  - Name:            gm_ff2
    SymbolName:      'gm_ff2@kd'
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
