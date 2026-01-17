//go:build arm64

package arm64

import (
	"math"
	"reflect"
	"testing"
	"unsafe"

	_ "github.com/go-webgpu/goffi/internal/fakecgo"
	"github.com/go-webgpu/goffi/types"
)

type abiCapture struct {
	GPR [8]uintptr
	FPR [8]uint64
	X8  uintptr
}

type abiStackCapture struct {
	GPR   [8]uintptr
	Stack [7]uintptr
	FPR   [8]uint64
	X8    uintptr
}

// captureABI is implemented in abi_capture_test.s.
//
//go:noescape
func captureABI(out *abiCapture)

// captureStackCABI0 is implemented in abi_capture_test.s and holds the raw entry point.
var captureStackCABI0 uintptr

// returnFloat32ABI0/returnFloat64ABI0/returnVec4ABI0 are implemented in abi_capture_test.s.
var returnFloat32ABI0 uintptr
var returnFloat64ABI0 uintptr
var returnVec4ABI0 uintptr

func prepareReturnCIF(cif *types.CallInterface) {
	cif.Convention = types.UnixCallingConvention
	cif.Flags = classifyReturnARM64(cif.ReturnType, cif.Convention)
}

func captureCall(t *testing.T, argTypes []*types.TypeDescriptor, args []unsafe.Pointer) abiCapture {
	t.Helper()
	var out abiCapture

	argTypes = append([]*types.TypeDescriptor{types.PointerTypeDescriptor}, argTypes...)
	outPtr := uintptr(unsafe.Pointer(&out))
	args = append([]unsafe.Pointer{unsafe.Pointer(&outPtr)}, args...)

	cif := &types.CallInterface{
		ArgCount:   len(argTypes),
		ArgTypes:   argTypes,
		ReturnType: types.VoidTypeDescriptor,
	}

	fnPtr := unsafe.Pointer(reflect.ValueOf(captureABI).Pointer())
	if fnPtr == nil {
		t.Fatalf("captureABI pointer is nil")
	}

	var impl Implementation
	if err := impl.Execute(cif, fnPtr, nil, args); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	return out
}

func captureStackCall(t *testing.T, argTypes []*types.TypeDescriptor, args []unsafe.Pointer) abiStackCapture {
	t.Helper()
	var out abiStackCapture

	argTypes = append([]*types.TypeDescriptor{types.PointerTypeDescriptor}, argTypes...)
	outPtr := uintptr(unsafe.Pointer(&out))
	args = append([]unsafe.Pointer{unsafe.Pointer(&outPtr)}, args...)

	cif := &types.CallInterface{
		ArgCount:   len(argTypes),
		ArgTypes:   argTypes,
		ReturnType: types.VoidTypeDescriptor,
	}

	fnPtr := unsafe.Pointer(captureStackCABI0)
	if fnPtr == nil {
		t.Fatalf("captureStackCABI0 pointer is nil")
	}

	var impl Implementation
	if err := impl.Execute(cif, fnPtr, nil, args); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	return out
}

func TestExecuteCaptureRegistersSimple(t *testing.T) {
	intPatterns := []uint64{
		0x0,
		0x1,
		0xFFFFFFFFFFFFFFFF,
		0x0123456789ABCDEF,
		0xFEDCBA9876543210,
		0x1111111122222222,
		0x3333333344444444,
		0x5555555566666666,
		0x7777777788888888,
	}
	floatBits := []uint64{
		0x0000000000000000, // +0
		0x8000000000000000, // -0
		0x3FF0000000000000, // 1.0
		0xBFF0000000000000, // -1.0
		0x4008000000000000, // 3.0
		0xC008000000000000, // -3.0
		0x7FEFFFFFFFFFFFFF, // max finite
		0x0010000000000000, // min normal
	}

	for i := 0; i+3 < len(intPatterns); i++ {
		a1 := intPatterns[i]
		a2 := intPatterns[i+1]
		a3 := intPatterns[i+2]
		a4 := intPatterns[i+3]
		f0 := math.Float64frombits(floatBits[i%len(floatBits)])
		f1 := math.Float64frombits(floatBits[(i+3)%len(floatBits)])

		out := captureCall(
			t,
			[]*types.TypeDescriptor{
				types.UInt64TypeDescriptor,
				types.UInt64TypeDescriptor,
				types.UInt64TypeDescriptor,
				types.UInt64TypeDescriptor,
				types.DoubleTypeDescriptor,
				types.DoubleTypeDescriptor,
			},
			[]unsafe.Pointer{
				unsafe.Pointer(&a1),
				unsafe.Pointer(&a2),
				unsafe.Pointer(&a3),
				unsafe.Pointer(&a4),
				unsafe.Pointer(&f0),
				unsafe.Pointer(&f1),
			},
		)

		if out.GPR[1] != uintptr(a1) || out.GPR[2] != uintptr(a2) ||
			out.GPR[3] != uintptr(a3) || out.GPR[4] != uintptr(a4) {
			t.Fatalf("GPR mismatch: %x %x %x %x", out.GPR[1], out.GPR[2], out.GPR[3], out.GPR[4])
		}
		if out.FPR[0] != math.Float64bits(f0) || out.FPR[1] != math.Float64bits(f1) {
			t.Fatalf("FPR mismatch: 0x%x 0x%x", out.FPR[0], out.FPR[1])
		}
	}
}

