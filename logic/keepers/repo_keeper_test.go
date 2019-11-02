package keepers

import (
	"github.com/makeos/mosdef/storage/tree"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	tmdb "github.com/tendermint/tm-db"
)

var _ = Describe("Account", func() {
	var state *tree.SafeTree
	var rk *RepoKeeper

	BeforeEach(func() {
		state = tree.NewSafeTree(tmdb.NewMemDB(), 128)
		rk = NewRepoKeeper(state)
	})

	Describe(".GetRepo", func() {
		When("repository does not exist", func() {
			It("should return a bare account", func() {
				repo := rk.GetRepo("unknown", 0)
				Expect(repo).To(Equal(types.BareRepository()))
			})
		})

		When("repository exists", func() {
			var testRepo = types.BareRepository()

			BeforeEach(func() {
				testRepo.CreatorPubKey = "creator_pk"
				repoKey := MakeRepoKey("repo1")
				state.Set(repoKey, testRepo.Bytes())
				_, _, err := state.SaveVersion()
				Expect(err).To(BeNil())
			})

			It("should successfully return the expected repo object", func() {
				acct := rk.GetRepo("repo1", 0)
				Expect(acct).To(BeEquivalentTo(testRepo))
			})
		})
	})

	Describe(".Update", func() {
		It("should update balance", func() {
			key := "repo1"
			repo := rk.GetRepo(key)
			Expect(repo.CreatorPubKey).To(Equal(util.String("")))

			repo.CreatorPubKey = "creator_pk"
			rk.Update(key, repo)

			repo2 := rk.GetRepo(key)
			Expect(repo2).To(Equal(repo))
		})
	})
})
