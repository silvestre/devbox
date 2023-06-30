package nixsearch

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/samber/lo"
	"go.jetpack.io/devbox/internal/devpkg/devpkgutil"
	"go.jetpack.io/devbox/internal/lock"
	"go.jetpack.io/devbox/internal/nix"
)

var ErrPackageNotFound = errors.New("package not found")
var ErrPackageNotInstalled = errors.New("package not installed")

type Info struct {
	// attribute key is different in flakes vs legacy so we should only use it
	// if we know exactly which version we are using
	AttributeKey string
	PName        string
	Version      string
}

func (i *Info) String() string {
	return fmt.Sprintf("%s-%s", i.PName, i.Version)
}

func PkgInfo(pkg string, lock lock.Locker) *Info {
	locked, err := lock.Resolve(pkg)
	if err != nil {
		return nil
	}

	results := Search(locked.Resolved)
	if len(results) == 0 {
		return nil
	}
	// we should only have one result
	return lo.Values(results)[0]
}

func Search(url string) map[string]*Info {
	return searchSystem(url, "")
}

func parseSearchResults(data []byte) map[string]*Info {
	var results map[string]map[string]any
	err := json.Unmarshal(data, &results)
	if err != nil {
		panic(err)
	}
	infos := map[string]*Info{}
	for key, result := range results {
		infos[key] = &Info{
			AttributeKey: key,
			PName:        result["pname"].(string),
			Version:      result["version"].(string),
		}

	}
	return infos
}

// PkgExistsForAnySystem is a bit slow (~600ms). Only use it if there's already
// been an error and we want to provide a better error message.
func PkgExistsForAnySystem(pkg string) bool {
	systems := []string{
		// Check most common systems first.
		"x86_64-linux",
		"x86_64-darwin",
		"aarch64-linux",
		"aarch64-darwin",

		"armv5tel-linux",
		"armv6l-linux",
		"armv7l-linux",
		"i686-linux",
		"mipsel-linux",
		"powerpc64le-linux",
		"riscv64-linux",
	}
	for _, system := range systems {
		if len(searchSystem(pkg, system)) > 0 {
			return true
		}
	}
	return false
}

func searchSystem(url string, system string) map[string]*Info {
	// Eventually we may pass a writer here, but for now it is safe to use stderr
	writer := os.Stderr
	// Search will download nixpkgs if it's not already downloaded. Adding this
	// check here provides a slightly better UX.
	if devpkgutil.IsGithubNixpkgsURL(url) {
		hash := devpkgutil.HashFromNixPkgsURL(url)
		// purposely ignore error here. The function already prints an error.
		// We don't want to panic or stop execution if we can't prefetch.
		_ = nix.EnsureNixpkgsPrefetched(writer, hash)
	}
	out, err := nix.Search(os.Stderr, url, system)
	if err != nil {
		// for now, assume all errors are invalid packages.
		return nil
	}
	return parseSearchResults(out)
}