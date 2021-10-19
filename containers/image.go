package containers

import (
	"context"
	"fmt"
	"log"
	"os"
	"rkd/helpers"
	"strings"

	"github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/docker/archive"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
)

// DownloadImage download docker images from src and create docker-archive
func DownloadImage(imgList []string, destImg string, fetchSignature bool, destSig string) error {

	// Create new dest archive
	aw, err := archive.NewWriter(nil, destImg)
	if err != nil {
		log.Printf("Error when initializing destination archive.\n")
		return err
	}
	defer aw.Close()

	for _, img := range imgList {

		// Ref
		imgRef := fmt.Sprintf("%s%s", "docker://", img)
		srcRef, err := alltransports.ParseImageName(imgRef)
		if err != nil {
			log.Printf("Error when parsing image name for %s", img)
			return err
		}

		////////// Dest
		imgNamed, err := reference.ParseDockerRef(img)
		if err != nil {
			log.Printf("Error when parsing image reference for %s", img)
			return err
		}
		imgNameTagged, err := reference.WithTag(imgNamed, getImgTag(img))
		if err != nil {
			log.Printf("Error when parsing image reference and tag for %s", img)
			return err
		}
		// Create dest ref
		destRef, err := aw.NewReference(imgNameTagged)
		if err != nil {
			log.Printf("Error when creating new image reference for %s", img)
			return err
		}

		// Download and create tar
		fmt.Printf("Copy %s to %s\n", img, destImg)
		rawManifest, err := copyImg(context.Background(), srcRef, destRef)
		if err != nil {
			log.Printf("Error when downloading image %s", img)
			return err
		}

		// Fetching Cosing signature
		// Do not fail at signature error (the signature may not exist yet)
		if fetchSignature {
			imgDigest, err := manifest.Digest(rawManifest)
			if err != nil {
				log.Printf("Error when fecthing digest image %s \n %s", srcRef.DockerReference().Name(), err)
				continue
			}

			// Creating Cosign reference
			imgRefSig := fmt.Sprintf("%s%s:%s%s", "docker://", strings.Split(imgNameTagged.Name(), ":")[0], strings.Replace(imgDigest.String(), ":", "-", 1), ".sig")
			srcRefSig, err := alltransports.ParseImageName(imgRefSig)
			if err != nil {
				log.Printf("Error when downloading signature %s \n %s", imgRefSig, err)
				continue
			}
			// Creating directory for signature
			// destDirSig := fmt.Sprintf("%s/%s/%s%s", dest, "signature", strings.Replace(imgDigest.String(), ":", "-", 1), ".sig")
			destDirSig := fmt.Sprintf("%s/%s%s", destSig, strings.Replace(imgDigest.String(), ":", "-", 1), ".sig")
			helpers.CreateDestDir(destDirSig)
			path, _ := os.Getwd()

			// Creating destination reference (Since it's not tar layered cannot reuse same method as above)
			destRefSig, err := alltransports.ParseImageName(fmt.Sprintf("%s/%s/%s", "dir:", path, destDirSig))
			if err != nil {
				log.Printf("Error when downloading signature %s \n %s", imgRefSig, err)
				continue
			}
			_, err = copyImg(context.Background(), srcRefSig, destRefSig)
			if err != nil {
				log.Printf("Error when downloading signature %s \n %s", imgRefSig, err)
				continue
			}
		}
	}
	return nil
}

func copyImg(ctx context.Context, srcImgRef types.ImageReference, destImgRef types.ImageReference) (manifest []byte, err error) {
	// Contexts
	defaultPolicy, err := signature.NewPolicyFromFile("policy.json")
	if err != nil {
		log.Printf("Default policy err.\n")
		return nil, err
	}
	policyContext, err := signature.NewPolicyContext(defaultPolicy)
	if err != nil {
		log.Printf("Policy context err.\n")
		return nil, err
	}
	defer policyContext.Destroy()
	// Create systemContext to select os and arch
	sysCtx := &types.SystemContext{
		ArchitectureChoice: "amd64",
		OSChoice:           "linux",
	}
	return copy.Image(context.Background(), policyContext, destImgRef, srcImgRef, &copy.Options{
		ReportWriter: os.Stdout,
		SourceCtx:    sysCtx,
	})
}

func getImgTag(imgStr string) string {
	strSlice := strings.Split(imgStr, ":")
	tag := "latest"
	if len(strSlice) > 1 {
		tag = strSlice[len(strSlice)-1]
	}
	return tag
}
