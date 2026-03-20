package generator

import (
	"context"
	"path/filepath"

	"github.com/cockroachdb/errors"
	"github.com/spf13/afero"
)

func (g *Generator) Remove(ctx context.Context) error {
	if err := g.verifyProject(); err != nil {
		return err
	}

	cmdDir, err := g.getCommandPath()
	if err != nil {
		return err
	}

	g.props.Logger.Infof("Removing command %s in %s...", g.config.Name, cmdDir)

	if err := g.performRemoval(cmdDir); err != nil {
		return err
	}

	g.cleanupDocumentation()

	// Also regenerate indices
	if err := g.generateCommandsIndex(); err != nil {
		g.props.Logger.Warnf("Failed to regenerate commands index: %v", err)
	}

	if err := g.regenerateMkdocsNav(); err != nil {
		g.props.Logger.Warnf("Failed to regenerate mkdocs navigation: %v", err)
	}

	g.props.Logger.Infof("Successfully removed command %s.", g.config.Name)

	return nil
}

func (g *Generator) performRemoval(cmdDir string) error {
	// 1. Deregister from parent
	if err := g.deregisterSubcommand(); err != nil {
		g.props.Logger.Warnf("Failed to deregister subcommand: %v", err)
	} else if err := g.updateParentCmdHash(); err != nil {
		g.props.Logger.Warnf("Failed to update parent command hash after deregistration: %v", err)
	}

	// 2. Remove from manifest
	if err := g.removeFromManifest(); err != nil {
		return err
	}

	// 3. Delete command directory
	if err := g.props.FS.RemoveAll(cmdDir); err != nil {
		return errors.Newf("failed to remove command directory: %w", err)
	}

	return nil
}

func (g *Generator) cleanupDocumentation() {
	// 4. Delete documentation
	promptParentParts, _ := g.FindCommandParentPath(g.config.Name)

	outRelPath := g.config.Name
	if len(promptParentParts) > 0 {
		outRelPath = filepath.Join(filepath.Join(promptParentParts...), g.config.Name)
	}

	docDir := filepath.Join(g.config.Path, "docs", "commands", outRelPath)
	if exists, _ := afero.Exists(g.props.FS, docDir); exists {
		if err := g.props.FS.RemoveAll(docDir); err != nil {
			g.props.Logger.Warnf("Failed to remove documentation directory: %v", err)
		}
	}
}
