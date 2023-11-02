package executor

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/sirupsen/logrus"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/cmd/exec"

	"github.com/harvester/pcidevices/pkg/apis/devices.harvesterhci.io/v1beta1"
	"github.com/harvester/pcidevices/pkg/generated/clientset/versioned/scheme"
)

type RemoteCommandExecutor struct {
	options   *exec.ExecOptions
	outBuffer *bytes.Buffer
	errBuffer *bytes.Buffer
}

// NewRemoteCommandExecutor is an implementation of Executor that runs commands in the driver pod
// which allows us to ship custom drivers as container images
func NewRemoteCommandExecutor(ctx context.Context, config *rest.Config, nodeName string, namespace string, label string) (*RemoteCommandExecutor, error) {
	cfgCopy := *config
	cfgCopy.GroupVersion = &schema.GroupVersion{Group: "", Version: "v1"}
	cfgCopy.NegotiatedSerializer = serializer.WithoutConversionCodecFactory{CodecFactory: scheme.Codecs}
	cfgCopy.APIPath = "/api"
	client, err := kubernetes.NewForConfig(&cfgCopy)
	if err != nil {
		return nil, fmt.Errorf("error generating client for config in remote command executor: %v", err)
	}

	pod, err := fetchPod(ctx, client, nodeName, namespace, label)
	if err != nil {
		return nil, err
	}

	logrus.Debugf("found pod %s for node %s", pod.Name, nodeName)
	iostreams, _, outBuffer, errBuffer := genericclioptions.NewTestIOStreams()

	streamOpts := exec.StreamOptions{
		Namespace:     namespace,
		PodName:       pod.Name,
		ContainerName: pod.Spec.Containers[0].Name,
		IOStreams:     iostreams,
		TTY:           false,
		Quiet:         true,
		Stdin:         false,
	}

	options := &exec.ExecOptions{
		StreamOptions: streamOpts,
		PodClient:     client.CoreV1(),
		Config:        &cfgCopy,
		Executor:      &exec.DefaultRemoteExecutor{},
	}

	r := &RemoteCommandExecutor{
		options:   options,
		outBuffer: outBuffer,
		errBuffer: errBuffer,
	}
	return r, nil
}

func (r *RemoteCommandExecutor) Run(cmd string, args []string) ([]byte, error) {
	cmdString := fmt.Sprintf("%s %s", cmd, strings.Join(args, " "))
	r.options.Command = []string{"/bin/sh", "-c", cmdString}
	err := r.options.Run()
	if err != nil {
		return r.errBuffer.Bytes(), fmt.Errorf("error during command execution: %v", err)
	}
	return r.outBuffer.Bytes(), nil
}

// fetchPod will identify the nvidia driver pod on the host matching nodeName where the remote command will be executed
func fetchPod(ctx context.Context, client *kubernetes.Clientset, nodeName string, namespace string, label string) (*corev1.Pod, error) {
	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: label,
	})

	if err != nil {
		return nil, fmt.Errorf("error listing pods in ns %s for label selector %s: %v", v1beta1.DefaultNamespace, v1beta1.NvidiaDriverLabel, err)
	}

	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("no nvidia driver pods found, cannot proceed with execution")
	}

	var matchingPods []corev1.Pod
	for _, v := range pods.Items {
		if v.Spec.NodeName == nodeName {
			matchingPods = append(matchingPods, v)
		}
	}

	if len(matchingPods) == 0 || len(matchingPods) > 1 {
		return nil, fmt.Errorf("expected to find exactly one pod for nvidia driver on node %s, but got %d", nodeName, len(matchingPods))
	}

	return &matchingPods[0], nil
}
