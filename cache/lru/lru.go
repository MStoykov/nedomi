// Package lru contains a LRU cache eviction implementation.
package lru

// !TODO: write about the tiered LRU

import (
	"container/list"
	"fmt"
	"log"
	"sync"

	"github.com/ironsmile/nedomi/config"
	"github.com/ironsmile/nedomi/types"
)

const (
	// How many segments are there in the cache. 0 is the "best" segment in sense that
	// it contains the most recent files.
	cacheTiers int = 4
)

// Element is stored in the cache lookup hashmap
type Element struct {
	// Pointer to the linked list element
	ListElem *list.Element

	// In which tier this LRU element is. Tiers are from 0 up to cacheTiers
	ListTier int
}

// TieredLRUCache implements segmented LRU Cache. It has cacheTiers segments.
type TieredLRUCache struct {
	CacheZone *config.CacheZoneSection

	tiers  [cacheTiers]*list.List
	lookup map[types.ObjectIndex]*Element
	mutex  sync.Mutex

	tierListSize int

	removeChan chan<- types.ObjectIndex

	// Used to track cache hit/miss information
	requests uint64
	hits     uint64
}

// Lookup implements part of types.CacheAlgorithm interface
func (tc *TieredLRUCache) Lookup(oi types.ObjectIndex) bool {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	tc.requests++

	_, ok := tc.lookup[oi]

	if ok {
		tc.hits++
	}

	return ok
}

// ShouldKeep implements part of types.CacheAlgorithm interface
func (tc *TieredLRUCache) ShouldKeep(oi types.ObjectIndex) bool {
	err := tc.AddObject(oi)
	if err != nil {
		log.Printf("Error storing object: %s", err)
		return true
	}
	return true
}

// AddObject implements part of types.CacheAlgorithm interface
func (tc *TieredLRUCache) AddObject(oi types.ObjectIndex) error {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	if _, ok := tc.lookup[oi]; ok {
		//!TODO: Create AlreadyInCacheErr type which implements the error interface
		return fmt.Errorf("Object already in cache: %s", oi)
	}

	lastList := tc.tiers[cacheTiers-1]

	if lastList.Len() >= tc.tierListSize {
		tc.freeSpaceInLastList()
	}

	le := &Element{
		ListTier: cacheTiers - 1,
		ListElem: lastList.PushFront(oi),
	}

	log.Printf("Storing %s in cache", oi)
	tc.lookup[oi] = le

	return nil
}

// This function makes space for a new object in a full last list.
// In case there is space in the upper lists it puts its first element upwards.
// In case there is not - it removes its last element to make space.
func (tc *TieredLRUCache) freeSpaceInLastList() {
	lastListInd := cacheTiers - 1
	lastList := tc.tiers[lastListInd]

	if lastList.Len() < 1 {
		log.Println("Last list is empty but cache is trying to free space in it")
		return
	}

	freeList := -1
	for i := lastListInd - 1; i >= 0; i-- {
		if tc.tiers[i].Len() < tc.tierListSize {
			freeList = i
			break
		}
	}

	if freeList != -1 {
		// There is a free space upwards in the list tiers. Move every front list
		// element to the back of the upper tier untill we reach this free slot.
		for i := lastListInd; i > freeList; i-- {
			front := tc.tiers[i].Front()
			if front == nil {
				continue
			}
			val := tc.tiers[i].Remove(front).(types.ObjectIndex)
			valLruEl, ok := tc.lookup[val]
			if !ok {
				log.Printf("ERROR! Object in cache list was not found in the "+
					" lookup map: %v", val)
				i++
				continue
			}
			valLruEl.ListElem = tc.tiers[i-1].PushBack(val)
		}
	} else {
		// There is no free slots anywhere in the upper tiers. So we will have to
		// remove something from the cache in order to make space.
		val := lastList.Remove(lastList.Back()).(types.ObjectIndex)
		tc.remove(val)
		delete(tc.lookup, val)
	}
}

func (tc *TieredLRUCache) remove(oi types.ObjectIndex) {
	log.Printf("Removing %s from cache", oi)
	if tc.removeChan == nil {
		log.Println("Error! LRU cache is trying to write into empty remove channel.")
		return
	}
	tc.removeChan <- oi
}

// ReplaceRemoveChannel implements the types.CacheAlgorithm interface
func (tc *TieredLRUCache) ReplaceRemoveChannel(ch chan<- types.ObjectIndex) {
	tc.removeChan = ch
}

// PromoteObject implements part of types.CacheAlgorithm interface.
// It will reorder the linked lists so that this object index will be promoted in
// rank.
func (tc *TieredLRUCache) PromoteObject(oi types.ObjectIndex) {

	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	lruEl, ok := tc.lookup[oi]

	if !ok {
		// Unlocking the mutex in order to prevent a deadlock while calling
		// AddObject which tries to lock it too.
		tc.mutex.Unlock()

		// This object is not in the cache yet. So we add it.
		if err := tc.AddObject(oi); err != nil {
			log.Printf("Adding object in cache failed. Object: %v\n%s\n", oi, err)
		}

		// The mutex must be locked because of the deferred Unlock
		tc.mutex.Lock()
		return
	}

	if lruEl.ListTier == 0 {
		// This object is in the uppermost tier. It has nowhere to be promoted to
		// but the front of the tier.
		if tc.tiers[lruEl.ListTier].Front() == lruEl.ListElem {
			return
		}
		tc.tiers[lruEl.ListTier].MoveToFront(lruEl.ListElem)
		return
	}

	upperTier := tc.tiers[lruEl.ListTier-1]

	defer func() {
		lruEl.ListTier--
	}()

	if upperTier.Len() < tc.tierListSize {
		// The upper tier is not yet full. So we can push our object at the end
		// of it without needing to remove anything from it.
		tc.tiers[lruEl.ListTier].Remove(lruEl.ListElem)
		lruEl.ListElem = upperTier.PushFront(oi)
		return
	}

	// The upper tier is full. An element from it will be swapped with the one
	// currently promted.
	upperListLastOi := upperTier.Remove(upperTier.Back()).(types.ObjectIndex)
	upperListLastLruEl, ok := tc.lookup[upperListLastOi]

	if !ok {
		log.Println("ERROR! Cache incosistency. Element from the linked list " +
			"was not found in the lookup table")
	} else {
		upperListLastLruEl.ListElem = tc.tiers[lruEl.ListTier].PushFront(upperListLastOi)
	}

	tc.tiers[lruEl.ListTier].Remove(lruEl.ListElem)
	lruEl.ListElem = upperTier.PushFront(oi)

}

// ConsumedSize implements part of types.CacheAlgorithm interface
func (tc *TieredLRUCache) ConsumedSize() types.BytesSize {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	return tc.consumedSize()
}

func (tc *TieredLRUCache) consumedSize() types.BytesSize {
	var sum types.BytesSize

	for i := 0; i < cacheTiers; i++ {
		sum += (tc.CacheZone.PartSize * types.BytesSize(tc.tiers[i].Len()))
	}

	return sum
}

func (tc *TieredLRUCache) init() {
	for i := 0; i < cacheTiers; i++ {
		tc.tiers[i] = list.New()
	}
	tc.lookup = make(map[types.ObjectIndex]*Element)
	tc.tierListSize = int(tc.CacheZone.StorageObjects / uint64(cacheTiers))
}

// New returns TieredLRUCache object ready for use.
func New(cz *config.CacheZoneSection) *TieredLRUCache {
	lru := &TieredLRUCache{CacheZone: cz}
	lru.init()
	return lru
}
