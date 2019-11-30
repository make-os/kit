package repo

import (
	"github.com/makeos/mosdef/params"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"
)

var _ = Describe("PushPool", func() {
	var pool *PushPool
	var tx *PushTx
	var tx2 *PushTx

	BeforeEach(func() {
		pool = NewPushPool(10)
	})

	BeforeEach(func() {
		tx = &PushTx{
			RepoName: "repo", NodeSig: []byte("sig"), PusherKeyID: "pk_id",
			References: []*PushedReference{
				{Name: "refs/heads/master", Nonce: 1, Sig: "abc_xyz", Fee: "0.2", AccountNonce: 2},
			},
		}
		tx2 = &PushTx{
			RepoName: "repo2", NodeSig: []byte("sig_2"), PusherKeyID: "pk_id_2",
			References: []*PushedReference{
				{Name: "refs/heads/master", Nonce: 1, Sig: "abc_xyz", Fee: "0.2", AccountNonce: 2},
			},
		}
	})

	Describe(".removeOld", func() {
		var err error

		When("pool has one item and pool TTL is 10ms", func() {
			BeforeEach(func() {
				pool = NewPushPool(1)
				err = pool.Add(tx)
				Expect(err).To(BeNil())
				params.PushPoolItemTTL = 10 * time.Millisecond
				time.Sleep(params.PushPoolItemTTL)
			})

			It("should return err=errFullPushPool", func() {
				Expect(pool.container).ToNot(BeEmpty())
				pool.removeOld()
				Expect(pool.container).To(BeEmpty())
				Expect(pool.index).To(BeEmpty())
				Expect(pool.refIndex).To(BeEmpty())
				Expect(pool.refNonceIdx).To(BeEmpty())
			})
		})
	})

	Describe(".Add", func() {
		var err error

		When("pool has reached capacity", func() {
			BeforeEach(func() {
				pool = NewPushPool(1)
				err = pool.Add(tx)
				Expect(err).To(BeNil())
				err = pool.Add(tx2)
				Expect(err).ToNot(BeNil())
			})

			It("should return err=errFullPushPool", func() {
				Expect(err).To(Equal(errFullPushPool))
			})
		})

		When("tx already exist in pool", func() {
			BeforeEach(func() {
				pool = NewPushPool(2)
				err = pool.Add(tx)
				Expect(err).To(BeNil())
				err = pool.Add(tx)
				Expect(err).ToNot(BeNil())
			})

			It("should return err=errTxExistInPushPool", func() {
				Expect(err).To(Equal(errTxExistInPushPool))
			})
		})

		When("tx doesn't already exist", func() {
			BeforeEach(func() {
				pool = NewPushPool(2)
				err = pool.Add(tx)
			})

			It("should return nil", func() {
				Expect(err).To(BeNil())
			})

			Specify("that the container has 1 item", func() {
				Expect(pool.container).To(HaveLen(1))
			})

			Specify("that the container index has 1 item which is the tx", func() {
				Expect(pool.index).To(HaveLen(1))
				Expect(pool.index).To(HaveKey(tx.ID().HexStr()))
			})
		})

		When("a reference (ref0) in new tx (tx_X) match an identical reference (ref0) of tx (tx_Y) "+
			"that already exist in the pool and tx_X and tx_Y have equal fee", func() {
			BeforeEach(func() {
				err = pool.Add(tx)
				Expect(err).To(BeNil())

				tx2 := &PushTx{
					RepoName: "repo", NodeSig: []byte("sig"), PusherKeyID: "pk_id",
					Timestamp: 100000000,
					References: []*PushedReference{
						{Name: "refs/heads/master", Nonce: 1, Sig: "abc_xyz", Fee: "0.2", AccountNonce: 2},
					},
				}

				err = pool.Add(tx2)
				Expect(err).ToNot(BeNil())
			})

			It("should reject new tx and return replace-by-fee inferiority error", func() {
				Expect(err.Error()).To(Equal("replace-by-fee on staged reference" +
					" (ref:refs/heads/master, repo:repo) not allowed due to inferior fee."))
			})
		})

		When("a reference (ref0) in new tx (tx_X) match an identical reference (ref0) of tx (tx_Y) "+
			"that already exist in the pool but ref0 has a higher nonce", func() {
			var tx2 *PushTx
			BeforeEach(func() {
				err = pool.Add(tx)
				Expect(err).To(BeNil())

				tx2 = &PushTx{
					RepoName: "repo", NodeSig: []byte("sig"), PusherKeyID: "pk_id",
					Timestamp: 100000000,
					References: []*PushedReference{
						{Name: "refs/heads/master", Nonce: 2, Sig: "abc_xyz", Fee: "0.01", AccountNonce: 2},
					},
				}
				pool.refIndex = containerIndex(map[string]*containerItem{})
				err = pool.Add(tx2)
				Expect(err).ToNot(BeNil())
			})

			It("should return err", func() {
				Expect(err.Error()).To(Equal("rejected because an identical reference with a lesser nonce has been staged"))
			})
		})

		When("a reference (ref0) in new tx (tx_X) match an identical reference (ref0) of tx (tx_Y) "+
			"that already exist in the pool but failed to read the ref index of the existing reference", func() {
			var tx2 *PushTx
			BeforeEach(func() {
				err = pool.Add(tx)
				Expect(err).To(BeNil())

				tx2 = &PushTx{
					RepoName: "repo", NodeSig: []byte("sig"), PusherKeyID: "pk_id",
					Timestamp: 100000000,
					References: []*PushedReference{
						{Name: "refs/heads/master", Nonce: 1, Sig: "abc_xyz", Fee: "0.01", AccountNonce: 2},
					},
				}
				pool.refIndex = containerIndex(map[string]*containerItem{})
			})

			It("should panic", func() {
				Expect(func() {
					pool.Add(tx2)
				}).To(Panic())
			})
		})

		When("a reference (ref0) in new tx (tx_X) match an identical reference (ref0) of tx (tx_Y) "+
			"that already exist in the pool and tx_X has a lower fee", func() {
			BeforeEach(func() {
				err = pool.Add(tx)
				Expect(err).To(BeNil())

				tx2 := &PushTx{
					RepoName: "repo", NodeSig: []byte("sig"), PusherKeyID: "pk_id",
					Timestamp: 100000000,
					References: []*PushedReference{
						{Name: "refs/heads/master", Nonce: 1, Sig: "abc_xyz", Fee: "0.01", AccountNonce: 2},
					},
				}

				err = pool.Add(tx2)
				Expect(err).ToNot(BeNil())
			})

			It("should reject new tx and return replace-by-fee inferiority error", func() {
				Expect(err.Error()).To(Equal("replace-by-fee on staged reference " +
					"(ref:refs/heads/master, repo:repo) not allowed due to inferior fee."))
			})
		})

		When("a reference (ref0) in new tx (tx_X) match an identical reference (ref0) of tx (tx_Y) "+
			"that already exist in the pool and tx_X has a higher fee", func() {
			var tx2 *PushTx
			BeforeEach(func() {
				err = pool.Add(tx)
				Expect(err).To(BeNil())

				tx2 = &PushTx{
					RepoName: "repo", NodeSig: []byte("sig"), PusherKeyID: "pk_id",
					Timestamp: 100000000,
					References: []*PushedReference{
						{Name: "refs/heads/master", Nonce: 1, Sig: "abc_xyz", Fee: "0.5", AccountNonce: 2}},
				}

				err = pool.Add(tx2)
			})

			It("should return nil", func() {
				Expect(err).To(BeNil())
			})

			Specify("that tx (tx_Y) was removed", func() {
				Expect(pool.index).ShouldNot(HaveKey(tx.ID().HexStr()))
			})

			Specify("that tx (tx_X) was added", func() {
				Expect(pool.index).Should(HaveKey(tx2.ID().HexStr()))
			})
		})

		When("a reference (ref0) in new tx (tx_X) match identical references (ref0) of tx (tx_Y) and (ref0) of tx (tx_Z) "+
			"that already exist in the pool and tx_X has a higher fee", func() {
			var txX, txY, txZ *PushTx
			BeforeEach(func() {
				txY = &PushTx{
					RepoName: "repo", NodeSig: []byte("sig"), PusherKeyID: "pk_id",
					Timestamp: 100000000,
					References: []*PushedReference{
						{Name: "refs/heads/master", Nonce: 1, Sig: "abc_xyz", Fee: "0.01", AccountNonce: 2},
					},
				}

				txZ = &PushTx{
					RepoName: "repo", NodeSig: []byte("sig"), PusherKeyID: "pk_id",
					Timestamp: 100000000,
					References: []*PushedReference{
						{Name: "refs/heads/update", Nonce: 1, Sig: "abc_xyz", Fee: "0.01", AccountNonce: 2},
					},
				}

				txX = &PushTx{
					RepoName: "repo", NodeSig: []byte("sig"), PusherKeyID: "pk_id",
					Timestamp: 100000000,
					References: []*PushedReference{
						{Name: "refs/heads/master", Nonce: 1, Sig: "abc_xyz", Fee: "0.02", AccountNonce: 2},
						{Name: "refs/heads/update", Nonce: 1, Sig: "abc_xyz", Fee: "0.01", AccountNonce: 2},
					},
				}

				err = pool.Add(txY)
				Expect(err).To(BeNil())
				err = pool.Add(txZ)
				Expect(err).To(BeNil())
				err = pool.Add(txX)
			})

			It("should return nil", func() {
				Expect(err).To(BeNil())
			})

			Specify("that tx (tx_Y and tx_Z) were removed", func() {
				Expect(pool.index).ShouldNot(HaveKey(txY.ID().HexStr()))
				Expect(pool.index).ShouldNot(HaveKey(txZ.ID().HexStr()))
			})

			Specify("that tx (tx_X) was added", func() {
				Expect(pool.index).Should(HaveKey(txX.ID().HexStr()))
			})
		})

		When("a references (ref0, ref1) in new tx (tx_X) match identical references (ref0) of tx (tx_Y) and (ref0) of tx (tx_Z) "+
			"that already exist in the pool and tx_X has lower total fee", func() {
			var txX, txY, txZ *PushTx
			BeforeEach(func() {
				txY = &PushTx{
					RepoName: "repo", NodeSig: []byte("sig"), PusherKeyID: "pk_id",
					Timestamp: 100000000,
					References: []*PushedReference{
						{Name: "refs/heads/master", Nonce: 1, Sig: "abc_xyz", Fee: "0.4", AccountNonce: 2},
					},
				}

				txZ = &PushTx{
					RepoName: "repo", NodeSig: []byte("sig"), PusherKeyID: "pk_id",
					Timestamp: 100000000,
					References: []*PushedReference{
						{Name: "refs/heads/update", Nonce: 1, Sig: "abc_xyz", Fee: "0.4", AccountNonce: 2},
					},
				}

				txX = &PushTx{
					RepoName: "repo", NodeSig: []byte("sig"), PusherKeyID: "pk_id",
					Timestamp: 100000000,
					References: []*PushedReference{
						{Name: "refs/heads/master", Nonce: 1, Sig: "abc_xyz", Fee: "0.4", AccountNonce: 2},
						{Name: "refs/heads/update", Nonce: 1, Sig: "abc_xyz", Fee: "0.3", AccountNonce: 2},
					},
				}

				err = pool.Add(txY)
				Expect(err).To(BeNil())
				err = pool.Add(txZ)
				Expect(err).To(BeNil())
				err = pool.Add(txX)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("replace-by-fee on multiple transactions not allowed due to inferior fee."))
			})
		})

	})

	Describe(".remove", func() {
		var err error

		When("pool has a tx", func() {
			BeforeEach(func() {
				pool = NewPushPool(2)
				err = pool.Add(tx)
				Expect(err).To(BeNil())
				Expect(pool.container).To(HaveLen(1))
				Expect(pool.index).To(HaveLen(1))
				Expect(pool.refIndex).To(HaveLen(1))
				Expect(pool.refNonceIdx).To(HaveLen(1))
				pool.remove(tx)
			})

			Specify("that the container and all indexes are empty", func() {
				Expect(pool.container).To(HaveLen(0))
				Expect(pool.index).To(HaveLen(0))
				Expect(pool.refIndex).To(HaveLen(0))
				Expect(pool.refNonceIdx).To(HaveLen(0))
			})
		})

		When("pool has two txs", func() {
			BeforeEach(func() {
				pool = NewPushPool(2)
				err = pool.Add(tx)
				err = pool.Add(tx2)
				Expect(err).To(BeNil())
				Expect(pool.container).To(HaveLen(2))
				Expect(pool.index).To(HaveLen(2))
				Expect(pool.refIndex).To(HaveLen(2))
				Expect(pool.refNonceIdx).To(HaveLen(2))
				pool.remove(tx)
			})

			Specify("that the container and all indexes have 1 item in them", func() {
				Expect(pool.container).To(HaveLen(1))
				Expect(pool.index).To(HaveLen(1))
				Expect(pool.refIndex).To(HaveLen(1))
				Expect(pool.refNonceIdx).To(HaveLen(1))
			})
		})
	})
})

