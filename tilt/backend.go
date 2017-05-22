// Copyright 2014 The go-tiltnet Authors
// This file is part of the go-tiltnet library.
//
// The go-tiltnet library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-tiltnet library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-tiltnet library. If not, see <http://www.gnu.org/licenses/>.

// Package tilt implements the Tiltnet protocol.
package tilt

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/megatilt/go-tilt/accounts"
	"github.com/megatilt/go-tilt/common"
	"github.com/megatilt/go-tilt/common/hexutil"
	"github.com/megatilt/go-tilt/consensus"
	"github.com/megatilt/go-tilt/consensus/tilthash"
	"github.com/megatilt/go-tilt/core"
	"github.com/megatilt/go-tilt/core/types"
	"github.com/megatilt/go-tilt/core/vm"
	"github.com/megatilt/go-tilt/event"
	"github.com/megatilt/go-tilt/internal/tiltapi"
	"github.com/megatilt/go-tilt/log"
	"github.com/megatilt/go-tilt/miner"
	"github.com/megatilt/go-tilt/node"
	"github.com/megatilt/go-tilt/p2p"
	"github.com/megatilt/go-tilt/params"
	"github.com/megatilt/go-tilt/rlp"
	"github.com/megatilt/go-tilt/rpc"
	"github.com/megatilt/go-tilt/tilt/downloader"
	"github.com/megatilt/go-tilt/tilt/filters"
	"github.com/megatilt/go-tilt/tilt/gasprice"
	"github.com/megatilt/go-tilt/tiltdb"
)

type LesServer interface {
	Start(srvr *p2p.Server)
	Stop()
	Protocols() []p2p.Protocol
}

// Tiltnet implements the Tiltnet full node service.
type Tiltnet struct {
	chainConfig *params.ChainConfig
	// Channel for shutting down the service
	shutdownChan  chan bool // Channel for shutting down the tiltnet
	stopDbUpgrade func()    // stop chain db sequential key upgrade
	// Handlers
	txPool          *core.TxPool
	txMu            sync.Mutex
	blockchain      *core.BlockChain
	protocolManager *ProtocolManager
	lesServer       LesServer
	// DB interfaces
	chainDb tiltdb.Database // Block chain database

	eventMux       *event.TypeMux
	engine         consensus.Engine
	accountManager *accounts.Manager

	ApiBackend *TiltApiBackend

	miner        *miner.Miner
	Mining       bool
	MinerThreads int
	tiltbase    common.Address

	networkId     uint64
	netRPCService *tiltapi.PublicNetAPI
}

func (s *Tiltnet) AddLesServer(ls LesServer) {
	s.lesServer = ls
	s.protocolManager.lesServer = ls
}

// New creates a new Tiltnet object (including the
// initialisation of the common Tiltnet object)
func New(ctx *node.ServiceContext, config *Config) (*Tiltnet, error) {
	if !config.SyncMode.IsValid() {
		return nil, fmt.Errorf("invalid sync mode %d", config.SyncMode)
	}

	chainDb, err := CreateDB(ctx, config, "chaindata")
	if err != nil {
		return nil, err
	}
	stopDbUpgrade := upgradeSequentialKeys(chainDb)
	chainConfig, genesisHash, genesisErr := core.SetupGenesisBlock(chainDb, config.Genesis)
	if _, ok := genesisErr.(*params.ConfigCompatError); genesisErr != nil && !ok {
		return nil, genesisErr
	}
	log.Info("Initialised chain configuration", "config", chainConfig)

	tilt := &Tiltnet{
		chainDb:        chainDb,
		chainConfig:    chainConfig,
		eventMux:       ctx.EventMux,
		accountManager: ctx.AccountManager,
		engine:         CreateConsensusEngine(ctx, config, chainConfig, chainDb),
		shutdownChan:   make(chan bool),
		stopDbUpgrade:  stopDbUpgrade,
		networkId:      config.NetworkId,
		tiltbase:      config.Tiltbase,
		MinerThreads:   config.MinerThreads,
	}

	if err := addMipmapBloomBins(chainDb); err != nil {
		return nil, err
	}
	log.Info("Initialising Tiltnet protocol", "versions", ProtocolVersions, "network", config.NetworkId)

	if !config.SkipBcVersionCheck {
		bcVersion := core.GetBlockChainVersion(chainDb)
		if bcVersion != core.BlockChainVersion && bcVersion != 0 {
			return nil, fmt.Errorf("Blockchain DB version mismatch (%d / %d). Run tiltnode upgradedb.\n", bcVersion, core.BlockChainVersion)
		}
		core.WriteBlockChainVersion(chainDb, core.BlockChainVersion)
	}

	vmConfig := vm.Config{EnablePreimageRecording: config.EnablePreimageRecording}
	tilt.blockchain, err = core.NewBlockChain(chainDb, tilt.chainConfig, tilt.engine, tilt.eventMux, vmConfig)
	if err != nil {
		return nil, err
	}
	// Rewind the chain in case of an incompatible config upgrade.
	if compat, ok := genesisErr.(*params.ConfigCompatError); ok {
		log.Warn("Rewinding chain to upgrade configuration", "err", compat)
		tilt.blockchain.SetHead(compat.RewindTo)
		core.WriteChainConfig(chainDb, genesisHash, chainConfig)
	}

	newPool := core.NewTxPool(tilt.chainConfig, tilt.EventMux(), tilt.blockchain.State, tilt.blockchain.GasLimit)
	tilt.txPool = newPool

	maxPeers := config.MaxPeers

	if tilt.protocolManager, err = NewProtocolManager(tilt.chainConfig, config.SyncMode, config.NetworkId, maxPeers, tilt.eventMux, tilt.txPool, tilt.engine, tilt.blockchain, chainDb); err != nil {
		return nil, err
	}

	tilt.miner = miner.New(tilt, tilt.chainConfig, tilt.EventMux(), tilt.engine)
	tilt.miner.SetGasPrice(config.GasPrice)
	tilt.miner.SetExtra(makeExtraData(config.ExtraData))

	tilt.ApiBackend = &TiltApiBackend{tilt, nil}
	gpoParams := config.GPO
	if gpoParams.Default == nil {
		gpoParams.Default = config.GasPrice
	}
	tilt.ApiBackend.gpo = gasprice.NewOracle(tilt.ApiBackend, gpoParams)

	return tilt, nil
}

