	.text
	.hsa_code_object_version 2,1
	.hsa_code_object_isa 8,0,3,"AMD","AMDGPU"
	.protected	mb_rmsnorm      ; -- Begin function mb_rmsnorm
	.globl	mb_rmsnorm
	.p2align	8
	.type	mb_rmsnorm,@function
	.amdgpu_hsa_kernel mb_rmsnorm
mb_rmsnorm:                             ; @mb_rmsnorm
mb_rmsnorm$local:
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
		kernarg_segment_byte_size = 96
		workgroup_fbarrier_count = 0
		wavefront_sgpr_count = 14
		workitem_vgpr_count = 8
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
	s_load_dwordx4 s[0:3], s[6:7], 0x18
	s_load_dword s9, s[6:7], 0x28
	s_load_dword s4, s[4:5], 0x4
	s_waitcnt lgkmcnt(0)
	s_and_b32 s4, s4, 0xffff
	s_mul_i32 s8, s8, s4
	s_add_i32 s9, s9, s8
	v_add_u32_e32 v0, vcc, s9, v0
	v_cmp_gt_i32_e32 vcc, s0, v0
	s_and_saveexec_b64 s[4:5], vcc
	s_cbranch_execz BB0_7
; %bb.1:
	s_load_dwordx4 s[8:11], s[6:7], 0x0
	s_load_dwordx4 s[4:7], s[6:7], 0x10
	s_waitcnt lgkmcnt(0)
	v_cmp_gt_i32_e64 s[6:7], s1, 0
	s_cmp_lt_i32 s1, 1
	v_mov_b32_e32 v1, 0
	s_cbranch_scc1 BB0_4
; %bb.2:                                ; %.lr.ph51.preheader
	v_mul_lo_u32 v1, s1, v0
	v_mov_b32_e32 v4, s9
	s_mov_b32 s0, s1
	v_ashrrev_i32_e32 v2, 31, v1
	v_lshlrev_b64 v[2:3], 2, v[1:2]
	v_mov_b32_e32 v1, 0
	v_add_u32_e32 v2, vcc, s8, v2
	v_addc_u32_e32 v3, vcc, v4, v3, vcc
BB0_3:                                  ; %.lr.ph51
                                        ; =>This Inner Loop Header: Depth=1
	flat_load_dword v4, v[2:3]
	v_add_u32_e32 v2, vcc, 4, v2
	s_add_i32 s0, s0, -1
	v_addc_u32_e32 v3, vcc, 0, v3, vcc
	s_cmp_eq_u32 s0, 0
	s_waitcnt vmcnt(0) lgkmcnt(0)
	v_mac_f32_e32 v1, v4, v4
	s_cbranch_scc0 BB0_3
BB0_4:                                  ; %mb_fast_rsqrt.exit
	s_andn2_b64 vcc, exec, s[6:7]
	s_cbranch_vccnz BB0_7
; %bb.5:                                ; %.lr.ph.preheader
	v_cvt_f32_i32_e32 v3, s2
	v_mul_lo_u32 v2, s1, v0
	v_mov_b32_e32 v5, s9
	v_mul_f32_e32 v0, 0x358637bd, v3
	v_mad_f32 v0, v0, v1, s3
	v_lshrrev_b32_e32 v1, 1, v0
	v_sub_u32_e32 v1, vcc, 0x5f3759df, v1
	v_mul_f32_e32 v3, -0.5, v0
	v_mul_f32_e32 v3, v3, v1
	v_madak_f32 v3, v3, v1, 0x3fc00000
	v_mul_f32_e32 v1, v3, v1
	v_ashrrev_i32_e32 v3, 31, v2
	v_lshlrev_b64 v[3:4], 2, v[2:3]
	v_cmp_nge_f32_e32 vcc, 0, v0
	v_cndmask_b32_e32 v0, 0, v1, vcc
	v_mov_b32_e32 v2, s5
	v_add_u32_e32 v1, vcc, s4, v3
	v_addc_u32_e32 v2, vcc, v2, v4, vcc
	v_add_u32_e32 v3, vcc, s8, v3
	v_addc_u32_e32 v4, vcc, v5, v4, vcc