var _ = Describe("refNonceIndex", func() {
	Describe(".add", func() {
		var idx refNonceIndex
		BeforeEach(func() {
			idx = refNonceIndex(make(map[string]uint64))
		})

		It("should add successfully", func() {
			idx.add("refs/heads/master", 1)
			Expect(idx).To(HaveKey("refs/heads/master"))
		})
	})

	Describe(".remove", func() {
		var idx refNonceIndex

		When("reference has a nonce indexed", func() {
			BeforeEach(func() {
				idx = refNonceIndex(make(map[string]uint64))
				idx.add("refs/heads/master", 10)
				idx.remove("refs/heads/master")
			})

			It("should return nonce=10", func() {
				Expect(idx).To(BeEmpty())
			})
		})
	})

	Describe(".getNonce", func() {
		var idx refNonceIndex
		var nonce uint64

		When("reference has a nonce indexed", func() {
			BeforeEach(func() {
				idx = refNonceIndex(make(map[string]uint64))
				idx.add("refs/heads/master", 10)
				nonce = idx.getNonce("refs/heads/master")
			})

			It("should return nonce=10", func() {
				Expect(nonce).To(Equal(uint64(10)))
			})
		})

		When("reference has no nonce indexed", func() {
			BeforeEach(func() {
				idx = refNonceIndex(make(map[string]uint64))
				nonce = idx.getNonce("refs/heads/master")
			})

			It("should return nonce=0", func() {
				Expect(nonce).To(Equal(uint64(0)))
			})
		})
	})
})

