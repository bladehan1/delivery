package clerk

import (
	"errors"
	"strconv"
	"time"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/tendermint/tendermint/libs/log"

	"github.com/maticnetwork/heimdall/chainmanager"
	"github.com/maticnetwork/heimdall/clerk/types"
	"github.com/maticnetwork/heimdall/params/subspace"
	hmTypes "github.com/maticnetwork/heimdall/types"
)

var (
	StateRecordPrefixKey = []byte{0x11} // prefix key for when storing state

	// DefaultValue default value
	DefaultValue = []byte{0x01}

	// RecordSequencePrefixKey represents record sequence prefix key
	RecordSequencePrefixKey = []byte{0x12}

	StateRecordPrefixKeyWithTime = []byte{0x13} // prefix key for when storing state with time

	TronStateRecordPrefixKeyWithTime = []byte{0x14} // tron prefix key for when storing state with time

	EthStateRecordPrefixKeyWithTime = []byte{0x15} // eth prefix key for when storing state with time

	TronStateRecordPrefixKey = []byte{0x16} // tron prefix key for when storing state

	EthStateRecordPrefixKey = []byte{0x17} // eth prefix key for when storing state

	LatestMsgIDPrefixKey = []byte{0x18} // eth prefix key for when storing state

	RootIdToHeimdallIDPrefixKey = []byte{0x19} // store <rootChainID, hemidall chainID>

)

// Keeper stores all related data
type Keeper struct {
	cdc *codec.Codec
	// The (unexposed) keys used to access the stores from the Context.
	storeKey sdk.StoreKey
	// codespace
	codespace sdk.CodespaceType
	// param space
	paramSpace subspace.Subspace
	// chain param keeper
	chainKeeper chainmanager.Keeper
}

// NewKeeper create new keeper
func NewKeeper(
	cdc *codec.Codec,
	storeKey sdk.StoreKey,
	paramSpace subspace.Subspace,
	codespace sdk.CodespaceType,
	chainKeeper chainmanager.Keeper,
) Keeper {
	keeper := Keeper{
		cdc:         cdc,
		storeKey:    storeKey,
		paramSpace:  paramSpace,
		codespace:   codespace,
		chainKeeper: chainKeeper,
	}
	return keeper
}

// Codespace returns the codespace
func (k Keeper) Codespace() sdk.CodespaceType {
	return k.codespace
}

// Logger returns a module-specific logger
func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", types.ModuleName)
}

// SetEventRecordWithTime sets event record id with time
func (k *Keeper) SetEventRecordWithTime(ctx sdk.Context, record types.EventRecord, latestID uint64) error {
	// set root chain msg ID
	rootChainKey := GetRootChainEventRecordKeyWithTimePrefix(record.RootChainType, record.ID, record.RecordTime)
	value, err := k.cdc.MarshalBinaryBare(record.ID)
	if err != nil {
		k.Logger(ctx).Error("Error marshalling record", "error", err)
		return err
	}
	if rootChainKey == nil {
		return errors.New(" unknown root chain type: " + record.RootChainType)
	}
	if err := k.setEventRecordStore(ctx, rootChainKey, value); err != nil {
		return err
	}

	// set hemidall msg ID
	record.ID = latestID + 1
	value, err = k.cdc.MarshalBinaryBare(record.ID)
	if err != nil {
		k.Logger(ctx).Error("Error marshalling record", "error", err)
		return err
	}
	key := GetEventRecordKeyWithTime(record.ID, record.RecordTime)

	if err := k.setEventRecordStore(ctx, key, value); err != nil {
		return err
	}
	return nil
}

