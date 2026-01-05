package queue

import (
	"errors"
	"sync/atomic"
	"unsafe"

	"github.com/goovo/matching-engine/pkg/protocol"
)

// SHMRingBuffer operates on a byte slice backed by shared memory.
// Layout:
// [ Tail (8B) ] [ Padding (56B) ] [ Head (8B) ] [ Padding (56B) ] [ Slots... ]
// Offset 0: Tail (Producer Index)
// Offset 64: Head (Consumer Index)
// Offset 128: Start of Buffer
type SHMRingBuffer struct {
	data []byte
	mask uint64
	base uintptr
}

const (
	OffsetTail      = 0
	OffsetHead      = 64
	OffsetDataStart = 128
	SlotSize        = 64
)

// NewSHMRingBuffer wraps an existing byte slice (from mmap).
// capacity must be power of 2.
func NewSHMRingBuffer(data []byte, capacity uint64) *SHMRingBuffer {
	return &SHMRingBuffer{
		data: data,
		mask: capacity - 1,
		base: uintptr(unsafe.Pointer(&data[0])),
	}
}

// TailAddr returns pointer to Tail index in SHM
func (rb *SHMRingBuffer) TailAddr() *uint64 {
	return (*uint64)(unsafe.Pointer(rb.base + uintptr(OffsetTail)))
}

// HeadAddr returns pointer to Head index in SHM
func (rb *SHMRingBuffer) HeadAddr() *uint64 {
	return (*uint64)(unsafe.Pointer(rb.base + uintptr(OffsetHead)))
}

// Write (Producer)
func (rb *SHMRingBuffer) Write(slot *protocol.OrderSlot) error {
	tailPtr := rb.TailAddr()
	
	// Atomic Increment Tail to claim slot
	// tailVal is the NEW value
	tailVal := atomic.AddUint64(tailPtr, 1)
	currIdx := tailVal - 1
	
	// Calc slot address
	slotIdx := currIdx & rb.mask
	slotOffset := OffsetDataStart + (slotIdx * SlotSize)
	targetPtr := unsafe.Pointer(rb.base + uintptr(slotOffset))
	
	targetSlot := (*protocol.OrderSlot)(targetPtr)

	// Check if slot is free (Simple flow control based on status)
	// Note: In a robust system we should also check Head distance
	status := targetSlot.GetStatus()
	if status != protocol.StatusEmpty && status != protocol.StatusDone {
		return errors.New("ring buffer slot busy")
	}

	// Copy data
	*targetSlot = *slot
	
	// Set Ready
	targetSlot.SetStatus(protocol.StatusReady)
	
	return nil
}

// Poll (Consumer)
func (rb *SHMRingBuffer) Poll() *protocol.OrderSlot {
	headPtr := rb.HeadAddr()
	headVal := atomic.LoadUint64(headPtr)
	
	slotIdx := headVal & rb.mask
	slotOffset := OffsetDataStart + (slotIdx * SlotSize)
	targetPtr := unsafe.Pointer(rb.base + uintptr(slotOffset))
	
	targetSlot := (*protocol.OrderSlot)(targetPtr)

	if targetSlot.GetStatus() == protocol.StatusReady {
		targetSlot.SetStatus(protocol.StatusProcessing)
		atomic.AddUint64(headPtr, 1) // Advance Head
		return targetSlot
	}
	
	return nil
}
