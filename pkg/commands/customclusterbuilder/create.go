package customclusterbuilder

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/ghodss/yaml"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/pivotal/build-service-cli/pkg/k8s"
	expv1alpha1 "github.com/pivotal/kpack/pkg/apis/experimental/v1alpha1"
)

const (
	defaultStore             = "default"
	kubectlLastAppliedConfig = "kubectl.kubernetes.io/last-applied-configuration"
)

func NewCreateCommand(clientSetProvider k8s.ClientSetProvider) *cobra.Command {
	var (
		stack string
		order string
	)

	cmd := &cobra.Command{
		Use:   "create <name> <tag>",
		Short: "Create a custom cluster builder",
		Long: `Create a custom cluster builder by providing command line arguments.
This custom cluster builder will be created if it does not yet exist.
`,
		Example: `tbctl ccb create my-builder my-registry.com/my-builder-tag --order /path/to/order.yaml --stack tiny
tbctl ccb create my-builder my-registry.com/my-builder-ta --order /path/to/order.yaml
`,
		Args:         cobra.ExactArgs(2),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			tag := args[1]

			cs, err := clientSetProvider.GetClientSet("")
			if err != nil {
				return err
			}

			ccb := &expv1alpha1.CustomClusterBuilder{
				TypeMeta: metav1.TypeMeta{
					Kind:       expv1alpha1.CustomClusterBuilderKind,
					APIVersion: "experimental.kpack.pivotal.io/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:        name,
					Annotations: map[string]string{},
				},
				Spec: expv1alpha1.CustomClusterBuilderSpec{
					CustomBuilderSpec: expv1alpha1.CustomBuilderSpec{
						Tag:   tag,
						Stack: stack,
						Store: defaultStore,
					},
					ServiceAccountRef: corev1.ObjectReference{
						Namespace: "build-service",
						Name:      "ccb-service-account"},
				},
			}

			ccb.Spec.Order, err = readOrder(order)
			if err != nil {
				return err
			}

			marshal, err := json.Marshal(ccb)
			if err != nil {
				return err
			}

			ccb.Annotations[kubectlLastAppliedConfig] = string(marshal)

			_, err = cs.KpackClient.ExperimentalV1alpha1().CustomClusterBuilders().Create(ccb)
			if err != nil {
				return err
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "\"%s\" created\n", ccb.Name)
			return err
		},
	}
	cmd.Flags().StringVarP(&stack, "stack", "s", "default", "stack resource to use")
	cmd.Flags().StringVarP(&order, "order", "o", "", "path to buildpack order yaml")

	return cmd
}

func readOrder(path string) ([]expv1alpha1.OrderEntry, error) {
	var (
		file io.ReadCloser
		err  error
	)

	if path == "-" {
		file = os.Stdin
	} else {
		file, err = os.Open(path)
		if err != nil {
			return nil, err
		}
	}
	defer file.Close()

	buf, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var order []expv1alpha1.OrderEntry
	return order, yaml.Unmarshal(buf, &order)
}
