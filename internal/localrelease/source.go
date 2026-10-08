package localrelease

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/neko233-com/gocode/internal/nativeguard"
)

func CheckCleanSource(ctx context.Context, project, source string) error {
	if !ValidSource(source) {
		return errors.New("local publication requires a full immutable source commit")
	}
	for _, query := range []struct {
		args []string
		want string
	}{{[]string{"git", "rev-parse", "HEAD"}, source}, {[]string{"git", "status", "--porcelain", "--untracked-files=normal"}, ""}} {
		result, err := nativeguard.Run(ctx, nativeguard.Options{Command: query.args, Directory: project, Timeout: 30 * time.Second})
		if err != nil || strings.TrimSpace(string(result.Stdout)) != query.want {
			return errors.New("local release requires the exact clean committed source, including helper and workflow changes")
		}
	}
	return nil
}

// InputDigest binds actual tracked source bytes, not a merely claimed commit.
// Generated caches/resources are covered by artifact hashes and are never
// smuggled into the source set as previously tested product code.
func InputDigest(ctx context.Context, project string) (string, error) {
	result, err := nativeguard.Run(ctx, nativeguard.Options{Command: []string{"git", "ls-files", "-z"}, Directory: project, Timeout: 30 * time.Second})
	if err != nil {
		return "", err
	}
	names := strings.Split(strings.TrimSuffix(string(result.Stdout), "\x00"), "\x00")
	if len(names) == 0 || len(names) > 4096 {
		return "", errors.New("bounded tracked source input list required")
	}
	slices.Sort(names)
	hash := sha256.New()
	for _, name := range names {
		path, err := Within(project, name)
		if err != nil {
			return "", err
		}
		file, err := HashFile(ctx, path, 128<<20)
		if err != nil {
			return "", err
		}
		_, _ = fmt.Fprintf(hash, "%s\x00%d\x00%s\n", name, file.Bytes, file.SHA256)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
