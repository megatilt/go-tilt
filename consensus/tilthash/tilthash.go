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

// Package tilthash implements the tilthash proof-of-work consensus engine.
package tilthash

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"sync"
	"time"
	"unsafe"

	mmap "github.com/edsrzf/mmap-go"
	"github.com/megatilt/go-tilt/consensus"
	"github.com/megatilt/go-tilt/log"
	"github.com/megatilt/go-tilt/rpc"
	metrics "github.com/rcrowley/go-metrics"
)

var ErrInvalidDumpMagic = errors.New("invalid dump magic")

var (
	// maxUint256 is a big integer representing 2^256-1
	maxUint256 = new(big.Int).Exp(big.NewInt(2), big.NewInt(256), big.NewInt(0))

	// sharedTilthash is a full instance that can be shared between multiple users.
	sharedTilthash = New("", 3, 0, "", 1, 0)

	// algorithmRevision is the data structure version used for file naming.
	algorithmRevision = 23

	// dumpMagic is a dataset dump header to sanity check a data dump.
	dumpMagic = []uint32{0xbaddcafe, 0xfee1dead}
)

// isLittleEndian returns whether the local system is running in little or big
// endian byte order.
func isLittleEndian() bool {
	n := uint32(0x01020304)
	return *(*byte)(unsafe.Pointer(&n)) == 0x04
}

// memoryMap tries to memory map a file of uint32s for read only access.
func memoryMap(path string) (*os.File, mmap.MMap, []uint32, error) {
	file, err := os.OpenFile(path, os.O_RDONLY, 0644)
	if err != nil {
		return nil, nil, nil, err
	}
	mem, buffer, err := memoryMapFile(file, false)
	if err != nil {
		file.Close()
		return nil, nil, nil, err
	}
	for i, magic := range dumpMagic {
		if buffer[i] != magic {
			mem.Unmap()
			file.Close()
			return nil, nil, nil, ErrInvalidDumpMagic
		}
	}
	return file, mem, buffer[len(dumpMagic):], err
}

// memoryMapFile tries to memory map an already opened file descriptor.
func memoryMapFile(file *os.File, write bool) (mmap.MMap, []uint32, error) {
	// Try to memory map the file
	flag := mmap.RDONLY
	if write {
		flag = mmap.RDWR
	}
	mem, err := mmap.Map(file, flag, 0)
	if err != nil {
		return nil, nil, err
	}
	// Yay, we managed to memory map the file, here be dragons
	header := *(*reflect.SliceHeader)(unsafe.Pointer(&mem))
	header.Len /= 4
	header.Cap /= 4

	return mem, *(*[]uint32)(unsafe.Pointer(&header)), nil
}

// memoryMapAndGenerate tries to memory map a temporary file of uint32s for write
// access, fill it with the data from a generator and then move it into the final
// path requested.
func memoryMapAndGenerate(path string, size uint64, generator func(buffer []uint32)) (*os.File, mmap.MMap, []uint32, error) {
	// Ensure the data folder exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, nil, nil, err
	}
	// Create a huge temporary empty file to fill with data
	temp := path + "." + strconv.Itoa(rand.Int())

	dump, err := os.Create(temp)
	if err != nil {
		return nil, nil, nil, err
	}
	if err = dump.Truncate(int64(len(dumpMagic))*4 + int64(size)); err != nil {
		return nil, nil, nil, err
	}
	// Memory map the file for writing and fill it with the generator
	mem, buffer, err := memoryMapFile(dump, true)
	if err != nil {
		dump.Close()
		return nil, nil, nil, err
	}
	copy(buffer, dumpMagic)

	data := buffer[len(dumpMagic):]
	generator(data)

	if err := mem.Unmap(); err != nil {
		return nil, nil, nil, err
	}
	if err := dump.Close(); err != nil {
		return nil, nil, nil, err
	}
	if err := os.Rename(temp, path); err != nil {
		return nil, nil, nil, err
	}
	return memoryMap(path)
}

// cache wraps an tilthash cache with some metadata to allow easier concurrent use.
type cache struct {
	epoch uint64 // Epoch for which this cache is relevant

	dump *os.File  // File descriptor of the memory mapped cache
	mmap mmap.MMap // Memory map itself to unmap before releasing

	cache []uint32   // The actual cache data content (may be memory mapped)
	used  time.Time  // Timestamp of the last use for smarter eviction
	once  sync.Once  // Ensures the cache is generated only once
	lock  sync.Mutex // Ensures thread safety for updating the usage time
}

