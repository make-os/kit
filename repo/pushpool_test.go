package repo

import (
	"fmt"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/makeos/mosdef/params"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/types/mocks"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func txCheckNoIssue(tx types.RepoPushNote, keepers types.Keepers, dht types.DHT) error {
	return nil
}

func txCheckErr(err error) func(tx types.RepoPushNote, keepers types.Keepers, dht types.DHT) error {
	return func(tx types.RepoPushNote, keepers types.Keepers, dht types.DHT) error {
		return err
	}
}

var _ = Describe("PushPool", func() {
	var pool *PushPool
	var tx *types.PushNote
	var tx2 *types.PushNote
	var ctrl *gomock.Controller
	var mockKeeper *mocks.MockKeepers
	var mockDHT *mocks.MockDHT

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockKeeper = mocks.NewMockKeepers(ctrl)
		mockDHT = mocks.NewMockDHT(ctrl)
		pool = NewPushPool(10, mockKeeper, mockDHT)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	BeforeEach(func() {
		tx = &types.PushNote{
			RepoName: "repo", NodeSig: []byte("sig"), PusherKeyID: "pk_id",
			References: []*types.PushedReference{
				{Name: "refs/heads/master", Nonce: 1, Fee: "0.2", AccountNonce: 2},
			},
		}
		tx2 = &types.PushNote{
			RepoName: "repo2", NodeSig: []byte("sig_2"), PusherKeyID: "pk_id_2",
			References: []*types.PushedReference{
				{Name: "refs/heads/master", Nonce: 1, Fee: "0.2", AccountNonce: 2},
			},
		}
	})

	Describe(".removeOld", func() {
		var err error

		When("pool has one item and pool TTL is 10ms", func() {
			BeforeEach(func() {
				pool = NewPushPool(1, mockKeeper, mockDHT)
				pool.noteChecker = txCheckNoIssue
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
				pool = NewPushPool(1, mockKeeper, mockDHT)
				pool.noteChecker = txCheckNoIssue
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
				pool = NewPushPool(2, mockKeeper, mockDHT)
				pool.noteChecker = txCheckNoIssue
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
				pool = NewPushPool(2, mockKeeper, mockDHT)
				pool.noteChecker = txCheckNoIssue
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
				pool.noteChecker = txCheckNoIssue
				err = pool.Add(tx)
				Expect(err).To(BeNil())

				tx2 := &types.PushNote{
					RepoName: "repo", NodeSig: []byte("sig"), PusherKeyID: "pk_id",
					Timestamp: 100000000,
					References: []*types.PushedReference{
						{Name: "refs/heads/master", Nonce: 1, Fee: "0.2", AccountNonce: 2},
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
			var tx2 *types.PushNote
			BeforeEach(func() {
				pool.noteChecker = txCheckNoIssue
				err = pool.Add(tx)
				Expect(err).To(BeNil())

				tx2 = &types.PushNote{
					RepoName: "repo", NodeSig: []byte("sig"), PusherKeyID: "pk_id",
					Timestamp: 100000000,
					References: []*types.PushedReference{
						{Name: "refs/heads/master", Nonce: 2, Fee: "0.01", AccountNonce: 2},
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
			var tx2 *types.PushNote
			BeforeEach(func() {
				pool.noteChecker = txCheckNoIssue
				err = pool.Add(tx)
				Expect(err).To(BeNil())

				tx2 = &types.PushNote{
					RepoName: "repo", NodeSig: []byte("sig"), PusherKeyID: "pk_id",
					Timestamp: 100000000,
					References: []*types.PushedReference{
						{Name: "refs/heads/master", Nonce: 1, Fee: "0.01", AccountNonce: 2},
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
				pool.noteChecker = txCheckNoIssue
				err = pool.Add(tx)
				Expect(err).To(BeNil())

				tx2 := &types.PushNote{
					RepoName: "repo", NodeSig: []byte("sig"), PusherKeyID: "pk_id",
					Timestamp: 100000000,
					References: []*types.PushedReference{
						{Name: "refs/heads/master", Nonce: 1, Fee: "0.01", AccountNonce: 2},
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
			var tx2 *types.PushNote
			BeforeEach(func() {
				pool.noteChecker = txCheckNoIssue
				err = pool.Add(tx)
				Expect(err).To(BeNil())

				tx2 = &types.PushNote{
					RepoName: "repo", NodeSig: []byte("sig"), PusherKeyID: "pk_id",
					Timestamp: 100000000,
					References: []*types.PushedReference{
						{Name: "refs/heads/master", Nonce: 1, Fee: "0.5", AccountNonce: 2}},
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
			var txX, txY, txZ *types.PushNote
			BeforeEach(func() {
				txY = &types.PushNote{
					RepoName: "repo", NodeSig: []byte("sig"), PusherKeyID: "pk_id",
					Timestamp: 100000000,
					References: []*types.PushedReference{
						{Name: "refs/heads/master", Nonce: 1, Fee: "0.01", AccountNonce: 2},
					},
				}

				txZ = &types.PushNote{
					RepoName: "repo", NodeSig: []byte("sig"), PusherKeyID: "pk_id",
					Timestamp: 100000000,
					References: []*types.PushedReference{
						{Name: "refs/heads/update", Nonce: 1, Fee: "0.01", AccountNonce: 2},
					},
				}

				txX = &types.PushNote{
					RepoName: "repo", NodeSig: []byte("sig"), PusherKeyID: "pk_id",
					Timestamp: 100000000,
					References: []*types.PushedReference{
						{Name: "refs/heads/master", Nonce: 1, Fee: "0.02", AccountNonce: 2},
						{Name: "refs/heads/update", Nonce: 1, Fee: "0.01", AccountNonce: 2},
					},
				}

				pool.noteChecker = txCheckNoIssue
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
			var txX, txY, txZ *types.PushNote
			BeforeEach(func() {
				txY = &types.PushNote{
					RepoName: "repo", NodeSig: []byte("sig"), PusherKeyID: "pk_id",
					Timestamp: 100000000,
					References: []*types.PushedReference{
						{Name: "refs/heads/master", Nonce: 1, Fee: "0.4", AccountNonce: 2},
					},
				}

				txZ = &types.PushNote{
					RepoName: "repo", NodeSig: []byte("sig"), PusherKeyID: "pk_id",
					Timestamp: 100000000,
					References: []*types.PushedReference{
						{Name: "refs/heads/update", Nonce: 1, Fee: "0.4", AccountNonce: 2},
					},
				}

				txX = &types.PushNote{
					RepoName: "repo", NodeSig: []byte("sig"), PusherKeyID: "pk_id",
					Timestamp: 100000000,
					References: []*types.PushedReference{
						{Name: "refs/heads/master", Nonce: 1, Fee: "0.4", AccountNonce: 2},
						{Name: "refs/heads/update", Nonce: 1, Fee: "0.3", AccountNonce: 2},
					},
				}

				pool.noteChecker = txCheckNoIssue
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

		When("validation check fails", func() {
			var txX *types.PushNote
			BeforeEach(func() {
				txX = &types.PushNote{
					RepoName: "repo", NodeSig: []byte("sig"), PusherKeyID: "pk_id",
					Timestamp: 100000000,
					References: []*types.PushedReference{
						{Name: "refs/heads/master", Nonce: 1, Fee: "0.01", AccountNonce: 2},
					},
				}

				pool.noteChecker = txCheckErr(fmt.Errorf("check failed"))
				err = pool.Add(txX)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("validation failed: check failed"))
			})
		})

		When("noValidation argument is true", func() {
			var txX *types.PushNote
			BeforeEach(func() {
				txX = &types.PushNote{
					RepoName: "repo", NodeSig: []byte("sig"), PusherKeyID: "pk_id",
					Timestamp: 100000000,
					References: []*types.PushedReference{
						{Name: "refs/heads/master", Nonce: 1, Fee: "0.01", AccountNonce: 2},
					},
				}

				pool.noteChecker = txCheckErr(fmt.Errorf("check failed"))
				err = pool.Add(txX, true)
			})

			It("should return no err", func() {
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".Get", func() {
		var err error

		When("note exist", func() {
			BeforeEach(func() {
				pool = NewPushPool(2, mockKeeper, mockDHT)
				pool.noteChecker = txCheckNoIssue
				err = pool.Add(tx)
				Expect(err).To(BeNil())
			})

			It("should return the push note", func() {
				note := pool.Get(tx.ID().String())
				Expect(note).To(Equal(tx))
			})
		})

		When("note does not exist", func() {
			BeforeEach(func() {
				pool = NewPushPool(2, mockKeeper, mockDHT)
				pool.noteChecker = txCheckNoIssue
			})

			It("should return the push note", func() {
				note := pool.Get(tx.ID().String())
				Expect(note).To(BeNil())
			})
		})
	})

	Describe(".remove", func() {
		var err error

		When("pool has a tx", func() {
			BeforeEach(func() {
				pool = NewPushPool(2, mockKeeper, mockDHT)
				pool.noteChecker = txCheckNoIssue
				err = pool.Add(tx)
				Expect(err).To(BeNil())
				Expect(pool.container).To(HaveLen(1))
				Expect(pool.index).To(HaveLen(1))
				Expect(pool.refIndex).To(HaveLen(1))
				Expect(pool.refNonceIdx).To(HaveLen(1))
				Expect(pool.repoNotesIdx).To(HaveLen(1))
				pool.remove(tx)
			})

			Specify("that the container and all indexes are empty", func() {
				Expect(pool.container).To(HaveLen(0))
				Expect(pool.index).To(HaveLen(0))
				Expect(pool.refIndex).To(HaveLen(0))
				Expect(pool.refNonceIdx).To(HaveLen(0))
				Expect(pool.repoNotesIdx).To(BeEmpty())
			})
		})

		When("pool has two txs", func() {
			BeforeEach(func() {
				pool = NewPushPool(2, mockKeeper, mockDHT)
				pool.noteChecker = txCheckNoIssue
				err = pool.Add(tx)
				err = pool.Add(tx2)
				Expect(err).To(BeNil())
				Expect(pool.container).To(HaveLen(2))
				Expect(pool.index).To(HaveLen(2))
				Expect(pool.refIndex).To(HaveLen(2))
				Expect(pool.refNonceIdx).To(HaveLen(2))
				Expect(pool.repoNotesIdx).To(HaveLen(2))
				pool.remove(tx)
			})

			Specify("that the container and all indexes have 1 item in them", func() {
				Expect(pool.container).To(HaveLen(1))
				Expect(pool.index).To(HaveLen(1))
				Expect(pool.refIndex).To(HaveLen(1))
				Expect(pool.refNonceIdx).To(HaveLen(1))
				Expect(pool.repoNotesIdx).To(HaveLen(1))
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

var _ = Describe("repoNotesIndex", func() {
	Describe(".add", func() {
		var idx repoNotesIndex
		BeforeEach(func() {
			idx = repoNotesIndex(map[string][]*containerItem{})
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
				idx = repoNotesIndex(map[string][]*containerItem{})
				has = idx.has("repo1")
			})

			It("should return false", func() {
				Expect(has).To(BeFalse())
			})
		})

		When("repo exist in index", func() {
			var has bool

			BeforeEach(func() {
				idx = repoNotesIndex(map[string][]*containerItem{
					"repo1": []*containerItem{},
				})
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
			var txA *types.PushNote
			BeforeEach(func() {
				txA = &types.PushNote{RepoName: "repo1", NodeSig: []byte("sig"), PusherKeyID: "pk_id", Timestamp: 100000000}
				idx = repoNotesIndex(map[string][]*containerItem{})
				idx.add("repo1", &containerItem{Note: txA})
				Expect(idx["repo1"]).To(HaveLen(1))
			})

			It("should remove repo completely", func() {
				idx.remove("repo1", txA.ID().String())
				Expect(idx).To(BeEmpty())
			})
		})

		When("repo has 2 txs (txA and TxB) and txA is removed", func() {
			var txA, txB *types.PushNote
			BeforeEach(func() {
				txA = &types.PushNote{RepoName: "repo1", NodeSig: []byte("sig"), PusherKeyID: "pk_id", Timestamp: 100000000}
				txB = &types.PushNote{RepoName: "repo1", NodeSig: []byte("sig"), PusherKeyID: "pk_id", Timestamp: 200000000}
				idx = repoNotesIndex(map[string][]*containerItem{})
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
