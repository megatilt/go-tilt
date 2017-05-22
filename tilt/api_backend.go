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

package tilt

import (
	"context"
	"math/big"

	"github.com/megatilt/go-tilt/accounts"
	"github.com/megatilt/go-tilt/common"
	"github.com/megatilt/go-tilt/common/math"
	"github.com/megatilt/go-tilt/core"
	"github.com/megatilt/go-tilt/core/state"
	"github.com/megatilt/go-tilt/core/types"
	"github.com/megatilt/go-tilt/core/vm"
	"github.com/megatilt/go-tilt/event"
	"github.com/megatilt/go-tilt/internal/tiltapi"
	"github.com/megatilt/go-tilt/params"
	"github.com/megatilt/go-tilt/rpc"
	"github.com/megatilt/go-tilt/tilt/downloader"
	"github.com/megatilt/go-tilt/tilt/gasprice"
	"github.com/megatilt/go-tilt/tiltdb"
)

// TiltApiBackend implements tiltapi.Backend for full nodes
type TiltApiBackend struct {
	tilt *Tiltnet
	gpo  *gasprice.Oracle
}

func (b *TiltApiBackend) ChainConfig() *params.ChainConfig {
	return b.tilt.chainConfig
}

func (b *TiltApiBackend) CurrentBlock() *types.Block {
	return b.tilt.blockchain.CurrentBlock()
}

func (b *TiltApiBackend) SetHead(number uint64) {
	b.tilt.protocolManager.downloader.Cancel()
	b.tilt.blockchain.SetHead(number)
}

func (b *TiltApiBackend) HeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.Header, error) {
	// Pending block is only known by the miner
	if blockNr == rpc.PendingBlockNumber {
		block := b.tilt.miner.PendingBlock()
		return block.Header(), nil
	}
	// Otherwise resolve and return the block
	if blockNr == rpc.LatestBlockNumber {
		return b.tilt.blockchain.CurrentBlock().Header(), nil
	}
	return b.tilt.blockchain.GetHeaderByNumber(uint64(blockNr)), nil
}

func (b *TiltApiBackend) BlockByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.Block, error) {
	// Pending block is only known by the miner
	if blockNr == rpc.PendingBlockNumber {
		block := b.tilt.miner.PendingBlock()
		return block, nil
	}
	// Otherwise resolve and return the block
	if blockNr == rpc.LatestBlockNumber {
		return b.tilt.blockchain.CurrentBlock(), nil
	}
	return b.tilt.blockchain.GetBlockByNumber(uint64(blockNr)), nil
}

func (b *TiltApiBackend) StateAndHeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (tiltapi.State, *types.Header, error) {
	// Pending state is only known by the miner
	if blockNr == rpc.PendingBlockNumber {
		block, state := b.tilt.miner.Pending()
		return TiltApiState{state}, block.Header(), nil
	}
	// Otherwise resolve the block number and return its state
	header, err := b.HeaderByNumber(ctx, blockNr)
	if header == nil || err != nil {
		return nil, nil, err
	}
	stateDb, err := b.tilt.BlockChain().StateAt(header.Root)
	return TiltApiState{stateDb}, header, err
}

func (b *TiltApiBackend) GetBlock(ctx context.Context, blockHash common.Hash) (*types.Block, error) {
	return b.tilt.blockchain.GetBlockByHash(blockHash), nil
}

func (b *TiltApiBackend) GetReceipts(ctx context.Context, blockHash common.Hash) (types.Receipts, error) {
	return core.GetBlockReceipts(b.tilt.chainDb, blockHash, core.GetBlockNumber(b.tilt.chainDb, blockHash)), nil
}

func (b *TiltApiBackend) GetTd(blockHash common.Hash) *big.Int {
	return b.tilt.blockchain.GetTdByHash(blockHash)
}

func (b *TiltApiBackend) GetTiltVM(ctx context.Context, msg core.Message, state tiltapi.State, header *types.Header, vmCfg vm.Config) (*vm.TiltVM, func() error, error) {
	statedb := state.(TiltApiState).state
	from := statedb.GetOrNewStateObject(msg.From())
	from.SetBalance(math.MaxBig256)
	vmError := func() error { return nil }

	context := core.NewTiltVMContext(msg, header, b.tilt.BlockChain(), nil)
	return vm.NewTiltVM(context, statedb, b.tilt.chainConfig, vmCfg), vmError, nil
}

func (b *TiltApiBackend) SendTx(ctx context.Context, signedTx *types.Transaction) error {
	b.tilt.txMu.Lock()
	defer b.tilt.txMu.Unlock()

	b.tilt.txPool.SetLocal(signedTx)
	return b.tilt.txPool.Add(signedTx)
}

func (b *TiltApiBackend) RemoveTx(txHash common.Hash) {
	b.tilt.txMu.Lock()
	defer b.tilt.txMu.Unlock()

	b.tilt.txPool.Remove(txHash)
}

func (b *TiltApiBackend) GetPoolTransactions() (types.Transactions, error) {
	b.tilt.txMu.Lock()
	defer b.tilt.txMu.Unlock()

	pending, err := b.tilt.txPool.Pending()
	if err != nil {
		return nil, err
	}

	var txs types.Transactions
	for _, batch := range pending {
		txs = append(txs, batch...)
	}
	return txs, nil
}

func (b *TiltApiBackend) GetPoolTransaction(hash common.Hash) *types.Transaction {
	b.tilt.txMu.Lock()
	defer b.tilt.txMu.Unlock()

	return b.tilt.txPool.Get(hash)
}

func (b *TiltApiBackend) GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error) {
	b.tilt.txMu.Lock()
	defer b.tilt.txMu.Unlock()

	return b.tilt.txPool.State().GetNonce(addr), nil
}

func (b *TiltApiBackend) Stats() (pending int, queued int) {
	b.tilt.txMu.Lock()
	defer b.tilt.txMu.Unlock()

	return b.tilt.txPool.Stats()
}

func (b *TiltApiBackend) TxPoolContent() (map[common.Address]types.Transactions, map[common.Address]types.Transactions) {
	b.tilt.txMu.Lock()
	defer b.tilt.txMu.Unlock()

	return b.tilt.TxPool().Content()
}

func (b *TiltApiBackend) Downloader() *downloader.Downloader {
	return b.tilt.Downloader()
}

func (b *TiltApiBackend) ProtocolVersion() int {
	return b.tilt.TiltVersion()
}

func (b *TiltApiBackend) SuggestPrice(ctx context.Context) (*big.Int, error) {
	return b.gpo.SuggestPrice(ctx)
}

func (b *TiltApiBackend) ChainDb() tiltdb.Database {
	return b.tilt.ChainDb()
}

func (b *TiltApiBackend) EventMux() *event.TypeMux {
	return b.tilt.EventMux()
}

func (b *TiltApiBackend) AccountManager() *accounts.Manager {
	return b.tilt.AccountManager()
}

type TiltApiState struct {
	state *state.StateDB
}

func (s TiltApiState) GetBalance(ctx context.Context, addr common.Address) (*big.Int, error) {
	return s.state.GetBalance(addr), nil
}

func (s TiltApiState) GetCode(ctx context.Context, addr common.Address) ([]byte, error) {
	return s.state.GetCode(addr), nil
}

func (s TiltApiState) GetState(ctx context.Context, a common.Address, b common.Hash) (common.Hash, error) {
	return s.state.GetState(a, b), nil
}

func (s TiltApiState) GetNonce(ctx context.Context, addr common.Address) (uint64, error) {
	return s.state.GetNonce(addr), nil
}
