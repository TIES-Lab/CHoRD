	.text
	.hsa_code_object_version 2,1
	.hsa_code_object_isa 8,0,3,"AMD","AMDGPU"
	.protected	ln_conv2d       ; -- Begin function ln_conv2d
	.globl	ln_conv2d
	.p2align	8
	.type	ln_conv2d,@function
	.amdgpu_hsa_kernel ln_conv2d
ln_conv2d:                              ; @ln_conv2d
ln_conv2d$local:
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
		kernarg_segment_byte_size = 128
		workgroup_fbarrier_count = 0
		wavefront_sgpr_count = 26
		workitem_vgpr_count = 16
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
	s_load_dwordx8 s[12:19], s[6:7], 0x20
	s_load_dword s2, s[6:7], 0x40
	s_load_dwordx4 s[20:23], s[6:7], 0x48
	s_load_dword s0, s[4:5], 0x4
	s_waitcnt lgkmcnt(0)
	s_and_b32 s1, s0, 0xffff
	s_lshr_b32 s0, s0, 16
	s_mul_i32 s8, s8, s1
	s_mul_i32 s9, s9, s0
	s_add_i32 s1, s20, s8
	s_add_i32 s0, s22, s9
	v_add_u32_e32 v1, vcc, s0, v1
	v_add_u32_e32 v0, vcc, s1, v0
	s_mul_i32 s0, s19, s15
	v_cmp_gt_i32_e32 vcc, s2, v0
	v_cmp_gt_i32_e64 s[0:1], s0, v1
	s_and_b64 s[0:1], vcc, s[0:1]
	s_and_saveexec_b64 s[4:5], s[0:1]
	s_cbranch_execz BB0_13
; %bb.1:
	s_ashr_i32 s3, s19, 31
	s_add_i32 s0, s19, s3
	s_xor_b32 s15, s0, s3
	v_cvt_f32_u32_e32 v2, s15
	v_ashrrev_i32_e32 v5, 31, v1
	s_load_dwordx8 s[4:11], s[6:7], 0x0
	v_rcp_iflag_f32_e32 v2, v2
	v_mul_f32_e32 v2, 0x4f800000, v2
	v_cvt_u32_f32_e32 v2, v2
	v_mul_lo_u32 v3, v2, s15
	v_mul_hi_u32 v4, v2, s15
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
	v_xor_b32_e32 v3, s3, v5
	s_waitcnt lgkmcnt(0)
	v_mov_b32_e32 v6, s9
	v_mul_lo_u32 v5, v2, s15
	v_add_u32_e32 v7, vcc, 1, v2
	v_add_u32_e32 v8, vcc, -1, v2
	v_sub_u32_e32 v9, vcc, v4, v5
	v_cmp_ge_u32_e32 vcc, v4, v5
	v_cmp_le_u32_e64 s[0:1], s15, v9
	s_and_b64 s[0:1], s[0:1], vcc
	v_cndmask_b32_e64 v2, v2, v7, s[0:1]
	v_cndmask_b32_e32 v2, v8, v2, vcc
	v_xor_b32_e32 v2, v2, v3
	v_sub_u32_e32 v3, vcc, v2, v3
	v_ashrrev_i32_e32 v4, 31, v3
	v_lshlrev_b64 v[4:5], 2, v[3:4]
	s_cmp_lt_i32 s12, 1
	v_add_u32_e32 v4, vcc, s8, v4
	v_addc_u32_e32 v5, vcc, v6, v5, vcc
	flat_load_dword v2, v[4:5]
	s_cbranch_scc1 BB0_12
; %bb.2:                                ; %.preheader102.lr.ph
	s_cmp_lt_i32 s16, 1
	s_cbranch_scc1 BB0_12
