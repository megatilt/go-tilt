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

package vm

import (
	"math/big"
	"sync/atomic"

	"github.com/megatilt/go-tilt/common"
	"github.com/megatilt/go-tilt/crypto"
	"github.com/megatilt/go-tilt/params"
)

type (
	CanTransferFunc func(StateDB, common.Address, *big.Int) bool
	TransferFunc    func(StateDB, common.Address, common.Address, *big.Int)
	// GetHashFunc returns the nth block hash in the blockchain
	// and is used by the BLOCKHASH TiltVM op code.
	GetHashFunc func(uint64) common.Hash
)

// Context provides the TiltVM with auxiliary information. Once provided it shouldn't be modified.
type Context struct {
	// CanTransfer returns whether the account contains
	// sufficient ether to transfer the value
	CanTransfer CanTransferFunc
	// Transfer transfers ether from one account to the other
	Transfer TransferFunc
	// GetHash returns the hash corresponding to n
	GetHash GetHashFunc

	// Message information
	Origin   common.Address // Provides information for ORIGIN
	GasPrice *big.Int       // Provides information for GASPRICE

	// Block information
	Coinbase    common.Address // Provides information for COINBASE
	GasLimit    *big.Int       // Provides information for GASLIMIT
	BlockNumber *big.Int       // Provides information for NUMBER
	Time        *big.Int       // Provides information for TIME
	Difficulty  *big.Int       // Provides information for DIFFICULTY
}

// TiltVM provides information about external sources for the TiltVM
//
// The TiltVM should never be reused and is not thread safe.
type TiltVM struct {
	// Context provides auxiliary blockchain related information
	Context
	// StateDB gives access to the underlying state
	StateDB StateDB
	// Depth is the current call stack
	depth int

	// chainConfig contains information about the current chain
	chainConfig *params.ChainConfig
	// virtual machine configuration options used to initialise the
	// tiltvm.
	vmConfig Config
	// global (to this context) tiltnet virtual machine
	// used throughout the execution of the tx.
	interpreter *Interpreter
	// abort is used to abort the TiltVM calling operations
	// NOTE: must be set atomically
	abort int32
}

// NewTiltVM retutrns a new TiltVM tiltvmironment.
func NewTiltVM(ctx Context, statedb StateDB, chainConfig *params.ChainConfig, vmConfig Config) *TiltVM {
	tiltvm := &TiltVM{
		Context:     ctx,
		StateDB:     statedb,
		vmConfig:    vmConfig,
		chainConfig: chainConfig,
	}

	tiltvm.interpreter = NewInterpreter(tiltvm, vmConfig)
	return tiltvm
}

// Cancel cancels any running TiltVM operation. This may be called concurrently and it's safe to be
// called multiple times.
func (tiltvm *TiltVM) Cancel() {
	atomic.StoreInt32(&tiltvm.abort, 1)
}

// Call executes the contract associated with the addr with the given input as parameters. It also handles any
// necessary value transfer required and takes the necessary steps to create accounts and reverses the state in
// case of an execution error or failed value transfer.
func (tiltvm *TiltVM) Call(caller ContractRef, addr common.Address, input []byte, gas uint64, value *big.Int) (ret []byte, leftOverGas uint64, err error) {
	if tiltvm.vmConfig.NoRecursion && tiltvm.depth > 0 {
		return nil, gas, nil
	}

	// Depth check execution. Fail if we're trying to execute above the
	// limit.
	if tiltvm.depth > int(params.CallCreateDepth) {
		return nil, gas, ErrDepth
	}
	if !tiltvm.Context.CanTransfer(tiltvm.StateDB, caller.Address(), value) {
		return nil, gas, ErrInsufficientBalance
	}

	var (
		to       = AccountRef(addr)
		snapshot = tiltvm.StateDB.Snapshot()
	)
	if !tiltvm.StateDB.Exist(addr) {
		if PrecompiledContracts[addr] == nil && value.Sign() == 0 {
			return nil, gas, nil
		}

		tiltvm.StateDB.CreateAccount(addr)
	}
	tiltvm.Transfer(tiltvm.StateDB, caller.Address(), to.Address(), value)

	// initialise a new contract and set the code that is to be used by the
	// E The contract is a scoped tiltvmironment for this execution context
	// only.
	contract := NewContract(caller, to, value, gas)
	contract.SetCallCode(&addr, tiltvm.StateDB.GetCodeHash(addr), tiltvm.StateDB.GetCode(addr))

	ret, err = tiltvm.interpreter.Run(contract, input)
	// When an error was returned by the TiltVM or when setting the creation code
	// above we revert to the snapshot and consume any gas remaining. Additionally
	// when we're in omaha this also counts for code storage gas errors.
	if err != nil {
		contract.UseGas(contract.Gas)

		tiltvm.StateDB.RevertToSnapshot(snapshot)
	}
	return ret, contract.Gas, err
}

