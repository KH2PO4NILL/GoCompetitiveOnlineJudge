package config

import (
	"path/filepath"
	"testing"

	"github.com/zeromicro/go-zero/core/conf"
)

func TestAuthConfigLoadsInternalAuthentication(t *testing.T) {
	t.Setenv("AUTH_INTERNAL_PROBLEM_TOKEN", "problem-token")
	t.Setenv("AUTH_INTERNAL_CONTEST_TOKEN", "contest-token")
	var c Config
	if err := conf.Load(filepath.Join("..", "..", "etc", "auth.yaml"), &c, conf.UseEnv()); err != nil {
		t.Fatalf("load auth config: %v", err)
	}
	if len(c.InternalAuth.Callers) != 2 {
		t.Fatalf("InternalAuth callers = %+v", c.InternalAuth.Callers)
	}
	if c.InternalAuth.Callers[0].Token != "problem-token" || c.InternalAuth.Callers[1].Token != "contest-token" {
		t.Fatalf("InternalAuth tokens were not expanded: %+v", c.InternalAuth.Callers)
	}
}
