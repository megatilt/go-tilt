// Copyright 2015 The go-tiltnet Authors
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

// Package tiltapi implements the general Tiltnet API functions.
package tiltapi

import (
	"context"
	"math/big"

	"github.com/megatilt/go-tilt/accounts"
	"github.com/megatilt/go-tilt/common"
	"github.com/megatilt/go-tilt/core"
	"github.com/megatilt/go-tilt/core/types"
	"github.com/megatilt/go-tilt/core/vm"
	"github.com/megatilt/go-tilt/tilt/downloader"
	"github.com/megatilt/go-tilt/tiltdb"
	"github.com/megatilt/go-tilt/event"
	"github.com/megatilt/go-tilt/params"
	"github.com/megatilt/go-tilt/rpc"
)

// Backend interface provides the common API services (that are provided by
// both full and light clients) with access to necessary functions.
type Backend interface {
	// general Tiltnet API
	Downloader() *downloader.Downloader
	ProtocolVersion() int
	SuggestPrice(ctx context.Context) (*big.Int, error)
	ChainDb() tiltdb.Database
	EventMux() *event.TypeMux
	AccountManager() *accounts.Manager
	// BlockChain API
	SetHead(number uint64)
	HeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.Header, error)
	BlockByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.Block, error)
	StateAndHeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (State, *types.Header, error)
	GetBlock(ctx context.Context, blockHash common.Hash) (*types.Block, error)
	GetReceipts(ctx context.Context, blockHash common.Hash) (types.Receipts, error)
	GetTd(blockHash common.Hash) *big.Int
	GetTiltVM(ctx context.Context, msg core.Message, state State, header *types.Header, vmCfg vm.Config) (*vm.TiltVM, func() error, error)
	// TxPool API
	SendTx(ctx context.Context, signedTx *types.Transaction) error
	RemoveTx(txHash common.Hash)
	GetPoolTransactions() (types.Transactions, error)
	GetPoolTransaction(txHash common.Hash) *types.Transaction
	GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error)
	Stats() (pending int, queued int)
	TxPoolContent() (map[common.Address]types.Transactions, map[common.Address]types.Transactions)

	ChainConfig() *params.ChainConfig
	CurrentBlock() *types.Block
}

type State interface {
	GetBalance(ctx context.Context, addr common.Address) (*big.Int, error)
	GetCode(ctx context.Context, addr common.Address) ([]byte, error)
	GetState(ctx context.Context, a common.Address, b common.Hash) (common.Hash, error)
	GetNonce(ctx context.Context, addr common.Address) (uint64, error)
}

func GetAPIs(apiBackend Backend) []rpc.API {
	return []rpc.API{
		{
			Namespace: "tilt",
			Version:   "1.0",
			Service:   NewPublicTiltnetAPI(apiBackend),
			Public:    true,
		}, {
			Namespace: "tilt",
			Version:   "1.0",
			Service:   NewPublicBlockChainAPI(apiBackend),
			Public:    true,
		}, {
			Namespace: "tilt",
			Version:   "1.0",
			Service:   NewPublicTransactionPoolAPI(apiBackend),
			Public:    true,
		}, {
			Namespace: "txpool",
			Version:   "1.0",
			Service:   NewPublicTxPoolAPI(apiBackend),
			Public:    true,
		}, {
			Namespace: "debug",
			Version:   "1.0",
			Service:   NewPublicDebugAPI(apiBackend),
			Public:    true,
		}, {
			Namespace: "debug",
			Version:   "1.0",
			Service:   NewPrivateDebugAPI(apiBackend),
		}, {
			Namespace: "tilt",
			Version:   "1.0",
			Service:   NewPublicAccountAPI(apiBackend.AccountManager()),
			Public:    true,
		}, {
			Namespace: "personal",
			Version:   "1.0",
			Service:   NewPrivateAccountAPI(apiBackend),
			Public:    false,
		},
	}
}
