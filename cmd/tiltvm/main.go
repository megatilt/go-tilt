// Copyright 2014 The go-tiltnet Authors
// This file is part of go-tiltnet.
//
// go-tiltnet is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// go-tiltnet is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with go-tiltnet. If not, see <http://www.gnu.org/licenses/>.

// tiltvm executes TiltVM code snippets.
package main

import (
	"fmt"
	"math/big"
	"os"

	"github.com/megatilt/go-tilt/cmd/utils"
	"gopkg.in/urfave/cli.v1"
)

var gitCommit = "" // Git SHA1 commit hash of the release (set via linker flags)

var (
	app = utils.NewApp(gitCommit, "the tiltvm command line interface")

	DebugFlag = cli.BoolFlag{
		Name:  "debug",
		Usage: "output full trace logs",
	}
	CodeFlag = cli.StringFlag{
		Name:  "code",
		Usage: "TiltVM code",
	}
	CodeFileFlag = cli.StringFlag{
		Name:  "codefile",
		Usage: "file containing TiltVM code",
	}
	GasFlag = cli.Uint64Flag{
		Name:  "gas",
		Usage: "gas limit for the tiltvm",
		Value: 10000000000,
	}
	PriceFlag = utils.BigFlag{
		Name:  "price",
		Usage: "price set for the tiltvm",
		Value: new(big.Int),
	}
	ValueFlag = utils.BigFlag{
		Name:  "value",
		Usage: "value set for the tiltvm",
		Value: new(big.Int),
	}
	DumpFlag = cli.BoolFlag{
		Name:  "dump",
		Usage: "dumps the state after the run",
	}
	InputFlag = cli.StringFlag{
		Name:  "input",
		Usage: "input for the TiltVM",
	}
	VerbosityFlag = cli.IntFlag{
		Name:  "verbosity",
		Usage: "sets the verbosity level",
	}
	CreateFlag = cli.BoolFlag{
		Name:  "create",
		Usage: "indicates the action should be create rather than call",
	}
	DisableGasMeteringFlag = cli.BoolFlag{
		Name:  "nogasmetering",
		Usage: "disable gas metering",
	}
)

func init() {
	app.Flags = []cli.Flag{
		CreateFlag,
		DebugFlag,
		VerbosityFlag,
		CodeFlag,
		CodeFileFlag,
		GasFlag,
		PriceFlag,
		ValueFlag,
		DumpFlag,
		InputFlag,
		DisableGasMeteringFlag,
	}
	app.Commands = []cli.Command{
		compileCommand,
		disasmCommand,
		runCommand,
	}
}

func main() {
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
