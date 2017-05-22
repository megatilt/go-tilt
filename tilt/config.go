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

package tilt

import (
	"math/big"
	"os"
	"os/user"
	"path/filepath"
	"runtime"

	"github.com/megatilt/go-tilt/common"
	"github.com/megatilt/go-tilt/common/hexutil"
	"github.com/megatilt/go-tilt/core"
	"github.com/megatilt/go-tilt/tilt/downloader"
	"github.com/megatilt/go-tilt/tilt/gasprice"
	"github.com/megatilt/go-tilt/params"
)

// DefaultConfig contains default settings for use on the Tiltnet main net.
var DefaultConfig = Config{
	SyncMode:             downloader.FullSync,
	TilthashCacheDir:       "tilthash",
	TilthashCachesInMem:    2,
	TilthashCachesOnDisk:   3,
	TilthashDatasetsInMem:  1,
	TilthashDatasetsOnDisk: 2,
	NetworkId:            1,
	DatabaseCache:        128,
	GasPrice:             big.NewInt(20 * params.Blom),

	GPO: gasprice.Config{
		Blocks:     10,
		Percentile: 50,
	},
}

func init() {
	home := os.Getenv("HOME")
	if home == "" {
		if user, err := user.Current(); err == nil {
			home = user.HomeDir
		}
	}
	if runtime.GOOS == "windows" {
		DefaultConfig.TilthashDatasetDir = filepath.Join(home, "AppData", "Tilthash")
	} else {
		DefaultConfig.TilthashDatasetDir = filepath.Join(home, ".tilthash")
	}
}

//go:generate gencodec -type Config -field-override configMarshaling -formats toml -out gen_config.go

type Config struct {
	// The genesis block, which is inserted if the database is empty.
	// If nil, the Tiltnet main net block is used.
	Genesis *core.Genesis `toml:",omitempty"`

	// Protocol options
	NetworkId uint64 // Network ID to use for selecting peers to connect to
	SyncMode  downloader.SyncMode

	MaxPeers int `toml:"-"` // Maximum number of global peers

	// Database options
	SkipBcVersionCheck bool `toml:"-"`
	DatabaseHandles    int  `toml:"-"`
	DatabaseCache      int

	// Mining-related options
	Tiltbase    common.Address `toml:",omitempty"`
	MinerThreads int            `toml:",omitempty"`
	ExtraData    []byte         `toml:",omitempty"`
	GasPrice     *big.Int

	// Tilthash options
	TilthashCacheDir       string
	TilthashCachesInMem    int
	TilthashCachesOnDisk   int
	TilthashDatasetDir     string
	TilthashDatasetsInMem  int
	TilthashDatasetsOnDisk int

	// Gas Price Oracle options
	GPO gasprice.Config

	// Enables tracking of SHA3 preimages in the VM
	EnablePreimageRecording bool

	// Miscellaneous options
	DocRoot   string `toml:"-"`
	PowFake   bool   `toml:"-"`
	PowTest   bool   `toml:"-"`
	PowShared bool   `toml:"-"`
}

type configMarshaling struct {
	ExtraData hexutil.Bytes
}
