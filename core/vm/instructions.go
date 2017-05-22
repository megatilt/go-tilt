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

package vm

import (
	"fmt"
	"math/big"

	"github.com/megatilt/go-tilt/common"
	"github.com/megatilt/go-tilt/common/math"
	"github.com/megatilt/go-tilt/core/types"
	"github.com/megatilt/go-tilt/crypto"
	"github.com/megatilt/go-tilt/params"
)

var bigZero = new(big.Int)

func opAdd(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x, y := stack.pop(), stack.pop()
	stack.push(math.U256(x.Add(x, y)))

	tiltvm.interpreter.intPool.put(y)

	return nil, nil
}

func opSub(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x, y := stack.pop(), stack.pop()
	stack.push(math.U256(x.Sub(x, y)))

	tiltvm.interpreter.intPool.put(y)

	return nil, nil
}

func opMul(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x, y := stack.pop(), stack.pop()
	stack.push(math.U256(x.Mul(x, y)))

	tiltvm.interpreter.intPool.put(y)

	return nil, nil
}

func opDiv(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x, y := stack.pop(), stack.pop()
	if y.Sign() != 0 {
		stack.push(math.U256(x.Div(x, y)))
	} else {
		stack.push(new(big.Int))
	}

	tiltvm.interpreter.intPool.put(y)

	return nil, nil
}

func opSdiv(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x, y := math.S256(stack.pop()), math.S256(stack.pop())
	if y.Sign() == 0 {
		stack.push(new(big.Int))
		return nil, nil
	} else {
		n := new(big.Int)
		if tiltvm.interpreter.intPool.get().Mul(x, y).Sign() < 0 {
			n.SetInt64(-1)
		} else {
			n.SetInt64(1)
		}

		res := x.Div(x.Abs(x), y.Abs(y))
		res.Mul(res, n)

		stack.push(math.U256(res))
	}
	tiltvm.interpreter.intPool.put(y)
	return nil, nil
}

func opMod(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x, y := stack.pop(), stack.pop()
	if y.Sign() == 0 {
		stack.push(new(big.Int))
	} else {
		stack.push(math.U256(x.Mod(x, y)))
	}
	tiltvm.interpreter.intPool.put(y)
	return nil, nil
}

func opSmod(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x, y := math.S256(stack.pop()), math.S256(stack.pop())

	if y.Sign() == 0 {
		stack.push(new(big.Int))
	} else {
		n := new(big.Int)
		if x.Sign() < 0 {
			n.SetInt64(-1)
		} else {
			n.SetInt64(1)
		}

		res := x.Mod(x.Abs(x), y.Abs(y))
		res.Mul(res, n)

		stack.push(math.U256(res))
	}
	tiltvm.interpreter.intPool.put(y)
	return nil, nil
}

func opExp(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	base, exponent := stack.pop(), stack.pop()
	stack.push(math.Exp(base, exponent))

	tiltvm.interpreter.intPool.put(base, exponent)

	return nil, nil
}

func opSignExtend(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	back := stack.pop()
	if back.Cmp(big.NewInt(31)) < 0 {
		bit := uint(back.Uint64()*8 + 7)
		num := stack.pop()
		mask := back.Lsh(common.Big1, bit)
		mask.Sub(mask, common.Big1)
		if num.Bit(int(bit)) > 0 {
			num.Or(num, mask.Not(mask))
		} else {
			num.And(num, mask)
		}

		stack.push(math.U256(num))
	}

	tiltvm.interpreter.intPool.put(back)
	return nil, nil
}

func opNot(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x := stack.pop()
	stack.push(math.U256(x.Not(x)))
	return nil, nil
}

func opLt(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x, y := stack.pop(), stack.pop()
	if x.Cmp(y) < 0 {
		stack.push(tiltvm.interpreter.intPool.get().SetUint64(1))
	} else {
		stack.push(new(big.Int))
	}

	tiltvm.interpreter.intPool.put(x, y)
	return nil, nil
}