// generate ensures that the cache content is generated before use.
func (c *cache) generate(dir string, limit int, test bool) {
	c.once.Do(func() {
		// If we have a testing cache, generate and return
		if test {
			c.cache = make([]uint32, 1024/4)
			generateCache(c.cache, c.epoch, seedHash(c.epoch*epochLength+1))
			return
		}
		// If we don't store anything on disk, generate and return
		size := cacheSize(c.epoch*epochLength + 1)
		seed := seedHash(c.epoch*epochLength + 1)

		if dir == "" {
			c.cache = make([]uint32, size/4)
			generateCache(c.cache, c.epoch, seed)
			return
		}
		// Disk storage is needed, this will get fancy
		var endian string
		if !isLittleEndian() {
			endian = ".be"
		}
		path := filepath.Join(dir, fmt.Sprintf("cache-R%d-%x%s", algorithmRevision, seed[:8], endian))
		logger := log.New("epoch", c.epoch)

		// Try to load the file from disk and memory map it
		var err error
		c.dump, c.mmap, c.cache, err = memoryMap(path)
		if err == nil {
			logger.Debug("Loaded old tilthash cache from disk")
			return
		}
		logger.Debug("Failed to load old tilthash cache", "err", err)

		// No previous cache available, create a new cache file to fill
		c.dump, c.mmap, c.cache, err = memoryMapAndGenerate(path, size, func(buffer []uint32) { generateCache(buffer, c.epoch, seed) })
		if err != nil {
			logger.Error("Failed to generate mapped tilthash cache", "err", err)

			c.cache = make([]uint32, size/4)
			generateCache(c.cache, c.epoch, seed)
		}
		// Iterate over all previous instances and delete old ones
		for ep := int(c.epoch) - limit; ep >= 0; ep-- {
			seed := seedHash(uint64(ep)*epochLength + 1)
			path := filepath.Join(dir, fmt.Sprintf("cache-R%d-%x%s", algorithmRevision, seed[:8], endian))
			os.Remove(path)
		}
	})
}

// release closes any file handlers and memory maps open.
func (c *cache) release() {
	if c.mmap != nil {
		c.mmap.Unmap()
		c.mmap = nil
	}
	if c.dump != nil {
		c.dump.Close()
		c.dump = nil
	}
}

// dataset wraps an tilthash dataset with some metadata to allow easier concurrent use.
type dataset struct {
	epoch uint64 // Epoch for which this cache is relevant

	dump *os.File  // File descriptor of the memory mapped cache
	mmap mmap.MMap // Memory map itself to unmap before releasing

	dataset []uint32   // The actual cache data content
	used    time.Time  // Timestamp of the last use for smarter eviction
	once    sync.Once  // Ensures the cache is generated only once
	lock    sync.Mutex // Ensures thread safety for updating the usage time
}

