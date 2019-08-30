package pkg

import (
	"path/filepath"

	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"stash.appscode.dev/stash/pkg/restic"
)

func NewCmdBackup() *cobra.Command {
	var (
		masterURL      string
		kubeconfigPath string
		namespace      string
		outputDir      string
		shards         []string
	)

	cmd := &cobra.Command{
		Use:               "backup",
		Short:             "Takes snapshots of statefulset's volumes",
		DisableAutoGenTag: true,
		RunE: func(cmd *cobra.Command, args []string) error {

			// prepare client
			config, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfigPath)
			if err != nil {
				return err
			}
			kubeClient := kubernetes.NewForConfigOrDie(config)

			// Freeze database
			err = freezeDatabase()
			if err != nil {
				return err
			}

			// Take snapshot of the shards
			backupOutput, backupErr := takeVolumeSnapshot(kubeClient, shards)

			// Resume database
			err = resumeDatabase()
			if err != nil {
				return err
			}

			// If output directory specified, then write the output in "output.json" file in the specified directory
			if backupErr == nil && outputDir != "" {
				err := backupOutput.WriteOutput(filepath.Join(outputDir, restic.DefaultOutputFileName))
				if err != nil {
					return err
				}
			}
			return backupErr
		},
	}

	cmd.Flags().StringVar(&masterURL, "master", masterURL, "The address of the Kubernetes API server (overrides any value in kubeconfig)")
	cmd.Flags().StringVar(&kubeconfigPath, "kubeconfig", kubeconfigPath, "Path to kubeconfig file with authorization information (the master location is set by the master flag).")
	cmd.Flags().StringVar(&namespace, "namespace", "default", "Namespace of Backup/Restore Session")
	cmd.Flags().StringVar(&outputDir, "output-dir", outputDir, "Directory where output.json file will be written (keep empty if you don't need to write output in file)")
	cmd.Flags().StringSliceVar(&shards, "shards", shards, "Name of the shards separated by comma(,)")

	return cmd
}

func takeVolumeSnapshot(kubeClient kubernetes.Interface, shads []string) (*restic.BackupOutput, error) {

	return nil, nil
}
