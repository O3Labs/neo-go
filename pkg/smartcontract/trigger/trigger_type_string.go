// Code generated by "stringer -type=Type -output=trigger_type_string.go"; DO NOT EDIT.

package trigger

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[OnPersist-1]
	_ = x[PostPersist-2]
	_ = x[Verification-32]
	_ = x[Application-64]
	_ = x[All-99]
}

const (
	_Type_name_0 = "OnPersistPostPersist"
	_Type_name_1 = "Verification"
	_Type_name_2 = "Application"
	_Type_name_3 = "All"
)

var (
	_Type_index_0 = [...]uint8{0, 9, 20}
)

func (i Type) String() string {
	switch {
	case 1 <= i && i <= 2:
		i -= 1
		return _Type_name_0[_Type_index_0[i]:_Type_index_0[i+1]]
	case i == 32:
		return _Type_name_1
	case i == 64:
		return _Type_name_2
	case i == 99:
		return _Type_name_3
	default:
		return "Type(" + strconv.FormatInt(int64(i), 10) + ")"
	}
}
