package plumbing_test

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/gohugoio/hugo/parser/pageparser"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/remote/plumbing"
	repo2 "gitlab.com/makeos/mosdef/remote/repo"
	testutil2 "gitlab.com/makeos/mosdef/remote/testutil"
	"gitlab.com/makeos/mosdef/testutil"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/util"
	plumbing2 "gopkg.in/src-d/go-git.v4/plumbing"
)

var _ = Describe("Post", func() {
	var err error
	var cfg *config.AppConfig
	var repo core.BareRepo
	var repoName, path string

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"

		repoName = util.RandString(5)
		path = filepath.Join(cfg.GetRepoRoot(), repoName)
		testutil2.ExecGit(cfg.GetRepoRoot(), "init", repoName)

		repo, err = repo2.GetRepoWithLiteGit(cfg.Node.GitBinPath, path)
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".GetPosts", func() {
		It("should return empty slice when no post reference is found by filter", func() {
			post, err := plumbing.GetPosts(repo, func(ref *plumbing2.Reference) bool { return false })
			Expect(err).To(BeNil())
			Expect(post).To(BeEmpty())
		})

		It("should return 1 post when a post reference is found by filter", func() {
			testutil2.AppendCommit(path, "file.txt", "some text 1", "commit 1")
			testutil2.CreateCheckoutOrphanBranch(path, "issues/1")
			testutil2.AppendCommit(path, "body", "some text 1", "commit 1")
			posts, err := plumbing.GetPosts(repo, func(ref *plumbing2.Reference) bool {
				return strings.Contains(ref.Name().String(), "issues")
			})
			Expect(err).To(BeNil())
			Expect(posts).To(HaveLen(1))
		})

		It("should return err when a post reference does not include body file", func() {
			testutil2.AppendCommit(path, "file.txt", "some text 1", "commit 1")
			testutil2.CreateCheckoutOrphanBranch(path, "issues/1")
			testutil2.AppendCommit(path, "some_file", "some text 1", "commit 1")
			_, err := plumbing.GetPosts(repo, func(ref *plumbing2.Reference) bool {
				return strings.Contains(ref.Name().String(), "issues")
			})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("body file is missing in refs/heads/issues/1"))
		})

		It("should return err when a post reference does not include body file", func() {
			testutil2.AppendCommit(path, "file.txt", "some text 1", "commit 1")
			testutil2.CreateCheckoutOrphanBranch(path, "issues/1")
			testutil2.AppendCommit(path, "body", "---\nbad body: {{}123\n---", "commit 2")
			_, err := plumbing.GetPosts(repo, func(ref *plumbing2.Reference) bool {
				return strings.Contains(ref.Name().String(), "issues")
			})
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("root commit of refs/heads/issues/1 has bad body file"))
		})
	})

	Describe(".GetCommentPreview", func() {
		It("should return sentence", func() {
			prev := plumbing.GetCommentPreview(&plumbing.Comment{Body: &plumbing.IssueBody{Content: []byte("This is a simulation. We are in a simulation.")}})
			Expect(strings.TrimSpace(prev)).To(Equal("This is a simulation..."))

			prev = plumbing.GetCommentPreview(&plumbing.Comment{Body: &plumbing.IssueBody{Content: []byte("This is a simulation.")}})
			Expect(strings.TrimSpace(prev)).To(Equal("This is a simulation."))
		})
	})

	Describe(".IssueBodyFromContentFrontMatter", func() {
		It("case 1", func() {
			issue := plumbing.IssueBodyFromContentFrontMatter(&pageparser.ContentFrontMatter{
				Content: []byte("content"),
				FrontMatter: map[string]interface{}{
					"title":     "My Title",
					"replyTo":   "12345",
					"reactions": []string{"smile"},
					"labels":    []string{"help"},
					"assignees": []string{"push1abc"},
					"fixers":    []string{"push1abc"},
					"close":     1,
				},
				FrontMatterFormat: "",
			})

			Expect(issue.Content).To(Equal([]byte("content")))
			Expect(issue.Title).To(Equal("My Title"))
			Expect(issue.Close).To(Equal(1))
			Expect(issue.ReplyTo).To(Equal("12345"))
			Expect(issue.Reactions).To(Equal([]string{"smile"}))
			Expect(issue.Labels).To(Equal([]string{"help"}))
			Expect(issue.Assignees).To(Equal([]string{"push1abc"}))
			Expect(issue.Fixers).To(Equal([]string{"push1abc"}))
		})
	})

	Describe(".IssueBodyToString", func() {
		It("cases", func() {
			body := &plumbing.IssueBody{Content: nil, Title: "my title", ReplyTo: "", Reactions: nil, Labels: nil, Assignees: nil, Fixers: nil, Close: -1}
			str := plumbing.IssueBodyToString(body)
			Expect(str).To(Equal("---\ntitle: my title\n---\n"))

			body = &plumbing.IssueBody{Content: []byte("my body"), Title: "my title", ReplyTo: "", Close: -1}
			str = plumbing.IssueBodyToString(body)
			Expect(str).To(Equal("---\ntitle: my title\n---\nmy body"))

			body = &plumbing.IssueBody{Content: []byte("my body"), Title: "my title", ReplyTo: "xyz", Close: -1}
			str = plumbing.IssueBodyToString(body)
			Expect(str).To(Equal("---\ntitle: my title\nreplyTo: xyz\n---\nmy body"))

			body = &plumbing.IssueBody{Content: []byte("my body"), Title: "my title", ReplyTo: "xyz", Labels: []string{"a", "b"}, Close: -1}
			str = plumbing.IssueBodyToString(body)
			Expect(str).To(Equal("---\ntitle: my title\nreplyTo: xyz\nlabels: [\"a\",\"b\"]\n---\nmy body"))

			body = &plumbing.IssueBody{Content: []byte("my body"), Title: "my title", ReplyTo: "xyz", Assignees: []string{"a", "b"}, Close: -1}
			str = plumbing.IssueBodyToString(body)
			Expect(str).To(Equal("---\ntitle: my title\nreplyTo: xyz\nassignees: [\"a\",\"b\"]\n---\nmy body"))

			body = &plumbing.IssueBody{Content: []byte("my body"), Title: "my title", ReplyTo: "xyz", Fixers: []string{"a", "b"}, Close: -1}
			str = plumbing.IssueBodyToString(body)
			Expect(str).To(Equal("---\ntitle: my title\nreplyTo: xyz\nfixers: [\"a\",\"b\"]\n---\nmy body"))

			body = &plumbing.IssueBody{Content: []byte("my body"), Title: "my title", ReplyTo: "xyz", Reactions: []string{"a", "b"}, Close: -1}
			str = plumbing.IssueBodyToString(body)
			Expect(str).To(Equal("---\ntitle: my title\nreplyTo: xyz\nreactions: [\"a\",\"b\"]\n---\nmy body"))

			body = &plumbing.IssueBody{Content: []byte("my body"), Title: "my title", ReplyTo: "xyz", Close: 1}
			str = plumbing.IssueBodyToString(body)
			Expect(str).To(Equal("---\ntitle: my title\nreplyTo: xyz\nclose: 1\n---\nmy body"))
		})
	})

})
