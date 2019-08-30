package pkg

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/appscode/go/types"
	vs "github.com/kubernetes-csi/external-snapshotter/pkg/apis/volumesnapshot/v1alpha1"
	vs_cs "github.com/kubernetes-csi/external-snapshotter/pkg/client/clientset/versioned"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"stash.appscode.dev/stash/apis"
	api_v1beta1 "stash.appscode.dev/stash/apis/stash/v1beta1"
	"stash.appscode.dev/stash/pkg/restic"
	stash_util "stash.appscode.dev/stash/pkg/util"
)

type vsOptions struct {
	namespace               string
	outputDir               string
	shards                  []string
	volumeSnapshotClassName string
	kubeClient              kubernetes.Interface
	vsClient                vs_cs.Interface
}

func NewCmdBackup() *cobra.Command {
	var (
		masterURL      string
		kubeconfigPath string
		opt            vsOptions
	)
	cmd := &cobra.Command{
		Use:               "backup",
		Short:             "Takes snapshots of StatefulSets volumes",
		DisableAutoGenTag: true,
		RunE: func(cmd *cobra.Command, args []string) error {

			// start clock  so that we can determine total duration it has taken to complete the VolumeSnapshots
			startTime := time.Now()

			// prepare clients
			config, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfigPath)
			if err != nil {
				return err
			}
			// kubernetes client
			kc := kubernetes.NewForConfigOrDie(config)
			opt.kubeClient = kc
			// VolumeSnapshot client
			vc := vs_cs.NewForConfigOrDie(config)
			opt.vsClient = vc

			// Freeze database
			err = freezeDatabase()
			if err != nil {
				return err
			}

			// Take snapshot of the shards
			backupOutput, backupErr := opt.takeVolumeSnapshot(startTime)

			// Resume database
			err = resumeDatabase()
			if err != nil {
				return err
			}

			// If output directory specified, then write the output in "output.json" file in the specified directory
			if backupErr == nil && opt.outputDir != "" {
				err := backupOutput.WriteOutput(filepath.Join(opt.outputDir, restic.DefaultOutputFileName))
				if err != nil {
					return err
				}
			}
			return backupErr
		},
	}

	cmd.Flags().StringVar(&masterURL, "master", masterURL, "The address of the Kubernetes API server (overrides any value in kubeconfig)")
	cmd.Flags().StringVar(&kubeconfigPath, "kubeconfig", kubeconfigPath, "Path to kubeconfig file with authorization information (the master location is set by the master flag).")
	cmd.Flags().StringVar(&opt.namespace, "namespace", "default", "Namespace of Backup/Restore Session")
	cmd.Flags().StringVar(&opt.volumeSnapshotClassName, "snapshot-class", opt.volumeSnapshotClassName, "Name of the VolumeSnapshotClass")
	cmd.Flags().StringVar(&opt.outputDir, "output-dir", opt.outputDir, "Directory where output.json file will be written (keep empty if you don't need to write output in file)")
	cmd.Flags().StringSliceVar(&opt.shards, "shards", opt.shards, "Name of the shards separated by comma(,)")

	return cmd
}

// takeVolumeSnapshot creates VolumeSnapshot crd for each targeted PVCs
func (opt *vsOptions) takeVolumeSnapshot(startTime time.Time) (*restic.BackupOutput, error) {
	// in order to take snapshot of a PVC, we just need to know its name
	// read the names of the PVCs of the shards
	var pvcNames []string
	for i := range opt.shards {
		ss, err := opt.kubeClient.AppsV1().StatefulSets(opt.namespace).Get(opt.shards[i], metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		// take backup only 0 th StatefulSet PVCs.
		// send ss.Spec.Replicas instead of 1 to backup the PVCs of all replicas
		pvcNames = append(getStatefulSetPVCNames(ss.Spec.VolumeClaimTemplates, ss.Name, types.Int32P(1)))
	}

	// now generate VolumeSnapshot definition for each PVCs and create the VolumeSnapshot
	vsMeta := []metav1.ObjectMeta{}
	for _, pvcName := range pvcNames {
		// get VolumeSnapshot definition for this PVC
		volumeSnapshot := opt.getVolumeSnapshotDefinition(pvcName, string(startTime.Unix()))

		// now create the VolumeSnapshot object. CSI driver will take care of taking snapshot
		snapshot, err := opt.vsClient.SnapshotV1alpha1().VolumeSnapshots(opt.namespace).Create(&volumeSnapshot)
		if err != nil {
			return nil, err
		}
		vsMeta = append(vsMeta, snapshot.ObjectMeta)
	}

	// now wait for all the VolumeSnapshots are completed (ready to to use)
	backupOutput := restic.BackupOutput{}
	for i, pvcName := range pvcNames {
		// wait until this VolumeSnapshot is ready to use
		err := stash_util.WaitUntilVolumeSnapshotReady(opt.vsClient, vsMeta[i])
		if err != nil {
			return nil, err
		}

		// current VolumeSnapshot is completed. read current time and calculate the time it took to complete its backup.
		endTime := time.Now()

		backupOutput.HostBackupStats = append(backupOutput.HostBackupStats, api_v1beta1.HostBackupStats{
			Hostname: pvcName,
			Phase:    api_v1beta1.HostBackupSucceeded,
			Duration: endTime.Sub(startTime).String(),
		},
		)
	}
	return &backupOutput, nil
}

// getVolumeSnapshotDefinition takes a pvc name and returns a VolumeSnapshot object definition to backup it
func (opt *vsOptions) getVolumeSnapshotDefinition(targetPVCName, timestamp string) (volumeSnapshot vs.VolumeSnapshot) {
	return vs.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", targetPVCName, timestamp),
			Namespace: opt.namespace,
		},
		Spec: vs.VolumeSnapshotSpec{
			VolumeSnapshotClassName: &opt.volumeSnapshotClassName,
			Source: &corev1.TypedLocalObjectReference{
				Kind: apis.KindPersistentVolumeClaim,
				Name: targetPVCName,
			},
		},
	}
}

// getStatefulSetPVCNames generate the PVCs names from VolumeClaimTemplate of a StatefulSet
func getStatefulSetPVCNames(volList []corev1.PersistentVolumeClaim, statefulSetName string, replicas *int32) []string {
	pvcNames := make([]string, 0)
	for i := int32(0); i < *replicas; i++ {
		podName := fmt.Sprintf("%v-%v", statefulSetName, i)
		for _, vol := range volList {
			pvcNames = append(pvcNames, fmt.Sprintf("%v-%v", vol.Name, podName))
		}
	}
	return pvcNames
}
