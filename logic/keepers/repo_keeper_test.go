package keepers

import (
	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/storage"
	"github.com/makeos/mosdef/storage/tree"
	"github.com/makeos/mosdef/testutil"
	"github.com/makeos/mosdef/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	tmdb "github.com/tendermint/tm-db"
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

	Describe(".GetRepo", func() {
		When("repository does not exist", func() {
			It("should return a bare repository", func() {
				repo := rk.GetRepo("unknown", 0)
				Expect(repo).To(Equal(types.BareRepository()))
			})
		})

		When("repository exists", func() {
			var testRepo = types.BareRepository()

			BeforeEach(func() {
				testRepo.AddOwner("owner", &types.RepoOwner{})

				repoKey := MakeRepoKey("repo1")
				state.Set(repoKey, testRepo.Bytes())
				_, _, err := state.SaveVersion()
				Expect(err).To(BeNil())
			})

			It("should successfully return the expected repo object", func() {
				repo := rk.GetRepo("repo1", 0)
				Expect(repo).To(BeEquivalentTo(testRepo))
			})
		})

		When("repo has a proposal that was introduced at height/stateVersion=1", func() {
			testRepo := types.BareRepository()
			repoAtVersion1 := types.BareRepository()

			BeforeEach(func() {
				repoAtVersion1.Config.Governace.ProposalFee = 100000
				state.Set(MakeRepoKey("repo1"), repoAtVersion1.Bytes())
				_, _, err := state.SaveVersion()
				Expect(err).To(BeNil())

				testRepo.Proposals.Add("1", &types.RepoProposal{Height: 1})
				testRepo.AddOwner("owner", &types.RepoOwner{})

				repoKey := MakeRepoKey("repo1")
				state.Set(repoKey, testRepo.Bytes())
				_, _, err = state.SaveVersion()
				Expect(err).To(BeNil())
			})

			It("should set proposal config to the config of the repo at height/stateVersion=1", func() {
				repo := rk.GetRepo("repo1", 0)
				Expect(repo).ToNot(BeEquivalentTo(testRepo))
				Expect(repo.Proposals).To(HaveLen(1))
				Expect(repo.Proposals.Get("1").Config).To(Equal(repoAtVersion1.Config.Governace))
			})
		})

		When("repo has a proposal with a config height that is the same as the current state version", func() {
			repo := types.BareRepository()

			BeforeEach(func() {
				// Version 1
				repo.Config.Governace.ProposalFee = 100000
				state.Set(MakeRepoKey("repo1"), repo.Bytes())
				state.SaveVersion()

				// Version 2
				repo.Config.Governace.ProposalFee = 200000
				state.Set(MakeRepoKey("repo1"), repo.Bytes())
				state.SaveVersion()

				repo.Proposals.Add("1", &types.RepoProposal{Height: 3})
				repo.AddOwner("owner", &types.RepoOwner{})

				repoKey := MakeRepoKey("repo1")
				state.Set(repoKey, repo.Bytes())

				// Version 3
				state.SaveVersion()
			})

			It("should set proposal config to the config of the repo at", func() {
				repo := rk.GetRepo("repo1", 0)
				Expect(repo.Proposals).To(HaveLen(1))
				Expect(repo.Proposals.Get("1").Config).To(Equal(repo.Config.Governace))
			})
		})
	})

	Describe(".Update", func() {
		It("should update repo object", func() {
			key := "repo1"
			repo := rk.GetRepo(key)
			Expect(repo.Owners).To(BeEmpty())

			repo.AddOwner("owner", &types.RepoOwner{})
			rk.Update(key, repo)

			repo2 := rk.GetRepo(key)
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
})