; %bb.3:                                ; %.preheader102.us.preheader
	v_mul_lo_u32 v4, v3, s19
	s_mul_i32 s3, s16, s16
	v_mul_lo_u32 v6, v0, s17
	s_mul_i32 s0, s3, s12
	v_sub_u32_e32 v4, vcc, v1, v4
	v_mul_lo_u32 v4, v4, s17
	v_mul_lo_u32 v3, s0, v3
	v_subrev_u32_e32 v6, vcc, s18, v6
	s_mul_i32 s8, s14, s13
	v_subrev_u32_e32 v4, vcc, s18, v4
	v_mul_lo_u32 v5, s14, v4
	s_mov_b32 s9, 0
	s_branch BB0_5
BB0_4:                                  ; %._crit_edge109.loopexit.us
                                        ;   in Loop: Header=BB0_5 Depth=1
	s_add_i32 s9, s9, 1
	v_add_u32_e32 v3, vcc, s3, v3
	s_cmp_lg_u32 s9, s12
	v_add_u32_e32 v5, vcc, s8, v5
	s_cbranch_scc0 BB0_12
BB0_5:                                  ; %.preheader102.us
                                        ; =>This Loop Header: Depth=1
                                        ;     Child Loop BB0_7 Depth 2
                                        ;       Child Loop BB0_10 Depth 3
	v_mov_b32_e32 v7, v5
	v_mov_b32_e32 v8, v3
	s_mov_b32 s15, 0
	s_branch BB0_7
BB0_6:                                  ; %Flow60
                                        ;   in Loop: Header=BB0_7 Depth=2
	s_or_b64 exec, exec, s[18:19]
	s_add_i32 s15, s15, 1
	v_add_u32_e32 v8, vcc, s16, v8
	s_cmp_lg_u32 s15, s16
	v_add_u32_e32 v7, vcc, s14, v7
	s_cbranch_scc0 BB0_4
BB0_7:                                  ; %.preheader.us.us
                                        ;   Parent Loop BB0_5 Depth=1
                                        ; =>  This Loop Header: Depth=2
                                        ;       Child Loop BB0_10 Depth 3
	v_add_u32_e32 v9, vcc, s15, v4
	v_cmp_lt_i32_e32 vcc, -1, v9
	v_cmp_gt_i32_e64 s[0:1], s13, v9
	s_and_b64 s[0:1], vcc, s[0:1]
	s_and_saveexec_b64 s[18:19], s[0:1]
	s_cbranch_execz BB0_6
; %bb.8:                                ; %.lr.ph.us.split.split.us.us.preheader
                                        ;   in Loop: Header=BB0_7 Depth=2
	v_mov_b32_e32 v9, v6
	v_mov_b32_e32 v10, v8
	s_mov_b32 s17, s16
	s_branch BB0_10
BB0_9:                                  ;   in Loop: Header=BB0_10 Depth=3
	s_or_b64 exec, exec, s[0:1]
	s_add_i32 s17, s17, -1
	v_add_u32_e32 v10, vcc, 1, v10
	s_cmp_lg_u32 s17, 0
	v_add_u32_e32 v9, vcc, 1, v9
	s_cbranch_scc0 BB0_6
BB0_10:                                 ; %.lr.ph.us.split.split.us.us
                                        ;   Parent Loop BB0_5 Depth=1
                                        ;     Parent Loop BB0_7 Depth=2
                                        ; =>    This Inner Loop Header: Depth=3
	v_cmp_lt_i32_e32 vcc, -1, v9
	v_cmp_gt_i32_e64 s[0:1], s14, v9
	s_and_b64 s[20:21], s[0:1], vcc
	s_and_saveexec_b64 s[0:1], s[20:21]
	s_cbranch_execz BB0_9