BB0_6:                                  ; %.lr.ph
                                        ; =>This Inner Loop Header: Depth=1
	v_mov_b32_e32 v5, s10
	flat_load_dword v7, v[3:4]
	v_mov_b32_e32 v6, s11
	flat_load_dword v5, v[5:6]
	v_add_u32_e32 v3, vcc, 4, v3
	s_add_i32 s1, s1, -1
	s_add_u32 s10, s10, 4
	v_addc_u32_e32 v4, vcc, 0, v4, vcc
	s_addc_u32 s11, s11, 0
	s_cmp_lg_u32 s1, 0
	s_waitcnt vmcnt(1) lgkmcnt(1)
	v_mul_f32_e32 v6, v0, v7
	s_waitcnt vmcnt(0) lgkmcnt(0)
	v_mul_f32_e32 v5, v6, v5
	flat_store_dword v[1:2], v5
	v_add_u32_e32 v1, vcc, 4, v1
	v_addc_u32_e32 v2, vcc, 0, v2, vcc
	s_cbranch_scc1 BB0_6
BB0_7:                                  ; %.loopexit
	s_endpgm
.Lfunc_end0:
	.size	mb_rmsnorm, .Lfunc_end0-mb_rmsnorm
                                        ; -- End function
	.section	.AMDGPU.csdata
; Kernel info:
; codeLenInByte = 376
; NumSgprs: 14
; NumVgprs: 8
; ScratchSize: 0
; MemoryBound: 0
; FloatMode: 192
; IeeeMode: 1
; LDSByteSize: 0 bytes/workgroup (compile time only)
; SGPRBlocks: 1
; VGPRBlocks: 1
; NumSGPRsForWavesPerEU: 14
; NumVGPRsForWavesPerEU: 8
; Occupancy: 10
; WaveLimiterHint : 0
; COMPUTE_PGM_RSRC2:USER_SGPR: 8
; COMPUTE_PGM_RSRC2:TRAP_HANDLER: 0
; COMPUTE_PGM_RSRC2:TGID_X_EN: 1
; COMPUTE_PGM_RSRC2:TGID_Y_EN: 0
; COMPUTE_PGM_RSRC2:TGID_Z_EN: 0
; COMPUTE_PGM_RSRC2:TIDIG_COMP_CNT: 0
	.text
	.protected	mb_inproj_x     ; -- Begin function mb_inproj_x
	.globl	mb_inproj_x
	.p2align	8
	.type	mb_inproj_x,@function
	.amdgpu_hsa_kernel mb_inproj_x
mb_inproj_x:                            ; @mb_inproj_x
mb_inproj_x$local:
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
	.size	mb_inproj_x, .Lfunc_end1-mb_inproj_x
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
	.protected	mb_inproj_z     ; -- Begin function mb_inproj_z
	.globl	mb_inproj_z
	.p2align	8
	.type	mb_inproj_z,@function
	.amdgpu_hsa_kernel mb_inproj_z
mb_inproj_z:                            ; @mb_inproj_z
mb_inproj_z$local:
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
	.size	mb_inproj_z, .Lfunc_end2-mb_inproj_z
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
	.protected	mb_conv1d       ; -- Begin function mb_conv1d
	.globl	mb_conv1d
	.p2align	8
	.type	mb_conv1d,@function
	.amdgpu_hsa_kernel mb_conv1d
mb_conv1d:                              ; @mb_conv1d
mb_conv1d$local:
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
	s_add_i32 s8, s0, s8
	s_lshr_b32 s0, s1, 16
	s_mul_i32 s9, s9, s0
	s_add_i32 s0, s2, s9
	v_add_u32_e32 v1, vcc, s0, v1
	v_add_u32_e32 v2, vcc, s8, v0
	v_cmp_gt_i32_e32 vcc, s13, v2
	v_cmp_gt_i32_e64 s[0:1], s12, v1
	s_and_b64 s[0:1], vcc, s[0:1]
	s_and_saveexec_b64 s[2:3], s[0:1]
	s_cbranch_execz BB3_7
; %bb.1:
	s_load_dwordx4 s[0:3], s[6:7], 0x10
	v_mul_lo_u32 v3, v1, s13
	s_cmp_lt_i32 s14, 1
	v_mov_b32_e32 v4, 0
	s_cbranch_scc1 BB3_6