func TestExecuteCaptureStructHFA(t *testing.T) {
	type Vec4 struct {
		A float64
		B float64
		C float64
		D float64
	}

	desc := &types.TypeDescriptor{
		Kind:      types.StructType,
		Alignment: 8,
		Members: []*types.TypeDescriptor{
			types.DoubleTypeDescriptor,
			types.DoubleTypeDescriptor,
			types.DoubleTypeDescriptor,
			types.DoubleTypeDescriptor,
		},
	}

	patterns := []Vec4{
		{A: 1.0, B: 2.0, C: 3.0, D: 4.0},
		{A: -1.0, B: -2.0, C: 0.5, D: 0.25},
		{A: math.Float64frombits(0x7FEFFFFFFFFFFFFF), B: 0, C: -0, D: 1},
	}

	for _, val := range patterns {
		out := captureCall(
			t,
			[]*types.TypeDescriptor{desc},
			[]unsafe.Pointer{unsafe.Pointer(&val)},
		)

		if out.GPR[1] != 0 {
			t.Fatalf("expected no GPR usage for HFA, got 0x%x", out.GPR[1])
		}
		if out.FPR[0] != math.Float64bits(val.A) ||
			out.FPR[1] != math.Float64bits(val.B) ||
			out.FPR[2] != math.Float64bits(val.C) ||
			out.FPR[3] != math.Float64bits(val.D) {
			t.Fatalf("unexpected FPR contents for HFA")
		}
	}
}

func TestExecuteCaptureStructMixedSmall(t *testing.T) {
	type Mixed struct {
		A uint32
		B float32
	}

	desc := &types.TypeDescriptor{
		Kind:      types.StructType,
		Alignment: 4,
		Members: []*types.TypeDescriptor{
			types.UInt32TypeDescriptor,
			types.FloatTypeDescriptor,
		},
	}

	patterns := []Mixed{
		{A: 0x11223344, B: 1.5},
		{A: 0xFFFFFFFF, B: -2.0},
		{A: 0x0, B: math.Float32frombits(0x7F7FFFFF)},
	}

	for _, val := range patterns {
		out := captureCall(
			t,
			[]*types.TypeDescriptor{desc},
			[]unsafe.Pointer{unsafe.Pointer(&val)},
		)

		want := uint64(val.A) | (uint64(math.Float32bits(val.B)) << 32)
		if uint64(out.GPR[1]) != want {
			t.Fatalf("packed GPR mismatch: 0x%x, want 0x%x", uint64(out.GPR[1]), want)
		}
		if out.FPR[0] != 0 {
			t.Fatalf("expected no FPR usage for mixed small struct, got 0x%x", out.FPR[0])
		}
	}
}

