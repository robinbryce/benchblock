// This tool is most conveniently used by "adding a workspace folder" in visual
// studio code. The directory containing the go.mod file is the folder to add.
// With that done, vscode can be used to install and configure the go tooling
// necessary to run this tool. Treat it like a 12 factor app and use settings
// and launch.jsons to configure the test runs. If everything is setup
// correctly in vscode, TestQuorum will have a little grey "run test" hyperlink
// above it.

package loader_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/robinbryce/blockbench/loadtool/cmd"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestQuorum(t *testing.T) {
	suite.Run(t, new(QuorumSuite))
}

type QuorumSuite struct {
	suite.Suite
	assert  *assert.Assertions
	require *require.Assertions

	cmd *cobra.Command
}

func (s *QuorumSuite) SetupSuite() {
	fmt.Printf("THREADS: %s\n", os.Getenv("THREADS"))
	s.assert = assert.New(s.T())
	s.require = require.New(s.T())

	s.cmd = cmd.NewRootCmd()
}

// TestOneTransact succeedes if a single "add" transaction can be made for the
// test contract.
func (s *QuorumSuite) TestOneTransact() {

	s.cmd.SetArgs([]string{"--one", "--singlenode", "-t", "1"})
	s.cmd.Execute()
}

// TestQuorum issues "add" transactions from multiple threads. Note that it is
// not very chatty.
func (s *QuorumSuite) TestQuorum() {
	s.cmd.SetArgs([]string{})
	s.cmd.Execute()
}