func opGt(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x, y := stack.pop(), stack.pop()
	if x.Cmp(y) > 0 {
		stack.push(tiltvm.interpreter.intPool.get().SetUint64(1))
	} else {
		stack.push(new(big.Int))
	}

	tiltvm.interpreter.intPool.put(x, y)
	return nil, nil
}

func opSlt(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x, y := math.S256(stack.pop()), math.S256(stack.pop())
	if x.Cmp(math.S256(y)) < 0 {
		stack.push(tiltvm.interpreter.intPool.get().SetUint64(1))
	} else {
		stack.push(new(big.Int))
	}

	tiltvm.interpreter.intPool.put(x, y)
	return nil, nil
}

func opSgt(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x, y := math.S256(stack.pop()), math.S256(stack.pop())
	if x.Cmp(y) > 0 {
		stack.push(tiltvm.interpreter.intPool.get().SetUint64(1))
	} else {
		stack.push(new(big.Int))
	}

	tiltvm.interpreter.intPool.put(x, y)
	return nil, nil
}

func opEq(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x, y := stack.pop(), stack.pop()
	if x.Cmp(y) == 0 {
		stack.push(tiltvm.interpreter.intPool.get().SetUint64(1))
	} else {
		stack.push(new(big.Int))
	}

	tiltvm.interpreter.intPool.put(x, y)
	return nil, nil
}

func opIszero(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x := stack.pop()
	if x.Sign() > 0 {
		stack.push(new(big.Int))
	} else {
		stack.push(tiltvm.interpreter.intPool.get().SetUint64(1))
	}

	tiltvm.interpreter.intPool.put(x)
	return nil, nil
}

func opAnd(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x, y := stack.pop(), stack.pop()
	stack.push(x.And(x, y))

	tiltvm.interpreter.intPool.put(y)
	return nil, nil
}
func opOr(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x, y := stack.pop(), stack.pop()
	stack.push(x.Or(x, y))

	tiltvm.interpreter.intPool.put(y)
	return nil, nil
}
func opXor(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x, y := stack.pop(), stack.pop()
	stack.push(x.Xor(x, y))

	tiltvm.interpreter.intPool.put(y)
	return nil, nil
}

func opByte(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	th, val := stack.pop(), stack.pop()
	if th.Cmp(big.NewInt(32)) < 0 {
		byte := tiltvm.interpreter.intPool.get().SetInt64(int64(math.PaddedBigBytes(val, 32)[th.Int64()]))
		stack.push(byte)
	} else {
		stack.push(new(big.Int))
	}

	tiltvm.interpreter.intPool.put(th, val)
	return nil, nil
}
func opAddmod(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x, y, z := stack.pop(), stack.pop(), stack.pop()
	if z.Cmp(bigZero) > 0 {
		add := x.Add(x, y)
		add.Mod(add, z)
		stack.push(math.U256(add))
	} else {
		stack.push(new(big.Int))
	}

	tiltvm.interpreter.intPool.put(y, z)
	return nil, nil
}
func opMulmod(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	x, y, z := stack.pop(), stack.pop(), stack.pop()
	if z.Cmp(bigZero) > 0 {
		mul := x.Mul(x, y)
		mul.Mod(mul, z)
		stack.push(math.U256(mul))
	} else {
		stack.push(new(big.Int))
	}

	tiltvm.interpreter.intPool.put(y, z)
	return nil, nil
}

func opSha3(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	offset, size := stack.pop(), stack.pop()
	data := memory.Get(offset.Int64(), size.Int64())
	hash := crypto.Keccak256(data)

	if tiltvm.vmConfig.EnablePreimageRecording {
		tiltvm.StateDB.AddPreimage(common.BytesToHash(hash), data)
	}

	stack.push(new(big.Int).SetBytes(hash))

	tiltvm.interpreter.intPool.put(offset, size)
	return nil, nil
}

func opAddress(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	stack.push(contract.Address().Big())
	return nil, nil
}

func opBalance(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	addr := common.BigToAddress(stack.pop())
	balance := tiltvm.StateDB.GetBalance(addr)

	stack.push(new(big.Int).Set(balance))
	return nil, nil
}

