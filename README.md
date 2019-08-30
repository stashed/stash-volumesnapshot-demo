# How to Run

## Create Alpha Cluster in GKE

- Install Kops: [kubernetes/kops](https://github.com/kubernetes/kops)
- Make sure `gcloud` is loged in into GCS account.
    - `gcloud login`
- Configure default credentials:
    ```console
    gcloud auth application-default login
    ```
- Create Cluster:
    ```console
    export KOPS_STATE_STORE=gs://appscode-qa/
    export PROJECT=ackube
    export KOPS_FEATURE_FLAGS=AlphaAllowGCE
    
    # create cluster configuration
    kops create cluster stash.k8s.local  \
    --zones us-central1-f                \
    --state ${KOPS_STATE_STORE}          \
    --project=${PROJECT}                 \
    --node-count=2                       \
    --kubernetes-version=v1.14.6

    # crate cluster
    kops update cluster stash.k8s.local --yes
    ```

Now, wait for few minutes to cluster to be ready.

- SSH into master node and add `--feature-gates=VolumeSnapshotDataSource=true` flag:
  ```console
  # run as root user
  sudo su

  # edit kube-apiserver and add: "--feature-gates=VolumeSnapshotDataSource=true" flag
  vi /etc/kubernetes/manifests/kube-apiserver.manifest
  ```

## Install GCP CSI Driver (Alpha)

- Clone this repo: [kubernetes-sigs/gcp-compute-persistent-disk-csi-driver](https://github.com/kubernetes-sigs/gcp-compute-persistent-disk-csi-driver)
- Checkout to `v0.5.1` release tag:
    ```console
    git checkout v0.5.1
    ```

- Setup Necessary Roles:
    ```
    export PROJECT=ackube
    export GCE_PD_SA_NAME=stash-volumesnapshot-demo
    export GCE_PD_SA_DIR=/home/emruz/dev/cred/gcs/
    
    # setup project
    ./deploy/setup-project.sh
    ```

- Deploy CSI driver
    ```console
    export GCE_PD_SA_DIR=/home/emruz/dev/cred/gcs
    export GCE_PD_DRIVER_VERSION=alpha
    ./deploy/kubernetes/deploy-driver.sh
    ```

- Verify that driver is running. You should see the flowing 3 pods are running

    ```console
    $ kubectl get pod | grep csi-gce-pd
    csi-gce-pd-controller-0   4/4     Running   1          71s
    csi-gce-pd-node-s5mvj     2/2     Running   0          70s
    csi-gce-pd-node-wgsmj     2/2     Running   0          70s
    ```

## Install Stash

```console
export STASH_IMAGE_TAG=support-vs-ft-model_linux_amd64

curl -fsSL https://github.com/stashed/installer/raw/v0.9.0-rc.0/deploy/stash.sh | bash -s -- --docker-registry=appscodeci
```

## Backup

- Create Storage Class using newly deployed CSI driver.
- Create VolumeSnapshotClass using same CSI driver.
- Create Function, Task, RBAC.
- Deploy StatefulSets. StatefulSets must use same storage class as VolumeSnapshotClass
- Create `BackupConfiguration`. Provide StatefulSet's name as shards params in `spec.Task.params` section.

**Make sure image field has been updated in Function spec to point the image with latest change.**
