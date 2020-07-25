package keepers

import (
	"os"

	state2 "github.com/themakeos/lobe/types/state"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	tmdb "github.com/tendermint/tm-db"
	"github.com/themakeos/lobe/config"
	"github.com/themakeos/lobe/pkgs/tree"
	"github.com/themakeos/lobe/storage"
	"github.com/themakeos/lobe/testutil"
)

var _ = Describe("RepoKeeper", func() {
	var state *tree.SafeTree
	var rk *RepoKeeper
	var err error
	var cfg *config.AppConfig
	var appDB *storage.Badger

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		appDB, _ = testutil.GetDB(cfg)
		state = tree.NewSafeTree(tmdb.NewMemDB(), 128)
		rk = NewRepoKeeper(state, appDB)
	})

	AfterEach(func() {
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".Get", func() {
		When("repository does not exist", func() {
			It("should return a bare repository", func() {
				repo := rk.Get("unknown", 0)
				Expect(repo).To(Equal(state2.BareRepository()))
			})
		})

		When("repository exists", func() {
			var testRepo = state2.BareRepository()

			BeforeEach(func() {
				testRepo.AddOwner("owner", &state2.RepoOwner{})

				repoKey := MakeRepoKey("repo1")
				state.Set(repoKey, testRepo.Bytes())
				_, _, err := state.SaveVersion()
				Expect(err).To(BeNil())
			})

			It("should successfully return the expected repo object", func() {
				repo := rk.Get("repo1", 0)
				Expect(repo).To(BeEquivalentTo(testRepo))
			})
		})

		When("repo has a proposal that was introduced at height/stateVersion=1", func() {
			testRepo := state2.BareRepository()
			repoAtVersion1 := state2.BareRepository()

			BeforeEach(func() {
				repoAtVersion1.Config.Gov.PropFee = 100000
				state.Set(MakeRepoKey("repo1"), repoAtVersion1.Bytes())
				_, _, err := state.SaveVersion()
				Expect(err).To(BeNil())

				testRepo.Proposals.Add("1", &state2.RepoProposal{Height: 1})
				testRepo.AddOwner("owner", &state2.RepoOwner{})

				repoKey := MakeRepoKey("repo1")
				state.Set(repoKey, testRepo.Bytes())
				_, _, err = state.SaveVersion()
				Expect(err).To(BeNil())
			})

			It("should set proposal config to the config of the repo at height/stateVersion=1", func() {
				repo := rk.Get("repo1", 0)
				Expect(repo).ToNot(BeEquivalentTo(testRepo))
				Expect(repo.Proposals).To(HaveLen(1))
				Expect(repo.Proposals.Get("1").Config).To(Equal(repoAtVersion1.Config.Gov))
			})
		})

		When("repo has a proposal with a config height that is the same as the current state version", func() {
			repo := state2.BareRepository()

			BeforeEach(func() {
				// Version 1
				repo.Config.Gov.PropFee = 100000
				state.Set(MakeRepoKey("repo1"), repo.Bytes())
				state.SaveVersion()

				// Version 2
				repo.Config.Gov.PropFee = 200000
				state.Set(MakeRepoKey("repo1"), repo.Bytes())
				state.SaveVersion()

				repo.Proposals.Add("1", &state2.RepoProposal{Height: 3})
				repo.AddOwner("owner", &state2.RepoOwner{})

				repoKey := MakeRepoKey("repo1")
				state.Set(repoKey, repo.Bytes())

				// Version 3
				state.SaveVersion()
			})

			It("should set proposal config to the config of the repo at", func() {
				repo := rk.Get("repo1", 0)
				Expect(repo.Proposals).To(HaveLen(1))
				Expect(repo.Proposals.Get("1").Config).To(Equal(repo.Config.Gov))
			})
		})
	})

	Describe(".Update", func() {
		It("should update repo object", func() {
			key := "repo1"
			repo := rk.Get(key)
			Expect(repo.Owners).To(BeEmpty())

			repo.AddOwner("owner", &state2.RepoOwner{})
			rk.Update(key, repo)

			repo2 := rk.Get(key)
			Expect(repo2).To(Equal(repo))
		})
	})

	Describe(".IndexProposalVote", func() {
		It("should save repo proposal vote", func() {
			err := rk.IndexProposalVote("repo1", "prop1", "addr", 1)
			Expect(err).To(BeNil())

			key := MakeRepoProposalVoteKey("repo1", "prop1", "addr")
			rec, err := appDB.Get(key)
			Expect(err).To(BeNil())
			Expect(rec.Value).To(Equal([]byte("1")))
		})
	})

	Describe(".GetProposalVote", func() {
		When("proposal vote was indexed", func() {
			It("should get repo proposal vote and found=true", func() {
				err := rk.IndexProposalVote("repo1", "prop1", "addr", 1)
				Expect(err).To(BeNil())

				vote, found, err := rk.GetProposalVote("repo1", "prop1", "addr")
				Expect(err).To(BeNil())
				Expect(vote).To(Equal(1))
				Expect(found).To(BeTrue())
			})
		})

		When("proposal vote was not indexed", func() {
			It("should not get repo proposal vote and found=false", func() {
				vote, found, err := rk.GetProposalVote("repo1", "prop1", "addr")
				Expect(err).To(BeNil())
				Expect(vote).To(Equal(0))
				Expect(found).To(BeFalse())
			})
		})
	})

	Describe(".IndexProposalEnd", func() {
		It("should save repo proposal by end height", func() {
			err := rk.IndexProposalEnd("repo1", "prop1", 100)
			Expect(err).To(BeNil())

			key := MakeRepoProposalEndIndexKey("repo1", "prop1", 100)
			rec, err := appDB.Get(key)
			Expect(err).To(BeNil())
			Expect(rec.Value).To(Equal([]byte("0")))
		})
	})

	Describe(".GetProposalsEndingAt", func() {
		When("only one proposal exist at end height 100", func() {
			It("should return 1 result", func() {
				err := rk.IndexProposalEnd("repo1", "prop1", 100)
				Expect(err).To(BeNil())
				res := rk.GetProposalsEndingAt(100)
				Expect(res).To(HaveLen(1))
				Expect(res[0].RepoName).To(Equal("repo1"))
				Expect(res[0].ProposalID).To(Equal("prop1"))
				Expect(res[0].EndHeight).To(Equal(uint64(100)))
			})
		})

		When("there are two proposal exist at end height 100", func() {
			It("should return 2 results", func() {
				err := rk.IndexProposalEnd("repo1", "prop1", 100)
				Expect(err).To(BeNil())
				err = rk.IndexProposalEnd("repo2", "prop2", 100)
				Expect(err).To(BeNil())

				res := rk.GetProposalsEndingAt(100)
				Expect(res).To(HaveLen(2))
				Expect(res[0].RepoName).To(Equal("repo1"))
				Expect(res[0].ProposalID).To(Equal("prop1"))
				Expect(res[0].EndHeight).To(Equal(uint64(100)))
				Expect(res[1].RepoName).To(Equal("repo2"))
				Expect(res[1].ProposalID).To(Equal("prop2"))
				Expect(res[1].EndHeight).To(Equal(uint64(100)))
			})
		})
	})

	Describe(".MarkProposalAsClosed", func() {
		It("should add mark", func() {
			err := rk.MarkProposalAsClosed("repo1", "prop1")
			Expect(err).To(BeNil())

			key := MakeClosedProposalKey("repo1", "prop1")
			rec, err := appDB.Get(key)
			Expect(err).To(BeNil())
			Expect(rec.Value).To(Equal([]byte("0")))
		})
	})

	Describe(".IsProposalClosed", func() {
		When("a proposal is not marked closed", func() {
			It("should return false and nil error", func() {
				closed, err := rk.IsProposalClosed("repo1", "prop1")
				Expect(err).To(BeNil())
				Expect(closed).To(BeFalse())
			})
		})

		When("a proposal is marked closed", func() {
			It("should return true and nil error", func() {
				err := rk.MarkProposalAsClosed("repo1", "prop1")
				Expect(err).To(BeNil())
				closed, err := rk.IsProposalClosed("repo1", "prop1")
				Expect(err).To(BeNil())
				Expect(closed).To(BeTrue())
			})
		})
	})
})
