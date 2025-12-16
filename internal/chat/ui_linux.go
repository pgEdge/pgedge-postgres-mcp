//go:build linux

package chat

import (
	"syscall"
	"time"
)

// stdinHasData uses select() syscall to check if stdin has data available
// Returns true if data is available within the timeout
// Linux-specific implementation
func stdinHasData(fd int, timeout time.Duration) bool {
	// Create fd_set for select()
	var readFds syscall.FdSet
	readFds.Bits[fd/64] |= 1 << (uint(fd) % 64)

	// Convert timeout to timeval
	// On Linux, Usec is int64
	tv := syscall.Timeval{
		Sec:  int64(timeout / time.Second),
		Usec: int64((timeout % time.Second) / time.Microsecond),
	}

	// Call select() - on Linux it returns (int, error)
	_, err := syscall.Select(fd+1, &readFds, nil, nil, &tv)
	if err != nil {
		return false
	}
	// Check if fd is still in the set (has data)
	return (readFds.Bits[fd/64] & (1 << (uint(fd) % 64))) != 0
}