; %bb.11:                               ;   in Loop: Header=BB0_10 Depth=3
	v_ashrrev_i32_e32 v11, 31, v10
	v_lshlrev_b64 v[11:12], 2, v[10:11]
	v_mov_b32_e32 v13, s7
	v_add_u32_e32 v11, vcc, s6, v11
	v_addc_u32_e32 v12, vcc, v13, v12, vcc
	v_add_u32_e32 v13, vcc, v7, v9
	v_ashrrev_i32_e32 v14, 31, v13
	v_lshlrev_b64 v[13:14], 2, v[13:14]
	v_mov_b32_e32 v15, s5
	v_add_u32_e32 v13, vcc, s4, v13
	v_addc_u32_e32 v14, vcc, v15, v14, vcc
	flat_load_dword v13, v[13:14]
	flat_load_dword v11, v[11:12]
	s_waitcnt vmcnt(0) lgkmcnt(0)
	v_mac_f32_e32 v2, v13, v11
	s_branch BB0_9
BB0_12:                                 ; %._crit_edge114
	v_mul_lo_u32 v1, v1, s2
	v_mov_b32_e32 v3, s11
	v_add_u32_e32 v0, vcc, v1, v0
	v_ashrrev_i32_e32 v1, 31, v0
	v_lshlrev_b64 v[0:1], 2, v[0:1]
	v_add_u32_e32 v0, vcc, s10, v0
	v_addc_u32_e32 v1, vcc, v3, v1, vcc
	s_waitcnt vmcnt(0) lgkmcnt(0)
	flat_store_dword v[0:1], v2
BB0_13:
	s_endpgm
.Lfunc_end0:
	.size	ln_conv2d, .Lfunc_end0-ln_conv2d
                                        ; -- End function
	.section	.AMDGPU.csdata
; Kernel info:
; codeLenInByte = 688
; NumSgprs: 26
; NumVgprs: 16
; ScratchSize: 0
; MemoryBound: 0
; FloatMode: 192
; IeeeMode: 1
; LDSByteSize: 0 bytes/workgroup (compile time only)
; SGPRBlocks: 3
; VGPRBlocks: 3
; NumSGPRsForWavesPerEU: 26
; NumVGPRsForWavesPerEU: 16
; Occupancy: 10
; WaveLimiterHint : 1
; COMPUTE_PGM_RSRC2:USER_SGPR: 8
; COMPUTE_PGM_RSRC2:TRAP_HANDLER: 0
; COMPUTE_PGM_RSRC2:TGID_X_EN: 1
; COMPUTE_PGM_RSRC2:TGID_Y_EN: 1
; COMPUTE_PGM_RSRC2:TGID_Z_EN: 0
; COMPUTE_PGM_RSRC2:TIDIG_COMP_CNT: 1
	.text
	.protected	ln_relu         ; -- Begin function ln_relu
	.globl	ln_relu
	.p2align	8
	.type	ln_relu,@function
	.amdgpu_hsa_kernel ln_relu
ln_relu:                                ; @ln_relu
ln_relu$local:
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
	s_cbranch_execz BB1_2
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
BB1_2:
	s_endpgm
.Lfunc_end1:
	.size	ln_relu, .Lfunc_end1-ln_relu
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
	.protected	ln_maxpool2d    ; -- Begin function ln_maxpool2d
	.globl	ln_maxpool2d
	.p2align	8
	.type	ln_maxpool2d,@function
	.amdgpu_hsa_kernel ln_maxpool2d
ln_maxpool2d:                           ; @ln_maxpool2d
ln_maxpool2d$local:
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
		kernarg_segment_byte_size = 104
		workgroup_fbarrier_count = 0
		wavefront_sgpr_count = 22
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
	s_load_dwordx4 s[16:19], s[6:7], 0x10
	s_load_dwordx4 s[12:15], s[6:7], 0x20
	s_load_dwordx4 s[0:3], s[6:7], 0x30
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
	s_mul_i32 s0, s13, s16
	v_cmp_gt_i32_e32 vcc, s14, v0
	v_cmp_gt_i32_e64 s[0:1], s0, v1
	s_and_b64 s[0:1], vcc, s[0:1]
	s_and_saveexec_b64 s[2:3], s[0:1]
	s_cbranch_execz BB2_10
