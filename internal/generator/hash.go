package generator

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"github.com/cockroachdb/errors"
	"github.com/spf13/afero"
)

func calculateHash(content []byte) string {
	hash := sha256.Sum256(content)

	return hex.EncodeToString(hash[:])
}

func (g *Generator) verifyHash(path string) error {
	existingContent, err := afero.ReadFile(g.props.FS, path)
	if err != nil {
		return err
	}

	currentHash := calculateHash(existingContent)

	// Retrieve stored hash from manifest if available
	var storedHash string

	if cmd, err := g.findManifestCommand(); err == nil && cmd != nil {
		filename := filepath.Base(path)

		storedHash = cmd.Hashes[filename]
		if storedHash == "" && filename == "cmd.go" {
			storedHash = cmd.Hash
		}
	}

	// If hashes differ and we are not forcing, prompt the user
	if storedHash != "" && storedHash != currentHash && !g.config.Force {
		g.props.Logger.Warnf("Conflict detected for %s: File has been manually modified.", path)

		confirm := g.promptOverwrite(path)
		if !confirm {
			g.props.Logger.Warnf("Skipping overwrite of %s", path)

			return errors.Newf("overwrite skipped by user")
		}

		g.props.Logger.Warnf("Overwriting modified file %s", path)
	}

	return nil
}

func (g *Generator) promptOverwrite(path string) bool {
	// Skip prompt in non-interactive environments
	if os.Getenv("GTB_NON_INTERACTIVE") == "true" {
		return false
	}

	confirm := false // Default to false for safety

	err := huh.NewConfirm().
		Title("Refusing to overwrite " + path).
		Description("The file has been modified since it was last generated. Do you want to overwrite it?").
		Value(&confirm).
		Run()
	if err != nil {
		g.props.Logger.Warnf("Prompt failed (non-interactive?): %v. Skipping overwrite.", err)

		return false
	}

	return confirm
}
