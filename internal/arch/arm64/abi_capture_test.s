//go:build arm64

#include "textflag.h"

// captureABI stores X0-X7, D0-D7, and X8 into the provided abiCapture buffer.
// The output pointer is expected in X0.
TEXT ·captureABI(SB), NOSPLIT|NOFRAME, $0-8
	MOVD R0, R9

	// GPRs
	MOVD R0, 0(R9)
	MOVD R1, 8(R9)
	MOVD R2, 16(R9)
	MOVD R3, 24(R9)
	MOVD R4, 32(R9)
	MOVD R5, 40(R9)
	MOVD R6, 48(R9)
	MOVD R7, 56(R9)

	// FPRs
	FMOVD F0, 64(R9)
	FMOVD F1, 72(R9)
	FMOVD F2, 80(R9)
	FMOVD F3, 88(R9)
	FMOVD F4, 96(R9)
	FMOVD F5, 104(R9)
	FMOVD F6, 112(R9)
	FMOVD F7, 120(R9)

	// X8 (sret)
	MOVD R8, 128(R9)

	RET

// captureStackC stores X0-X7, stack args (a9-a15), D0-D7, and X8 into the provided buffer.
// The output pointer is expected in X0. The caller must not adjust SP.
// This symbol is invoked via a raw pointer to avoid ABI wrappers.
TEXT ·captureStackC(SB), NOSPLIT|NOFRAME, $0-8
	MOVD R0, R9

	// GPRs
	MOVD R0, 0(R9)
	MOVD R1, 8(R9)
	MOVD R2, 16(R9)
	MOVD R3, 24(R9)
	MOVD R4, 32(R9)
	MOVD R5, 40(R9)
	MOVD R6, 48(R9)
	MOVD R7, 56(R9)

	// Stack arguments a9-a15 at SP+0..48
	MOVD 0(RSP), R10
	MOVD R10, 64(R9)
	MOVD 8(RSP), R10
	MOVD R10, 72(R9)
	MOVD 16(RSP), R10
	MOVD R10, 80(R9)
	MOVD 24(RSP), R10
	MOVD R10, 88(R9)
	MOVD 32(RSP), R10
	MOVD R10, 96(R9)
	MOVD 40(RSP), R10
	MOVD R10, 104(R9)
	MOVD 48(RSP), R10
	MOVD R10, 112(R9)

	// FPRs
	FMOVD F0, 120(R9)
	FMOVD F1, 128(R9)
	FMOVD F2, 136(R9)
	FMOVD F3, 144(R9)
	FMOVD F4, 152(R9)
	FMOVD F5, 160(R9)
	FMOVD F6, 168(R9)
	FMOVD F7, 176(R9)

	// X8 (sret)
	MOVD R8, 184(R9)

	RET

// captureStackCABI0 exposes the raw entry point for captureStackC.
GLOBL ·captureStackCABI0(SB), NOPTR|RODATA, $8
DATA ·captureStackCABI0(SB)/8, $·captureStackC(SB)
