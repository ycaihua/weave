package gossipdns

import (
	"bytes"
	"encoding/gob"
	"sync"
	"time"

	"github.com/weaveworks/weave/ipam/address"
	"github.com/weaveworks/weave/router"
)

const (
	// Tombstones do not need to survive long periods of peer disconnection, as
	// we delete entries for disconnected peers.  Therefore they just need to hang
	// around to account for propagation delay through gossip.  10 minutes sounds
	// long enough.
	tombstoneTimeout = time.Minute * 10
)

// Nameserver: gossip-based, in memory nameserver.
// - Holds a sorted list of (hostname, peer, container id, ip) tuples for the whole cluster.
// - This list is gossiped & merged around the cluser.
// - Lookup-by-hostname are O(nlogn), and return a (copy of a) slice of the entries
// - Update is O(n) for now
type Nameserver struct {
	sync.RWMutex
	ourName router.PeerName
	gossip  router.Gossip
	entries Entries
	quit    chan struct{}
}

func NewNameserver(ourName router.PeerName) *Nameserver {
	ns := &Nameserver{
		ourName: ourName,
	}
	return ns
}

func (n *Nameserver) SetGossip(gossip router.Gossip) {
	n.gossip = gossip
}

func (n *Nameserver) Start() {
	go func() {
		ticker := time.Tick(tombstoneTimeout)
		for {
			select {
			case <-n.quit:
				return
			case <-ticker:
				n.deleteTombstones()
			}
		}
	}()
}

func (n *Nameserver) Stop() {
	n.quit <- struct{}{}
}

func (n *Nameserver) AddEntry(e Entry) error {
	n.Lock()
	newEntries := n.entries.merge(Entries{e})
	n.Unlock()

	return n.gossip.GossipBroadcast(&newEntries)
}

func (n *Nameserver) Lookup(hostname string) []address.Address {
	n.RLock()
	defer n.RUnlock()

	entries := n.entries.lookup(hostname)
	result := []address.Address{}
	for _, e := range entries {
		if e.Tombstone > 0 {
			continue
		}
		result = append(result, e.Addr)
	}
	return result
}

func (n *Nameserver) ReverseLookup(ip address.Address) (string, error) {
	n.RLock()
	defer n.RUnlock()

	match, err := n.entries.first(func(e *Entry) bool {
		return e.Addr == ip
	})
	if err != nil {
		return "", err
	}
	return match.Hostname, nil
}

func (n *Nameserver) ContainerDied(ident string) error {
	n.Lock()
	defer n.Unlock()
	n.entries.tombstone(n.ourName, func(e *Entry) bool {
		return e.ContainerID == ident
	})
	return nil
}

func (n *Nameserver) PeerGone(peer *router.Peer) {
	n.Lock()
	defer n.Unlock()
	n.entries.delete(func(e *Entry) bool {
		return e.Origin == peer.Name
	})
}

func (n *Nameserver) Delete(hostname, containerid, ipStr string, ip address.Address) {
	n.Lock()
	defer n.Unlock()
	n.entries.tombstone(n.ourName, func(e *Entry) bool {
		if hostname != "*" && e.Hostname != hostname {
			return false
		}

		if containerid != "*" && e.ContainerID != containerid {
			return false
		}

		if ipStr != "*" && e.Addr != ip {
			return false
		}

		return true
	})
}

func (n *Nameserver) deleteTombstones() {
	n.Lock()
	defer n.Unlock()
	now := time.Now().Unix()
	n.entries.delete(func(e *Entry) bool {
		return now-e.Tombstone > int64(tombstoneTimeout/time.Second)
	})
}

func (n *Nameserver) Gossip() router.GossipData {
	n.RLock()
	defer n.RUnlock()
	result := make(Entries, len(n.entries))
	copy(result, n.entries)
	return &result
}

func (n *Nameserver) OnGossipUnicast(sender router.PeerName, msg []byte) error {
	return nil
}

func (n *Nameserver) receiveGossip(msg []byte) (router.GossipData, router.GossipData, error) {
	var entries Entries
	if err := gob.NewDecoder(bytes.NewReader(msg)).Decode(&entries); err != nil {
		return nil, nil, err
	}

	if err := entries.check(); err != nil {
		return nil, nil, err
	}

	n.Lock()
	defer n.Unlock()
	newEntries := n.entries.merge(entries)
	return &newEntries, &entries, nil
}

// merge received data into state and return "everything new I've
// just learnt", or nil if nothing in the received data was new
func (n *Nameserver) OnGossip(msg []byte) (router.GossipData, error) {
	newEntries, _, err := n.receiveGossip(msg)
	return newEntries, err
}

// merge received data into state and return a representation of
// the received data, for further propagation
func (n *Nameserver) OnGossipBroadcast(msg []byte) (router.GossipData, error) {
	_, entries, err := n.receiveGossip(msg)
	return entries, err
}