func opOrigin(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	stack.push(tiltvm.Origin.Big())
	return nil, nil
}

func opCaller(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	stack.push(contract.Caller().Big())
	return nil, nil
}

func opCallValue(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	stack.push(tiltvm.interpreter.intPool.get().Set(contract.value))
	return nil, nil
}

func opCalldataLoad(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	stack.push(new(big.Int).SetBytes(getData(contract.Input, stack.pop(), common.Big32)))
	return nil, nil
}

func opCalldataSize(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	stack.push(tiltvm.interpreter.intPool.get().SetInt64(int64(len(contract.Input))))
	return nil, nil
}

func opCalldataCopy(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	var (
		mOff = stack.pop()
		cOff = stack.pop()
		l    = stack.pop()
	)
	memory.Set(mOff.Uint64(), l.Uint64(), getData(contract.Input, cOff, l))

	tiltvm.interpreter.intPool.put(mOff, cOff, l)
	return nil, nil
}

func opExtCodeSize(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	a := stack.pop()

	addr := common.BigToAddress(a)
	a.SetInt64(int64(tiltvm.StateDB.GetCodeSize(addr)))
	stack.push(a)

	return nil, nil
}

func opCodeSize(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	l := tiltvm.interpreter.intPool.get().SetInt64(int64(len(contract.Code)))
	stack.push(l)
	return nil, nil
}

func opCodeCopy(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	var (
		mOff = stack.pop()
		cOff = stack.pop()
		l    = stack.pop()
	)
	codeCopy := getData(contract.Code, cOff, l)

	memory.Set(mOff.Uint64(), l.Uint64(), codeCopy)

	tiltvm.interpreter.intPool.put(mOff, cOff, l)
	return nil, nil
}

func opExtCodeCopy(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	var (
		addr = common.BigToAddress(stack.pop())
		mOff = stack.pop()
		cOff = stack.pop()
		l    = stack.pop()
	)
	codeCopy := getData(tiltvm.StateDB.GetCode(addr), cOff, l)

	memory.Set(mOff.Uint64(), l.Uint64(), codeCopy)

	tiltvm.interpreter.intPool.put(mOff, cOff, l)

	return nil, nil
}

func opGasprice(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	stack.push(tiltvm.interpreter.intPool.get().Set(tiltvm.GasPrice))
	return nil, nil
}

func opBlockhash(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	num := stack.pop()

	n := tiltvm.interpreter.intPool.get().Sub(tiltvm.BlockNumber, common.Big257)
	if num.Cmp(n) > 0 && num.Cmp(tiltvm.BlockNumber) < 0 {
		stack.push(tiltvm.GetHash(num.Uint64()).Big())
	} else {
		stack.push(new(big.Int))
	}

	tiltvm.interpreter.intPool.put(num, n)
	return nil, nil
}

func opCoinbase(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	stack.push(tiltvm.Coinbase.Big())
	return nil, nil
}

func opTimestamp(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	stack.push(math.U256(new(big.Int).Set(tiltvm.Time)))
	return nil, nil
}

func opNumber(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	stack.push(math.U256(new(big.Int).Set(tiltvm.BlockNumber)))
	return nil, nil
}

func opDifficulty(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	stack.push(math.U256(new(big.Int).Set(tiltvm.Difficulty)))
	return nil, nil
}

func opGasLimit(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	stack.push(math.U256(new(big.Int).Set(tiltvm.GasLimit)))
	return nil, nil
}

func opPop(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	tiltvm.interpreter.intPool.put(stack.pop())
	return nil, nil
}

func opMload(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	offset := stack.pop()
	val := new(big.Int).SetBytes(memory.Get(offset.Int64(), 32))
	stack.push(val)

	tiltvm.interpreter.intPool.put(offset)
	return nil, nil
}