; %bb.2:                                ; %.lr.ph
	s_load_dwordx4 s[4:7], s[6:7], 0x0
	v_mul_lo_u32 v5, s14, v2
	v_add_u32_e32 v0, vcc, v0, v3
	v_add_u32_e32 v7, vcc, s8, v0
	v_mov_b32_e32 v4, 0
	s_waitcnt lgkmcnt(0)
	s_mov_b32 s2, s14
	s_branch BB3_4
BB3_3:                                  ;   in Loop: Header=BB3_4 Depth=1
	s_or_b64 exec, exec, s[8:9]
	s_add_i32 s2, s2, -1
	v_add_u32_e32 v5, vcc, 1, v5
	v_subrev_u32_e32 v7, vcc, s13, v7
	s_cmp_eq_u32 s2, 0
	v_add_u32_e32 v1, vcc, -1, v1
	s_cbranch_scc1 BB3_6
BB3_4:                                  ; =>This Inner Loop Header: Depth=1
	v_cmp_lt_i32_e32 vcc, -1, v1
	s_and_saveexec_b64 s[8:9], vcc
	s_cbranch_execz BB3_3
; %bb.5:                                ;   in Loop: Header=BB3_4 Depth=1
	v_ashrrev_i32_e32 v8, 31, v7
	v_lshlrev_b64 v[8:9], 2, v[7:8]
	v_mov_b32_e32 v0, s5
	v_add_u32_e32 v8, vcc, s4, v8
	v_addc_u32_e32 v9, vcc, v0, v9, vcc
	v_ashrrev_i32_e32 v6, 31, v5
	flat_load_dword v0, v[8:9]
	v_lshlrev_b64 v[8:9], 2, v[5:6]
	v_mov_b32_e32 v6, s7
	v_add_u32_e32 v8, vcc, s6, v8
	v_addc_u32_e32 v9, vcc, v6, v9, vcc
	flat_load_dword v6, v[8:9]
	s_waitcnt vmcnt(0) lgkmcnt(0)
	v_mac_f32_e32 v4, v0, v6
	s_branch BB3_3
BB3_6:                                  ; %Flow29
	v_add_u32_e32 v0, vcc, v3, v2
	v_ashrrev_i32_e32 v1, 31, v0
	v_lshlrev_b64 v[0:1], 2, v[0:1]
	s_waitcnt lgkmcnt(0)
	v_mov_b32_e32 v2, s1
	v_add_u32_e32 v0, vcc, s0, v0
	v_addc_u32_e32 v1, vcc, v2, v1, vcc
	flat_store_dword v[0:1], v4
BB3_7:
	s_endpgm
.Lfunc_end3:
	.size	mb_conv1d, .Lfunc_end3-mb_conv1d
                                        ; -- End function
	.section	.AMDGPU.csdata
; Kernel info:
; codeLenInByte = 320
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
	.protected	mb_selective_scan ; -- Begin function mb_selective_scan
	.globl	mb_selective_scan
	.p2align	8
	.type	mb_selective_scan,@function
	.amdgpu_hsa_kernel mb_selective_scan
mb_selective_scan:                      ; @mb_selective_scan
mb_selective_scan$local:
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
		kernarg_segment_byte_size = 112
		workgroup_fbarrier_count = 0
		wavefront_sgpr_count = 18
		workitem_vgpr_count = 12
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
	s_load_dwordx2 s[0:1], s[6:7], 0x30
	s_load_dwordx2 s[2:3], s[6:7], 0x38
	s_waitcnt lgkmcnt(0)
	s_load_dword s3, s[4:5], 0x4
	s_waitcnt lgkmcnt(0)
	s_and_b32 s3, s3, 0xffff
	s_mul_i32 s8, s8, s3
	v_add_u32_e32 v0, vcc, s8, v0
	v_add_u32_e32 v0, vcc, s2, v0
	v_cmp_gt_i32_e32 vcc, s1, v0
	s_and_saveexec_b64 s[2:3], vcc
	s_cbranch_execz BB4_4
; %bb.1:
	s_cmp_lt_i32 s0, 1
	s_cbranch_scc1 BB4_4
