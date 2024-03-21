package app

import (
	"crypto/md5"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path"
	"testing"
	"unsafe"

	"github.com/cosmos/cosmos-sdk/codec"

	"github.com/maticnetwork/heimdall/chainmanager/types"
	"github.com/maticnetwork/heimdall/params/subspace"

	"github.com/ethereum/go-ethereum/common/hexutil"

	"github.com/cosmos/cosmos-sdk/baseapp"
	bam "github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	dbm "github.com/tendermint/tm-db"

	"github.com/maticnetwork/heimdall/app/helpers"
	authTypes "github.com/maticnetwork/heimdall/auth/types"
	govTypes "github.com/maticnetwork/heimdall/gov/types"
	paramTypes "github.com/maticnetwork/heimdall/params/types"
	"github.com/maticnetwork/heimdall/simulation"
	stakingTypes "github.com/maticnetwork/heimdall/staking/types"
	supplyTypes "github.com/maticnetwork/heimdall/supply/types"
)

// Get flags every time the simulator is run
func init() {
	GetSimulatorFlags()
}

type StoreKeysPrefixes struct {
	A        sdk.StoreKey
	B        sdk.StoreKey
	Prefixes [][]byte
}

// fauxMerkleModeOpt returns a BaseApp option to use a dbStoreAdapter instead of
// an IAVLStore for faster simulation speed.
func fauxMerkleModeOpt(bapp *baseapp.BaseApp) {
	bapp.SetFauxMerkleMode()
}

func TestFullAppSimulation(t *testing.T) {
	config, db, dir, logger, skip, err := SetupSimulation("leveldb-app-sim", "Simulation")
	if skip {
		t.Skip("skipping application simulation")
	}
	require.NoError(t, err, "simulation setup failed")
	require.NotNil(t, db, "DB should not be nil")

	defer func() {
		db.Close()
		require.NoError(t, os.RemoveAll(dir))
	}()

	app := NewHeimdallApp(logger, db)
	require.Equal(t, AppName, app.Name())

	// run randomized simulation
	_, simParams, simErr := simulation.SimulateFromSeed(
		t, os.Stdout, app.BaseApp, AppStateFn(app.Codec(), app.SimulationManager()),
		SimulationOperations(app, app.Codec(), config),
		app.ModuleAccountAddrs(), config,
	)

	// export state and simParams before the simulation error is checked
	err = CheckExportSimulation(app, config, simParams)
	require.NoError(t, err)
	require.NoError(t, simErr)

	if config.Commit {
		PrintStats(db)
	}
}

func TestAppImportExport(t *testing.T) {
	config, db, dir, logger, skip, err := SetupSimulation("leveldb-app-sim", "Simulation")
	if skip {
		t.Skip("skipping application import/export simulation")
	}
	require.NoError(t, err, "simulation setup failed")

	defer func() {
		db.Close()
		require.NoError(t, os.RemoveAll(dir))
	}()

	app := NewHeimdallApp(logger, db)
	require.Equal(t, AppName, app.Name())

	// Run randomized simulation
	_, simParams, simErr := simulation.SimulateFromSeed(
		t, os.Stdout, app.BaseApp, AppStateFn(app.Codec(), app.SimulationManager()),
		SimulationOperations(app, app.Codec(), config),
		app.ModuleAccountAddrs(), config,
	)

	// export state and simParams before the simulation error is checked
	err = CheckExportSimulation(app, config, simParams)
	require.NoError(t, err)
	require.NoError(t, simErr)

	if config.Commit {
		PrintStats(db)
	}

	fmt.Printf("exporting genesis...\n")

	appState, _, err := app.ExportAppStateAndValidators()
	require.NoError(t, err)

	fmt.Printf("importing genesis...\n")

	_, newDB, newDir, _, _, err := SetupSimulation("leveldb-app-sim-2", "Simulation-2")
	require.NoError(t, err, "simulation setup failed")

	defer func() {
		newDB.Close()
		require.NoError(t, os.RemoveAll(newDir))
	}()

	newApp := NewHeimdallApp(logger, newDB)
	require.Equal(t, AppName, newApp.Name())

	var genesisState GenesisState
	err = app.Codec().UnmarshalJSON(appState, &genesisState)
	require.NoError(t, err)

	ctxA := app.NewContext(true, abci.Header{Height: app.LastBlockHeight()})
	ctxB := newApp.NewContext(true, abci.Header{Height: app.LastBlockHeight()})
	newApp.mm.InitGenesis(ctxB, genesisState)

	fmt.Printf("comparing stores...\n")

	storeKeysPrefixes := []StoreKeysPrefixes{
		{app.keys[baseapp.MainStoreKey], newApp.keys[baseapp.MainStoreKey], [][]byte{}},
		{app.keys[authTypes.StoreKey], newApp.keys[authTypes.StoreKey], [][]byte{}},
		{app.keys[stakingTypes.StoreKey], newApp.keys[stakingTypes.StoreKey], [][]byte{}},
		{app.keys[supplyTypes.StoreKey], newApp.keys[supplyTypes.StoreKey], [][]byte{}},
		{app.keys[paramTypes.StoreKey], newApp.keys[paramTypes.StoreKey], [][]byte{}},
		{app.keys[govTypes.StoreKey], newApp.keys[govTypes.StoreKey], [][]byte{}},
	}

	for _, skp := range storeKeysPrefixes {
		storeA := ctxA.KVStore(skp.A)
		storeB := ctxB.KVStore(skp.B)

		_, _, _, equal := sdk.DiffKVStores(storeA, storeB, skp.Prefixes)
		require.True(t, equal, "unequal sets of key-values to compare")
	}
}