// generate ensures that the dataset content is generated before use.
func (d *dataset) generate(dir string, limit int, test bool) {
	d.once.Do(func() {
		// If we have a testing dataset, generate and return
		if test {
			cache := make([]uint32, 1024/4)
			generateCache(cache, d.epoch, seedHash(d.epoch*epochLength+1))

			d.dataset = make([]uint32, 32*1024/4)
			generateDataset(d.dataset, d.epoch, cache)

			return
		}
		// If we don't store anything on disk, generate and return
		csize := cacheSize(d.epoch*epochLength + 1)
		dsize := datasetSize(d.epoch*epochLength + 1)
		seed := seedHash(d.epoch*epochLength + 1)

		if dir == "" {
			cache := make([]uint32, csize/4)
			generateCache(cache, d.epoch, seed)

			d.dataset = make([]uint32, dsize/4)
			generateDataset(d.dataset, d.epoch, cache)
		}
		// Disk storage is needed, this will get fancy
		var endian string
		if !isLittleEndian() {
			endian = ".be"
		}
		path := filepath.Join(dir, fmt.Sprintf("full-R%d-%x%s", algorithmRevision, seed[:8], endian))
		logger := log.New("epoch", d.epoch)

		// Try to load the file from disk and memory map it
		var err error
		d.dump, d.mmap, d.dataset, err = memoryMap(path)
		if err == nil {
			logger.Debug("Loaded old tilthash dataset from disk")
			return
		}
		logger.Debug("Failed to load old tilthash dataset", "err", err)

		// No previous dataset available, create a new dataset file to fill
		cache := make([]uint32, csize/4)
		generateCache(cache, d.epoch, seed)

		d.dump, d.mmap, d.dataset, err = memoryMapAndGenerate(path, dsize, func(buffer []uint32) { generateDataset(buffer, d.epoch, cache) })
		if err != nil {
			logger.Error("Failed to generate mapped tilthash dataset", "err", err)

			d.dataset = make([]uint32, dsize/2)
			generateDataset(d.dataset, d.epoch, cache)
		}
		// Iterate over all previous instances and delete old ones
		for ep := int(d.epoch) - limit; ep >= 0; ep-- {
			seed := seedHash(uint64(ep)*epochLength + 1)
			path := filepath.Join(dir, fmt.Sprintf("full-R%d-%x%s", algorithmRevision, seed[:8], endian))
			os.Remove(path)
		}
	})
}

// release closes any file handlers and memory maps open.
func (d *dataset) release() {
	if d.mmap != nil {
		d.mmap.Unmap()
		d.mmap = nil
	}
	if d.dump != nil {
		d.dump.Close()
		d.dump = nil
	}
}

// MakeCache generates a new tilthash cache and optionally stores it to disk.
func MakeCache(block uint64, dir string) {
	c := cache{epoch: block/epochLength + 1}
	c.generate(dir, math.MaxInt32, false)
	c.release()
}

// MakeDataset generates a new tilthash dataset and optionally stores it to disk.
func MakeDataset(block uint64, dir string) {
	d := dataset{epoch: block/epochLength + 1}
	d.generate(dir, math.MaxInt32, false)
	d.release()
}

// Tilthash is a consensus engine based on proot-of-work implementing the tilthash
// algorithm.
type Tilthash struct {
	cachedir     string // Data directory to store the verification caches
	cachesinmem  int    // Number of caches to keep in memory
	cachesondisk int    // Number of caches to keep on disk
	dagdir       string // Data directory to store full mining datasets
	dagsinmem    int    // Number of mining datasets to keep in memory
	dagsondisk   int    // Number of mining datasets to keep on disk

	caches   map[uint64]*cache   // In memory caches to avoid regenerating too often
	fcache   *cache              // Pre-generated cache for the estimated future epoch
	datasets map[uint64]*dataset // In memory datasets to avoid regenerating too often
	fdataset *dataset            // Pre-generated dataset for the estimated future epoch

	// Mining related fields
	rand     *rand.Rand    // Properly seeded random source for nonces
	threads  int           // Number of threads to mine on if mining
	update   chan struct{} // Notification channel to update mining parameters
	hashrate metrics.Meter // Meter tracking the average hashrate

	// The fields below are hooks for testing
	tester    bool          // Flag whether to use a smaller test dataset
	shared    *Tilthash       // Shared PoW verifier to avoid cache regeneration
	fakeMode  bool          // Flag whether to disable PoW checking
	fakeFull  bool          // Flag whether to disable all consensus rules
	fakeFail  uint64        // Block number which fails PoW check even in fake mode
	fakeDelay time.Duration // Time delay to sleep for before returning from verify

	lock sync.Mutex // Ensures thread safety for the in-memory caches and mining fields
}

// New creates a full sized tilthash PoW scheme.
func New(cachedir string, cachesinmem, cachesondisk int, dagdir string, dagsinmem, dagsondisk int) *Tilthash {
	if cachesinmem <= 0 {
		log.Warn("One tilthash cache must alwast be in memory", "requested", cachesinmem)
		cachesinmem = 1
	}
	if cachedir != "" && cachesondisk > 0 {
		log.Info("Disk storage enabled for tilthash caches", "dir", cachedir, "count", cachesondisk)
	}
	if dagdir != "" && dagsondisk > 0 {
		log.Info("Disk storage enabled for tilthash DAGs", "dir", dagdir, "count", dagsondisk)
	}
	return &Tilthash{
		cachedir:     cachedir,
		cachesinmem:  cachesinmem,
		cachesondisk: cachesondisk,
		dagdir:       dagdir,
		dagsinmem:    dagsinmem,
		dagsondisk:   dagsondisk,
		caches:       make(map[uint64]*cache),
		datasets:     make(map[uint64]*dataset),
		update:       make(chan struct{}),
		hashrate:     metrics.NewMeter(),
	}
}