func opMstore(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	// pop value of the stack
	mStart, val := stack.pop(), stack.pop()
	memory.Set(mStart.Uint64(), 32, math.PaddedBigBytes(val, 32))

	tiltvm.interpreter.intPool.put(mStart, val)
	return nil, nil
}

func opMstore8(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	off, val := stack.pop().Int64(), stack.pop().Int64()
	memory.store[off] = byte(val & 0xff)

	return nil, nil
}

func opSload(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	loc := common.BigToHash(stack.pop())
	val := tiltvm.StateDB.GetState(contract.Address(), loc).Big()
	stack.push(val)
	return nil, nil
}

func opSstore(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	loc := common.BigToHash(stack.pop())
	val := stack.pop()
	tiltvm.StateDB.SetState(contract.Address(), loc, common.BigToHash(val))

	tiltvm.interpreter.intPool.put(val)
	return nil, nil
}

func opJump(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	pos := stack.pop()
	if !contract.jumpdests.has(contract.CodeHash, contract.Code, pos) {
		nop := contract.GetOp(pos.Uint64())
		return nil, fmt.Errorf("invalid jump destination (%v) %v", nop, pos)
	}
	*pc = pos.Uint64()

	tiltvm.interpreter.intPool.put(pos)
	return nil, nil
}
func opJumpi(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	pos, cond := stack.pop(), stack.pop()
	if cond.Sign() != 0 {
		if !contract.jumpdests.has(contract.CodeHash, contract.Code, pos) {
			nop := contract.GetOp(pos.Uint64())
			return nil, fmt.Errorf("invalid jump destination (%v) %v", nop, pos)
		}
		*pc = pos.Uint64()
	} else {
		*pc++
	}

	tiltvm.interpreter.intPool.put(pos, cond)
	return nil, nil
}
func opJumpdest(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	return nil, nil
}

func opPc(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	stack.push(tiltvm.interpreter.intPool.get().SetUint64(*pc))
	return nil, nil
}

func opMsize(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	stack.push(tiltvm.interpreter.intPool.get().SetInt64(int64(memory.Len())))
	return nil, nil
}

func opGas(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	stack.push(tiltvm.interpreter.intPool.get().SetUint64(contract.Gas))
	return nil, nil
}

func opCreate(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	var (
		value        = stack.pop()
		offset, size = stack.pop(), stack.pop()
		input        = memory.Get(offset.Int64(), size.Int64())
		gas          = contract.Gas
	)
	gas -= gas / 64

	contract.UseGas(gas)
	_, addr, returnGas, suberr := tiltvm.Create(contract, input, gas, value)
	// Push item on the stack based on the returned error. If the ruleset is
	// omaha we must check for CodeStoreOutOfGasError (omaha only
	// rule) and treat as an error, if the ruleset is holdem we must
	// ignore this error and pretend the operation was successful.
	if suberr == ErrCodeStoreOutOfGas {
		stack.push(new(big.Int))
	} else if suberr != nil && suberr != ErrCodeStoreOutOfGas {
		stack.push(new(big.Int))
	} else {
		stack.push(addr.Big())
	}
	contract.Gas += returnGas

	tiltvm.interpreter.intPool.put(value, offset, size)

	return nil, nil
}

func opCall(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	gas := stack.pop().Uint64()
	// pop gas and value of the stack.
	addr, value := stack.pop(), stack.pop()
	value = math.U256(value)
	// pop input size and offset
	inOffset, inSize := stack.pop(), stack.pop()
	// pop return size and offset
	retOffset, retSize := stack.pop(), stack.pop()

	address := common.BigToAddress(addr)

	// Get the arguments from the memory
	args := memory.Get(inOffset.Int64(), inSize.Int64())

	if value.Sign() != 0 {
		gas += params.CallStipend
	}

	ret, returnGas, err := tiltvm.Call(contract, address, args, gas, value)
	if err != nil {
		stack.push(new(big.Int))
	} else {
		stack.push(big.NewInt(1))

		memory.Set(retOffset.Uint64(), retSize.Uint64(), ret)
	}
	contract.Gas += returnGas

	tiltvm.interpreter.intPool.put(addr, value, inOffset, inSize, retOffset, retSize)
	return nil, nil
}