func TestAppSimulationAfterImport(t *testing.T) {
	config, db, dir, logger, skip, err := SetupSimulation("leveldb-app-sim", "Simulation")
	if skip {
		t.Skip("skipping application simulation after import")
	}
	require.NoError(t, err, "simulation setup failed")

	defer func() {
		db.Close()
		require.NoError(t, os.RemoveAll(dir))
	}()

	app := NewHeimdallApp(logger, db)
	require.Equal(t, AppName, app.Name())

	// Run randomized simulation
	stopEarly, simParams, simErr := simulation.SimulateFromSeed(
		t, os.Stdout, app.BaseApp, AppStateFn(app.Codec(), app.SimulationManager()),
		SimulationOperations(app, app.Codec(), config),
		app.ModuleAccountAddrs(), config,
	)

	// export state and simParams before the simulation error is checked
	err = CheckExportSimulation(app, config, simParams)
	require.NoError(t, err)
	require.NoError(t, simErr)

	if config.Commit {
		PrintStats(db)
	}

	if stopEarly {
		fmt.Println("can't export or import a zero-validator genesis, exiting test...")
		return
	}

	fmt.Printf("exporting genesis...\n")

	appState, _, err := app.ExportAppStateAndValidators()
	require.NoError(t, err)

	fmt.Printf("importing genesis...\n")

	_, newDB, newDir, _, _, err := SetupSimulation("leveldb-app-sim-2", "Simulation-2")
	require.NoError(t, err, "simulation setup failed")

	defer func() {
		newDB.Close()
		require.NoError(t, os.RemoveAll(newDir))
	}()

	newApp := NewHeimdallApp(logger, newDB)
	require.Equal(t, AppName, app.Name())

	newApp.InitChain(abci.RequestInitChain{
		AppStateBytes: appState,
	})
	stopEarly, _, err = simulation.SimulateFromSeed(
		t, os.Stdout, newApp.BaseApp, AppStateFn(app.Codec(), app.SimulationManager()),
		SimulationOperations(newApp, newApp.Codec(), config),
		newApp.ModuleAccountAddrs(), config,
	)
	require.False(t, stopEarly)
	require.NoError(t, err)
}

// TODO: Make another test for the fuzzer itself, which just has noOp txs
// and doesn't depend on the application.
func TestAppStateDeterminism(t *testing.T) {
	if !FlagEnabledValue {
		t.Skip("skipping application simulation")
	}

	config := NewConfigFromFlags()
	config.InitialBlockHeight = 1
	config.ExportParamsPath = ""
	config.OnOperation = false
	config.AllInvariants = false
	config.ChainID = helpers.SimAppChainID

	numSeeds := 3
	numTimesToRunPerSeed := 5
	appHashList := make([]json.RawMessage, numTimesToRunPerSeed)

	for i := 0; i < numSeeds; i++ {
		config.Seed = rand.Int63()

		for j := 0; j < numTimesToRunPerSeed; j++ {
			var logger log.Logger
			if FlagVerboseValue {
				logger = log.TestingLogger()
			} else {
				logger = log.NewNopLogger()
			}

			db := dbm.NewMemDB()

			// app := NewSimApp(logger, db, nil, true, map[int64]bool{}, DefaultNodeHome, FlagPeriodValue, interBlockCacheOpt())
			app := NewHeimdallApp(logger, db, nil)
			err := app.LoadLatestVersion(app.keys[bam.MainStoreKey])
			if err != nil {
				require.NoError(t, err)
			}
			require.Equal(t, AppName, app.Name())

			fmt.Printf(
				"running non-determinism simulation; seed %d: %d/%d, attempt: %d/%d\n",
				config.Seed, i+1, numSeeds, j+1, numTimesToRunPerSeed,
			)

			_, _, err = simulation.SimulateFromSeed(
				t, os.Stdout, app.BaseApp, AppStateFn(app.Codec(), app.SimulationManager()),
				SimulationOperations(app, app.Codec(), config),
				app.ModuleAccountAddrs(), config,
			)
			require.NoError(t, err)

			if config.Commit {
				PrintStats(db)
			}

			appHash := app.LastCommitID().Hash
			appHashList[j] = appHash

			if j != 0 {
				require.Equal(
					t, appHashList[0], appHashList[j],
					"non-determinism in seed %d: %d/%d, attempt: %d/%d\n", config.Seed, i+1, numSeeds, j+1, numTimesToRunPerSeed,
				)
			}
		}
	}
}

