package validation_test

import (
	"fmt"
	"os"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/mocks"
	plumbing2 "gitlab.com/makeos/mosdef/remote/plumbing"
	"gitlab.com/makeos/mosdef/remote/repo"
	"gitlab.com/makeos/mosdef/remote/validation"
	"gitlab.com/makeos/mosdef/testutil"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/util"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/filemode"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

var _ = Describe("Validation", func() {
	var err error
	var cfg *config.AppConfig
	var ctrl *gomock.Controller
	var mockRepo *mocks.MockBareRepo
	var commit *mocks.MockCommit

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"

		commit = mocks.NewMockCommit(ctrl)
		mockRepo = mocks.NewMockBareRepo(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".ValidateIssueCommit", func() {
		It("should return error when unable to get commit object from local repo", func() {
			change := &core.ItemChange{Item: &plumbing2.Obj{Data: "069199ae527ca118368d93af02feefa80432e563"}}
			mockRepo.EXPECT().WrappedCommitObject(plumbing.NewHash(change.Item.GetData())).Return(nil, fmt.Errorf("error"))
			args := &validation.ValidateIssueCommitArg{OldHash: "", Change: change}
			err := validation.ValidateIssueCommit(mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("unable to get commit object: error"))
		})

		It("should return error when commit failed commit validation", func() {
			change := &core.ItemChange{Item: &plumbing2.Obj{Data: "069199ae527ca118368d93af02feefa80432e563"}}
			mockRepo.EXPECT().WrappedCommitObject(plumbing.NewHash(change.Item.GetData())).Return(repo.NewWrappedCommit(&object.Commit{}), nil)
			args := &validation.ValidateIssueCommitArg{OldHash: "", Change: change,
				CheckCommit: func(commit *object.Commit, txDetail *types.TxDetail, getPushKey core.PushKeyGetter) error {
					return fmt.Errorf("check error")
				},
			}
			err := validation.ValidateIssueCommit(mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("check error"))
		})

		It("should return error when commit failed issue commit validation ", func() {
			change := &core.ItemChange{Item: &plumbing2.Obj{Data: "069199ae527ca118368d93af02feefa80432e563"}}
			testCommit := repo.NewWrappedCommit(&object.Commit{Message: "commit 1"})
			mockRepo.EXPECT().WrappedCommitObject(plumbing.NewHash(change.Item.GetData())).Return(testCommit, nil)
			detail := &types.TxDetail{
				Reference: "refs/heads/issue/1",
			}

			checkIssueCommitCalled := 0
			args := &validation.ValidateIssueCommitArg{OldHash: "", Change: change,
				TxDetail: detail,
				CheckCommit: func(commit *object.Commit, txDetail *types.TxDetail, getPushKey core.PushKeyGetter) error {
					return nil
				},
				CheckIssueCommit: func(commit core.Commit, reference, oldHash string, r core.BareRepo) (*plumbing2.IssueBody, error) {
					checkIssueCommitCalled++
					Expect(commit).To(Equal(testCommit))
					Expect(reference).To(Equal(detail.Reference))
					return nil, fmt.Errorf("issue check error")
				},
			}
			err := validation.ValidateIssueCommit(mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("issue check error"))
			Expect(checkIssueCommitCalled).To(Equal(1))
		})

		When("commit has no parent", func() {
			It("should return no error when issue commit check is passed and issue checker func is called once", func() {
				change := &core.ItemChange{Item: &plumbing2.Obj{Data: "069199ae527ca118368d93af02feefa80432e563"}}
				testCommit := repo.NewWrappedCommit(&object.Commit{Message: "commit 1"})
				mockRepo.EXPECT().WrappedCommitObject(plumbing.NewHash(change.Item.GetData())).Return(testCommit, nil)
				detail := &types.TxDetail{Reference: "refs/heads/issue/1"}

				checkIssueCommitCalled := 0
				args := &validation.ValidateIssueCommitArg{OldHash: "", Change: change,
					TxDetail: detail,
					CheckCommit: func(commit *object.Commit, txDetail *types.TxDetail, getPushKey core.PushKeyGetter) error {
						return nil
					},
					CheckIssueCommit: func(commit core.Commit, reference, oldHash string, r core.BareRepo) (*plumbing2.IssueBody, error) {
						checkIssueCommitCalled++
						Expect(commit).To(Equal(testCommit))
						Expect(reference).To(Equal(detail.Reference))
						return &plumbing2.IssueBody{}, nil
					},
				}
				err := validation.ValidateIssueCommit(mockRepo, args)
				Expect(err).To(BeNil())
				Expect(checkIssueCommitCalled).To(Equal(1))
			})

			It("should set tx detail policy checker flag to true if issue body updates admin fields like 'labels'", func() {
				change := &core.ItemChange{Item: &plumbing2.Obj{Data: "069199ae527ca118368d93af02feefa80432e563"}}
				testCommit := repo.NewWrappedCommit(&object.Commit{Message: "commit 1"})
				mockRepo.EXPECT().WrappedCommitObject(plumbing.NewHash(change.Item.GetData())).Return(testCommit, nil)
				detail := &types.TxDetail{Reference: "refs/heads/issue/1"}

				checkIssueCommitCalled := 0
				args := &validation.ValidateIssueCommitArg{OldHash: "", Change: change,
					TxDetail: detail,
					CheckCommit: func(commit *object.Commit, txDetail *types.TxDetail, getPushKey core.PushKeyGetter) error {
						return nil
					},
					CheckIssueCommit: func(commit core.Commit, reference, oldHash string, r core.BareRepo) (*plumbing2.IssueBody, error) {
						checkIssueCommitCalled++
						Expect(commit).To(Equal(testCommit))
						Expect(reference).To(Equal(detail.Reference))
						return &plumbing2.IssueBody{Labels: []string{"label_update"}}, nil
					},
				}
				err := validation.ValidateIssueCommit(mockRepo, args)
				Expect(err).To(BeNil())
				Expect(checkIssueCommitCalled).To(Equal(1))
				Expect(detail.FlagCheckIssueUpdatePolicy).To(BeTrue())
			})
		})

		When("commit has a parent", func() {
			It("should return error when parent commit failed issue commit check and issue checker func is called twice", func() {
				change := &core.ItemChange{Item: &plumbing2.Obj{Data: "069199ae527ca118368d93af02feefa80432e563"}}
				testCommit := mocks.NewMockCommit(ctrl)
				mockRepo.EXPECT().WrappedCommitObject(plumbing.NewHash(change.Item.GetData())).Return(testCommit, nil)
				testCommit.EXPECT().UnWrap().Return(&object.Commit{})
				testCommit.EXPECT().NumParents().Return(1)
				parentCommit := mocks.NewMockCommit(ctrl)
				parentCommit.EXPECT().GetHash().Return(plumbing.NewHash("d5a0d6d0bae56ce76c3f29f9b4006ccc8ea452a4"))
				parentCommit.EXPECT().NumParents().Return(0)
				testCommit.EXPECT().Parent(0).Return(parentCommit, nil)

				checkIssueCommitCalled := 0
				detail := &types.TxDetail{Reference: "refs/heads/issue/1"}
				args := &validation.ValidateIssueCommitArg{OldHash: "", Change: change,
					TxDetail: detail,
					CheckCommit: func(commit *object.Commit, txDetail *types.TxDetail, getPushKey core.PushKeyGetter) error {
						return nil
					},
					CheckIssueCommit: func(commit core.Commit, reference, oldHash string, r core.BareRepo) (*plumbing2.IssueBody, error) {
						checkIssueCommitCalled++
						Expect(reference).To(Equal(detail.Reference))
						return &plumbing2.IssueBody{}, nil
					},
				}
				err := validation.ValidateIssueCommit(mockRepo, args)
				Expect(err).To(BeNil())
				Expect(checkIssueCommitCalled).To(Equal(2))
			})

			It("should return error when parent commit hash is the same with old commit hash and issue checker func is called once", func() {
				change := &core.ItemChange{Item: &plumbing2.Obj{Data: "069199ae527ca118368d93af02feefa80432e563"}}
				oldHash := "d5a0d6d0bae56ce76c3f29f9b4006ccc8ea452a4"

				testCommit := mocks.NewMockCommit(ctrl)
				mockRepo.EXPECT().WrappedCommitObject(plumbing.NewHash(change.Item.GetData())).Return(testCommit, nil)
				testCommit.EXPECT().UnWrap().Return(&object.Commit{})
				testCommit.EXPECT().NumParents().Return(1)
				parentCommit := mocks.NewMockCommit(ctrl)
				parentCommit.EXPECT().GetHash().Return(plumbing.NewHash(oldHash))
				testCommit.EXPECT().Parent(0).Return(parentCommit, nil)

				checkIssueCommitCalled := 0
				detail := &types.TxDetail{Reference: "refs/heads/issue/1"}
				args := &validation.ValidateIssueCommitArg{OldHash: oldHash, Change: change,
					TxDetail: detail,
					CheckCommit: func(commit *object.Commit, txDetail *types.TxDetail, getPushKey core.PushKeyGetter) error {
						return nil
					},
					CheckIssueCommit: func(commit core.Commit, reference, oldHash string, r core.BareRepo) (*plumbing2.IssueBody, error) {
						checkIssueCommitCalled++
						Expect(reference).To(Equal(detail.Reference))
						return &plumbing2.IssueBody{}, nil
					},
				}
				err := validation.ValidateIssueCommit(mockRepo, args)
				Expect(err).To(BeNil())
				Expect(checkIssueCommitCalled).To(Equal(1))
			})
		})
	})

	Describe(".CheckIssueCommit", func() {
		It("should return error when issue number is not valid", func() {
			_, err := validation.CheckIssueCommit(commit, "refs/heads/"+plumbing2.IssueBranchPrefix+"/abc", "", mockRepo)
			Expect(err).NotTo(BeNil())
			Expect(err).To(MatchError("issue number is not valid. Must be numeric"))
		})

		It("should return error when issue commit has more than 1 parents", func() {
			issueBranch := plumbing2.MakeIssueReference(1)
			commit.EXPECT().NumParents().Return(2)
			_, err := validation.CheckIssueCommit(commit, issueBranch, "", mockRepo)
			Expect(err).NotTo(BeNil())
			Expect(err).To(MatchError("issue commit cannot have more than one parent"))
		})

		It("should return error when reference has a merge commit in its history", func() {
			issueBranch := plumbing2.MakeIssueReference(1)
			commit.EXPECT().NumParents().Return(1)
			mockRepo.EXPECT().HasMergeCommits(issueBranch).Return(false, fmt.Errorf("error"))
			_, err := validation.CheckIssueCommit(commit, issueBranch, "", mockRepo)
			Expect(err).NotTo(BeNil())
			Expect(err).To(MatchError("failed to check for merges in issue commit history: error"))
		})

		It("should return error when the reference of the issue commit is new but the issue commit has multiple parents ", func() {
			issueBranch := plumbing2.MakeIssueReference(1)
			commit.EXPECT().NumParents().Return(1).Times(2)
			repoState := &state.Repository{}
			mockRepo.EXPECT().GetState().Return(repoState)
			mockRepo.EXPECT().HasMergeCommits(issueBranch).Return(false, nil)
			_, err := validation.CheckIssueCommit(commit, issueBranch, "", mockRepo)
			Expect(err).NotTo(BeNil())
			Expect(err).To(MatchError("first commit of a new issue must have no parent"))
		})

		It("should return error when the issue commit alters history", func() {
			issueBranch := plumbing2.MakeIssueReference(1)
			commit.EXPECT().NumParents().Return(1)
			commit.EXPECT().GetHash().Return(plumbing2.MakeCommitHash("hash"))
			repoState := &state.Repository{References: map[string]*state.Reference{issueBranch: {}}}
			mockRepo.EXPECT().GetState().Return(repoState)
			mockRepo.EXPECT().HasMergeCommits(issueBranch).Return(false, nil)
			mockRepo.EXPECT().IsAncestor(gomock.Any(), gomock.Any()).Return(fmt.Errorf("error"))
			_, err := validation.CheckIssueCommit(commit, issueBranch, "", mockRepo)
			Expect(err).NotTo(BeNil())
			Expect(err).To(MatchError("issue commit must not alter history"))
		})

		It("should return error when unable to get commit tree", func() {
			issueBranch := plumbing2.MakeIssueReference(1)
			commit.EXPECT().NumParents().Return(1)
			commit.EXPECT().GetHash().Return(plumbing2.MakeCommitHash("hash"))
			commit.EXPECT().GetTree().Return(nil, fmt.Errorf("bad query"))
			repoState := &state.Repository{References: map[string]*state.Reference{issueBranch: {}}}
			mockRepo.EXPECT().GetState().Return(repoState)
			mockRepo.EXPECT().HasMergeCommits(issueBranch).Return(false, nil)
			mockRepo.EXPECT().IsAncestor(gomock.Any(), gomock.Any()).Return(nil)
			_, err := validation.CheckIssueCommit(commit, issueBranch, "", mockRepo)
			Expect(err).NotTo(BeNil())
			Expect(err).To(MatchError("unable to read issue commit tree"))
		})

		It("should return error when issue commit tree does not have 'body' file", func() {
			issueBranch := plumbing2.MakeIssueReference(1)
			commit.EXPECT().NumParents().Return(1)
			commit.EXPECT().GetHash().Return(plumbing2.MakeCommitHash("hash"))
			tree := &object.Tree{Entries: []object.TreeEntry{}}
			commit.EXPECT().GetTree().Return(tree, nil)
			repoState := &state.Repository{References: map[string]*state.Reference{issueBranch: {}}}
			mockRepo.EXPECT().GetState().Return(repoState)
			mockRepo.EXPECT().HasMergeCommits(issueBranch).Return(false, nil)
			mockRepo.EXPECT().IsAncestor(gomock.Any(), gomock.Any()).Return(nil)
			_, err := validation.CheckIssueCommit(commit, issueBranch, "", mockRepo)
			Expect(err).NotTo(BeNil())
			Expect(err).To(MatchError("issue commit must have a 'body' file"))
		})

		It("should return error when issue commit tree has more than 1 files", func() {
			issueBranch := plumbing2.MakeIssueReference(1)
			commit.EXPECT().NumParents().Return(1)
			commit.EXPECT().GetHash().Return(plumbing2.MakeCommitHash("hash"))
			tree := &object.Tree{Entries: []object.TreeEntry{{}, {}}}
			commit.EXPECT().GetTree().Return(tree, nil)
			repoState := &state.Repository{References: map[string]*state.Reference{issueBranch: {}}}
			mockRepo.EXPECT().GetState().Return(repoState)
			mockRepo.EXPECT().HasMergeCommits(issueBranch).Return(false, nil)
			mockRepo.EXPECT().IsAncestor(gomock.Any(), gomock.Any()).Return(nil)
			_, err := validation.CheckIssueCommit(commit, issueBranch, "", mockRepo)
			Expect(err).NotTo(BeNil())
			Expect(err).To(MatchError("issue commit tree must only include a 'body' file"))
		})

		It("should return error when issue commit tree has a body entry that isn't a regular file", func() {
			issueBranch := plumbing2.MakeIssueReference(1)
			commit.EXPECT().NumParents().Return(1)
			commit.EXPECT().GetHash().Return(plumbing2.MakeCommitHash("hash"))
			tree := &object.Tree{Entries: []object.TreeEntry{{Name: "body", Mode: filemode.Dir}}}
			commit.EXPECT().GetTree().Return(tree, nil)
			repoState := &state.Repository{References: map[string]*state.Reference{issueBranch: {}}}
			mockRepo.EXPECT().GetState().Return(repoState)
			mockRepo.EXPECT().HasMergeCommits(issueBranch).Return(false, nil)
			mockRepo.EXPECT().IsAncestor(gomock.Any(), gomock.Any()).Return(nil)
			_, err := validation.CheckIssueCommit(commit, issueBranch, "", mockRepo)
			Expect(err).NotTo(BeNil())
			Expect(err).To(MatchError("issue body file is not a regular file"))
		})
	})

	Describe(".CheckIssueBody", func() {
		var commit *object.Commit

		BeforeEach(func() {
			commit = &object.Commit{Hash: plumbing2.MakeCommitHash("hash")}
		})

		It("should return error when an unexpected field exists", func() {
			err := validation.CheckIssueBody(nil, repo.NewWrappedCommit(commit), true, map[string]interface{}{"field1": "xyz"}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.field1, msg:unknown field"))
		})

		It("should return error when an 'title' value is not string", func() {
			err := validation.CheckIssueBody(nil, repo.NewWrappedCommit(commit), true, map[string]interface{}{"title": 1}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.title, msg:expected a string value"))
		})

		It("should return error when an 'replyTo' value is not string", func() {
			err := validation.CheckIssueBody(nil, repo.NewWrappedCommit(commit), true, map[string]interface{}{"replyTo": 1}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.replyTo, msg:expected a string value"))
		})

		It("should return error when a 'labels' value is not a string slice", func() {
			err := validation.CheckIssueBody(nil, repo.NewWrappedCommit(commit), true, map[string]interface{}{"labels": []int{1}}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.labels, msg:expected a list of string values"))
		})

		It("should return error when an 'assignees' value is not a string slice", func() {
			err := validation.CheckIssueBody(nil, repo.NewWrappedCommit(commit), true, map[string]interface{}{"assignees": []int{1}}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.assignees, msg:expected a list of string values"))
		})

		It("should return error when a 'fixers' value is not a string slice", func() {
			err := validation.CheckIssueBody(nil, repo.NewWrappedCommit(commit), true, map[string]interface{}{"fixers": []int{1}}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.fixers, msg:expected a list of string values"))
		})

		It("should return error when a 'reactions' value is not a string slice", func() {
			err := validation.CheckIssueBody(nil, repo.NewWrappedCommit(commit), true, map[string]interface{}{"reactions": []int{1}}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.reactions, msg:expected a list of string values"))
		})

		It("should return error when a 'replyTo' field is set and issue commit is new", func() {
			err := validation.CheckIssueBody(nil, repo.NewWrappedCommit(commit), true, map[string]interface{}{"replyTo": "xyz"}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.replyTo, msg:not expected in a new issue commit"))
		})

		It("should return error when issue is not new, a 'replyTo' field is set and title is set", func() {
			err := validation.CheckIssueBody(nil, repo.NewWrappedCommit(commit), false, map[string]interface{}{"replyTo": "xyz", "title": "abc"}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.title, msg:title is not required when replying"))
		})

		It("should return error when issue is new and title is not set", func() {
			err := validation.CheckIssueBody(nil, repo.NewWrappedCommit(commit), true, map[string]interface{}{}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.title, msg:title is required"))
		})

		It("should return error when issue is not new and title is set", func() {
			err := validation.CheckIssueBody(nil, repo.NewWrappedCommit(commit), false, map[string]interface{}{"title": "xyz"}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.title, msg:title is not required for comment commit"))
		})

		It("should return error when title length is greater than max.", func() {
			err := validation.CheckIssueBody(nil, repo.NewWrappedCommit(commit), true, map[string]interface{}{"title": util.RandString(validation.MaxIssueTitleLen + 1)}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.title, msg:title is too long and cannot exceed 256 characters"))
		})

		It("should return error when replyTo value has length < 4 or > 40", func() {
			err := validation.CheckIssueBody(nil, repo.NewWrappedCommit(commit), false, map[string]interface{}{"replyTo": "abc"}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.replyTo, msg:invalid hash value"))

			err = validation.CheckIssueBody(nil, repo.NewWrappedCommit(commit), false, map[string]interface{}{"replyTo": util.RandString(41)}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.replyTo, msg:invalid hash value"))
		})

		It("should return error when replyTo hash is not an ancestor", func() {
			replyTo := plumbing2.MakeCommitHash("hash").String()
			mockRepo := mocks.NewMockBareRepo(ctrl)
			mockRepo.EXPECT().IsAncestor(commit.Hash.String(), replyTo).Return(fmt.Errorf("error"))
			err := validation.CheckIssueBody(mockRepo, repo.NewWrappedCommit(commit), false, map[string]interface{}{"replyTo": replyTo}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.replyTo, msg:not a valid hash of a commit in the issue"))
		})

		It("should return error when reactions field is not slice of string", func() {
			err := validation.CheckIssueBody(nil, repo.NewWrappedCommit(commit), true, map[string]interface{}{
				"title":     util.RandString(10),
				"reactions": []interface{}{1, 2, 3},
			}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.reactions, msg:expected a string list"))
		})

		It("should return error when reactions exceed max", func() {
			err := validation.CheckIssueBody(nil, repo.NewWrappedCommit(commit), true, map[string]interface{}{
				"title":     util.RandString(10),
				"reactions": []interface{}{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"},
			}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.reactions, msg:too many reactions. Cannot exceed 10"))
		})

		It("should return error when a reaction is unknown", func() {
			err := validation.CheckIssueBody(nil, repo.NewWrappedCommit(commit), true, map[string]interface{}{
				"title":     util.RandString(10),
				"reactions": []interface{}{"unknown"},
			}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.reactions, msg:unknown reaction"))
		})

		It("should return error when labels exceed max", func() {
			err := validation.CheckIssueBody(nil, repo.NewWrappedCommit(commit), true, map[string]interface{}{
				"title":  util.RandString(10),
				"labels": []interface{}{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"},
			}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.labels, msg:too many labels. Cannot exceed 10"))
		})

		It("should return error when labels does not include string values", func() {
			err := validation.CheckIssueBody(nil, repo.NewWrappedCommit(commit), true, map[string]interface{}{
				"title":  util.RandString(10),
				"labels": []interface{}{1, 2},
			}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.labels, msg:expected a string list"))
		})

		It("should return error when assignees does not include string values", func() {
			err := validation.CheckIssueBody(nil, repo.NewWrappedCommit(commit), true, map[string]interface{}{
				"title":     util.RandString(10),
				"assignees": []interface{}{1, 2},
			}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.assignees, msg:expected a string list"))
		})

		It("should return error when assignees includes invalid push keys", func() {
			err := validation.CheckIssueBody(nil, repo.NewWrappedCommit(commit), true, map[string]interface{}{
				"title":     util.RandString(10),
				"assignees": []interface{}{"invalid_push_key"},
			}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("index:0, field:<commit#.*>.assignees, msg:invalid push key ID"))
		})

		It("should return error when fixers does not include string values", func() {
			err := validation.CheckIssueBody(nil, repo.NewWrappedCommit(commit), true, map[string]interface{}{
				"title":  util.RandString(10),
				"fixers": []interface{}{1, 2},
			}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.fixers, msg:expected a string list"))
		})

		It("should return error when fixers includes invalid push keys", func() {
			err := validation.CheckIssueBody(nil, repo.NewWrappedCommit(commit), true, map[string]interface{}{
				"title":  util.RandString(10),
				"fixers": []interface{}{"invalid_push_key"},
			}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("index:0, field:<commit#.*>.fixers, msg:invalid push key ID"))
		})

		It("should return error when issue is new but content is unset", func() {
			err := validation.CheckIssueBody(nil, repo.NewWrappedCommit(commit), true, map[string]interface{}{"title": util.RandString(10)}, nil)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.content, msg:issue content is required"))
		})

		It("should return error when content surpassed max. limit", func() {
			err := validation.CheckIssueBody(nil, repo.NewWrappedCommit(commit), true, map[string]interface{}{"title": util.RandString(10)}, util.RandBytes(validation.MaxIssueContentLen+1))
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(MatchRegexp("field:<commit#.*>.content, msg:issue content length exceeded max character limit"))
		})

		It("should return no error when fields are acceptable", func() {
			err := validation.CheckIssueBody(nil, repo.NewWrappedCommit(commit), true, map[string]interface{}{"title": util.RandString(10)}, []byte("abc"))
			Expect(err).To(BeNil())
		})
	})
})
