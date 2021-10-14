
# CSI H3 mount plugin

**This project is a clone of the [CSI rclone mount plugin](https://github.com/wunderio/csi-rclone), modified for H3. This README has been updated for H3, but if you need more information, please refer to the code of the original module.**

This project implements a Container Storage Interface (CSI) plugin that allows using [H3](https://github.com/CARV-ICS-FORTH/H3) as a storage backend. H3 parameters can be configured using a `Secret` or `PersistentVolume` `volumeAttributes`.

## Kubernetes cluster compatability
Has been tested with 1.19.x and 1.22.x.

## Installing the CSI driver to a Kubernetes cluster

CSI H3 is deployed using [Helm](https://helm.sh) (version 3).

Create a namespace for `csi-h3` and install CSI H3:
```
kubectl create namespace csi-h3
helm install -n csi-h3 csi-h3 ./chart/csi-h3
```

## Usage and examples

To use `csi-h3` you need to configure the H3 object store with the appropriate storage URI and bucket.

1. Set up a storage backend. You can use [Redis](https://redis.io), or any compatible key-value store (like [Ardb](https://github.com/yinqiwen/ardb)).

    Deploy the Redis example with:
    ```
    kubectl apply -f example/kubernetes/redis-example.yaml
    ```

2. Configure defaults by pushing a `Secret` to the `kube-system` namespace. This is optional if you will always set `volumeAttributes` in `PersistentVolume` definitions. The bucket specified will be created if it does not already exist.

    ```
    apiVersion: v1
    kind: Secret
    metadata:
      name: h3-secret
    type: Opaque
    stringData:
      storageUri: "redis://redis.default.svc:6379"
      bucket: "b1"
    ```

    Deploy the example secret with:
    ```
    kubectl apply -f example/kubernetes/h3-secret-example.yaml --namespace kube-system
    ```

3. You can override configuration in the `PersistentVolume` resource definition. Leave `volumeAttributes` empty if you don't want to. Keys in `volumeAttributes` will be merged with predefined parameters.

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
          storageUri: "redis://redis.default.svc:6379"
          bucket: "b1"
    ```

    The provided NGINX example contains two pods (with associated services) sharing the same `PersistentVolume` that uses the "b1" bucket on the `redis://redis.default.svc:6379` storage backend. The first pod runs a web server and the second a web-based file browser.

    Deploy the NGINX example with:
    ```
    kubectl apply -f example/kubernetes/nginx-example.yaml
    ```

    Then:
    * Forward the NGINX port to localhost with `kubectl port-forward svc/nginx-example 8000:80`.
    * Forward the File Browser port to localhost with `kubectl port-forward svc/filebrowser-example 8080:80`.
    * Point your browser to http://localhost:8080 (File Browser), drop in a file named `index.html` and verify it is visible at http://localhost:8000 (NGINX).

## Building plugin and creating image
The code references the project repository at GitHub. If you fork the repository, you have to change Go includes in several places (use search and replace).

1. First push the changed code to remote. The build will use paths from the `pkg/` directory.

2. Build the plugin:
    ```
    make plugin
    ```

3. Build the container and inject the plugin into it:
    ```
    make container
    ```

4. Change docker.io account in `Makefile` and use `make push` to push the image to remote:
    ```
    make push
    ```

## Acknowledgements
This project has received funding from the European Union’s Horizon 2020 research and innovation programme under grant agreement No 825061 (EVOLVE - [website](https://www.evolve-h2020.eu>), [CORDIS](https://cordis.europa.eu/project/id/825061)).