type UnsafeBaseApp struct {
	logger log.Logger
	name   string               // application name from abci.Info
	db     dbm.DB               // common DB backend
	cms    sdk.CommitMultiStore // Main (uncached) state
}
type UnsafeSubspace struct {
	cdc *codec.Codec
}

var (
	externalParam string
)

func TestMain(m *testing.M) {
	// 使用 flag 包定义外部参数
	flag.StringVar(&externalParam, "externalParam", "defaultValue", "Description of externalParam")
	// 解析命令行参数
	flag.Parse()
	// 运行测试
	m.Run()
}

func TestKeyValue(t *testing.T) {
	home := "/Volumes/data/1024node/deliveryd"
	if externalParam != "defaultValue" {
		home = externalParam
	}
	home = externalParam
	dataDir := path.Join(home, "data")
	logger := log.NewTMLogger(log.NewSyncWriter(os.Stdout))
	db, err := sdk.NewLevelDB("application", dataDir)
	if err != nil {
		panic(err)
	}

	happ := NewHeimdallApp(logger, db)
	chainManagerModule := "chainmanager"
	subspaceChainManager := happ.GetSubspace(chainManagerModule)
	// ctx 从哪里获取,使用app的cms,
	//通过unsafe包获取私有字段
	unsafeApp := (*UnsafeBaseApp)(unsafe.Pointer(uintptr(unsafe.Pointer(happ.BaseApp))))
	ctx := sdk.NewContext(unsafeApp.cms, abci.Header{}, false, log.NewTMLogger(os.Stdout))
	multiChainBytes := subspaceChainManager.GetRaw(ctx, []byte("ParamsWithMultiChains"))
	md5value := md5.Sum(multiChainBytes)
	fmt.Printf("externalParam:%s\n", externalParam)
	fmt.Printf("ParamsWithMultiChains:valuelen:%d,md5value:%s,value:%s\n", len(multiChainBytes), hexutil.Encode(md5value[:]), hexutil.Encode(multiChainBytes))
	multiChain := types.ParamsWithMultiChains{}
	err = json.Unmarshal(multiChainBytes, &multiChain)
	if err != nil {
		t.Logf("unmarshal multiChain error %e", err)
		unsafeSubspace := (*UnsafeSubspace)(unsafe.Pointer(uintptr(unsafe.Pointer(&subspaceChainManager))))
		err = unsafeSubspace.cdc.UnmarshalJSON(multiChainBytes, &multiChain)
		if err != nil {
			t.Fatalf("cdc unmarshal multiChain error %e", err)
		} else {
			fmt.Printf("cdc ParamsWithMultiChains:%s\n", multiChain)
		}
	} else {
		fmt.Printf("ParamsWithMultiChains:%s\n", multiChain)
	}
	subspaceFeatureManager := happ.GetSubspace("featuremanager")
	checkSupportFeature(ctx, subspaceFeatureManager)
	checkSupportFeature(ctx, subspaceChainManager)

}
func checkSupportFeature(ctx sdk.Context, subspace subspace.Subspace) {
	supportFeature := "SupportFeature"
	supportFeatureByte := subspace.GetRaw(ctx, []byte(supportFeature))
	md5Vlaue := md5.Sum(supportFeatureByte)
	fmt.Printf("supportFeatureByte:valuelen:%d,md5value:%s,value:%s\n", len(supportFeatureByte), hexutil.Encode(md5Vlaue[:]), hexutil.Encode(supportFeatureByte))
}

func getMultiStore() {

}
