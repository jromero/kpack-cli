package custombuilder

import (
	"fmt"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/pivotal/build-service-cli/pkg/builder"
	"github.com/pivotal/build-service-cli/pkg/k8s"
)

func NewPatchCommand(clientSetProvider k8s.ClientSetProvider) *cobra.Command {
	var (
		namespace string
		stack     string
		order     string
	)

	cmd := &cobra.Command{
		Use:          "patch <name>",
		Short:        "Patch an existing custom builder configuration",
		Long:         ` `,
		Example:      `tbctl cb patch my-builder`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cs, err := clientSetProvider.GetClientSet(namespace)
			if err != nil {
				return err
			}

			cb, err := cs.KpackClient.ExperimentalV1alpha1().CustomBuilders(cs.Namespace).Get(args[0], metav1.GetOptions{})
			if err != nil {
				return err
			}

			patchedCb := cb.DeepCopy()

			if stack != "" {
				patchedCb.Spec.Stack = stack
			}

			if order != "" {
				orderEntries, err := builder.ReadOrder(order)
				if err != nil {
					return err
				}

				patchedCb.Spec.Order = orderEntries
			}

			patch, err := k8s.CreatePatch(cb, patchedCb)
			if err != nil {
				return err
			}

			if len(patch) == 0 {
				_, err = fmt.Fprintln(cmd.OutOrStdout(), "nothing to patch")
				return err
			}

			_, err = cs.KpackClient.ExperimentalV1alpha1().CustomBuilders(cs.Namespace).Patch(args[0], types.MergePatchType, patch)
			if err != nil {
				return err
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "\"%s\" patched\n", cb.Name)
			return err
		},
	}
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "kubernetes namespace")
	cmd.Flags().StringVarP(&stack, "stack", "s", "", "stack resource to use")
	cmd.Flags().StringVarP(&order, "order", "o", "", "path to buildpack order yaml")

	return cmd
}