func TestExecuteCaptureStructByRefLarge(t *testing.T) {
	type Large struct {
		A uint64
		B uint64
		C uint64
	}

	desc := &types.TypeDescriptor{
		Kind: types.StructType,
		Members: []*types.TypeDescriptor{
			types.UInt64TypeDescriptor,
			types.UInt64TypeDescriptor,
			types.UInt64TypeDescriptor,
		},
	}

	patterns := []Large{
		{A: 1, B: 2, C: 3},
		{A: 0xFFFFFFFFFFFFFFFF, B: 0, C: 0xAAAAAAAAAAAAAAAA},
		{A: 0x0123456789ABCDEF, B: 0xFEDCBA9876543210, C: 0},
	}

	for _, val := range patterns {
		out := captureCall(
			t,
			[]*types.TypeDescriptor{desc},
			[]unsafe.Pointer{unsafe.Pointer(&val)},
		)

		if out.GPR[1] == 0 {
			t.Fatalf("by-ref GPR is zero")
		}
		if out.GPR[1] == uintptr(unsafe.Pointer(&val)) {
			t.Fatalf("expected by-ref copy pointer, got original: 0x%x", out.GPR[1])
		}
	}
}

func TestExecuteCaptureStackArgs(t *testing.T) {
	values := []uint64{
		0x1111111122222222,
		0x3333333344444444,
		0x5555555566666666,
		0x7777777788888888,
		0x99999999aaaaaaaa,
		0xbbbbbbbbcccccccc,
		0xddddddddeeeeeeee,
		0x0123456789abcdef,
		0xfedcba9876543210,
		0x0f0f0f0f0f0f0f0f,
		0x1f1f1f1f1f1f1f1f,
	}

	argTypes := make([]*types.TypeDescriptor, 0, len(values))
	args := make([]unsafe.Pointer, 0, len(values))
	for i := range values {
		argTypes = append(argTypes, types.UInt64TypeDescriptor)
		args = append(args, unsafe.Pointer(&values[i]))
	}

	out := captureStackCall(t, argTypes, args)

	for i := 0; i < 7; i++ {
		if out.GPR[i+1] != uintptr(values[i]) {
			t.Fatalf("GPR[%d] = 0x%x, want 0x%x", i+1, out.GPR[i+1], values[i])
		}
	}
	if out.Stack[0] != uintptr(values[7]) ||
		out.Stack[1] != uintptr(values[8]) ||
		out.Stack[2] != uintptr(values[9]) ||
		out.Stack[3] != uintptr(values[10]) {
		t.Fatalf("stack args = [0x%x 0x%x 0x%x 0x%x], want [0x%x 0x%x 0x%x 0x%x]",
			out.Stack[0], out.Stack[1], out.Stack[2], out.Stack[3],
			values[7], values[8], values[9], values[10])
	}
}

func TestExecuteCaptureStackFloat64Args(t *testing.T) {
	values := []float64{
		0.0,
		-0.0,
		1.0,
		-1.0,
		3.5,
		-4.25,
		math.Float64frombits(0x7FEFFFFFFFFFFFFF),
		math.Float64frombits(0x0010000000000000),
		math.Float64frombits(0x400921FB54442D18),
	}

	argTypes := make([]*types.TypeDescriptor, 0, len(values))
	args := make([]unsafe.Pointer, 0, len(values))
	for i := range values {
		argTypes = append(argTypes, types.DoubleTypeDescriptor)
		args = append(args, unsafe.Pointer(&values[i]))
	}

	out := captureStackCall(t, argTypes, args)

	for i := 0; i < 8; i++ {
		if out.FPR[i] != math.Float64bits(values[i]) {
			t.Fatalf("FPR[%d] = 0x%x, want 0x%x", i, out.FPR[i], math.Float64bits(values[i]))
		}
	}
	if out.Stack[0] != uintptr(math.Float64bits(values[8])) {
		t.Fatalf("stack float arg = 0x%x, want 0x%x", out.Stack[0], math.Float64bits(values[8]))
	}
}

func TestExecuteCaptureStackFloat32Args(t *testing.T) {
	values := []float32{
		0.0,
		-0.0,
		1.0,
		-1.0,
		3.5,
		-4.25,
		math.Float32frombits(0x7F7FFFFF),
		math.Float32frombits(0x00800000),
		math.Float32frombits(0x40490FDB),
	}

	argTypes := make([]*types.TypeDescriptor, 0, len(values))
	args := make([]unsafe.Pointer, 0, len(values))
	for i := range values {
		argTypes = append(argTypes, types.FloatTypeDescriptor)
		args = append(args, unsafe.Pointer(&values[i]))
	}

	out := captureStackCall(t, argTypes, args)

	for i := 0; i < 8; i++ {
		want := uint64(math.Float32bits(values[i]))
		if out.FPR[i] != want {
			t.Fatalf("FPR[%d] = 0x%x, want 0x%x", i, out.FPR[i], want)
		}
	}
	want := uintptr(math.Float32bits(values[8]))
	if out.Stack[0] != want {
		t.Fatalf("stack float arg = 0x%x, want 0x%x", out.Stack[0], want)
	}
}

