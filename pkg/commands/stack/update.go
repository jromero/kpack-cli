package stack

import (
	"fmt"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	expv1alpha1 "github.com/pivotal/kpack/pkg/apis/experimental/v1alpha1"
	"github.com/pivotal/kpack/pkg/client/clientset/versioned"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/pivotal/build-service-cli/pkg/commands"
	stackpkg "github.com/pivotal/build-service-cli/pkg/stack"
)

type ImageFetcher interface {
	Fetch(src string) (v1.Image, error)
}

type ImageRelocator interface {
	Relocate(image v1.Image, dest string) (string, error)
}

func NewUpdateCommand(kpackClient versioned.Interface, fetcher ImageFetcher, relocator ImageRelocator) *cobra.Command {
	var (
		buildImageRef string
		runImageRef   string
	)

	cmd := &cobra.Command{
		Use:   "update <name>",
		Short: "Update a stack",
		Long: `Updates the run and build images of a stack.

The run and build images will be uploaded to the the registry configured on your stack.
Therefore, you must have credentials to access the registry on your machine.
`,
		Example: `tbctl stack update my-stack --build-image my-registry.com/build --run-image my-registry.com/run
tbctl stack update my-stack --build-image ../path/to/build.tar --run-image ../path/to/run.tar
`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			printer := commands.NewPrinter(cmd)

			stack, err := kpackClient.ExperimentalV1alpha1().Stacks().Get(args[0], metav1.GetOptions{})
			if err != nil {
				return err
			}

			repository, ok := stack.Annotations[stackpkg.DefaultRepositoryAnnotation]
			if !ok || repository == "" {
				return errors.Errorf("Unable to find default registry for stack: %s", args[0])
			}

			printer.Printf("Uploading to '%s'...", repository)

			buildImage, err := fetcher.Fetch(buildImageRef)
			if err != nil {
				return err
			}

			buildStackId, err := stackpkg.GetStackId(buildImage)
			if err != nil {
				return err
			}

			runImage, err := fetcher.Fetch(runImageRef)
			if err != nil {
				return err
			}

			runStackId, err := stackpkg.GetStackId(runImage)
			if err != nil {
				return err
			}

			if buildStackId != runStackId {
				return errors.Errorf("build stack '%s' does not match run stack '%s'", buildStackId, runStackId)
			}

			relocatedBuildImageRef, err := relocator.Relocate(buildImage, fmt.Sprintf("%s/%s", repository, stackpkg.BuildImageName))
			if err != nil {
				return err
			}

			relocatedRunImageRef, err := relocator.Relocate(runImage, fmt.Sprintf("%s/%s", repository, stackpkg.RunImageName))
			if err != nil {
				return err
			}

			if wasUpdated, err := updateStack(stack, relocatedBuildImageRef, relocatedRunImageRef, buildStackId); err != nil {
				return err
			} else if !wasUpdated {
				printer.Printf("Build and Run images already exist in stack\nStack Unchanged")
				return nil
			}

			_, err = kpackClient.ExperimentalV1alpha1().Stacks().Update(stack)
			if err != nil {
				return err
			}

			printer.Printf("Stack Updated")
			return nil
		},
	}

	cmd.Flags().StringVarP(&buildImageRef, "build-image", "b", "", "build image tag or local tar file path")
	cmd.Flags().StringVarP(&runImageRef, "run-image", "r", "", "run image tag or local tar file path")
	_ = cmd.MarkFlagRequired("build-image")
	_ = cmd.MarkFlagRequired("run-image")

	return cmd
}

func updateStack(stack *expv1alpha1.Stack, buildImageRef, runImageRef, stackId string) (bool, error) {
	oldBuildDigest, err := stackpkg.GetDigest(stack.Status.BuildImage.LatestImage)
	if err != nil {
		return false, err
	}

	newBuildDigest, err := stackpkg.GetDigest(buildImageRef)
	if err != nil {
		return false, err
	}

	oldRunDigest, err := stackpkg.GetDigest(stack.Status.RunImage.LatestImage)
	if err != nil {
		return false, err
	}

	newRunDigest, err := stackpkg.GetDigest(runImageRef)
	if err != nil {
		return false, err
	}

	if oldBuildDigest != newBuildDigest && oldRunDigest != newRunDigest {
		stack.Spec.BuildImage.Image = buildImageRef
		stack.Spec.RunImage.Image = runImageRef
		stack.Spec.Id = stackId
		return true, nil
	}

	return false, nil
}