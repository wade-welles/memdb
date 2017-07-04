package memdb

import (
	"github.com/google/btree"

	"sort"
	"strings"
	"testing"
)

var expired = 0

type X struct {
	a int
	b string
	c string
}

func (x *X) Less(o Indexer) bool {
	return x.a < o.(*X).a
}
func (x *X) IsExpired() bool {
	return x.a == expired
}
func (x *X) GetField(f string) string {
	if f == "c" {
		return x.c
	}
	return x.b
}

func TestCreateField(t *testing.T) {
	s := NewStore()
	s.CreateField("test")

	f := s.Fields()
	if len(f) != 1 {
		t.Errorf("Fields length should be 1 (is %d)", len(f))
	}
	if f[0] == nil || len(f[0]) != 1 || f[0][0] != "test" {
		t.Errorf("Fields should be []string{\"test\"} (is: %#v)", f)
	}
}

func TestCreateAfterStore(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
	}()

	s := NewStore()
	s.Put(&X{})
	s.CreateField("b")
}

func TestGet(t *testing.T) {
	s := NewStore()
	orig := &X{a: 1}
	s.Put(orig)
	v := s.Get(&X{a: 1})
	if orig != v {
		t.Errorf("Gotten value should be same as original instance")
	}
}

func TestNoGet(t *testing.T) {
	s := NewStore()
	orig := &X{a: 1}
	s.Put(orig)
	v := s.Get(&X{a: 2})
	if v != nil {
		t.Errorf("Gotten value should be nil (not present)")
	}
}

func TestLookup(t *testing.T) {
	s := NewStore()
	s.CreateField("b")
	s.Put(&X{a: 1, b: "test"})
	s.Put(&X{a: 2, b: "test"})
	s.Put(&X{a: 3, b: "not"})
	vals := s.In("b").Lookup("test")
	if len(vals) != 2 {
		t.Errorf("Length of looked up values should be 2 (was %s)", len(vals))
	}
	a := vals[0].(*X)
	b := vals[1].(*X)
	if (a.a == 1 && b.a == 2) || (a.a == 2 && b.a == 1) {
		return
	}
	t.Errorf("Expected to return 1 and 2 (got %#v)", vals)
}

func TestLookupInvalidField(t *testing.T) {
	s := NewStore()
	s.CreateField("b")
	s.Put(&X{a: 1, b: "test"})
	vals := s.In("c").Lookup("test")
	if vals != nil {
		t.Errorf("Lookup of invalid field should be nil (was %#v)", vals)
	}
}

func TestLookupNonPresentKey(t *testing.T) {
	s := NewStore()
	s.CreateField("b")
	s.Put(&X{a: 1, b: "test"})
	vals := s.In("b").Lookup("dumb")
	if vals != nil {
		t.Errorf("Lookup of non-present key should be nil (was %#v)", vals)
	}
}

