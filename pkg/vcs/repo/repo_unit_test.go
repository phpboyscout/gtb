package repo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/phpboyscout/go-tool-base/pkg/config"
	"github.com/phpboyscout/go-tool-base/pkg/logger"
	"github.com/phpboyscout/go-tool-base/pkg/props"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepo_Unit_OpenLocal(t *testing.T) {
	tmpDir := t.TempDir()
	p := &props.Props{
		FS:     afero.NewOsFs(),
		Logger: logger.NewNoop(),
		Config: nil,
	}

	r, err := NewRepo(p)
	require.NoError(t, err)

	t.Run("init_new_repo", func(t *testing.T) {
		repo, tree, err := r.OpenLocal(tmpDir, "main")
		assert.NoError(t, err)
		assert.NotNil(t, repo)
		assert.NotNil(t, tree)
	})

	t.Run("open_existing_repo", func(t *testing.T) {
		repo, tree, err := r.OpenLocal(tmpDir, "main")
		assert.NoError(t, err)
		assert.NotNil(t, repo)
		assert.NotNil(t, tree)
	})
}

func TestRepo_Unit_GitOperations(t *testing.T) {
	tmpDir := t.TempDir()
	p := &props.Props{
		FS:     afero.NewOsFs(),
		Logger: logger.NewNoop(),
		Config: nil,
	}

	r, _ := NewRepo(p)
	_, wt, _ := r.OpenLocal(tmpDir, "main")

	// Create a dummy file for initial commit to ensure HEAD exists
	_ = os.WriteFile(filepath.Join(tmpDir, ".initial"), []byte("init"), 0644)
	_, _ = wt.Add(".initial")
	_, _ = r.Commit("initial commit", &git.CommitOptions{
		Author: &object.Signature{Name: "Test", Email: "test@example.com", When: time.Now()},
	})

	// Create and commit a file
	relPath := "test.txt"
	absPath := filepath.Join(tmpDir, relPath)
	_ = os.WriteFile(absPath, []byte("hello"), 0644)
	_, _ = wt.Add(relPath)
	_, _ = r.Commit("test commit", &git.CommitOptions{
		Author: &object.Signature{Name: "Test", Email: "test@example.com", When: time.Now()},
	})

	t.Run("FileExists", func(t *testing.T) {
		exists, err := r.FileExists(relPath)
		assert.NoError(t, err)
		assert.True(t, exists)

		exists, err = r.FileExists("missing.txt")
		assert.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("DirectoryExists", func(t *testing.T) {
		exists, err := r.DirectoryExists("")
		assert.NoError(t, err)
		assert.True(t, exists)

		// Create a file in a subdirectory
		subDir := "subdir"
		_ = os.Mkdir(filepath.Join(tmpDir, subDir), 0755)
		subFile := filepath.Join(subDir, "file.txt")
		_ = os.WriteFile(filepath.Join(tmpDir, subFile), []byte("sub"), 0644)
		_, _ = wt.Add(subFile)
		_, _ = r.Commit("subdir commit", &git.CommitOptions{
			Author: &object.Signature{Name: "T", Email: "e", When: time.Now()},
		})

		exists, err = r.DirectoryExists(subDir)
		assert.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("GetFile", func(t *testing.T) {
		file, err := r.GetFile(relPath)
		assert.NoError(t, err)
		assert.NotNil(t, file)
		assert.Equal(t, relPath, file.Name)
	})

	t.Run("AddToFS", func(t *testing.T) {
		memFS := afero.NewMemMapFs()
		file, _ := r.GetFile(relPath)
		targetPath := "/copied.txt"
		
		err := r.AddToFS(memFS, file, targetPath)
		assert.NoError(t, err)
		
		content, _ := afero.ReadFile(memFS, targetPath)
		assert.Equal(t, "hello", string(content))
	})

	t.Run("WalkTree", func(t *testing.T) {
		var files []string
		err := r.WalkTree(func(f *object.File) error {
			files = append(files, f.Name)
			return nil
		})
		assert.NoError(t, err)
		assert.Contains(t, files, relPath)
	})

	t.Run("CheckoutAndCreateBranch", func(t *testing.T) {
		err := r.CreateBranch("feature")
		assert.NoError(t, err)
		
		head, _ := r.repo.Head()
		assert.Equal(t, "refs/heads/feature", head.Name().String())

		err = r.Checkout(plumbing.NewBranchReferenceName("main"))
		assert.NoError(t, err)
		
		head, _ = r.repo.Head()
		assert.Equal(t, "refs/heads/main", head.Name().String())
	})
}

func TestRepo_Unit_AuthConfig(t *testing.T) {
	fs := afero.NewMemMapFs()
	
	t.Run("token_auth", func(t *testing.T) {
		cfg := config.NewReaderContainer(logger.NewNoop(), "yaml", strings.NewReader(`github: {auth: {env: "G"}}`))
		t.Setenv("G", "test-token")
		p := &props.Props{
			FS:     fs,
			Logger: logger.NewNoop(),
			Config: cfg,
		}
		r, err := NewRepo(p)
		assert.NoError(t, err)
		assert.NotNil(t, r.GetAuth())
	})

	t.Run("ssh_auth_agent", func(t *testing.T) {
		cfg := config.NewReaderContainer(logger.NewNoop(), "yaml", strings.NewReader(`github: {ssh: {key: {type: "agent"}}}`))
		p := &props.Props{
			FS:     fs,
			Logger: logger.NewNoop(),
			Config: cfg,
		}
		// This might fail if no agent is running, but let's see
		_, _ = NewRepo(p)
	})
}

func TestRepo_Unit_Options(t *testing.T) {
	t.Parallel()
	
	t.Run("WithConfig", func(t *testing.T) {
		r := &Repo{}
		cfg := &gitconfig.Config{}
		err := WithConfig(cfg)(r)
		assert.NoError(t, err)
		assert.Equal(t, cfg, r.config)
	})

	t.Run("CloneOptions", func(t *testing.T) {
		opts := &git.CloneOptions{}
		
		WithShallowClone(1)(opts)
		assert.Equal(t, 1, opts.Depth)
		
		WithSingleBranch("develop")(opts)
		assert.True(t, opts.SingleBranch)
		assert.Equal(t, "refs/heads/develop", opts.ReferenceName.String())
		
		WithNoTags()(opts)
		assert.Equal(t, git.NoTags, opts.Tags)
		
		WithRecurseSubmodules()(opts)
		assert.Equal(t, git.DefaultSubmoduleRecursionDepth, opts.RecurseSubmodules)
	})
}
