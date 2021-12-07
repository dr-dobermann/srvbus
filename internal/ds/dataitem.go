package ds

import "io"

// DataTtem is base for Message and Event objects.
//
// DataItem consists a common field like Name and data.
// It also provides the io.Reader interface to read from the data
type DataItem struct {
	Name string
	data []byte

	// readed is used by io.Read function to keep current
	// readed position in data
	readed int
}

// New creates a new DataItem with given name and data.
func NewItem(name string, buf []byte) *DataItem {
	di := new(DataItem)
	di.Name = name
	di.data = make([]byte, len(buf))
	copy(di.data, buf)

	return di
}

// Data returns a copy of di.data.
func (di *DataItem) Data() []byte {
	return append([]byte{}, di.data...)
}

// Copy returns a copy of the whole DataItem.
func (di *DataItem) Copy() *DataItem {
	return &DataItem{
		Name: di.Name,
		data: append([]byte{}, di.data...),
	}
}

// Read implements a io.Reader interface for
// di.data.
func (di *DataItem) Read(p []byte) (n int, err error) {
	// if len(p) == 0 then return 0 and nil according io.Reader description.
	if len(p) == 0 {
		return
	}

	n = copy(p, di.data[di.readed:])

	di.readed += n
	if di.readed == len(di.data) {
		err = io.EOF
	}

	return
}
