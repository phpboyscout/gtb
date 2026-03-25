package utils

import (
	"testing"

	"github.com/phpboyscout/go-tool-base/pkg/logger"
	"github.com/stretchr/testify/assert"
)

func TestGracefulGetPath_Success(t *testing.T) {
	l := logger.NewNoop()
	path, err := GracefulGetPath("ls", l)
	assert.NoError(t, err)
	assert.NotEmpty(t, path)
}

func TestGracefulGetPath_Failure(t *testing.T) {
	l := logger.NewNoop()
	path, err := GracefulGetPath("non_existent_cmd_xyz_123", l, "Instructions")
	assert.Error(t, err)
	assert.Empty(t, path)
}

func TestGracefulGetPath_KnownInstruction(t *testing.T) {
	// Add a temporary entry to the Instructions map so the known-instruction
	// branch is exercised without depending on system tooling.
	const testCmd = "gtb-test-tool-xyz"
	Instructions[testCmd] = "Install test tool: example.com/install"
	t.Cleanup(func() { delete(Instructions, testCmd) })

	l := logger.NewNoop()
	path, err := GracefulGetPath(testCmd, l)
	assert.Error(t, err)
	assert.Empty(t, path)
}
