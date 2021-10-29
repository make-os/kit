package plumbing_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	plumbing2 "github.com/go-git/go-git/v5/plumbing"
	"github.com/gohugoio/hugo/parser/pageparser"
	"github.com/golang/mock/gomock"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/mocks"
	"github.com/make-os/kit/remote/plumbing"
	"github.com/make-os/kit/remote/repo"
	testutil2 "github.com/make-os/kit/remote/testutil"
	"github.com/make-os/kit/testutil"
	"github.com/make-os/kit/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Post", func() {
	var err error
	var cfg *config.AppConfig
	var testRepo plumbing.LocalRepo
	var repoName, path string
	var cls = true
	var dontClose = false
	var ctrl *gomock.Controller
	var mockRepo *mocks.MockLocalRepo

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())

		repoName = util.RandString(5)
		path = filepath.Join(cfg.GetRepoRoot(), repoName)
		testutil2.ExecGit(cfg.GetRepoRoot(), "init", repoName)

		testRepo, err = repo.GetWithGitModule(cfg.Node.GitBinPath, path)
		Expect(err).To(BeNil())

		mockRepo = mocks.NewMockLocalRepo(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe("Posts.Reverse", func() {
		It("should reverse posts", func() {
			posts := plumbing.Posts{
				&plumbing.Post{Title: "t1", Comment: nil},
				&plumbing.Post{Title: "t2", Comment: nil},
			}
			posts.Reverse()
			Expect(posts[0].GetTitle()).To(Equal("t2"))
			Expect(posts[1].GetTitle()).To(Equal("t1"))
		})
	})

	Describe("Posts.SortByFirstPostCreationTimeDesc", func() {
		It("should sort by first post creation time", func() {
			posts := plumbing.Posts{
				&plumbing.Post{Title: "t1", Comment: &plumbing.Comment{CreatedAt: time.Now().Add(-1 * time.Minute)}},
				&plumbing.Post{Title: "t2", Comment: &plumbing.Comment{CreatedAt: time.Now()}},
			}
			posts.SortByFirstPostCreationTimeDesc()
			Expect(posts[0].(*plumbing.Post).Title).To(Equal("t2"))
			Expect(posts[1].(*plumbing.Post).Title).To(Equal("t1"))
		})
	})

	Describe("Comments.Reverse", func() {
		It("should reverse posts", func() {
			posts := plumbing.Comments{{Author: "a1"}, {Author: "a2"}}
			posts.Reverse()
			Expect(posts[0].Author).To(Equal("a2"))
			Expect(posts[1].Author).To(Equal("a1"))
		})
	})

	Describe(".GetFreePostID", func() {
		It("should return error when post reference type is unknown", func() {
			_, err := plumbing.GetFreePostID(mockRepo, 1, "unknown")
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("unknown post reference type"))
		})

		It("should return error when unable to query post reference", func() {
			mockRepo.EXPECT().RefGet(plumbing.MakeIssueReference("1")).Return("", fmt.Errorf("error"))
			_, err := plumbing.GetFreePostID(mockRepo, 1, plumbing.IssueBranchPrefix)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})

		It("should return 1 when the post reference has no entries", func() {
			mockRepo.EXPECT().RefGet(plumbing.MakeIssueReference("1")).Return("", plumbing.ErrRefNotFound)
			n, err := plumbing.GetFreePostID(mockRepo, 1, plumbing.IssueBranchPrefix)
			Expect(err).To(BeNil())
			Expect(n).To(Equal(1))
		})

		It("should return 2 when the post reference has entry at index 1", func() {
			mockRepo.EXPECT().RefGet(plumbing.MakeIssueReference("1")).Return("hash1", plumbing.ErrRefNotFound)
			mockRepo.EXPECT().RefGet(plumbing.MakeIssueReference("2")).Return("", plumbing.ErrRefNotFound)
			n, err := plumbing.GetFreePostID(mockRepo, 1, plumbing.IssueBranchPrefix)
			Expect(err).To(BeNil())
			Expect(n).To(Equal(2))
		})
	})

	Describe(".GetPosts", func() {
		It("should return error when unable to get repo references", func() {
			mockRepo.EXPECT().GetReferences().Return(nil, fmt.Errorf("error"))
			_, err := plumbing.GetPosts(mockRepo, nil)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})

		It("should return error when unable to get an issue reference root commit hash", func() {
			refs := []plumbing2.ReferenceName{plumbing2.ReferenceName("refs/heads/issues/1")}
			mockRepo.EXPECT().GetReferences().Return(refs, nil)
			mockRepo.EXPECT().GetRefRootCommit(refs[0].String()).Return("", fmt.Errorf("error"))
			_, err := plumbing.GetPosts(mockRepo, func(ref plumbing2.ReferenceName) bool { return true })
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})

		It("should return error when unable to get first comment of issue reference", func() {
			refs := []plumbing2.ReferenceName{plumbing2.ReferenceName("refs/heads/issues/1")}
			mockRepo.EXPECT().GetReferences().Return(refs, nil)
			rootHash := "e41db497eff0acf90c32a3a2560b76682a262fb4"
			mockRepo.EXPECT().GetRefRootCommit(refs[0].String()).Return(rootHash, nil)
			mockRepo.EXPECT().ReadPostBody(rootHash).Return(nil, nil, fmt.Errorf("error"))
			_, err := plumbing.GetPosts(mockRepo, func(ref plumbing2.ReferenceName) bool { return true })
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})

		It("should return error when unable to get an issue reference hash", func() {
			refs := []plumbing2.ReferenceName{plumbing2.ReferenceName("refs/heads/issues/1")}
			mockRepo.EXPECT().GetReferences().Return(refs, nil)
			rootHash := "e41db497eff0acf90c32a3a2560b76682a262fb4"
			mockRepo.EXPECT().GetRefRootCommit(refs[0].String()).Return(rootHash, nil)
			mockRepo.EXPECT().ReadPostBody(rootHash).Return(&plumbing.PostBody{}, nil, nil)
			mockRepo.EXPECT().RefGet(refs[0].String()).Return("", fmt.Errorf("error here"))
			_, err := plumbing.GetPosts(mockRepo, func(ref plumbing2.ReferenceName) bool { return true })
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error here"))
		})

		It("should return error when unable to read issue reference", func() {
			refs := []plumbing2.ReferenceName{plumbing2.ReferenceName("refs/heads/issues/1")}
			mockRepo.EXPECT().GetReferences().Return(refs, nil)
			rootHash := "e41db497eff0acf90c32a3a2560b76682a262fb4"
			mockRepo.EXPECT().GetRefRootCommit(refs[0].String()).Return(rootHash, nil)
			mockRepo.EXPECT().ReadPostBody(rootHash).Return(&plumbing.PostBody{}, nil, nil)
			recentHash := "a62s6d736acf90c32a3a2560b76682a262fb4"
			mockRepo.EXPECT().RefGet(refs[0].String()).Return(recentHash, nil)
			mockRepo.EXPECT().ReadPostBody(recentHash).Return(nil, nil, fmt.Errorf("error here"))
			_, err := plumbing.GetPosts(mockRepo, func(ref plumbing2.ReferenceName) bool { return true })
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error here"))
		})

		It("should return empty slice when no post reference is found by filter", func() {
			refs := []plumbing2.ReferenceName{plumbing2.ReferenceName("refs/heads/issues/1")}
			mockRepo.EXPECT().GetReferences().Return(refs, nil)
			post, err := plumbing.GetPosts(mockRepo, func(ref plumbing2.ReferenceName) bool { return false })
			Expect(err).To(BeNil())
			Expect(post).To(BeEmpty())
		})

		It("should return 1 post when a post reference is found by filter", func() {
			testutil2.AppendCommit(path, "file.txt", "some text 1", "commit 1")
			testutil2.CreateCheckoutOrphanBranch(path, "issues/1")
			testutil2.AppendCommit(path, "body", "some text 1", "commit 1")
			posts, err := plumbing.GetPosts(testRepo, func(ref plumbing2.ReferenceName) bool {
				return strings.Contains(ref.String(), plumbing.IssueBranchPrefix)
			})
			Expect(err).To(BeNil())
			Expect(posts).To(HaveLen(1))
		})

		It("should return err when a post reference does not include body file", func() {
			testutil2.AppendCommit(path, "file.txt", "some text 1", "commit 1")
			testutil2.CreateCheckoutOrphanBranch(path, "issues/1")
			testutil2.AppendCommit(path, "some_file", "some text 1", "commit 1")
			_, err := plumbing.GetPosts(testRepo, func(ref plumbing2.ReferenceName) bool {
				return strings.Contains(ref.String(), plumbing.IssueBranchPrefix)
			})
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(MatchRegexp("body file of commit (.*) is missing"))
		})

		It("should return err when a post reference does not include body file", func() {
			testutil2.AppendCommit(path, "file.txt", "some text 1", "commit 1")
			testutil2.CreateCheckoutOrphanBranch(path, "issues/1")
			testutil2.AppendCommit(path, "body", "---\nbad body: {{}123\n---", "commit 2")
			_, err := plumbing.GetPosts(testRepo, func(ref plumbing2.ReferenceName) bool {
				return strings.Contains(ref.String(), plumbing.IssueBranchPrefix)
			})
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(MatchRegexp("commit (.*) has bad body file: failed to unmarshal YAML.*"))
		})
	})

	Describe(".GetComments", func() {

		It("should return error when unable to query comment commits", func() {
			var post = &plumbing.Post{Repo: mockRepo, Name: plumbing.MakeIssueReference(1)}
			mockRepo.EXPECT().GetRefCommits(post.Name, true).Return(nil, fmt.Errorf("error"))
			_, err := post.GetComments()
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})

		It("should return error when unable to read a comment commit", func() {
			var post = &plumbing.Post{Repo: mockRepo, Name: plumbing.MakeIssueReference(1)}
			var commentHashes = []string{"e41db497eff0acf90c32a3a2560b76682a262fb4", "ce6fe17ea12b1c313c4868b4320cee41b9e20c07"}
			mockRepo.EXPECT().GetRefCommits(post.Name, true).Return(commentHashes, nil)
			mockRepo.EXPECT().CommitObject(plumbing2.NewHash(commentHashes[0])).Return(nil, fmt.Errorf("error"))
			_, err := post.GetComments()
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("unable to read commit (" + commentHashes[0] + ")"))
		})

		It("should return error when commit body file cannot be read", func() {
			testutil2.CreateCheckoutOrphanBranch(path, "issues/1")
			testutil2.AppendCommit(path, "useless_file", "---\n\n---\nthe content", "commit 1")
			var post = &plumbing.Post{Repo: testRepo, Name: plumbing.MakeIssueReference(1)}
			_, err := post.GetComments()
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(MatchRegexp("body file of commit (.*) is missing"))
		})

		It("should return error when commit body content cannot be parsed", func() {
			testutil2.CreateCheckoutOrphanBranch(path, "issues/1")
			testutil2.AppendCommit(path, "body", "---\nfield: {bad value}:a\n---", "commit 1")
			var post = &plumbing.Post{Repo: testRepo, Name: plumbing.MakeIssueReference(1)}
			_, err := post.GetComments()
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(MatchRegexp("commit (.*) has bad body file.*"))
		})

		It("should return error when attempt to expand ReplyTo hash fails", func() {
			testutil2.CreateCheckoutOrphanBranch(path, "issues/1")
			testutil2.AppendCommit(path, "body", "", "commit 1")
			testutil2.AppendCommit(path, "body", "---\nreplyTo: bad_short_hash\n---\ncontent", "commit 1")
			var post = &plumbing.Post{Repo: testRepo, Name: plumbing.MakeIssueReference(1)}
			_, err := post.GetComments()
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(MatchRegexp("commit (.*) reply hash could not be expanded"))
		})

		It("should expand ReplyTo hash if it is short", func() {
			testutil2.CreateCheckoutOrphanBranch(path, "issues/1")
			testutil2.AppendCommit(path, "body", "", "commit 1")
			recentHash := testutil2.GetRecentCommitHash(path, "issues/1")
			testutil2.AppendCommit(path, "body", "---\nreplyTo: "+recentHash[:7]+"\n---\ncontent", "commit 1")
			var post = &plumbing.Post{Repo: testRepo, Name: plumbing.MakeIssueReference(1)}
			comments, err := post.GetComments()
			Expect(err).To(BeNil())
			Expect(comments[0].Body.ReplyTo).To(Equal(recentHash))
		})

		It("should return one comment when issue contains only one commit", func() {
			testutil2.CreateCheckoutOrphanBranch(path, "issues/1")
			testutil2.AppendCommit(path, "body", "---\n\n---\nthe content", "commit 1")
			posts, err := plumbing.GetPosts(testRepo, func(ref plumbing2.ReferenceName) bool {
				return strings.Contains(ref.String(), plumbing.IssueBranchPrefix)
			})
			Expect(err).To(BeNil())
			Expect(posts[0].GetComments()).To(HaveLen(1))
		})

		It("should return two comments when issue contains two commits", func() {
			testutil2.CreateCheckoutOrphanBranch(path, "issues/1")
			testutil2.AppendCommit(path, "body", "---\n\n---\nthe content", "commit 1")
			testutil2.AppendCommit(path, "body", "---\n\n---\ncontent updated", "commit 2")
			posts, err := plumbing.GetPosts(testRepo, func(ref plumbing2.ReferenceName) bool {
				return strings.Contains(ref.String(), plumbing.IssueBranchPrefix)
			})
			Expect(err).To(BeNil())
			Expect(posts[0].GetComments()).To(HaveLen(2))
		})

		It("should return process reaction when a commit comment is a reply", func() {
			testutil2.CreateCheckoutOrphanBranch(path, "issues/1")
			testutil2.AppendCommit(path, "body", "", "commit 1")
			recentHash := testutil2.GetRecentCommitHash(path, "issues/1")
			testutil2.AppendCommit(path, "body", `---
replyTo: `+recentHash+`
reactions: ["smile","anger"]
---
content`, "commit 2")

			posts, err := plumbing.GetPosts(testRepo, func(ref plumbing2.ReferenceName) bool {
				return strings.Contains(ref.String(), plumbing.IssueBranchPrefix)
			})
			Expect(err).To(BeNil())
			Expect(posts).To(HaveLen(1))
			comments, err := posts[0].GetComments()
			Expect(err).To(BeNil())
			Expect(comments).To(HaveLen(2))
			Expect(comments[1].GetReactions()).To(Equal(map[string]int{"smile": 1, "anger": 1}))
		})
	})

	Describe(".GetCommentPreview", func() {
		It("should return short version (with ellipsis) when content length is above 80", func() {
			text := `Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.`
			prev := plumbing.GetCommentPreview(&plumbing.Comment{Body: &plumbing.PostBody{Content: []byte(text)}})
			Expect(strings.TrimSpace(prev)).To(Equal("Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor i..."))
			Expect(len(strings.TrimSpace(prev))).To(Equal(80 + 3))
		})

		It("should return full version when content length is below/equal 80", func() {
			text := `Lorem ipsum dolor sit amet, consectetur adipiscing elit`
			prev := plumbing.GetCommentPreview(&plumbing.Comment{Body: &plumbing.PostBody{Content: []byte(text)}})
			Expect(strings.TrimSpace(prev)).To(Equal(text))
		})
	})

	Describe(".PostBodyFromContentFrontMatter", func() {
		It("case 1", func() {
			issue := plumbing.PostBodyFromContentFrontMatter(&pageparser.ContentFrontMatter{
				Content: []byte("content"),
				FrontMatter: map[string]interface{}{
					"title":     "My Title",
					"replyTo":   "12345",
					"reactions": []interface{}{"smile"},
					"labels":    []interface{}{"help"},
					"assignees": []interface{}{"pk1abc"},
					"close":     true,
				},
				FrontMatterFormat: "",
			})

			Expect(issue.Content).To(Equal([]byte("content")))
			Expect(issue.Title).To(Equal("My Title"))
			Expect(*issue.Close).To(BeTrue())
			Expect(issue.ReplyTo).To(Equal("12345"))
			Expect(issue.Reactions).To(Equal([]string{"smile"}))
			Expect(issue.Labels).To(Equal([]string{"help"}))
			Expect(issue.Assignees).To(Equal([]string{"pk1abc"}))
		})

		It("case 2 - when close, labels, assignees are unset, it should be nil", func() {
			issue := plumbing.PostBodyFromContentFrontMatter(&pageparser.ContentFrontMatter{
				Content: []byte("content"), FrontMatter: map[string]interface{}{},
			})
			Expect(issue.Close).To(BeNil())
			Expect(issue.Assignees).To(BeNil())
			Expect(issue.Labels).To(BeNil())
		})
	})

	Describe(".PostBodyToString", func() {
		It("should set only 'title' when only 'title' is set", func() {
			body := &plumbing.PostBody{Title: "my title"}
			str := plumbing.PostBodyToString(body)
			Expect(str).To(Equal("---\ntitle: my title\n---\n"))
		})

		It("should set only 'content' when only 'content' is set", func() {
			body := &plumbing.PostBody{Content: []byte("xyz")}
			str := plumbing.PostBodyToString(body)
			Expect(str).To(Equal("xyz"))
		})

		It("should set only 'replyTo' when only 'replyTo' is set", func() {
			body := &plumbing.PostBody{ReplyTo: "0x123"}
			str := plumbing.PostBodyToString(body)
			Expect(str).To(Equal("---\nreplyTo: \"0x123\"\n---\n"))
		})

		It("should not set 'reactions' when empty", func() {
			body := &plumbing.PostBody{Reactions: []string{}}
			str := plumbing.PostBodyToString(body)
			Expect(str).To(Equal(""))
		})

		It("should set only 'reactions' when only 'reactions' is set", func() {
			body := &plumbing.PostBody{Reactions: []string{"smile"}}
			str := plumbing.PostBodyToString(body)
			Expect(str).To(Equal("---\nreactions: [smile]\n---\n"))
		})

		It("should not set 'close' when it is nil", func() {
			body := &plumbing.PostBody{Close: nil}
			str := plumbing.PostBodyToString(body)
			Expect(str).To(Equal(""))
		})

		It("should set 'close' when it is false/true", func() {
			cls := false
			body := &plumbing.PostBody{Close: &cls}
			str := plumbing.PostBodyToString(body)
			Expect(str).To(Equal("---\nclose: false\n---\n"))
			cls = true
			body = &plumbing.PostBody{Close: &cls}
			str = plumbing.PostBodyToString(body)
			Expect(str).To(Equal("---\nclose: true\n---\n"))
		})

		It("should not set 'label' when it is nil", func() {
			body := &plumbing.PostBody{Close: nil,
				IssueFields:        &plumbing.IssueFields{Labels: nil},
				MergeRequestFields: &plumbing.MergeRequestFields{},
			}
			str := plumbing.PostBodyToString(body)
			Expect(str).To(Equal(""))
		})

		It("should set 'label' when it is not nil", func() {
			var lbls []string
			body := &plumbing.PostBody{Close: nil, IssueFields: &plumbing.IssueFields{Labels: lbls}}
			str := plumbing.PostBodyToString(body)
			Expect(str).To(Equal(""))
		})
	})

	Describe("PostBody.IncludesAdminFields", func() {
		It("should return true when labels, assignees and close are set", func() {
			Expect((&plumbing.PostBody{IssueFields: &plumbing.IssueFields{Labels: []string{"val"}}}).IncludesAdminFields()).To(BeTrue())
			Expect((&plumbing.PostBody{IssueFields: &plumbing.IssueFields{Assignees: []string{"val"}}}).IncludesAdminFields()).To(BeTrue())
			Expect((&plumbing.PostBody{}).IncludesAdminFields()).To(BeFalse())
			Expect((&plumbing.PostBody{Close: &cls}).IncludesAdminFields()).To(BeTrue())
			Expect((&plumbing.PostBody{Close: &dontClose}).IncludesAdminFields()).To(BeTrue())
			Expect((&plumbing.PostBody{MergeRequestFields: &plumbing.MergeRequestFields{}}).IncludesAdminFields()).To(BeFalse())
			Expect((&plumbing.PostBody{MergeRequestFields: &plumbing.MergeRequestFields{BaseBranch: "base1"}}).IncludesAdminFields()).To(BeTrue())
		})
	})

	Describe(".GetReactionsForComment", func() {
		It("case 1", func() {
			reactions := plumbing.ReactionMap{
				"hash1": map[string]map[string]int{
					"smile": {"pk1": 1, "push2": 1},
					"anger": {"push2": 1},
				},
			}
			res := plumbing.GetReactionsForComment(reactions, "hash1")
			Expect(res).To(Equal(map[string]int{"smile": 2, "anger": 1}))
		})

		It("should return zero (0) for a reaction whose total count is below zero", func() {
			reactions := plumbing.ReactionMap{
				"hash1": map[string]map[string]int{
					"smile": {"pk1": -2, "push2": 1},
					"anger": {"push2": 1},
				},
			}
			res := plumbing.GetReactionsForComment(reactions, "hash1")
			Expect(res).To(Equal(map[string]int{"smile": 0, "anger": 1}))
		})
	})

	Describe(".UpdateReactions", func() {
		It("case 1", func() {
			dst := map[string]map[string]map[string]int{}
			plumbing.UpdateReactions([]string{"smile", "anger"}, "target1", "pusher1", dst)
			Expect(dst).To(HaveKey("target1"))
			Expect(dst["target1"]).To(HaveKey("smile"))
			Expect(dst["target1"]).To(HaveKey("anger"))
			Expect(dst["target1"]["smile"]).To(HaveKey("pusher1"))
			Expect(dst["target1"]["anger"]).To(HaveKey("pusher1"))
			Expect(dst["target1"]["smile"]["pusher1"]).To(Equal(1))
			Expect(dst["target1"]["anger"]["pusher1"]).To(Equal(1))
		})

		It("case 2", func() {
			dst := map[string]map[string]map[string]int{}
			plumbing.UpdateReactions([]string{"smile", "anger"}, "target1", "pusher1", dst)
			plumbing.UpdateReactions([]string{"smile", "anger"}, "target1", "pusher1", dst)
			Expect(dst).To(HaveKey("target1"))
			Expect(dst["target1"]).To(HaveKey("smile"))
			Expect(dst["target1"]).To(HaveKey("anger"))
			Expect(dst["target1"]["smile"]).To(HaveKey("pusher1"))
			Expect(dst["target1"]["anger"]).To(HaveKey("pusher1"))
			Expect(dst["target1"]["smile"]["pusher1"]).To(Equal(2))
			Expect(dst["target1"]["anger"]["pusher1"]).To(Equal(2))
		})

		It("case 3", func() {
			dst := map[string]map[string]map[string]int{}
			plumbing.UpdateReactions([]string{"smile", "anger"}, "target1", "pusher1", dst)
			plumbing.UpdateReactions([]string{"smile"}, "target1", "pusher1", dst)
			Expect(dst).To(HaveKey("target1"))
			Expect(dst["target1"]).To(HaveKey("smile"))
			Expect(dst["target1"]).To(HaveKey("anger"))
			Expect(dst["target1"]["smile"]).To(HaveKey("pusher1"))
			Expect(dst["target1"]["anger"]).To(HaveKey("pusher1"))
			Expect(dst["target1"]["smile"]["pusher1"]).To(Equal(2))
			Expect(dst["target1"]["anger"]["pusher1"]).To(Equal(1))
		})

		It("case 4 - negation test", func() {
			dst := map[string]map[string]map[string]int{}
			plumbing.UpdateReactions([]string{"smile", "anger"}, "target1", "pusher1", dst)
			plumbing.UpdateReactions([]string{"smile", "anger"}, "target1", "pusher1", dst)
			plumbing.UpdateReactions([]string{"-anger"}, "target1", "pusher1", dst)
			Expect(dst).To(HaveKey("target1"))
			Expect(dst["target1"]).To(HaveKey("smile"))
			Expect(dst["target1"]).To(HaveKey("anger"))
			Expect(dst["target1"]["smile"]).To(HaveKey("pusher1"))
			Expect(dst["target1"]["anger"]).To(HaveKey("pusher1"))
			Expect(dst["target1"]["smile"]["pusher1"]).To(Equal(2))
			Expect(dst["target1"]["anger"]["pusher1"]).To(Equal(1))
		})

		It("case 5 - negation test", func() {
			dst := map[string]map[string]map[string]int{}
			plumbing.UpdateReactions([]string{"smile", "anger"}, "target1", "pusher1", dst)
			plumbing.UpdateReactions([]string{"smile", "anger"}, "target1", "pusher1", dst)
			plumbing.UpdateReactions([]string{"-anger"}, "target1", "pusher1", dst)
			plumbing.UpdateReactions([]string{"-anger"}, "target1", "pusher1", dst)
			Expect(dst).To(HaveKey("target1"))
			Expect(dst["target1"]).To(HaveKey("smile"))
			Expect(dst["target1"]).To(HaveKey("anger"))
			Expect(dst["target1"]["smile"]).To(HaveKey("pusher1"))
			Expect(dst["target1"]["anger"]).To(HaveKey("pusher1"))
			Expect(dst["target1"]["smile"]["pusher1"]).To(Equal(2))
			Expect(dst["target1"]["anger"]["pusher1"]).To(Equal(0))
		})

		It("case 6 - negation test", func() {
			dst := map[string]map[string]map[string]int{}
			plumbing.UpdateReactions([]string{"smile", "anger"}, "target1", "pusher1", dst)
			plumbing.UpdateReactions([]string{"smile", "anger"}, "target1", "pusher1", dst)
			plumbing.UpdateReactions([]string{"-anger"}, "target1", "pusher1", dst)
			plumbing.UpdateReactions([]string{"-anger"}, "target1", "pusher1", dst)
			plumbing.UpdateReactions([]string{"-anger"}, "target1", "pusher1", dst)
			Expect(dst).To(HaveKey("target1"))
			Expect(dst["target1"]).To(HaveKey("smile"))
			Expect(dst["target1"]).To(HaveKey("anger"))
			Expect(dst["target1"]["smile"]).To(HaveKey("pusher1"))
			Expect(dst["target1"]["anger"]).To(HaveKey("pusher1"))
			Expect(dst["target1"]["smile"]["pusher1"]).To(Equal(2))
			Expect(dst["target1"]["anger"]["pusher1"]).To(Equal(-1))
		})

		It("case 7 - negation test", func() {
			dst := map[string]map[string]map[string]int{}
			plumbing.UpdateReactions([]string{"-anger"}, "target1", "pusher1", dst)
			plumbing.UpdateReactions([]string{"-anger"}, "target1", "pusher1", dst)
			plumbing.UpdateReactions([]string{"-anger"}, "target1", "pusher1", dst)
			plumbing.UpdateReactions([]string{"anger"}, "target1", "pusher1", dst)
			plumbing.UpdateReactions([]string{"smile"}, "target1", "pusher1", dst)
			plumbing.UpdateReactions([]string{"smile"}, "target1", "pusher1", dst)
			plumbing.UpdateReactions([]string{"smile"}, "target1", "pusher1", dst)
			plumbing.UpdateReactions([]string{"anger"}, "target1", "pusher1", dst)
			Expect(dst).To(HaveKey("target1"))
			Expect(dst["target1"]).To(HaveKey("smile"))
			Expect(dst["target1"]).To(HaveKey("anger"))
			Expect(dst["target1"]["smile"]).To(HaveKey("pusher1"))
			Expect(dst["target1"]["anger"]).To(HaveKey("pusher1"))
			Expect(dst["target1"]["smile"]["pusher1"]).To(Equal(3))
			Expect(dst["target1"]["anger"]["pusher1"]).To(Equal(-1))
		})
	})

	Describe(".CreatePostCommit", func() {
		It("should return error when unable to check if repo worktree is clean", func() {
			mockRepo.EXPECT().IsClean().Return(false, fmt.Errorf("error"))
			_, _, err := plumbing.CreatePostCommit(mockRepo, &plumbing.CreatePostCommitArgs{})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to check repo status: error"))
		})

		It("should return error when repo worktree is not clean", func() {
			mockRepo.EXPECT().IsClean().Return(false, nil)
			_, _, err := plumbing.CreatePostCommit(mockRepo, &plumbing.CreatePostCommitArgs{})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("dirty working tree; there are uncommitted changes"))
		})

		When("post ref number is not provided", func() {
			It("should return err when unable to get free post number", func() {
				mockRepo.EXPECT().IsClean().Return(true, nil)
				args := &plumbing.CreatePostCommitArgs{Type: plumbing.IssueBranchPrefix, ID: 0, Body: "", IsComment: false,
					GetFreePostID: func(repo plumbing.LocalRepo, startID int, postRefType string) (int, error) {
						return 0, fmt.Errorf("error")
					},
				}
				_, _, err := plumbing.CreatePostCommit(mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("failed to find free post number: error"))
			})
		})

		When("args.Force=true", func() {
			It("should return err when reference type is unknown", func() {
				args := &plumbing.CreatePostCommitArgs{Type: "unknown", Force: true, ID: 1, Body: "", IsComment: false,
					GetFreePostID: func(repo plumbing.LocalRepo, startID int, postRefType string) (int, error) {
						return 0, fmt.Errorf("error")
					},
				}
				_, _, err := plumbing.CreatePostCommit(mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("unknown post reference type"))
			})

			It("should return error when unable to query post reference", func() {
				args := &plumbing.CreatePostCommitArgs{Type: plumbing.IssueBranchPrefix, Force: true, ID: 0, Body: "", IsComment: false,
					GetFreePostID: func(repo plumbing.LocalRepo, startID int, postRefType string) (int, error) { return 1, nil },
				}
				mockRepo.EXPECT().RefGet(plumbing.MakeIssueReference(1)).Return("", fmt.Errorf("error"))
				_, _, err := plumbing.CreatePostCommit(mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("failed to check post reference existence: error"))
			})

			When("comment is requested but issue does not exist", func() {
				It("should return err", func() {
					args := &plumbing.CreatePostCommitArgs{
						Type: plumbing.IssueBranchPrefix, ID: 0,
						Force: true,
						Body:  "", IsComment: true,
						GetFreePostID: func(repo plumbing.LocalRepo, startID int, postRefType string) (int, error) { return 1, nil },
					}
					mockRepo.EXPECT().RefGet(plumbing.MakeIssueReference(1)).Return("", plumbing.ErrRefNotFound)
					_, _, err := plumbing.CreatePostCommit(mockRepo, args)
					Expect(err).ToNot(BeNil())
					Expect(err).To(MatchError("can't add comment to a non-existing post"))
				})
			})

			It("should return err when unable to create a single file commit", func() {
				args := &plumbing.CreatePostCommitArgs{Type: plumbing.IssueBranchPrefix, Force: true, ID: 0,
					Body: "body content", IsComment: false,
					GetFreePostID: func(repo plumbing.LocalRepo, startID int, postRefType string) (int, error) { return 1, nil },
				}
				hash := util.RandString(40)
				mockRepo.EXPECT().RefGet(plumbing.MakeIssueReference(1)).Return(hash, nil)
				mockRepo.EXPECT().CreateSingleFileCommit("body", "body content", "", hash).Return("", fmt.Errorf("error"))
				_, _, err := plumbing.CreatePostCommit(mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("failed to create post commit: error"))
			})

			It("should return err when unable to update issue reference target hash", func() {
				refname := plumbing.MakeMergeRequestReference(1)
				hash := util.RandString(40)
				issueHash := util.RandString(40)
				mockRepo.EXPECT().RefGet(refname).Return(hash, nil)
				mockRepo.EXPECT().CreateSingleFileCommit("body", "body content", "", hash).Return(issueHash, nil)
				mockRepo.EXPECT().RefUpdate(refname, issueHash).Return(fmt.Errorf("error"))

				args := &plumbing.CreatePostCommitArgs{Type: plumbing.MergeRequestBranchPrefix, Force: true, ID: 0,
					Body: "body content", IsComment: false,
					GetFreePostID: func(repo plumbing.LocalRepo, startID int, postRefType string) (int, error) { return 1, nil },
				}
				_, _, err := plumbing.CreatePostCommit(mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("failed to update post reference target hash: error"))
			})

			It("should return error when unable to get repo HEAD", func() {
				refname := plumbing.MakeIssueReference(1)
				hash := util.RandString(40)
				issueHash := util.RandString(40)
				mockRepo.EXPECT().RefGet(refname).Return(hash, nil)
				mockRepo.EXPECT().CreateSingleFileCommit("body", "body content", "", hash).Return(issueHash, nil)
				mockRepo.EXPECT().RefUpdate(refname, issueHash).Return(nil)
				mockRepo.EXPECT().Head().Return("", fmt.Errorf("error"))

				args := &plumbing.CreatePostCommitArgs{Type: plumbing.IssueBranchPrefix, Force: true, ID: 0,
					Body: "body content", IsComment: false,
					GetFreePostID: func(repo plumbing.LocalRepo, startID int, postRefType string) (int, error) { return 1, nil },
				}
				_, _, err := plumbing.CreatePostCommit(mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("failed to get HEAD: error"))
			})

			When("HEAD and reference match", func() {
				It("should return error when unable to checkout reference", func() {
					refname := plumbing.MakeIssueReference(1)
					hash := util.RandString(40)
					issueHash := util.RandString(40)
					mockRepo.EXPECT().RefGet(refname).Return(hash, nil)
					mockRepo.EXPECT().CreateSingleFileCommit("body", "body content", "", hash).Return(issueHash, nil)
					mockRepo.EXPECT().RefUpdate(refname, issueHash).Return(nil)
					mockRepo.EXPECT().Head().Return(refname, nil)

					args := &plumbing.CreatePostCommitArgs{Type: plumbing.IssueBranchPrefix, Force: true, ID: 0,
						Body: "body content", IsComment: false,
						GetFreePostID: func(repo plumbing.LocalRepo, startID int, postRefType string) (int, error) { return 1, nil },
					}
					mockRepo.EXPECT().Checkout(plumbing2.ReferenceName(refname).Short(), false, args.Force).Return(fmt.Errorf("error"))

					_, _, err := plumbing.CreatePostCommit(mockRepo, args)
					Expect(err).ToNot(BeNil())
					Expect(err).To(MatchError("failed to checkout post reference: error"))
				})

				It("should return no error checkout succeeds", func() {
					refname := plumbing.MakeIssueReference(1)
					hash := util.RandString(40)
					issueHash := util.RandString(40)
					mockRepo.EXPECT().RefGet(refname).Return(hash, nil)
					mockRepo.EXPECT().CreateSingleFileCommit("body", "body content", "", hash).Return(issueHash, nil)
					mockRepo.EXPECT().RefUpdate(refname, issueHash).Return(nil)
					mockRepo.EXPECT().Head().Return(refname, nil)

					args := &plumbing.CreatePostCommitArgs{Type: plumbing.IssueBranchPrefix, Force: true, ID: 0,
						Body: "body content", IsComment: false,
						GetFreePostID: func(repo plumbing.LocalRepo, startID int, postRefType string) (int, error) { return 1, nil },
					}
					mockRepo.EXPECT().Checkout(plumbing2.ReferenceName(refname).Short(), false, args.Force).Return(nil)

					_, _, err := plumbing.CreatePostCommit(mockRepo, args)
					Expect(err).To(BeNil())
				})
			})
		})
	})
})