// SetEventRecordWithID adds record to store with ID
func (k *Keeper) SetEventRecordWithID(ctx sdk.Context, record types.EventRecord) (uint64, error) {

	// root chain ID
	rootChainKey := GetRootChainEventRecordKey(record.RootChainType, record.ID)
	value, err := k.cdc.MarshalBinaryBare(record)
	if err != nil {
		k.Logger(ctx).Error("Error marshalling rootChain record", "error", err)
		return 0, err
	}
	if rootChainKey == nil {
		return 0, errors.New(" unknown root chain type: " + record.RootChainType)
	}
	if err := k.setEventRecordStore(ctx, rootChainKey, value); err != nil {
		return 0, err
	}

	// set rootchainID and heimdall id pair
	latestID := k.GetLatestID(ctx)
	rootToHeimdallKey := GetRootToHeimdallIdKey(record.RootChainType, record.ID)
	value, err = k.cdc.MarshalBinaryBare(latestID + 1)
	if err != nil {
		k.Logger(ctx).Error("Error marshalling record", "error", err)
		return 0, err
	}
	if err := k.setEventRecordStore(ctx, rootToHeimdallKey, value); err != nil {
		return 0, err
	}

	// hemidall  ID
	record.ID = latestID + 1
	key := GetEventRecordKey(record.ID)
	value, err = k.cdc.MarshalBinaryBare(record)
	if err != nil {
		k.Logger(ctx).Error("Error marshalling hemidall record", "error", err)
		return 0, err
	}
	if err := k.setEventRecordStore(ctx, key, value); err != nil {
		return 0, err
	}

	k.SetLatestID(ctx, latestID+1) // increment latest hemidall id
	// pass to next function
	return latestID, nil
}

// setEventRecordStore adds value to store by key
func (k *Keeper) setEventRecordStore(ctx sdk.Context, key, value []byte) error {
	store := ctx.KVStore(k.storeKey)
	// check if already set
	if store.Has(key) {
		return errors.New("Key already exists")
	}

	// store value in provided key
	store.Set(key, value)
	// return
	return nil
}

// SetEventRecord adds record to store
func (k *Keeper) SetEventRecord(ctx sdk.Context, record types.EventRecord) error {
	var latestID uint64
	var err error
	if latestID, err = k.SetEventRecordWithID(ctx, record); err != nil {
		return err
	}
	if err = k.SetEventRecordWithTime(ctx, record, latestID); err != nil {
		return err
	}
	return nil
}

// get latest msg ID

func (k *Keeper) GetLatestID(ctx sdk.Context) uint64 {
	store := ctx.KVStore(k.storeKey)
	if store.Has(LatestMsgIDPrefixKey) {
		result, err := strconv.ParseUint(string(store.Get(LatestMsgIDPrefixKey)), 10, 64)
		if err == nil {
			return result
		}
	}
	return 0

}

// update latest msg ID
func (k *Keeper) SetLatestID(ctx sdk.Context, num uint64) {
	store := ctx.KVStore(k.storeKey)
	store.Set(LatestMsgIDPrefixKey, []byte(strconv.FormatUint(num, 10)))
}

// update latest msg ID
func (k *Keeper) GetHeimdallIDByRootID(ctx sdk.Context, rootChainType string, id uint64) (*uint64, error) {
	store := ctx.KVStore(k.storeKey)
	key := GetRootToHeimdallIdKey(rootChainType, id)
	// check store has data
	if store.Has(key) {
		var heimdallID uint64
		err := k.cdc.UnmarshalBinaryBare(store.Get(key), &heimdallID)
		if err != nil {
			return nil, err
		}

		return &heimdallID, nil
	}

	// return no error error
	return nil, errors.New("No record found")
}

// GetEventRecord returns record from store
func (k *Keeper) GetEventRecord(ctx sdk.Context, stateID uint64) (*types.EventRecord, error) {
	store := ctx.KVStore(k.storeKey)
	key := GetEventRecordKey(stateID)

	// check store has data
	if store.Has(key) {
		var _record types.EventRecord
		err := k.cdc.UnmarshalBinaryBare(store.Get(key), &_record)
		if err != nil {
			return nil, err
		}

		return &_record, nil
	}

	// return no error error
	return nil, errors.New("No record found")
}

// GetRootChainEventRecord returns record from store
func (k *Keeper) GetRootChainEventRecord(ctx sdk.Context, stateID uint64, rootChainType string) (*types.EventRecord, error) {
	store := ctx.KVStore(k.storeKey)
	rootChainKey := GetRootChainEventRecordKey(rootChainType, stateID)

	// check store has data
	if store.Has(rootChainKey) {
		var _record types.EventRecord
		err := k.cdc.UnmarshalBinaryBare(store.Get(rootChainKey), &_record)
		if err != nil {
			return nil, err
		}

		return &_record, nil
	}

	// return no error error
	return nil, errors.New("No root chain record found")
}

