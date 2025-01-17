// Copyright 2020-Present VMware, Inc.
// SPDX-License-Identifier: Apache-2.0

package clusterstore

import (
	"fmt"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"

	"github.com/vmware-tanzu/kpack-cli/pkg/commands"
	"github.com/vmware-tanzu/kpack-cli/pkg/k8s"
)

func NewRemoveCommand(clientSetProvider k8s.ClientSetProvider, newWaiter func(dynamic.Interface) commands.ResourceWaiter) *cobra.Command {
	var buildpackages []string

	cmd := &cobra.Command{
		Use:   "remove <store> -b <buildpackage> [-b <buildpackage>...]",
		Short: "Remove buildpackage(s) from cluster store",
		Long: `Removes existing buildpackage(s) from a specific cluster-scoped buildpack store.
`,
		Example: `kp clusterstore remove my-store -b buildpackage@1.0.0
kp clusterstore remove my-store -b buildpackage@1.0.0 -b other-buildpackage@2.0.0
`,
		Args:         commands.ExactArgsWithUsage(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cs, err := clientSetProvider.GetClientSet("")
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			w := newWaiter(cs.DynamicClient)

			ch, err := commands.NewCommandHelper(cmd)
			if err != nil {
				return err
			}

			storeName := args[0]

			store, err := cs.KpackClient.KpackV1alpha2().ClusterStores().Get(ctx, storeName, metav1.GetOptions{})
			if k8serrors.IsNotFound(err) {
				return errors.Errorf("ClusterStore '%s' does not exist", storeName)
			} else if err != nil {
				return err
			}

			bpToStoreImage := map[string]corev1alpha1.StoreImage{}
			for _, bp := range buildpackages {
				if storeImage, ok := getStoreImage(store, bp); !ok {
					return errors.Errorf("Buildpackage '%s' does not exist in the ClusterStore", bp)
				} else {
					bpToStoreImage[bp] = storeImage
				}
			}

			if err = ch.PrintStatus("Removing Buildpackages..."); err != nil {
				return err
			}

			removeBuildpackages(ch, store, buildpackages, bpToStoreImage)

			if !ch.IsDryRun() {
				store, err = cs.KpackClient.KpackV1alpha2().ClusterStores().Update(ctx, store, metav1.UpdateOptions{})
				if err != nil {
					return err
				}
				if err := w.Wait(ctx, store); err != nil {
					return err
				}
			}

			if err = ch.PrintObj(store); err != nil {
				return err
			}

			return ch.PrintResult("ClusterStore %q updated", store.Name)
		},
	}
	cmd.Flags().StringArrayVarP(&buildpackages, "buildpackage", "b", []string{}, "buildpackage to remove")
	commands.SetDryRunOutputFlags(cmd)
	return cmd
}

func getStoreImage(store *v1alpha2.ClusterStore, buildpackage string) (corev1alpha1.StoreImage, bool) {
	for _, bp := range store.Status.Buildpacks {
		if fmt.Sprintf("%s@%s", bp.Id, bp.Version) == buildpackage {
			return bp.StoreImage, true
		}
	}
	return corev1alpha1.StoreImage{}, false
}

func removeBuildpackages(ch *commands.CommandHelper, store *v1alpha2.ClusterStore, buildpackages []string, bpToStoreImage map[string]corev1alpha1.StoreImage) {
	for _, bp := range buildpackages {
		ch.Printlnf("Removing buildpackage %s", bp)

		for i, img := range store.Spec.Sources {
			if img.Image == bpToStoreImage[bp].Image {
				store.Spec.Sources = append(store.Spec.Sources[:i], store.Spec.Sources[i+1:]...)
				break
			}
		}
	}
}