; %bb.2:                                ; %.lr.ph
	s_load_dwordx8 s[8:15], s[6:7], 0x0
	s_load_dwordx4 s[4:7], s[6:7], 0x20
	v_mov_b32_e32 v2, 0
	v_mov_b32_e32 v3, v0
	v_ashrrev_i64 v[6:7], 30, v[2:3]
	s_waitcnt lgkmcnt(0)
	v_mov_b32_e32 v1, s13
	v_add_u32_e32 v3, vcc, s12, v6
	v_addc_u32_e32 v4, vcc, v1, v7, vcc
	flat_load_dword v3, v[3:4]
	v_mov_b32_e32 v1, s15
	v_add_u32_e32 v4, vcc, s14, v6
	v_addc_u32_e32 v5, vcc, v1, v7, vcc
	v_mov_b32_e32 v1, s5
	v_add_u32_e32 v8, vcc, s4, v6
	v_addc_u32_e32 v9, vcc, v1, v7, vcc
	v_mov_b32_e32 v1, s11
	v_add_u32_e32 v6, vcc, s10, v6
	v_addc_u32_e32 v7, vcc, v1, v7, vcc
	flat_load_dword v1, v[6:7]
	flat_load_dword v4, v[4:5]
	flat_load_dword v5, v[8:9]
	s_waitcnt vmcnt(2) lgkmcnt(2)
	v_xor_b32_e32 v6, 0x80000000, v1
BB4_3:                                  ; =>This Inner Loop Header: Depth=1
	v_ashrrev_i32_e32 v1, 31, v0
	v_lshlrev_b64 v[7:8], 2, v[0:1]
	v_add_u32_e32 v0, vcc, s1, v0
	v_mov_b32_e32 v10, s9
	v_add_u32_e32 v9, vcc, s8, v7
	v_addc_u32_e32 v10, vcc, v10, v8, vcc
	flat_load_dword v1, v[9:10]
	s_add_i32 s0, s0, -1
	v_mov_b32_e32 v11, s7
	v_add_u32_e32 v7, vcc, s6, v7
	s_cmp_lg_u32 s0, 0
	v_addc_u32_e32 v8, vcc, v11, v8, vcc
	s_waitcnt vmcnt(0) lgkmcnt(0)
	v_mul_f32_e32 v1, v3, v1
	v_mac_f32_e32 v1, v6, v2
	v_mac_f32_e32 v2, v5, v1
	v_mul_f32_e32 v1, v4, v2
	flat_store_dword v[7:8], v1
	s_cbranch_scc1 BB4_3
BB4_4:                                  ; %.loopexit
	s_endpgm
.Lfunc_end4:
	.size	mb_selective_scan, .Lfunc_end4-mb_selective_scan
                                        ; -- End function
	.section	.AMDGPU.csdata
; Kernel info:
; codeLenInByte = 292
; NumSgprs: 18
; NumVgprs: 12
; ScratchSize: 0
; MemoryBound: 0
; FloatMode: 192
; IeeeMode: 1
; LDSByteSize: 0 bytes/workgroup (compile time only)
; SGPRBlocks: 2
; VGPRBlocks: 2
; NumSGPRsForWavesPerEU: 18
; NumVGPRsForWavesPerEU: 12
; Occupancy: 10
; WaveLimiterHint : 1
; COMPUTE_PGM_RSRC2:USER_SGPR: 8
; COMPUTE_PGM_RSRC2:TRAP_HANDLER: 0
; COMPUTE_PGM_RSRC2:TGID_X_EN: 1
; COMPUTE_PGM_RSRC2:TGID_Y_EN: 0
; COMPUTE_PGM_RSRC2:TGID_Z_EN: 0
; COMPUTE_PGM_RSRC2:TIDIG_COMP_CNT: 0
	.text
	.protected	mb_gate_mul     ; -- Begin function mb_gate_mul
	.globl	mb_gate_mul
	.p2align	8
	.type	mb_gate_mul,@function
	.amdgpu_hsa_kernel mb_gate_mul
mb_gate_mul:                            ; @mb_gate_mul
mb_gate_mul$local:
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
	s_cbranch_execz BB5_2
