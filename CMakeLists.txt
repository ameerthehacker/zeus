cmake_minimum_required(VERSION 3.14)
project(Build_BDWGC_Only C)

# -----------------------------------------------------------------------------
# 1. Output Directories
# This organizes the compiled libraries and binaries directly into third_party/bdwgc/
# -----------------------------------------------------------------------------
set(CMAKE_ARCHIVE_OUTPUT_DIRECTORY "${CMAKE_SOURCE_DIR}/third_party/bdwgc/lib")
set(CMAKE_LIBRARY_OUTPUT_DIRECTORY "${CMAKE_SOURCE_DIR}/third_party/bdwgc/lib")
set(CMAKE_RUNTIME_OUTPUT_DIRECTORY "${CMAKE_SOURCE_DIR}/third_party/bdwgc/bin")

# -----------------------------------------------------------------------------
# 2. bdwgc Configuration Flags
# -----------------------------------------------------------------------------
set(build_tests OFF CACHE INTERNAL "")
set(enable_docs OFF CACHE INTERNAL "")

# -----------------------------------------------------------------------------
# 3. Download and Build v8.2.12
# -----------------------------------------------------------------------------
include(FetchContent)

FetchContent_Declare(
    libatomic_ops
    URL https://github.com/ivmai/libatomic_ops/archive/refs/tags/v7.8.2.tar.gz
    DOWNLOAD_EXTRACT_TIMESTAMP TRUE
)

FetchContent_Declare(
    bdwgc
    URL https://github.com/ivmai/bdwgc/archive/refs/tags/v8.2.12.tar.gz
    DOWNLOAD_EXTRACT_TIMESTAMP TRUE
)

FetchContent_MakeAvailable(libatomic_ops bdwgc)

# -----------------------------------------------------------------------------
# 4. Copy Headers to third_party/bdwgc/include
# -----------------------------------------------------------------------------
file(MAKE_DIRECTORY "${CMAKE_SOURCE_DIR}/third_party/bdwgc/include")
add_custom_target(copy_gc_headers ALL
    COMMAND ${CMAKE_COMMAND} -E copy_directory 
            "${bdwgc_SOURCE_DIR}/include" 
            "${CMAKE_SOURCE_DIR}/third_party/bdwgc/include"
    COMMENT "Copying bdwgc 8.2.12 headers to third_party/bdwgc/include..."
    DEPENDS gc
)