// hemidall chain id : HasEventRecord check if state record
func (k *Keeper) HasEventRecord(ctx sdk.Context, stateID uint64) bool {
	store := ctx.KVStore(k.storeKey)
	key := GetEventRecordKey(stateID)
	return store.Has(key)
}

// root chain msg id   check if msg have been processed
func (k *Keeper) HasRootChainEventRecord(ctx sdk.Context, rootChainType string, stateID uint64) bool {
	store := ctx.KVStore(k.storeKey)
	key := GetRootChainEventRecordKey(rootChainType, stateID)
	return store.Has(key)
}

// GetAllEventRecords get all state records
func (k *Keeper) GetAllEventRecords(ctx sdk.Context) (records []*types.EventRecord) {
	// iterate through spans and create span update array
	k.IterateRecordsAndApplyFn(ctx, func(record types.EventRecord) error {
		// append to list of validatorUpdates
		records = append(records, &record)
		return nil
	})

	return
}

// GetEventRecordList returns all records with params like page and limit
func (k *Keeper) GetEventRecordList(ctx sdk.Context, page uint64, limit uint64) ([]types.EventRecord, error) {
	store := ctx.KVStore(k.storeKey)

	// create records
	var records []types.EventRecord

	// have max limit
	if limit > 50 {
		limit = 50
	}

	// get paginated iterator
	iterator := hmTypes.KVStorePrefixIteratorPaginated(store, StateRecordPrefixKey, uint(page), uint(limit))

	// loop through records to get valid records
	for ; iterator.Valid(); iterator.Next() {
		var record types.EventRecord
		if err := k.cdc.UnmarshalBinaryBare(iterator.Value(), &record); err == nil {
			records = append(records, record)
		}
	}

	return records, nil
}

// GetEventRecordListWithTime returns all records with params like fromTime and toTime
func (k *Keeper) GetEventRecordListWithTime(ctx sdk.Context, fromTime, toTime time.Time, page, limit uint64) ([]types.EventRecord, error) {
	var iterator sdk.Iterator
	store := ctx.KVStore(k.storeKey)

	// create records
	var records []types.EventRecord

	// have max limit
	if limit > 50 {
		limit = 50
	}

	if page == 0 && limit == 0 {
		iterator = store.Iterator(GetEventRecordKeyWithTimePrefix(fromTime), GetEventRecordKeyWithTimePrefix(toTime))
	} else {
		iterator = hmTypes.KVStorePrefixRangeIteratorPaginated(store, uint(page), uint(limit), GetEventRecordKeyWithTimePrefix(fromTime), GetEventRecordKeyWithTimePrefix(toTime))
	}

	// get range iterator
	defer iterator.Close()
	// loop through records to get valid records
	for ; iterator.Valid(); iterator.Next() {
		var stateID uint64
		if err := k.cdc.UnmarshalBinaryBare(iterator.Value(), &stateID); err == nil {
			record, err := k.GetEventRecord(ctx, stateID)
			if err != nil {
				k.Logger(ctx).Error("GetEventRecordListWithTime | GetEventRecord", "error", err)
				continue
			}
			records = append(records, *record)
		}
	}

	return records, nil
}

//
// GetEventRecordKey returns key for state record
//
func GetRecordKey(prefix []byte, stateID uint64) []byte {
	stateIDBytes := []byte(strconv.FormatUint(stateID, 10))
	return append(prefix, stateIDBytes...)
}

// GetEventRecordKey appends prefix to state id
func GetEventRecordKey(stateID uint64) []byte {
	return GetRecordKey(StateRecordPrefixKey, stateID)
}

// GetRootChainEventRecordKey appends prefix to state id
func GetRootChainEventRecordKey(rootChainType string, stateID uint64) []byte {
	if rootChainType == hmTypes.RootChainTypeEth {
		return GetRecordKey(EthStateRecordPrefixKey, stateID)
	} else if rootChainType == hmTypes.RootChainTypeTron {
		return GetRecordKey(TronStateRecordPrefixKey, stateID)
	} else {
		return nil
	}
}

