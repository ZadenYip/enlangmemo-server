// 所有 auth integration 测试的入口
package authintegration

import (
	"os"
	"testing"

	"github.com/zadenyip/enlangmemo-server/test/testenv"
)

var (
	suite *testenv.Suite
)

func TestMain(m *testing.M) {
	os.Exit(testenv.Run(m, bindSuite))
}

func bindSuite(s *testenv.Suite) {
	suite = s
}

func resetEnv(t *testing.T) {
	t.Helper()
	suite.Reset(t)
	bindSuite(suite)
}