func makeExtraData(extra []byte) []byte {
	if len(extra) == 0 {
		// create default extradata
		extra, _ = rlp.EncodeToBytes([]interface{}{
			uint(params.VersionMajor<<16 | params.VersionMinor<<8 | params.VersionPatch),
			"tiltnode",
			runtime.Version(),
			runtime.GOOS,
		})
	}
	if uint64(len(extra)) > params.MaximumExtraDataSize {
		log.Warn("Miner extra data exceed limit", "extra", hexutil.Bytes(extra), "limit", params.MaximumExtraDataSize)
		extra = nil
	}
	return extra
}

// CreateDB creates the chain database.
func CreateDB(ctx *node.ServiceContext, config *Config, name string) (tiltdb.Database, error) {
	db, err := ctx.OpenDatabase(name, config.DatabaseCache, config.DatabaseHandles)
	if db, ok := db.(*tiltdb.LDBDatabase); ok {
		db.Meter("tilt/db/chaindata/")
	}
	return db, err
}

// CreateConsensusEngine creates the required type of consensus engine instance for an Tiltnet service
func CreateConsensusEngine(ctx *node.ServiceContext, config *Config, chainConfig *params.ChainConfig, db tiltdb.Database) consensus.Engine {
	switch {
	case config.PowFake:
		log.Warn("Tilthash used in fake mode")
		return tilthash.NewFaker()
	case config.PowTest:
		log.Warn("Tilthash used in test mode")
		return tilthash.NewTester()
	case config.PowShared:
		log.Warn("Tilthash used in shared mode")
		return tilthash.NewShared()
	default:
		engine := tilthash.New(ctx.ResolvePath(config.TilthashCacheDir), config.TilthashCachesInMem, config.TilthashCachesOnDisk,
			config.TilthashDatasetDir, config.TilthashDatasetsInMem, config.TilthashDatasetsOnDisk)
		engine.SetThreads(-1) // Disable CPU mining
		return engine
	}
}

// APIs returns the collection of RPC services the tiltnet package offers.
// NOTE, some of these services probably need to be moved to somewhere else.
func (s *Tiltnet) APIs() []rpc.API {
	apis := tiltapi.GetAPIs(s.ApiBackend)

	// Append any APIs exposed explicitly by the consensus engine
	apis = append(apis, s.engine.APIs(s.BlockChain())...)

	// Append all the local APIs and return
	return append(apis, []rpc.API{
		{
			Namespace: "tilt",
			Version:   "1.0",
			Service:   NewPublicTiltnetAPI(s),
			Public:    true,
		}, {
			Namespace: "tilt",
			Version:   "1.0",
			Service:   NewPublicMinerAPI(s),
			Public:    true,
		}, {
			Namespace: "tilt",
			Version:   "1.0",
			Service:   downloader.NewPublicDownloaderAPI(s.protocolManager.downloader, s.eventMux),
			Public:    true,
		}, {
			Namespace: "miner",
			Version:   "1.0",
			Service:   NewPrivateMinerAPI(s),
			Public:    false,
		}, {
			Namespace: "tilt",
			Version:   "1.0",
			Service:   filters.NewPublicFilterAPI(s.ApiBackend, false),
			Public:    true,
		}, {
			Namespace: "admin",
			Version:   "1.0",
			Service:   NewPrivateAdminAPI(s),
		}, {
			Namespace: "debug",
			Version:   "1.0",
			Service:   NewPublicDebugAPI(s),
			Public:    true,
		}, {
			Namespace: "debug",
			Version:   "1.0",
			Service:   NewPrivateDebugAPI(s.chainConfig, s),
		}, {
			Namespace: "net",
			Version:   "1.0",
			Service:   s.netRPCService,
			Public:    true,
		},
	}...)
}