// GetEventRecordKeyWithTime appends prefix to state id and record time
func GetEventRecordKeyWithTime(stateID uint64, recordTime time.Time) []byte {
	return GetRecordKey(GetEventRecordKeyWithTimePrefix(recordTime), stateID)
}

// GetEventRecordKeyWithTime appends prefix to state id and record time
func GetRootToHeimdallIdKey(rootChainType string, stateID uint64) []byte {
	rootToHeimdallPrefix := append(RootIdToHeimdallIDPrefixKey, []byte(rootChainType)...)
	return GetRecordKey(rootToHeimdallPrefix, stateID)
}

// GetEventRecordKeyWithTimePrefix gives prefix for record time key
func GetEventRecordKeyWithTimePrefix(recordTime time.Time) []byte {
	return GetRecordKeyWithTimePrefix(StateRecordPrefixKeyWithTime, recordTime)
}

// GetRootChainEventRecordKeyWithTimePrefix gives prefix for record time key
func GetRootChainEventRecordKeyWithTimePrefix(rootChainType string, id uint64, recordTime time.Time) []byte {
	if rootChainType == hmTypes.RootChainTypeEth {
		return GetRecordKey(GetRecordKeyWithTimePrefix(EthStateRecordPrefixKeyWithTime, recordTime), id)
	} else if rootChainType == hmTypes.RootChainTypeTron {
		return GetRecordKey(GetRecordKeyWithTimePrefix(TronStateRecordPrefixKeyWithTime, recordTime), id)
	} else {
		return nil
	}
}

func GetRecordKeyWithTimePrefix(prefix []byte, recordTime time.Time) []byte {
	recordTimeBytes := sdk.FormatTimeBytes(recordTime)
	return append(prefix, recordTimeBytes...)
}

// GetRecordSequenceKey returns record sequence key
func GetRecordSequenceKey(sequence string) []byte {
	return append(RecordSequencePrefixKey, []byte(sequence)...)
}

//
// Utils
//

// IterateRecordsAndApplyFn interate records and apply the given function.
func (k *Keeper) IterateRecordsAndApplyFn(ctx sdk.Context, f func(record types.EventRecord) error) {
	store := ctx.KVStore(k.storeKey)

	// get span iterator
	iterator := sdk.KVStorePrefixIterator(store, StateRecordPrefixKey)
	defer iterator.Close()

	// loop through spans to get valid spans
	for ; iterator.Valid(); iterator.Next() {
		// unmarshall span
		var result types.EventRecord
		if err := k.cdc.UnmarshalBinaryBare(iterator.Value(), &result); err != nil {
			k.Logger(ctx).Error("IterateRecordsAndApplyFn | UnmarshalBinaryBare", "error", err)
			return
		}
		// call function and return if required
		if err := f(result); err != nil {
			return
		}
	}
}

// GetRecordSequences checks if record already exists
func (k *Keeper) GetRecordSequences(ctx sdk.Context) (sequences []string) {
	k.IterateRecordSequencesAndApplyFn(ctx, func(sequence string) error {
		sequences = append(sequences, sequence)
		return nil
	})
	return
}

// IterateRecordSequencesAndApplyFn interate records and apply the given function.
func (k *Keeper) IterateRecordSequencesAndApplyFn(ctx sdk.Context, f func(sequence string) error) {
	store := ctx.KVStore(k.storeKey)

	// get sequence iterator
	iterator := sdk.KVStorePrefixIterator(store, RecordSequencePrefixKey)
	defer iterator.Close()

	// loop through sequences
	for ; iterator.Valid(); iterator.Next() {
		sequence := string(iterator.Key()[len(RecordSequencePrefixKey):])

		// call function and return if required
		if err := f(sequence); err != nil {
			return
		}
	}
}

// SetRecordSequence sets mapping for sequence id to bool
func (k *Keeper) SetRecordSequence(ctx sdk.Context, sequence string) {
	store := ctx.KVStore(k.storeKey)
	store.Set(GetRecordSequenceKey(sequence), DefaultValue)
}

// HasRecordSequence checks if record already exists
func (k *Keeper) HasRecordSequence(ctx sdk.Context, sequence string) bool {
	store := ctx.KVStore(k.storeKey)
	return store.Has(GetRecordSequenceKey(sequence))
}