var _ = Describe("containerIndex", func() {
	Describe(".add", func() {
		var idx containerIndex
		BeforeEach(func() {
			idx = containerIndex(map[string]*containerItem{})
		})

		It("should add successfully", func() {
			idx.add("0x123", &containerItem{})
			Expect(idx).To(HaveKey("0x123"))
		})
	})

	Describe(".has", func() {
		When("hash does not exist", func() {
			var idx containerIndex
			BeforeEach(func() {
				idx = containerIndex(map[string]*containerItem{})
			})

			It("should return false", func() {
				Expect(idx.has("0x123")).To(BeFalse())
			})
		})
	})

	Describe(".has", func() {
		When("hash exist", func() {
			var idx containerIndex
			BeforeEach(func() {
				idx = containerIndex(map[string]*containerItem{})
				idx.add("0x123", &containerItem{})
			})

			It("should return false", func() {
				Expect(idx.has("0x123")).To(BeTrue())
			})
		})
	})

	Describe(".remove", func() {
		var idx containerIndex
		BeforeEach(func() {
			idx = containerIndex(map[string]*containerItem{})
			idx.add("0x123", &containerItem{})
			Expect(idx.has("0x123")).To(BeTrue())
			idx.remove("0x123")
		})

		Specify("that index does not contain removed hash", func() {
			Expect(idx.has("0x123")).To(BeFalse())
		})
	})

	Describe(".get", func() {
		var idx containerIndex

		When("hash exist", func() {
			var item = &containerItem{FeeRate: "123"}
			BeforeEach(func() {
				idx = containerIndex(map[string]*containerItem{})
				idx.add("0x123", item)
			})

			Specify("that the expected item is returned", func() {
				Expect(idx.get("0x123")).To(Equal(item))
			})
		})

		When("hash does not exist", func() {
			BeforeEach(func() {
				idx = containerIndex(map[string]*containerItem{})
			})

			It("should return nil", func() {
				Expect(idx.get("0x123")).To(BeNil())
			})
		})
	})
})
