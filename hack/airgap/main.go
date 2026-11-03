package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

var (
	maxDownloadSize int64 = 1 << 20 // 1MB

	// requiredImages defines the list of images that must be saved
	// into the output tar.
	requiredImages = map[string]struct{}{
		"docker.io/rancher/mirrored-pause":           {},
		"docker.io/rancher/mirrored-coredns-coredns": {},
	}

	k3sVersion = regexp.MustCompile(`^v\d+\.\d+\.\d+\+k3s\d+$`)

	// ease testing
	fetcher      = fetch
	imageFetcher = fetchImage
)

var (
	version = flag.String("k3s-version", "", "k3s version to be used to pull the images file")
	output  = flag.String("output", "", "target file to be generated")
	arch    = flag.String("arch", runtime.GOARCH, "target architecture for images")
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	flag.Parse()
	if err := Save(ctx, *version, *arch, *output); err != nil {
		log.Fatalf("failed to save tarball: %+v\n", err)
	}
}

// Save exports specific k3s container images into a tarball.
// Only the images defined in requiredImages are exported.
//
// This is needed for Rancher Manager's airgap support and
// replaces docker/podman's save command.
// Note that both docker and podman do not support the execution
// of save during a container image building process.
func Save(ctx context.Context, version, arch, output string) error {
	if version == "" {
		return fmt.Errorf("k3s version must be specified")
	} else if arch == "" {
		return fmt.Errorf("arch must be specified")
	} else if output == "" {
		return fmt.Errorf("output must be specified")
	}

	imgs, err := k3sImages(ctx, version)
	if err != nil {
		return err
	}

	// shortcut: pulls single-arch images for target linux platform rather than multi-arch indexes, upgrade to multi-arch index preservation if k3s airgap tar format changes
	platform := v1.Platform{OS: "linux", Architecture: arch}
	refToImage := make(map[name.Reference]v1.Image, len(imgs))

	for _, img := range imgs {
		fmt.Printf("pulling image %s\n", img)
		ref, err := name.ParseReference(img)
		if err != nil {
			return fmt.Errorf("invalid image reference %q: %w", img, err)
		}

		imgObj, err := imageFetcher(ctx, ref, platform)
		if err != nil {
			return fmt.Errorf("failed to fetch image %q: %w", img, err)
		}
		refToImage[ref] = imgObj
	}

	if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		return err
	}

	return tarball.MultiRefWriteToFile(output, refToImage)
}

func fetchImage(ctx context.Context, ref name.Reference, platform v1.Platform) (v1.Image, error) {
	return remote.Image(ref,
		remote.WithContext(ctx),
		remote.WithPlatform(platform),
	)
}

func fetch(ctx context.Context, url string) (io.ReadCloser, error) {
	fmt.Println("fetching", url)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request unsuccessful: %w", err)
	}
	if res.StatusCode != http.StatusOK {
		_ = res.Body.Close()
		return nil, fmt.Errorf("request failed with status %s", res.Status)
	}
	return res.Body, nil
}

func k3sImages(ctx context.Context, version string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if !k3sVersion.MatchString(version) {
		return nil, fmt.Errorf("invalid k3s version: %s", version)
	}

	url := fmt.Sprintf("https://github.com/k3s-io/k3s/releases/download/%s/k3s-images.txt", version)
	body, err := fetcher(ctx, url)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	images := []string{}
	scanner := bufio.NewScanner(io.LimitReader(body, maxDownloadSize))
	for scanner.Scan() {
		fqn := scanner.Text()
		imageName := strings.SplitN(fqn, ":", 2)[0]
		if _, ok := requiredImages[imageName]; ok {
			images = append(images, fqn)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error processing k3s-images.txt file: %w", err)
	}

	return images, nil
}