; %bb.1:
	s_load_dwordx4 s[4:7], s[6:7], 0x0
	s_cmp_lt_i32 s19, 1
	v_mov_b32_e32 v2, 0xf149f2ca
	s_cbranch_scc1 BB2_9
; %bb.2:                                ; %.preheader.lr.ph
	s_ashr_i32 s2, s13, 31
	s_add_i32 s0, s13, s2
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
	v_xor_b32_e32 v2, v2, v3
	v_sub_u32_e32 v2, vcc, v2, v3
	v_mul_lo_u32 v3, v2, s13
	v_mul_lo_u32 v2, s17, v2
	v_mul_lo_u32 v5, s12, v0
	s_mov_b32 s0, 0
	v_sub_u32_e32 v3, vcc, v1, v3
	v_mul_lo_u32 v3, v3, s12
	v_add_u32_e32 v2, vcc, v3, v2
	v_mul_lo_u32 v4, s18, v2
	v_mov_b32_e32 v2, 0xf149f2ca
	s_branch BB2_4
BB2_3:                                  ; %Flow37
                                        ;   in Loop: Header=BB2_4 Depth=1
	s_or_b64 exec, exec, s[2:3]
	s_add_i32 s0, s0, 1
	s_cmp_lg_u32 s0, s19
	v_add_u32_e32 v4, vcc, s18, v4
	s_cbranch_scc0 BB2_9
BB2_4:                                  ; %.preheader.us
                                        ; =>This Loop Header: Depth=1
                                        ;     Child Loop BB2_7 Depth 2
	v_add_u32_e32 v6, vcc, s0, v3
	v_cmp_gt_i32_e32 vcc, s17, v6
	s_and_saveexec_b64 s[2:3], vcc
	s_cbranch_execz BB2_3
; %bb.5:                                ; %.lr.ph.us.split.us.preheader
                                        ;   in Loop: Header=BB2_4 Depth=1
	s_mov_b32 s1, s19
	v_mov_b32_e32 v6, v5
	s_branch BB2_7
BB2_6:                                  ;   in Loop: Header=BB2_7 Depth=2
	s_or_b64 exec, exec, s[8:9]
	s_add_i32 s1, s1, -1
	s_cmp_lg_u32 s1, 0
	v_add_u32_e32 v6, vcc, 1, v6
	s_cbranch_scc0 BB2_3
BB2_7:                                  ; %.lr.ph.us.split.us
                                        ;   Parent Loop BB2_4 Depth=1
                                        ; =>  This Inner Loop Header: Depth=2
	v_cmp_gt_i32_e32 vcc, s18, v6
	s_and_saveexec_b64 s[8:9], vcc
	s_cbranch_execz BB2_6
; %bb.8:                                ;   in Loop: Header=BB2_7 Depth=2
	v_add_u32_e32 v7, vcc, v4, v6
	v_ashrrev_i32_e32 v8, 31, v7
	v_lshlrev_b64 v[7:8], 2, v[7:8]
	s_waitcnt lgkmcnt(0)
	v_mov_b32_e32 v9, s5
	v_add_u32_e32 v7, vcc, s4, v7
	v_addc_u32_e32 v8, vcc, v9, v8, vcc
	flat_load_dword v7, v[7:8]
	s_waitcnt vmcnt(0) lgkmcnt(0)
	v_cmp_gt_f32_e32 vcc, v7, v2
	v_cndmask_b32_e32 v2, v2, v7, vcc
	s_branch BB2_6
BB2_9:                                  ; %Flow39
	v_mul_lo_u32 v1, v1, s14
	s_waitcnt lgkmcnt(0)
	v_mov_b32_e32 v3, s7
	v_add_u32_e32 v0, vcc, v1, v0
	v_ashrrev_i32_e32 v1, 31, v0
	v_lshlrev_b64 v[0:1], 2, v[0:1]
	v_add_u32_e32 v0, vcc, s6, v0
	v_addc_u32_e32 v1, vcc, v3, v1, vcc
	flat_store_dword v[0:1], v2