func TestTraverse(t *testing.T) {
	s := NewStore()
	s.CreateField("b")
	v1 := &X{a: 1, b: "one:"}
	v2 := &X{a: 2, b: "two:"}
	v3 := &X{a: 3, b: "three:"}
	v4 := &X{a: 4, b: "four:"}
	v8 := &X{a: 8, b: "eight:"}

	s.Put(v1)
	s.Put(v2)
	s.Put(v4)
	s.Put(v8)

	n := s.Len()
	if n != 4 {
		t.Errorf("Expected 4 items in length (got %d)", n)
	}

	k := s.Keys("b")
	if len(k) != 4 {
		t.Errorf("Expected 4 items in keys for field (got %#v)", k)
	}

	sort.Strings(k)
	j := strings.Join(k, "")
	if j != "eight:four:one:two:" {
		t.Errorf("Unexpected items in keys for field (got %s)", j)
	}

	var got, expect string
	var stop *X

	iter := func(i Indexer) bool {
		got += i.(*X).b
		return i != stop
	}

	got = ""
	s.Ascend(iter)
	expect = "one:two:four:eight:"
	if got != expect {
		t.Errorf("Traversed in wrong direction expected %s (got %s)", expect, got)
	}

	got = ""
	s.Descend(iter)
	expect = "eight:four:two:one:"
	if got != expect {
		t.Errorf("Traversed in wrong direction expected %s (got %s)", expect, got)
	}

	got = ""
	s.DescendStarting(v3, iter)
	expect = "two:one:"
	if got != expect {
		t.Errorf("Traversed in wrong direction expected %s (got %s)", expect, got)
	}

	got = ""
	s.AscendStarting(v3, iter)
	expect = "four:eight:"
	if got != expect {
		t.Errorf("Traversed in wrong direction expected %s (got %s)", expect, got)
	}

	got = ""
	stop = v2
	s.Ascend(iter)
	expect = "one:two:"
	if got != expect {
		t.Errorf("Traversal didn't stop expected %s (got %s)", expect, got)
	}

	got = ""
	stop = v4
	s.Descend(iter)
	expect = "eight:four:"
	if got != expect {
		t.Errorf("Traversal didn't stop expected %s (got %s)", expect, got)
	}

	got = ""
	stop = v4
	s.AscendStarting(v3, iter)
	expect = "four:"
	if got != expect {
		t.Errorf("Traversal didn't stop expected %s (got %s)", expect, got)
	}

	got = ""
	stop = v2
	s.DescendStarting(v3, iter)
	expect = "two:"
	if got != expect {
		t.Errorf("Traversal didn't stop expected %s (got %s)", expect, got)
	}

	expired = 4
	s.Expire()

	got = ""
	stop = nil
	s.Ascend(iter)
	expect = "one:two:eight:"
	if got != expect {
		t.Errorf("Expired item not removed expected %s (got %s)", expect, got)
	}

	vals := s.In("b").Lookup("four")
	if vals != nil {
		t.Errorf("Expired item found by field (got %#v)", vals)
	}

	s.Delete(v2)

	got = ""
	s.Ascend(iter)
	expect = "one:eight:"
	if got != expect {
		t.Errorf("Deleted item not removed expected %s (got %s)", expect, got)
	}
}

func TestNotificates(t *testing.T) {
	s := NewStore()
	s.CreateField("b")
	v1 := &X{a: 1, b: "one:"}
	v2 := &X{a: 1, b: "two:"}

	var expectEvent Event
	var expectOld Indexer
	var expectNew Indexer
	h := func(event Event, old, new Indexer) {
		if event != expectEvent {
			t.Errorf("Expected event %#v (got %#v)", expectEvent, event)
		}
		if old != expectOld {
			t.Errorf("Expected %#v old value %#v (got %#v)", event, expectOld, old)
		}
		if new != expectNew {
			t.Errorf("Expected %#v new value %#v (got %#v)", event, expectNew, new)
		}
	}

	s.On(Insert, h)
	s.On(Update, h)
	s.On(Remove, h)
	s.On(Expiry, h)

	expectEvent = Insert
	expectOld = nil
	expectNew = v1
	s.Put(v1)

	expectEvent = Update
	expectOld = v1
	expectNew = v2
	s.Put(v2)

	expectEvent = Remove
	expectOld = v2
	expectNew = nil
	s.Delete(v1) // This is a trick as we asked to delete v1, but v2 is actually getting deleted and should be expected

	expectEvent = Insert
	expectOld = nil
	expectNew = v1
	s.Put(v1)

	expired = 1
	expectEvent = Expiry
	expectOld = v1
	expectNew = nil
	s.Expire()

	expired = 0
}

func TestCompound(t *testing.T) {
	s := NewStore()
	s.CreateField("b", "c")
	v1a := &X{a: 1, b: "one", c: "xxx"}
	v1b := &X{a: 2, b: "one", c: "zzz"}
	v2a := &X{a: 3, b: "two", c: "xxx"}
	v2b := &X{a: 4, b: "two", c: "zzz"}

	s.Put(v1a)
	s.Put(v1b)
	s.Put(v2a)
	s.Put(v2b)

	out := s.In("b", "c").Lookup("one", "zzz")
	if n := len(out); n != 1 {
		t.Errorf("Expected exactly one response from compound lookup (got %s)", n)
	}
	if out[0].(*X).a != 2 {
		t.Errorf("Expected a = 2 in compound result (got %#v)", out[0])
	}
}

func TestUnsure(t *testing.T) {
	if Unsure("A", "Z") != true {
		t.Errorf("Expected A to be < Z")
	}
}

func TestLess(t *testing.T) {
	v1 := &wrap{&X{a: 1, b: "one:"}, nil}
	vx := btree.Int(5)

	if v1.Less(vx) {
		t.Errorf("Comparison with non-Indexer item should be false")
	}
}
