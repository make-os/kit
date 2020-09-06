package pool

import (
	"time"

	"github.com/golang/mock/gomock"
	"github.com/make-os/lobe/crypto"
	"github.com/make-os/lobe/mocks"
	"github.com/make-os/lobe/params"
	"github.com/make-os/lobe/remote/push/types"
	crypto2 "github.com/make-os/lobe/util/crypto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("PushPool", func() {
	var pool *PushPool
	var note *types.Note
	var note2 *types.Note
	var ctrl *gomock.Controller
	var mockLogic *mocks.MockLogic
	var pushKeyID = crypto.NewKeyFromIntSeed(1).PushAddr().String()
	var pushKeyID2 = crypto.NewKeyFromIntSeed(2).PushAddr().String()

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockLogic = mocks.NewMockLogic(ctrl)
		pool = NewPushPool(10, mockLogic)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	BeforeEach(func() {
		note = &types.Note{
			RepoName:        "repo",
			RemoteNodeSig:   []byte("sig"),
			PushKeyID:       crypto2.MustDecodePushKeyID(pushKeyID),
			PusherAcctNonce: 2,
			References: []*types.PushedReference{
				{Name: "refs/heads/master", Fee: "0.2", Nonce: 1},
			},
		}
		note2 = &types.Note{
			RepoName:        "repo2",
			RemoteNodeSig:   []byte("sig_2"),
			PushKeyID:       crypto2.MustDecodePushKeyID(pushKeyID2),
			PusherAcctNonce: 2,
			References: []*types.PushedReference{
				{Name: "refs/heads/master", Fee: "0.2", Nonce: 1},
			},
		}
	})

	Describe(".removeOld", func() {
		var err error

		When("pool has one item and pool TTL is 10ms", func() {
			BeforeEach(func() {
				pool = NewPushPool(1, mockLogic)
				err = pool.Add(note)
				Expect(err).To(BeNil())
				params.PushPoolItemTTL = 10 * time.Millisecond
				time.Sleep(params.PushPoolItemTTL)
			})

			It("should return err=errFullPushPool", func() {
				Expect(pool.container).ToNot(BeEmpty())
				pool.removeOld()
				Expect(pool.container).To(BeEmpty())
				Expect(pool.noteIdx).To(BeEmpty())
				Expect(pool.refIdx).To(BeEmpty())
				Expect(pool.refNonceIdx).To(BeEmpty())
			})
		})
	})

	Describe(".Add", func() {
		var err error
		var baseNote *types.Note

		BeforeEach(func() {
			baseNote = &types.Note{
				RepoName:      "repo",
				RemoteNodeSig: []byte("sig"),
				PushKeyID:     crypto2.MustDecodePushKeyID(pushKeyID),
				Timestamp:     100000000,
			}
		})

		When("pool has reached capacity", func() {
			BeforeEach(func() {
				pool = NewPushPool(1, mockLogic)
				err = pool.Add(note)
				Expect(err).To(BeNil())
				err = pool.Add(note2)
				Expect(err).ToNot(BeNil())
			})

			It("should return err=errFullPushPool", func() {
				Expect(err).To(Equal(errFullPushPool))
			})
		})

		When("tx already exist in pool", func() {
			It("should return nil and it should not add duplicate to the pool", func() {
				pool = NewPushPool(2, mockLogic)
				err = pool.Add(note)
				Expect(err).To(BeNil())
				err = pool.Add(note)
				Expect(err).To(BeNil())
				Expect(pool.Len()).To(Equal(1))
			})
		})

		When("tx doesn't already exist", func() {
			BeforeEach(func() {
				pool = NewPushPool(2, mockLogic)
				err = pool.Add(note)
			})

			It("should return nil", func() {
				Expect(err).To(BeNil())
			})

			Specify("that the container has 1 item", func() {
				Expect(pool.container).To(HaveLen(1))
			})

			Specify("that the container index has 1 item which is the tx", func() {
				Expect(pool.noteIdx).To(HaveLen(1))
				Expect(pool.noteIdx).To(HaveKey(note.ID().HexStr()))
			})
		})

		When("a reference (ref0) in new tx (txA) match an identical reference (ref0) of tx (tx_Y) "+
			"that already exist in the pool and txA and tx_Y have equal total fee", func() {
			BeforeEach(func() {
				err = pool.Add(note)
				Expect(err).To(BeNil())

				baseNote.References = append(baseNote.References, &types.PushedReference{Name: "refs/heads/master", Fee: "0.2", Nonce: 1})
				txA := baseNote

				err = pool.Add(txA)
				Expect(err).ToNot(BeNil())
			})

			It("should reject new tx and return replace-by-fee inferiority error", func() {
				Expect(err.Error()).To(Equal("replace-by-fee on staged reference" +
					" (ref:refs/heads/master, repo:repo) not allowed due to inferior fee."))
			})
		})

		When("a reference (ref0) in new tx (txA) match an identical reference (ref0) of tx (tx_Y) "+
			"that already exist in the pool but ref0 has a higher nonce", func() {
			BeforeEach(func() {
				err = pool.Add(note)
				Expect(err).To(BeNil())

				baseNote.References = append(baseNote.References, &types.PushedReference{Name: "refs/heads/master", Fee: "0.01", Nonce: 2})
				txA := baseNote

				pool.refIdx = map[string]*containerItem{}
				err = pool.Add(txA)
				Expect(err).ToNot(BeNil())
			})

			It("should return err", func() {
				Expect(err.Error()).To(Equal("rejected: pool has existing reference with a lower nonce"))
			})
		})

		When("a reference (ref0) in new tx (txA) match an identical reference (ref0) of tx (tx_Y) "+
			"that already exist in the pool but failed to read the ref index of the existing reference", func() {
			var txA *types.Note
			BeforeEach(func() {
				err = pool.Add(note)
				Expect(err).To(BeNil())

				baseNote.References = append(baseNote.References, &types.PushedReference{Name: "refs/heads/master", Fee: "0.01", Nonce: 1})
				txA = baseNote
				pool.refIdx = map[string]*containerItem{}
			})

			It("should panic", func() {
				Expect(func() {
					pool.Add(txA)
				}).To(Panic())
			})
		})

		When("a reference (ref0) in new tx (txA) match an identical reference (ref0) of tx (tx_Y) "+
			"that already exist in the pool and txA has a lower fee", func() {
			BeforeEach(func() {
				err = pool.Add(note)
				Expect(err).To(BeNil())

				baseNote.References = append(baseNote.References, &types.PushedReference{Name: "refs/heads/master", Fee: "0.01", Nonce: 1})
				txA := baseNote

				err = pool.Add(txA)
				Expect(err).ToNot(BeNil())
			})

			It("should reject new tx and return replace-by-fee inferiority error", func() {
				Expect(err.Error()).To(Equal("replace-by-fee on staged reference " +
					"(ref:refs/heads/master, repo:repo) not allowed due to inferior fee."))
			})
		})

		When("a reference (ref0) in new tx (txA) match an identical reference (ref0) of tx (tx_Y) "+
			"that already exist in the pool and txA has a higher fee", func() {
			var txA *types.Note
			BeforeEach(func() {
				err = pool.Add(note)
				Expect(err).To(BeNil())

				baseNote.References = append(baseNote.References, &types.PushedReference{Name: "refs/heads/master", Fee: "0.5", Nonce: 1})
				txA = baseNote

				err = pool.Add(txA)
			})

			It("should return nil", func() {
				Expect(err).To(BeNil())
			})

			Specify("that tx (tx_Y) was removed", func() {
				Expect(pool.noteIdx).ShouldNot(HaveKey(note.ID().HexStr()))
			})

			Specify("that tx (txA) was added", func() {
				Expect(pool.noteIdx).Should(HaveKey(txA.ID().HexStr()))
			})
		})

		When("a reference (ref0) in new tx (txA) match identical references (ref0) of tx (tx_Y) and (ref0) of tx (tx_Z) "+
			"that already exist in the pool and txA has a higher total fee", func() {
			var txX, txY, txZ *types.Note
			BeforeEach(func() {
				txY = &types.Note{
					RepoName:        "repo",
					RemoteNodeSig:   []byte("sig"),
					PushKeyID:       crypto2.MustDecodePushKeyID(pushKeyID),
					Timestamp:       100000000,
					PusherAcctNonce: 2,
					References: []*types.PushedReference{
						{Name: "refs/heads/master", Nonce: 1, Fee: "0.01"},
					},
				}

				txZ = &types.Note{
					RepoName:        "repo",
					RemoteNodeSig:   []byte("sig"),
					PushKeyID:       crypto2.MustDecodePushKeyID(pushKeyID),
					Timestamp:       100000000,
					PusherAcctNonce: 2,
					References: []*types.PushedReference{
						{Name: "refs/heads/update", Nonce: 1, Fee: "0.01"},
					},
				}

				txX = &types.Note{
					RepoName:        "repo",
					RemoteNodeSig:   []byte("sig"),
					PushKeyID:       crypto2.MustDecodePushKeyID(pushKeyID),
					Timestamp:       100000000,
					PusherAcctNonce: 2,
					References: []*types.PushedReference{
						{Name: "refs/heads/master", Nonce: 1, Fee: "0.02"},
						{Name: "refs/heads/update", Nonce: 1, Fee: "0.01"},
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
				Expect(pool.noteIdx).ShouldNot(HaveKey(txY.ID().HexStr()))
				Expect(pool.noteIdx).ShouldNot(HaveKey(txZ.ID().HexStr()))
			})

			Specify("that tx (txA) was added", func() {
				Expect(pool.noteIdx).Should(HaveKey(txX.ID().HexStr()))
			})
		})

		When("references (ref0, ref1) in new tx (txA) match identical references (ref0) of tx (tx_Y) and (ref0) of tx (tx_Z) "+
			"that already exist in the pool and txA has lower total fee", func() {
			var txX, txY, txZ *types.Note
			BeforeEach(func() {
				txY = &types.Note{
					RepoName:        "repo",
					RemoteNodeSig:   []byte("sig"),
					PushKeyID:       crypto2.MustDecodePushKeyID(pushKeyID),
					Timestamp:       100000000,
					PusherAcctNonce: 2,
					References: []*types.PushedReference{
						{Name: "refs/heads/master", Nonce: 1, Fee: "0.4"},
					},
				}

				txZ = &types.Note{
					RepoName:        "repo",
					RemoteNodeSig:   []byte("sig"),
					PushKeyID:       crypto2.MustDecodePushKeyID(pushKeyID),
					Timestamp:       100000000,
					PusherAcctNonce: 2,
					References: []*types.PushedReference{
						{Name: "refs/heads/update", Nonce: 1, Fee: "0.4"},
					},
				}

				txX = &types.Note{
					RepoName:        "repo",
					RemoteNodeSig:   []byte("sig"),
					PushKeyID:       crypto2.MustDecodePushKeyID(pushKeyID),
					Timestamp:       100000000,
					PusherAcctNonce: 2,
					References: []*types.PushedReference{
						{Name: "refs/heads/master", Nonce: 1, Fee: "0.6"},
						{Name: "refs/heads/update", Nonce: 1, Fee: "0.1"},
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
				Expect(err.Error()).To(Equal("replace-by-fee on multiple push notes not allowed due to inferior fee"))
			})
		})
	})

	Describe(".Get", func() {
		var err error

		When("note exist", func() {
			BeforeEach(func() {
				pool = NewPushPool(2, mockLogic)
				err = pool.Add(note)
				Expect(err).To(BeNil())
			})

			It("should return the push note", func() {
				note := pool.Get(note.ID().String())
				Expect(note).To(Equal(note))
			})
		})

		When("note does not exist", func() {
			BeforeEach(func() {
				pool = NewPushPool(2, mockLogic)
			})

			It("should return the push note", func() {
				note := pool.Get(note.ID().String())
				Expect(note).To(BeNil())
			})
		})
	})

	Describe(".remove", func() {
		var err error

		When("pool has a tx", func() {
			BeforeEach(func() {
				pool = NewPushPool(2, mockLogic)
				err = pool.Add(note)
				Expect(err).To(BeNil())
				Expect(pool.container).To(HaveLen(1))
				Expect(pool.noteIdx).To(HaveLen(1))
				Expect(pool.refIdx).To(HaveLen(1))
				Expect(pool.refNonceIdx).To(HaveLen(1))
				pool.remove(note)
			})

			Specify("that the container and all indexes are empty", func() {
				Expect(pool.container).To(HaveLen(0))
				Expect(pool.noteIdx).To(HaveLen(0))
				Expect(pool.refIdx).To(HaveLen(0))
				Expect(pool.refNonceIdx).To(HaveLen(0))
			})
		})

		When("pool has two txs", func() {
			BeforeEach(func() {
				pool = NewPushPool(2, mockLogic)
				err = pool.Add(note)
				err = pool.Add(note2)
				Expect(err).To(BeNil())
				Expect(pool.container).To(HaveLen(2))
				Expect(pool.noteIdx).To(HaveLen(2))
				Expect(pool.refIdx).To(HaveLen(2))
				Expect(pool.refNonceIdx).To(HaveLen(2))
				pool.remove(note)
			})

			Specify("that the container and all indexes have 1 item in them", func() {
				Expect(pool.container).To(HaveLen(1))
				Expect(pool.noteIdx).To(HaveLen(1))
				Expect(pool.refIdx).To(HaveLen(1))
				Expect(pool.refNonceIdx).To(HaveLen(1))
			})
		})
	})

	Describe(".Len", func() {
		var err error

		When("pool has a tx", func() {
			BeforeEach(func() {
				pool = NewPushPool(2, mockLogic)
				err = pool.Add(note)
				Expect(err).To(BeNil())
			})

			It("should return 1", func() {
				Expect(pool.Len()).To(Equal(1))
			})
		})

		When("pool has no tx", func() {
			BeforeEach(func() {
				pool = NewPushPool(2, mockLogic)
			})

			It("should return 0", func() {
				Expect(pool.Len()).To(Equal(0))
			})
		})
	})

	Describe(".HasSeen", func() {
		It("should return true if tx was added to the pool", func() {
			pool = NewPushPool(2, mockLogic)
			err := pool.Add(note)
			Expect(err).To(BeNil())
			pool.Remove(note)
			Expect(pool.Len()).To(Equal(0))
			Expect(pool.HasSeen(note.ID().String())).To(BeTrue())
		})

		It("should return false when tx was not added to the pool", func() {
			pool = NewPushPool(2, mockLogic)
			Expect(pool.HasSeen(note.ID().String())).To(BeFalse())
		})
	})
})

var _ = Describe("refNonceIndex", func() {
	Describe(".add", func() {
		var idx refNonceIndex
		BeforeEach(func() {
			idx = make(map[string]uint64)
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
				idx = make(map[string]uint64)
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
				idx = make(map[string]uint64)
				idx.add("refs/heads/master", 10)
				nonce = idx.getNonce("refs/heads/master")
			})

			It("should return nonce=10", func() {
				Expect(nonce).To(Equal(uint64(10)))
			})
		})

		When("reference has no nonce indexed", func() {
			BeforeEach(func() {
				idx = make(map[string]uint64)
				nonce = idx.getNonce("refs/heads/master")
			})

			It("should return nonce=0", func() {
				Expect(nonce).To(Equal(uint64(0)))
			})
		})
	})
})

var _ = Describe("repoNotesIndex", func() {
	var pushKeyID = crypto.BytesToPushKeyID([]byte("pk1wfx7vp8qfyv98cctvamqwec5xjrj48tpxaa77t"))

	Describe(".add", func() {
		var idx repoNotesIndex
		BeforeEach(func() {
			idx = map[string][]*containerItem{}
		})

		It("should add successfully", func() {
			idx.add("repo1", &containerItem{})
			Expect(idx).To(HaveKey("repo1"))
			Expect(idx["repo1"]).To(HaveLen(1))
		})
	})

	Describe(".has", func() {
		var idx repoNotesIndex
		When("repo does not exist in index", func() {
			var has bool

			BeforeEach(func() {
				idx = map[string][]*containerItem{}
				has = idx.has("repo1")
			})

			It("should return false", func() {
				Expect(has).To(BeFalse())
			})
		})

		When("repo exist in index", func() {
			var has bool

			BeforeEach(func() {
				idx = map[string][]*containerItem{
					"repo1": {},
				}
				has = idx.has("repo1")
			})

			It("should return true", func() {
				Expect(has).To(BeTrue())
			})
		})
	})

	Describe(".remove", func() {
		var idx repoNotesIndex

		When("repo has 1 txA and txA is removed", func() {
			var txA *types.Note
			BeforeEach(func() {
				txA = &types.Note{RepoName: "repo1", RemoteNodeSig: []byte("sig"), PushKeyID: crypto2.MustDecodePushKeyID(pushKeyID), Timestamp: 100000000}
				idx = map[string][]*containerItem{}
				idx.add("repo1", &containerItem{Note: txA})
				Expect(idx["repo1"]).To(HaveLen(1))
			})

			It("should remove repo completely", func() {
				idx.remove("repo1", txA.ID().String())
				Expect(idx).To(BeEmpty())
			})
		})

		When("repo has 2 txs (txA and TxB) and txA is removed", func() {
			var txA, txB *types.Note
			BeforeEach(func() {
				txA = &types.Note{RepoName: "repo1", RemoteNodeSig: []byte("sig"), PushKeyID: crypto2.MustDecodePushKeyID(pushKeyID), Timestamp: 100000000}
				txB = &types.Note{RepoName: "repo1", RemoteNodeSig: []byte("sig"), PushKeyID: crypto2.MustDecodePushKeyID(pushKeyID), Timestamp: 200000000}
				idx = map[string][]*containerItem{}
				idx.add("repo1", &containerItem{Note: txA})
				idx.add("repo1", &containerItem{Note: txB})
				Expect(idx["repo1"]).To(HaveLen(2))
			})

			It("should remove only txA", func() {
				idx.remove("repo1", txA.ID().String())
				Expect(idx).ToNot(BeEmpty())
				Expect(idx["repo1"]).To(HaveLen(1))
				actual := idx["repo1"][0]
				Expect(actual.Note.ID()).To(Equal(txB.ID()))
			})
		})
	})
})

var _ = Describe("containerIndex", func() {
	Describe(".add", func() {
		var idx containerIndex
		BeforeEach(func() {
			idx = map[string]*containerItem{}
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
				idx = map[string]*containerItem{}
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
				idx = map[string]*containerItem{}
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
			idx = map[string]*containerItem{}
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
				idx = map[string]*containerItem{}
				idx.add("0x123", item)
			})

			Specify("that the expected item is returned", func() {
				Expect(idx.get("0x123")).To(Equal(item))
			})
		})

		When("hash does not exist", func() {
			BeforeEach(func() {
				idx = map[string]*containerItem{}
			})

			It("should return nil", func() {
				Expect(idx.get("0x123")).To(BeNil())
			})
		})
	})
})
