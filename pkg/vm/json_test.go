package vm

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/encoding/bigint"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

type (
	vmUT struct {
		Category string      `json:"category"`
		Name     string      `json:"name"`
		Tests    []vmUTEntry `json:"tests"`
	}

	vmUTActionType string

	vmUTEntry struct {
		Name   string
		Script vmUTScript
		Steps  []vmUTStep
	}

	vmUTExecutionContextState struct {
		Instruction        string          `json:"nextInstruction"`
		InstructionPointer int             `json:"instructionPointer"`
		EStack             []vmUTStackItem `json:"evaluationStack"`
		StaticFields       []vmUTStackItem `json:"staticFields"`
	}

	vmUTExecutionEngineState struct {
		State           State                       `json:"state"`
		ResultStack     []vmUTStackItem             `json:"resultStack"`
		InvocationStack []vmUTExecutionContextState `json:"invocationStack"`
	}

	vmUTScript []byte

	vmUTStackItem struct {
		Type  vmUTStackItemType
		Value interface{}
	}

	vmUTStep struct {
		Actions []vmUTActionType         `json:"actions"`
		Result  vmUTExecutionEngineState `json:"result"`
	}

	vmUTStackItemType string
)

// stackItemAUX is used as an intermediate structure
// to conditionally unmarshal vmUTStackItem based
// on the value of Type field.
type stackItemAUX struct {
	Type  vmUTStackItemType `json:"type"`
	Value json.RawMessage   `json:"value"`
}

const (
	vmExecute  vmUTActionType = "execute"
	vmStepInto vmUTActionType = "stepinto"
	vmStepOut  vmUTActionType = "stepout"
	vmStepOver vmUTActionType = "stepover"

	typeArray      vmUTStackItemType = "array"
	typeBoolean    vmUTStackItemType = "boolean"
	typeBuffer     vmUTStackItemType = "buffer"
	typeByteString vmUTStackItemType = "bytestring"
	typeInteger    vmUTStackItemType = "integer"
	typeInterop    vmUTStackItemType = "interop"
	typeMap        vmUTStackItemType = "map"
	typeNull       vmUTStackItemType = "null"
	typePointer    vmUTStackItemType = "pointer"
	typeString     vmUTStackItemType = "string"
	typeStruct     vmUTStackItemType = "struct"

	testsDir = "testdata/neo-vm/tests/neo-vm.Tests/Tests/"
)

func TestUT(t *testing.T) {
	testsRan := false
	err := filepath.Walk(testsDir, func(path string, info os.FileInfo, err error) error {
		if !strings.HasSuffix(path, ".json") {
			return nil
		}

		testFile(t, path)
		testsRan = true
		return nil
	})

	require.NoError(t, err)
	require.Equal(t, true, testsRan, "neo-vm tests should be available (check submodules)")
}

func getTestingInterop(id uint32) *InteropFuncPrice {
	f := func(v *VM) error {
		v.estack.PushVal(stackitem.NewInterop(new(int)))
		return nil
	}
	switch id {
	case 0x77777777:
		return &InteropFuncPrice{Func: f}
	case 0x66666666:
		return &InteropFuncPrice{
			Func:          f,
			RequiredFlags: smartcontract.ReadOnly,
		}
	case 0x55555555:
		return &InteropFuncPrice{
			Func: f,
		}
	case 0xADDEADDE:
		return &InteropFuncPrice{
			Func: func(v *VM) error {
				v.throw(stackitem.Make("error"))
				return nil
			},
		}
	}
	return nil
}

func testFile(t *testing.T, filename string) {
	data, err := ioutil.ReadFile(filename)
	require.NoError(t, err)

	// get rid of possible BOM
	if len(data) > 2 && data[0] == 0xef && data[1] == 0xbb && data[2] == 0xbf {
		data = data[3:]
	}
	if strings.HasSuffix(filename, "MEMCPY.json") {
		return // FIXME not a valid JSON https://github.com/neo-project/neo-vm/issues/322
	}

	ut := new(vmUT)
	require.NoErrorf(t, json.Unmarshal(data, ut), "file: %s", filename)

	t.Run(ut.Category+":"+ut.Name, func(t *testing.T) {
		for i := range ut.Tests {
			test := ut.Tests[i]
			if test.Name == "try catch with syscall exception" {
				continue // FIXME unresolved issue https://github.com/neo-project/neo-vm/issues/343
			}
			t.Run(ut.Tests[i].Name, func(t *testing.T) {
				prog := []byte(test.Script)
				vm := load(prog)
				vm.state = BreakState
				vm.RegisterInteropGetter(getTestingInterop)

				for i := range test.Steps {
					execStep(t, vm, test.Steps[i])
					result := test.Steps[i].Result
					require.Equal(t, result.State, vm.state)
					if result.State == FaultState { // do not compare stacks on fault
						continue
					}

					if len(result.InvocationStack) > 0 {
						for i, s := range result.InvocationStack {
							ctx := vm.istack.Peek(i).Value().(*Context)
							if ctx.nextip < len(ctx.prog) {
								require.Equal(t, s.InstructionPointer, ctx.nextip)
								op, err := opcode.FromString(s.Instruction)
								require.NoError(t, err)
								require.Equal(t, op, opcode.Opcode(ctx.prog[ctx.nextip]))
							}
							compareStacks(t, s.EStack, vm.estack)
							compareSlots(t, s.StaticFields, ctx.static)
						}
					}

					if len(result.ResultStack) != 0 {
						compareStacks(t, result.ResultStack, vm.estack)
					}
				}
			})
		}
	})
}

