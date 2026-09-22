package docker

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/containerd/errdefs"
	"github.com/moby/moby/client"

	"github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora/canary/model"
)

const revisionLabel = "org.opencontainers.image.revision"

var (
	imageIDPattern  = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)
	revisionPattern = regexp.MustCompile(`^[a-f0-9]{40}$`)
)

// ResolveImage pins an image reference for one canary run and reports the
// source commit it was built from. A reference that is not present locally is
// pulled first.
//
// A reference to the official repository resolves to the registry digest
// Docker recorded for it, so a run is bound to published content even when the
// reference was a tag. Anything else, such as a local build, resolves to its
// image ID. Docker's containerd image store records a digest for local builds
// too, so the repository, not the presence of a digest, decides. The commit
// comes from the OCI revision label, which the official images carry and a
// local build must set with
// `docker build --label org.opencontainers.image.revision=<commit>`.
func ResolveImage(ctx context.Context, ref string) (image, revision string, err error) {
	ref = strings.TrimSpace(ref)
	if ref == "" || strings.ContainsAny(ref, " \t\r\n") {
		return "", "", errors.New("image reference required")
	}
	cli, err := client.New(client.FromEnv)
	if err != nil {
		return "", "", err
	}
	defer cli.Close()

	inspected, err := cli.ImageInspect(ctx, ref)
	if errdefs.IsNotFound(err) {
		pull, pullErr := cli.ImagePull(ctx, ref, client.ImagePullOptions{})
		if pullErr != nil {
			return "", "", pullErr
		}
		pullErr = pull.Wait(ctx)
		pull.Close()
		if pullErr != nil {
			return "", "", pullErr
		}
		inspected, err = cli.ImageInspect(ctx, ref)
	}
	if err != nil {
		return "", "", err
	}
	if inspected.Config != nil {
		revision = inspected.Config.Labels[revisionLabel]
	}
	if !revisionPattern.MatchString(revision) {
		return "", "", errors.New("image carries no full-commit " + revisionLabel + " label")
	}
	if image = officialDigest(ref, inspected.RepoDigests); image != "" {
		return image, revision, nil
	}
	if !imageIDPattern.MatchString(inspected.ID) {
		return "", "", errors.New("image has no content-addressed ID")
	}
	return inspected.ID, revision, nil
}

// officialDigest picks the digest recorded for the official repository when ref
// names that repository.
func officialDigest(ref string, digests []string) string {
	repository := ref
	if at := strings.Index(repository, "@"); at >= 0 {
		repository = repository[:at]
	} else if colon := strings.LastIndex(repository, ":"); colon > strings.LastIndex(repository, "/") {
		repository = repository[:colon]
	}
	if repository != model.OfficialImageRepository {
		return ""
	}
	for _, digest := range digests {
		if strings.HasPrefix(digest, repository+"@sha256:") {
			return digest
		}
	}
	return ""
}
