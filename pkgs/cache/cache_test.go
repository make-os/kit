package cache

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Cache", func() {

	var cache *Cache
	var expEntryCache *Cache

	BeforeEach(func() {
		DefaultRemovalInterval = 5 * time.Second
		cache = NewCache(10)
		expEntryCache = NewCacheWithExpiringEntry(10)
	})

	Describe(".Add", func() {
		It("should successfully add an item", func() {
			Expect(cache.container.Len()).To(Equal(0))
			cache.Add("key", "val")
			Expect(cache.container.Len()).To(Equal(1))
		})

		Context("using expiring entry cache", func() {
			It("should successfully add an item with an expiry time", func() {
				expAt := time.Now().Add(20 * time.Second)
				Expect(expEntryCache.container.Len()).To(Equal(0))
				expEntryCache.Add("key", "val", expAt)
				Expect(expEntryCache.container.Len()).To(Equal(1))
				val, _ := expEntryCache.container.Get("key")
				Expect(val.(*cacheValue).expAt).To(Equal(expAt))
			})

			It("should remove previously expired entry", func() {
				expAt := time.Now().Add(1 * time.Millisecond)
				expEntryCache.Add("key", "val", expAt)
				Expect(expEntryCache.container.Len()).To(Equal(1))
				time.Sleep(2 * time.Millisecond)
				expEntryCache.Add("key2", "val")
				Expect(expEntryCache.container.Len()).To(Equal(1))
				val, _ := expEntryCache.container.Get("key2")
				Expect(val).ToNot(BeNil())
			})

			Context("periodic removal test", func() {
				It("should remove expired entry", func() {
					DefaultRemovalInterval = 1 * time.Millisecond
					expEntryCache = NewCacheWithExpiringEntry(10)
					expAt := time.Now().Add(1 * time.Millisecond)
					expEntryCache.Add("key", "val", expAt)
					Expect(expEntryCache.container.Len()).To(Equal(1))
					time.Sleep(2 * time.Millisecond)
					Expect(expEntryCache.container.Len()).To(Equal(0))
				})
			})
		})
	})

	Describe(".Peek", func() {
		It("should return value of item", func() {
			cache.Add("some_key", "some_value")
			val := cache.Peek("some_key")
			Expect(val).To(Equal("some_value"))
		})

		It("should return nil if item does not exist", func() {
			val := cache.Peek("some_key")
			Expect(val).To(BeNil())
		})
	})

	Describe(".Get", func() {
		It("should return value of item", func() {
			cache.Add("some_key", "some_value")
			val := cache.Get("some_key")
			Expect(val).To(Equal("some_value"))
		})

		It("should return nil if item does not exist", func() {
			val := cache.Get("some_key")
			Expect(val).To(BeNil())
		})
	})

	Describe(".Has", func() {
		It("should return true if item exists", func() {
			cache.Add("k1", "some_value")
			Expect(cache.Has("k1")).To(BeTrue())
		})

		It("should return false if item does not exists", func() {
			cache.Add("k1", "some_value")
			Expect(cache.Has("k2")).To(BeFalse())
		})
	})

	Describe(".Keys", func() {
		It("should return two keys (k1, k2)", func() {
			cache.Add("k1", "some_value")
			cache.Add("k2", "some_value2")
			Expect(cache.Keys()).To(HaveLen(2))
			Expect(cache.Keys()).To(Equal([]interface{}{"k1", "k2"}))
		})

		It("should return empty", func() {
			keys := cache.Keys()
			Expect(keys).To(HaveLen(0))
			Expect(keys).To(Equal([]interface{}{}))
		})
	})

	Describe(".Remove", func() {
		It("should successfully remove item", func() {
			cache.Add("k1", "some_value")
			cache.Add("k2", "some_value2")
			cache.Remove("k1")
			Expect(cache.Has("k1")).To(BeFalse())
			Expect(cache.Has("k2")).To(BeTrue())
		})
	})

	Describe(".Len", func() {
		It("should successfully return length = 2", func() {
			cache.Add("k1", "some_value")
			cache.Add("k2", "some_value2")
			Expect(cache.Len()).To(Equal(2))
		})
	})

})
