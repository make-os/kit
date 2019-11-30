package repo

import (
	"crypto/rsa"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bitfield/script"
	"golang.org/x/crypto/openpgp"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/testutil"
	"github.com/makeos/mosdef/util"
)

var _ = Describe("Validation", func() {
	var err error
	var cfg *config.EngineConfig
	var repo *Repo
	var path string
	var gpgKeyID string
	var pubKey string
	var privKey *openpgp.Entity
	var privKey2 *openpgp.Entity

	var gpgPubKeyGetter = func(pkId string) (string, error) {
		return pubKey, nil
	}

	var gpgInvalidPubKeyGetter = func(pkId string) (string, error) {
		return "invalid key", nil
	}

	var gpgPubKeyGetterWithErr = func(err error) func(pkId string) (string, error) {
		return func(pkId string) (string, error) {
			return "", err
		}
	}

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"
		gpgKeyID = testutil.CreateGPGKey(testutil.GPGProgramPath, cfg.DataDir())
		gpgKeyID2 := testutil.CreateGPGKey(testutil.GPGProgramPath, cfg.DataDir())
		pubKey, err = crypto.GetGPGPublicKeyStr(gpgKeyID, testutil.GPGProgramPath, cfg.DataDir())
		privKey, err = crypto.GetGPGPrivateKey(gpgKeyID, testutil.GPGProgramPath, cfg.DataDir())
		privKey2, err = crypto.GetGPGPrivateKey(gpgKeyID2, testutil.GPGProgramPath, cfg.DataDir())
		Expect(err).To(BeNil())
		GitEnv = append(GitEnv, "GNUPGHOME="+cfg.DataDir())
	})

	BeforeEach(func() {
		repoName := util.RandString(5)
		path = filepath.Join(cfg.GetRepoRoot(), repoName)
		execGit(cfg.GetRepoRoot(), "init", repoName)
		repo, err = getRepoWithGitOpt(cfg.Node.GitBinPath, path)
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".checkCommit", func() {
		var cob *object.Commit
		var err error

		When("txline is not set", func() {
			BeforeEach(func() {
				appendCommit(path, "file.txt", "line 1", "commit 1")
				commitHash, _ := script.ExecInDir(`git --no-pager log --oneline -1 --pretty=%H`,
					path).String()
				cob, _ = repo.CommitObject(plumbing.NewHash(strings.TrimSpace(commitHash)))
				_, err = checkCommit(cob, false, repo, gpgPubKeyGetter)
			})

			It("should return err='txline was not set'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("txline was not set"))
			})
		})

		When("txline.pkId is not valid", func() {
			BeforeEach(func() {
				txLine := fmt.Sprintf("tx: fee=%s, nonce=%s, pkId=%s", "0", "0", "invalid_pk_id")
				appendCommit(path, "file.txt", "line 1", txLine)
				commitHash, _ := script.ExecInDir(`git --no-pager log --oneline -1 --pretty=%H`,
					path).String()
				cob, _ = repo.CommitObject(plumbing.NewHash(strings.TrimSpace(commitHash)))
				_, err = checkCommit(cob, false, repo, gpgPubKeyGetter)
			})

			It("should return err='public key id is invalid'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("public key id is invalid"))
			})
		})

		When("commit is not signed", func() {
			BeforeEach(func() {
				pkEntity, _ := crypto.PGPEntityFromPubKey(pubKey)
				pkID := util.RSAPubKeyID(pkEntity.PrimaryKey.PublicKey.(*rsa.PublicKey))
				txLine := fmt.Sprintf("tx: fee=%s, nonce=%s, pkId=%s", "0", "0", pkID)
				appendCommit(path, "file.txt", "line 1", txLine)
				commitHash, _ := script.ExecInDir(`git --no-pager log --oneline -1 --pretty=%H`, path).String()
				cob, _ = repo.CommitObject(plumbing.NewHash(strings.TrimSpace(commitHash)))
				_, err = checkCommit(cob, false, repo, gpgPubKeyGetter)
			})

			It("should return err='is unsigned. please sign the commit with your gpg key'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("is unsigned. please sign the commit with your gpg key"))
			})
		})

		When("commit is signed but unable to get public key using the pkID", func() {
			BeforeEach(func() {
				pkEntity, _ := crypto.PGPEntityFromPubKey(pubKey)
				pkID := util.RSAPubKeyID(pkEntity.PrimaryKey.PublicKey.(*rsa.PublicKey))
				txLine := fmt.Sprintf("tx: fee=%s, nonce=%s, pkId=%s", "0", "0", pkID)
				appendCommit(path, "file.txt", "line 1", txLine)
				commitHash, _ := script.ExecInDir(`git --no-pager log --oneline -1 --pretty=%H`, path).String()
				cob, _ = repo.CommitObject(plumbing.NewHash(strings.TrimSpace(commitHash)))
				cob.PGPSignature = "signature"
				_, err = checkCommit(cob, false, repo, gpgPubKeyGetterWithErr(fmt.Errorf("bad error")))
			})

			It("should return err='..public key..was not found'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp(".*public key.*was not found"))
			})
		})

		When("commit has a signature but the signature is not valid", func() {
			BeforeEach(func() {
				pkEntity, _ := crypto.PGPEntityFromPubKey(pubKey)
				pkID := util.RSAPubKeyID(pkEntity.PrimaryKey.PublicKey.(*rsa.PublicKey))
				txLine := fmt.Sprintf("tx: fee=%s, nonce=%s, pkId=%s", "0", "0", pkID)
				appendCommit(path, "file.txt", "line 1", txLine)
				commitHash, _ := script.ExecInDir(`git --no-pager log --oneline -1 --pretty=%H`, path).String()
				cob, _ = repo.CommitObject(plumbing.NewHash(strings.TrimSpace(commitHash)))
				cob.PGPSignature = "signature"
				_, err = checkCommit(cob, false, repo, gpgPubKeyGetter)
			})

			It("should return err='..signature verification failed..'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("signature verification failed"))
			})
		})

		When("commit has a signature and the signature is valid", func() {
			BeforeEach(func() {
				pkEntity, _ := crypto.PGPEntityFromPubKey(pubKey)
				pkID := util.RSAPubKeyID(pkEntity.PrimaryKey.PublicKey.(*rsa.PublicKey))
				txLine := fmt.Sprintf("tx: fee=%s, nonce=%s, pkId=%s", "0", "0", pkID)
				appendSignedCommit(path, "file.txt", "line 1", txLine, gpgKeyID)
				commitHash, _ := script.ExecInDir(`git --no-pager log --oneline -1 --pretty=%H`, path).String()
				cob, _ = repo.CommitObject(plumbing.NewHash(strings.TrimSpace(commitHash)))
				_, err = checkCommit(cob, false, repo, gpgPubKeyGetter)
			})

			It("should return nil", func() {
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".checkAnnotatedTag", func() {
		var err error
		var tob *object.Tag

		When("txline is not set", func() {
			BeforeEach(func() {
				createCommitAndAnnotatedTag(path, "file.txt", "first file", "commit 1", "v1")
				tagRef, _ := repo.Tag("v1")
				tob, _ = repo.TagObject(tagRef.Hash())
				_, err = checkAnnotatedTag(tob, repo, gpgPubKeyGetter)
			})

			It("should return err='txline was not set'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("txline was not set"))
			})
		})

		When("txline.pkId is not valid", func() {
			BeforeEach(func() {
				txLine := fmt.Sprintf("tx: fee=%s, nonce=%s, pkId=%s", "0", "0", "invalid_pk_id")
				createCommitAndAnnotatedTag(path, "file.txt", "first file", txLine, "v1")
				tagRef, _ := repo.Tag("v1")
				tob, _ = repo.TagObject(tagRef.Hash())
				_, err = checkAnnotatedTag(tob, repo, gpgPubKeyGetter)
			})

			It("should return err='public key id is invalid'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("public key id is invalid"))
			})
		})

		When("tag is not signed", func() {
			BeforeEach(func() {
				pkEntity, _ := crypto.PGPEntityFromPubKey(pubKey)
				pkID := util.RSAPubKeyID(pkEntity.PrimaryKey.PublicKey.(*rsa.PublicKey))
				txLine := fmt.Sprintf("tx: fee=%s, nonce=%s, pkId=%s", "0", "0", pkID)
				createCommitAndAnnotatedTag(path, "file.txt", "first file", txLine, "v1")
				tagRef, _ := repo.Tag("v1")
				tob, _ = repo.TagObject(tagRef.Hash())
				_, err = checkAnnotatedTag(tob, repo, gpgPubKeyGetter)
			})

			It("should return err='is unsigned. please sign the tag with your gpg key'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("is unsigned. please sign the tag with your gpg key"))
			})
		})

		When("tag is signed but unable to get public key using the pkID", func() {
			BeforeEach(func() {
				pkEntity, _ := crypto.PGPEntityFromPubKey(pubKey)
				pkID := util.RSAPubKeyID(pkEntity.PrimaryKey.PublicKey.(*rsa.PublicKey))
				txLine := fmt.Sprintf("tx: fee=%s, nonce=%s, pkId=%s", "0", "0", pkID)
				createCommitAndAnnotatedTag(path, "file.txt", "first file", txLine, "v1")
				tagRef, _ := repo.Tag("v1")
				tob, _ = repo.TagObject(tagRef.Hash())
				tob.PGPSignature = "signature"
				_, err = checkAnnotatedTag(tob, repo, gpgPubKeyGetterWithErr(fmt.Errorf("bad error")))
			})

			It("should return err='..public key..was not found'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp(".*public key.*was not found"))
			})
		})

		When("tag has a signature but the signature is not valid", func() {
			BeforeEach(func() {
				pkEntity, _ := crypto.PGPEntityFromPubKey(pubKey)
				pkID := util.RSAPubKeyID(pkEntity.PrimaryKey.PublicKey.(*rsa.PublicKey))
				txLine := fmt.Sprintf("tx: fee=%s, nonce=%s, pkId=%s", "0", "0", pkID)
				createCommitAndAnnotatedTag(path, "file.txt", "first file", txLine, "v1")
				tagRef, _ := repo.Tag("v1")
				tob, _ = repo.TagObject(tagRef.Hash())
				tob.PGPSignature = "signature"
				_, err = checkAnnotatedTag(tob, repo, gpgPubKeyGetter)
			})

			It("should return err='..signature verification failed..'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("signature verification failed"))
			})
		})

		When("tag has a valid signature but the referenced commit is unsigned", func() {
			BeforeEach(func() {
				pkEntity, _ := crypto.PGPEntityFromPubKey(pubKey)
				pkID := util.RSAPubKeyID(pkEntity.PrimaryKey.PublicKey.(*rsa.PublicKey))
				txLine := fmt.Sprintf("tx: fee=%s, nonce=%s, pkId=%s", "0", "0", pkID)
				createCommitAndSignedAnnotatedTag(path, "file.txt", "first file", txLine, "v1", gpgKeyID)
				tagRef, _ := repo.Tag("v1")
				tob, _ = repo.TagObject(tagRef.Hash())
				_, err = checkAnnotatedTag(tob, repo, gpgPubKeyGetter)
			})

			It("should return err=''", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp(".*referenced commit.* is unsigned.*"))
			})
		})

		When("tag has a valid signature and the referenced commit is valid", func() {
			BeforeEach(func() {
				pkEntity, _ := crypto.PGPEntityFromPubKey(pubKey)
				pkID := util.RSAPubKeyID(pkEntity.PrimaryKey.PublicKey.(*rsa.PublicKey))
				txLine := fmt.Sprintf("tx: fee=%s, nonce=%s, pkId=%s", "0", "0", pkID)
				createSignedCommitAndSignedAnnotatedTag(path, "file.txt", "first file", txLine, "v1", gpgKeyID)
				tagRef, _ := repo.Tag("v1")
				tob, _ = repo.TagObject(tagRef.Hash())
				_, err = checkAnnotatedTag(tob, repo, gpgPubKeyGetter)
			})

			It("should return nil", func() {
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".checkNote", func() {
		var err error

		When("target note does not exist", func() {
			BeforeEach(func() {
				_, err = checkNote(repo, "unknown", gpgPubKeyGetter)
			})

			It("should return err='unable to fetch note entries (unknown)'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("unable to fetch note entries (unknown)"))
			})
		})

		When("a note does not have a tx blob object", func() {
			BeforeEach(func() {
				createCommitAndNote(path, "file.txt", "a file", "commit msg", "note1")
				_, err = checkNote(repo, "refs/notes/note1", gpgPubKeyGetter)
			})

			It("should return err='unacceptable note. it does not have a signed transaction object'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("unacceptable note. it does not have a signed transaction object"))
			})
		})

		When("a notes tx blob has invalid tx line", func() {
			BeforeEach(func() {
				createCommitAndNote(path, "file.txt", "a file", "commit msg", "note1")
				createNoteEntry(path, "note1", "tx:invalid")
				_, err = checkNote(repo, "refs/notes/note1", gpgPubKeyGetter)
			})

			It("should return err='note (refs/notes/note1): txline is malformed'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("note (refs/notes/note1): txline is malformed"))
			})
		})

		When("a notes tx blob has invalid signature format", func() {
			BeforeEach(func() {
				createCommitAndNote(path, "file.txt", "a file", "commit msg", "note1")
				createNoteEntry(path, "note1", "tx: fee=0 nonce=0 pkId=0x0000000000000000000000000000000000000000 sig=xyz")
				_, err = checkNote(repo, "refs/notes/note1", gpgPubKeyGetter)
			})

			It("should return err='note (refs/notes/note1): field:sig, msg: signature format is not valid'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("note (refs/notes/note1): field:sig, msg: signature format is not valid"))
			})
		})

		When("a notes tx blob has an unknown public key id", func() {
			BeforeEach(func() {
				createCommitAndNote(path, "file.txt", "a file", "commit msg", "note1")
				createNoteEntry(path, "note1", "tx: fee=0 nonce=0 pkId=0x0000000000000000000000000000000000000000 sig=0x616263")
				_, err = checkNote(repo, "refs/notes/note1", gpgPubKeyGetterWithErr(fmt.Errorf("error finding pub key")))
			})

			It("should return err='unable to verify note (refs/notes/note1). public key was not found.'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("unable to verify note (refs/notes/note1). public key was not found"))
			})
		})

		When("a notes tx blob includes a public key id to an invalid public key", func() {
			BeforeEach(func() {
				createCommitAndNote(path, "file.txt", "a file", "commit msg", "note1")
				createNoteEntry(path, "note1", "tx: fee=0 nonce=0 pkId=0x0000000000000000000000000000000000000000 sig=0x616263")
				_, err = checkNote(repo, "refs/notes/note1", gpgInvalidPubKeyGetter)
			})

			It("should return err='unable to verify note (refs/notes/note1). public key .. was not found.'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("unable to verify note (refs/notes/note1). public key is not valid"))
			})
		})

		When("a note's signature is not signed with an expected private key", func() {
			var sig []byte
			BeforeEach(func() {
				createCommitAndNote(path, "file.txt", "a file", "commit msg", "note1")
				commitHash := getRecentCommitHash(path, "refs/notes/note1")
				msg := []byte("0" + "0" + "0x0000000000000000000000000000000000000000" + commitHash)
				sig, err = crypto.GPGSign(privKey2, msg)
				Expect(err).To(BeNil())
				sigHex := hex.EncodeToString(sig)
				createNoteEntry(path, "note1", "tx: fee=0 nonce=0 pkId=0x0000000000000000000000000000000000000000 sig=0x"+sigHex)
				_, err = checkNote(repo, "refs/notes/note1", gpgPubKeyGetter)
			})

			It("should return err='invalid signature: RSA verification failure'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("invalid signature: RSA verification failure"))
			})
		})

		When("a note's signature message content/format is not expected", func() {
			var sig []byte
			BeforeEach(func() {
				createCommitAndNote(path, "file.txt", "a file", "commit msg", "note1")
				commitHash := getRecentCommitHash(path, "refs/notes/note1")
				msg := []byte("0" + "0" + "0x0000000000000000000000000000000000000001" + commitHash)
				sig, err = crypto.GPGSign(privKey, msg)
				Expect(err).To(BeNil())
				sigHex := hex.EncodeToString(sig)
				createNoteEntry(path, "note1", "tx: fee=0 nonce=0 pkId=0x0000000000000000000000000000000000000000 sig=0x"+sigHex)
				_, err = checkNote(repo, "refs/notes/note1", gpgPubKeyGetter)
			})

			It("should return err='...invalid signature: hash tag doesn't match'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("invalid signature: hash tag doesn't match"))
			})
		})

		When("a note's signature is valid", func() {
			var sig []byte
			BeforeEach(func() {
				createCommitAndNote(path, "file.txt", "a file", "commit msg", "note1")
				commitHash := getRecentCommitHash(path, "refs/notes/note1")
				msg := []byte("0" + "0" + "0x0000000000000000000000000000000000000000" + commitHash)
				sig, err = crypto.GPGSign(privKey, msg)
				Expect(err).To(BeNil())
				sigHex := hex.EncodeToString(sig)
				createNoteEntry(path, "note1", "tx: fee=0 nonce=0 pkId=0x0000000000000000000000000000000000000000 sig=0x"+sigHex)
				_, err = checkNote(repo, "refs/notes/note1", gpgPubKeyGetter)
			})

			It("should return nil", func() {
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".validateChange", func() {
		var err error

		When("change item has a reference name format that is not known", func() {
			BeforeEach(func() {
				change := &ItemChange{Item: &Obj{Name: "refs/others/name", Data: "stuff"}}
				_, err = validateChange(repo, change, gpgPubKeyGetter)
			})

			It("should return err='unrecognised change item'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("unrecognised change item"))
			})
		})

		When("change item referenced object is an unknown commit object", func() {
			BeforeEach(func() {
				change := &ItemChange{Item: &Obj{Name: "refs/heads/unknown", Data: "unknown_hash"}}
				_, err = validateChange(repo, change, gpgPubKeyGetter)
			})

			It("should return err='unable to get commit object: object not found'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("unable to get commit object: object not found"))
			})
		})

		When("change item referenced object is an unknown tag object", func() {
			BeforeEach(func() {
				change := &ItemChange{Item: &Obj{Name: "refs/tags/unknown", Data: "unknown_hash"}}
				_, err = validateChange(repo, change, gpgPubKeyGetter)
			})

			It("should return err='unable to get tag object: tag not found'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("unable to get tag object: tag not found"))
			})
		})

		When("branch item points to a valid commit", func() {
			var cob *object.Commit
			var err error

			BeforeEach(func() {
				pkEntity, _ := crypto.PGPEntityFromPubKey(pubKey)
				pkID := util.RSAPubKeyID(pkEntity.PrimaryKey.PublicKey.(*rsa.PublicKey))
				txLine := fmt.Sprintf("tx: fee=%s, nonce=%s, pkId=%s", "0", "0", pkID)
				appendSignedCommit(path, "file.txt", "line 1", txLine, gpgKeyID)
				commitHash, _ := script.ExecInDir(`git --no-pager log --oneline -1 --pretty=%H`, path).String()
				cob, _ = repo.CommitObject(plumbing.NewHash(strings.TrimSpace(commitHash)))

				change := &ItemChange{Item: &Obj{Name: "refs/heads/master", Data: cob.Hash.String()}}
				_, err = validateChange(repo, change, gpgPubKeyGetter)
			})

			It("should return nil", func() {
				Expect(err).To(BeNil())
			})
		})

		When("annotated tag item points to a valid commit", func() {
			var tob *object.Tag
			var err error

			BeforeEach(func() {
				pkEntity, _ := crypto.PGPEntityFromPubKey(pubKey)
				pkID := util.RSAPubKeyID(pkEntity.PrimaryKey.PublicKey.(*rsa.PublicKey))
				txLine := fmt.Sprintf("tx: fee=%s, nonce=%s, pkId=%s", "0", "0", pkID)
				createSignedCommitAndSignedAnnotatedTag(path, "file.txt", "first file", txLine, "v1", gpgKeyID)
				tagRef, _ := repo.Tag("v1")
				tob, _ = repo.TagObject(tagRef.Hash())

				change := &ItemChange{Item: &Obj{Name: "refs/tags/v1", Data: tob.Hash.String()}}
				_, err = validateChange(repo, change, gpgPubKeyGetter)
			})

			It("should return nil", func() {
				Expect(err).To(BeNil())
			})
		})

		When("lightweight tag item points to a valid commit", func() {
			var err error

			BeforeEach(func() {
				pkEntity, _ := crypto.PGPEntityFromPubKey(pubKey)
				pkID := util.RSAPubKeyID(pkEntity.PrimaryKey.PublicKey.(*rsa.PublicKey))
				txLine := fmt.Sprintf("tx: fee=%s, nonce=%s, pkId=%s", "0", "0", pkID)
				createSignedCommitAndLightWeightTag(path, "file.txt", "first file", txLine, "v1", gpgKeyID)
				tagRef, _ := repo.Tag("v1")

				change := &ItemChange{Item: &Obj{Name: "refs/tags/v1", Data: tagRef.Target().String()}}
				_, err = validateChange(repo, change, gpgPubKeyGetter)
			})

			It("should return nil", func() {
				Expect(err).To(BeNil())
			})
		})
	})
})
