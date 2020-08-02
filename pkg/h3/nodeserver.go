package h3

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubernetes/pkg/util/mount"
	"k8s.io/kubernetes/pkg/volume/util"

	csicommon "github.com/kubernetes-csi/drivers/pkg/csi-common"
)

type nodeServer struct {
	*csicommon.DefaultNodeServer
	mounter *mount.SafeFormatAndMount
}

type mountPoint struct {
	VolumeId  string
	MountPath string
}

func (ns *nodeServer) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	klog.Infof("NodePublishVolume: called with args %+v", *req)

	targetPath := req.GetTargetPath()

	notMnt, err := mount.New("").IsLikelyNotMountPoint(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(targetPath, 0750); err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
			notMnt = true
		} else {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	if !notMnt {
		// testing original mount point, make sure the mount link is valid
		if _, err := ioutil.ReadDir(targetPath); err == nil {
			klog.Infof("already mounted to target %s", targetPath)
			return &csi.NodePublishVolumeResponse{}, nil
		}
		// todo: mount link is invalid, now unmount and remount later (built-in functionality)
		klog.Warningf("ReadDir %s failed with %v, unmount this directory", targetPath, err)

		ns.mounter = &mount.SafeFormatAndMount{
			Interface: mount.New(""),
			Exec:      mount.NewOsExec(),
		}

		if err := ns.mounter.Unmount(targetPath); err != nil {
			klog.Errorf("Unmount directory %s failed with %v", targetPath, err)
			return nil, err
		}
	}

	mountOptions := req.GetVolumeCapability().GetMount().GetMountFlags()
	if req.GetReadonly() {
		mountOptions = append(mountOptions, "ro")
	}

	// Load default connection settings from secret
	secret, e := getSecret("h3-secret")

	storageType, storageConfig, bucket, flags, e := extractFlags(req.GetVolumeContext(), secret)
	if e != nil {
		klog.Warningf("storage parameter error: %s", e)
		return nil, e
	}

	e = Mount(storageType, storageConfig, bucket, targetPath, flags)
	if e != nil {
		if os.IsPermission(e) {
			return nil, status.Error(codes.PermissionDenied, e.Error())
		}
		if strings.Contains(e.Error(), "invalid argument") {
			return nil, status.Error(codes.InvalidArgument, e.Error())
		}
		return nil, status.Error(codes.Internal, e.Error())
	}

	return &csi.NodePublishVolumeResponse{}, nil
}

func extractFlags(volumeContext map[string]string, secret *v1.Secret) (string, string, string, map[string]string, error) {

	// Empty argument list
	flags := make(map[string]string)

	// Secret values are default, gets merged and overriden by corresponding PV values
	if secret != nil && secret.Data != nil && len(secret.Data) > 0 {

		// Needs byte to string casting for map values
		for k, v := range secret.Data {
			flags[k] = string(v)
		}
	} else {
		klog.Infof("No csi-h3 connection defaults secret found.")
	}

	if len(volumeContext) > 0 {
		for k, v := range volumeContext {
			flags[k] = v
		}
	}

	if e := validateFlags(flags); e != nil {
		return "", "", "", flags, e
	}

	storageType := flags["storageType"]
	storageConfig := flags["storageConfig"]
	bucket := flags["bucket"]

	delete(flags, "storageType")
	delete(flags, "storageConfig")
	delete(flags, "bucket")

	return storageType, storageConfig, bucket, flags, nil
}

func (ns *nodeServer) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {

	klog.Infof("NodeUnPublishVolume: called with args %+v", *req)

	targetPath := req.GetTargetPath()
	if len(targetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "NodeUnpublishVolume Target Path must be provided")
	}

	m := mount.New("")

	notMnt, err := m.IsLikelyNotMountPoint(targetPath)
	if err != nil && !mount.IsCorruptedMnt(err) {
		return nil, status.Error(codes.Internal, err.Error())
	}

	if notMnt && !mount.IsCorruptedMnt(err) {
		klog.Infof("Volume not mounted")

	} else {
		err = util.UnmountPath(req.GetTargetPath(), m)
		if err != nil {
			klog.Infof("Error while unmounting path: %s", err)
			// This will exit and fail the NodeUnpublishVolume making it to retry unmount on the next api schedule trigger.
			// Since we mount the volume with allow-non-empty now, we could skip this one too.
			return nil, status.Error(codes.Internal, err.Error())
		}

		klog.Infof("Volume %s unmounted successfully", req.VolumeId)
	}

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (ns *nodeServer) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	klog.Infof("NodeUnstageVolume: called with args %+v", *req)
	return &csi.NodeUnstageVolumeResponse{}, nil
}

