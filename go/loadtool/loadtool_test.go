// This tool is most conveniently used by "adding a workspace folder" in visual
// studio code. The directory containing the go.mod file is the folder to add.
// With that done, vscode can be used to install and configure the go tooling
// necessary to run this tool. Treat it like a 12 factor app and use settings
// and launch.jsons to configure the test runs. If everything is setup
// correctly in vscode, TestQuorum will have a little grey "run test" hyperlink
// above it.

package loadtool_test

import (
	"context"
	"testing"

	"github.com/robinbryce/blockbench/loadtool"
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

	loadtool.LoadConfig
	adder loadtool.Adder
}

func (s *QuorumSuite) SetupSuite() {
	s.assert = assert.New(s.T())
	s.require = require.New(s.T())

	s.LoadConfig = loadtool.NewConfigFromEnv()
	var err error
	s.adder, err = loadtool.NewAdder(context.Background(), &s.LoadConfig)
	s.require.NoError(err)

}

// TestOneTransact succeedes if a single "add" transaction can be made for the
// test contract.
func (s *QuorumSuite) TestOneTransact() {
	err := s.adder.RunOne()
	s.require.NoError(err)
}

// TestQuorum issues "add" transactions from multiple threads. Note that it is
// not very chatty.
func (s *QuorumSuite) TestQuorum() {
	s.adder.Run()
}
