package deflect

// git pack index <4GB
type object32 struct {
	offset uint32
	index  uint32
}

// git pack index >4GB
type object64 struct {
	offset int64
	index  uint32
}

// Offset arranged in ascending order, and then subtracted one by one to get the rough size of the object

type object32s []object32

// Len len exports
func (o object32s) Len() int { return len(o) }

// Less less
func (o object32s) Less(i, j int) bool { return o[i].offset > o[j].offset }

// Swap function
func (o object32s) Swap(i, j int) { o[i], o[j] = o[j], o[i] }

type object64s []object64

// Len len exports
func (o object64s) Len() int { return len(o) }

// Less less
func (o object64s) Less(i, j int) bool { return o[i].offset > o[j].offset }

// Swap function
func (o object64s) Swap(i, j int) { o[i], o[j] = o[j], o[i] }