func (s *Tiltnet) ResetWithGenesisBlock(gb *types.Block) {
	s.blockchain.ResetWithGenesisBlock(gb)
}

func (s *Tiltnet) Tiltbase() (eb common.Address, err error) {
	if s.tiltbase != (common.Address{}) {
		return s.tiltbase, nil
	}
	if wallets := s.AccountManager().Wallets(); len(wallets) > 0 {
		if accounts := wallets[0].Accounts(); len(accounts) > 0 {
			return accounts[0].Address, nil
		}
	}
	return common.Address{}, fmt.Errorf("tiltbase address must be explicitly specified")
}

// set in js console via admin interface or wrapper from cli flags
func (self *Tiltnet) SetTiltbase(tiltbase common.Address) {
	self.tiltbase = tiltbase
	self.miner.SetTiltbase(tiltbase)
}

func (s *Tiltnet) StartMining(local bool) error {
	eb, err := s.Tiltbase()
	if err != nil {
		log.Error("Cannot start mining without tiltbase", "err", err)
		return fmt.Errorf("tiltbase missing: %v", err)
	}
	if local {
		// If local (CPU) mining is started, we can disable the transaction rejection
		// mechanism introduced to speed sync times. CPU mining on mainnet is ludicrous
		// so noone will ever hit this path, whereas marking sync done on CPU mining
		// will ensure that private networks work in single miner mode too.
		atomic.StoreUint32(&s.protocolManager.acceptTxs, 1)
	}
	go s.miner.Start(eb)
	return nil
}

func (s *Tiltnet) StopMining()         { s.miner.Stop() }
func (s *Tiltnet) IsMining() bool      { return s.miner.Mining() }
func (s *Tiltnet) Miner() *miner.Miner { return s.miner }

func (s *Tiltnet) AccountManager() *accounts.Manager  { return s.accountManager }
func (s *Tiltnet) BlockChain() *core.BlockChain       { return s.blockchain }
func (s *Tiltnet) TxPool() *core.TxPool               { return s.txPool }
func (s *Tiltnet) EventMux() *event.TypeMux           { return s.eventMux }
func (s *Tiltnet) Engine() consensus.Engine           { return s.engine }
func (s *Tiltnet) ChainDb() tiltdb.Database           { return s.chainDb }
func (s *Tiltnet) IsListening() bool                  { return true } // Always listening
func (s *Tiltnet) TiltVersion() int                   { return int(s.protocolManager.SubProtocols[0].Version) }
func (s *Tiltnet) NetVersion() uint64                 { return s.networkId }
func (s *Tiltnet) Downloader() *downloader.Downloader { return s.protocolManager.downloader }

// Protocols implements node.Service, returning all the currently configured
// network protocols to start.
func (s *Tiltnet) Protocols() []p2p.Protocol {
	if s.lesServer == nil {
		return s.protocolManager.SubProtocols
	} else {
		return append(s.protocolManager.SubProtocols, s.lesServer.Protocols()...)
	}
}

// Start implements node.Service, starting all internal goroutines needed by the
// Tiltnet protocol implementation.
func (s *Tiltnet) Start(srvr *p2p.Server) error {
	s.netRPCService = tiltapi.NewPublicNetAPI(srvr, s.NetVersion())

	s.protocolManager.Start()
	if s.lesServer != nil {
		s.lesServer.Start(srvr)
	}
	return nil
}

// Stop implements node.Service, terminating all internal goroutines used by the
// Tiltnet protocol.
func (s *Tiltnet) Stop() error {
	if s.stopDbUpgrade != nil {
		s.stopDbUpgrade()
	}
	s.blockchain.Stop()
	s.protocolManager.Stop()
	if s.lesServer != nil {
		s.lesServer.Stop()
	}
	s.txPool.Stop()
	s.miner.Stop()
	s.eventMux.Stop()

	s.chainDb.Close()
	close(s.shutdownChan)

	return nil
}
