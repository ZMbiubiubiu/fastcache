package fastcache

import (
	"fmt"
	"testing"
	"time"
)

func TestCacheSmall(t *testing.T) {
	c := New(1)

	v := c.Get(nil, []byte("aaa"))
	if len(v) != 0 {
		t.Fatalf("unexpected non-empty value obtained from small cache: %q", v)
	}

	c.Set([]byte("key"), []byte("value"))
	v = c.Get(nil, []byte("key"))
	if string(v) != "value" {
		t.Fatalf("unexpected value obtained; got %q; want %q", v, "value")
	}

	v = c.Get(nil, nil)
	if len(v) != 0 {
		t.Fatalf("unexpected non-empty value obtained from small cache: %q", v)
	}
	v = c.Get(nil, []byte("aaa"))
	if len(v) != 0 {
		t.Fatalf("unexpected non-empty value obtained from small cache: %q", v)
	}

	c.Set([]byte("aaa"), []byte("bbb"))
	v = c.Get(nil, []byte("aaa"))
	if string(v) != "bbb" {
		t.Fatalf("unexpected value obtained; got %q; want %q", v, "bbb")
	}

	c.Reset()
	v = c.Get(nil, []byte("aaa"))
	if len(v) != 0 {
		t.Fatalf("unexpected non-empty value obtained from empty cache: %q", v)
	}
}

func TestCacheWrap(t *testing.T) {
	c := New(bucketsCount * chunkSize * 1.5)

	calls := uint64(5e6)

	for i := uint64(0); i < calls; i++ {
		k := []byte(fmt.Sprintf("key %d", i))
		v := []byte(fmt.Sprintf("value %d", i))
		c.Set(k, v)
		vv := c.Get(nil, k)
		if string(vv) != string(v) {
			t.Fatalf("unexpected value for key %q; got %q; want %q", k, vv, v)
		}
	}
	for i := uint64(0); i < calls/10; i++ {
		x := i * 10
		k := []byte(fmt.Sprintf("key %d", x))
		v := []byte(fmt.Sprintf("value %d", x))
		vv := c.Get(nil, k)
		if len(vv) > 0 && string(v) != string(vv) {
			t.Fatalf("unexpected value for key %q; got %q; want %q", k, vv, v)
		}
	}

	var s Stats
	c.UpdateStats(&s)
	getCalls := calls + calls/10
	if s.GetCalls != getCalls {
		t.Fatalf("unexpected number of getCalls; got %d; want %d", s.GetCalls, getCalls)
	}
	if s.SetCalls != calls {
		t.Fatalf("unexpected number of setCalls; got %d; want %d", s.SetCalls, calls)
	}
	if s.Misses == 0 || s.Misses >= calls/10 {
		t.Fatalf("unexpected number of misses; got %d; it should be between 0 and %d", s.Misses, calls/10)
	}
	if s.Collisions != 0 {
		t.Fatalf("unexpected number of collisions; got %d; want 0", s.Collisions)
	}
	if s.EntriesCount < calls/5 {
		t.Fatalf("unexpected number of items; got %d; cannot be smaller than %d", s.EntriesCount, calls/5)
	}
	if s.BytesSize < 1024 {
		t.Fatalf("unexpected number of bytesSize; got %d; cannot be smaller than %d", s.BytesSize, 1024)
	}
}

func TestCacheDel(t *testing.T) {
	c := New(1024)
	for i := 0; i < 100; i++ {
		k := []byte(fmt.Sprintf("key %d", i))
		v := []byte(fmt.Sprintf("value %d", i))
		c.Set(k, v)
		vv := c.Get(nil, k)
		if string(vv) != string(v) {
			t.Fatalf("unexpected value for key %q; got %q; want %q", k, vv, v)
		}
		c.Del(k)
		vv = c.Get(nil, k)
		if len(vv) > 0 {
			t.Fatalf("unexpected non-empty value got for key %q: %q", k, vv)
		}
	}
}

func TestCacheBigKeyValue(t *testing.T) {
	c := New(1024)

	// Both key and value exceed 64Kb
	k := make([]byte, 90*1024)
	v := make([]byte, 100*1024)
	c.Set(k, v)
	vv := c.Get(nil, k)
	if len(vv) > 0 {
		t.Fatalf("unexpected non-empty value got for key %q: %q", k, vv)
	}

	// len(key) + len(value) > 64Kb
	k = make([]byte, 40*1024)
	v = make([]byte, 40*1024)
	c.Set(k, v)
	vv = c.Get(nil, k)
	if len(vv) > 0 {
		t.Fatalf("unexpected non-empty value got for key %q: %q", k, vv)
	}
}

func TestCacheSetGetSerial(t *testing.T) {
	itemsCount := 10000
	c := New(30 * itemsCount)
	if err := testCacheGetSet(c, itemsCount); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
}

func TestCacheGetSetConcurrent(t *testing.T) {
	itemsCount := 10000
	const gorotines = 10
	c := New(30 * itemsCount * gorotines)

	ch := make(chan error, gorotines)
	for i := 0; i < gorotines; i++ {
		go func() {
			ch <- testCacheGetSet(c, itemsCount)
		}()
	}
	for i := 0; i < gorotines; i++ {
		select {
		case err := <-ch:
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("timeout")
		}
	}
}

func testCacheGetSet(c *Cache, itemsCount int) error {
	for i := 0; i < itemsCount; i++ {
		k := []byte(fmt.Sprintf("key %d", i))
		v := []byte(fmt.Sprintf("value %d", i))
		c.Set(k, v)
		vv := c.Get(nil, k)
		if string(vv) != string(v) {
			return fmt.Errorf("unexpected value for key %q after insertion; got %q; want %q", k, vv, v)
		}
	}
	misses := 0
	for i := 0; i < itemsCount; i++ {
		k := []byte(fmt.Sprintf("key %d", i))
		vExpected := fmt.Sprintf("value %d", i)
		v := c.Get(nil, k)
		if string(v) != string(vExpected) {
			if len(v) > 0 {
				return fmt.Errorf("unexpected value for key %q after all insertions; got %q; want %q", k, v, vExpected)
			}
			misses++
		}
	}
	if misses >= itemsCount/100 {
		return fmt.Errorf("too many cache misses; got %d; want less than %d", misses, itemsCount/100)
	}
	return nil
}