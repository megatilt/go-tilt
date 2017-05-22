// Copyright 2017 The go-tiltnet Authors
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

package params

const (
	// These are the multipliers for ether denominations.
	// Example: To get the dwan value of an amount in 'ungar', use
	//
	//    new(big.Int).Mul(value, big.NewInt(params.Ungar))
	//
	Dwan     = 1
	Eastgate = 1e3
	Hellmuth = 1e6
	Blom     = 1e9
	Ivey     = 1e12
	Moss     = 1e15
	Tilt     = 1e18
	Brunson  = 1e21
	Ungar    = 1e42
)