func opCallCode(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	gas := stack.pop().Uint64()
	// pop gas and value of the stack.
	addr, value := stack.pop(), stack.pop()
	value = math.U256(value)
	// pop input size and offset
	inOffset, inSize := stack.pop(), stack.pop()
	// pop return size and offset
	retOffset, retSize := stack.pop(), stack.pop()

	address := common.BigToAddress(addr)

	// Get the arguments from the memory
	args := memory.Get(inOffset.Int64(), inSize.Int64())

	if value.Sign() != 0 {
		gas += params.CallStipend
	}

	ret, returnGas, err := tiltvm.CallCode(contract, address, args, gas, value)
	if err != nil {
		stack.push(new(big.Int))

	} else {
		stack.push(big.NewInt(1))

		memory.Set(retOffset.Uint64(), retSize.Uint64(), ret)
	}
	contract.Gas += returnGas

	tiltvm.interpreter.intPool.put(addr, value, inOffset, inSize, retOffset, retSize)
	return nil, nil
}

func opDelegateCall(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {

	gas, to, inOffset, inSize, outOffset, outSize := stack.pop().Uint64(), stack.pop(), stack.pop(), stack.pop(), stack.pop(), stack.pop()

	toAddr := common.BigToAddress(to)
	args := memory.Get(inOffset.Int64(), inSize.Int64())

	ret, returnGas, err := tiltvm.DelegateCall(contract, toAddr, args, gas)
	if err != nil {
		stack.push(new(big.Int))
	} else {
		stack.push(big.NewInt(1))
		memory.Set(outOffset.Uint64(), outSize.Uint64(), ret)
	}
	contract.Gas += returnGas

	tiltvm.interpreter.intPool.put(to, inOffset, inSize, outOffset, outSize)
	return nil, nil
}

func opReturn(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	offset, size := stack.pop(), stack.pop()
	ret := memory.GetPtr(offset.Int64(), size.Int64())

	tiltvm.interpreter.intPool.put(offset, size)
	return ret, nil
}

func opStop(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	return nil, nil
}

func opSuicide(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
	balance := tiltvm.StateDB.GetBalance(contract.Address())
	tiltvm.StateDB.AddBalance(common.BigToAddress(stack.pop()), balance)

	tiltvm.StateDB.Suicide(contract.Address())

	return nil, nil
}

// following functions are used by the instruction jump  table

// make log instruction function
func makeLog(size int) executionFunc {
	return func(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
		topics := make([]common.Hash, size)
		mStart, mSize := stack.pop(), stack.pop()
		for i := 0; i < size; i++ {
			topics[i] = common.BigToHash(stack.pop())
		}

		d := memory.Get(mStart.Int64(), mSize.Int64())
		tiltvm.StateDB.AddLog(&types.Log{
			Address: contract.Address(),
			Topics:  topics,
			Data:    d,
			// This is a non-consensus field, but assigned here because
			// core/state doesn't know the current block number.
			BlockNumber: tiltvm.BlockNumber.Uint64(),
		})

		tiltvm.interpreter.intPool.put(mStart, mSize)
		return nil, nil
	}
}

// make push instruction function
func makePush(size uint64, bsize *big.Int) executionFunc {
	return func(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
		byts := getData(contract.Code, tiltvm.interpreter.intPool.get().SetUint64(*pc+1), bsize)
		stack.push(new(big.Int).SetBytes(byts))
		*pc += size
		return nil, nil
	}
}

// make push instruction function
func makeDup(size int64) executionFunc {
	return func(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
		stack.dup(int(size))
		return nil, nil
	}
}

// make swap instruction function
func makeSwap(size int64) executionFunc {
	// switch n + 1 otherwise n would be swapped with n
	size += 1
	return func(pc *uint64, tiltvm *TiltVM, contract *Contract, memory *Memory, stack *Stack) ([]byte, error) {
		stack.swap(int(size))
		return nil, nil
	}
}
