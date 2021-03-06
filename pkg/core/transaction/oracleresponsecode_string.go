// Code generated by "stringer -type=OracleResponseCode"; DO NOT EDIT.

package transaction

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[Success-0]
	_ = x[ProtocolNotSupported-16]
	_ = x[ConsensusUnreachable-18]
	_ = x[NotFound-20]
	_ = x[Timeout-22]
	_ = x[Forbidden-24]
	_ = x[ResponseTooLarge-26]
	_ = x[InsufficientFunds-28]
	_ = x[Error-255]
}

const (
	_OracleResponseCode_name_0 = "Success"
	_OracleResponseCode_name_1 = "ProtocolNotSupported"
	_OracleResponseCode_name_2 = "ConsensusUnreachable"
	_OracleResponseCode_name_3 = "NotFound"
	_OracleResponseCode_name_4 = "Timeout"
	_OracleResponseCode_name_5 = "Forbidden"
	_OracleResponseCode_name_6 = "ResponseTooLarge"
	_OracleResponseCode_name_7 = "InsufficientFunds"
	_OracleResponseCode_name_8 = "Error"
)

func (i OracleResponseCode) String() string {
	switch {
	case i == 0:
		return _OracleResponseCode_name_0
	case i == 16:
		return _OracleResponseCode_name_1
	case i == 18:
		return _OracleResponseCode_name_2
	case i == 20:
		return _OracleResponseCode_name_3
	case i == 22:
		return _OracleResponseCode_name_4
	case i == 24:
		return _OracleResponseCode_name_5
	case i == 26:
		return _OracleResponseCode_name_6
	case i == 28:
		return _OracleResponseCode_name_7
	case i == 255:
		return _OracleResponseCode_name_8
	default:
		return "OracleResponseCode(" + strconv.FormatInt(int64(i), 10) + ")"
	}
}
