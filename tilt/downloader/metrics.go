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

// Contains the metrics collected by the downloader.

package downloader

import (
	"github.com/megatilt/go-tilt/metrics"
)

var (
	headerInMeter      = metrics.NewMeter("tilt/downloader/headers/in")
	headerReqTimer     = metrics.NewTimer("tilt/downloader/headers/req")
	headerDropMeter    = metrics.NewMeter("tilt/downloader/headers/drop")
	headerTimeoutMeter = metrics.NewMeter("tilt/downloader/headers/timeout")

	bodyInMeter      = metrics.NewMeter("tilt/downloader/bodies/in")
	bodyReqTimer     = metrics.NewTimer("tilt/downloader/bodies/req")
	bodyDropMeter    = metrics.NewMeter("tilt/downloader/bodies/drop")
	bodyTimeoutMeter = metrics.NewMeter("tilt/downloader/bodies/timeout")

	receiptInMeter      = metrics.NewMeter("tilt/downloader/receipts/in")
	receiptReqTimer     = metrics.NewTimer("tilt/downloader/receipts/req")
	receiptDropMeter    = metrics.NewMeter("tilt/downloader/receipts/drop")
	receiptTimeoutMeter = metrics.NewMeter("tilt/downloader/receipts/timeout")

	stateInMeter      = metrics.NewMeter("tilt/downloader/states/in")
	stateReqTimer     = metrics.NewTimer("tilt/downloader/states/req")
	stateDropMeter    = metrics.NewMeter("tilt/downloader/states/drop")
	stateTimeoutMeter = metrics.NewMeter("tilt/downloader/states/timeout")
)
