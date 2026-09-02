package deps

import "testing"

func TestRemoteDependenciesRejectHTTP(t *testing.T) {
	for _, dep := range []Dependency{
		NewGitDependency("git", "http://example.com/repo.git", "main", nil),
		NewTarballDependency("tar", "http://example.com/archive.tar.gz", "", "", nil),
	} {
		if err := dep.Validate(); err == nil {
			t.Errorf("%s dependency accepted unauthenticated HTTP", dep.Type())
		}
	}
}
