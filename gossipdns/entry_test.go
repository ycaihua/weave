package gossipdns

import (
	"reflect"
	"testing"

	"github.com/weaveworks/weave/router"
)

func TestEntries(t *testing.T) {
	e1 := Entries{
		Entry{Hostname: "A"},
		Entry{Hostname: "C"},
		Entry{Hostname: "D"},
		Entry{Hostname: "F"},
	}

	e2 := Entries{
		Entry{Hostname: "B"},
		Entry{Hostname: "E"},
		Entry{Hostname: "F"},
	}

	e1.merge(e2)
	expected := Entries{
		Entry{Hostname: "A"},
		Entry{Hostname: "B"},
		Entry{Hostname: "C"},
		Entry{Hostname: "D"},
		Entry{Hostname: "E"},
		Entry{Hostname: "F"},
	}

	if !reflect.DeepEqual(e1, expected) {
		t.Fatalf("Unexpected: %v", e1)
	}
}

func TestTombstone(t *testing.T) {
	oldNow := now
	defer func() { now = oldNow }()
	now = func() int64 { return 1234 }

	es := Entries{
		Entry{Hostname: "A"},
		Entry{Hostname: "B"},
	}

	es.tombstone(router.UnknownPeerName, func(e *Entry) bool {
		return e.Hostname == "B"
	})
	expected := Entries{
		Entry{Hostname: "A"},
		Entry{Hostname: "B", Version: 1, Tombstone: 1234},
	}
	if !reflect.DeepEqual(es, expected) {
		t.Fatalf("Unexpected: %v", es)
	}
}

func TestDelete(t *testing.T) {
	es := Entries{
		Entry{Hostname: "A"},
		Entry{Hostname: "B"},
	}

	es.delete(func(e *Entry) bool {
		return e.Hostname == "A"
	})
	expected := Entries{
		Entry{Hostname: "B"},
	}
	if !reflect.DeepEqual(es, expected) {
		t.Fatalf("Unexpected: %v", es)
	}
}

func TestLookup(t *testing.T) {
	es := Entries{
		Entry{Hostname: "A"},
		Entry{Hostname: "B", ContainerID: "foo"},
		Entry{Hostname: "B", ContainerID: "bar"},
		Entry{Hostname: "C"},
	}

	have := es.lookup("B")
	want := Entries{
		Entry{Hostname: "B", ContainerID: "foo"},
		Entry{Hostname: "B", ContainerID: "bar"},
	}
	if !reflect.DeepEqual(have, want) {
		t.Fatalf("Unexpected: %v", have)
	}
}
