package state

import (
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/encoding/bigint"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// NEP17TransferBatchSize is the maximum number of entries for NEP17TransferLog.
const NEP17TransferBatchSize = 128

// NEP17Tracker contains info about a single account in a NEP17 contract.
type NEP17Tracker struct {
	// Balance is the current balance of the account.
	Balance big.Int
	// LastUpdatedBlock is a number of block when last `transfer` to or from the
	// account occurred.
	LastUpdatedBlock uint32
}

// NEP17TransferLog is a log of NEP17 token transfers for the specific command.
type NEP17TransferLog struct {
	Raw []byte
}

// NEP17Transfer represents a single NEP17 Transfer event.
type NEP17Transfer struct {
	// Asset is a NEP17 contract ID.
	Asset int32
	// Address is the address of the sender.
	From util.Uint160
	// To is the address of the receiver.
	To util.Uint160
	// Amount is the amount of tokens transferred.
	// It is negative when tokens are sent and positive if they are received.
	Amount big.Int
	// Block is a number of block when the event occurred.
	Block uint32
	// Timestamp is the timestamp of the block where transfer occurred.
	Timestamp uint64
	// Tx is a hash the transaction.
	Tx util.Uint256
}

// NEP17Balances is a map of the NEP17 contract IDs
// to the corresponding structures.
type NEP17Balances struct {
	Trackers map[int32]NEP17Tracker
	// NextTransferBatch stores an index of the next transfer batch.
	NextTransferBatch uint32
	// NewBatch is true if batch with the `NextTransferBatch` index should be created.
	NewBatch bool
}

// NewNEP17Balances returns new NEP17Balances.
func NewNEP17Balances() *NEP17Balances {
	return &NEP17Balances{
		Trackers: make(map[int32]NEP17Tracker),
	}
}

// DecodeBinary implements io.Serializable interface.
func (bs *NEP17Balances) DecodeBinary(r *io.BinReader) {
	bs.NextTransferBatch = r.ReadU32LE()
	bs.NewBatch = r.ReadBool()
	lenBalances := r.ReadVarUint()
	m := make(map[int32]NEP17Tracker, lenBalances)
	for i := 0; i < int(lenBalances); i++ {
		key := int32(r.ReadU32LE())
		var tr NEP17Tracker
		tr.DecodeBinary(r)
		m[key] = tr
	}
	bs.Trackers = m
}

// EncodeBinary implements io.Serializable interface.
func (bs *NEP17Balances) EncodeBinary(w *io.BinWriter) {
	w.WriteU32LE(bs.NextTransferBatch)
	w.WriteBool(bs.NewBatch)
	w.WriteVarUint(uint64(len(bs.Trackers)))
	for k, v := range bs.Trackers {
		w.WriteU32LE(uint32(k))
		v.EncodeBinary(w)
	}
}

// Append appends single transfer to a log.
func (lg *NEP17TransferLog) Append(tr *NEP17Transfer) error {
	w := io.NewBufBinWriter()
	// The first entry, set up counter.
	if len(lg.Raw) == 0 {
		w.WriteB(1)
	}
	tr.EncodeBinary(w.BinWriter)
	if w.Err != nil {
		return w.Err
	}
	if len(lg.Raw) != 0 {
		lg.Raw[0]++
	}
	lg.Raw = append(lg.Raw, w.Bytes()...)
	return nil
}

// ForEach iterates over transfer log returning on first error.
func (lg *NEP17TransferLog) ForEach(f func(*NEP17Transfer) (bool, error)) (bool, error) {
	if lg == nil || len(lg.Raw) == 0 {
		return true, nil
	}
	transfers := make([]NEP17Transfer, lg.Size())
	r := io.NewBinReaderFromBuf(lg.Raw[1:])
	for i := 0; i < lg.Size(); i++ {
		transfers[i].DecodeBinary(r)
	}
	if r.Err != nil {
		return false, r.Err
	}
	for i := len(transfers) - 1; i >= 0; i-- {
		cont, err := f(&transfers[i])
		if err != nil {
			return false, err
		}
		if !cont {
			return false, nil
		}
	}
	return true, nil
}

// Size returns an amount of transfer written in log.
func (lg *NEP17TransferLog) Size() int {
	if len(lg.Raw) == 0 {
		return 0
	}
	return int(lg.Raw[0])
}

// EncodeBinary implements io.Serializable interface.
func (t *NEP17Tracker) EncodeBinary(w *io.BinWriter) {
	w.WriteVarBytes(bigint.ToBytes(&t.Balance))
	w.WriteU32LE(t.LastUpdatedBlock)
}

// DecodeBinary implements io.Serializable interface.
func (t *NEP17Tracker) DecodeBinary(r *io.BinReader) {
	t.Balance = *bigint.FromBytes(r.ReadVarBytes())
	t.LastUpdatedBlock = r.ReadU32LE()
}

// EncodeBinary implements io.Serializable interface.
func (t *NEP17Transfer) EncodeBinary(w *io.BinWriter) {
	w.WriteU32LE(uint32(t.Asset))
	w.WriteBytes(t.Tx[:])
	w.WriteBytes(t.From[:])
	w.WriteBytes(t.To[:])
	w.WriteU32LE(t.Block)
	w.WriteU64LE(t.Timestamp)
	amount := bigint.ToBytes(&t.Amount)
	w.WriteVarBytes(amount)
}

// DecodeBinary implements io.Serializable interface.
func (t *NEP17Transfer) DecodeBinary(r *io.BinReader) {
	t.Asset = int32(r.ReadU32LE())
	r.ReadBytes(t.Tx[:])
	r.ReadBytes(t.From[:])
	r.ReadBytes(t.To[:])
	t.Block = r.ReadU32LE()
	t.Timestamp = r.ReadU64LE()
	amount := r.ReadVarBytes(bigint.MaxBytesLen)
	t.Amount = *bigint.FromBytes(amount)
}
