cmake_minimum_required(VERSION 3.13)
project(mgvm_kernel_analyzer CXX)

find_package(LLVM REQUIRED CONFIG)

message(STATUS "Found LLVM ${LLVM_PACKAGE_VERSION}")
message(STATUS "Using LLVMConfig.cmake in: ${LLVM_DIR}")

# ROCm's LLVM is often built without RTTI/EH; our tool must match.
set(MGVM_DISABLE_RTTI OFF)
set(MGVM_DISABLE_EH OFF)

if(DEFINED LLVM_ENABLE_RTTI)
  if(NOT LLVM_ENABLE_RTTI)
    set(MGVM_DISABLE_RTTI ON)
  endif()
endif()

if(DEFINED LLVM_ENABLE_EH)
  if(NOT LLVM_ENABLE_EH)
    set(MGVM_DISABLE_EH ON)
  endif()
endif()

# Fallback to llvm-config if LLVMConfig did not export RTTI/EH info.
if((NOT DEFINED LLVM_ENABLE_RTTI) OR (NOT DEFINED LLVM_ENABLE_EH))
  find_program(LLVM_CONFIG_EXECUTABLE llvm-config
    HINTS
      ${LLVM_TOOLS_BINARY_DIR}
      ${LLVM_TOOLS_BINARY_DIR}/..
      ${LLVM_DIR}/../../../bin
  )

  if(LLVM_CONFIG_EXECUTABLE)
    if(NOT DEFINED LLVM_ENABLE_RTTI)
      execute_process(
        COMMAND ${LLVM_CONFIG_EXECUTABLE} --has-rtti
        OUTPUT_VARIABLE HAS_RTTI
        OUTPUT_STRIP_TRAILING_WHITESPACE
      )
      if(HAS_RTTI STREQUAL "NO")
        set(MGVM_DISABLE_RTTI ON)
      endif()
    endif()

    if(NOT DEFINED LLVM_ENABLE_EH)
      execute_process(
        COMMAND ${LLVM_CONFIG_EXECUTABLE} --has-eh
        OUTPUT_VARIABLE HAS_EH
        OUTPUT_STRIP_TRAILING_WHITESPACE
      )
      if(HAS_EH STREQUAL "NO")
        set(MGVM_DISABLE_EH ON)
      endif()
    endif()
  endif()
endif()

include_directories(SYSTEM ${LLVM_INCLUDE_DIRS})
add_definitions(${LLVM_DEFINITIONS})

add_executable(mgvm-kernel-analyzer
  src/main.cpp
  src/scev_linear.cpp
)

# Keep this list small; add more components only if you get missing symbols.
llvm_map_components_to_libnames(LLVM_LIBS
  core
  support
  irreader
  analysis
)

target_link_libraries(mgvm-kernel-analyzer PRIVATE ${LLVM_LIBS})

target_compile_features(mgvm-kernel-analyzer PRIVATE cxx_std_17)

if(MGVM_DISABLE_RTTI)
  message(STATUS "LLVM built without RTTI; building analyzer with -fno-rtti")
  target_compile_options(mgvm-kernel-analyzer PRIVATE -fno-rtti)
endif()

if(MGVM_DISABLE_EH)
  message(STATUS "LLVM built without EH; building analyzer with -fno-exceptions")
  target_compile_options(mgvm-kernel-analyzer PRIVATE -fno-exceptions)
endif()
