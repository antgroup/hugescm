package item

import (
	"math"
)

// clampIntToUint8 safely converts an int to uint8, clamping to valid range
func clampIntToUint8(val int) uint8 {
	if val < 0 {
		return 0
	}
	if val > math.MaxUint8 {
		return math.MaxUint8
	}
	return uint8(val)
}

// clampIntToUint32 safely converts an int to uint32, clamping to valid range
func clampIntToUint32(val int) uint32 {
	if val < 0 {
		return 0
	}
	if uint64(val) > math.MaxUint32 {
		return math.MaxUint32
	}
	return uint32(val)
}