BB2_10:
	s_endpgm
.Lfunc_end2:
	.size	ln_maxpool2d, .Lfunc_end2-ln_maxpool2d
                                        ; -- End function
	.section	.AMDGPU.csdata
; Kernel info:
; codeLenInByte = 552
; NumSgprs: 22
; NumVgprs: 10
; ScratchSize: 0
; MemoryBound: 0
; FloatMode: 192
; IeeeMode: 1
; LDSByteSize: 0 bytes/workgroup (compile time only)
; SGPRBlocks: 2
; VGPRBlocks: 2
; NumSGPRsForWavesPerEU: 22
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
	.protected	ln_fc           ; -- Begin function ln_fc
	.globl	ln_fc
	.p2align	8
	.type	ln_fc,@function
	.amdgpu_hsa_kernel ln_fc
ln_fc:                                  ; @ln_fc
ln_fc$local:
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
	s_load_dwordx2 s[10:11], s[6:7], 0x20
	s_load_dwordx2 s[0:1], s[6:7], 0x28
	s_waitcnt lgkmcnt(0)
	s_load_dword s1, s[4:5], 0x4
	s_waitcnt lgkmcnt(0)
	s_and_b32 s1, s1, 0xffff
	s_mul_i32 s8, s8, s1
	v_add_u32_e32 v0, vcc, s8, v0
	v_add_u32_e32 v0, vcc, s0, v0
	v_cmp_gt_i32_e32 vcc, s11, v0
	s_and_saveexec_b64 s[0:1], vcc
	s_cbranch_execz BB3_5
; %bb.1:
	s_load_dwordx8 s[0:7], s[6:7], 0x0
	v_mov_b32_e32 v2, 0
	v_mov_b32_e32 v3, v0
	v_ashrrev_i64 v[2:3], 30, v[2:3]
	v_ashrrev_i32_e32 v1, 31, v0
	s_waitcnt lgkmcnt(0)
	v_mov_b32_e32 v4, s5
	v_add_u32_e32 v2, vcc, s4, v2
	v_addc_u32_e32 v3, vcc, v4, v3, vcc
	flat_load_dword v2, v[2:3]
	s_cmp_lt_i32 s10, 1
	s_cbranch_scc1 BB3_4
; %bb.2:                                ; %.lr.ph.preheader
	v_mul_lo_u32 v3, s10, v0
	v_mov_b32_e32 v5, s3
	v_ashrrev_i32_e32 v4, 31, v3
	v_lshlrev_b64 v[3:4], 2, v[3:4]
	v_add_u32_e32 v3, vcc, s2, v3
	v_addc_u32_e32 v4, vcc, v5, v4, vcc
BB3_3:                                  ; %.lr.ph
                                        ; =>This Inner Loop Header: Depth=1
	flat_load_dword v5, v[3:4]
	s_load_dword s2, s[0:1], 0x0
	s_add_i32 s10, s10, -1
	s_add_u32 s0, s0, 4
	v_add_u32_e32 v3, vcc, 4, v3
	s_addc_u32 s1, s1, 0
	v_addc_u32_e32 v4, vcc, 0, v4, vcc
	s_cmp_eq_u32 s10, 0
	s_waitcnt vmcnt(0) lgkmcnt(0)
	v_mac_f32_e32 v2, s2, v5
	s_cbranch_scc0 BB3_3
BB3_4:                                  ; %Flow22
	v_lshlrev_b64 v[0:1], 2, v[0:1]
	v_mov_b32_e32 v3, s7
	v_add_u32_e32 v0, vcc, s6, v0
	v_addc_u32_e32 v1, vcc, v3, v1, vcc
	s_waitcnt vmcnt(0) lgkmcnt(0)
	flat_store_dword v[0:1], v2
