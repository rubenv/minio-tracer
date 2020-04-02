package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func main() {
	err := do()
	if err != nil {
		log.Fatal(err)
	}
}

func do() error {
	secretName := os.Getenv("TRACE_SECRET")
	if secretName == "" {
		secretName = "minio"
	}

	serviceName := os.Getenv("TRACE_SERVICE")
	if serviceName == "" {
		serviceName = "minio"
	}

	namespace := os.Getenv("TRACE_NAMESPACE")
	if namespace == "" {
		namespace = "default"
	}

	// Try in-cluster first
	config, err := rest.InClusterConfig()
	if err != nil {
		// Fall back to kubeconfig
		kubeconfig, ok := os.LookupEnv("KUBECONFIG")
		if !ok {
			hd, err := os.UserHomeDir()
			if err != nil {
				return err
			}

			kubeconfig = filepath.Join(hd, ".kube", "config")
		}

		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return err
		}
	} else {
		// Grab namespace from in-cluster info
		ns, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
		if err == nil {
			namespace = string(ns)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	// Fetch credentials
	secret, err := clientset.CoreV1().Secrets(namespace).Get(secretName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	ak, ok := secret.Data["accesskey"]
	if !ok {
		return fmt.Errorf("Missing accesskey in secret")
	}

	sk, ok := secret.Data["secretkey"]
	if !ok {
		return fmt.Errorf("Missing secretkey in secret")
	}

	// Watch endpoint
	for {
		watch, err := clientset.CoreV1().Endpoints(namespace).Watch(metav1.ListOptions{})
		if err != nil {
			return err
		}
		defer watch.Stop()

		events := watch.ResultChan()
		for range events {
			endpoints, err := getEndpoints(namespace, serviceName, clientset)
			if err != nil {
				return err
			}

			err = ensureTracing(string(ak), string(sk), endpoints)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func getEndpoints(namespace, serviceName string, client *kubernetes.Clientset) ([]string, error) {
	endpoints, err := client.CoreV1().Endpoints(namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := make([]string, 0)
	for _, ep := range endpoints.Items {
		if ep.ObjectMeta.Name != serviceName || ep.ObjectMeta.Namespace != namespace {
			continue
		}

		for _, ss := range ep.Subsets {
			for _, addr := range ss.Addresses {
				result = append(result, fmt.Sprintf("%s:%d", addr.IP, ss.Ports[0].Port))
			}
		}
	}

	return result, nil
}

type tracer struct {
	ctx       context.Context
	cf        context.CancelFunc
	endpoint  string
	accesskey string
	secretkey string
}

var running = make(map[string]*tracer)

func ensureTracing(ak, sk string, endpoints []string) error {
	unseen := make(map[string]*tracer)
	for r, t := range running {
		unseen[r] = t
	}

	for _, ep := range endpoints {
		delete(unseen, ep)

		_, ok := running[ep]
		if ok {
			continue
		}

		t, err := startTracing(ak, sk, ep)
		if err != nil {
			return err
		}
		running[ep] = t
	}

	for ep, t := range unseen {
		t.stop()
		delete(running, ep)
	}

	return nil
}

func startTracing(accesskey, secretkey, endpoint string) (*tracer, error) {
	ctx := context.Background()
	ctx, cf := context.WithCancel(ctx)

	t := &tracer{
		ctx:       ctx,
		cf:        cf,
		endpoint:  endpoint,
		accesskey: accesskey,
		secretkey: secretkey,
	}
	go t.run()
	return t, nil
}

func (t *tracer) stop() {
	t.cf()
}

func (t *tracer) run() {
	err := t.do()
	if err != nil {
		log.Printf("Tracer failed for %s: %s", t.endpoint, err)
	}
}

func (t *tracer) do() error {
	conn := fmt.Sprintf("MC_HOST_host=http://%s:%s@%s", t.accesskey, t.secretkey, t.endpoint)
	cmd := exec.CommandContext(t.ctx, "mc", "admin", "trace", "host")
	cmd.Env = append(os.Environ(), conn)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	err := cmd.Run()
	if t.ctx.Err() == context.Canceled {
		err = nil // Ignore errors after a request to stop
	}
	return err
}
