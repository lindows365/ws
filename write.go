package ws

import (
	"encoding/binary"
	"io"
	"reflect"
	"unsafe"
)

// Header size length bounds in bytes.
const (
	MaxHeaderSize = 14
	MinHeaderSize = 2
)

const (
	bit0 = 0x80
	bit1 = 0x40
	bit2 = 0x20
	bit3 = 0x10
	bit4 = 0x08
	bit5 = 0x04
	bit6 = 0x02
	bit7 = 0x01

	len7  = int64(125)
	len16 = int64(^(uint16(0)))
	len64 = int64(^(uint64(0)) >> 1)
)

// HeaderSize returns number of bytes that are needed to encode given header.
// It returns -1 if header is malformed.
func HeaderSize(h Header) (n int) {
	switch {
	case h.Length < 126:
		n = 2
	case h.Length <= len16:
		n = 4
	case h.Length <= len64:
		n = 10
	default:
		return -1
	}
	if h.Masked {
		n += len(h.Mask)
	}
	return n
}

// WriteHeader writes header binary representation into w.
func WriteHeader(w io.Writer, h Header) error {
	// Make slice of bytes with capacity 14 that could hold any header.
	//
	// We use unsafe to stick bts to stack and avoid allocations.
	//
	// Using stack based slice is safe here, cause golang docs for io.Writer
	// says that "Implementations must not retain p".
	// See https://golang.org/pkg/io/#Writer
	var b [MaxHeaderSize]byte
	bp := uintptr(unsafe.Pointer(&b))
	bh := &reflect.SliceHeader{
		Data: bp,
		Len:  MaxHeaderSize,
		Cap:  MaxHeaderSize,
	}
	bts := *(*[]byte)(unsafe.Pointer(bh))
	_ = bts[MaxHeaderSize-1] // bounds check hint to compiler.

	if h.Fin {
		bts[0] |= bit0
	}
	bts[0] |= h.Rsv << 4
	bts[0] |= byte(h.OpCode)

	var n int
	switch {
	case h.Length <= len7:
		bts[1] = byte(h.Length)
		n = 2

	case h.Length <= len16:
		bts[1] = 126
		binary.BigEndian.PutUint16(bts[2:4], uint16(h.Length))
		n = 4

	case h.Length <= len64:
		bts[1] = 127
		binary.BigEndian.PutUint64(bts[2:10], uint64(h.Length))
		n = 10

	default:
		return ErrHeaderLengthUnexpected
	}

	if h.Masked {
		bts[1] |= bit0
		n += copy(bts[n:], h.Mask[:])
	}

	_, err := w.Write(bts[:n])

	return err
}

// WriteFrame writes frame binary representation into w.
func WriteFrame(w io.Writer, f Frame) error {
	err := WriteHeader(w, f.Header)
	if err != nil {
		return err
	}
	_, err = w.Write(f.Payload)
	return err
}
