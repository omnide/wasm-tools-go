package cm

import (
	"testing"
	"unsafe"
)

func TestVariantLayout(t *testing.T) {
	// 8 on 64-bit, 4 on 32-bit
	ptrSize := unsafe.Sizeof(uintptr(0))

	tests := []struct {
		name   string
		v      VariantDebug
		size   uintptr
		offset uintptr
	}{
		{"variant { string; string }", Variant[bool, string, string]{}, sizePlusAlignOf[string](), ptrSize},
		{"variant { bool; string }", Variant[bool, string, bool]{}, sizePlusAlignOf[string](), ptrSize},
		{"variant { string; _ }", Variant[bool, string, string]{}, sizePlusAlignOf[string](), ptrSize},
		{"variant { _; _ }", Variant[bool, string, struct{}]{}, sizePlusAlignOf[string](), ptrSize},
		{"variant { u64; u64 }", Variant[bool, uint64, uint64]{}, 16, alignOf[uint64]()},
		{"variant { u32; u64 }", Variant[bool, uint64, uint32]{}, 16, alignOf[uint64]()},
		{"variant { u64; u32 }", Variant[bool, uint64, uint32]{}, 16, alignOf[uint64]()},
		{"variant { u8; u64 }", Variant[bool, uint64, uint8]{}, 16, alignOf[uint64]()},
		{"variant { u64; u8 }", Variant[bool, uint64, uint8]{}, 16, alignOf[uint64]()},
		{"variant { u8; u32 }", Variant[bool, uint32, uint8]{}, 8, alignOf[uint32]()},
		{"variant { u32; u8 }", Variant[bool, uint32, uint8]{}, 8, alignOf[uint32]()},
		{"variant { [9]u8, u64 }", Variant[bool, [9]byte, uint64]{}, 24, alignOf[uint64]()},
	}

	for _, tt := range tests {
		typ := typeName(tt.v)
		t.Run(tt.name, func(t *testing.T) {
			if got, want := tt.v.Size(), tt.size; got != want {
				t.Errorf("(%s).Size() == %v, expected %v", typ, got, want)
			}
			if got, want := tt.v.DataOffset(), tt.offset; got != want {
				t.Errorf("(%s).DataOffset() == %v, expected %v", typ, got, want)
			}
		})
	}
}

func TestGetBoundsCheck(t *testing.T) {
	if !BoundsCheck {
		return // TinyGo does not support t.SkipNow
	}
	defer func() {
		if recover() == nil {
			t.Errorf("Get did not panic")
		}
	}()
	var v Variant[uint8, uint8, uint8]
	_ = Case[string](&v, 0)
}

func TestNewVariantBoundsCheck(t *testing.T) {
	if !BoundsCheck {
		return // TinyGo does not support t.SkipNow
	}
	defer func() {
		if recover() == nil {
			t.Errorf("NewVariant did not panic")
		}
	}()
	_ = NewVariant[uint8, uint8, uint8](0, "hello world")
}