func TestExecuteReturnFloat32(t *testing.T) {
	values := []uint32{
		0x00000000,
		0x80000000,
		0x3F800000,
		0xBF800000,
		0x7F7FFFFF,
		0x7FC00001,
	}

	if returnFloat32ABI0 == 0 {
		t.Fatal("returnFloat32ABI0 pointer is nil")
	}

	for _, bits := range values {
		in := math.Float32frombits(bits)
		var out float32
		cif := &types.CallInterface{
			ArgCount:   1,
			ArgTypes:   []*types.TypeDescriptor{types.FloatTypeDescriptor},
			ReturnType: types.FloatTypeDescriptor,
		}
		prepareReturnCIF(cif)

		var impl Implementation
		if err := impl.Execute(cif, unsafe.Pointer(returnFloat32ABI0), unsafe.Pointer(&out), []unsafe.Pointer{unsafe.Pointer(&in)}); err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if math.Float32bits(out) != bits {
			t.Fatalf("return float32 = 0x%x, want 0x%x", math.Float32bits(out), bits)
		}
	}
}

func TestExecuteReturnFloat64(t *testing.T) {
	values := []uint64{
		0x0000000000000000,
		0x8000000000000000,
		0x3FF0000000000000,
		0xBFF0000000000000,
		0x7FEFFFFFFFFFFFFF,
		0x7FF8000000000001,
	}

	if returnFloat64ABI0 == 0 {
		t.Fatal("returnFloat64ABI0 pointer is nil")
	}

	for _, bits := range values {
		in := math.Float64frombits(bits)
		var out float64
		cif := &types.CallInterface{
			ArgCount:   1,
			ArgTypes:   []*types.TypeDescriptor{types.DoubleTypeDescriptor},
			ReturnType: types.DoubleTypeDescriptor,
		}
		prepareReturnCIF(cif)

		var impl Implementation
		if err := impl.Execute(cif, unsafe.Pointer(returnFloat64ABI0), unsafe.Pointer(&out), []unsafe.Pointer{unsafe.Pointer(&in)}); err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if math.Float64bits(out) != bits {
			t.Fatalf("return float64 = 0x%x, want 0x%x", math.Float64bits(out), bits)
		}
	}
}

func TestExecuteReturnHFA(t *testing.T) {
	type Vec4 struct {
		A float64
		B float64
		C float64
		D float64
	}

	desc := &types.TypeDescriptor{
		Kind:      types.StructType,
		Alignment: 8,
		Members: []*types.TypeDescriptor{
			types.DoubleTypeDescriptor,
			types.DoubleTypeDescriptor,
			types.DoubleTypeDescriptor,
			types.DoubleTypeDescriptor,
		},
	}

	if returnVec4ABI0 == 0 {
		t.Fatal("returnVec4ABI0 pointer is nil")
	}

	patterns := []Vec4{
		{A: 1.0, B: 2.0, C: 3.0, D: 4.0},
		{A: -1.0, B: -2.0, C: 0.5, D: 0.25},
		{A: math.Float64frombits(0x7FEFFFFFFFFFFFFF), B: 0, C: -0, D: 1},
	}

	for _, val := range patterns {
		var out Vec4
		cif := &types.CallInterface{
			ArgCount:   1,
			ArgTypes:   []*types.TypeDescriptor{desc},
			ReturnType: desc,
		}
		prepareReturnCIF(cif)

		var impl Implementation
		if err := impl.Execute(cif, unsafe.Pointer(returnVec4ABI0), unsafe.Pointer(&out), []unsafe.Pointer{unsafe.Pointer(&val)}); err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if math.Float64bits(out.A) != math.Float64bits(val.A) ||
			math.Float64bits(out.B) != math.Float64bits(val.B) ||
			math.Float64bits(out.C) != math.Float64bits(val.C) ||
			math.Float64bits(out.D) != math.Float64bits(val.D) {
			t.Fatalf("HFA return mismatch: got %+v want %+v", out, val)
		}
	}
}
