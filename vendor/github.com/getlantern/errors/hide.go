package errors

import (
	"encoding/binary"
	"sync"

	"github.com/getlantern/hidden"
)

var (
	hiddenErrors = make([]hideableError, 100)
	nextID       = uint64(0)
	hiddenMutex  sync.RWMutex
)

type hideableError interface {
	Error
	id() uint64
	setID(uint64)
	setHiddenID(string)
}

// This trick saves the error to a ring buffer and embeds a non-printing
// hiddenID in the error's description, so that if the error is later wrapped
// by a standard error using something like
// fmt.Errorf("An error occurred: %v", thisError), we can subsequently extract
// the error simply using the hiddenID in the string.
func bufferError(e hideableError) {
	hiddenMutex.Lock()
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, nextID)
	e.setID(nextID)
	e.setHiddenID(hidden.ToString(b))
	hiddenErrors[idxForID(nextID)] = e
	nextID++
	hiddenMutex.Unlock()
}

func get(hiddenID []byte) Error {
	if len(hiddenID) != 8 {
		return nil
	}
	id := binary.BigEndian.Uint64(hiddenID)
	hiddenMutex.RLock()
	err := hiddenErrors[idxForID(id)]
	hiddenMutex.RUnlock()
	if err != nil && err.id() == id {
		// Found it!
		return err
	}
	// buffer has rolled over
	return nil
}

func idxForID(id uint64) int {
	return int(id % uint64(len(hiddenErrors)))
}