BB3_5:
	s_endpgm
.Lfunc_end3:
	.size	ln_fc, .Lfunc_end3-ln_fc
                                        ; -- End function
	.section	.AMDGPU.csdata
; Kernel info:
; codeLenInByte = 244
; NumSgprs: 14
; NumVgprs: 6
; ScratchSize: 0
; MemoryBound: 0
; FloatMode: 192
; IeeeMode: 1
; LDSByteSize: 0 bytes/workgroup (compile time only)
; SGPRBlocks: 1
; VGPRBlocks: 1
; NumSGPRsForWavesPerEU: 14
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
  - Name:            ln_conv2d
    SymbolName:      'ln_conv2d@kd'
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
      - Name:            W
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
      - Name:            Cin
        TypeName:        int
        Size:            4
        Align:           4
        ValueKind:       ByValue
        ValueType:       I32
        AccQual:         Default
      - Name:            Hin
        TypeName:        int
        Size:            4
        Align:           4
        ValueKind:       ByValue
        ValueType:       I32
        AccQual:         Default
      - Name:            Win
        TypeName:        int
        Size:            4
        Align:           4
        ValueKind:       ByValue
        ValueType:       I32
        AccQual:         Default
      - Name:            Cout
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
      - Name:            Stride
        TypeName:        int
        Size:            4
        Align:           4
        ValueKind:       ByValue
        ValueType:       I32
        AccQual:         Default
      - Name:            Pad
        TypeName:        int
        Size:            4
        Align:           4
        ValueKind:       ByValue
        ValueType:       I32
        AccQual:         Default
      - Name:            Hout
        TypeName:        int
        Size:            4
        Align:           4
        ValueKind:       ByValue
        ValueType:       I32
        AccQual:         Default
      - Name:            Wout
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
      KernargSegmentSize: 128
      GroupSegmentFixedSize: 0
      PrivateSegmentFixedSize: 0
      KernargSegmentAlign: 8
      WavefrontSize:   64
      NumSGPRs:        26
      NumVGPRs:        16
      MaxFlatWorkGroupSize: 256
  - Name:            ln_relu
    SymbolName:      'ln_relu@kd'
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
  - Name:            ln_maxpool2d
    SymbolName:      'ln_maxpool2d@kd'
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
      - Name:            C
        TypeName:        int
        Size:            4
        Align:           4
        ValueKind:       ByValue
        ValueType:       I32
        AccQual:         Default
      - Name:            Hin
        TypeName:        int
        Size:            4
        Align:           4
        ValueKind:       ByValue
        ValueType:       I32
        AccQual:         Default
      - Name:            Win
        TypeName:        int
        Size:            4
        Align:           4
        ValueKind:       ByValue
        ValueType:       I32
        AccQual:         Default
      - Name:            Pool
        TypeName:        int
        Size:            4
        Align:           4
        ValueKind:       ByValue
        ValueType:       I32
        AccQual:         Default
      - Name:            Stride
        TypeName:        int
        Size:            4
        Align:           4
        ValueKind:       ByValue
        ValueType:       I32
        AccQual:         Default
      - Name:            Hout
        TypeName:        int
        Size:            4
        Align:           4
        ValueKind:       ByValue
        ValueType:       I32
        AccQual:         Default
      - Name:            Wout
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
      NumSGPRs:        22
      NumVGPRs:        10
      MaxFlatWorkGroupSize: 256
  - Name:            ln_fc
    SymbolName:      'ln_fc@kd'
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
      - Name:            W
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
      - Name:            InDim
        TypeName:        int
        Size:            4
        Align:           4
        ValueKind:       ByValue
        ValueType:       I32
        AccQual:         Default
      - Name:            OutDim
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
      NumSGPRs:        14
      NumVGPRs:        6
      MaxFlatWorkGroupSize: 256
...

	.end_amd_amdgpu_hsa_metadata
