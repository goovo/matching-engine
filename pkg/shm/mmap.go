package shm

import (
	"fmt"
	"os"
	"syscall"
)

// MappedFile represents a memory mapped file region
type MappedFile struct {
	File *os.File
	Data []byte
	Size int64
}

// OpenOrCreate maps a file to memory.
// If create=true, it truncates/extends the file to size.
func OpenOrCreate(path string, size int64) (*MappedFile, error) {
	flag := os.O_RDWR
	if _, err := os.Stat(path); os.IsNotExist(err) {
		flag |= os.O_CREATE
	}

	f, err := os.OpenFile(path, flag, 0666)
	if err != nil {
		return nil, fmt.Errorf("open file failed: %w", err)
	}

	stat, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}

	if stat.Size() != size {
		if err := f.Truncate(size); err != nil {
			f.Close()
			return nil, fmt.Errorf("truncate failed: %w", err)
		}
	}

	// PROT_READ | PROT_WRITE
	// MAP_SHARED (Updates visible to other processes)
	data, err := syscall.Mmap(int(f.Fd()), 0, int(size), syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("mmap failed: %w", err)
	}

	return &MappedFile{
		File: f,
		Data: data,
		Size: size,
	}, nil
}

func (mf *MappedFile) Close() error {
	if mf.Data != nil {
		syscall.Munmap(mf.Data)
	}
	if mf.File != nil {
		return mf.File.Close()
	}
	return nil
}