// NewTester creates a small sized tilthash PoW scheme useful only for testing
// purposes.
func NewTester() *Tilthash {
	return &Tilthash{
		cachesinmem: 1,
		caches:      make(map[uint64]*cache),
		datasets:    make(map[uint64]*dataset),
		tester:      true,
		update:      make(chan struct{}),
		hashrate:    metrics.NewMeter(),
	}
}

// NewFaker creates a tilthash consensus engine with a fake PoW scheme that accepts
// all blocks' seal as valid, though they still have to conform to the Tiltnet
// consensus rules.
func NewFaker() *Tilthash {
	return &Tilthash{fakeMode: true}
}

// NewFakeFailer creates a tilthash consensus engine with a fake PoW scheme that
// accepts all blocks as valid apart from the single one specified, though they
// still have to conform to the Tiltnet consensus rules.
func NewFakeFailer(fail uint64) *Tilthash {
	return &Tilthash{fakeMode: true, fakeFail: fail}
}

// NewFakeDelayer creates a tilthash consensus engine with a fake PoW scheme that
// accepts all blocks as valid, but delays verifications by some time, though
// they still have to conform to the Tiltnet consensus rules.
func NewFakeDelayer(delay time.Duration) *Tilthash {
	return &Tilthash{fakeMode: true, fakeDelay: delay}
}

// NewFullFaker creates a tilthash consensus engine with a full fake scheme that
// accepts all blocks as valid, without checking any consensus rules whatsoever.
func NewFullFaker() *Tilthash {
	return &Tilthash{fakeMode: true, fakeFull: true}
}

// NewShared creates a full sized tilthash PoW shared between all requesters running
// in the same process.
func NewShared() *Tilthash {
	return &Tilthash{shared: sharedTilthash}
}

// cache tries to retrieve a verification cache for the specified block number
// by first checking against a list of in-memory caches, then against caches
// stored on disk, and finally generating one if none can be found.
func (tilthash *Tilthash) cache(block uint64) []uint32 {
	epoch := block / epochLength

	// If we have a PoW for that epoch, use that
	tilthash.lock.Lock()

	current, future := tilthash.caches[epoch], (*cache)(nil)
	if current == nil {
		// No in-memory cache, evict the oldest if the cache limit was reached
		for len(tilthash.caches) > 0 && len(tilthash.caches) >= tilthash.cachesinmem {
			var evict *cache
			for _, cache := range tilthash.caches {
				if evict == nil || evict.used.After(cache.used) {
					evict = cache
				}
			}
			delete(tilthash.caches, evict.epoch)
			evict.release()

			log.Trace("Evicted tilthash cache", "epoch", evict.epoch, "used", evict.used)
		}
		// If we have the new cache pre-generated, use that, otherwise create a new one
		if tilthash.fcache != nil && tilthash.fcache.epoch == epoch {
			log.Trace("Using pre-generated cache", "epoch", epoch)
			current, tilthash.fcache = tilthash.fcache, nil
		} else {
			log.Trace("Requiring new tilthash cache", "epoch", epoch)
			current = &cache{epoch: epoch}
		}
		tilthash.caches[epoch] = current

		// If we just used up the future cache, or need a refresh, regenerate
		if tilthash.fcache == nil || tilthash.fcache.epoch <= epoch {
			if tilthash.fcache != nil {
				tilthash.fcache.release()
			}
			log.Trace("Requiring new future tilthash cache", "epoch", epoch+1)
			future = &cache{epoch: epoch + 1}
			tilthash.fcache = future
		}
		// New current cache, set its initial timestamp
		current.used = time.Now()
	}
	tilthash.lock.Unlock()

	// Wait for generation finish, bump the timestamp and finalize the cache
	current.generate(tilthash.cachedir, tilthash.cachesondisk, tilthash.tester)

	current.lock.Lock()
	current.used = time.Now()
	current.lock.Unlock()

	// If we exhausted the future cache, now's a good time to regenerate it
	if future != nil {
		go future.generate(tilthash.cachedir, tilthash.cachesondisk, tilthash.tester)
	}
	return current.cache
}