; %bb.1:
	s_load_dwordx4 s[0:3], s[6:7], 0x0
	s_load_dwordx4 s[4:7], s[6:7], 0x10
	v_mov_b32_e32 v1, 0
	v_mov_b32_e32 v2, v0
	v_ashrrev_i64 v[0:1], 30, v[1:2]
	s_waitcnt lgkmcnt(0)
	v_mov_b32_e32 v3, s3
	v_add_u32_e32 v2, vcc, s2, v0
	v_addc_u32_e32 v3, vcc, v3, v1, vcc
	flat_load_dword v4, v[2:3]
	v_add_u32_e32 v2, vcc, s0, v0
	v_mov_b32_e32 v3, s1
	v_addc_u32_e32 v3, vcc, v3, v1, vcc
	flat_load_dword v2, v[2:3]
	s_mov_b32 s0, 0x3e4ccccd
	v_mov_b32_e32 v5, s5
	s_waitcnt vmcnt(1) lgkmcnt(1)
	v_mad_f32 v3, v4, s0, 0.5
	v_cmp_ngt_f32_e32 vcc, 0, v3
	v_cndmask_b32_e32 v3, 0, v3, vcc
	v_cmp_nlt_f32_e32 vcc, 1.0, v3
	v_cndmask_b32_e32 v3, 1.0, v3, vcc
	v_add_u32_e32 v0, vcc, s4, v0
	v_mul_f32_e32 v3, v4, v3
	s_waitcnt vmcnt(0) lgkmcnt(0)
	v_mul_f32_e32 v2, v2, v3
	v_addc_u32_e32 v1, vcc, v5, v1, vcc
	flat_store_dword v[0:1], v2
BB5_2:
	s_endpgm
.Lfunc_end5:
	.size	mb_gate_mul, .Lfunc_end5-mb_gate_mul
                                        ; -- End function
	.section	.AMDGPU.csdata
; Kernel info:
; codeLenInByte = 212
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
	.protected	mb_outproj      ; -- Begin function mb_outproj
	.globl	mb_outproj
	.p2align	8
	.type	mb_outproj,@function
	.amdgpu_hsa_kernel mb_outproj
mb_outproj:                             ; @mb_outproj
mb_outproj$local:
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
	.size	mb_outproj, .Lfunc_end6-mb_outproj
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
	.protected	mb_add_resid    ; -- Begin function mb_add_resid
	.globl	mb_add_resid
	.p2align	8
	.type	mb_add_resid,@function
	.amdgpu_hsa_kernel mb_add_resid
mb_add_resid:                           ; @mb_add_resid
mb_add_resid$local:
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
	.size	mb_add_resid, .Lfunc_end7-mb_add_resid
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
	.ident	"clang version 11.0.0 (https://github.com/ROCm/llvm-project.git b98349b12ffa706d0e863a3f1176b20d2a6c438b)"
	.section	".note.GNU-stack"
	.addrsig
	.amd_amdgpu_isa "amdgcn-amd-amdhsa--gfx803"
	.amd_amdgpu_hsa_metadata
---
Version:         [ 1, 0 ]
Kernels:
  - Name:            mb_rmsnorm
    SymbolName:      'mb_rmsnorm@kd'
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
      - Name:            Weight
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
      - Name:            InvDModelQ
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
      KernargSegmentSize: 96
      GroupSegmentFixedSize: 0
      PrivateSegmentFixedSize: 0
      KernargSegmentAlign: 8
      WavefrontSize:   64
      NumSGPRs:        14
      NumVGPRs:        8
      MaxFlatWorkGroupSize: 256
  - Name:            mb_inproj_x
    SymbolName:      'mb_inproj_x@kd'
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
  - Name:            mb_inproj_z
    SymbolName:      'mb_inproj_z@kd'
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
  - Name:            mb_conv1d
    SymbolName:      'mb_conv1d@kd'
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
      - Name:            Weight
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
  - Name:            mb_selective_scan
    SymbolName:      'mb_selective_scan@kd'
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
        IsConst:         true
      - Name:            Delta
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
      KernargSegmentSize: 112
      GroupSegmentFixedSize: 0
      PrivateSegmentFixedSize: 0
      KernargSegmentAlign: 8
      WavefrontSize:   64
      NumSGPRs:        18
      NumVGPRs:        12
      MaxFlatWorkGroupSize: 256
  - Name:            mb_gate_mul
    SymbolName:      'mb_gate_mul@kd'
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
  - Name:            mb_outproj
    SymbolName:      'mb_outproj@kd'
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
  - Name:            mb_add_resid
    SymbolName:      'mb_add_resid@kd'
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
...

	.end_amd_amdgpu_hsa_metadata
