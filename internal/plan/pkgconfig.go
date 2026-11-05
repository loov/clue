package plan

import (
	"context"

	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/deps"
	"github.com/loov/clue/internal/toolchain"
)

// ResolvePkgConfig queries pkg-config for pkg through tc, searching only
// the configured toolchain.pkgConfigPath when it is set.
func ResolvePkgConfig(ctx context.Context, cfg *config.Config, tc toolchain.Toolchain, pkg *deps.PkgConfigDependency) (deps.Usage, error) {
	return pkg.ResolveWithRunner(ctx, cfg.Toolchain.PkgConfigPath, func(ctx context.Context, env []string, name string, args ...string) (string, error) {
		return toolchain.OutputEnv(ctx, tc, ".", env, name, args...)
	})
}