func compareItems(t *testing.T, a, b stackitem.Item) {
	switch si := a.(type) {
	case *stackitem.BigInteger:
		val := si.Value().(*big.Int).Int64()
		switch ac := b.(type) {
		case *stackitem.BigInteger:
			require.Equal(t, val, ac.Value().(*big.Int).Int64())
		case *stackitem.ByteArray:
			require.Equal(t, val, bigint.FromBytes(ac.Value().([]byte)).Int64())
		case *stackitem.Bool:
			if ac.Value().(bool) {
				require.Equal(t, val, int64(1))
			} else {
				require.Equal(t, val, int64(0))
			}
		default:
			require.Fail(t, "wrong type")
		}
	case *stackitem.Pointer:
		p, ok := b.(*stackitem.Pointer)
		require.True(t, ok)
		require.Equal(t, si.Position(), p.Position()) // there no script in test files
	default:
		require.Equal(t, a, b)
	}
}

func compareStacks(t *testing.T, expected []vmUTStackItem, actual *Stack) {
	compareItemArrays(t, expected, actual.Len(), func(i int) stackitem.Item { return actual.Peek(i).Item() })
}

func compareSlots(t *testing.T, expected []vmUTStackItem, actual *Slot) {
	if actual.storage == nil && len(expected) == 0 {
		return
	}
	require.NotNil(t, actual)
	compareItemArrays(t, expected, actual.Size(), actual.Get)
}

func compareItemArrays(t *testing.T, expected []vmUTStackItem, n int, getItem func(i int) stackitem.Item) {
	if expected == nil {
		return
	}

	require.Equal(t, len(expected), n)
	for i, item := range expected {
		it := getItem(i)
		require.NotNil(t, it)

		if item.Type == typeInterop {
			require.IsType(t, (*stackitem.Interop)(nil), it)
			continue
		}
		compareItems(t, item.toStackItem(), it)
	}
}

func (v *vmUTStackItem) toStackItem() stackitem.Item {
	switch v.Type.toLower() {
	case typeArray:
		items := v.Value.([]vmUTStackItem)
		result := make([]stackitem.Item, len(items))
		for i := range items {
			result[i] = items[i].toStackItem()
		}
		return stackitem.NewArray(result)
	case typeString:
		panic("not implemented")
	case typeMap:
		return v.Value.(*stackitem.Map)
	case typeInterop:
		panic("not implemented")
	case typeByteString:
		return stackitem.NewByteArray(v.Value.([]byte))
	case typeBuffer:
		return stackitem.NewBuffer(v.Value.([]byte))
	case typePointer:
		return stackitem.NewPointer(v.Value.(int), nil)
	case typeNull:
		return stackitem.Null{}
	case typeBoolean:
		return stackitem.NewBool(v.Value.(bool))
	case typeInteger:
		return stackitem.NewBigInteger(v.Value.(*big.Int))
	case typeStruct:
		items := v.Value.([]vmUTStackItem)
		result := make([]stackitem.Item, len(items))
		for i := range items {
			result[i] = items[i].toStackItem()
		}
		return stackitem.NewStruct(result)
	default:
		panic(fmt.Sprintf("invalid type: %s", v.Type))
	}
}

func execStep(t *testing.T, v *VM, step vmUTStep) {
	for i, a := range step.Actions {
		var err error
		switch a.toLower() {
		case vmExecute:
			err = v.Run()
		case vmStepInto:
			err = v.StepInto()
		case vmStepOut:
			err = v.StepOut()
		case vmStepOver:
			err = v.StepOver()
		default:
			panic(fmt.Sprintf("invalid action: %s", a))
		}

		// only the last action is allowed to fail
		if i+1 < len(step.Actions) {
			require.NoError(t, err)
		}
	}
}

