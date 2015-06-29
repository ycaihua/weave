package gossipdns

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"sort"
	"time"

	"github.com/weaveworks/weave/ipam/address"
	"github.com/weaveworks/weave/router"
)

var now = func() int64 { return time.Now().Unix() }

type Entry struct {
	ContainerID string
	Origin      router.PeerName
	Addr        address.Address
	Hostname    string
	Version     int
	Tombstone   int64 // timestamp of when it was deleted
}

type Entries []Entry

func (e1 *Entry) equal(e2 *Entry) bool {
	return e1.ContainerID == e2.ContainerID &&
		e1.Origin == e2.Origin &&
		e1.Addr == e2.Addr &&
		e1.Hostname == e2.Hostname
}

func (e1 *Entry) less(e2 *Entry) bool {
	// Entries are kept sorted by Hostname, Origin, ContainerID then address
	if e1.Hostname < e2.Hostname {
		return true
	}

	if e1.Origin < e2.Origin {
		return true
	}

	if e1.ContainerID < e2.ContainerID {
		return true
	}

	return e1.Addr < e2.Addr
}

func (e1 *Entry) merge(e2 *Entry) {
	// we know container id, origin, add and hostname are equal
	if e2.Version > e1.Version {
		e1.Version = e2.Version
		e1.Tombstone = e2.Tombstone
	}
}

func (es Entries) Len() int           { return len(es) }
func (es Entries) Swap(i, j int)      { panic("Swap") }
func (es Entries) Less(i, j int) bool { return es[i].less(&es[j]) }

func (es *Entries) check() error {
	if !sort.IsSorted(es) {
		return fmt.Errorf("Not sorted!")
	}
	return nil
}

func (es *Entries) merge(incoming Entries) Entries {
	var (
		newEntries Entries
		i          = 0
	)

	for _, entry := range incoming {
		for i < len(*es) && (*es)[i].less(&entry) {
			i++
		}
		if i < len(*es) && (*es)[i].equal(&entry) {
			(*es)[i].merge(&entry)
			continue
		}
		*es = append((*es), Entry{})
		copy((*es)[i+1:], (*es)[i:])
		(*es)[i] = entry
		newEntries = append(newEntries, entry)
	}

	return newEntries
}

func (es *Entries) tombstone(ourname router.PeerName, f func(*Entry) bool) {
	for i, e := range *es {
		if f(&e) && e.Origin == ourname {
			e.Version++
			e.Tombstone = now()
			(*es)[i] = e
		}
	}
}

func (es *Entries) delete(f func(*Entry) bool) {
	i := 0
	for _, e := range *es {
		if f(&e) {
			continue
		}
		(*es)[i] = e
		i++
	}
	*es = (*es)[:i]
}

func (es Entries) lookup(hostname string) Entries {
	i := sort.Search(len(es), func(i int) bool {
		return es[i].Hostname >= hostname
	})
	if i >= len(es) || es[i].Hostname != hostname {
		return Entries{}
	}

	j := sort.Search(len(es)-i, func(j int) bool {
		return es[i+j].Hostname > hostname
	})

	return es[i : i+j]
}

func (es *Entries) Merge(other router.GossipData) {
	if err := es.merge(*other.(*Entries)); err != nil {
		panic(err)
	}
}

func (es *Entries) Encode() [][]byte {
	buf := &bytes.Buffer{}
	if err := gob.NewEncoder(buf).Encode(es); err != nil {
		panic(err)
	}
	return [][]byte{buf.Bytes()}
}
