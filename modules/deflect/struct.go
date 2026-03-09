package deflect

// object32 represents an object in pack files with 32-bit offsets (< 4GB)
// The offset and index are used to sort objects by position in pack file
type object32 struct {
	offset uint32 // Object offset in pack file (32-bit)
	index  uint32 // Original object index in pack index file
}

// object64 represents an object in pack files with 64-bit offsets (>= 4GB)
// Used for large pack files where 32-bit offsets are insufficient
type object64 struct {
	offset int64  // Object offset in pack file (64-bit)
	index  uint32 // Original object index in pack index file
}

// Object size calculation strategy:
// Offsets are arranged in ascending order, then subtracted one by one
// to estimate the rough size of each object in the pack file.
// This provides size estimation without decompressing each object.

type object32s []object32

// Len implements sort.Interface for object32s
func (o object32s) Len() int { return len(o) }

// Less implements sort.Interface for object32s
// Descending order by offset (largest offset first)
func (o object32s) Less(i, j int) bool { return o[i].offset > o[j].offset }

// Swap implements sort.Interface for object32s
func (o object32s) Swap(i, j int) { o[i], o[j] = o[j], o[i] }

type object64s []object64

// Len implements sort.Interface for object64s
func (o object64s) Len() int { return len(o) }

// Less implements sort.Interface for object64s
// Descending order by offset (largest offset first)
func (o object64s) Less(i, j int) bool { return o[i].offset > o[j].offset }

// Swap implements sort.Interface for object64s
func (o object64s) Swap(i, j int) { o[i], o[j] = o[j], o[i] }
