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

// Contains the metrics collected by the fetcher.

package fetcher

import (
	"github.com/megatilt/go-tilt/metrics"
)

var (
	propAnnounceInMeter   = metrics.NewMeter("tilt/fetcher/prop/announces/in")
	propAnnounceOutTimer  = metrics.NewTimer("tilt/fetcher/prop/announces/out")
	propAnnounceDropMeter = metrics.NewMeter("tilt/fetcher/prop/announces/drop")
	propAnnounceDOSMeter  = metrics.NewMeter("tilt/fetcher/prop/announces/dos")

	propBroadcastInMeter   = metrics.NewMeter("tilt/fetcher/prop/broadcasts/in")
	propBroadcastOutTimer  = metrics.NewTimer("tilt/fetcher/prop/broadcasts/out")
	propBroadcastDropMeter = metrics.NewMeter("tilt/fetcher/prop/broadcasts/drop")
	propBroadcastDOSMeter  = metrics.NewMeter("tilt/fetcher/prop/broadcasts/dos")

	headerFetchMeter = metrics.NewMeter("tilt/fetcher/fetch/headers")
	bodyFetchMeter   = metrics.NewMeter("tilt/fetcher/fetch/bodies")

	headerFilterInMeter  = metrics.NewMeter("tilt/fetcher/filter/headers/in")
	headerFilterOutMeter = metrics.NewMeter("tilt/fetcher/filter/headers/out")
	bodyFilterInMeter    = metrics.NewMeter("tilt/fetcher/filter/bodies/in")
	bodyFilterOutMeter   = metrics.NewMeter("tilt/fetcher/filter/bodies/out")
)
