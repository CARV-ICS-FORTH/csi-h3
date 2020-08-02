
# CSI H3 mount plugin

* This project is a clone of the [CSI rclone mount plugin](https://github.com/wunderio/csi-rclone), modified for H3. This README has been updated for H3, but if you need more information, please refer to the code of the original module. *

This project implements Container Storage Interface (CSI) plugin that allows using [H3](https://github.com/CARV-ICS-FORTH/H3) as the storage backend. H3 mount points and parameters can be configured using Secret or PersistentVolume volumeAttibutes.

## Kubernetes cluster compatability
Has only been tested with 1.15.x.


## Installing CSI driver to kubernetes cluster
TLDR: `kubectl apply -f deploy/kubernetes --username=admin --password=123`

1. Set up storage backend. You can use [Redis](https://redis.io), or any compatible key-value store.

2. Configure defaults by pushing secret to kube-system namespace. This is optional if you will always define `volumeAttributes` in PersistentVolume.

```
apiVersion: v1
kind: Secret
metadata:
  name: h3-secret
type: Opaque
stringData:
  storageType: "redis"
  storageConfig: "127.0.0.1:6379"
  bucket: "b1"
```

Deploy example secret
> `kubectl apply -f example/kubernetes/h3-secret-example.yaml --namespace kube-system`

3. You can override configuration via PersistentStorage resource definition. Leave volumeAttributes empty if you don't want to. Keys in `volumeAttributes` will be merged with predefined parameters.

```
apiVersion: v1
kind: PersistentVolume
metadata:
  name: data-h3-example
  labels:
    name: data-h3-example
spec:
  accessModes:
  - ReadWriteMany
  capacity:
    storage: 10Gi
  storageClassName: h3
  csi:
    driver: csi-h3
    volumeHandle: data-id
    volumeAttributes:
      storageType: "redis"
      storageConfig: "127.0.0.1:6379"
      bucket: "b1"
```

Deploy example definition
> `kubectl apply -f example/kubernetes/nginx-example.yaml`


## Building plugin and creating image
Current code is referencing projects repository on github.com. If you fork the repository, you have to change go includes in several places (use search and replace).


1. First push the changed code to remote. The build will use paths from `pkg/` directory.

2. Build the plugin
```
make plugin
```

3. Build the container and inject the plugin into it.
```
make container
```

4. Change docker.io account in `Makefile` and use `make push` to push the image to remote.
```
make push
```