// dataset tries to retrieve a mining dataset for the specified block number
// by first checking against a list of in-memory datasets, then against DAGs
// stored on disk, and finally generating one if none can be found.
func (tilthash *Tilthash) dataset(block uint64) []uint32 {
	epoch := block / epochLength

	// If we have a PoW for that epoch, use that
	tilthash.lock.Lock()

	current, future := tilthash.datasets[epoch], (*dataset)(nil)
	if current == nil {
		// No in-memory dataset, evict the oldest if the dataset limit was reached
		for len(tilthash.datasets) > 0 && len(tilthash.datasets) >= tilthash.dagsinmem {
			var evict *dataset
			for _, dataset := range tilthash.datasets {
				if evict == nil || evict.used.After(dataset.used) {
					evict = dataset
				}
			}
			delete(tilthash.datasets, evict.epoch)
			evict.release()

			log.Trace("Evicted tilthash dataset", "epoch", evict.epoch, "used", evict.used)
		}
		// If we have the new cache pre-generated, use that, otherwise create a new one
		if tilthash.fdataset != nil && tilthash.fdataset.epoch == epoch {
			log.Trace("Using pre-generated dataset", "epoch", epoch)
			current = &dataset{epoch: tilthash.fdataset.epoch} // Reload from disk
			tilthash.fdataset = nil
		} else {
			log.Trace("Requiring new tilthash dataset", "epoch", epoch)
			current = &dataset{epoch: epoch}
		}
		tilthash.datasets[epoch] = current

		// If we just used up the future dataset, or need a refresh, regenerate
		if tilthash.fdataset == nil || tilthash.fdataset.epoch <= epoch {
			if tilthash.fdataset != nil {
				tilthash.fdataset.release()
			}
			log.Trace("Requiring new future tilthash dataset", "epoch", epoch+1)
			future = &dataset{epoch: epoch + 1}
			tilthash.fdataset = future
		}
		// New current dataset, set its initial timestamp
		current.used = time.Now()
	}
	tilthash.lock.Unlock()

	// Wait for generation finish, bump the timestamp and finalize the cache
	current.generate(tilthash.dagdir, tilthash.dagsondisk, tilthash.tester)

	current.lock.Lock()
	current.used = time.Now()
	current.lock.Unlock()

	// If we exhausted the future dataset, now's a good time to regenerate it
	if future != nil {
		go future.generate(tilthash.dagdir, tilthash.dagsondisk, tilthash.tester)
	}
	return current.dataset
}

// Threads returns the number of mining threads currently enabled. This doesn't
// necessarily mean that mining is running!
func (tilthash *Tilthash) Threads() int {
	tilthash.lock.Lock()
	defer tilthash.lock.Unlock()

	return tilthash.threads
}

// SetThreads updates the number of mining threads currently enabled. Calling
// this method does not start mining, only sets the thread count. If zero is
// specified, the miner will use all cores of the machine. Setting a thread
// count below zero is allowed and will cause the miner to idle, without any
// work being done.
func (tilthash *Tilthash) SetThreads(threads int) {
	tilthash.lock.Lock()
	defer tilthash.lock.Unlock()

	// If we're running a shared PoW, set the thread count on that instead
	if tilthash.shared != nil {
		tilthash.shared.SetThreads(threads)
		return
	}
	// Update the threads and ping any running seal to pull in any changes
	tilthash.threads = threads
	select {
	case tilthash.update <- struct{}{}:
	default:
	}
}

// Hashrate implements PoW, returning the measured rate of the search invocations
// per second over the last minute.
func (tilthash *Tilthash) Hashrate() float64 {
	return tilthash.hashrate.Rate1()
}

// APIs implements consensus.Engine, returning the user facing RPC APIs. Currently
// that is empty.
func (tilthash *Tilthash) APIs(chain consensus.ChainReader) []rpc.API {
	return nil
}

// SeedHash is the seed to use for generating a verification cache and the mining
// dataset.
func SeedHash(block uint64) []byte {
	return seedHash(block)
}