func jsonStringToInteger(s string) stackitem.Item {
	b, err := decodeHex(s)
	if err == nil {
		return stackitem.NewBigInteger(new(big.Int).SetBytes(b))
	}
	return nil
}

func (v vmUTStackItemType) toLower() vmUTStackItemType {
	return vmUTStackItemType(strings.ToLower(string(v)))
}

func (v *vmUTScript) UnmarshalJSON(data []byte) error {
	var ops []string
	if err := json.Unmarshal(data, &ops); err != nil {
		return err
	}

	var script []byte
	for i := range ops {
		if b, ok := decodeSingle(ops[i]); ok {
			script = append(script, b...)
		} else {
			const regex = `(?P<hex>(?:0x)?[0-9a-zA-Z]+)\*(?P<num>[0-9]+)`
			re := regexp.MustCompile(regex)
			ss := re.FindStringSubmatch(ops[i])
			if len(ss) != 3 {
				return fmt.Errorf("invalid script part: %s", ops[i])
			}
			b, ok := decodeSingle(ss[1])
			if !ok {
				return fmt.Errorf("invalid script part: %s", ops[i])
			}
			num, err := strconv.Atoi(ss[2])
			if err != nil {
				return fmt.Errorf("invalid script part: %s", ops[i])
			}
			for i := 0; i < num; i++ {
				script = append(script, b...)
			}
		}
	}

	*v = script
	return nil
}

func decodeSingle(s string) ([]byte, bool) {
	if op, err := opcode.FromString(s); err == nil {
		return []byte{byte(op)}, true
	}
	b, err := decodeHex(s)
	return b, err == nil
}

func (v vmUTActionType) toLower() vmUTActionType {
	return vmUTActionType(strings.ToLower(string(v)))
}

func (v *vmUTActionType) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, (*string)(v))
}

func (v *vmUTStackItem) UnmarshalJSON(data []byte) error {
	var si stackItemAUX
	if err := json.Unmarshal(data, &si); err != nil {
		return err
	}

	v.Type = si.Type

	switch typ := si.Type.toLower(); typ {
	case typeArray, typeStruct:
		var a []vmUTStackItem
		if err := json.Unmarshal(si.Value, &a); err != nil {
			return err
		}
		v.Value = a
	case typeInteger, typePointer:
		num := new(big.Int)
		var a int64
		var s string
		if err := json.Unmarshal(si.Value, &a); err == nil {
			num.SetInt64(a)
		} else if err := json.Unmarshal(si.Value, &s); err == nil {
			num.SetString(s, 10)
		} else {
			panic(fmt.Sprintf("invalid integer: %v", si.Value))
		}
		if typ == typePointer {
			v.Value = int(num.Int64())
		} else {
			v.Value = num
		}
	case typeBoolean:
		var b bool
		if err := json.Unmarshal(si.Value, &b); err != nil {
			return err
		}
		v.Value = b
	case typeByteString, typeBuffer:
		b, err := decodeBytes(si.Value)
		if err != nil {
			return err
		}
		v.Value = b
	case typeInterop, typeNull:
		v.Value = nil
	case typeMap:
		// we want to have the same order as in test file, so a custom decoder is used
		d := json.NewDecoder(bytes.NewReader(si.Value))
		if tok, err := d.Token(); err != nil || tok != json.Delim('{') {
			return fmt.Errorf("invalid map start")
		}

		result := stackitem.NewMap()
		for {
			tok, err := d.Token()
			if err != nil {
				return err
			} else if tok == json.Delim('}') {
				break
			}
			key, ok := tok.(string)
			if !ok {
				return fmt.Errorf("string expected in map key")
			}

			var it vmUTStackItem
			if err := d.Decode(&it); err != nil {
				return fmt.Errorf("can't decode map value: %v", err)
			}

			item := jsonStringToInteger(key)
			if item == nil {
				return fmt.Errorf("can't unmarshal Item %s", key)
			}
			result.Add(item, it.toStackItem())
		}
		v.Value = result
	case typeString:
		panic("not implemented")
	default:
		panic(fmt.Sprintf("unknown type: %s", si.Type))
	}
	return nil
}

// decodeBytes tries to decode bytes from string.
// It tries hex and base64 encodings.
func decodeBytes(data []byte) ([]byte, error) {
	if len(data) == 2 {
		return []byte{}, nil
	}

	data = data[1 : len(data)-1] // strip quotes
	if b, err := decodeHex(string(data)); err == nil {
		return b, nil
	}

	r := base64.NewDecoder(base64.StdEncoding, bytes.NewReader(data))
	return ioutil.ReadAll(r)
}

func decodeHex(s string) ([]byte, error) {
	if strings.HasPrefix(s, "0x") {
		s = s[2:]
	}
	return hex.DecodeString(s)
}
