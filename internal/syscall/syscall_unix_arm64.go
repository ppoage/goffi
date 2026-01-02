//go:build (linux || darwin) && arm64

// AAPCS64 ABI syscall implementation (Linux, macOS on ARM64)
// ARM64 Procedure Call Standard - identical on all Unix-like systems.
package syscall

import (
	"unsafe"
)

//go:linkname runtime_cgocall runtime.cgocall
func runtime_cgocall(fn uintptr, arg unsafe.Pointer) int32

// syscall15Args matches the layout expected by syscall15 assembly.
// ARM64 AAPCS64 uses X0-X7 (8 GPRs) and D0-D7 (8 FPRs) for arguments.
// Additional integer arguments (a9-a15) are passed on the stack.
//
// Layout (offsets must match assembly):
//
//	fn:     0
//	a1-a8:  8-64   (X0-X7 arguments)
//	a9-a15: 72-120 (stack arguments)
//	f1-f8:  128-184 (D0-D7 arguments)
//	r1-r2:  192-200 (X0-X1 returns)
//	fr1-fr4: 208-232 (D0-D3 float returns for HFA)
//	r8:     240 (X8 - large struct return pointer)
//
// NOTE: f1-f8 and fr1-fr4 are raw bit patterns. For float32 values, the
// lower 32 bits contain the float32 representation (upper 32 bits are ignored).
type syscall15Args struct {
	fn                               uintptr
	a1, a2, a3, a4, a5, a6, a7, a8   uintptr // X0-X7 (offsets 8-64)
	a9, a10, a11, a12, a13, a14, a15 uintptr // stack arguments (offsets 72-120)
	f1, f2, f3, f4, f5, f6, f7, f8   uintptr // D0-D7 arguments (offsets 128-184)
	r1, r2                           uintptr // X0-X1 integer returns (offsets 192-200)
	fr1, fr2, fr3, fr4               uintptr // D0-D3 float returns for HFA (offsets 208-232)
	r8                               uintptr // X8 - large struct return pointer (offset 240)
}

// syscall15 is implemented in syscall_unix_arm64.s
//
//nolint:unused // Called from assembly
func syscall15(args unsafe.Pointer)

// syscall15ABI0 is the ABI0 entry point for syscall15
var syscall15ABI0 uintptr

// Call15Float calls a C function with up to 15 integer arguments and 8 float arguments.
// This follows the AAPCS64 calling convention for ARM64.
//
// fpr contains raw 64-bit bit patterns that will be loaded into D0-D7.
//
// Returns:
//   - r1: X0 integer return value
//   - r2: X1 integer return value (used for 9-16 byte struct returns)
//   - fret: raw 64-bit bit patterns from D0-D3 (used for float returns and HFA)
func Call15Float(fn uintptr, gpr [15]uintptr, fpr [8]uint64, r8 uintptr) (r1 uintptr, r2 uintptr, fret [4]uint64) {
	args := syscall15Args{
		fn: fn,
		a1: gpr[0], a2: gpr[1], a3: gpr[2], a4: gpr[3],
		a5: gpr[4], a6: gpr[5], a7: gpr[6], a8: gpr[7],
		a9: gpr[8], a10: gpr[9], a11: gpr[10], a12: gpr[11],
		a13: gpr[12], a14: gpr[13], a15: gpr[14],
		f1: uintptr(fpr[0]),
		f2: uintptr(fpr[1]),
		f3: uintptr(fpr[2]),
		f4: uintptr(fpr[3]),
		f5: uintptr(fpr[4]),
		f6: uintptr(fpr[5]),
		f7: uintptr(fpr[6]),
		f8: uintptr(fpr[7]),
		r8: r8, // X8 for large struct returns
	}
	runtime_cgocall(syscall15ABI0, unsafe.Pointer(&args))
	r1 = args.r1
	r2 = args.r2
	fret[0] = uint64(args.fr1)
	fret[1] = uint64(args.fr2)
	fret[2] = uint64(args.fr3)
	fret[3] = uint64(args.fr4)
	return
}