// CallCode executes the contract associated with the addr with the given input as parameters. It also handles any
// necessary value transfer required and takes the necessary steps to create accounts and reverses the state in
// case of an execution error or failed value transfer.
//
// CallCode differs from Call in the sense that it executes the given address' code with the caller as context.
func (tiltvm *TiltVM) CallCode(caller ContractRef, addr common.Address, input []byte, gas uint64, value *big.Int) (ret []byte, leftOverGas uint64, err error) {
	if tiltvm.vmConfig.NoRecursion && tiltvm.depth > 0 {
		return nil, gas, nil
	}

	// Depth check execution. Fail if we're trying to execute above the
	// limit.
	if tiltvm.depth > int(params.CallCreateDepth) {
		return nil, gas, ErrDepth
	}
	if !tiltvm.CanTransfer(tiltvm.StateDB, caller.Address(), value) {
		return nil, gas, ErrInsufficientBalance
	}

	var (
		snapshot = tiltvm.StateDB.Snapshot()
		to       = AccountRef(caller.Address())
	)
	// initialise a new contract and set the code that is to be used by the
	// E The contract is a scoped tiltvmironment for this execution context
	// only.
	contract := NewContract(caller, to, value, gas)
	contract.SetCallCode(&addr, tiltvm.StateDB.GetCodeHash(addr), tiltvm.StateDB.GetCode(addr))

	ret, err = tiltvm.interpreter.Run(contract, input)
	if err != nil {
		contract.UseGas(contract.Gas)

		tiltvm.StateDB.RevertToSnapshot(snapshot)
	}

	return ret, contract.Gas, err
}

// DelegateCall executes the contract associated with the addr with the given input as parameters.
// It reverses the state in case of an execution error.
//
// DelegateCall differs from CallCode in the sense that it executes the given address' code with the caller as context
// and the caller is set to the caller of the caller.
func (tiltvm *TiltVM) DelegateCall(caller ContractRef, addr common.Address, input []byte, gas uint64) (ret []byte, leftOverGas uint64, err error) {
	if tiltvm.vmConfig.NoRecursion && tiltvm.depth > 0 {
		return nil, gas, nil
	}

	// Depth check execution. Fail if we're trying to execute above the
	// limit.
	if tiltvm.depth > int(params.CallCreateDepth) {
		return nil, gas, ErrDepth
	}

	var (
		snapshot = tiltvm.StateDB.Snapshot()
		to       = AccountRef(caller.Address())
	)

	// Iinitialise a new contract and make initialise the delegate values
	contract := NewContract(caller, to, nil, gas).AsDelegate()
	contract.SetCallCode(&addr, tiltvm.StateDB.GetCodeHash(addr), tiltvm.StateDB.GetCode(addr))

	ret, err = tiltvm.interpreter.Run(contract, input)
	if err != nil {
		contract.UseGas(contract.Gas)

		tiltvm.StateDB.RevertToSnapshot(snapshot)
	}

	return ret, contract.Gas, err
}

// Create creates a new contract using code as deployment code.
func (tiltvm *TiltVM) Create(caller ContractRef, code []byte, gas uint64, value *big.Int) (ret []byte, contractAddr common.Address, leftOverGas uint64, err error) {
	if tiltvm.vmConfig.NoRecursion && tiltvm.depth > 0 {
		return nil, common.Address{}, gas, nil
	}

	// Depth check execution. Fail if we're trying to execute above the
	// limit.
	if tiltvm.depth > int(params.CallCreateDepth) {
		return nil, common.Address{}, gas, ErrDepth
	}
	if !tiltvm.CanTransfer(tiltvm.StateDB, caller.Address(), value) {
		return nil, common.Address{}, gas, ErrInsufficientBalance
	}

	// Create a new account on the state
	nonce := tiltvm.StateDB.GetNonce(caller.Address())
	tiltvm.StateDB.SetNonce(caller.Address(), nonce+1)

	snapshot := tiltvm.StateDB.Snapshot()
	contractAddr = crypto.CreateAddress(caller.Address(), nonce)
	tiltvm.StateDB.CreateAccount(contractAddr)
	tiltvm.StateDB.SetNonce(contractAddr, 1)
	tiltvm.Transfer(tiltvm.StateDB, caller.Address(), contractAddr, value)

	// initialise a new contract and set the code that is to be used by the
	// E The contract is a scoped tiltvmironment for this execution context
	// only.
	contract := NewContract(caller, AccountRef(contractAddr), value, gas)
	contract.SetCallCode(&contractAddr, crypto.Keccak256Hash(code), code)

	ret, err = tiltvm.interpreter.Run(contract, nil)

	// check whether the max code size has been exceeded
	maxCodeSizeExceeded := len(ret) > params.MaxCodeSize
	// if the contract creation ran successfully and no errors were returned
	// calculate the gas required to store the code. If the code could not
	// be stored due to not enough gas set an error and let it be handled
	// by the error checking condition below.
	if err == nil && !maxCodeSizeExceeded {
		createDataGas := uint64(len(ret)) * params.CreateDataGas
		if contract.UseGas(createDataGas) {
			tiltvm.StateDB.SetCode(contractAddr, ret)
		} else {
			err = ErrCodeStoreOutOfGas
		}
	}

	// When an error was returned by the TiltVM or when setting the creation code
	// above we revert to the snapshot and consume any gas remaining. Additionally
	// when we're in omaha this also counts for code storage gas errors.
	if maxCodeSizeExceeded ||
		(err != nil && (err != ErrCodeStoreOutOfGas)) {
		tiltvm.StateDB.RevertToSnapshot(snapshot)

		// Nothing should be returned when an error is thrown.
		return nil, contractAddr, 0, err
	}
	// If the vm returned with an error the return value should be set to nil.
	// This isn't consensus critical but merely to for behaviour reasons such as
	// tests, RPC calls, etc.
	if err != nil {
		ret = nil
	}

	return ret, contractAddr, contract.Gas, err
}

// ChainConfig returns the tiltvmironment's chain configuration
func (tiltvm *TiltVM) ChainConfig() *params.ChainConfig { return tiltvm.chainConfig }

// Interpreter returns the TiltVM interpreter
func (tiltvm *TiltVM) Interpreter() *Interpreter { return tiltvm.interpreter }
