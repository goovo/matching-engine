package protocol

import (
	"encoding/binary"
	"fmt"
	"unsafe"
)

// OrderSlot represents a 64-byte aligned structure for RDMA transmission.
// It fits exactly into one CPU cache line.
type OrderSlot [64]byte

// Offsets for fields within the 64-byte slot
const (
	// Header (8B)
	OffsetStatus    = 0
	OffsetChannelID = 1
	OffsetOpCode    = 2
	OffsetVersion   = 3
	OffsetChecksum  = 4 // 4 bytes

	// Body (48B)
	OffsetOrderID  = 8
	OffsetMarketID = 16
	OffsetPrice    = 24
	OffsetAmount   = 32
	OffsetUserID   = 40
	OffsetFlags    = 48 // 2 bytes
	// Padding: 50-55 (6 bytes)

	// Tail (8B)
	OffsetTimestamp = 56
)

// Status Codes
const (
	StatusEmpty      uint8 = 0
	StatusWriting    uint8 = 1 // Optimistic Lock: Writer is filling this slot
	StatusReady      uint8 = 2 // Writer finished, ready for Reader
	StatusProcessing uint8 = 3 // Reader picked it up
	StatusDone       uint8 = 4 // Processed (Implicit ACK)
)

// Accessors using Unsafe Pointer for zero-copy access (simulated)
// In a real RDMA scenario, we would write to the mapped memory directly.

func (s *OrderSlot) SetStatus(status uint8) {
	s[OffsetStatus] = status
}

func (s *OrderSlot) GetStatus() uint8 {
	return s[OffsetStatus]
}

func (s *OrderSlot) Pack(orderID, marketID, price, amount, userID uint64, flags uint16, ts int64) {
	// Header
	s[OffsetChannelID] = 1 // Default Channel
	s[OffsetOpCode] = 1    // New Order
	s[OffsetVersion] = 1

	// Body - Big Endian for network standard, though Host Endian is faster for local shared mem
	// Using BigEndian here to be safe across archs if RDMA goes over wire
	binary.BigEndian.PutUint64(s[OffsetOrderID:], orderID)
	binary.BigEndian.PutUint64(s[OffsetMarketID:], marketID)
	binary.BigEndian.PutUint64(s[OffsetPrice:], price)
	binary.BigEndian.PutUint64(s[OffsetAmount:], amount)
	binary.BigEndian.PutUint64(s[OffsetUserID:], userID)
	binary.BigEndian.PutUint16(s[OffsetFlags:], flags)

	// Tail
	binary.BigEndian.PutUint64(s[OffsetTimestamp:], uint64(ts))
}

// Unpack is used by the Engine to read the slot
func (s *OrderSlot) Unpack() string {
	oid := binary.BigEndian.Uint64(s[OffsetOrderID:])
	uid := binary.BigEndian.Uint64(s[OffsetUserID:])
	price := binary.BigEndian.Uint64(s[OffsetPrice:])
	return fmt.Sprintf("OID:%d UID:%d Price:%d", oid, uid, price)
}

// Direct Unsafe Access (Fastest, Architecture Dependent)
func (s *OrderSlot) PackUnsafe(orderID, marketID, price, amount, userID uint64, flags uint16, ts int64) {
	// Warning: This assumes Little Endian (x86/ARM) usually. 
	// Only safe if both Sender and Receiver have same Endianness.
	
	// Get pointer to start of array
	ptr := unsafe.Pointer(&s[0])
	
	// Offset arithmetic
	*(*uint64)(unsafe.Pointer(uintptr(ptr) + uintptr(OffsetOrderID))) = orderID
	*(*uint64)(unsafe.Pointer(uintptr(ptr) + uintptr(OffsetMarketID))) = marketID
	*(*uint64)(unsafe.Pointer(uintptr(ptr) + uintptr(OffsetPrice))) = price
	*(*uint64)(unsafe.Pointer(uintptr(ptr) + uintptr(OffsetAmount))) = amount
	*(*uint64)(unsafe.Pointer(uintptr(ptr) + uintptr(OffsetUserID))) = userID
	*(*uint16)(unsafe.Pointer(uintptr(ptr) + uintptr(OffsetFlags))) = flags
	*(*int64)(unsafe.Pointer(uintptr(ptr) + uintptr(OffsetTimestamp))) = ts
}