func (ns *nodeServer) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	klog.Infof("NodeStageVolume: called with args %+v", *req)
	return &csi.NodeStageVolumeResponse{}, nil
}

func validateFlags(flags map[string]string) error {
	if _, ok := flags["storageType"]; !ok {
		return status.Errorf(codes.InvalidArgument, "missing volume context value: storageType")
	}
	if _, ok := flags["storageConfig"]; !ok {
		return status.Errorf(codes.InvalidArgument, "missing volume context value: storageConfig")
	}
	if _, ok := flags["bucket"]; !ok {
		return status.Errorf(codes.InvalidArgument, "missing volume context value: bucket")
	}
	return nil
}

func getSecret(secretName string) (*v1.Secret, error) {
	clientset, e := GetK8sClient()
	if e != nil {
		return nil, status.Errorf(codes.Internal, "can not create kubernetes client: %s", e)
	}

	kubeconfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	)

	namespace, _, err := kubeconfig.Namespace()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "can't get current namespace, error %s", secretName, err)
	}

	klog.Infof("Loading csi-h3 connection defaults from secret %s/%s", namespace, secretName)

	secret, e := clientset.CoreV1().
		Secrets(namespace).
		Get(secretName, metav1.GetOptions{})

	if e != nil {
		return nil, status.Errorf(codes.Internal, "can't load csi-h3 settings from secret %s: %s", secretName, e)
	}

	return secret, nil
}

func writeConfigFile(storageType string, storageConfig string) (string, error) {
	// Convert configuration to file contents
	config := []string{"[H3]"}
	if storageType == "redis" {
		config = append(config, "store = redis")

		parts := strings.Split(storageConfig, ":")
		if len(parts) != 2 {
			return "", status.Errorf(codes.InvalidArgument, fmt.Sprintf("H3 unknown storage config (should be \"host:port\"): %s", storageConfig))
		}
		config = append(config, "", "[REDIS]", fmt.Sprintf("host = %s", parts[0]), fmt.Sprintf("port = %s", parts[1]))
	} else {
		return "", status.Errorf(codes.InvalidArgument, fmt.Sprintf("H3 unknown storage type: %s", storageType))
	}

	// Write the configuration file
	configFile := "/tmp/h3config.ini"
	f, err := os.Create(configFile)
	if err != nil {
		f.Close()
		return "", err
	}
	for _, v := range config {
		fmt.Fprintln(f, v)
		if err != nil {
			f.Close()
			return "", err
		}
	}
	err = f.Close()
	if err != nil {
		return "", err
	}
	klog.Infof("H3 configuration file written at: %s", configFile)

	return configFile, nil
}

// Mount routine.
func Mount(storageType string, storageConfig string, bucket string, targetPath string, flags map[string]string) error {
	// Create configuration and bucket
	configFile, err := writeConfigFile(storageType, storageConfig)
	if err != nil {
		klog.Errorf(err.Error())
		return err
	}

	command := []string{"h3cli",
						"--config",
						configFile,
						"mb",
						fmt.Sprintf("h3://%s", bucket)}
	klog.Infof("H3 running: %s", strings.Join(command, " "))

	out, err := exec.Command(command[0], command[1:]...).Output()
	if err != nil {
		klog.Errorf(err.Error())
		// output := string(out[:])
		// klog.Infof("H3 output: %s", output)
		// return err
	}
	output := string(out[:])
	klog.Infof("H3 output: %s", output)

	// Back to mounting.
	mountCmd := "h3fuse"
	mountArgs := []string{}

	// h3fuse -o cfg=/path/to/config.ini -o bucket=bucket targetPath

	mountArgs = append(
		mountArgs,
		fmt.Sprintf("-o cfg=%s", configFile),
		fmt.Sprintf("-o bucket=%s", bucket),
		targetPath,
	)

	// Other user supplied flags are ignored

	// create target, os.Mkdirall is noop if it exists
	err = os.MkdirAll(targetPath, 0750)
	if err != nil {
		return err
	}

	klog.Infof("executing mount command cmd=%s, configFile=%s, bucket=%s, targetpath=%s", mountCmd, configFile, bucket, targetPath)

	out, err = exec.Command(mountCmd, mountArgs...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("mounting failed: %v cmd: '%s' configFile: '%s' bucket: '%s' targetpath: %s output: %q",
			err, mountCmd, configFile, bucket, targetPath, string(out))
	}

	return nil
}
